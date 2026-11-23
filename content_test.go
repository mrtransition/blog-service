package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func findArticle(t *testing.T, snap *Snapshot, rel string) ArticleMeta {
	t.Helper()
	for _, article := range snap.Articles {
		if article.RelPath == rel {
			return article
		}
	}
	t.Fatalf("未找到文章 %q", rel)
	return ArticleMeta{}
}

func TestSplitFrontmatter(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "no frontmatter",
			content:   "# 标题\n正文",
			wantTitle: "",
			wantBody:  "# 标题\n正文",
		},
		{
			name:      "with frontmatter",
			content:   "---\ntitle: 我的文章\ndate: 2024-01-02\n---\n正文",
			wantTitle: "我的文章",
			wantBody:  "正文",
		},
		{
			name:      "quoted values",
			content:   "---\ntitle: \"带引号\"\n---\n正文",
			wantTitle: "带引号",
			wantBody:  "正文",
		},
		{
			name:      "crlf",
			content:   "---\r\ntitle: 换行\r\n---\r\n正文",
			wantTitle: "换行",
			wantBody:  "正文",
		},
		{
			name:      "unclosed frontmatter",
			content:   "---\ntitle: 未闭合\n正文",
			wantTitle: "",
			wantBody:  "---\ntitle: 未闭合\n正文",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, body := splitFrontmatter(tc.content)
			if fm["title"] != tc.wantTitle {
				t.Fatalf("title = %q, want %q", fm["title"], tc.wantTitle)
			}
			if body != tc.wantBody {
				t.Fatalf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

func TestParseArticleDate(t *testing.T) {
	cases := []struct {
		value string
		ok    bool
	}{
		{"2024-05-06", true},
		{"2024-05-06T07:08:09Z", true},
		{"2024-05-06 07:08:09", true},
		{"2024/05/06", true},
		{"", false},
		{"not-a-date", false},
	}
	for _, tc := range cases {
		_, ok := parseArticleDate(tc.value)
		if ok != tc.ok {
			t.Errorf("parseArticleDate(%q) ok = %v, want %v", tc.value, ok, tc.ok)
		}
	}
}

func TestScanBlog(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "关于.md", "# 关于\n\n这是介绍。")
	writeTestFile(t, root, "技术类/发布.md", `---
title: 已发布文章
date: 2024-05-06
---

正文内容，包含 **加粗** 与 [链接](https://example.com)。
`)
	writeTestFile(t, root, "技术类/_草稿.md", "# 文件名草稿")
	writeTestFile(t, root, "技术类/草稿二.md", `---
title: frontmatter 草稿
draft: true
---
正文`)
	writeTestFile(t, root, "笔记.txt", "ignored")
	writeTestFile(t, root, "页面/展示.html", "<h1>HTML 页面</h1>")
	if err := os.MkdirAll(filepath.Join(root, "空目录"), 0o755); err != nil {
		t.Fatal(err)
	}

	snap, err := scanBlog(root)
	if err != nil {
		t.Fatalf("scanBlog: %v", err)
	}

	if got := snap.PublishedCount(); got != 3 {
		t.Errorf("PublishedCount = %d, want 3", got)
	}
	if got := snap.DraftCount(); got != 2 {
		t.Errorf("DraftCount = %d, want 2", got)
	}

	published := findArticle(t, snap, "技术类/发布.md")
	if published.Title != "已发布文章" {
		t.Errorf("title = %q", published.Title)
	}
	if published.Category != "技术类" {
		t.Errorf("category = %q", published.Category)
	}
	if published.Date.Year() != 2024 || published.Date.Month() != time.May || published.Date.Day() != 6 {
		t.Errorf("date = %v", published.Date)
	}
	if !strings.Contains(published.Summary, "正文内容") || strings.Contains(published.Summary, "**") {
		t.Errorf("summary = %q", published.Summary)
	}

	about := findArticle(t, snap, "关于.md")
	if about.Title != "关于" {
		t.Errorf("heading title = %q", about.Title)
	}

	frontmatterDraft := findArticle(t, snap, "技术类/草稿二.md")
	if !frontmatterDraft.IsDraft {
		t.Error("frontmatter draft: true 未被识别")
	}

	if !treeHasFile(snap.Tree, "关于.md") ||
		!treeHasFile(snap.Tree, "技术类/发布.md") ||
		!treeHasFile(snap.Tree, "页面/展示.html") {
		t.Fatalf("正常文章缺失: %+v", snap.Tree)
	}
	if treeHasFile(snap.Tree, "技术类/_草稿.md") || treeHasFile(snap.Tree, "技术类/草稿二.md") {
		t.Fatal("草稿出现在导航树中")
	}
	if !treeHasDir(snap.Tree, "空目录") {
		t.Error("空目录应保留在导航树中")
	}
	if treeHasFile(snap.Tree, "笔记.txt") {
		t.Error("非 md/html 文件不应进入导航")
	}
}

func TestMakeSummary(t *testing.T) {
	body := "# 标题\n\n![图片](a.png) 一段 **文字**，[链接](https://x.y)。\n\n```go\nfmt.Println(\"code\")\n```\n\n<!-- 注释 -->\n结尾"
	got := makeSummary(body, 100)
	if strings.Contains(got, "```") || strings.Contains(got, "fmt.Println") {
		t.Fatalf("代码块未被剔除: %q", got)
	}
	if strings.Contains(got, "![") || strings.Contains(got, "](") || strings.Contains(got, "<!--") {
		t.Fatalf("Markdown 标记未被剔除: %q", got)
	}
	if !strings.Contains(got, "一段 文字") || !strings.Contains(got, "结尾") {
		t.Fatalf("summary = %q", got)
	}

	long := makeSummary(strings.Repeat("字", 300), 10)
	if len([]rune(long)) != 11 || !strings.HasSuffix(long, "…") {
		t.Fatalf("截断结果异常: %q", long)
	}
}

func TestFirstMarkdownHeading(t *testing.T) {
	body := "```\n# 代码里的标题\n```\n\n## 二级\n\n# 真正的一级标题\n"
	if got := firstMarkdownHeading(body); got != "真正的一级标题" {
		t.Fatalf("heading = %q", got)
	}
}

func treeHasDir(nodes []*TreeNode, target string) bool {
	for _, node := range nodes {
		if node.Type == "directory" && node.Path == target {
			return true
		}
		if treeHasDir(node.Children, target) {
			return true
		}
	}
	return false
}
