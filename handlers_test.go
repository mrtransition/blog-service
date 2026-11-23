package main

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func newTestApp(t *testing.T) (*App, string) {
	t.Helper()
	root := t.TempDir()
	app, err := newApp(root, Config{BlogName: "测试博客"})
	if err != nil {
		t.Fatalf("newApp: %v", err)
	}
	return app, root
}

func mustReload(t *testing.T, app *App) {
	t.Helper()
	if _, err := app.reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
}

func serve(t *testing.T, app *App, method, target string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	if mutate != nil {
		mutate(req)
	}
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	return rr
}

func treeHasFile(nodes []*TreeNode, target string) bool {
	for _, node := range nodes {
		if node.Type == "file" && node.Path == target {
			return true
		}
		if treeHasFile(node.Children, target) {
			return true
		}
	}
	return false
}

func TestIndexETag(t *testing.T) {
	app, _ := newTestApp(t)

	first := serve(t, app, http.MethodGet, "/", nil)
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", first.Code)
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("首次请求未返回 ETag")
	}
	if cc := first.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", cc)
	}

	second := serve(t, app, http.MethodGet, "/", func(r *http.Request) {
		r.Header.Set("If-None-Match", etag)
	})
	if second.Code != http.StatusNotModified {
		t.Fatalf("二次请求 status = %d, want 304", second.Code)
	}
	if second.Body.Len() != 0 {
		t.Fatalf("304 响应不应包含 body，实际 %d 字节", second.Body.Len())
	}
}

func TestEtagMatches(t *testing.T) {
	cases := []struct {
		header string
		want   bool
	}{
		{`"abc"`, true},
		{`W/"abc"`, true},
		{`"x", "abc"`, true},
		{`*`, true},
		{`"other"`, false},
		{``, false},
	}
	for _, tc := range cases {
		if got := etagMatches(tc.header, `"abc"`); got != tc.want {
			t.Errorf("etagMatches(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestAPIFile(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/文章.md", "# 标题\n\n正文")
	writeTestFile(t, root, "_draft.md", "# 草稿")
	outside := writeTestFile(t, t.TempDir(), "secret.md", "secret")
	mustReload(t, app)

	t.Run("normal", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/file?path="+url.QueryEscape("技术类/文章.md"), nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var body map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("json: %v", err)
		}
		if body["content"] != "# 标题\n\n正文" {
			t.Fatalf("content = %q", body["content"])
		}
		if body["fileType"] != "markdown" || body["dirPath"] != "技术类" {
			t.Fatalf("body = %#v", body)
		}
	})

	t.Run("draft direct access", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/file?path=_draft.md", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("草稿应可通过 URL 直接访问，status = %d", rr.Code)
		}
	})

	t.Run("query traversal", func(t *testing.T) {
		for _, p := range []string{"../../etc/passwd", `..\..\etc\passwd`, "%2e%2e%2f%2e%2e%2fetc%2fpasswd"} {
			rr := serve(t, app, http.MethodGet, "/api/file?path="+url.QueryEscape(p), nil)
			if rr.Code < 400 || rr.Code >= 500 {
				t.Errorf("path=%q status = %d, want 4xx", p, rr.Code)
			}
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/file?path="+url.QueryEscape(outside), nil)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rr.Code)
		}
	})

	t.Run("non article extension", func(t *testing.T) {
		writeTestFile(t, root, "note.txt", "hello")
		rr := serve(t, app, http.MethodGet, "/api/file?path=note.txt", nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/file", nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

func TestAPITreeFiltersDrafts(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "关于.md", "# 关于")
	writeTestFile(t, root, "_draft-root.md", "# 草稿")
	writeTestFile(t, root, "技术类/文章.md", "# 文章")
	writeTestFile(t, root, "技术类/_draft-child.md", "# 子草稿")
	mustReload(t, app)

	rr := serve(t, app, http.MethodGet, "/api/tree", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Children []*TreeNode `json:"children"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !treeHasFile(body.Children, "关于.md") || !treeHasFile(body.Children, "技术类/文章.md") {
		t.Fatalf("正常文章缺失: %s", rr.Body.String())
	}
	if treeHasFile(body.Children, "_draft-root.md") || treeHasFile(body.Children, "技术类/_draft-child.md") {
		t.Fatalf("草稿出现在导航中: %s", rr.Body.String())
	}
}

func TestFeed(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/发布.md", `---
title: 已发布文章
date: 2024-05-06
---

正文内容，包含 **加粗** 与 [链接](https://example.com)。
`)
	writeTestFile(t, root, "_草稿.md", `---
title: 草稿文章
date: 2025-01-01
---

不应出现在 feed 中。
`)
	mustReload(t, app)

	rr := serve(t, app, http.MethodGet, "/feed.xml", func(r *http.Request) {
		r.Host = "example.com"
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/rss+xml") {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "已发布文章") {
		t.Fatalf("feed 缺少已发布文章: %s", body)
	}
	if strings.Contains(body, "草稿文章") {
		t.Fatalf("草稿出现在 feed 中: %s", body)
	}

	var feed rssFeed
	if err := xml.Unmarshal(rr.Body.Bytes(), &feed); err != nil {
		t.Fatalf("feed 不是合法 XML: %v", err)
	}
	if feed.Version != "2.0" || feed.Channel.Title != "测试博客" {
		t.Fatalf("feed 头信息异常: %#v", feed.Channel)
	}
	if len(feed.Channel.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(feed.Channel.Items))
	}
	item := feed.Channel.Items[0]
	if !strings.Contains(item.Link, "http://example.com/article/") {
		t.Fatalf("link = %q", item.Link)
	}
	if item.PubDate != "Mon, 06 May 2024 00:00:00 +0000" {
		t.Fatalf("pubDate = %q", item.PubDate)
	}
}

func TestReload(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "旧文章.md", "# 旧")
	mustReload(t, app)

	rr := serve(t, app, http.MethodGet, "/api/tree", nil)
	if !strings.Contains(rr.Body.String(), "旧文章.md") {
		t.Fatalf("初始快照缺少文章: %s", rr.Body.String())
	}

	// 非回环地址必须被拒绝
	rr = serve(t, app, http.MethodPost, "/reload", func(r *http.Request) {
		r.RemoteAddr = "192.0.2.10:1234"
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("非回环 reload status = %d, want 403", rr.Code)
	}

	// 只允许 POST
	rr = serve(t, app, http.MethodGet, "/reload", func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:1234"
	})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET reload status = %d, want 405", rr.Code)
	}

	writeTestFile(t, root, "新文章.md", "# 新")
	rr = serve(t, app, http.MethodPost, "/reload", func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:1234"
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("reload status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	rr = serve(t, app, http.MethodGet, "/api/tree", nil)
	if !strings.Contains(rr.Body.String(), "新文章.md") {
		t.Fatalf("reload 后新文章未出现在导航: %s", rr.Body.String())
	}
}

func TestTraversalAtMuxLevel(t *testing.T) {
	app, _ := newTestApp(t)
	for _, target := range []string{
		"/../etc/passwd",
		"/%2e%2e/%2e%2e/etc/passwd",
		"/blog-files/%2e%2e/secret.txt",
	} {
		rr := serve(t, app, http.MethodGet, target, nil)
		if rr.Code < 400 || rr.Code >= 500 {
			t.Errorf("%s status = %d, want 4xx", target, rr.Code)
		}
	}
}

func TestArticleURL(t *testing.T) {
	got := articleURL("http://example.com", "技术类/我的 文章.md")
	want := "http://example.com/article/%E6%8A%80%E6%9C%AF%E7%B1%BB/%E6%88%91%E7%9A%84%20%E6%96%87%E7%AB%A0.md"
	if got != want {
		t.Fatalf("articleURL = %q, want %q", got, want)
	}
}

func TestArticleSEOPage(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/发布.md", `---
title: SEO 文章
date: 2024-05-06
---

正文摘要。
`)
	writeTestFile(t, root, "_草稿.md", `---
title: 秘密草稿
date: 2025-01-01
---

不应被索引。
`)
	mustReload(t, app)

	target := "/article/" + escapeRelPath("技术类/发布.md")
	rr := serve(t, app, http.MethodGet, target, func(r *http.Request) {
		r.Host = "example.com"
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q", ct)
	}
	body := rr.Body.String()
	canonical := `http://example.com/article/%E6%8A%80%E6%9C%AF%E7%B1%BB/%E5%8F%91%E5%B8%83.md`
	for _, want := range []string{
		`<title>SEO 文章 - 测试博客</title>`,
		`rel="canonical" href="` + canonical + `"`,
		`property="og:title" content="SEO 文章"`,
		`property="og:url" content="` + canonical + `"`,
		`property="article:published_time" content="2024-05-06T00:00:00Z"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("文章页缺少 %q", want)
		}
	}

	draft := serve(t, app, http.MethodGet, "/article/"+escapeRelPath("_草稿.md"), nil)
	if draft.Code != http.StatusNotFound {
		t.Fatalf("草稿入口 status = %d, want 404", draft.Code)
	}

	missing := serve(t, app, http.MethodGet, "/article/不存在.md", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("缺失文章 status = %d, want 404", missing.Code)
	}
}

func TestSitemap(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "技术类/发布.md", `---
title: 已发布
date: 2024-05-06
---
正文
`)
	writeTestFile(t, root, "_草稿.md", `---
title: 草稿
date: 2025-01-01
---
草稿
`)
	mustReload(t, app)

	rr := serve(t, app, http.MethodGet, "/sitemap.xml", func(r *http.Request) {
		r.Host = "example.com"
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Fatalf("Content-Type = %q", ct)
	}

	var set sitemapURLSet
	if err := xml.Unmarshal(rr.Body.Bytes(), &set); err != nil {
		t.Fatalf("sitemap 不是合法 XML: %v\n%s", err, rr.Body.String())
	}
	if len(set.URLs) != 1 {
		t.Fatalf("urls = %d, want 1: %s", len(set.URLs), rr.Body.String())
	}
	want := "http://example.com/article/%E6%8A%80%E6%9C%AF%E7%B1%BB/%E5%8F%91%E5%B8%83.md"
	if set.URLs[0].Loc != want {
		t.Fatalf("loc = %q, want %q", set.URLs[0].Loc, want)
	}
	if set.URLs[0].LastMod != "2024-05-06" {
		t.Fatalf("lastmod = %q", set.URLs[0].LastMod)
	}
	if strings.Contains(rr.Body.String(), "%E8%8D%89%E7%A8%BF") || strings.Contains(rr.Body.String(), "_草稿") {
		t.Fatalf("草稿出现在 sitemap 中: %s", rr.Body.String())
	}
}

func TestReloadToken(t *testing.T) {
	app, root := newTestApp(t)
	app.cfg.ReloadToken = "s3cret"
	writeTestFile(t, root, "文章.md", "# 文章")
	mustReload(t, app)

	cases := []struct {
		name   string
		remote string
		mutate func(*http.Request)
		want   int
	}{
		{"loopback no token", "127.0.0.1:1234", nil, http.StatusOK},
		{"remote no token", "192.0.2.10:1234", nil, http.StatusForbidden},
		{"remote wrong token", "192.0.2.10:1234", func(r *http.Request) {
			r.Header.Set("X-Reload-Token", "nope")
		}, http.StatusForbidden},
		{"remote header token", "192.0.2.10:1234", func(r *http.Request) {
			r.Header.Set("X-Reload-Token", "s3cret")
		}, http.StatusOK},
		{"remote bearer token", "192.0.2.10:1234", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer s3cret")
		}, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := serve(t, app, http.MethodPost, "/reload", func(r *http.Request) {
				r.RemoteAddr = tc.remote
				if tc.mutate != nil {
					tc.mutate(r)
				}
			})
			if rr.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tc.want, rr.Body.String())
			}
		})
	}

	app.cfg.ReloadToken = ""
	rr := serve(t, app, http.MethodPost, "/reload", func(r *http.Request) {
		r.RemoteAddr = "192.0.2.10:1234"
		r.Header.Set("X-Reload-Token", "anything")
	})
	if rr.Code != http.StatusForbidden {
		t.Fatalf("未配置 token 时 status = %d, want 403", rr.Code)
	}
}

func TestBasicAuth(t *testing.T) {
	app, root := newTestApp(t)
	writeTestFile(t, root, "文章.md", "# 文章")
	app.cfg.Bind = "0.0.0.0"
	app.cfg.AuthUser = "alice"
	app.cfg.AuthPass = "wonderland"
	mustReload(t, app)

	t.Run("missing credentials", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/config", nil)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
		if got := rr.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, "Basic ") {
			t.Fatalf("WWW-Authenticate = %q", got)
		}
	})

	t.Run("wrong credentials", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/config", func(r *http.Request) {
			r.SetBasicAuth("alice", "wrong")
		})
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rr.Code)
		}
	})

	t.Run("correct credentials", func(t *testing.T) {
		rr := serve(t, app, http.MethodGet, "/api/config", func(r *http.Request) {
			r.SetBasicAuth("alice", "wonderland")
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("reload exempt from basic auth", func(t *testing.T) {
		rr := serve(t, app, http.MethodPost, "/reload", func(r *http.Request) {
			r.RemoteAddr = "127.0.0.1:1234"
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("本机 reload status = %d, want 200: %s", rr.Code, rr.Body.String())
		}
		rr = serve(t, app, http.MethodPost, "/reload", func(r *http.Request) {
			r.RemoteAddr = "192.0.2.10:1234"
		})
		if rr.Code != http.StatusForbidden {
			t.Fatalf("外部 reload status = %d, want 403", rr.Code)
		}
	})

	t.Run("disabled on loopback bind", func(t *testing.T) {
		app.cfg.Bind = "127.0.0.1"
		rr := serve(t, app, http.MethodGet, "/api/config", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
	})
}

func TestIsLoopbackBind(t *testing.T) {
	cases := []struct {
		bind string
		want bool
	}{
		{"", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"[::1]", true},
		{"localhost", true},
		{"0.0.0.0", false},
		{"192.168.1.10", false},
		{"::", false},
	}
	for _, tc := range cases {
		if got := isLoopbackBind(tc.bind); got != tc.want {
			t.Errorf("isLoopbackBind(%q) = %v, want %v", tc.bind, got, tc.want)
		}
	}
}

func TestIsLoopbackRequest(t *testing.T) {
	cases := []struct {
		remote string
		want   bool
	}{
		{"127.0.0.1:1234", true},
		{"[::1]:1234", true},
		{"192.0.2.1:1234", false},
		{"", false},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/reload", nil)
		req.RemoteAddr = tc.remote
		if got := isLoopbackRequest(req); got != tc.want {
			t.Errorf("isLoopbackRequest(%q) = %v, want %v", tc.remote, got, tc.want)
		}
	}
}

func TestRootIsResolved(t *testing.T) {
	app, root := newTestApp(t)
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if app.root != want {
		t.Fatalf("root = %q, want %q", app.root, want)
	}
}
