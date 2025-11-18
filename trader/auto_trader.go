package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"strings"
	"time"
)

// AutoTraderConfig 自动交易配置（简化版 - AI全权决策）
type AutoTraderConfig struct {
	// Trader标识
	ID      string // Trader唯一标识（用于日志目录等）
	Name    string // Trader显示名称
	AIModel string // AI模型: "qwen" 或 "deepseek"

	// 截图功能配置（仅Gemini支持）
	EnableScreenshot bool // 是否启用图表截图功能

	// 交易平台选择
	Exchange string // "binance", "hyperliquid" 或 "aster"

	// 币安API配置
	BinanceAPIKey    string
	BinanceSecretKey string

	// Hyperliquid配置
	HyperliquidPrivateKey string
	HyperliquidWalletAddr string
	HyperliquidTestnet    bool

	// Aster配置
	AsterUser       string // Aster主钱包地址
	AsterSigner     string // Aster API钱包地址
	AsterPrivateKey string // Aster API钱包私钥

	CoinPoolAPIURL string

	// AI配置
	UseQwen     bool
	DeepSeekKey string
	QwenKey     string
	GeminiKey   string

	// 自定义AI API配置
	CustomAPIURL    string
	CustomAPIKey    string
	CustomModelName string

	// 扫描配置
	ScanInterval        time.Duration // 扫描间隔（建议3分钟）
	ScanIntervalMinutes int           // 扫描间隔分钟数

	// 账户配置
	InitialBalance float64 // 初始金额（用于计算盈亏，需手动设置）

	// 杠杆配置
	BTCETHLeverage  int // BTC和ETH的杠杆倍数
	AltcoinLeverage int // 山寨币的杠杆倍数

	// 移动止盈配置
	EnableTrailingStop     bool    // 是否启用移动止盈
	TrailingStopDistance   float64 // 移动止盈距离（从峰值回撤百分比）
	TrailingStopActivation float64 // 移动止盈激活条件（盈利达到多少时触发）

	// 分仓止盈配置（基于AI给出的止盈价格）
	EnablePartialTakeProfit bool // 是否启用分仓止盈（50%目标平50%仓位，100%目标平剩余50%）

	// 幂级避让配置（指数退避）
	EnableExponentialBackoff bool    // 是否启用幂级避让
	BackoffBaseMinutes       int     // 基础休息时间（分钟）
	BackoffMultiplier        float64 // 倍数
	BackoffMaxMinutes        int     // 最大休息时间（分钟）
	BackoffResetHours        int     // 重置计数器的时间（小时）

	// 风险控制（仅作为提示，AI可自主决定）
	MaxDailyLoss    float64       // 最大日亏损百分比（提示）
	MaxDrawdown     float64       // 最大回撤百分比（提示）
	StopTradingTime time.Duration // 触发风控后暂停时长
}

// AutoTrader 自动交易器
type AutoTrader struct {
	id                             string // Trader唯一标识
	name                           string // Trader显示名称
	aiModel                        string // AI模型名称
	exchange                       string // 交易平台名称
	enableScreenshot               bool   // 是否启用截图功能
	config                         AutoTraderConfig
	trader                         Trader // 使用Trader接口（支持多平台）
	mcpClient                      *mcp.Client
	decisionLogger                 *logger.DecisionLogger // 决策日志记录器
	initialBalance                 float64
	dailyPnL                       float64
	lastResetTime                  time.Time
	stopUntil                      time.Time
	isRunning                      bool
	startTime                      time.Time               // 系统启动时间
	callCount                      int                     // AI调用次数
	positionFirstSeenTime          map[string]int64        // 持仓首次出现时间 (symbol_side -> timestamp毫秒)
	positionInvalidationConditions map[string]string       // 持仓离场条件 (symbol -> invalidation_condition)
	positionReasonings             map[string]string       // 持仓开仓理由 (symbol -> opening_reason)
	positionPnLTracking            map[string]*PnLTracking // 持仓盈亏跟踪 (symbol_side -> PnL tracking)
	stopLossCount                  int                     // 止损次数计数器
	lastStopLossTime               time.Time               // 最后一次止损时间
	backoffUntil                   time.Time               // 幂级避让暂停到的时间
	lastTotalEquity                float64                 // 上一次的账户总额（用于检测亏损）
}

// PnLTracking 持仓盈亏跟踪数据
type PnLTracking struct {
	MaxProfitPct          float64 // 最大盈利百分比
	MaxLossPct            float64 // 最大亏损百分比（负数）
	TakeProfitPrice       float64 // AI设置的止盈价格
	StopLossPrice         float64 // AI设置的止损价格
	EntryPrice            float64 // 开仓价格
	PartialTP50Executed   bool    // 是否已执行50%止盈
	PartialTP100Executed  bool    // 是否已执行100%止盈
	TrailingStopActivated bool    // 移动止盈是否已激活（一旦激活就持续跟踪）
}

// NewAutoTrader 创建自动交易器
func NewAutoTrader(config AutoTraderConfig) (*AutoTrader, error) {
	// 设置默认值
	if config.ID == "" {
		config.ID = "default_trader"
	}
	if config.Name == "" {
		config.Name = "Default Trader"
	}
	if config.AIModel == "" {
		if config.UseQwen {
			config.AIModel = "qwen"
		} else {
			config.AIModel = "deepseek"
		}
	}

	// 设置幂级避让默认值
	if config.EnableExponentialBackoff {
		if config.BackoffBaseMinutes <= 0 {
			config.BackoffBaseMinutes = 45 // 默认45分钟
		}
		if config.BackoffMultiplier <= 0 {
			config.BackoffMultiplier = 2.67 // 默认2.67倍
		}
		if config.BackoffMaxMinutes <= 0 {
			config.BackoffMaxMinutes = 360 // 默认最大6小时
		}
		if config.BackoffResetHours <= 0 {
			config.BackoffResetHours = 24 // 默认24小时重置
		}
		log.Printf("⚙️  [%s] 幂级避让已启用: 基础%d分钟, 倍数%.2f, 最大%d分钟, %d小时重置",
			config.Name, config.BackoffBaseMinutes, config.BackoffMultiplier,
			config.BackoffMaxMinutes, config.BackoffResetHours)
	}

	mcpClient := mcp.New()

	// 初始化AI
	if config.AIModel == "custom" {
		// 使用自定义API
		mcpClient.SetCustomAPI(config.CustomAPIURL, config.CustomAPIKey, config.CustomModelName)
		log.Printf("🤖 [%s] 使用自定义AI API: %s (模型: %s)", config.Name, config.CustomAPIURL, config.CustomModelName)
	} else if config.AIModel == "gemini" {
		// 使用Gemini
		if err := mcpClient.SetGeminiAPIKey(config.GeminiKey); err != nil {
			return nil, fmt.Errorf("初始化Gemini API失败: %w", err)
		}
		log.Printf("🤖 [%s] 使用Google Gemini AI", config.Name)
		if config.EnableScreenshot {
			log.Printf("📊 [%s] 启用图表截图功能", config.Name)
		}
	} else if config.UseQwen || config.AIModel == "qwen" {
		// 使用Qwen
		mcpClient.SetQwenAPIKey(config.QwenKey, "")
		log.Printf("🤖 [%s] 使用阿里云Qwen AI", config.Name)
	} else {
		// 默认使用DeepSeek
		mcpClient.SetDeepSeekAPIKey(config.DeepSeekKey)
		log.Printf("🤖 [%s] 使用DeepSeek AI", config.Name)
	}

	// 初始化币种池API
	if config.CoinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(config.CoinPoolAPIURL)
	}

	// 设置默认交易平台
	if config.Exchange == "" {
		config.Exchange = "binance"
	}

	// 根据配置创建对应的交易器
	var trader Trader
	var err error

	switch config.Exchange {
	case "binance":
		log.Printf("🏦 [%s] 使用币安合约交易", config.Name)
		trader = NewFuturesTrader(config.BinanceAPIKey, config.BinanceSecretKey)
	case "hyperliquid":
		log.Printf("🏦 [%s] 使用Hyperliquid交易", config.Name)
		trader, err = NewHyperliquidTrader(config.HyperliquidPrivateKey, config.HyperliquidWalletAddr, config.HyperliquidTestnet)
		if err != nil {
			return nil, fmt.Errorf("初始化Hyperliquid交易器失败: %w", err)
		}
	case "aster":
		log.Printf("🏦 [%s] 使用Aster交易", config.Name)
		trader, err = NewAsterTrader(config.AsterUser, config.AsterSigner, config.AsterPrivateKey)
		if err != nil {
			return nil, fmt.Errorf("初始化Aster交易器失败: %w", err)
		}
	default:
		return nil, fmt.Errorf("不支持的交易平台: %s", config.Exchange)
	}

	// 验证初始金额配置
	if config.InitialBalance <= 0 {
		return nil, fmt.Errorf("初始金额必须大于0，请在配置中设置InitialBalance")
	}

	// 初始化决策日志记录器（使用trader ID创建独立目录）
	logDir := fmt.Sprintf("decision_logs/%s", config.ID)
	decisionLogger := logger.NewDecisionLogger(logDir)

	return &AutoTrader{
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
		lastResetTime:                  time.Now(),
		startTime:                      time.Now(),
		callCount:                      0,
		isRunning:                      false,
		positionFirstSeenTime:          make(map[string]int64),
		positionInvalidationConditions: make(map[string]string),
		positionReasonings:             make(map[string]string),
		positionPnLTracking:            make(map[string]*PnLTracking),
		stopLossCount:                  0,
		lastStopLossTime:               time.Time{},
		backoffUntil:                   time.Time{},
	}, nil
}

// Run 运行自动交易主循环
func (at *AutoTrader) Run() error {
	at.isRunning = true
	log.Println("🚀 AI驱动自动交易系统启动")
	log.Printf("💰 初始余额: %.2f USDT", at.initialBalance)
	log.Printf("⚙️  扫描间隔: %v", at.config.ScanInterval)
	log.Println("🤖 AI将全权决定杠杆、仓位大小、止损止盈等参数")

	ticker := time.NewTicker(at.config.ScanInterval)
	defer ticker.Stop()

	// 首次立即执行
	if err := at.runCycle(); err != nil {
		log.Printf("❌ 执行失败: %v", err)
	}

	for at.isRunning {
		select {
		case <-ticker.C:
			if err := at.runCycle(); err != nil {
				log.Printf("❌ 执行失败: %v", err)
			}
		}
	}

	return nil
}

// Stop 停止自动交易
func (at *AutoTrader) Stop() {
	at.isRunning = false
	log.Println("⏹ 自动交易系统停止")
}

// runCycle 运行一个交易周期（使用AI全权决策）
func (at *AutoTrader) runCycle() error {
	at.callCount++

	log.Printf("%s", "\n"+strings.Repeat("=", 70))
	log.Printf("⏰ %s - AI决策周期 #%d", time.Now().Format("2006-01-02 15:04:05"), at.callCount)
	log.Printf("%s", strings.Repeat("=", 70))

	// 创建决策记录
	record := &logger.DecisionRecord{
		ExecutionLog: []string{},
		Success:      true,
	}

	// 1. 检查幂级避让暂停
	if at.config.EnableExponentialBackoff && time.Now().Before(at.backoffUntil) {
		remaining := time.Until(at.backoffUntil)
		log.Printf("⏸ 幂级避让：暂停交易中（第%d次止损），剩余 %.0f 分钟", at.stopLossCount, remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("幂级避让暂停中（第%d次止损），剩余 %.0f 分钟", at.stopLossCount, remaining.Minutes())
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 2. 检查是否需要停止交易（原有风控）
	if time.Now().Before(at.stopUntil) {
		remaining := time.Until(at.stopUntil)
		log.Printf("⏸ 风险控制：暂停交易中，剩余 %.0f 分钟", remaining.Minutes())
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("风险控制暂停中，剩余 %.0f 分钟", remaining.Minutes())
		at.decisionLogger.LogDecision(record)
		return nil
	}

	// 3. 检查是否需要重置止损计数器
	if at.config.EnableExponentialBackoff && !at.lastStopLossTime.IsZero() {
		hoursSinceLastStopLoss := time.Since(at.lastStopLossTime).Hours()
		if hoursSinceLastStopLoss >= float64(at.config.BackoffResetHours) {
			if at.stopLossCount > 0 {
				log.Printf("✅ 幂级避让：距离上次止损已超过%d小时，重置计数器（%d -> 0）", at.config.BackoffResetHours, at.stopLossCount)
				at.stopLossCount = 0
			}
		}
	}

	// 4. 重置日盈亏（每天重置）
	if time.Since(at.lastResetTime) > 24*time.Hour {
		at.dailyPnL = 0
		at.lastResetTime = time.Now()
		log.Println("📅 日盈亏已重置")
	}

	// 5. 收集交易上下文
	ctx, err := at.buildTradingContext()
	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("构建交易上下文失败: %v", err)
		at.decisionLogger.LogDecision(record)
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

	// 保存候选币种列表
	for _, coin := range ctx.CandidateCoins {
		record.CandidateCoins = append(record.CandidateCoins, coin.Symbol)
	}

	log.Printf("📊 账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
		ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)

	// 6. 检查止损触发（用于幂级避让）
	at.checkStopLossTriggered(ctx)

	// 7. 检查移动止盈和分仓止盈（在AI决策之前执行）
	hasExecutedTP := false
	if err := at.checkTrailingStopAndPartialTP(ctx, record, &hasExecutedTP); err != nil {
		log.Printf("⚠️  检查止盈条件失败: %v", err)
	}

	// 8. 如果执行了止盈操作，重新构建交易上下文（确保AI看到最新的持仓数据）
	if hasExecutedTP {
		log.Println("🔄 止盈操作已执行，重新获取最新持仓数据...")
		time.Sleep(2 * time.Second) // 等待交易所更新持仓数据
		ctx, err = at.buildTradingContext()
		if err != nil {
			record.Success = false
			record.ErrorMessage = fmt.Sprintf("重新构建交易上下文失败: %v", err)
			at.decisionLogger.LogDecision(record)
			return fmt.Errorf("重新构建交易上下文失败: %w", err)
		}
		log.Printf("📊 更新后账户净值: %.2f USDT | 可用: %.2f USDT | 持仓: %d",
			ctx.Account.TotalEquity, ctx.Account.AvailableBalance, ctx.Account.PositionCount)
	}

	// 9. 调用AI获取完整决策
	log.Println("🤖 正在请求AI分析并决策...")
	decision, err := decision.GetFullDecision(ctx, at.mcpClient, at.enableScreenshot)

	// 即使有错误，也保存思维链、决策和输入prompt（用于debug）
	if decision != nil {
		record.InputPrompt = decision.UserPrompt
		record.CoTTrace = decision.CoTTrace
		if len(decision.Decisions) > 0 {
			decisionJSON, _ := json.MarshalIndent(decision.Decisions, "", "  ")
			record.DecisionJSON = string(decisionJSON)
		}
	}

	if err != nil {
		record.Success = false
		record.ErrorMessage = fmt.Sprintf("获取AI决策失败: %v", err)

		// 打印AI思维链（即使有错误）
		if decision != nil && decision.CoTTrace != "" {
			log.Printf("%s", "\n"+strings.Repeat("-", 70))
			log.Println("💭 AI思维链分析（错误情况）:")
			log.Println(strings.Repeat("-", 70))
			log.Println(decision.CoTTrace)
			log.Printf("%s", strings.Repeat("-", 70)+"\n")
		}

		at.decisionLogger.LogDecision(record)
		return fmt.Errorf("获取AI决策失败: %w", err)
	}

	// 10. 打印AI思维链
	log.Printf("%s", "\n"+strings.Repeat("-", 70))
	log.Println("💭 AI思维链分析:")
	log.Println(strings.Repeat("-", 70))
	log.Println(decision.CoTTrace)
	log.Printf("%s", strings.Repeat("-", 70)+"\n")

	// 11. 打印AI决策
	log.Printf("📋 AI决策列表 (%d 个):\n", len(decision.Decisions))
	for i, d := range decision.Decisions {
		log.Printf("  [%d] %s: %s - %s", i+1, d.Symbol, d.Action, d.Reasoning)
		if d.Action == "open_long" || d.Action == "open_short" {
			log.Printf("      杠杆: %dx | 仓位: %.2f USDT | 止损: %.4f | 止盈: %.4f",
				d.Leverage, d.PositionSizeUSD, d.StopLoss, d.TakeProfit)
		}
	}
	log.Println()

	// 12. 对决策排序：确保先平仓后开仓（防止仓位叠加超限）
	sortedDecisions := sortDecisionsByPriority(decision.Decisions)

	log.Println("🔄 执行顺序（已优化）: 先平仓→后开仓")
	for i, d := range sortedDecisions {
		log.Printf("  [%d] %s %s", i+1, d.Symbol, d.Action)
	}
	log.Println()

	// 13. 执行决策并记录结果
	for _, d := range sortedDecisions {
		actionRecord := logger.DecisionAction{
			Action:    d.Action,
			Symbol:    d.Symbol,
			Quantity:  0,
			Leverage:  d.Leverage,
			Price:     0,
			Timestamp: time.Now(),
			Success:   false,
		}

		if err := at.executeDecisionWithRecord(&d, &actionRecord); err != nil {
			log.Printf("❌ 执行决策失败 (%s %s): %v", d.Symbol, d.Action, err)
			actionRecord.Error = err.Error()
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("❌ %s %s 失败: %v", d.Symbol, d.Action, err))
		} else {
			actionRecord.Success = true
			record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 成功", d.Symbol, d.Action))
			// 成功执行后短暂延迟
			time.Sleep(1 * time.Second)
		}

		record.Decisions = append(record.Decisions, actionRecord)
	}

	// 14. 保存决策记录
	if err := at.decisionLogger.LogDecision(record); err != nil {
		log.Printf("⚠ 保存决策记录失败: %v", err)
	}

	return nil
}

// checkStopLossTriggered 检查是否有止损单被触发（用于幂级避让）
func (at *AutoTrader) checkStopLossTriggered(ctx *decision.Context) {
	if !at.config.EnableExponentialBackoff {
		return
	}

	// 获取当前持仓的key集合
	currentPositionKeys := make(map[string]bool)
	for _, pos := range ctx.Positions {
		posKey := pos.Symbol + "_" + pos.Side
		currentPositionKeys[posKey] = true
	}

	// 检查账户总额是否减少（用于判断是否亏损）
	currentEquity := ctx.Account.TotalEquity
	equityDecreased := false
	if at.lastTotalEquity > 0 && currentEquity < at.lastTotalEquity {
		equityDecreased = true
	}

	// 检查是否有持仓消失（可能是止损、止盈或手动平仓）
	for posKey := range at.positionPnLTracking {
		if !currentPositionKeys[posKey] {
			tracking := at.positionPnLTracking[posKey]
			if tracking == nil {
				continue
			}

			// 获取该币种的最新价格
			parts := strings.Split(posKey, "_")
			if len(parts) != 2 {
				continue
			}
			symbol := parts[0]
			side := parts[1]

			// 获取市场价格
			marketData, err := market.Get(symbol, 3)
			if err != nil {
				log.Printf("⚠️  获取 %s 市场价格失败: %v", symbol, err)
				continue
			}

			currentPrice := marketData.CurrentPrice
			stopLossPrice := tracking.StopLossPrice
			entryPrice := tracking.EntryPrice

			// 判断是否是止损触发（改进的判断逻辑）
			// 核心思路：如果账户总额减少 + 持仓消失 = 很可能是止损
			isStopLoss := false

			// 条件1：账户总额必须减少（这是止损的核心特征）
			if !equityDecreased {
				log.Printf("   ℹ️  %s 持仓消失但账户总额未减少，可能是止盈或手动平仓，跳过", posKey)
				continue
			}

			// 条件2：检查是否是亏损方向（防止误判止盈为止损）
			// 如果止损价格有效（>0），检查价格是否在止损方向
			if stopLossPrice > 0 {
				priceNearStopLoss := false
				if side == "long" {
					// 多仓：当前价格应该接近或低于止损价（容差5%，因为价格可能已反弹）
					if currentPrice <= stopLossPrice*1.05 {
						priceNearStopLoss = true
					}
				} else {
					// 空仓：当前价格应该接近或高于止损价（容差5%）
					if currentPrice >= stopLossPrice*0.95 {
						priceNearStopLoss = true
					}
				}

				if priceNearStopLoss {
					isStopLoss = true
				} else {
					log.Printf("   ℹ️  %s 价格(%.4f)偏离止损价(%.4f)过远，可能不是止损触发", posKey, currentPrice, stopLossPrice)
				}
			} else {
				// 如果止损价格无效，使用入场价格判断亏损方向
				if entryPrice > 0 {
					isProfitable := false
					if side == "long" {
						isProfitable = currentPrice > entryPrice
					} else {
						isProfitable = currentPrice < entryPrice
					}

					// 如果是盈利方向，不认为是止损
					if isProfitable {
						log.Printf("   ℹ️  %s 当前价格(%.4f)相对入场价(%.4f)是盈利方向，不认为是止损", posKey, currentPrice, entryPrice)
						continue
					} else {
						// 亏损方向 + 账户总额减少 = 很可能是止损
						isStopLoss = true
					}
				} else {
					// 如果连入场价格都没有，只能根据账户总额减少来判断
					// 为了安全起见，认为是止损
					log.Printf("   ⚠️  %s 缺少止损价格和入场价格信息，根据账户总额减少判断为止损", posKey)
					isStopLoss = true
				}
			}

			if isStopLoss {
				at.stopLossCount++
				at.lastStopLossTime = time.Now()

				// 计算休息时间（指数增长）
				backoffMinutes := float64(at.config.BackoffBaseMinutes)
				for i := 1; i < at.stopLossCount; i++ {
					backoffMinutes *= at.config.BackoffMultiplier
				}

				// 限制最大休息时间
				if backoffMinutes > float64(at.config.BackoffMaxMinutes) {
					backoffMinutes = float64(at.config.BackoffMaxMinutes)
				}

				at.backoffUntil = time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

				equityChange := currentEquity - at.lastTotalEquity
				log.Printf("🛑 检测到止损触发（第%d次）：%s", at.stopLossCount, posKey)
				log.Printf("   止损价: %.4f | 当前价: %.4f | 入场价: %.4f", stopLossPrice, currentPrice, entryPrice)
				log.Printf("   账户总额: %.2f -> %.2f (%.2f)", at.lastTotalEquity, currentEquity, equityChange)
				log.Printf("⏸ 幂级避让启动：暂停交易%.0f分钟（至%s）",
					backoffMinutes, at.backoffUntil.Format("15:04:05"))
			}
		}
	}

	// 在检测完所有止损后，更新记录的账户总额（关键：必须在检测之后更新）
	at.lastTotalEquity = currentEquity
}

// checkTrailingStopAndPartialTP 检查移动止盈和分仓止盈条件
func (at *AutoTrader) checkTrailingStopAndPartialTP(ctx *decision.Context, record *logger.DecisionRecord, hasExecuted *bool) error {
	if len(ctx.Positions) == 0 {
		return nil
	}

	for _, pos := range ctx.Positions {
		posKey := pos.Symbol + "_" + pos.Side
		tracking := at.positionPnLTracking[posKey]
		if tracking == nil {
			continue
		}

		// 1. 检查移动止盈
		if at.config.EnableTrailingStop {
			// 检查是否达到激活条件（只需要达到一次，之后就持续跟踪）
			if !tracking.TrailingStopActivated && pos.UnrealizedPnLPct >= at.config.TrailingStopActivation*100 {
				tracking.TrailingStopActivated = true
				log.Printf("✨ [移动止盈激活] %s %s: 盈利%.2f%% 达到激活条件%.2f%%, 开始跟踪峰值",
					pos.Symbol, pos.Side, pos.UnrealizedPnLPct, at.config.TrailingStopActivation*100)
			}

			// 如果已激活，检查是否触发移动止盈（从峰值回撤超过设定距离）
			if tracking.TrailingStopActivated {
				drawdownFromPeak := (tracking.MaxProfitPct - pos.UnrealizedPnLPct) / 100
				if drawdownFromPeak >= at.config.TrailingStopDistance {
					log.Printf("🎯 [移动止盈] %s %s: 盈利%.2f%% (峰值%.2f%%), 回撤%.2f%% >= %.2f%%, 触发移动止盈",
						pos.Symbol, pos.Side, pos.UnrealizedPnLPct, tracking.MaxProfitPct,
						drawdownFromPeak*100, at.config.TrailingStopDistance*100)

					// 执行平仓
					if err := at.executeTrailingStop(&pos, record); err != nil {
						log.Printf("❌ 移动止盈平仓失败: %v", err)
					} else {
						log.Printf("✓ 移动止盈平仓成功")
						*hasExecuted = true
					}
					continue
				}
			}
		}

		// 2. 检查分仓止盈（基于AI给出的止盈价格）
		if at.config.EnablePartialTakeProfit && tracking.TakeProfitPrice > 0 {
			// 计算50%和100%目标价格
			var target50Price, target100Price float64
			if pos.Side == "long" {
				// 多仓：目标价格 > 开仓价格
				priceMove := tracking.TakeProfitPrice - tracking.EntryPrice
				target50Price = tracking.EntryPrice + priceMove*0.5
				target100Price = tracking.TakeProfitPrice
			} else {
				// 空仓：目标价格 < 开仓价格
				priceMove := tracking.EntryPrice - tracking.TakeProfitPrice
				target50Price = tracking.EntryPrice - priceMove*0.5
				target100Price = tracking.TakeProfitPrice
			}

			// 检查50%目标
			if !tracking.PartialTP50Executed {
				reachedTarget := false
				if pos.Side == "long" && pos.MarkPrice >= target50Price {
					reachedTarget = true
				} else if pos.Side == "short" && pos.MarkPrice <= target50Price {
					reachedTarget = true
				}

				if reachedTarget {
					log.Printf("🎯 [分仓止盈50%%] %s %s: 当前价%.4f 达到50%%目标%.4f, 平仓50%%",
						pos.Symbol, pos.Side, pos.MarkPrice, target50Price)

					if err := at.executePartialTakeProfit(&pos, 0.5, record); err != nil {
						log.Printf("❌ 分仓止盈50%%失败: %v", err)
					} else {
						log.Printf("✓ 分仓止盈50%%成功")
						*hasExecuted = true
						tracking.PartialTP50Executed = true
					}
				}
			}

			// 检查100%目标
			if !tracking.PartialTP100Executed {
				reachedTarget := false
				if pos.Side == "long" && pos.MarkPrice >= target100Price {
					reachedTarget = true
				} else if pos.Side == "short" && pos.MarkPrice <= target100Price {
					reachedTarget = true
				}

				if reachedTarget {
					log.Printf("🎯 [分仓止盈100%%] %s %s: 当前价%.4f 达到100%%目标%.4f, 平仓剩余50%%",
						pos.Symbol, pos.Side, pos.MarkPrice, target100Price)

					if err := at.executePartialTakeProfit(&pos, 0.5, record); err != nil {
						log.Printf("❌ 分仓止盈100%%失败: %v", err)
					} else {
						log.Printf("✓ 分仓止盈100%%成功")
						*hasExecuted = true
						tracking.PartialTP100Executed = true
					}
				}
			}
		}
	}

	return nil
}

// executeTrailingStop 执行移动止盈平仓
func (at *AutoTrader) executeTrailingStop(pos *decision.PositionInfo, record *logger.DecisionRecord) error {
	actionRecord := logger.DecisionAction{
		Action:    "trailing_stop_" + pos.Side,
		Symbol:    pos.Symbol,
		Quantity:  pos.Quantity,
		Price:     pos.MarkPrice,
		Timestamp: time.Now(),
		Success:   false,
	}

	var err error
	if pos.Side == "long" {
		_, err = at.trader.CloseLong(pos.Symbol, 0)
	} else {
		_, err = at.trader.CloseShort(pos.Symbol, 0)
	}

	if err != nil {
		actionRecord.Error = err.Error()
		record.Decisions = append(record.Decisions, actionRecord)
		return err
	}

	actionRecord.Success = true
	record.Decisions = append(record.Decisions, actionRecord)
	record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 移动止盈成功", pos.Symbol, pos.Side))
	return nil
}

// executePartialTakeProfit 执行分仓止盈
func (at *AutoTrader) executePartialTakeProfit(pos *decision.PositionInfo, closeRatio float64, record *logger.DecisionRecord) error {
	closeQuantity := pos.Quantity * closeRatio

	actionRecord := logger.DecisionAction{
		Action:    "partial_tp_" + pos.Side,
		Symbol:    pos.Symbol,
		Quantity:  closeQuantity,
		Price:     pos.MarkPrice,
		Timestamp: time.Now(),
		Success:   false,
	}

	var err error
	if pos.Side == "long" {
		_, err = at.trader.CloseLong(pos.Symbol, closeQuantity)
	} else {
		_, err = at.trader.CloseShort(pos.Symbol, closeQuantity)
	}

	if err != nil {
		actionRecord.Error = err.Error()
		record.Decisions = append(record.Decisions, actionRecord)
		return err
	}

	actionRecord.Success = true
	record.Decisions = append(record.Decisions, actionRecord)
	record.ExecutionLog = append(record.ExecutionLog, fmt.Sprintf("✓ %s %s 分仓止盈%.0f%%成功", pos.Symbol, pos.Side, closeRatio*100))
	return nil
}

// buildTradingContext 构建交易上下文
func (at *AutoTrader) buildTradingContext() (*decision.Context, error) {
	// 1. 获取账户信息
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 2. 获取持仓信息
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var positionInfos []decision.PositionInfo
	totalMarginUsed := 0.0

	// 当前持仓的key集合（用于清理已平仓的记录）
	currentPositionKeys := make(map[string]bool)

	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		side := pos["side"].(string)
		entryPrice := pos["entryPrice"].(float64)
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity // 空仓数量为负，转为正数
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		liquidationPrice := pos["liquidationPrice"].(float64)

		// 计算占用保证金（估算）
		leverage := 10 // 默认值，实际应该从持仓信息获取
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed

		// 计算盈亏百分比
		pnlPct := 0.0
		if side == "long" {
			pnlPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			pnlPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		// 跟踪持仓首次出现时间
		posKey := symbol + "_" + side
		currentPositionKeys[posKey] = true
		if _, exists := at.positionFirstSeenTime[posKey]; !exists {
			// 新持仓，记录当前时间
			at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()
		}
		updateTime := at.positionFirstSeenTime[posKey]

		// 获取离场条件（使用symbol作为key，同一币种共享离场条件）
		invalidationCondition := at.positionInvalidationConditions[symbol]

		// 跟踪盈亏统计（最大盈利、最大亏损、回撤）
		if _, exists := at.positionPnLTracking[posKey]; !exists {
			at.positionPnLTracking[posKey] = &PnLTracking{
				MaxProfitPct: pnlPct,
				MaxLossPct:   pnlPct,
			}
		}
		tracking := at.positionPnLTracking[posKey]

		// 更新最大盈利和最大亏损
		if pnlPct > tracking.MaxProfitPct {
			tracking.MaxProfitPct = pnlPct
		}
		if pnlPct < tracking.MaxLossPct {
			tracking.MaxLossPct = pnlPct
		}

		// 计算从峰值的回撤百分比
		drawdownFromPeakPct := 0.0
		if tracking.MaxProfitPct > 0 {
			// 峰值盈利回撤 = 峰值盈利 - 当前盈利
			drawdownFromPeakPct = tracking.MaxProfitPct - pnlPct
		}

		// 获取开仓理由（使用symbol作为key）
		openingReason := at.positionReasonings[symbol]

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
	for key := range at.positionFirstSeenTime {
		if !currentPositionKeys[key] {
			delete(at.positionFirstSeenTime, key)
			delete(at.positionPnLTracking, key) // 同时清理PnL跟踪数据
		}
	}

	// 清理已完全平仓币种的离场条件和开仓理由
	currentSymbols := make(map[string]bool)
	for _, pos := range positions {
		symbol := pos["symbol"].(string)
		currentSymbols[symbol] = true
	}
	for symbol := range at.positionInvalidationConditions {
		if !currentSymbols[symbol] {
			delete(at.positionInvalidationConditions, symbol)
		}
	}
	for symbol := range at.positionReasonings {
		if !currentSymbols[symbol] {
			delete(at.positionReasonings, symbol)
		}
	}

	// 3. 获取合并的候选币种池（AI500 + OI Top，去重）
	// 无论有没有持仓，都分析相同数量的币种（让AI看到所有好机会）
	// AI会根据保证金使用率和现有持仓情况，自己决定是否要换仓
	const ai500Limit = 20 // AI500取前20个评分最高的币种

	// 获取合并后的币种池（AI500 + OI Top）
	mergedPool, err := pool.GetMergedCoinPool(ai500Limit)
	if err != nil {
		return nil, fmt.Errorf("获取合并币种池失败: %w", err)
	}

	// 构建候选币种列表（包含来源信息）
	var candidateCoins []decision.CandidateCoin
	for _, symbol := range mergedPool.AllSymbols {
		sources := mergedPool.SymbolSources[symbol]
		candidateCoins = append(candidateCoins, decision.CandidateCoin{
			Symbol:  symbol,
			Sources: sources, // "ai500" 和/或 "oi_top"
		})
	}

	log.Printf("📋 合并币种池: AI500前%d + OI_Top20 = 总计%d个候选币种",
		ai500Limit, len(candidateCoins))

	// 4. 计算总盈亏
	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	// 5. 分析历史表现（最近100个周期，避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := at.decisionLogger.AnalyzePerformance(100)
	if err != nil {
		log.Printf("⚠️  分析历史表现失败: %v", err)
		// 不影响主流程，继续执行（但设置performance为nil以避免传递错误数据）
		performance = nil
	}

	// 6. 构建上下文
	ctx := &decision.Context{
		CurrentTime:         time.Now().Format("2006-01-02 15:04:05"),
		RuntimeMinutes:      int(time.Since(at.startTime).Minutes()),
		CallCount:           at.callCount,
		BTCETHLeverage:      at.config.BTCETHLeverage,      // 使用配置的杠杆倍数
		AltcoinLeverage:     at.config.AltcoinLeverage,     // 使用配置的杠杆倍数
		ScanIntervalMinutes: at.config.ScanIntervalMinutes, // 使用配置的扫描间隔
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
		CandidateCoins: candidateCoins,
		Performance:    performance, // 添加历史表现分析
	}

	return ctx, nil
}

// executeDecisionWithRecord 执行AI决策并记录详细信息
func (at *AutoTrader) executeDecisionWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	switch decision.Action {
	case "open_long":
		return at.executeOpenLongWithRecord(decision, actionRecord)
	case "open_short":
		return at.executeOpenShortWithRecord(decision, actionRecord)
	case "close_long":
		return at.executeCloseLongWithRecord(decision, actionRecord)
	case "close_short":
		return at.executeCloseShortWithRecord(decision, actionRecord)
	case "hold", "wait":
		// 无需执行，仅记录
		return nil
	default:
		return fmt.Errorf("未知的action: %s", decision.Action)
	}
}

// executeOpenLongWithRecord 执行开多仓并记录详细信息
func (at *AutoTrader) executeOpenLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📈 开多仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "long" {
				return fmt.Errorf("❌ %s 已有多仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_long 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, 3)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// 开仓
	order, err := at.trader.OpenLong(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间和离场条件
	posKey := decision.Symbol + "_long"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置该币种的离场条件和开仓理由（开仓时清空旧条件，设置新条件）
	at.positionInvalidationConditions[decision.Symbol] = decision.InvalidationCondition
	at.positionReasonings[decision.Symbol] = decision.Reasoning

	// 初始化盈亏跟踪（保存止盈价格和止损价格）
	at.positionPnLTracking[posKey] = &PnLTracking{
		TakeProfitPrice:       decision.TakeProfit,
		StopLossPrice:         decision.StopLoss,
		EntryPrice:            marketData.CurrentPrice,
		PartialTP50Executed:   false,
		PartialTP100Executed:  false,
		TrailingStopActivated: false,
	}

	// 设置止损（如果启用分仓止盈，则不设置交易所的止盈单，由系统自动管理）
	if err := at.trader.SetStopLoss(decision.Symbol, "LONG", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if !at.config.EnablePartialTakeProfit {
		// 只有在未启用分仓止盈时才设置交易所的止盈单
		if err := at.trader.SetTakeProfit(decision.Symbol, "LONG", quantity, decision.TakeProfit); err != nil {
			log.Printf("  ⚠ 设置止盈失败: %v", err)
		}
	} else {
		log.Printf("  ℹ️  已启用分仓止盈，将在达到50%%目标(%.4f)和100%%目标(%.4f)时自动平仓",
			marketData.CurrentPrice+(decision.TakeProfit-marketData.CurrentPrice)*0.5,
			decision.TakeProfit)
	}

	return nil
}

// executeOpenShortWithRecord 执行开空仓并记录详细信息
func (at *AutoTrader) executeOpenShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  📉 开空仓: %s", decision.Symbol)

	// ⚠️ 关键：检查是否已有同币种同方向持仓，如果有则拒绝开仓（防止仓位叠加超限）
	positions, err := at.trader.GetPositions()
	if err == nil {
		for _, pos := range positions {
			if pos["symbol"] == decision.Symbol && pos["side"] == "short" {
				return fmt.Errorf("❌ %s 已有空仓，拒绝开仓以防止仓位叠加超限。如需换仓，请先给出 close_short 决策", decision.Symbol)
			}
		}
	}

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, 3)
	if err != nil {
		return err
	}

	// 计算数量
	quantity := decision.PositionSizeUSD / marketData.CurrentPrice
	actionRecord.Quantity = quantity
	actionRecord.Price = marketData.CurrentPrice

	// 开仓
	order, err := at.trader.OpenShort(decision.Symbol, quantity, decision.Leverage)
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 开仓成功，订单ID: %v, 数量: %.4f", order["orderId"], quantity)

	// 记录开仓时间和离场条件
	posKey := decision.Symbol + "_short"
	at.positionFirstSeenTime[posKey] = time.Now().UnixMilli()

	// 设置该币种的离场条件和开仓理由（开仓时清空旧条件，设置新条件）
	at.positionInvalidationConditions[decision.Symbol] = decision.InvalidationCondition
	at.positionReasonings[decision.Symbol] = decision.Reasoning

	// 初始化盈亏跟踪（保存止盈价格和止损价格）
	at.positionPnLTracking[posKey] = &PnLTracking{
		TakeProfitPrice:       decision.TakeProfit,
		StopLossPrice:         decision.StopLoss,
		EntryPrice:            marketData.CurrentPrice,
		PartialTP50Executed:   false,
		PartialTP100Executed:  false,
		TrailingStopActivated: false,
	}

	// 设置止损（如果启用分仓止盈，则不设置交易所的止盈单，由系统自动管理）
	if err := at.trader.SetStopLoss(decision.Symbol, "SHORT", quantity, decision.StopLoss); err != nil {
		log.Printf("  ⚠ 设置止损失败: %v", err)
	}
	if !at.config.EnablePartialTakeProfit {
		// 只有在未启用分仓止盈时才设置交易所的止盈单
		if err := at.trader.SetTakeProfit(decision.Symbol, "SHORT", quantity, decision.TakeProfit); err != nil {
			log.Printf("  ⚠ 设置止盈失败: %v", err)
		}
	} else {
		log.Printf("  ℹ️  已启用分仓止盈，将在达到50%%目标(%.4f)和100%%目标(%.4f)时自动平仓",
			marketData.CurrentPrice-(marketData.CurrentPrice-decision.TakeProfit)*0.5,
			decision.TakeProfit)
	}

	return nil
}

// executeCloseLongWithRecord 执行平多仓并记录详细信息
func (at *AutoTrader) executeCloseLongWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平多仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, 3)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseLong(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// executeCloseShortWithRecord 执行平空仓并记录详细信息
func (at *AutoTrader) executeCloseShortWithRecord(decision *decision.Decision, actionRecord *logger.DecisionAction) error {
	log.Printf("  🔄 平空仓: %s", decision.Symbol)

	// 获取当前价格
	marketData, err := market.Get(decision.Symbol, 3)
	if err != nil {
		return err
	}
	actionRecord.Price = marketData.CurrentPrice

	// 平仓
	order, err := at.trader.CloseShort(decision.Symbol, 0) // 0 = 全部平仓
	if err != nil {
		return err
	}

	// 记录订单ID
	if orderID, ok := order["orderId"].(int64); ok {
		actionRecord.OrderID = orderID
	}

	log.Printf("  ✓ 平仓成功")
	return nil
}

// GetID 获取trader ID
func (at *AutoTrader) GetID() string {
	return at.id
}

// GetName 获取trader名称
func (at *AutoTrader) GetName() string {
	return at.name
}

// GetAIModel 获取AI模型
func (at *AutoTrader) GetAIModel() string {
	return at.aiModel
}

// GetDecisionLogger 获取决策日志记录器
func (at *AutoTrader) GetDecisionLogger() *logger.DecisionLogger {
	return at.decisionLogger
}

// GetStatus 获取系统状态（用于API）
func (at *AutoTrader) GetStatus() map[string]interface{} {
	aiProvider := "DeepSeek"
	if at.config.UseQwen {
		aiProvider = "Qwen"
	}

	return map[string]interface{}{
		"trader_id":       at.id,
		"trader_name":     at.name,
		"ai_model":        at.aiModel,
		"exchange":        at.exchange,
		"is_running":      at.isRunning,
		"start_time":      at.startTime.Format(time.RFC3339),
		"runtime_minutes": int(time.Since(at.startTime).Minutes()),
		"call_count":      at.callCount,
		"initial_balance": at.initialBalance,
		"scan_interval":   at.config.ScanInterval.String(),
		"stop_until":      at.stopUntil.Format(time.RFC3339),
		"last_reset_time": at.lastResetTime.Format(time.RFC3339),
		"ai_provider":     aiProvider,
	}
}

// GetAccountInfo 获取账户信息（用于API）
func (at *AutoTrader) GetAccountInfo() (map[string]interface{}, error) {
	balance, err := at.trader.GetBalance()
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
	}

	// 获取账户字段
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0
	availableBalance := 0.0

	if wallet, ok := balance["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balance["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}
	if avail, ok := balance["availableBalance"].(float64); ok {
		availableBalance = avail
	}

	// Total Equity = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	// 获取持仓计算总保证金
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	totalMarginUsed := 0.0
	totalUnrealizedPnL := 0.0
	for _, pos := range positions {
		markPrice := pos["markPrice"].(float64)
		quantity := pos["positionAmt"].(float64)
		if quantity < 0 {
			quantity = -quantity
		}
		unrealizedPnl := pos["unRealizedProfit"].(float64)
		totalUnrealizedPnL += unrealizedPnl

		leverage := 10
		if lev, ok := pos["leverage"].(float64); ok {
			leverage = int(lev)
		}
		marginUsed := (quantity * markPrice) / float64(leverage)
		totalMarginUsed += marginUsed
	}

	totalPnL := totalEquity - at.initialBalance
	totalPnLPct := 0.0
	if at.initialBalance > 0 {
		totalPnLPct = (totalPnL / at.initialBalance) * 100
	}

	marginUsedPct := 0.0
	if totalEquity > 0 {
		marginUsedPct = (totalMarginUsed / totalEquity) * 100
	}

	return map[string]interface{}{
		// 核心字段
		"total_equity":      totalEquity,           // 账户净值 = wallet + unrealized
		"wallet_balance":    totalWalletBalance,    // 钱包余额（不含未实现盈亏）
		"unrealized_profit": totalUnrealizedProfit, // 未实现盈亏（从API）
		"available_balance": availableBalance,      // 可用余额

		// 盈亏统计
		"total_pnl":            totalPnL,           // 总盈亏 = equity - initial
		"total_pnl_pct":        totalPnLPct,        // 总盈亏百分比
		"total_unrealized_pnl": totalUnrealizedPnL, // 未实现盈亏（从持仓计算）
		"initial_balance":      at.initialBalance,  // 初始余额
		"daily_pnl":            at.dailyPnL,        // 日盈亏

		// 持仓信息
		"position_count":  len(positions),  // 持仓数量
		"margin_used":     totalMarginUsed, // 保证金占用
		"margin_used_pct": marginUsedPct,   // 保证金使用率
	}, nil
}

// GetPositions 获取持仓列表（用于API）
func (at *AutoTrader) GetPositions() ([]map[string]interface{}, error) {
	positions, err := at.trader.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var result []map[string]interface{}
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

		pnlPct := 0.0
		if side == "long" {
			pnlPct = ((markPrice - entryPrice) / entryPrice) * float64(leverage) * 100
		} else {
			pnlPct = ((entryPrice - markPrice) / entryPrice) * float64(leverage) * 100
		}

		marginUsed := (quantity * markPrice) / float64(leverage)

		result = append(result, map[string]interface{}{
			"symbol":             symbol,
			"side":               side,
			"entry_price":        entryPrice,
			"mark_price":         markPrice,
			"quantity":           quantity,
			"leverage":           leverage,
			"unrealized_pnl":     unrealizedPnl,
			"unrealized_pnl_pct": pnlPct,
			"liquidation_price":  liquidationPrice,
			"margin_used":        marginUsed,
		})
	}

	return result, nil
}

// sortDecisionsByPriority 对决策排序：先平仓，再开仓，最后hold/wait
// 这样可以避免换仓时仓位叠加超限
func sortDecisionsByPriority(decisions []decision.Decision) []decision.Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	// 定义优先级
	getActionPriority := func(action string) int {
		switch action {
		case "close_long", "close_short":
			return 1 // 最高优先级：先平仓
		case "open_long", "open_short":
			return 2 // 次优先级：后开仓
		case "hold", "wait":
			return 3 // 最低优先级：观望
		default:
			return 999 // 未知动作放最后
		}
	}

	// 复制决策列表
	sorted := make([]decision.Decision, len(decisions))
	copy(sorted, decisions)

	// 按优先级排序
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if getActionPriority(sorted[i].Action) > getActionPriority(sorted[j].Action) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
