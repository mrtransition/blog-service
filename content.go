package main

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ArticleMeta 是文章元信息的只读快照，启动时扫描一次，导航、列表和 feed 共用。
type ArticleMeta struct {
	RelPath  string    // 相对博客根目录的斜杠路径
	Title    string    // frontmatter title / 一级标题 / 文件名
	Date     time.Time // frontmatter date，缺失时回退文件 mtime
	Category string    // 父目录（相对路径），根目录为空
	IsDraft  bool      // 文件名以 _ 开头，或 frontmatter 标记 draft: true
	ModTime  time.Time
	Summary  string
}

// Snapshot 是不可变快照，整体通过 atomic.Pointer 原子替换，读请求无需加锁。
type Snapshot struct {
	Articles  []ArticleMeta
	Tree      []*TreeNode
	Generated time.Time
}

func (s *Snapshot) PublishedCount() int {
	n := 0
	for _, a := range s.Articles {
		if !a.IsDraft {
			n++
		}
	}
	return n
}

func (s *Snapshot) DraftCount() int {
	return len(s.Articles) - s.PublishedCount()
}

// scanBlog 一次性 WalkDir 扫描博客目录，构建元信息快照与导航树。
// 不跟随符号链接，草稿文件不进入导航树。
func scanBlog(root string) (*Snapshot, error) {
	dirs := make(map[string][]string)       // 父目录 -> 子目录名
	files := make(map[string][]ArticleMeta) // 父目录 -> 文章元信息

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		parent := path.Dir(relSlash)
		if parent == "." {
			parent = ""
		}

		if d.IsDir() {
			dirs[parent] = append(dirs[parent], d.Name())
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		ext := strings.ToLower(path.Ext(relSlash))
		if ext != ".md" && ext != ".html" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files[parent] = append(files[parent], readArticleMeta(p, relSlash, info.ModTime()))
		return nil
	})
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{
		Generated: time.Now(),
		Tree:      buildTreeFromScan(dirs, files),
	}
	for _, group := range files {
		snap.Articles = append(snap.Articles, group...)
	}
	sort.SliceStable(snap.Articles, func(i, j int) bool {
		if !snap.Articles[i].Date.Equal(snap.Articles[j].Date) {
			return snap.Articles[i].Date.After(snap.Articles[j].Date)
		}
		return snap.Articles[i].RelPath < snap.Articles[j].RelPath
	})
	return snap, nil
}

// buildTreeFromScan 按“目录在前、文件在后”的既有顺序生成导航树。
func buildTreeFromScan(dirs map[string][]string, files map[string][]ArticleMeta) []*TreeNode {
	var build func(rel string) []*TreeNode
	build = func(rel string) []*TreeNode {
		var nodes []*TreeNode
		for _, name := range dirs[rel] {
			childRel := path.Join(rel, name)
			nodes = append(nodes, &TreeNode{
				Name:     name,
				Type:     "directory",
				Path:     childRel,
				Children: build(childRel),
			})
		}
		for _, article := range files[rel] {
			if article.IsDraft {
				continue
			}
			dirPath := rel
			if dirPath == "" {
				dirPath = "."
			}
			fileType := "markdown"
			if strings.EqualFold(path.Ext(article.RelPath), ".html") {
				fileType = "html"
			}
			nodes = append(nodes, &TreeNode{
				Name:     path.Base(article.RelPath),
				Type:     "file",
				Path:     article.RelPath,
				DirPath:  dirPath,
				FileType: fileType,
			})
		}
		return nodes
	}
	return build("")
}

func readArticleMeta(fullPath, relPath string, modTime time.Time) ArticleMeta {
	base := path.Base(relPath)
	meta := ArticleMeta{
		RelPath:  relPath,
		Title:    strings.TrimSuffix(base, path.Ext(base)),
		Category: path.Dir(relPath),
		IsDraft:  strings.HasPrefix(base, "_"),
		ModTime:  modTime,
		Date:     modTime,
	}
	if meta.Category == "." {
		meta.Category = ""
	}

	head := readHead(fullPath, 128<<10)
	fm, body := splitFrontmatter(head)
	if title := strings.TrimSpace(fm["title"]); title != "" {
		meta.Title = title
	} else if strings.EqualFold(path.Ext(relPath), ".md") {
		if heading := firstMarkdownHeading(body); heading != "" {
			meta.Title = heading
		}
	}
	if d, ok := parseArticleDate(fm["date"]); ok {
		meta.Date = d
	}
	if draft, ok := parseArticleBool(fm["draft"]); ok && draft {
		meta.IsDraft = true
	}
	meta.Summary = makeSummary(body, 160)
	return meta
}

func readHead(fullPath string, limit int64) string {
	f, err := os.Open(fullPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, limit)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return ""
	}
	// 截断可能落在多字节字符中间，用 ToValidUTF8 丢弃残片。
	return strings.ToValidUTF8(string(buf[:n]), "")
}

// splitFrontmatter 解析 --- 包围的简单 key: value 前置元数据。
// 仅支持单层键值，不支持嵌套 YAML；解析失败时按无 frontmatter 处理。
func splitFrontmatter(content string) (map[string]string, string) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]string{}, content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return parseFrontmatterValues(lines[1:i]), strings.Join(lines[i+1:], "\n")
		}
	}
	return map[string]string{}, content
}

func parseFrontmatterValues(lines []string) map[string]string {
	values := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		values[key] = value
	}
	return values
}

var articleDateLayouts = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006/01/02",
}

func parseArticleDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range articleDateLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func parseArticleBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "on", "1":
		return true, true
	case "false", "no", "off", "0":
		return false, true
	}
	return false, false
}

func firstMarkdownHeading(body string) string {
	lines := strings.Split(body, "\n")
	if len(lines) > 100 {
		lines = lines[:100]
	}
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

var (
	mdImageRE = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	mdLinkRE  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTagRE = regexp.MustCompile(`<[^>]*>`)
)

// makeSummary 从 Markdown/HTML 正文中提取纯文本摘要，供 feed 使用。
func makeSummary(body string, maxRunes int) string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" || strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		trimmed = strings.TrimLeft(trimmed, "#>-*+ \t")
		trimmed = mdImageRE.ReplaceAllString(trimmed, "$1")
		trimmed = mdLinkRE.ReplaceAllString(trimmed, "$1")
		trimmed = htmlTagRE.ReplaceAllString(trimmed, "")
		trimmed = strings.ReplaceAll(trimmed, "`", "")
		trimmed = strings.ReplaceAll(trimmed, "**", "")
		trimmed = strings.ReplaceAll(trimmed, "__", "")
		if trimmed == "" {
			continue
		}
		b.WriteString(trimmed)
		b.WriteString(" ")
	}

	text := strings.Join(strings.Fields(b.String()), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return text
}
