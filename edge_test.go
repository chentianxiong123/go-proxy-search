package search

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	xhtml "golang.org/x/net/html"
)

// ── NeedCookie（各引擎）──
func TestEnginesNeedCookie(t *testing.T) {
	if NewBing().NeedCookie() {
		t.Fatal("bing 不需要 cookie")
	}
	if !NewSo360().NeedCookie() {
		t.Fatal("360 需要 cookie")
	}
	if !NewBaidu().NeedCookie() {
		t.Fatal("baidu 需要 cookie")
	}
}

// ── readBody（读响应体 + 4MB 上限）──
func TestReadBody(t *testing.T) {
	// 正常
	body := io.NopCloser(strings.NewReader("hello world"))
	got := readBody(&http.Response{Body: body})
	if got != "hello world" {
		t.Fatalf("readBody 应读全文, got %q", got)
	}
	// 超大（>4MB）截断不崩
	big := io.NopCloser(bytes.NewReader(make([]byte, 5<<20)))
	got2 := readBody(&http.Response{Body: big})
	if len(got2) > 4<<20 {
		t.Fatalf("readBody 应限 4MB, got %d", len(got2))
	}
}

// ── get（200 + 非 200 + 带 cookie/referer）──
func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头透传
		if r.Header.Get("Referer") != "https://ref.com" {
			t.Errorf("Referer 未透传")
		}
		if r.Header.Get("Cookie") != "k=v" {
			t.Errorf("Cookie 未透传")
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()
	body, err := get(context.Background(), srv.URL, "https://ref.com", "k=v", 3*time.Second)
	if err != nil {
		t.Fatalf("get err: %v", err)
	}
	if !strings.Contains(body, "ok") {
		t.Fatalf("get body=%q", body)
	}
}

func TestGetNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()
	_, err := get(context.Background(), srv.URL, "", "", 3*time.Second)
	if err == nil {
		t.Fatal("非 200 应报错")
	}
}

// ── bingRealURL / cleanTitle 边界 ──
func TestBingRealURLEdges(t *testing.T) {
	// 非 ck/a 原样返回
	if u := bingRealURL("https://direct.com/d.html"); u != "https://direct.com/d.html" {
		t.Fatalf("非 ck/a 应原样: %s", u)
	}
	// 非 a1 前缀
	if u := bingRealURL("/ck/a?u=xyz"); u != "/ck/a?u=xyz" {
		t.Fatalf("非 a1 应原样: %s", u)
	}
	// 解码真 URL
	u := bingRealURL("/ck/a?u=a1aHR0cHM6Ly9leGFtcGxlLmNvbS9kLmh0bWw")
	if !strings.Contains(u, "example.com") {
		t.Fatalf("应解码: %s", u)
	}
}

func TestCleanTitle(t *testing.T) {
	// 带 URL 混入
	if s := cleanTitle("剧名 https://x.com"); s != "剧名" {
		t.Fatalf("cleanTitle 应去 URL: %q", s)
	}
	// 正常
	if s := cleanTitle("  纯标题  "); s != "纯标题" {
		t.Fatalf("cleanTitle 应 trim: %q", s)
	}
}

// ── parseBaiduHTML 边界 ──
func TestParseBaiduHTMLEdges(t *testing.T) {
	// javascript mu 跳过
	html := `<a data-log='{"mu":"javascript:void(0)"}' href="#">x</a>`
	if rs := NewBaidu().Parse(html); len(rs) != 0 {
		t.Fatalf("javascript mu 应跳过, got %d", len(rs))
	}
	// 直链兜底（非 baidu 域）
	html2 := `<a href="https://real-site.com/d.html">x</a><a href="https://www.baidu.com/x">b</a>`
	rs2 := NewBaidu().Parse(html2)
	if len(rs2) != 1 || rs2[0].URL != "https://real-site.com/d.html" {
		t.Fatalf("直链兜底应 1 条非 baidu, got %+v", rs2)
	}
	// 空 HTML
	if rs := NewBaidu().Parse(""); rs != nil {
		t.Fatal("空应 nil")
	}
}

// ── isCaptcha 各分支 ──
func TestIsCaptchaBranches(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"百度安全验证", true},
		{"captcha/tuxing", true},
		{"请输入验证码", true},
		{"访问验证", true},
		{"wappass.baidu.com/verify", true},
		{"wappass 登录跳转", false},        // 登录配置不算
		{"正常结果页面", false},           // 无验证码
		{"wappass loginUrlPrefix", false}, // 页面常量不算
	}
	for _, c := range cases {
		if got := isCaptcha(c.body); got != c.want {
			t.Fatalf("isCaptcha(%q)=%v, want %v", c.body, got, c.want)
		}
	}
}

// ── walkNodes ──
func TestWalkNodesNil(t *testing.T) {
	walkNodes(nil, func(*xhtml.Node) {}) // nil 不 panic
}

// ── isLegitSite 更多边界 ──
func TestIsLegitSiteEdges(t *testing.T) {
	// 子域名
	if !isLegitSite("https://v.qq.com/x/1.html") {
		t.Fatal("v.qq.com 应合法")
	}
	if !isLegitSite("https://www.bilibili.com/video/BV1xx") {
		t.Fatal("bilibili 应合法")
	}
	// 百度跳转链接不当合法站
	if isLegitSite("https://www.baidu.com/link?url=abc") {
		t.Fatal("baidu link 不应算合法站")
	}
	// 解析失败
	if isLegitSite("://bad") {
		t.Fatal("坏 URL 应 false")
	}
}

// ── 引擎 Name / SearchURL ──
func TestEngineMeta(t *testing.T) {
	if NewBing().Name() != "bing" || !strings.Contains(NewBing().SearchURL("q"), "q=") {
		t.Fatal("bing meta 错")
	}
	if NewSo360().Name() != "so" || !strings.Contains(NewSo360().SearchURL("q"), "q=") {
		t.Fatal("so meta 错")
	}
	if NewBaidu().Name() != "baidu" || !strings.Contains(NewBaidu().SearchURL("q"), "wd=") {
		t.Fatal("baidu meta 错")
	}
}

// ── BrowserSearch 无渲染器 ──
func TestBrowserSearchNoRenderer(t *testing.T) {
	a := New(nil)
	_, _, err := a.BrowserSearch(context.Background(), "q", nil, time.Second)
	if err == nil {
		t.Fatal("无渲染器应报错")
	}
}
