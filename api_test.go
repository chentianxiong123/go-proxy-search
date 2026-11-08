package search

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// ── Registry 引擎注册表 ──
func TestNewRegistryDefaultEngines(t *testing.T) {
	r := NewRegistry()
	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("默认应 3 引擎, got %d %v", len(names), names)
	}
	// 三引擎都在
	for _, want := range []string{"bing", "so", "baidu"} {
		if _, ok := r.Get(want); !ok {
			t.Fatalf("缺引擎 %s", want)
		}
	}
}

func TestRegistryGetMissing(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("nope"); ok {
		t.Fatal("未知引擎应不存在")
	}
}

func TestRegistryRegisterOverride(t *testing.T) {
	r := NewRegistry()
	// 注册一个同名引擎覆盖
	r.Register(&fakeEngine{name: "bing"})
	names := r.Names()
	if len(names) != 4 {
		t.Fatalf("覆盖后应 4 个名（含重复 bing）, got %d", len(names))
	}
	if e, _ := r.Get("bing"); e.Name() != "bing" {
		t.Fatal("覆盖后应返回新引擎")
	}
}

// fakeEngine 测试引擎。
type fakeEngine struct{ name string }

func (f *fakeEngine) Name() string { return f.name }
func (f *fakeEngine) SearchURL(q string) string { return "https://example.com/s?q=" + q }
func (f *fakeEngine) Parse(html string) []Result {
	if strings.Contains(html, "hit") {
		return []Result{{Engine: f.name, Title: "t", URL: "https://hit.com/d.html"}}
	}
	return nil
}
func (f *fakeEngine) NeedCookie() bool { return false }

// ── 引擎开关过滤（BrowserEngines）──
func TestBrowserEnginesFilter(t *testing.T) {
	// nil=全开
	all := BrowserEngines("q", nil)
	if len(all) != 3 {
		t.Fatalf("nil 应全开 3 引擎, got %d", len(all))
	}
	// 只开 baidu+so
	filtered := BrowserEngines("q", []string{"baidu", "so"})
	if len(filtered) != 2 {
		t.Fatalf("过滤应 2 引擎, got %d %v", len(filtered), filtered)
	}
	for _, e := range filtered {
		if e.Name != "baidu" && e.Name != "so" {
			t.Fatalf("过滤错: %s", e.Name)
		}
	}
}

func TestAllEnginesURL(t *testing.T) {
	engs := AllEngines("测试 查询")
	if len(engs) != 3 {
		t.Fatalf("AllEngines 应 3, got %d", len(engs))
	}
	for _, e := range engs {
		if !strings.Contains(e.URL, "q=") && !strings.Contains(e.URL, "wd=") {
			t.Fatalf("引擎 %s URL 缺查询参数: %s", e.Name, e.URL)
		}
	}
}

// ── Aggregator 构造 + SetRenderer ──
func TestNewNilRegistry(t *testing.T) {
	a := New(nil)
	if a == nil {
		t.Fatal("New(nil) 不应 nil")
	}
}

func TestSetRendererNil(t *testing.T) {
	a := New(nil)
	a.SetRenderer(nil) // nil 注入不改变，不 panic
	if a.renderer != nil {
		t.Fatal("默认应无渲染器（WebView 由宿主注入）")
	}
}

// 无渲染器：Search 应返回 errBrowserUnavailable（宿主必须注入 WebView）。
func TestSearchNoRenderer(t *testing.T) {
	a := New(nil)
	_, err := a.Search(context.Background(), "q", Options{Engines: []string{"bing"}})
	if !errors.Is(err, errBrowserUnavailable) {
		t.Fatalf("无渲染器应返回 errBrowserUnavailable, got %v", err)
	}
}

// ── Search 边界 ──
func TestSearchEmptyQuery(t *testing.T) {
	a := New(nil)
	if _, err := a.Search(context.Background(), "", Options{}); err == nil {
		t.Fatal("空查询应报错")
	}
}

func TestSearchNoResults(t *testing.T) {
	a := New(nil)
	a.SetRenderer(func(ctx context.Context, u string, d time.Duration) (string, error) {
		return "<html><body>无结果页面</body></html>", nil
	})
	_, err := a.Search(context.Background(), "啥都没有", Options{Engines: []string{"bing"}, Timeout: 2 * time.Second})
	if err == nil {
		t.Fatal("无结果应报错")
	}
}

func TestSearchAllEngineFail(t *testing.T) {
	a := New(nil)
	a.SetRenderer(func(ctx context.Context, u string, d time.Duration) (string, error) {
		return "", errors.New("渲染失败")
	})
	_, err := a.Search(context.Background(), "q", Options{Engines: []string{"bing", "so"}, Timeout: 2 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "渲染失败") {
		t.Fatalf("全失败应报渲染错误, got %v", err)
	}
}

func TestSearchPartialEngineFail(t *testing.T) {
	a := New(nil)
	a.SetRenderer(func(ctx context.Context, u string, d time.Duration) (string, error) {
		if strings.Contains(u, "bing") {
			return "", errors.New("bing 失败")
		}
		return `<li class="res-list"><h3 class="res-title"><a data-mdurl="https://ok.com/d.html">OK</a></h3></li>`, nil
	})
	rs, err := a.Search(context.Background(), "q", Options{Engines: []string{"bing", "so"}, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("部分失败应仍返回结果: %v", err)
	}
	if len(rs) != 1 || rs[0].URL != "https://ok.com/d.html" {
		t.Fatalf("部分失败结果错: %+v", rs)
	}
}

func TestSearchMaxResults(t *testing.T) {
	a := New(nil)
	// so 引擎渲染返回 5 条
	a.SetRenderer(func(ctx context.Context, u string, d time.Duration) (string, error) {
		var b strings.Builder
		for i := 0; i < 5; i++ {
			b.WriteString(`<li class="res-list"><h3 class="res-title"><a data-mdurl="https://x.com/d` + string(rune('0'+i)) + `.html">t</a></h3></li>`)
		}
		return b.String(), nil
	})
	rs, err := a.Search(context.Background(), "q", Options{Engines: []string{"so"}, Timeout: 2 * time.Second, MaxResults: 3})
	if err != nil {
		t.Fatalf("Search err: %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("MaxResults=3 应限 3 条, got %d", len(rs))
	}
}

// ── 正版站过滤（IsLegitSite）──
func TestIsLegitSite(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.bilibili.com/video/BV1xx", true},
		{"https://v.qq.com/x/cover/1.html", true},
		{"https://www.iqiyi.com/v_1.html", true},
		{"https://www.youku.com/v/1.html", true},
		{"https://www.so.com/s?q=test", true},
		{"https://www.baidu.com/link?url=abc", false}, // 百度跳转链接不算合法站
		{"https://pirate-site.com/detail/1.html", false},
	}
	for _, c := range cases {
		if got := IsLegitSite(c.url); got != c.want {
			t.Fatalf("IsLegitSite(%s) = %v, want %v", c.url, got, c.want)
		}
	}
}

// ── ParseRendered 未知引擎全部解析器 ──
func TestParseRenderedUnknownEngine(t *testing.T) {
	a := New(nil)
	// 未知引擎：尝试全部解析器，bing 格式能解析
	html := `<li class="b_algo"><h2><a href="https://y.com/d.html">未知引擎解析</a></h2></li>`
	rs := a.ParseRendered(html, "unknown")
	if len(rs) == 0 {
		t.Fatal("未知引擎应尝试全部解析器出结果")
	}
}

func TestParseRenderedEmpty(t *testing.T) {
	a := New(nil)
	if rs := a.ParseRendered("   ", "bing"); rs != nil {
		t.Fatal("空 HTML 应 nil")
	}
}

// ── HTTPError ──
func TestHTTPError(t *testing.T) {
	e := &HTTPError{Code: 403}
	if e.Error() == "" || e.Code != 403 {
		t.Fatalf("HTTPError 错: %s", e.Error())
	}
}

// ── get 请求（渲染后备）──
func TestGetStatusCode(t *testing.T) {
	// 无网络依赖：直接测错误码路径（非 200）
	_, err := get(context.Background(), "http://127.0.0.1:1/x", "", "", time.Second)
	if err == nil {
		t.Fatal("连不上应报错")
	}
}

// ── BrowserSearch ──
func TestBrowserSearchFirstEngine(t *testing.T) {
	a := New(nil)
	a.SetRenderer(func(ctx context.Context, u string, d time.Duration) (string, error) {
		if strings.Contains(u, "bing.com") {
			return `<li class="b_algo"><h2><a href="https://first.com/d.html">首个</a></h2></li>`, nil
		}
		return `<li class="res-list"><h3 class="res-title"><a data-mdurl="https://second.com/d.html">第二个</a></h3></li>`, nil
	})
	rs, eng, err := a.BrowserSearch(context.Background(), "q", []string{"bing", "so"}, 2*time.Second)
	if err != nil {
		t.Fatalf("BrowserSearch err: %v", err)
	}
	if eng != "bing" || len(rs) == 0 {
		t.Fatalf("首个出结果引擎应 bing, got %s", eng)
	}
}

func TestBrowserSearchNoResults(t *testing.T) {
	a := New(nil)
	a.SetRenderer(func(ctx context.Context, u string, d time.Duration) (string, error) {
		return "<html><body>无结果</body></html>", nil
	})
	_, _, err := a.BrowserSearch(context.Background(), "q", nil, 2*time.Second)
	if err == nil {
		t.Fatal("无结果应报 errBrowserNoResults")
	}
}
