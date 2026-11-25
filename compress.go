package main

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// compressible 判断响应类型是否值得 gzip 压缩。
// 图片、字体、视频等本身已压缩的格式直接跳过，避免徒增 CPU 与体积。
func compressible(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		return false
	}
	for _, prefix := range []string{
		"text/",
		"application/javascript",
		"application/json",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml",
	} {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	wroteHeader bool
	compress    bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	// 只压缩正常的完整响应；304/206/4xx/5xx 保持原样透传。
	if code == http.StatusOK && compressible(w.Header().Get("Content-Type")) {
		w.compress = true
		w.gz = gzip.NewWriter(w.ResponseWriter)
		// 压缩后长度由 gzip 决定，必须去掉原 Content-Length，否则浏览器会截断。
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if !w.compress {
		return w.ResponseWriter.Write(p)
	}
	return w.gz.Write(p)
}

func (w *gzipResponseWriter) Flush() {
	if w.compress {
		w.gz.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// gzipMiddleware 对可压缩响应做 gzip 编码。
//
// public/vendor 下的 marked / DOMPurify / highlight.js 未压缩时合计约 214KB，
// 弱网（400kbps）下光下载就要 4 秒以上；压缩后约 60KB，首屏直接受益。
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// HEAD 不应产生响应体，交给下游处理即可。
		if r.Method == http.MethodHead || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipResponseWriter{ResponseWriter: w}
		defer func() {
			if gw.compress {
				gw.gz.Close()
			}
		}()
		next.ServeHTTP(gw, r)
	})
}
