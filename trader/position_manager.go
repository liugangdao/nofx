package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"strings"
	"time"
)

// PositionManagerConfig 仓位管理器配置
type PositionManagerConfig struct {
	ID                  string        // 管理器唯一标识
	Name                string        // 管理器显示名称
	AIModel             string        // AI模型: "qwen", "deepseek", "gemini", "custom"
	Exchange            string        // 交易平台: "binance", "hyperliquid", "aster"
	EnableScreenshot    bool          // 是否启用图表截图
	ScanInterval        time.Duration // 扫描间隔
	ScanIntervalMinutes int           // 扫描间隔分钟数
	InitialBalance      float64       // 初始余额（用于计算盈亏）
	BTCETHLeverage      int           // BTC/ETH杠杆倍数
	AltcoinLeverage     int           // 山寨币杠杆倍数

	// 交易器配置（从现有trader复用）
	BinanceAPIKey         string
	BinanceSecretKey      string
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool
	AsterUser             string
	AsterSigner           string
	AsterPrivateKey       string

	// AI配置
	DeepSeekKey     string
	QwenKey         string
	GeminiKey       string
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string
}

// PositionManager 仓位管理器（只管理现有仓位，不开新仓）
type PositionManager struct {
	id                             string
	name                           string
	aiModel                        string
	exchange                       string
	enableScreenshot               bool
	config                         PositionManagerConfig
	trader                         Trader
	mcpClient                      *mcp.Client
	decisionLogger                 *logger.DecisionLogger
	initialBalance                 float64
	isRunning                      bool
	startTime                      time.Time
	callCount                      int
	positionFirstSeenTime          map[string]int64
	positionInvalidationConditions map[string]string
	positionReasonings             map[string]string
	positionPnLTracking            map[string]*PnLTracking
}

// NewPositionManager 创建仓位管理器
func NewPositionManager(config PositionManagerConfig) (*PositionManager, error) {
	if config.ID == "" {
		config.ID = "position_manager"
	}
	if config.Name == "" {
		config.Name = "Position Manager"
	}

	// 初始化MCP客户端
	mcpClient := mcp.New()

	// 配置AI
	switch config.AIModel {
	case "custom":
		mcpClient.SetCustomAPI(config.CustomAPIURL, config.CustomAPIKey, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s", config.Name, config.CustomAPIURL)
	case "gemini":
		if err := mcpClient.SetGeminiAPIKey(config.GeminiKey); err != nil {
			return nil, fmt.Errorf("初始化Gemini API失败: %w", err)
		}
		log.Printf("🤖 [%s] 使用Google Gemini AI", config.Name)
	case "qwen":
		mcpClient.SetQwenAPIKey(config.QwenKey, "")
		log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
	default:
		mcpClient.SetDeepSeekAPIKey(config.DeepSeekKey)
		log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
	}

	// 创建交易器
	var trader Trader
	var err error

	switch config.Exchange {
	case "binance":
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey)
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
	case "hyperliquid":
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
	case "aster":
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
		log.Printf("🏦 [%s] 使用Aster交易", config.Name)
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 初始化决策日志
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	return &PositionManager{
		id:                             config.ID,
		name:                           config.Name,
		aiModel:                        config.AIModel,
		exchange:                       config.Exchange,
		enableScreenshot:               config.EnableScreenshot,
		config:                         config,
		trader:                         trader,
		mcpClient:                      mcpClient,
		decisionLogger:                 decisionLogger,
		initialBalance:                 config.InitialBalance,
		isRunning:                      false,
		startTime:                      time.Now(),
		callCount:                      0,
		positionFirstSeenTime:          make(map[string]int64),
		positionInvalidationConditions: make(map[string]string),
		positionReasonings:             make(map[string]string),
		positionPnLTracking:            make(map[string]*PnLTracking),
	}, nil
}

// Run 运行仓位管理主循环
func (pm *PositionManager) Run() error {
	pm.isRunning = true
	log.Printf("🚀 [%s] 仓位管理系统启动", pm.name)
	log.Printf("💰 初始余额: %.2f USDT", pm.initialBalance)
	log.Printf("⚙️  扫描间隔: %v", pm.config.ScanInterval)
	log.Println("📊 只管理现有仓位，不会开新仓")

	ticker := time.NewTicker(pm.config.ScanInterval)
	defer ticker.Stop()

	// 首次立即执行
	if err := pm.runCycle(); err != nil {
		log.Printf("❌ 执行失败: %v", err)
	}

	for pm.isRunning {
		select {
		case <-ticker.C:
			if err := pm.runCycle(); err != nil {
				log.Printf("❌ 执行失败: %v", err)
			}
		}
	}

	return nil
}

// Stop 停止仓位管理
func (pm *PositionManager) Stop() {
	pm.isRunning = false
	log.Printf("⏹ [%s] 仓位管理系统停止", pm.name)
}

// runCycle 运行一个管理周期
func (pm *PositionManager) runCycle() error {
	pm.callCount++

	log.Printf("%s", "\n"+strings.Repeat("=", 70))
	log.Printf("⏰ %s - [%s] 仓位管理周期 #%d", time.Now().Format("2006-01-02 15:04:05"), pm.name, pm.callCount)
	log.Printf("%s", strings.Repeat("=", 70))

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. 获取当前持仓
	positions, err := pm.trader.GetPositions()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取持仓失败: %v", err)
		pm.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 如果没有持仓，跳过本周期
	if len(positions) == 0 {
		log.Println("📭 当前无持仓，跳过本周期")
		record.ExecutionLog = append(record.ExecutionLog, "无持仓，跳过")
		pm.decisionLogger.LogDecision(record)
		return nil
	}

	log.Printf("📊 当前持仓数量: %d", len(positions))

	// 2. 构建交易上下文
	ctx, err := pm.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		pm.decisionLogger.LogDecision(record)
		return fmt.Errorf("构建交易上下文失败: %w", err)
	}

	// 保存账户状态快照
	record.AccountState = logger.AccountSnapshot{
		TotalBalance:          ctx.Account.TotalEquity,
		AvailableBalance:      ctx.Account.AvailableBalance,
		TotalUnrealizedProfit: ctx.Account.TotalPnL,
		PositionCount:         ctx.Account.PositionCount,
		MarginUsedPct:         ctx.Account.MarginUsedPct,
	}

	// 保存持仓快照
	for _, pos := range ctx.Positions {
		record.Positions = append(record.Positions, logger.PositionSnapshot{
			Symbol:           pos.Symbol,
			Side:             pos.Side,
			PositionAmt:      pos.Quantity,
			EntryPrice:       pos.EntryPrice,
			MarkPrice:        pos.MarkPrice,
			UnrealizedProfit: pos.UnrealizedPnL,
			Leverage:         float64(pos.Leverage),
			LiquidationPrice: pos.LiquidationPrice,
		})
	}

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 3. 调用AI获取仓位管理决策
	log.Println("🤖 正在请求AI分析仓位并决策...")
	fullDecision, err := pm.getPositionManagementDecision(ctx)

	// 保存思维链和决策
	if fullDecision != nil {
		record.InputPrompt = fullDecision.UserPrompt
		record.CoTTrace = fullDecision.CoTTrace
		if len(fullDecision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(fullDecision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", err)
		if fullDecision != nil && fullDecision.CoTTrace != "" {
			log.Printf("%s", "\n"+strings.Repeat("-", 70))
			log.Println("💭 AI思维链分析（错误情况）:")
			log.Println(strings.Repeat("-", 70))
			log.Println(fullDecision.CoTTrace)
			log.Printf("%s", strings.Repeat("-", 70)+"\n")
		}
		pm.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取AI决策失败: %w", err)
	}

	// 4. 打印AI思维链
	log.Printf("%s", "\n"+strings.Repeat("-", 70))
	log.Println("💭 AI思维链分析:")
	log.Println(strings.Repeat("-", 70))
	log.Println(fullDecision.CoTTrace)
	log.Printf("%s", strings.Repeat("-", 70)+"\n")

	// 5. 打印AI决策
	log.Printf("📋 AI决策列表 (%d 个):\n", len(fullDecision.Decisions))
	for i, d := range fullDecision.Decisions {
		log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
	}
	log.Println()

	// 6. 执行决策
	for _, d := range fullDecision.Decisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := pm.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 7. 保存决策记录
	if err := pm.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// buildTradingContext 构建交易上下文（只包含现有持仓）
func (pm *PositionManager) buildTradingContext() (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := pm.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	totalWalletBalance := balance["totalWalletBalance"].(float64)
	totalUnrealizedProfit := balance["totalUnrealizedProfit"].(float64)
	availableBalance := balance["availableBalance"].(float64)
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. 获取持仓信息
	positions, err := pm.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		pnlPct := 0.0
		if side == "long" {
			pnlPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			pnlPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		if _, exists := pm.positionFirstSeenTime[posKey]; !exists {
			pm.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		}
		updateTime := pm.positionFirstSeenTime[posKey]

		invalidationCondition := pm.positionInvalidationConditions[symbol]

		if _, exists := pm.positionPnLTracking[posKey]; !exists {
			// 首次看到这个仓位，初始化追踪数据
			tracking := &PnLTracking{
				MaxProfitPct:      pnlPct,
				MaxLossPct:        pnlPct,
				Stage:             1,
				RemainingQuantity: 1.0, // 100%
				EntryPrice:        entryPrice,
			}

			// 尝试从交易所读取现有的止盈止损订单
			orders, err := pm.trader.GetOpenOrders(symbol)
			if err != nil {
				log.Printf("⚠️  获取 %s 的委托单失败: %v", symbol, err)
			} else {
				// 解析止盈止损价格
				for _, order := range orders {
					orderType, _ := order["type"].(string)
					stopPrice, _ := order["stopPrice"].(float64)
					triggerPx, _ := order["triggerPx"].(float64) // Hyperliquid使用triggerPx

					// 币安使用stopPrice，Hyperliquid使用triggerPx
					if stopPrice == 0 && triggerPx > 0 {
						stopPrice = triggerPx
					}

					if stopPrice > 0 {
						// 判断是止损还是止盈
						if side == "long" {
							if stopPrice < markPrice {
								// 多头：触发价低于当前价 = 止损
								tracking.StopLossPrice = stopPrice
							} else {
								// 多头：触发价高于当前价 = 止盈
								tracking.TakeProfitPrice = stopPrice
							}
						} else {
							// 空头
							if stopPrice > markPrice {
								// 空头：触发价高于当前价 = 止损
								tracking.StopLossPrice = stopPrice
							} else {
								// 空头：触发价低于当前价 = 止盈
								tracking.TakeProfitPrice = stopPrice
							}
						}
					}

					// 也可以通过订单类型判断（币安）
					if orderType == "STOP_MARKET" || orderType == "STOP" {
						if side == "long" && stopPrice < markPrice {
							tracking.StopLossPrice = stopPrice
						} else if side == "short" && stopPrice > markPrice {
							tracking.StopLossPrice = stopPrice
						}
					} else if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
						if side == "long" && stopPrice > markPrice {
							tracking.TakeProfitPrice = stopPrice
						} else if side == "short" && stopPrice < markPrice {
							tracking.TakeProfitPrice = stopPrice
						}
					}
				}

				if tracking.StopLossPrice > 0 || tracking.TakeProfitPrice > 0 {
					log.Printf("📋 [%s %s] 读取到现有订单 - 止损: %.4f, 止盈: %.4f",
						symbol, side, tracking.StopLossPrice, tracking.TakeProfitPrice)
				}
			}

			pm.positionPnLTracking[posKey] = tracking
		}
		tracking := pm.positionPnLTracking[posKey]

		if pnlPct > tracking.MaxProfitPct {
			tracking.MaxProfitPct = pnlPct
		}
		if pnlPct < tracking.MaxLossPct {
			tracking.MaxLossPct = pnlPct
		}

		drawdownFromPeakPct := 0.0
		if tracking.MaxProfitPct > 0 {
			drawdownFromPeakPct = tracking.MaxProfitPct - pnlPct
		}

		openingReason := pm.positionReasonings[symbol]

		positionInfos = append(positionInfos, decision.PositionInfo{
			Symbol:                symbol,
			Side:                  side,
			EntryPrice:            entryPrice,
			MarkPrice:             markPrice,
			Quantity:              quantity,
			Leverage:              leverage,
			UnrealizedPnL:         unrealizedPnl,
			UnrealizedPnLPct:      pnlPct,
			LiquidationPrice:      liquidationPrice,
			MarginUsed:            marginUsed,
			UpdateTime:            updateTime,
			InvalidationCondition: invalidationCondition,
			Reasoning:             openingReason,
			MaxProfitPct:          tracking.MaxProfitPct,
			MaxLossPct:            tracking.MaxLossPct,
			DrawdownFromPeakPct:   drawdownFromPeakPct,
			StopLossPrice:         tracking.StopLossPrice,
			TakeProfitPrice:       tracking.TakeProfitPrice,
		})
	}

	// 清理已平仓的持仓记录
	for key := range pm.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(pm.positionFirstSeenTime, key)
			delete(pm.positionPnLTracking, key)
		}
	}

	currentSymbols := make(map[string]bool)
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		currentSymbols[symbol] = true
	}
	for symbol := range pm.positionInvalidationConditions {
		if !currentSymbols[symbol] {
			delete(pm.positionInvalidationConditions, symbol)
		}
	}
	for symbol := range pm.positionReasonings {
		if !currentSymbols[symbol] {
			delete(pm.positionReasonings, symbol)
		}
	}

	totalPnL := totalEquity - pm.initialBalance
	totalPnLPct := 0.0
	if pm.initialBalance > 0 {
		totalPnLPct = (totalPnL / pm.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	ctx := &decision.Context{
		CurrentTime:         time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:      int(time.Since(pm.startTime).Minutes()),
		CallCount:           pm.callCount,
		BTCETHLeverage:      pm.config.BTCETHLeverage,
		AltcoinLeverage:     pm.config.AltcoinLeverage,
		ScanIntervalMinutes: pm.config.ScanIntervalMinutes,
		Account: decision.AccountInfo{
			TotalEquity:      totalEquity,
			AvailableBalance: availableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			MarginUsed:       totalMarginUsed,
			MarginUsedPct:    marginUsedPct,
			PositionCount:    len(positionInfos),
		},
		Positions:      positionInfos,
		CandidateCoins: []decision.CandidateCoin{}, // 仓位管理器不需要候选币种
	}

	return ctx, nil
}

// getPositionManagementDecision 获取仓位管理决策（专用prompt）
func (pm *PositionManager) getPositionManagementDecision(ctx *decision.Context) (*decision.FullDecision, error) {
	// 1. 为所有持仓币种获取市场数据
	ctx.MarketDataMap = make(map[string]*market.Data)
	for _, pos := range ctx.Positions {
		data, err := market.Get(pos.Symbol, ctx.ScanIntervalMinutes)
		if err != nil {
			log.Printf("⚠️ 获取%s市场数据失败: %v", pos.Symbol, err)
			continue
		}
		ctx.MarketDataMap[pos.Symbol] = data
	}

	// 2. 构建专用的System Prompt
	systemPrompt := pm.buildPositionManagementSystemPrompt(ctx.Account.TotalEquity)

	// 3. 构建User Prompt
	userPrompt := pm.buildPositionManagementUserPrompt(ctx)

	// 4. 调用AI API
	var aiResponse string
	var err error

	log.Printf("📝 正在调用AI API（仓位管理模式）")
	aiResponse, err = pm.mcpClient.CallWithMessages(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	log.Printf("✅ AI API调用成功，响应长度: %d 字符", len(aiResponse))

	// 5. 解析AI响应
	fullDecision, err := pm.parsePositionManagementResponse(aiResponse, ctx.Account.TotalEquity)
	if err != nil {
		responsePreview := aiResponse
		if len(responsePreview) > 500 {
			responsePreview = responsePreview[:500] + "..."
		}
		log.Printf("❌ AI响应解析失败，响应预览:\n%s", responsePreview)
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	fullDecision.Timestamp = time.Now()
	fullDecision.UserPrompt = userPrompt
	return fullDecision, nil
}

// buildPositionManagementSystemPrompt 构建仓位管理专用的System Prompt
func (pm *PositionManager) buildPositionManagementSystemPrompt(accountEquity float64) string {
	var sb strings.Builder

	sb.WriteString("你是专业的仓位管理AI，专注于管理现有持仓。\n")
	sb.WriteString("# 🎯 核心职责: 只管理现有仓位，不开新仓\n\n")

	sb.WriteString("你的任务:\n")
	sb.WriteString("1. 分析每个持仓的K线数据和技术指标\n")
	sb.WriteString("2. 根据市场走势决定是否加仓、减仓、平仓或移动止损\n")
	sb.WriteString("3. 如果没有持仓，直接返回空决策列表\n\n")

	sb.WriteString("# 📊 决策依据\n")
	sb.WriteString("## 1. K线分析\n")
	sb.WriteString("- 趋势延续：价格突破关键阻力/支撑，考虑加仓\n")
	sb.WriteString("- 趋势反转：出现反转信号（吞没、十字星），考虑减仓或平仓\n")
	sb.WriteString("- 震荡整理：价格在区间内波动，考虑移动止损保护利润\n\n")

	sb.WriteString("## 2. 技术指标\n")
	sb.WriteString("- RSI: >70超买考虑减仓，<30超卖考虑加仓（多头）\n")
	sb.WriteString("- MACD: 金叉/死叉确认趋势变化\n")
	sb.WriteString("- 成交量: 放量突破确认趋势，缩量警惕反转\n")
	sb.WriteString("- ADX: >25趋势强劲，<20趋势减弱\n\n")

	sb.WriteString("## 3. 两阶段移动止盈策略\n")
	sb.WriteString("**第一阶段 (固定目标止盈)**:\n")
	sb.WriteString("- 当浮盈达到2R (2倍初始止损距离)时:\n")
	sb.WriteString("  * 使用 decrease_long/short 平仓50%仓位锁定利润\n")
	sb.WriteString("  * 使用 update_loss_profit 将剩余50%仓位的止损移至入场价(保本)\n")
	sb.WriteString("  * 标记进入第二阶段\n\n")
	sb.WriteString("**第二阶段 (移动止盈)**:\n")
	sb.WriteString("- 剩余50%仓位使用超级趋势线作为移动止损:\n")
	sb.WriteString("  * 多头: 止损设在超级趋势支撑位 (Supertrend.SupportLevel)\n")
	sb.WriteString("  * 空头: 止损设在超级趋势阻力位 (Supertrend.ResistanceLevel)\n")
	sb.WriteString("  * 当价格突破超级趋势线时平仓离场\n")
	sb.WriteString("  * 或使用 ATR 移动止损: 止损距离 = 当前价 ± 2*ATR\n\n")
	sb.WriteString("**其他风险管理**:\n")
	sb.WriteString("- 峰值回撤>30%: 考虑减仓或平仓\n")
	sb.WriteString("- 接近止损: 评估是否需要提前离场\n")
	sb.WriteString("- 趋势反转信号: 及时平仓保护利润\n\n")

	sb.WriteString("# 🔧 可用操作\n")
	sb.WriteString("1. **increase_long/short**: 加仓（趋势延续时）\n")
	sb.WriteString("2. **decrease_long/short**: 减仓（部分止盈或风险增加）\n")
	sb.WriteString("3. **close_long/short**: 平仓（趋势反转或达到目标）\n")
	sb.WriteString("4. **update_loss_profit**: 移动止损/止盈（保护利润）\n")
	sb.WriteString("5. **hold**: 继续持有（趋势未变）\n\n")

	sb.WriteString("# 📤 输出格式\n")
	sb.WriteString("**第一步: 思维链分析**\n")
	sb.WriteString("简洁分析每个持仓的市场状态、技术指标和决策理由。\n\n")

	sb.WriteString("**第二步: JSON决策数组**\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString("  {\"symbol\": \"BTCUSDT\", \"action\": \"update_loss_profit\", \"stop_loss\": 95000, \"take_profit\": 105000, \"reasoning\": \"价格已到达1R目标，移动止损至保本价\", \"invalidation_condition\": \"4h close below 94000\"},\n")
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"decrease_long\", \"position_size_usd\": 500, \"reasoning\": \"价格到达2R目标，部分止盈30%\", \"invalidation_condition\": \"none\"},\n")
	sb.WriteString("  {\"symbol\": \"SOLUSDT\", \"action\": \"close_short\", \"reasoning\": \"15m出现反转信号，止损离场\", \"invalidation_condition\": \"none\"}\n")
	sb.WriteString("]\n```\n\n")

	sb.WriteString("**重要**: 如果所有持仓都应该继续持有，返回空数组 `[]`\n")

	return sb.String()
}

// buildPositionManagementUserPrompt 构建仓位管理专用的User Prompt
func (pm *PositionManager) buildPositionManagementUserPrompt(ctx *decision.Context) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	sb.WriteString(fmt.Sprintf("**账户**: 净值%.2f | 余额%.2f | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	sb.WriteString("## 当前持仓\n")
	for i, pos := range ctx.Positions {
		holdingDuration := ""
		if pos.UpdateTime > 0 {
			durationMs := time.Now().UnixMilli() - pos.UpdateTime
			durationMin := durationMs / (1000 * 60)
			if durationMin < 60 {
				holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
			} else {
				durationHour := durationMin / 60
				durationMinRemainder := durationMin % 60
				holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
			}
		}

		// 获取止盈阶段信息
		posKey := pos.Symbol + "_" + pos.Side
		stage := 1
		remainingPct := 100.0
		if tracking, exists := pm.positionPnLTracking[posKey]; exists {
			stage = tracking.Stage
			if stage == 0 {
				stage = 1 // 默认第一阶段
			}
			remainingPct = tracking.RemainingQuantity * 100
			if remainingPct == 0 {
				remainingPct = 100.0
			}
		}

		stageInfo := fmt.Sprintf("阶段%d", stage)
		if stage == 2 {
			stageInfo = fmt.Sprintf("阶段2(已部分止盈,剩余%.0f%%)", remainingPct)
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f | 当前价%.4f | 盈亏%+.2f%% | 杠杆%dx | %s%s\n",
			i+1, pos.Symbol, strings.ToUpper(pos.Side),
			pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
			pos.Leverage, stageInfo, holdingDuration))

		sb.WriteString(fmt.Sprintf("   止损价%.4f | 止盈价%.4f | 最高盈利%+.2f%% | 峰值回撤%+.2f%%\n",
			pos.StopLossPrice, pos.TakeProfitPrice, pos.MaxProfitPct, pos.DrawdownFromPeakPct))

		if pos.InvalidationCondition != "" {
			sb.WriteString(fmt.Sprintf("   **离场条件**: %s\n", pos.InvalidationCondition))
		}

		// 添加超级趋势信息
		if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
			if marketData.Timeframe4h != nil && marketData.Timeframe4h.Supertrend != nil {
				st := marketData.Timeframe4h.Supertrend
				sb.WriteString(fmt.Sprintf("   **超级趋势(4h)**: %s | 支撑%.4f | 阻力%.4f\n",
					st.Trend, st.SupportLevel, st.ResistanceLevel))
			}
			if marketData.Timeframe1h != nil && marketData.Timeframe1h.Supertrend != nil {
				st := marketData.Timeframe1h.Supertrend
				sb.WriteString(fmt.Sprintf("   **超级趋势(15m)**: %s | 支撑%.4f | 阻力%.4f\n",
					st.Trend, st.SupportLevel, st.ResistanceLevel))
			}
		}
		sb.WriteString("\n")

		if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
			sb.WriteString(market.Format(marketData))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析每个持仓并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// parsePositionManagementResponse 解析仓位管理AI响应
func (pm *PositionManager) parsePositionManagementResponse(aiResponse string, accountEquity float64) (*decision.FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &decision.FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []decision.Decision{},
		}, fmt.Errorf("提取决策失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	// 验证决策（仓位管理模式：不允许开仓）
	for i, d := range decisions {
		if d.Action == "open_long" || d.Action == "open_short" {
			return &decision.FullDecision{
				CoTTrace:  cotTrace,
				Decisions: decisions,
			}, fmt.Errorf("决策 #%d 错误: 仓位管理模式不允许开仓操作 (%s)", i+1, d.Action)
		}
	}

	return &decision.FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// executeDecisionWithRecord 执行决策并记录
func (pm *PositionManager) executeDecisionWithRecord(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	switch d.Action {
	case "increase_long":
		return pm.executeIncreaseLong(d, actionRecord)
	case "increase_short":
		return pm.executeIncreaseShort(d, actionRecord)
	case "decrease_long":
		return pm.executeDecreaseLong(d, actionRecord)
	case "decrease_short":
		return pm.executeDecreaseShort(d, actionRecord)
	case "close_long":
		return pm.executeCloseLong(d, actionRecord)
	case "close_short":
		return pm.executeCloseShort(d, actionRecord)
	case "update_loss_profit":
		return pm.executeUpdateLossProfit(d, actionRecord)
	case "hold":
		return nil
	default:
		return fmt.Errorf("未知的action: %s", d.Action)
	}
}

// executeIncreaseLong 执行加多仓
func (pm *PositionManager) executeIncreaseLong(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 加多仓: %s", d.Symbol)

	positions, err := pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓信息失败: %w", err)
	}

	hasPosition := false
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol && pos["side"] == "long" {
			hasPosition = true
			break
		}
	}

	if !hasPosition {
		return fmt.Errorf("❌ %s 没有多仓，无法加仓", d.Symbol)
	}

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}

	quantity := d.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	order, err := pm.trader.OpenLong(d.Symbol, quantity, d.Leverage)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 加仓成功，数量: %.4f", quantity)

	// 更新止损止盈
	posKey := d.Symbol + "_long"
	if tracking, exists := pm.positionPnLTracking[posKey]; exists {
		tracking.TakeProfitPrice = d.TakeProfit
		tracking.StopLossPrice = d.StopLoss
	}

	pm.positionInvalidationConditions[d.Symbol] = d.InvalidationCondition

	if err := pm.trader.CancelAllOrders(d.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈止损失败: %v", err)
	}

	positions, err = pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取加仓后持仓失败: %w", err)
	}

	var totalQuantity float64
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol && pos["side"] == "long" {
			totalQuantity = pos["positionAmt"].(float64)
			break
		}
	}

	if err := pm.trader.SetStopLoss(d.Symbol, "LONG", totalQuantity, d.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := pm.trader.SetTakeProfit(d.Symbol, "LONG", totalQuantity, d.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	return nil
}

// executeIncreaseShort 执行加空仓
func (pm *PositionManager) executeIncreaseShort(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 加空仓: %s", d.Symbol)

	positions, err := pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓信息失败: %w", err)
	}

	hasPosition := false
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol && pos["side"] == "short" {
			hasPosition = true
			break
		}
	}

	if !hasPosition {
		return fmt.Errorf("❌ %s 没有空仓，无法加仓", d.Symbol)
	}

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}

	quantity := d.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	order, err := pm.trader.OpenShort(d.Symbol, quantity, d.Leverage)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 加仓成功，数量: %.4f", quantity)

	posKey := d.Symbol + "_short"
	if tracking, exists := pm.positionPnLTracking[posKey]; exists {
		tracking.TakeProfitPrice = d.TakeProfit
		tracking.StopLossPrice = d.StopLoss
	}

	pm.positionInvalidationConditions[d.Symbol] = d.InvalidationCondition

	if err := pm.trader.CancelAllOrders(d.Symbol); err != nil {
		log.Printf("  ⚠ 取消旧止盈止损失败: %v", err)
	}

	positions, err = pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取加仓后持仓失败: %w", err)
	}

	var totalQuantity float64
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol && pos["side"] == "short" {
			totalQuantity = pos["positionAmt"].(float64)
			if totalQuantity < 0 {
				totalQuantity = -totalQuantity
			}
			break
		}
	}

	if err := pm.trader.SetStopLoss(d.Symbol, "SHORT", totalQuantity, d.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if err := pm.trader.SetTakeProfit(d.Symbol, "SHORT", totalQuantity, d.TakeProfit); err != nil {
		log.Printf("  ⚠ 设置止盈失败: %v", err)
	}

	return nil
}

// executeDecreaseLong 执行减多仓
func (pm *PositionManager) executeDecreaseLong(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 减多仓: %s", d.Symbol)

	positions, err := pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓信息失败: %w", err)
	}

	var currentQuantity float64
	hasPosition := false
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol && pos["side"] == "long" {
			currentQuantity = pos["positionAmt"].(float64)
			hasPosition = true
			break
		}
	}

	if !hasPosition {
		return fmt.Errorf("❌ %s 没有多仓，无法减仓", d.Symbol)
	}

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}

	decreaseQuantity := d.PositionSizeUSD / marketData.CurrentPrice

	if decreaseQuantity >= currentQuantity {
		return fmt.Errorf("❌ 减仓数量(%.4f)不能大于等于当前持仓(%.4f)，请使用close_long完全平仓", decreaseQuantity, currentQuantity)
	}

	actionRecord.Quantity = decreaseQuantity
	actionRecord.Price = marketData.CurrentPrice

	order, err := pm.trader.CloseLong(d.Symbol, decreaseQuantity)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 减仓成功，数量: %.4f (剩余: %.4f)", decreaseQuantity, currentQuantity-decreaseQuantity)

	// 更新止盈阶段信息
	posKey := d.Symbol + "_long"
	if tracking, exists := pm.positionPnLTracking[posKey]; exists {
		remainingQuantity := currentQuantity - decreaseQuantity
		remainingPct := remainingQuantity / currentQuantity
		tracking.RemainingQuantity = remainingPct
		tracking.PartialTakenAt = marketData.CurrentPrice

		// 如果减仓约50%，标记进入第二阶段
		if remainingPct >= 0.4 && remainingPct <= 0.6 && tracking.Stage == 1 {
			tracking.Stage = 2
			log.Printf("  📊 进入第二阶段移动止盈 (剩余仓位: %.0f%%)", remainingPct*100)
		}
	}

	return nil
}

// executeDecreaseShort 执行减空仓
func (pm *PositionManager) executeDecreaseShort(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 减空仓: %s", d.Symbol)

	positions, err := pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓信息失败: %w", err)
	}

	var currentQuantity float64
	hasPosition := false
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol && pos["side"] == "short" {
			currentQuantity = pos["positionAmt"].(float64)
			if currentQuantity < 0 {
				currentQuantity = -currentQuantity
			}
			hasPosition = true
			break
		}
	}

	if !hasPosition {
		return fmt.Errorf("❌ %s 没有空仓，无法减仓", d.Symbol)
	}

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}

	decreaseQuantity := d.PositionSizeUSD / marketData.CurrentPrice

	if decreaseQuantity >= currentQuantity {
		return fmt.Errorf("❌ 减仓数量(%.4f)不能大于等于当前持仓(%.4f)，请使用close_short完全平仓", decreaseQuantity, currentQuantity)
	}

	actionRecord.Quantity = decreaseQuantity
	actionRecord.Price = marketData.CurrentPrice

	order, err := pm.trader.CloseShort(d.Symbol, decreaseQuantity)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 减仓成功，数量: %.4f (剩余: %.4f)", decreaseQuantity, currentQuantity-decreaseQuantity)

	// 更新止盈阶段信息
	posKey := d.Symbol + "_short"
	if tracking, exists := pm.positionPnLTracking[posKey]; exists {
		remainingQuantity := currentQuantity - decreaseQuantity
		remainingPct := remainingQuantity / currentQuantity
		tracking.RemainingQuantity = remainingPct
		tracking.PartialTakenAt = marketData.CurrentPrice

		// 如果减仓约50%，标记进入第二阶段
		if remainingPct >= 0.4 && remainingPct <= 0.6 && tracking.Stage == 1 {
			tracking.Stage = 2
			log.Printf("  📊 进入第二阶段移动止盈 (剩余仓位: %.0f%%)", remainingPct*100)
		}
	}

	return nil
}

// executeCloseLong 执行平多仓
func (pm *PositionManager) executeCloseLong(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", d.Symbol)

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	order, err := pm.trader.CloseLong(d.Symbol, 0)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeCloseShort 执行平空仓
func (pm *PositionManager) executeCloseShort(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", d.Symbol)

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	order, err := pm.trader.CloseShort(d.Symbol, 0)
	if err != nil {
		return err
	}

	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeUpdateLossProfit 执行更新止盈止损
func (pm *PositionManager) executeUpdateLossProfit(d *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 更新止盈止损: %s", d.Symbol)

	positions, err := pm.trader.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓信息失败: %w", err)
	}

	var position map[string]interface{}
	var positionSide string
	for _, pos := range positions {
		if pos["symbol"] == d.Symbol {
			position = pos
			positionSide = pos["side"].(string)
			break
		}
	}

	if position == nil {
		return fmt.Errorf("❌ %s 没有持仓，无法更新止盈止损", d.Symbol)
	}

	quantity, ok := position["positionAmt"].(float64)
	if !ok {
		return fmt.Errorf("无法获取持仓数量")
	}

	marketData, err := market.Get(d.Symbol, pm.config.ScanIntervalMinutes)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	log.Printf("  📊 当前持仓: %s | 当前价格: %.4f | 新止损: %.4f | 新止盈: %.4f",
		strings.ToUpper(positionSide), marketData.CurrentPrice, d.StopLoss, d.TakeProfit)

	if positionSide == "long" {
		if d.TakeProfit <= d.StopLoss {
			return fmt.Errorf("❌ 多头持仓时，止盈价格(%.4f)必须大于止损价格(%.4f)", d.TakeProfit, d.StopLoss)
		}
		if d.StopLoss >= marketData.CurrentPrice {
			return fmt.Errorf("❌ 多头持仓时，止损价格(%.4f)应该低于当前价格(%.4f)", d.StopLoss, marketData.CurrentPrice)
		}
	} else if positionSide == "short" {
		if d.StopLoss <= d.TakeProfit {
			return fmt.Errorf("❌ 空头持仓时，止损价格(%.4f)必须大于止盈价格(%.4f)", d.StopLoss, d.TakeProfit)
		}
		if d.StopLoss <= marketData.CurrentPrice {
			return fmt.Errorf("❌ 空头持仓时，止损价格(%.4f)应该高于当前价格(%.4f)", d.StopLoss, marketData.CurrentPrice)
		}
	}

	if err := pm.trader.CancelAllOrders(d.Symbol); err != nil {
		log.Printf("  ⚠️  取消全部委托订单失败: %v", err)
	}

	positionSideUpper := strings.ToUpper(positionSide)
	if err := pm.trader.SetStopLoss(d.Symbol, positionSideUpper, quantity, d.StopLoss); err != nil {
		return fmt.Errorf("设置新止损失败: %w", err)
	}

	if err := pm.trader.SetTakeProfit(d.Symbol, positionSideUpper, quantity, d.TakeProfit); err != nil {
		return fmt.Errorf("设置新止盈失败: %w", err)
	}

	posKey := d.Symbol + "_" + strings.ToLower(positionSide)
	if tracking, exists := pm.positionPnLTracking[posKey]; exists {
		tracking.StopLossPrice = d.StopLoss
		tracking.TakeProfitPrice = d.TakeProfit
		log.Printf("  ✅ 止盈止损更新成功 - 新止损: %.4f, 新止盈: %.4f", d.StopLoss, d.TakeProfit)
	}

	pm.positionInvalidationConditions[d.Symbol] = d.InvalidationCondition

	return nil
}

// GetID 获取管理器ID
func (pm *PositionManager) GetID() string {
	return pm.id
}

// GetName 获取管理器名称
func (pm *PositionManager) GetName() string {
	return pm.name
}

// GetAIModel 获取AI模型
func (pm *PositionManager) GetAIModel() string {
	return pm.aiModel
}

// GetStatus 获取状态
func (pm *PositionManager) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"manager_id":      pm.id,
		"manager_name":    pm.name,
		"ai_model":        pm.aiModel,
		"exchange":        pm.exchange,
		"is_running":      pm.isRunning,
		"start_time":      pm.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(pm.startTime).Minutes()),
		"call_count":      pm.callCount,
		"initial_balance": pm.initialBalance,
		"scan_interval":   pm.config.ScanInterval.String(),
	}
}

// 以下是从decision包复制的辅助函数

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		return strings.TrimSpace(response[:jsonStart])
	}
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]decision.Decision, error) {
	arrayStart := strings.Index(response, "[")
	if arrayStart == -1 {
		preview := response
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("无法找到JSON数组起始，响应内容: %s", preview)
	}

	arrayEnd := findMatchingBracket(response, arrayStart)
	if arrayEnd == -1 {
		return nil, fmt.Errorf("无法找到JSON数组结束")
	}

	jsonContent := strings.TrimSpace(response[arrayStart : arrayEnd+1])
	jsonContent = fixMissingQuotes(jsonContent)

	var decisions []decision.Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
}

// findMatchingBracket 查找匹配的右括号
func findMatchingBracket(s string, start int) int {
	if start >= len(s) || s[start] != '[' {
		return -1
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// fixMissingQuotes 替换中文引号为英文引号
func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '
	return jsonStr
}
