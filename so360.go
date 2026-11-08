package search

import (
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"
)

// So360 360 搜索引擎（对标 SearXNG searx/engines/360search.py）。
// 请求 www.so.com/s（带 cookie）→ li.res-list/h3.res-title/a → title + data-mdurl。
type So360 struct{}

// NewSo360 建 360 引擎。
func NewSo360() *So360 { return &So360{} }

// Name 引擎名。
func (s *So360) Name() string { return "so" }

// SearchURL 360 搜索页 URL。
func (s *So360) SearchURL(query string) string {
	return "https://www.so.com/s?q=" + url.QueryEscape(query)
}

// NeedCookie 360 需要 cookie（预热拿 set-cookie，缓存由调用方/渲染器管）。
func (s *So360) NeedCookie() bool { return true }

// Parse 解析 360 结果页 → 结果。
func (s *So360) Parse(htmlText string) []Result {
	return parseSoHTML(htmlText)
}

// parseSoHTML 解析 360 HTML（li.res-list/h3.res-title/a → title+data-mdurl）。
func parseSoHTML(htmlText string) []Result {
	doc, err := xhtml.Parse(strings.NewReader(htmlText))
	if err != nil {
		return nil
	}
	var rs []Result
	walkNodes(doc, func(n *xhtml.Node) {
		if n.Type != xhtml.ElementNode || n.Data != "li" || !hasClass(n, "res-list") {
			return
		}
		title, urlStr := "", ""
		// 标题：h3.res-title 内 a 文本 + data-mdurl（兜底 href）
		walkNodes(n, func(m *xhtml.Node) {
			if m.Type == xhtml.ElementNode && m.Data == "a" && urlStr == "" {
				title = strings.TrimSpace(textOf(m))
				for _, at := range m.Attr {
					if at.Key == "data-mdurl" {
						urlStr = at.Val
					}
				}
				if urlStr == "" {
					for _, at := range m.Attr {
						if at.Key == "href" {
							urlStr = at.Val
						}
					}
				}
			}
		})
		if title != "" && urlStr != "" {
			rs = append(rs, Result{Engine: "so", Title: title, URL: urlStr})
		}
	})
	return rs
}
