package search

import (
	"context"
	"strings"
	"time"
)

// BrowserEngine 一个搜索引擎的浏览器渲染定义（对标 SearXNG engines 注册）。
type BrowserEngine struct {
	Name string // so / baidu / bing
	URL  string // 搜索 URL
}

// AllEngines 全部引擎搜索 URL（顺序即优先级）。
// 渲染搜索用：引擎开关由调用方 Options.Engines 控制（库无状态，不读配置文件）。
func AllEngines(query string) []BrowserEngine {
	engines := NewRegistry()
	var out []BrowserEngine
	for _, name := range engines.Names() {
		eng, _ := engines.Get(name)
		out = append(out, BrowserEngine{Name: name, URL: eng.SearchURL(query)})
	}
	return out
}

// BrowserEngines 按开关过滤引擎（enabled 空=全部）。
func BrowserEngines(query string, enabled []string) []BrowserEngine {
	if enabled == nil {
		return AllEngines(query)
	}
	set := map[string]bool{}
	for _, e := range enabled {
		set[e] = true
	}
	var out []BrowserEngine
	for _, eng := range AllEngines(query) {
		if set[eng.Name] {
			out = append(out, eng)
		}
	}
	return out
}

// BrowserSearch 浏览器渲染搜索（框架入口，对标 SearXNG）：
// 依次渲染各引擎（宿主 WebView：Android 系统 WebView / 桌面 WebKitGTK）→
// ParseRendered 解析 → 过滤正版站。首个出真结果的引擎够用。
// 宿主必须 SetRenderer 注入 WebView 渲染函数（复用 mycast 已有 WebView 机制）。
// enabled 引擎开关（空=全部）；timeout 单引擎超时。
func (a *Aggregator) BrowserSearch(ctx context.Context, query string, enabled []string, timeout time.Duration) ([]Result, string, error) {
	if a.renderer == nil {
		return nil, "", ErrNoRenderer
	}
	if timeout == 0 {
		timeout = 18 * time.Second
	}
	seen := map[string]bool{}
	var all []Result
	for _, eng := range BrowserEngines(query, enabled) {
		htmlText, err := a.renderer(ctx, eng.URL, timeout)
		if err != nil || strings.TrimSpace(htmlText) == "" {
			continue
		}
		for _, r := range a.ParseRendered(htmlText, eng.Name) {
			if r.URL == "" || seen[r.URL] || isLegitSite(r.URL) {
				continue
			}
			seen[r.URL] = true
			all = append(all, r)
		}
		if len(all) > 0 {
			return all, eng.Name, nil // 首个出真结果的引擎够用
		}
	}
	if len(all) == 0 {
		return nil, "", errBrowserNoResults
	}
	return all, "", nil
}
