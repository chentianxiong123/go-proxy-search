package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SearchWithEngine 聚合搜索：并行跑启用引擎，去重，过滤，限 N 条。
// opts 控制 cookie/UA/超时/引擎开关/渲染器。
func (a *Aggregator) SearchWithEngine(ctx context.Context, query string, opts Options) ([]Result, error) {
	if query == "" {
		return nil, errors.New("查询词为空")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}
	if opts.MaxResults == 0 {
		opts.MaxResults = 30
	}
	// 启用引擎（空=全部）
	names := a.registry.Names()
	if len(opts.Engines) > 0 {
		names = opts.Engines
	}
	type engRes struct {
		name string
		rs   []Result
		err  error
	}
	ch := make(chan engRes, len(names))
	for _, name := range names {
		eng, ok := a.registry.Get(name)
		if !ok {
			continue
		}
		go func(name string, eng Engine) {
			rs, err := a.searchEngine(ctx, eng, query, opts)
			ch <- engRes{name, rs, err}
		}(name, eng)
	}
	seen := map[string]bool{}
	var all []Result
	var errs []string
	captcha := false
	for range names {
		res := <-ch
		if res.err != nil {
			if errors.Is(res.err, ErrCaptcha) {
				captcha = true
			}
			errs = append(errs, res.err.Error())
		}
		for _, r := range res.rs {
			if r.URL == "" || seen[r.URL] {
				continue
			}
			seen[r.URL] = true
			all = append(all, r)
			if len(all) >= opts.MaxResults {
				break
			}
		}
	}
	if captcha && len(all) == 0 {
		return nil, fmt.Errorf("%w", ErrCaptcha)
	}
	if len(all) == 0 {
		// 全引擎都是「无渲染器」→ 直接返回该错误（保留 errors.Is 链，宿主可识别）
		allUnavailable := len(errs) > 0
		for _, e := range errs {
			if !strings.Contains(e, ErrNoRenderer.Error()) {
				allUnavailable = false
				break
			}
		}
		if allUnavailable {
			return nil, ErrNoRenderer
		}
		if len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, "; "))
		}
		return nil, errors.New("无结果")
	}
	return all, nil
}

// searchEngine 单引擎搜索：WebView 渲染搜索页 → 解析 → 结果。
// 用 opts.Renderer（或宿主 SetRenderer 注入的 WebView 渲染），cookie 由 opts.CookieFn 提供。
// 无渲染器 → errBrowserUnavailable（宿主必须注入 WebView，不造 Chrome 轮子）。
func (a *Aggregator) searchEngine(ctx context.Context, eng Engine, query string, opts Options) ([]Result, error) {
	renderer := opts.Renderer
	if renderer == nil {
		renderer = a.renderer
	}
	if renderer == nil {
		return nil, ErrNoRenderer
	}
	htmlText, err := renderer(ctx, eng.SearchURL(query), opts.Timeout)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(htmlText) == "" {
		return nil, errors.New(eng.Name() + " 渲染为空")
	}
	if isCaptcha(htmlText) {
		return nil, fmt.Errorf("%w", ErrCaptcha)
	}
	return eng.Parse(htmlText), nil
}

// ParseRendered 解析已渲染 HTML（移动端前端 WebView 回传 outerHTML 时用）。
// engine 指定引擎名分派解析；空则尝试全部解析器去重。
func (a *Aggregator) ParseRendered(htmlText, engine string) []Result {
	if strings.TrimSpace(htmlText) == "" {
		return nil
	}
	if eng, ok := a.registry.Get(engine); ok {
		return eng.Parse(htmlText)
	}
	// 未知引擎：全部解析器去重合并
	seen := map[string]bool{}
	var out []Result
	for _, name := range a.registry.Names() {
		eng, _ := a.registry.Get(name)
		for _, r := range eng.Parse(htmlText) {
			if r.URL == "" || seen[r.URL] {
				continue
			}
			seen[r.URL] = true
			out = append(out, r)
		}
	}
	return out
}
