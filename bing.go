package search

import (
	"encoding/base64"
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
)

// Bing 引擎（对标 SearXNG searx/engines/bing.py）。
// 请求 www.bing.com/search → li.b_algo/h2/a → title + href（ck/a base64url 解码）。
type Bing struct{}

// NewBing 建 Bing 引擎。
func NewBing() *Bing { return &Bing{} }

// Name 引擎名。
func (b *Bing) Name() string { return "bing" }

// SearchURL Bing 搜索页 URL。
func (b *Bing) SearchURL(query string) string {
	return "https://www.bing.com/search?q=" + url.QueryEscape(query)
}

// NeedCookie Bing 不需要 cookie。
func (b *Bing) NeedCookie() bool { return false }

// Parse 解析 Bing 结果页 → 结果。
func (b *Bing) Parse(htmlText string) []Result {
	return parseBingHTML(htmlText)
}

// parseBingHTML 解析 Bing HTML（li.b_algo/h2/a → title+href）。
func parseBingHTML(htmlText string) []Result {
	doc, err := xhtml.Parse(strings.NewReader(htmlText))
	if err != nil {
		return nil
	}
	var rs []Result
	walkNodes(doc, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || n.Data != "li" || !hasClass(n, "b_algo") {
			return
		}
		title, href := "", ""
		walkNodes(n, func(m *xhtml.Node) {
			if m.Type == xhtml.ElementNode && m.Data == "a" && href == "" {
				for _, at := range m.Attr {
					if at.Key == "href" {
						href = at.Val
					}
				}
				title = strings.TrimSpace(textOf(m))
			}
		})
		href = bingRealURL(href)
		title = cleanTitle(title)
		if title != "" && href != "" {
			rs = append(rs, Result{Engine: "bing", Title: title, URL: href})
		}
	})
	return rs
}

// bingRealURL Bing 跳转链接 base64url 解码：/ck/a?u=a1<base64url> → 真实 URL。
func bingRealURL(href string) string {
	if !strings.Contains(href, "/ck/a?") {
		return href
	}
	qs, err := url.Parse(href)
	if err != nil {
		return href
	}
	u := qs.Query().Get("u")
	if !strings.HasPrefix(u, "a1") {
		return href
	}
	enc := u[2:]
	// base64 填充：保证 0-3（Go % 对负数返回负，会 panic strings.Repeat 负数）。
	enc += strings.Repeat("=", (4-len(enc)%4)%4)
	b, err := base64.URLEncoding.DecodeString(enc)
	if err != nil {
		return href
	}
	return string(b)
}

// cleanTitle 清理标题：去 URL 混入（bing 卡片可能把域名/URL 混进文本）。
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "http") {
		if i := strings.Index(s, "http"); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	return s
}
