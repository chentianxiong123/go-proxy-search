package search

import (
	"context"
	"errors"
	"time"
)

// ErrCaptcha 验证码/安全验证特征：搜索引擎触发反爬验证码。
// 检测到立即返回（不重试/不死磕），调用方识别后走 WebView 兜底。
var ErrCaptcha = errors.New("触发安全验证（验证码）")

// WebViewRender 跨平台 WebView 渲染能力（核心语义）：
// 宿主注入「渲染 URL → 拿 HTML」的函数，跨端通用：
//   - Android：系统 WebView（JS 渲染引擎页 → 回传 outerHTML）
//   - 桌面：WebKitGTK（wails3 原生 WebView 窗口 → 回传 outerHTML）
//
// 库不创建 WebView（那是宿主 GUI 组件），由宿主注入。库只管：
// 拿引擎 URL → 调 WebView 渲染 → 解析 HTML → 结果。
//
// 这就是「搜索引擎全走 WebView 渲染，跨平台通用」的实现：
// 不依赖无头 Chrome（手机没有），不依赖 fetch（引擎反爬），只依赖宿主 WebView。
//
// 返回 HTML 必须是 JS 渲染后的 document.documentElement.outerHTML
// （搜索引擎空壳页靠 JS 渲染才出真结果）。
type WebViewRender func(ctx context.Context, rawURL string, timeout time.Duration) (string, error)
