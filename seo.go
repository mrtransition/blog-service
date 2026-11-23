package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// handleArticle 为搜索引擎与外部链接提供带 og/canonical 的文章入口页。
// 页面本身仍是 SPA 外壳：注入的脚本会把地址规范化为 /#/<文章路径>，
// 随后由前端按 hash 加载正文；草稿不提供该入口，避免被索引。
func (a *App) handleArticle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/article/")
	rel = filepath.ToSlash(strings.TrimLeft(filepath.FromSlash(rel), `/\`))
	ext := strings.ToLower(path.Ext(rel))
	if ext != ".md" && ext != ".html" {
		http.NotFound(w, r)
		return
	}

	snap := a.currentSnapshot()
	var meta *ArticleMeta
	for i := range snap.Articles {
		if snap.Articles[i].RelPath == rel {
			meta = &snap.Articles[i]
			break
		}
	}
	if meta == nil || meta.IsDraft {
		http.NotFound(w, r)
		return
	}
	// 快照可能落后于磁盘，额外确认文件仍存在且在根目录内。
	if _, err := resolveInsideRoot(a.root, rel); err != nil {
		http.NotFound(w, r)
		return
	}

	canonical := requestBaseURL(r) + "/article/" + escapeRelPath(rel)
	page := a.renderArticleIndex(*meta, canonical)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(page)
}

func (a *App) renderArticleIndex(meta ArticleMeta, canonical string) []byte {
	title := meta.Title + " - " + a.cfg.BlogName
	description := meta.Summary
	if description == "" {
		description = meta.Title
	}

	escTitle := html.EscapeString(title)
	escDesc := html.EscapeString(description)
	escCanonical := html.EscapeString(canonical)
	escSite := html.EscapeString(a.cfg.BlogName)
	published := meta.Date.UTC().Format(time.RFC3339)

	metaTags := fmt.Sprintf(`
    <meta name="description" content="%s">
    <link rel="canonical" href="%s">
    <meta property="og:type" content="article">
    <meta property="og:title" content="%s">
    <meta property="og:description" content="%s">
    <meta property="og:url" content="%s">
    <meta property="og:site_name" content="%s">
    <meta property="article:published_time" content="%s">
`, escDesc, escCanonical, html.EscapeString(meta.Title), escDesc, escCanonical, escSite, published)

	// JSON 字符串字面量同时是合法的 JS 字符串字面量，且会转义 <、>、&。
	relJSON, _ := json.Marshal(meta.RelPath)
	initScript := fmt.Sprintf(`    <script>(function(){var p=%s;try{history.replaceState(null,'','/#/'+encodeURIComponent(p));}catch(e){location.hash='#/'+encodeURIComponent(p);}})();</script>
`, relJSON)

	page := string(a.indexHTML)
	page = strings.Replace(page, "<title>轻舟博客</title>", "<title>"+escTitle+"</title>", 1)
	if strings.Contains(page, "<!-- SEO_META -->") {
		page = strings.Replace(page, "<!-- SEO_META -->", metaTags+initScript, 1)
	} else {
		page = strings.Replace(page, "</head>", metaTags+initScript+"</head>", 1)
	}
	return []byte(page)
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (a *App) handleSitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snap := a.currentSnapshot()
	baseURL := requestBaseURL(r)
	set := sitemapURLSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, article := range snap.Articles {
		if article.IsDraft {
			continue
		}
		set.URLs = append(set.URLs, sitemapURL{
			Loc:     baseURL + "/article/" + escapeRelPath(article.RelPath),
			LastMod: article.Date.UTC().Format("2006-01-02"),
		})
	}

	body, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		http.Error(w, "生成 sitemap 失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(xml.Header))
	w.Write(body)
}

// escapeRelPath 按路径段编码文章相对路径，保留 / 作为分隔符。
func escapeRelPath(rel string) string {
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
