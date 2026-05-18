package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed public/index.html
var frontend embed.FS

// 配置结构
type Config struct {
	Port     int    `json:"port"`
	BlogDir  string `json:"blogDir"`
	BlogName string `json:"blogName"` // 新增
}

var (
	config     Config
	configFile = flag.String("config", "config.json", "配置文件路径")
	port       = flag.Int("port", 0, "服务端口（覆盖配置文件）")
	blogDir    = flag.String("dir", "", "博客文件根目录（覆盖配置文件）")
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

func main() {
	flag.Parse()

	initConfigFile()
	loadConfig()

	if *port != 0 {
		config.Port = *port
	}
	if *blogDir != "" {
		config.BlogDir = *blogDir
	}

	initBlogDir()

	absBlogDir, err := filepath.Abs(config.BlogDir)
	if err != nil {
		log.Fatalf("无法解析博客目录: %v", err)
	}

	mux := http.NewServeMux()

	// 博客静态文件
	blogFileServer := http.FileServer(http.Dir(absBlogDir))
	mux.Handle("/blog-files/", http.StripPrefix("/blog-files/", blogFileServer))

	// API：获取配置（新增）
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{
			"blogName": config.BlogName,
			"rootDir":  absBlogDir,
		})
	})

	// API：获取目录树
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		tree := buildTree(absBlogDir, "")
		resp := map[string]interface{}{
			"rootDir":  absBlogDir,
			"children": tree,
		}
		writeJSON(w, resp)
	})

	// API：获取文件内容
	mux.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, `{"error":"缺少 path 参数"}`, http.StatusBadRequest)
			return
		}

		cleanPath := filepath.Clean(filePath)
		fullPath := filepath.Join(absBlogDir, cleanPath)
		rel, err := filepath.Rel(absBlogDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			http.Error(w, `{"error":"禁止访问"}`, http.StatusForbidden)
			return
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
			return
		}

		dirPath := filepath.ToSlash(filepath.Dir(cleanPath))
		if dirPath == "." {
			dirPath = ""
		}

		ext := strings.ToLower(filepath.Ext(cleanPath))
		fileType := "markdown"
		if ext == ".html" {
			fileType = "html"
		}

		resp := map[string]string{
			"content":  string(content),
			"dirPath":  dirPath,
			"fileType": fileType,
		}
		writeJSON(w, resp)
	})

	// 前端页面
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			data, err := frontend.ReadFile("public/index.html")
			if err != nil {
				http.Error(w, "前端页面丢失", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		fileServer := http.FileServer(http.FS(frontend))
		fileServer.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("🚀 %s 已启动: http://localhost%s", config.BlogName, addr)
	log.Printf("📂 博客目录: %s", absBlogDir)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// 初始化配置文件（若不存在则创建）
func initConfigFile() {
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		defaultCfg := Config{
			Port:     3000,
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

// 递归生成目录树
func buildTree(basePath, relPath string) []*TreeNode {
	fullPath := filepath.Join(basePath, relPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil
	}

	var nodes []*TreeNode

	// 目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		childRel := filepath.Join(relPath, entry.Name())
		children := buildTree(basePath, childRel)
		if len(children) > 0 {
			nodes = append(nodes, &TreeNode{
				Name:     entry.Name(),
				Type:     "directory",
				Path:     filepath.ToSlash(childRel),
				Children: children,
			})
		}
	}

	// 文件
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".md" && ext != ".html" {
			continue
		}

		fileRel := filepath.Join(relPath, name)
		dirPath := filepath.ToSlash(relPath)
		if dirPath == "" || dirPath == "." {
			dirPath = "."
		}

		fileType := "markdown"
		if ext == ".html" {
			fileType = "html"
		}

		nodes = append(nodes, &TreeNode{
			Name:     name,
			Type:     "file",
			Path:     filepath.ToSlash(fileRel),
			DirPath:  dirPath,
			FileType: fileType,
		})
	}

	return nodes
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}