package search

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ErrCaptcha 验证码/安全验证特征：搜索引擎触发反爬验证码。
// 检测到立即返回（不重试/不死磕），调用方识别后走 WebView 兜底。
var ErrCaptcha = errors.New("触发安全验证（验证码）")

// ChromePath 缓存检测到的 chrome/chromium 路径。
var ChromePath string

// FindChrome 检测系统 chrome/chromium（桌面版渲染器；Android 版用 WebView 不需要）。
// 返回路径或 ""。
func FindChrome() string {
	if ChromePath != "" {
		return ChromePath
	}
	for _, cand := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if p, err := exec.LookPath(cand); err == nil {
			ChromePath = p
			return p
		}
	}
	for _, p := range []string{
		"/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/chromium-browser",
	} {
		if _, err := os.Stat(p); err == nil {
			ChromePath = p
			return p
		}
	}
	return ""
}

// RenderWithChrome 用系统 Chrome headless 渲染 URL，返回渲染后的 HTML。
// virtual-time-budget 让 JS/网络回调跑完再 dump（搜索引擎空壳靠它出真结果）。
// 零重量级依赖：只 exec 系统 chrome（Blink 内核 = Android WebView 同源）。
// 这是默认渲染器（桌面）；移动端调用方注入 WebView 渲染函数。
func RenderWithChrome(ctx context.Context, rawURL string, timeout time.Duration) (string, error) {
	chrome := FindChrome()
	if chrome == "" {
		return "", errors.New("未找到系统 Chrome（桌面版渲染需要）")
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 独立 user-data-dir：带 cookie 态（模拟浏览器上下文，360 需要）
	dir, _ := os.MkdirTemp("", "search-service-chrome-*")
	defer os.RemoveAll(dir)

	cmd := exec.CommandContext(cctx, chrome,
		"--headless", "--disable-gpu", "--no-sandbox",
		"--user-data-dir="+dir,
		"--virtual-time-budget=8000",
		"--dump-dom", rawURL,
	)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		if cctx.Err() != nil {
			return "", errors.New("Chrome 渲染超时")
		}
		return "", errors.New("Chrome 执行失败: " + err.Error())
	}
	return out.String(), nil
}
