package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed public
var frontend embed.FS

// 配置结构
type Config struct {
	Port        int    `json:"port"`
	Bind        string `json:"bind"` // 监听地址，默认仅本机 127.0.0.1
	BlogDir     string `json:"blogDir"`
	BlogName    string `json:"blogName"`
	ReloadToken string `json:"reloadToken"` // 非本机调用 /reload 时使用的共享密钥
	AuthUser    string `json:"authUser"`    // 绑定非回环地址时启用的 Basic Auth 用户名
	AuthPass    string `json:"authPass"`    // Basic Auth 密码（明文传输，建议配合 HTTPS 反向代理）
}

var (
	config      Config
	configFile  = flag.String("config", "config.json", "配置文件路径")
	port        = flag.Int("port", 0, "服务端口（覆盖配置文件）")
	bind        = flag.String("bind", "", "监听地址（覆盖配置文件，默认 127.0.0.1）")
	blogDir     = flag.String("dir", "", "博客文件根目录（覆盖配置文件）")
	reloadToken = flag.String("reload-token", "", "非本机调用 /reload 的共享密钥（覆盖配置文件）")
	authUser    = flag.String("auth-user", "", "Basic Auth 用户名（覆盖配置文件）")
	authPass    = flag.String("auth-pass", "", "Basic Auth 密码（覆盖配置文件）")
)

// 目录树节点
type TreeNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Path     string      `json:"path,omitempty"`
	DirPath  string      `json:"dirPath,omitempty"`
	FileType string      `json:"fileType,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// App 持有服务运行期的只读状态；元信息快照通过原子替换更新。
type App struct {
	root        string
	cfg         Config
	snapshot    atomic.Pointer[Snapshot]
	indexHTML   []byte
	indexETag   string
	publicFS    fs.FS
	vendorETags map[string]string // /vendor/... 路径 -> 内容 ETag
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()

	initConfigFile()
	loadConfig()

	if *port != 0 {
		config.Port = *port
	}
	if *bind != "" {
		config.Bind = *bind
	}
	if *blogDir != "" {
		config.BlogDir = *blogDir
	}
	if *reloadToken != "" {
		config.ReloadToken = *reloadToken
	}
	if *authUser != "" {
		config.AuthUser = *authUser
	}
	if *authPass != "" {
		config.AuthPass = *authPass
	}
	if strings.TrimSpace(config.Bind) == "" {
		config.Bind = "127.0.0.1"
	}
	if config.BlogDir == "" {
		config.BlogDir = "blog-files"
	}
	if config.BlogName == "" {
		config.BlogName = "轻舟博客"
	}

	initBlogDir()

	app, err := newApp(config.BlogDir, config)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	addr := net.JoinHostPort(config.Bind, strconv.Itoa(config.Port))
	log.Printf("🚀 %s 已启动: http://%s", config.BlogName, addr)
	log.Printf("📂 博客目录: %s", app.root)
	if !isLoopbackBind(config.Bind) {
		log.Printf("⚠️ 当前监听非回环地址 %s，局域网/公网均可访问", config.Bind)
		if config.ReloadToken == "" {
			log.Printf("⚠️ 未配置 reloadToken，外部请求无法触发 /reload")
		}
		if config.AuthUser == "" || config.AuthPass == "" {
			log.Printf("⚠️ 未启用 Basic Auth，文章内容对网络内所有人可见")
		} else {
			log.Printf("⚠️ Basic Auth 凭据通过明文 HTTP 传输，可能被窃听，请置于 HTTPS 反向代理之后")
		}
	}
	if err := http.ListenAndServe(addr, app.routes()); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

func newApp(blogDir string, cfg Config) (*App, error) {
	if cfg.BlogName == "" {
		cfg.BlogName = "轻舟博客"
	}
	if strings.TrimSpace(cfg.Bind) == "" {
		cfg.Bind = "127.0.0.1"
	}
	absDir, err := filepath.Abs(blogDir)
	if err != nil {
		return nil, fmt.Errorf("无法解析博客目录: %w", err)
	}
	// 启动时解析根目录自身的符号链接，后续所有路径校验都以真实路径为基准。
	root, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		return nil, fmt.Errorf("无法解析博客目录真实路径: %w", err)
	}

	publicFS, err := fs.Sub(frontend, "public")
	if err != nil {
		return nil, fmt.Errorf("前端资源丢失: %w", err)
	}
	indexHTML, err := fs.ReadFile(publicFS, "index.html")
	if err != nil {
		return nil, fmt.Errorf("前端页面丢失: %w", err)
	}
	sum := sha256.Sum256(indexHTML)

	app := &App{
		root:      root,
		cfg:       cfg,
		indexHTML: indexHTML,
		indexETag: `"` + hex.EncodeToString(sum[:16]) + `"`,
		publicFS:  publicFS,
	}
	app.vendorETags, err = buildVendorETags(publicFS)
	if err != nil {
		return nil, err
	}
	if snap, err := app.reload(); err != nil {
		log.Printf("⚠️ 扫描博客目录失败: %v", err)
		app.snapshot.Store(&Snapshot{Generated: time.Now()})
	} else {
		log.Printf("📚 已索引 %d 篇文章（草稿 %d 篇）", snap.PublishedCount(), snap.DraftCount())
	}
	return app, nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()

	// 博客静态文件（已做根目录与符号链接校验）
	mux.HandleFunc("/blog-files/", serveBlogFile(a.root))

	// API：获取配置
	mux.HandleFunc("/api/config", a.handleConfig)

	// API：获取目录树
	mux.HandleFunc("/api/tree", a.handleTree)

	// API：获取文件内容
	mux.HandleFunc("/api/file", a.handleFile)

	// 在系统文件管理器中定位文章（仅本机）
	mux.HandleFunc("/api/reveal", a.handleReveal)

	// RSS 订阅
	mux.HandleFunc("/feed.xml", a.handleFeed)

	// 站点地图（仅收录已发布文章）
	mux.HandleFunc("/sitemap.xml", a.handleSitemap)

	// 带 SEO 元信息的文章入口页
	mux.HandleFunc("/article/", a.handleArticle)

	// 重新扫描博客目录（仅回环地址或持有共享密钥）
	mux.HandleFunc("/reload", a.handleReload)

	// 前端页面
	mux.HandleFunc("/", a.handleIndex)

	return loggingMiddleware(gzipMiddleware(safePathMiddleware(a.authMiddleware(mux))))
}

func (a *App) reload() (*Snapshot, error) {
	snap, err := scanBlog(a.root)
	if err != nil {
		return nil, err
	}
	a.snapshot.Store(snap)
	return snap, nil
}

func (a *App) currentSnapshot() *Snapshot {
	if snap := a.snapshot.Load(); snap != nil {
		return snap
	}
	return &Snapshot{}
}

func (a *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	payload := map[string]string{
		"blogName": a.cfg.BlogName,
	}
	// 根目录是本机绝对路径，只对回环请求返回，避免绑定局域网时泄露宿主机目录结构。
	if isLoopbackRequest(r) {
		payload["rootDir"] = a.root
	}
	writeJSON(w, payload)
}

func (a *App) handleTree(w http.ResponseWriter, r *http.Request) {
	snap := a.currentSnapshot()
	writeJSON(w, map[string]interface{}{
		"rootDir":  a.root,
		"children": snap.Tree,
	})
}

func (a *App) handleFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, `{"error":"缺少 path 参数"}`, http.StatusBadRequest)
		return
	}

	rel := filepath.ToSlash(strings.TrimLeft(filepath.FromSlash(filePath), `/\`))
	ext := strings.ToLower(path.Ext(rel))
	if ext != ".md" && ext != ".html" {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	fullPath, err := resolveInsideRoot(a.root, rel)
	if err != nil {
		if errors.Is(err, errUnsafePath) {
			http.Error(w, `{"error":"禁止访问"}`, http.StatusForbidden)
		} else {
			http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		}
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	dirPath := path.Dir(rel)
	if dirPath == "." {
		dirPath = ""
	}
	fileType := "markdown"
	if ext == ".html" {
		fileType = "html"
	}

	writeJSON(w, map[string]string{
		"content":  string(content),
		"dirPath":  dirPath,
		"fileType": fileType,
	})
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/" {
		// 提供本地嵌入的前端静态资源（/vendor/...），不依赖外部 CDN。
		a.serveVendor(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("ETag", a.indexETag)
	if etagMatches(r.Header.Get("If-None-Match"), a.indexETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Write(a.indexHTML)
}

// buildVendorETags 为 public/ 下的静态资源预计算内容哈希。
//
// embed.FS 的文件 ModTime 恒为零值，http.FileServer 因此不会输出 Last-Modified，
// 浏览器只能靠 ETag 做条件请求。这里显式提供强 ETag，配合长缓存避免每次刷新
// 都重新下载 marked / DOMPurify / highlight.js（合计约 214KB）。
func buildVendorETags(publicFS fs.FS) (map[string]string, error) {
	etags := make(map[string]string)
	err := fs.WalkDir(publicFS, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || p == "index.html" {
			return nil
		}
		data, err := fs.ReadFile(publicFS, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		etags["/"+filepath.ToSlash(p)] = `"` + hex.EncodeToString(sum[:16]) + `"`
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("前端静态资源索引失败: %w", err)
	}
	return etags, nil
}

// serveVendor 提供嵌入的 vendor 静态资源。
//
// 资源内容随二进制固化，可用长缓存 + 强 ETag 做条件请求；
// 命中缓存或 304 时完全跳过文件读取，首屏不再等待 214KB 脚本下载。
func (a *App) serveVendor(w http.ResponseWriter, r *http.Request) {
	etag := a.vendorETags[r.URL.Path]
	if etag == "" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	http.FileServer(http.FS(a.publicFS)).ServeHTTP(w, r)
}

func (a *App) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, `{"error":"仅支持 POST"}`, http.StatusMethodNotAllowed)
		return
	}
	if !a.reloadAllowed(r) {
		http.Error(w, `{"error":"未授权的 reload 请求"}`, http.StatusForbidden)
		return
	}

	snap, err := a.reload()
	if err != nil {
		http.Error(w, `{"error":"重新扫描失败"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{
		"ok":        true,
		"articles":  snap.PublishedCount(),
		"drafts":    snap.DraftCount(),
		"generated": snap.Generated,
	})
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// etagMatches 实现 If-None-Match 的弱比较。
func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == etag || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}

// 初始化配置文件（若不存在则创建）
func initConfigFile() {
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		defaultCfg := Config{
			Port:     3000,
			Bind:     "127.0.0.1",
			BlogDir:  "blog-files",
			BlogName: "轻舟博客", // 默认名称
		}
		data, _ := json.MarshalIndent(defaultCfg, "", "  ")
		err := os.WriteFile(*configFile, data, 0644)
		if err != nil {
			log.Printf("自动创建配置文件失败: %v", err)
		} else {
			log.Printf("已创建默认配置文件: %s", *configFile)
		}
	}
}

// 加载配置文件
func loadConfig() {
	config = Config{
		Port:     3000,
		Bind:     "127.0.0.1",
		BlogDir:  "blog-files",
		BlogName: "轻舟博客", // 默认值
	}

	data, err := os.ReadFile(*configFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("未找到配置文件 %s，使用默认值", *configFile)
		} else {
			log.Printf("读取配置文件失败: %v，使用默认值", err)
		}
		return
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("配置文件解析失败: %v，使用默认值", err)
		return
	}
	log.Printf("已加载配置文件: %s", *configFile)
}

// 初始化博客目录（若不存在则创建并添加示例文件）
func initBlogDir() {
	dir := config.BlogDir
	if dir == "" {
		dir = "blog-files"
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
		log.Printf("已创建博客目录: %s", dir)

		sampleFile := filepath.Join(dir, "关于.md")
		sampleContent := `# 欢迎使用 ` + config.BlogName + `

## 关于本站
这是一个基于 **Go** 和 **Markdown/HTML** 的轻量博客系统。

## 功能特点
- 自动读取目录结构，生成侧边栏导航
- 支持 .md 和 .html两种格式
- 右侧悬浮目录，点击快速跳转
- 代码高亮、图片自动适配路径
- 无需数据库，纯文件驱动

## 快速开始
1. 将你的 .md 或 .html 文件放入当前目录（或其子文件夹）
2. 图片等资源可直接放在同级目录，文章内使用相对路径引用
3. 刷新页面即可看到新增的文章

> 享受写作的乐趣吧！
`
		os.WriteFile(sampleFile, []byte(sampleContent), 0644)
		log.Printf("已创建示例文章: %s", sampleFile)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}
