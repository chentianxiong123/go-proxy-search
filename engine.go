// Package search 无 UI 元搜索引擎聚合库（对标 SearXNG，纯库、无状态）。
//
// 核心思想：
//   - 引擎抽象：每个搜索引擎一个 Engine 实现（bing/360/baidu），注册进 Registry。
//   - WebView 渲染：搜索靠「宿主注入的 WebView 渲染」拿 HTML（跨平台：
//     Android 系统 WebView / 桌面 WebKitGTK），库不造渲染轮子。
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
	// Renderer 渲染器（WebView 渲染搜索页拿 HTML）。
	// 空=用 Aggregator 注入的 WebView 渲染（宿主 SetRenderer）；都没设则报错。
	// 跨平台：Android 系统 WebView / 桌面 WebKitGTK（复用 mycast 已有 WebView 机制）。
	Renderer RenderFunc
	// MaxResults 聚合结果上限（空=默认 30）。
	MaxResults int
}

// RenderFunc 渲染一个 URL 返回 HTML（WebView 渲染核心）。
// 宿主注入（复用 mycast 已有的 WebView 机制，不造轮子）：
//   - Android：系统 WebView 渲染引擎页 → JS 回传 outerHTML
//   - 桌面：WebKitGTK（wails3 子 WebView 窗口）→ 回传 outerHTML
//
// 库不实现渲染（那是宿主 GUI 的事，mycast 已做），只接收宿主注入的函数。
type RenderFunc func(ctx context.Context, rawURL string, timeout time.Duration) (string, error)

// Aggregator 聚合搜索器（无状态：不持跨请求 cookie/配置）。
// 对标 SearXNG 的 Search 聚合：跑所有引擎、URL 去重、过滤、限 N 条。
type Aggregator struct {
	registry *Registry
	renderer RenderFunc // WebView 渲染函数（宿主注入，跨平台：Android WebView / WebKitGTK）
}

// New 建聚合搜索器（registry 空则默认三引擎）。
// 渲染器默认 nil：宿主必须 SetRenderer 注入 WebView 渲染函数
// （复用 mycast 已有的 WebView 机制），否则 Search 返回 errBrowserUnavailable。
func New(registry *Registry) *Aggregator {
	if registry == nil {
		registry = NewRegistry()
	}
	return &Aggregator{
		registry: registry,
		renderer: nil, // 无默认渲染器：WebView 渲染由宿主注入（不造 Chrome headless 轮子）
	}
}

// SetRenderer 注入 WebView 渲染函数（宿主：Android 系统 WebView / 桌面 WebKitGTK）。
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
	errBrowserUnavailable = errors.New("WebView 渲染器未注入（宿主 SetRenderer 传 Android WebView / WebKitGTK 渲染函数）")
	errBrowserNoResults   = errors.New("浏览器渲染未提取到结果")
)

// ErrNoRenderer 渲染器未注入：宿主必须 SetRenderer 注入 WebView 渲染函数，
// 否则库无法搜（库无 GUI 不能创建 WebView）。公开导出，宿主可 errors.Is 识别。
var ErrNoRenderer = errBrowserUnavailable

// Result 一条搜索结果。
type Result struct {
	Engine  string `json:"engine"`           // bing / so / baidu
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content,omitempty"`
}
