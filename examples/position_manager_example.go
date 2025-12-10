package main

import (
	"log"
	"nofx/trader"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Println("🚀 仓位管理AI机器人示例")
	log.Println("=" + "=")

	// 创建仓位管理器配置
	pmConfig := trader.PositionManagerConfig{
		ID:                  "position_manager_example",
		Name:                "Position Manager Example",
		AIModel:             "deepseek", // 可选: "deepseek", "qwen", "gemini", "custom"
		Exchange:            "binance",  // 可选: "binance", "hyperliquid", "aster"
		EnableScreenshot:    false,
		ScanInterval:        3 * time.Minute,
		ScanIntervalMinutes: 3,
		InitialBalance:      1000.0,
		BTCETHLeverage:      5,
		AltcoinLeverage:     3,

		// 币安配置（从环境变量读取）
		BinanceAPIKey:    os.Getenv("BINANCE_API_KEY"),
		BinanceSecretKey: os.Getenv("BINANCE_SECRET_KEY"),

		// AI配置（从环境变量读取）
		DeepSeekKey: os.Getenv("DEEPSEEK_API_KEY"),
		QwenKey:     os.Getenv("QWEN_API_KEY"),
		GeminiKey:   os.Getenv("GEMINI_API_KEY"),
	}

	// 验证必要的配置
	if pmConfig.Exchange == "binance" {
		if pmConfig.BinanceAPIKey == "" || pmConfig.BinanceSecretKey == "" {
			log.Fatal("❌ 请设置环境变量: BINANCE_API_KEY 和 BINANCE_SECRET_KEY")
		}
	}

	if pmConfig.AIModel == "deepseek" && pmConfig.DeepSeekKey == "" {
		log.Fatal("❌ 请设置环境变量: DEEPSEEK_API_KEY")
	}

	// 创建仓位管理器
	pm, err := trader.NewPositionManager(pmConfig)
	if err != nil {
		log.Fatalf("❌ 创建仓位管理器失败: %v", err)
	}

	log.Printf("✅ 仓位管理器创建成功: %s", pm.GetName())
	log.Printf("📊 配置信息:")
	log.Printf("   - AI模型: %s", pmConfig.AIModel)
	log.Printf("   - 交易平台: %s", pmConfig.Exchange)
	log.Printf("   - 扫描间隔: %v", pmConfig.ScanInterval)
	log.Printf("   - 初始余额: %.2f USDT", pmConfig.InitialBalance)
	log.Println()

	// 设置信号处理（优雅退出）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在goroutine中运行仓位管理器
	go func() {
		if err := pm.Run(); err != nil {
			log.Printf("❌ 仓位管理器运行错误: %v", err)
		}
	}()

	// 等待退出信号
	<-sigChan
	log.Println("\n⏹ 收到退出信号，正在停止...")
	pm.Stop()
	log.Println("👋 仓位管理器已停止")
}
