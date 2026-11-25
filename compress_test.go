package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

// decodeBody 在响应带 Content-Encoding: gzip 时解压，否则原样返回。
func decodeBody(t *testing.T, body io.Reader, encoding string) string {
	t.Helper()
	if encoding != "gzip" {
		data, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("读取响应体: %v", err)
		}
		return string(data)
	}
	zr, err := gzip.NewReader(body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer zr.Close()
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("解压响应体: %v", err)
	}
	return string(data)
}

func TestVendorGzipCompression(t *testing.T) {
	app, _ := newTestApp(t)

	rr := serve(t, app, http.MethodGet, "/vendor/highlight.min.js", func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "gzip")
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if enc := rr.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", enc)
	}
	if vary := rr.Header().Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %q, want 包含 Accept-Encoding", vary)
	}
	// 压缩后不能再带原始 Content-Length，否则浏览器按旧长度截断。
	if cl := rr.Header().Get("Content-Length"); cl != "" {
		t.Errorf("压缩响应不应保留 Content-Length，实际 %q", cl)
	}

	body := decodeBody(t, rr.Body, rr.Header().Get("Content-Encoding"))
	if !strings.Contains(body, "highlight.js") {
		t.Fatal("解压后的内容不是 highlight.js")
	}
	// 121KB 的脚本压缩后应明显小于原始体积。
	if rr.Body.Len() >= 121727 {
		t.Fatalf("压缩后体积 %d 未减小", rr.Body.Len())
	}
}

func TestVendorWithoutGzipSupport(t *testing.T) {
	app, _ := newTestApp(t)

	// 客户端不支持 gzip 时必须原样返回，否则浏览器无法解析。
	rr := serve(t, app, http.MethodGet, "/vendor/marked.umd.js", func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "identity")
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if enc := rr.Header().Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding = %q, want 空", enc)
	}
	if !strings.Contains(rr.Body.String(), "marked") {
		t.Fatal("响应体不是 marked.js")
	}
}

func TestVendorETagAndCache(t *testing.T) {
	app, _ := newTestApp(t)

	first := serve(t, app, http.MethodGet, "/vendor/marked.umd.js", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("vendor 资源未返回 ETag")
	}
	// 嵌入资源内容固定，可安全长缓存；否则每次刷新都要重下 214KB。
	if cc := first.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("Cache-Control = %q, want 包含 immutable", cc)
	}

	second := serve(t, app, http.MethodGet, "/vendor/marked.umd.js", func(r *http.Request) {
		r.Header.Set("If-None-Match", etag)
	})
	if second.Code != http.StatusNotModified {
		t.Fatalf("条件请求 status = %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 响应不应包含 body，实际 %d 字节", second.Body.Len())
	}
}

func TestVendorUnknownPath(t *testing.T) {
	app, _ := newTestApp(t)
	for _, p := range []string{"/vendor/nope.js", "/vendor/", "/unknown.txt"} {
		rr := serve(t, app, http.MethodGet, p, nil)
		if rr.Code != http.StatusNotFound {
			t.Errorf("path=%q status = %d, want 404", p, rr.Code)
		}
	}
}

func TestCompressible(t *testing.T) {
	cases := []struct {
		contentType string
		want        bool
	}{
		{"text/html; charset=utf-8", true},
		{"text/css", true},
		{"application/javascript", true},
		{"application/json; charset=utf-8", true},
		{"application/xml", true},
		{"image/svg+xml", true},
		{"image/png", false},
		{"font/woff2", false},
		{"video/mp4", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := compressible(tc.contentType); got != tc.want {
			t.Errorf("compressible(%q) = %v, want %v", tc.contentType, got, tc.want)
		}
	}
}

func TestIndexNotCompressedWhenNoGzip(t *testing.T) {
	app, _ := newTestApp(t)

	rr := serve(t, app, http.MethodGet, "/", func(r *http.Request) {
		r.Header.Set("Accept-Encoding", "identity")
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if enc := rr.Header().Get("Content-Encoding"); enc != "" {
		t.Fatalf("Content-Encoding = %q, want 空", enc)
	}
	if !strings.Contains(rr.Body.String(), "<!DOCTYPE html>") {
		t.Fatal("首页内容异常")
	}
}
