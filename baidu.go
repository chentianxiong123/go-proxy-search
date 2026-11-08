package search

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// Baidu 百度引擎（对标 SearXNG searx/engines/baidu.py）。
// 请求 www.baidu.com/s?wd= → data-log mu / baidu.com/link?url= / 直链 解析。
type Baidu struct{}

// NewBaidu 建百度引擎。
func NewBaidu() *Baidu { return &Baidu{} }

// Name 引擎名。
func (b *Baidu) Name() string { return "baidu" }

// SearchURL 百度搜索页 URL。
func (b *Baidu) SearchURL(query string) string {
	return "https://www.baidu.com/s?wd=" + url.QueryEscape(query)
}

// NeedCookie 百度需要 cookie（预热 image.baidu.com 过验证码）。
func (b *Baidu) NeedCookie() bool { return true }

// Parse 解析百度结果页 → 结果。
func (b *Baidu) Parse(htmlText string) []Result {
	return parseBaiduHTML(htmlText)
}

// baiduMu 解析 data-log JSON 的 mu 字段（真实 URL）。
func baiduMu(raw string) string {
	decoded := html.UnescapeString(raw)
	var obj struct {
		Mu string `json:"mu"`
	}
	if err := json.Unmarshal([]byte(decoded), &obj); err != nil {
		return ""
	}
	return obj.Mu
}

// parseBaiduHTML 解析百度 HTML → 结果（三规则：data-log mu / baidu.com/link / 直链）。
func parseBaiduHTML(htmlText string) []Result {
	var rs []Result
	// 规则1（旧版）：data-log='{"mu":"真实URL"}' 或 data-log="..."
	for _, re := range []*regexp.Regexp{reDataLog, reDataLog2} {
		for _, m := range re.FindAllStringSubmatch(htmlText, -1) {
			mu := baiduMu(m[1])
			if mu == "" || strings.HasPrefix(mu, "javascript") {
				continue
			}
			rs = append(rs, Result{Engine: "baidu", URL: mu})
		}
		if len(rs) > 0 {
			break
		}
	}
	// 规则2（新版 2026+）：结果链接全走 http(s)://www.baidu.com/link?url= 跳转
	if len(rs) == 0 {
		seen := map[string]bool{}
		for _, m := range reBaiduLink.FindAllStringSubmatch(htmlText, -1) {
			u := html.UnescapeString(m[1])
			key := u
			if i := strings.Index(u, "&"); i > 0 {
				key = u[:i]
			}
			if u == "" || seen[key] {
				continue
			}
			seen[key] = true
			rs = append(rs, Result{Engine: "baidu", URL: u})
		}
		// 规则3（兜底）：非 baidu 域的直链
		if len(rs) == 0 {
			for _, m := range reAnyHref.FindAllStringSubmatch(htmlText, -1) {
				u := html.UnescapeString(m[1])
				lu := strings.ToLower(u)
				if strings.Contains(lu, "baidu.com") || strings.Contains(lu, "bdstatic") ||
					strings.Contains(lu, "bcebos") || strings.Contains(lu, "hao123") {
					continue
				}
				if !strings.HasPrefix(lu, "http") || seen[u] {
					continue
				}
				seen[u] = true
				rs = append(rs, Result{Engine: "baidu", URL: u})
			}
		}
	}
	return rs
}

var (
	// 百度 data-log：优先单引号（移动版 data-log='{"mu":...}'，内容含双引号可匹配），
	// 兜底双引号（data-log="..."）。JSON 里含双引号，用 [^'x]* 允许引号内任意。
	reDataLog  = regexp.MustCompile(`data-log='([^']*)'`)
	reDataLog2 = regexp.MustCompile(`data-log="([^"]*)"`)
	// 百度新版（2026+）：结果链接全走 baidu.com/link?url= 跳转
	reBaiduLink = regexp.MustCompile(`href="(https?://(?:www\.)?baidu\.com/link\?url=[^"]+)"`)
	// 兜底：任意 <a href="http...">（新版百度真链，排除 baidu 域在代码里判）
	reAnyHref = regexp.MustCompile(`href="(https?://[^"]+)"`)
)
