package search

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
)

// hasClass 判断节点是否含指定 class。
func hasClass(n *xhtml.Node, cls string) bool {
	for _, at := range n.Attr {
		if at.Key == "class" {
			for _, c := range strings.Fields(at.Val) {
				if c == cls {
					return true
				}
			}
		}
	}
	return false
}

// textOf 提取节点纯文本。
func textOf(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(m *xhtml.Node) {
		if m.Type == xhtml.TextNode {
			b.WriteString(m.Data)
		}
		for c := m.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(b.String())
}

// walkNodes 深度遍历，回调每个节点（含自身）。
func walkNodes(n *xhtml.Node, fn func(*xhtml.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNodes(c, fn)
	}
}

// readBody 读响应体（限制 4MB 防超大，精确不超读）。
func readBody(resp *http.Response) string {
	const max = 4 << 20
	buf := make([]byte, 0, 1<<20)
	tmp := make([]byte, 64<<10)
	for len(buf) < max {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			if len(buf)+n > max {
				n = max - len(buf) // 截到 4MB，不超读
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}

// isCaptcha 验证码/安全验证检测：命中返回 true（应换引擎）。
func isCaptcha(body string) bool {
	low := strings.ToLower(body)
	for _, k := range []string{
		"captcha/tuxing", "百度安全验证", "请输入验证码",
		"访问验证", "验证码验证", "百度智能云安全",
	} {
		if strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	// wappass 跳转只在指向 /verify 或 /captcha 时才是真验证码
	if strings.Contains(low, "wappass") &&
		(strings.Contains(low, "/verify") || strings.Contains(low, "/captcha")) {
		return true
	}
	return false
}

// IsLegitSite 已知正版平台域名（导出，供过滤）。
func IsLegitSite(rawURL string) bool { return isLegitSite(rawURL) }

// isLegitSite 已知正版平台域名。
func isLegitSite(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(u.Path, "/link") && strings.Contains(u.RawQuery, "url=") &&
		(host == "baidu.com" || strings.HasSuffix(host, ".baidu.com")) {
		return false
	}
	for _, d := range []string{
		"bilibili.com", "b23.tv", "youku.com", "iqiyi.com", "qiyi.com",
		"qq.com", "v.qq.com", "mgtv.com", "sohu.com", "douyin.com",
		"miguvideo.com", "1905.com", "cctv.com", "xigua.com", "baidu.com",
		"zhihu.com", "weibo.com", "douban.com", "sogou.com", "so.com",
	} {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// applyBrowserHeaders 补全浏览器头（模拟 Chrome，防搜索引擎识别为爬虫）。
func applyBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
}

// get 公共 HTTP GET（带浏览器头 + Referer + cookie）。
func get(ctx context.Context, rawURL, referer, cookie string, timeout time.Duration) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	applyBrowserHeaders(req)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{Code: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// HTTPError 带状态码的 HTTP 错误。
type HTTPError struct{ Code int }

func (e *HTTPError) Error() string {
	return "HTTP " + http.StatusText(e.Code)
}
