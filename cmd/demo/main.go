package main

import (
	"context"
	"fmt"
	"os"
	"time"

	search "github.com/chentianxiong123/go-proxy-search"
)

// 演示「库用法」：宿主注入 WebView 渲染函数（跨平台：Android 系统 WebView / 桌面 WebKitGTK）。
// 真实宿主（mycast wails3/Android）已有 WebView 渲染机制，注入即可，库不造轮子。
// 这里演示注入一个假渲染器（返回固定 HTML），展示库的搜索流程。
func main() {
	q := os.Args[1]
	if q == "" {
		q = "繁花 在线观看"
	}
	a := search.New(nil)
	// 宿主注入 WebView 渲染函数（真实环境：mycast 的 WebView 窗口渲染 + outerHTML 回传）
	a.SetRenderer(func(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
		// 演示：假渲染返回一个 360 引擎结果页 HTML（真实环境这里调宿主 WebView 渲染）
		return `<html><body><li class="res-list"><h3 class="res-title"><a data-mdurl="https://example.com/d1.html">演示站</a></h3></li></body></html>`, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rs, err := a.Search(ctx, q, search.Options{Engines: []string{"so"}, Timeout: 15 * time.Second, MaxResults: 10})
	if err != nil {
		fmt.Println("搜索失败:", err)
		os.Exit(1)
	}
	fmt.Printf("搜索结果 %d 条:\n", len(rs))
	for i, r := range rs {
		fmt.Printf("  [%d] %s | %s | %s\n", i, r.Engine, r.Title, r.URL)
	}
}
