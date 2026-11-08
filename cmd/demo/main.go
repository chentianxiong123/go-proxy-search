package main

import (
	"context"
	"fmt"
	"os"
	"time"

	search "github.com/chentianxiong123/search-service"
)

func main() {
	q := os.Args[1]
	if q == "" {
		q = "繁花 在线观看"
	}
	a := search.New(nil) // 默认渲染器 = Chrome headless（WebView 渲染）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rs, err := a.Search(ctx, q, search.Options{Engines: []string{"bing", "so"}, Timeout: 15 * time.Second, MaxResults: 10})
	if err != nil {
		fmt.Println("搜索失败:", err)
		os.Exit(1)
	}
	fmt.Printf("搜索结果 %d 条:\n", len(rs))
	for i, r := range rs {
		fmt.Printf("  [%d] %s | %s | %s\n", i, r.Engine, r.Title, r.URL)
	}
}
