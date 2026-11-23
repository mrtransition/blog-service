package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, root, rel, content string) string {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return full
}

func TestResolveInsideRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "技术类/文章.md", "# 文章")
	outside := t.TempDir()
	outsideFile := writeTestFile(t, outside, "secret.txt", "secret")

	cases := []struct {
		name    string
		rel     string
		wantErr bool
	}{
		{"normal", "技术类/文章.md", false},
		{"leading slash", "/技术类/文章.md", false},
		{"parent escape", "../secret.txt", true},
		{"nested parent escape", "技术类/../../secret.txt", true},
		{"dot", ".", true},
		{"empty", "", true},
		{"absolute", outsideFile, true},
		{"windows separator escape", `..\..\secret.txt`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveInsideRoot(root, tc.rel)
			if tc.wantErr && err == nil {
				t.Fatalf("resolveInsideRoot(%q) = nil error, want error", tc.rel)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("resolveInsideRoot(%q) error = %v", tc.rel, err)
			}
		})
	}

	t.Run("symlink escape", func(t *testing.T) {
		link := filepath.Join(root, "escape.txt")
		if err := os.Symlink(outsideFile, link); err != nil {
			t.Skipf("当前环境无法创建符号链接: %v", err)
		}
		if _, err := resolveInsideRoot(root, "escape.txt"); err == nil {
			t.Fatal("指向根目录外的符号链接未被拦截")
		}
	})
}

func TestIsSafeRequestPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/blog-files/技术类/文章.md", true},
		{"/feed.xml", true},
		{"/blog-files/../secret.txt", false},
		{"/%2e%2e/%2e%2e/etc/passwd", false},
		{"/blog-files/%2E%2E/secret.txt", false},
		{"/blog-files/a%2e%2e/b", false},
		{"/blog-files/a%5cb", false},
		{"/blog-files/%00.txt", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if got := isSafeRequestPath(req.URL); got != tc.want {
			t.Errorf("isSafeRequestPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestSafePathMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := safePathMiddleware(next)

	blocked := []string{
		"/../etc/passwd",
		"/%2e%2e/%2e%2e/etc/passwd",
		"/blog-files/%2E%2E/secret.txt",
	}
	for _, p := range blocked {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, p, nil))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", p, rr.Code)
		}
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blog-files/技术类/文章.md", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("合法路径被拦截: status = %d", rr.Code)
	}
}

func TestServeBlogFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "技术类/文章.md", "# 标题")
	writeTestFile(t, root, "子目录/index.html", "<h1>hi</h1>")
	handler := serveBlogFile(root)

	t.Run("normal", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blog-files/技术类/文章.md", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "# 标题") {
			t.Fatalf("body = %q", rr.Body.String())
		}
	})

	t.Run("directory listing disabled", func(t *testing.T) {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blog-files/技术类/", nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("traversal", func(t *testing.T) {
		for _, target := range []string{"/blog-files/../secret.md", "/blog-files/%2e%2e/secret.md"} {
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, target, nil))
			if rr.Code < 400 || rr.Code >= 500 {
				t.Errorf("%s status = %d, want 4xx", target, rr.Code)
			}
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		outside := writeTestFile(t, t.TempDir(), "secret.txt", "secret")
		link := filepath.Join(root, "escape.txt")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("当前环境无法创建符号链接: %v", err)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/blog-files/escape.txt", nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}
