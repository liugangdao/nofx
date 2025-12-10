package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/sonirico/go-hyperliquid"
)

// ConfigFile 配置文件结构
type ConfigFile struct {
	Port                int     `json:"port"`
	PrivateKey          string  `json:"private_key"`
	WalletAddr          string  `json:"wallet_addr"`
	Testnet             bool    `json:"testnet"`
	DefaultQuantity     float64 `json:"default_quantity"`      // 固定数量（可选，如果为0则使用资金百分比）
	PositionSizePercent float64 `json:"position_size_percent"` // 资金百分比（默认5%）
	DefaultLeverage     int     `json:"default_leverage"`
	WebhookSecret       string  `json:"webhook_secret"`
}

// TradingViewSignal TradingView信号结构
type TradingViewSignal struct {
	Action   string  `json:"action"`   // "buy", "sell", "close_long", "close_short"
	Symbol   string  `json:"symbol"`   // "BTCUSDT"
	Quantity float64 `json:"quantity"` // 下单数量（可选，如果为0则使用默认值）
	Leverage int     `json:"leverage"` // 杠杆倍数（可选，默认5x）
	Price    float64 `json:"price"`    // 当前价格（可选，用于日志）
}

// WebhookServer TradingView Webhook服务器
type WebhookServer struct {
	router     *gin.Engine
	exchange   *hyperliquid.Exchange
	ctx        context.Context
	walletAddr string
	meta       *hyperliquid.Meta

	// 配置参数
	defaultQuantity     float64 // 固定下单数量（如果为0则使用资金百分比）
	positionSizePercent float64 // 资金百分比（默认5%）
	defaultLeverage     int     // 默认杠杆倍数
}

// Config Webhook服务器配置
type Config struct {
	Port                int     // 服务器端口
	PrivateKey          string  // Hyperliquid私钥（不带0x前缀）
	WalletAddr          string  // 钱包地址
	Testnet             bool    // 是否使用测试网
	DefaultQuantity     float64 // 固定下单数量（如果为0则使用资金百分比）
	PositionSizePercent float64 // 资金百分比（默认5%）
	DefaultLeverage     int     // 默认杠杆倍数
	WebhookSecret       string  // Webhook密钥（可选，用于验证请求）
}

func main() {
	// 命令行参数
	configFile := flag.String("config", "tradingview_config.json", "配置文件路径")
	port := flag.Int("port", 0, "服务器端口（覆盖配置文件）")
	flag.Parse()

	// 读取配置文件
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("❌ 加载配置文件失败: %v", err)
	}

	// 命令行参数覆盖配置文件
	if *port > 0 {
		config.Port = *port
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		log.Fatalf("❌ 配置验证失败: %v", err)
	}

	// 创建Webhook服务器
	server, err := NewWebhookServer(Config{
		Port:                config.Port,
		PrivateKey:          config.PrivateKey,
		WalletAddr:          config.WalletAddr,
		Testnet:             config.Testnet,
		DefaultQuantity:     config.DefaultQuantity,
		PositionSizePercent: config.PositionSizePercent,
		DefaultLeverage:     config.DefaultLeverage,
		WebhookSecret:       config.WebhookSecret,
	})
	if err != nil {
		log.Fatalf("❌ 创建服务器失败: %v", err)
	}

	// 启动服务器
	log.Printf("🚀 TradingView Webhook服务器启动中...")
	log.Printf("📋 配置信息:")
	log.Printf("  • 端口: %d", config.Port)
	log.Printf("  • 钱包: %s", config.WalletAddr)
	log.Printf("  • 测试网: %v", config.Testnet)

	// 显示下单数量配置
	if config.DefaultQuantity > 0 {
		log.Printf("  • 下单模式: 固定数量 (%.8f)", config.DefaultQuantity)
	} else {
		posPercent := config.PositionSizePercent
		if posPercent == 0 {
			posPercent = 5.0
		}
		log.Printf("  • 下单模式: 资金百分比 (%.1f%%)", posPercent)
	}

	log.Printf("  • 默认杠杆: %dx", config.DefaultLeverage)
	if config.WebhookSecret != "" {
		log.Printf("  • Webhook密钥: 已配置 ✓")
	} else {
		log.Printf("  • Webhook密钥: 未配置 ⚠️")
	}
	log.Println()

	if err := server.Start(config.Port); err != nil {
		log.Fatalf("❌ 服务器启动失败: %v", err)
	}
}

// NewWebhookServer 创建Webhook服务器
func NewWebhookServer(config Config) (*WebhookServer, error) {
	// 解析私钥
	privateKey, err := crypto.HexToECDSA(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	// 选择API URL
	apiURL := hyperliquid.MainnetAPIURL
	if config.Testnet {
		apiURL = hyperliquid.TestnetAPIURL
	}

	ctx := context.Background()

	// 创建Exchange客户端
	exchange := hyperliquid.NewExchange(
		ctx,
		privateKey,
		apiURL,
		nil,
		"",
		config.WalletAddr,
		nil,
	)

	// 获取meta信息
	meta, err := exchange.Info().Meta(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取meta信息失败: %w", err)
	}

	log.Printf("✓ Hyperliquid交易器初始化成功 (testnet=%v, wallet=%s)", config.Testnet, config.WalletAddr)

	// 设置Gin为Release模式
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// 启用CORS
	router.Use(corsMiddleware())

	// 设置默认资金百分比（如果未配置则使用5%）
	positionSizePercent := config.PositionSizePercent
	if positionSizePercent == 0 {
		positionSizePercent = 5.0 // 默认5%
	}

	server := &WebhookServer{
		router:              router,
		exchange:            exchange,
		ctx:                 ctx,
		walletAddr:          config.WalletAddr,
		meta:                meta,
		defaultQuantity:     config.DefaultQuantity,
		positionSizePercent: positionSizePercent,
		defaultLeverage:     config.DefaultLeverage,
	}

	// 设置路由
	server.setupRoutes(config.WebhookSecret)

	return server, nil
}

// Start 启动服务器
func (s *WebhookServer) Start(port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	log.Printf("🌐 TradingView Webhook服务器启动在 http://0.0.0.0:%d", port)
	log.Printf("📡 本地访问: http://localhost:%d/webhook", port)
	log.Printf("📡 外部访问: http://你的服务器IP:%d/webhook", port)
	log.Printf("💡 TradingView Alert配置示例:")
	log.Printf(`  {
    "action": "buy",
    "symbol": "BTCUSDT",
    "quantity": 0.01,
    "leverage": 5
  }`)
	log.Println()

	return s.router.Run(addr)
}

// setupRoutes 设置路由
func (s *WebhookServer) setupRoutes(webhookSecret string) {
	// 健康检查
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// TradingView Webhook端点
	s.router.POST("/webhook", s.createWebhookHandler(webhookSecret))
}

// createWebhookHandler 创建Webhook处理器（支持密钥验证）
func (s *WebhookServer) createWebhookHandler(webhookSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 验证Webhook密钥（如果配置了）
		if webhookSecret != "" {
			providedSecret := c.GetHeader("X-Webhook-Secret")
			if providedSecret != webhookSecret {
				log.Printf("❌ Webhook密钥验证失败")
				c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的webhook密钥"})
				return
			}
		}

		// 解析信号
		var signal TradingViewSignal
		if err := c.ShouldBindJSON(&signal); err != nil {
			log.Printf("❌ 解析信号失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的信号格式"})
			return
		}

		log.Printf("📨 收到TradingView信号: %+v", signal)

		// 处理信号
		result, err := s.handleSignal(&signal)
		if err != nil {
			log.Printf("❌ 处理信号失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		log.Printf("✓ 信号处理成功: %+v", result)
		c.JSON(http.StatusOK, result)
	}
}

// handleSignal 处理TradingView信号
func (s *WebhookServer) handleSignal(signal *TradingViewSignal) (map[string]any, error) {
	// 标准化symbol格式
	symbol := strings.ToUpper(signal.Symbol)
	if !strings.HasSuffix(symbol, "USDT") {
		symbol += "USDT"
	}

	// 计算下单数量
	quantity := signal.Quantity
	if quantity == 0 {
		// 如果配置了固定数量，使用固定数量
		if s.defaultQuantity > 0 {
			quantity = s.defaultQuantity
		} else {
			// 否则根据账户资金百分比自动计算
			calculatedQty, err := s.calculateQuantityByPercent(symbol, s.positionSizePercent)
			if err != nil {
				return nil, fmt.Errorf("计算下单数量失败: %w", err)
			}
			quantity = calculatedQty
			log.Printf("  💰 自动计算数量: %.8f (账户资金的 %.1f%%)", quantity, s.positionSizePercent)
		}
	}

	leverage := signal.Leverage
	if leverage == 0 {
		leverage = s.defaultLeverage
	}

	// 根据action执行相应操作
	action := strings.ToLower(signal.Action)
	switch action {
	case "buy", "long":
		return s.openLong(symbol, quantity, leverage)
	case "sell", "short":
		return s.openShort(symbol, quantity, leverage)
	case "close_long", "close":
		return s.closeLong(symbol, quantity)
	case "close_short":
		return s.closeShort(symbol, quantity)
	default:
		return nil, fmt.Errorf("未知的操作: %s", signal.Action)
	}
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Webhook-Secret")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// loadConfig 加载配置文件
func loadConfig(filename string) (*ConfigFile, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config ConfigFile
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &config, nil
}

// validateConfig 验证配置
func validateConfig(config *ConfigFile) error {
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d", config.Port)
	}

	if config.PrivateKey == "" {
		return fmt.Errorf("私钥不能为空")
	}

	if config.WalletAddr == "" {
		return fmt.Errorf("钱包地址不能为空")
	}

	// 如果没有配置固定数量，则必须配置资金百分比
	if config.DefaultQuantity == 0 && config.PositionSizePercent == 0 {
		// 使用默认值5%
		config.PositionSizePercent = 5.0
		log.Printf("⚠️  未配置下单数量和资金百分比，使用默认值: 5%%")
	}

	// 验证资金百分比范围
	if config.PositionSizePercent < 0 || config.PositionSizePercent > 100 {
		return fmt.Errorf("资金百分比必须在0-100之间")
	}

	if config.DefaultLeverage <= 0 || config.DefaultLeverage > 50 {
		return fmt.Errorf("默认杠杆必须在1-50之间")
	}

	return nil
}

// ==================== 交易操作函数 ====================

// openLong 开多仓
func (s *WebhookServer) openLong(symbol string, quantity float64, leverage int) (map[string]any, error) {
	log.Printf("📈 开多仓: %s 数量: %.4f 杠杆: %dx", symbol, quantity, leverage)

	// 检查是否已有该币种的仓位
	hasPosition, positionSide, err := s.checkExistingPosition(symbol)
	if err != nil {
		log.Printf("  ⚠ 检查仓位失败: %v", err)
	} else if hasPosition {
		log.Printf("  ⚠️ %s 已有 %s 仓位，跳过开仓", symbol, positionSide)
		return map[string]any{
			"action":  "buy",
			"symbol":  symbol,
			"status":  "skipped",
			"reason":  fmt.Sprintf("已有%s仓位", positionSide),
			"message": fmt.Sprintf("%s 已有 %s 仓位，跳过开多仓", symbol, positionSide),
		}, nil
	}

	// 先取消该币种的所有委托单
	if err := s.cancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败: %v", err)
	}

	// 设置杠杆
	if err := s.setLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// 转换symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// 获取当前价格
	price, err := s.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// 处理数量和价格精度
	roundedQuantity := s.roundToSzDecimals(coin, quantity)
	aggressivePrice := s.roundPriceToSigfigs(price * 1.01)

	log.Printf("  📏 数量: %.8f -> %.8f", quantity, roundedQuantity)
	log.Printf("  💰 价格: %.8f -> %.8f", price*1.01, aggressivePrice)

	// 创建市价买入订单
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity,
		Price: aggressivePrice,
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: false,
	}

	_, err = s.exchange.Order(s.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("开多仓失败: %w", err)
	}

	log.Printf("✓ 开多仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	return map[string]any{
		"action":   "buy",
		"symbol":   symbol,
		"quantity": roundedQuantity,
		"price":    aggressivePrice,
		"leverage": leverage,
		"status":   "success",
	}, nil
}

// openShort 开空仓
func (s *WebhookServer) openShort(symbol string, quantity float64, leverage int) (map[string]any, error) {
	log.Printf("📉 开空仓: %s 数量: %.4f 杠杆: %dx", symbol, quantity, leverage)

	// 检查是否已有该币种的仓位
	hasPosition, positionSide, err := s.checkExistingPosition(symbol)
	if err != nil {
		log.Printf("  ⚠ 检查仓位失败: %v", err)
	} else if hasPosition {
		log.Printf("  ⚠️ %s 已有 %s 仓位，跳过开仓", symbol, positionSide)
		return map[string]any{
			"action":  "sell",
			"symbol":  symbol,
			"status":  "skipped",
			"reason":  fmt.Sprintf("已有%s仓位", positionSide),
			"message": fmt.Sprintf("%s 已有 %s 仓位，跳过开空仓", symbol, positionSide),
		}, nil
	}

	// 先取消该币种的所有委托单
	if err := s.cancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消旧委托单失败: %v", err)
	}

	// 设置杠杆
	if err := s.setLeverage(symbol, leverage); err != nil {
		return nil, err
	}

	// 转换symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// 获取当前价格
	price, err := s.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// 处理数量和价格精度
	roundedQuantity := s.roundToSzDecimals(coin, quantity)
	aggressivePrice := s.roundPriceToSigfigs(price * 0.99)

	log.Printf("  📏 数量: %.8f -> %.8f", quantity, roundedQuantity)
	log.Printf("  💰 价格: %.8f -> %.8f", price*0.99, aggressivePrice)

	// 创建市价卖出订单
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity,
		Price: aggressivePrice,
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: false,
	}

	_, err = s.exchange.Order(s.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("开空仓失败: %w", err)
	}

	log.Printf("✓ 开空仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	return map[string]any{
		"action":   "sell",
		"symbol":   symbol,
		"quantity": roundedQuantity,
		"price":    aggressivePrice,
		"leverage": leverage,
		"status":   "success",
	}, nil
}

// closeLong 平多仓
func (s *WebhookServer) closeLong(symbol string, quantity float64) (map[string]any, error) {
	log.Printf("🔄 平多仓: %s", symbol)

	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := s.getPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
	}

	// 转换symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// 获取当前价格
	price, err := s.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// 处理数量和价格精度
	roundedQuantity := s.roundToSzDecimals(coin, quantity)
	aggressivePrice := s.roundPriceToSigfigs(price * 0.99)

	log.Printf("  📏 数量: %.8f -> %.8f", quantity, roundedQuantity)
	log.Printf("  💰 价格: %.8f -> %.8f", price*0.99, aggressivePrice)

	// 创建平仓订单
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: false,
		Size:  roundedQuantity,
		Price: aggressivePrice,
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true,
	}

	_, err = s.exchange.Order(s.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("平多仓失败: %w", err)
	}

	log.Printf("✓ 平多仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	// 平仓后取消该币种的所有挂单
	if err := s.cancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]any{
		"action":   "close_long",
		"symbol":   symbol,
		"quantity": roundedQuantity,
		"price":    aggressivePrice,
		"status":   "success",
	}, nil
}

// closeShort 平空仓
func (s *WebhookServer) closeShort(symbol string, quantity float64) (map[string]any, error) {
	log.Printf("🔄 平空仓: %s", symbol)

	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := s.getPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
	}

	// 转换symbol格式
	coin := convertSymbolToHyperliquid(symbol)

	// 获取当前价格
	price, err := s.getMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// 处理数量和价格精度
	roundedQuantity := s.roundToSzDecimals(coin, quantity)
	aggressivePrice := s.roundPriceToSigfigs(price * 1.01)

	log.Printf("  📏 数量: %.8f -> %.8f", quantity, roundedQuantity)
	log.Printf("  💰 价格: %.8f -> %.8f", price*1.01, aggressivePrice)

	// 创建平仓订单
	order := hyperliquid.CreateOrderRequest{
		Coin:  coin,
		IsBuy: true,
		Size:  roundedQuantity,
		Price: aggressivePrice,
		OrderType: hyperliquid.OrderType{
			Limit: &hyperliquid.LimitOrderType{
				Tif: hyperliquid.TifIoc,
			},
		},
		ReduceOnly: true,
	}

	_, err = s.exchange.Order(s.ctx, order, nil)
	if err != nil {
		return nil, fmt.Errorf("平空仓失败: %w", err)
	}

	log.Printf("✓ 平空仓成功: %s 数量: %.4f", symbol, roundedQuantity)

	// 平仓后取消该币种的所有挂单
	if err := s.cancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	return map[string]any{
		"action":   "close_short",
		"symbol":   symbol,
		"quantity": roundedQuantity,
		"price":    aggressivePrice,
		"status":   "success",
	}, nil
}

// ==================== 辅助函数 ====================

// checkExistingPosition 检查是否已有该币种的仓位
// 返回: (是否有仓位, 仓位方向, 错误)
func (s *WebhookServer) checkExistingPosition(symbol string) (bool, string, error) {
	positions, err := s.getPositions()
	if err != nil {
		return false, "", err
	}

	for _, pos := range positions {
		if pos["symbol"] == symbol {
			side := pos["side"].(string)
			return true, side, nil
		}
	}

	return false, "", nil
}

// calculateQuantityByPercent 根据账户资金百分比计算下单数量
func (s *WebhookServer) calculateQuantityByPercent(symbol string, percent float64) (float64, error) {
	// 获取账户余额
	accountState, err := s.exchange.Info().UserState(s.ctx, s.walletAddr)
	if err != nil {
		return 0, fmt.Errorf("获取账户信息失败: %w", err)
	}

	// 解析账户净值
	var accountValue float64
	fmt.Sscanf(accountState.MarginSummary.AccountValue, "%f", &accountValue)

	if accountValue <= 0 {
		return 0, fmt.Errorf("账户余额为0或无效")
	}

	// 获取当前价格
	price, err := s.getMarketPrice(symbol)
	if err != nil {
		return 0, err
	}

	// 计算下单金额（账户净值 * 百分比）
	orderValue := accountValue * (percent / 100.0)

	// 计算下单数量（下单金额 / 价格）
	quantity := orderValue / price

	log.Printf("  📊 账户净值: %.2f USDT", accountValue)
	log.Printf("  📊 下单金额: %.2f USDT (%.1f%%)", orderValue, percent)
	log.Printf("  📊 当前价格: %.2f USDT", price)
	log.Printf("  📊 计算数量: %.8f", quantity)

	return quantity, nil
}

// setLeverage 设置杠杆
func (s *WebhookServer) setLeverage(symbol string, leverage int) error {
	coin := convertSymbolToHyperliquid(symbol)
	_, err := s.exchange.UpdateLeverage(s.ctx, leverage, coin, false)
	if err != nil {
		return fmt.Errorf("设置杠杆失败: %w", err)
	}
	log.Printf("  ✓ %s 杠杆已设置为 %dx", symbol, leverage)
	return nil
}

// cancelAllOrders 取消该币种的所有委托单
func (s *WebhookServer) cancelAllOrders(symbol string) error {
	coin := convertSymbolToHyperliquid(symbol)

	openOrders, err := s.exchange.Info().OpenOrders(s.ctx, s.walletAddr)
	if err != nil {
		return fmt.Errorf("获取挂单失败: %w", err)
	}

	for _, order := range openOrders {
		if order.Coin == coin {
			_, err := s.exchange.Cancel(s.ctx, coin, order.Oid)
			if err != nil {
				log.Printf("  ⚠ 取消订单失败 (oid=%d): %v", order.Oid, err)
			}
		}
	}

	return nil
}

// getMarketPrice 获取市场价格
func (s *WebhookServer) getMarketPrice(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	allMids, err := s.exchange.Info().AllMids(s.ctx)
	if err != nil {
		return 0, fmt.Errorf("获取价格失败: %w", err)
	}

	if priceStr, ok := allMids[coin]; ok {
		var price float64
		_, err := fmt.Sscanf(priceStr, "%f", &price)
		if err == nil {
			return price, nil
		}
		return 0, fmt.Errorf("价格格式错误: %v", err)
	}

	return 0, fmt.Errorf("未找到 %s 的价格", symbol)
}

// getPositions 获取所有持仓
func (s *WebhookServer) getPositions() ([]map[string]any, error) {
	accountState, err := s.exchange.Info().UserState(s.ctx, s.walletAddr)
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]any

	for _, assetPos := range accountState.AssetPositions {
		position := assetPos.Position

		var posAmt float64
		fmt.Sscanf(position.Szi, "%f", &posAmt)

		if posAmt == 0 {
			continue
		}

		posMap := make(map[string]any)
		symbol := position.Coin + "USDT"
		posMap["symbol"] = symbol

		if posAmt > 0 {
			posMap["side"] = "long"
			posMap["positionAmt"] = posAmt
		} else {
			posMap["side"] = "short"
			posMap["positionAmt"] = -posAmt
		}

		result = append(result, posMap)
	}

	return result, nil
}

// getSzDecimals 获取币种的数量精度
func (s *WebhookServer) getSzDecimals(coin string) int {
	if s.meta == nil {
		return 4
	}

	for _, asset := range s.meta.Universe {
		if asset.Name == coin {
			return asset.SzDecimals
		}
	}

	return 4
}

// roundToSzDecimals 将数量四舍五入到正确的精度
func (s *WebhookServer) roundToSzDecimals(coin string, quantity float64) float64 {
	szDecimals := s.getSzDecimals(coin)

	multiplier := 1.0
	for i := 0; i < szDecimals; i++ {
		multiplier *= 10.0
	}

	return float64(int(quantity*multiplier+0.5)) / multiplier
}

// roundPriceToSigfigs 将价格四舍五入到5位有效数字
func (s *WebhookServer) roundPriceToSigfigs(price float64) float64 {
	if price == 0 {
		return 0
	}

	const sigfigs = 5

	magnitude := price
	if magnitude < 0 {
		magnitude = -magnitude
	}

	multiplier := 1.0
	for magnitude >= 10 {
		magnitude /= 10
		multiplier /= 10
	}
	for magnitude < 1 {
		magnitude *= 10
		multiplier *= 10
	}

	for i := 0; i < sigfigs-1; i++ {
		multiplier *= 10
	}

	return float64(int(price*multiplier+0.5)) / multiplier
}

// convertSymbolToHyperliquid 将标准symbol转换为Hyperliquid格式
func convertSymbolToHyperliquid(symbol string) string {
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}
