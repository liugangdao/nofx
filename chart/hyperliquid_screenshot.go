package chart

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

// ScreenshotHyperliquidChart 直接从Hyperliquid网页截图图表
func ScreenshotHyperliquidChart(symbol string) ([]byte, error) {
	// 构建Hyperliquid交易页面URL
	url := fmt.Sprintf("https://app.hyperliquid.xyz/trade/%s", symbol)

	log.Printf("📊 正在从Hyperliquid截图: %s", url)

	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "nofx_hyperliquid_screenshots")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 创建chromedp上下文，启用无头模式
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-features", "VizDisplayCompositor"),
		chromedp.WindowSize(1920, 1080),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 设置超时时间
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var buf []byte

	// 执行截图任务 - 简单直接的方法
	err := chromedp.Run(ctx,
		// 导航到页面
		chromedp.Navigate(url),

		// 等待页面完全加载（给足够的时间让图表渲染）
		chromedp.Sleep(8*time.Second),

		// 对图表容器进行截图
		chromedp.Screenshot("div[id='tv_chart_container']", &buf, chromedp.ByQuery),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp执行失败: %w", err)
	}

	if len(buf) == 0 {
		return nil, fmt.Errorf("截图为空")
	}

	// 可选：保存截图到临时文件用于调试
	tempFile := filepath.Join(tempDir, fmt.Sprintf("%s_chart_%d.png", symbol, time.Now().Unix()))
	if err := os.WriteFile(tempFile, buf, 0644); err != nil {
		log.Printf("⚠️ 保存临时截图文件失败: %v", err)
	} else {
		log.Printf("✅ 截图已保存到: %s", tempFile)

		// 5分钟后清理临时文件
		go func() {
			time.Sleep(5 * time.Minute)
			os.Remove(tempFile)
		}()
	}

	log.Printf("✅ Hyperliquid截图完成，大小: %d bytes", len(buf))
	return buf, nil
}
