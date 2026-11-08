package search

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// 引擎解析测试（对标 SearXNG 各引擎解析策略）。
func TestBingParse(t *testing.T) {
	html := `<html><body><ol id="b_results">
	<li class="b_algo"><h2><a href="/ck/a?u=a1aHR0cHM6Ly9leGFtcGxlLmNvbS9kMS5odG1s">测试站点</a></h2></li>
	<li class="b_algo"><h2><a href="https://other.com/v2.html">另一个站</a></h2></li>
	</ol></body></html>`
	rs := NewBing().Parse(html)
	if len(rs) != 2 {
		t.Fatalf("bing 应 2 条, got %d", len(rs))
	}
	if rs[0].Title != "测试站点" {
		t.Fatalf("bing title=%q", rs[0].Title)
	}
}

func TestSo360Parse(t *testing.T) {
	html := `<html><body>
	<li class="res-list"><h3 class="res-title"><a data-mdurl="https://so-site.com/d.html">360站点</a></h3></li>
	<li class="res-list"><h3 class="res-title"><a href="https://so-site2.com/v.html">360二号</a></h3></li>
	</body></html>`
	rs := NewSo360().Parse(html)
	if len(rs) != 2 {
		t.Fatalf("360 应 2 条, got %d", len(rs))
	}
	if rs[0].Title != "360站点" || rs[0].URL != "https://so-site.com/d.html" {
		t.Fatalf("360 解析错: %+v", rs[0])
	}
}

func TestBaiduParse(t *testing.T) {
	// data-log mu 规则
	html := `<html><body><a data-log='{"mu":"https://bd-site.com/d.html"}' href="#">x</a></body></html>`
	rs := NewBaidu().Parse(html)
	if len(rs) != 1 || rs[0].URL != "https://bd-site.com/d.html" {
		t.Fatalf("baidu data-log 解析错: %+v", rs)
	}
	// baidu.com/link 规则
	html2 := `<a href="https://www.baidu.com/link?url=abc123">x</a><a href="https://www.baidu.com/link?url=def456">y</a>`
	rs2 := NewBaidu().Parse(html2)
	if len(rs2) != 2 {
		t.Fatalf("baidu link 应 2 条, got %d", len(rs2))
	}
}

// 聚合去重 + 渲染器注入（无状态：渲染器由调用方传）。
func TestAggregateDedup(t *testing.T) {
	a := New(nil)
	// 注入假渲染器：返回含结果的 HTML（bing 引擎搜索）
	a.SetRenderer(func(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
		if strings.Contains(rawURL, "bing.com") {
			return `<li class="b_algo"><h2><a href="https://dup.com/d.html">去重测试</a></h2></li>`, nil
		}
		return `<li class="res-list"><h3 class="res-title"><a data-mdurl="https://dup.com/d.html">去重测试</a></h3></li>`, nil
	})
	rs, err := a.Search(context.Background(), "测试", Options{Engines: []string{"bing", "so"}, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	// bing+so 都返回 dup.com/d.html → 去重后应 1 条
	if len(rs) != 1 {
		t.Fatalf("去重后应 1 条, got %d", len(rs))
	}
}

// 验证码立即返回（不死磕）。
func TestCaptchaReturn(t *testing.T) {
	a := New(nil)
	a.SetRenderer(func(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
		return `<html><body>百度安全验证 请输入验证码</body></html>`, nil
	})
	_, err := a.Search(context.Background(), "测试", Options{Engines: []string{"baidu"}, Timeout: 2 * time.Second})
	if !errors.Is(err, ErrCaptcha) {
		t.Fatalf("应返回 ErrCaptcha, got %v", err)
	}
}

// ParseRendered 按引擎分派。
func TestParseRenderedDispatch(t *testing.T) {
	a := New(nil)
	html := `<li class="b_algo"><h2><a href="https://x.com/d.html">分派</a></h2></li>`
	rs := a.ParseRendered(html, "bing")
	if len(rs) != 1 || rs[0].Engine != "bing" {
		t.Fatalf("分派错: %+v", rs)
	}
}

// bing base64url 解码边界（负 pad 不 panic）。
func TestBingRealURLPad(t *testing.T) {
	u := bingRealURL("/ck/a?u=a1YQ==")
	if u == "" {
		t.Fatal("bingRealURL 应解码")
	}
}
