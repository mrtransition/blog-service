package main

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"
)

// RSS 2.0 结构。使用 encoding/xml 自动转义，避免手写字符串拼接。
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string  `xml:"title"`
	Link        string  `xml:"link"`
	GUID        rssGUID `xml:"guid"`
	PubDate     string  `xml:"pubDate"`
	Description string  `xml:"description"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func (a *App) handleFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snap := a.currentSnapshot()
	baseURL := requestBaseURL(r)
	channel := rssChannel{
		Title:         sanitizeXMLText(a.cfg.BlogName),
		Link:          baseURL + "/",
		Description:   sanitizeXMLText(a.cfg.BlogName + " 的最新文章"),
		Language:      "zh-CN",
		LastBuildDate: snap.Generated.UTC().Format(time.RFC1123Z),
	}

	for _, article := range snap.Articles {
		if article.IsDraft {
			continue
		}
		link := articleURL(baseURL, article.RelPath)
		description := article.Summary
		if description == "" {
			description = article.Title
		}
		channel.Items = append(channel.Items, rssItem{
			Title:       sanitizeXMLText(article.Title),
			Link:        link,
			GUID:        rssGUID{IsPermaLink: "false", Value: link},
			PubDate:     article.Date.UTC().Format(time.RFC1123Z),
			Description: sanitizeXMLText(description),
		})
	}

	feed := rssFeed{Version: "2.0", Channel: channel}
	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		http.Error(w, "生成 feed 失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(xml.Header))
	w.Write(body)
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if first == "http" || first == "https" {
			scheme = first
		}
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

// articleURL 生成带 SEO 元信息的文章入口地址（/article/<路径>）。
// 该地址会返回 SPA 外壳并附带 canonical，浏览器随后规范化为 hash 路由。
func articleURL(baseURL, relPath string) string {
	return baseURL + "/article/" + escapeRelPath(relPath)
}

// sanitizeXMLText 移除 XML 1.0 不允许的控制字符，避免 Marshal 失败。
func sanitizeXMLText(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return r
		case r < 0x20:
			return -1
		default:
			return r
		}
	}, s)
}
