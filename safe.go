package main

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var errUnsafePath = errors.New("path escapes blog root")

// resolveInsideRoot 把用户可控的相对路径解析为根目录内的绝对路径。
//
// 路径防护方案选型：go.mod 声明的语言版本是 Go 1.21，无法使用 Go 1.24 才引入的
// os.Root，因此采用 filepath.IsLocal + EvalSymlinks 结果做根目录前缀校验的回退方案。
//
// 残余风险（TOCTOU）：EvalSymlinks 校验通过后、调用方 os.Open 之前，若攻击者能在
// 本机替换路径中的符号链接，仍存在短暂窗口可读取根目录外文件。该攻击要求本地文件
// 系统写权限，不属于当前威胁模型；若将来部署到多用户环境，应把 go.mod 升到 1.24+
// 并改用 os.OpenRoot/root.Open。
func resolveInsideRoot(root, rel string) (string, error) {
	// 允许调用方传入以 / 开头的“根目录相对路径”，但绝对路径（含盘符）仍会被拒绝。
	rel = strings.TrimLeft(filepath.FromSlash(rel), `/\`)
	if rel == "" || rel == "." || !filepath.IsLocal(rel) {
		return "", errUnsafePath
	}

	full := filepath.Join(root, rel)
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	if !isInsideRoot(root, resolved) {
		return "", errUnsafePath
	}
	return resolved, nil
}

// isInsideRoot 判断 p 是否位于 root 之内。root 必须是已解析符号链接的绝对路径。
// Windows 下文件系统大小写不敏感，比较时统一转小写。
func isInsideRoot(root, p string) bool {
	r := filepath.Clean(root)
	q := filepath.Clean(p)
	if r == q {
		return true
	}
	if runtime.GOOS == "windows" {
		r = strings.ToLower(r)
		q = strings.ToLower(q)
	}
	return strings.HasPrefix(q, r+string(filepath.Separator))
}

// safePathMiddleware 在路由分发前拒绝明显恶意的 URL 路径，确保编码后的
// ../、..\ 和 NUL 字节不会先被 ServeMux 301 重定向再落到其它 handler。
func safePathMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isSafeRequestPath(r.URL) {
			http.Error(w, "非法路径", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeRequestPath(u *url.URL) bool {
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2e") ||
		strings.Contains(escaped, "%5c") ||
		strings.Contains(escaped, "%00") {
		return false
	}
	if strings.ContainsRune(u.Path, 0) {
		return false
	}
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

// serveBlogFile 提供 blog-files/ 下的静态资源。相比 http.FileServer(http.Dir(...))，
// 这里额外做了符号链接逃逸校验，并禁用目录列表。
func serveBlogFile(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rel := strings.TrimPrefix(r.URL.Path, "/blog-files/")
		full, err := resolveInsideRoot(root, rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(full)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	}
}
