// Package search 无 UI 元搜索引擎聚合库（对标 SearXNG，纯库、无状态）。
//
// 核心思想：
//   - 引擎抽象：每个搜索引擎一个 Engine 实现（bing/360/baidu），注册进 Registry。
//   - WebView 渲染：搜索靠「渲染器」拿 HTML（桌面 Chrome headless / 移动 WebView），
//     渲染器由调用方注入（RenderFunc），库不知道具体渲染环境。
//   - 无状态：Aggregator 不持 cookie/配置持久状态；行为（cookie/UA/超时/引擎开关）
//     全由调用方通过 Options 传入。目标就是「搜索 → 拿结果」。
//
// 对标 SearXNG（searx/engines/*.py）：每引擎一套经过验证的请求+解析策略。
package search

import (
	"context"
	"errors"
	"time"
)

// Engine 一个搜索引擎的实现（对标 SearXNG engines/*.py）。
// Search 渲染/请求该引擎搜索页，返回解析后的结果。
// 引擎无状态：每次 Search 用传入的 opts（cookie/UA），不持跨请求状态。
type Engine interface {
	// Name 引擎名（bing / so / baidu），用于结果标记和解析分派。
	Name() string
	// SearchURL 给定查询词返回该引擎的搜索页 URL。
	SearchURL(query string) string
	// Parse 解析引擎搜索页 HTML → 结果（引擎自己的解析策略）。
	Parse(htmlText string) []Result
	// NeedCookie 该引擎是否需要 cookie（360/百度需要，bing 不需要）。
	NeedCookie() bool
}

// Registry 引擎注册表（对标 SearXNG 引擎加载）：按名查引擎。
type Registry struct {
	engines map[string]Engine
	order   []string // 优先级顺序（并行跑）
}

// NewRegistry 建注册表并注册默认三引擎（bing/so/baidu）。
func NewRegistry() *Registry {
	r := &Registry{engines: map[string]Engine{}}
	// 默认引擎（顺序即优先级，并行跑）
	r.Register(NewBing())
	r.Register(NewSo360())
	r.Register(NewBaidu())
	return r
}

// Register 注册引擎（同名覆盖）。
func (r *Registry) Register(e Engine) {
	r.engines[e.Name()] = e
	r.order = append(r.order, e.Name())
}

// Get 按名取引擎。
func (r *Registry) Get(name string) (Engine, bool) {
	e, ok := r.engines[name]
	return e, ok
}

// Names 已注册引擎名（顺序）。
func (r *Registry) Names() []string {
	return append([]string(nil), r.order...)
}

// Options 一次搜索的调用参数（无状态：每次搜索全量传入）。
// 这就是「库的用法」：调用方决定 cookie、UA、超时、引擎开关、渲染器。
type Options struct {
	// CookieFn 引擎 → cookie header（360/百度需要真实登录态 cookie 最稳）。
	// 可为 nil（无 cookie，靠渲染器带 cookie 态）。
	CookieFn func(engine string) string
	// UA 请求 User-Agent（空=默认桌面 Chrome）。
	UA string
	// Timeout 单引擎超时（空=默认）。
	Timeout time.Duration
	// Engines 启用的引擎（空=全部；否则按名过滤，如 ["baidu","so"]）。
	Engines []string
	// Renderer 渲染器（WebView 渲染搜索页拿 HTML）。空=用默认 Chrome headless。
	// 移动端调用方传 WebView 渲染函数（前端回传 HTML），桌面传 nil（用 Chrome）。
	Renderer RenderFunc
	// MaxResults 聚合结果上限（空=默认 30）。
	MaxResults int
}

// RenderFunc 渲染一个 URL 返回 HTML（WebView 渲染核心）。
// 桌面：调用方传 Chrome headless 渲染（或留空用默认）。
// 移动：调用方传「WebView 打开 URL → 回传 outerHTML」的函数。
type RenderFunc func(ctx context.Context, rawURL string, timeout time.Duration) (string, error)

// Aggregator 聚合搜索器（无状态：不持跨请求 cookie/配置）。
// 对标 SearXNG 的 Search 聚合：跑所有引擎、URL 去重、过滤、限 N 条。
type Aggregator struct {
	registry *Registry
	renderer RenderFunc // 默认渲染器（Chrome headless）
}

// New 建聚合搜索器（registry 空则默认三引擎）。
func New(registry *Registry) *Aggregator {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Aggregator{
		registry: registry,
		renderer: RenderWithChrome, // 默认：桌面 Chrome headless（Blink=Android WebView 同源）
	}
}

// SetRenderer 覆盖默认渲染器（移动端 WebView 回传）。
func (a *Aggregator) SetRenderer(r RenderFunc) {
	if r != nil {
		a.renderer = r
	}
}

// Search 聚合搜索：并行跑启用引擎，URL 去重，返回结果。
// opts 控制 cookie/UA/超时/引擎开关/渲染器。
func (a *Aggregator) Search(ctx context.Context, query string, opts Options) ([]Result, error) {
	return a.SearchWithEngine(ctx, query, opts)
}

// 浏览器渲染错误。
var (
	errBrowserUnavailable = errors.New("浏览器不可用（Android 无 Chrome，前端走 WebView）")
	errBrowserNoResults   = errors.New("浏览器渲染未提取到结果")
)

// Result 一条搜索结果。
type Result struct {
	Engine  string `json:"engine"`           // bing / so / baidu
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content,omitempty"`
}
