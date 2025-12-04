package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"nofx/chart"
	"nofx/market"
	"nofx/mcp"
	"nofx/pool"
	"os"
	"strings"
	"time"
)

// PositionInfo 持仓信息
type PositionInfo struct {
	Symbol                string  `json:"symbol"`
	Side                  string  `json:"side"` // "long" or "short"
	EntryPrice            float64 `json:"entry_price"`
	MarkPrice             float64 `json:"mark_price"`
	Quantity              float64 `json:"quantity"`
	Leverage              int     `json:"leverage"`
	UnrealizedPnL         float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct      float64 `json:"unrealized_pnl_pct"`
	LiquidationPrice      float64 `json:"liquidation_price"`
	MarginUsed            float64 `json:"margin_used"`
	UpdateTime            int64   `json:"update_time"`                      // 持仓更新时间戳（毫秒）
	InvalidationCondition string  `json:"invalidation_condition,omitempty"` // 开仓时设定的离场条件
	Reasoning             string  `json:"reasoning,omitempty"`              // 开仓理由
	MaxProfitPct          float64 `json:"max_profit_pct"`                   // 最大盈利百分比
	MaxLossPct            float64 `json:"max_loss_pct"`                     // 最大亏损百分比
	DrawdownFromPeakPct   float64 `json:"drawdown_from_peak_pct"`           // 从峰值回撤百分比
	StopLossPrice         float64 `json:"stop_loss_price"`                  // 止损价格
	TakeProfitPrice       float64 `json:"take_profit_price"`                // 止盈价格

}

// AccountInfo 账户信息
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // 账户净值
	AvailableBalance float64 `json:"available_balance"` // 可用余额
	TotalPnL         float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
	MarginUsed       float64 `json:"margin_used"`       // 已用保证金
	MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
	PositionCount    int     `json:"position_count"`    // 持仓数量
}

// CandidateCoin 候选币种（来自币种池）
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // 来源: "ai500" 和/或 "oi_top"
}

// OITopData 持仓量增长Top数据（用于AI决策参考）
type OITopData struct {
	Rank              int     // OI Top排名
	OIDeltaPercent    float64 // 持仓量变化百分比（1小时）
	OIDeltaValue      float64 // 持仓量变化价值
	PriceDeltaPercent float64 // 价格变化百分比
	NetLong           float64 // 净多仓
	NetShort          float64 // 净空仓
}

// Context 交易上下文（传递给AI的完整信息）
type Context struct {
	CurrentTime         string                  `json:"current_time"`
	RuntimeMinutes      int                     `json:"runtime_minutes"`
	CallCount           int                     `json:"call_count"`
	Account             AccountInfo             `json:"account"`
	Positions           []PositionInfo          `json:"positions"`
	CandidateCoins      []CandidateCoin         `json:"candidate_coins"`
	MarketDataMap       map[string]*market.Data `json:"-"` // 不序列化，但内部使用
	OITopDataMap        map[string]*OITopData   `json:"-"` // OI Top数据映射
	Performance         any                     `json:"-"` // 历史表现分析（logger.PerformanceAnalysis）
	BTCETHLeverage      int                     `json:"-"` // BTC/ETH杠杆倍数（从配置读取）
	AltcoinLeverage     int                     `json:"-"` // 山寨币杠杆倍数（从配置读取）
	ScanIntervalMinutes int                     `json:"-"` // 扫描间隔分钟数（从配置读取）
}

// Decision AI的交易决策
type Decision struct {
	Symbol                string  `json:"symbol"`
	Action                string  `json:"action"` // "open_long", "open_short", "close_long", "close_short", "increase_long", "increase_short", "decrease_long", "decrease_short", "hold", "wait"
	Leverage              int     `json:"leverage,omitempty"`
	PositionSizeUSD       float64 `json:"position_size_usd,omitempty"`
	EntryPrice            float64 `json:"entry_price,omitempty"` // 入场价格（开仓/加仓时必填）
	StopLoss              float64 `json:"stop_loss,omitempty"`
	TakeProfit            float64 `json:"take_profit,omitempty"`
	Confidence            int     `json:"confidence,omitempty"` // 信心度 (0-100)
	RiskUSD               float64 `json:"risk_usd,omitempty"`   // 最大美元风险
	Reasoning             string  `json:"reasoning"`
	InvalidationCondition string  `json:"invalidation_condition,omitempty"` // 离场条件
}

// FullDecision AI的完整决策（包含思维链）
type FullDecision struct {
	UserPrompt string     `json:"user_prompt"` // 发送给AI的输入prompt
	CoTTrace   string     `json:"cot_trace"`   // 思维链分析（AI输出）
	Decisions  []Decision `json:"decisions"`   // 具体决策列表
	Timestamp  time.Time  `json:"timestamp"`
}

// GetFullDecision 获取AI的完整交易决策（批量分析所有币种和持仓）
func GetFullDecision(ctx *Context, mcpClient *mcp.Client, enableScreenshot bool) (*FullDecision, error) {
	// 1. 为所有币种获取市场数据
	if err := fetchMarketDataForContext(ctx); err != nil {
		return nil, fmt.Errorf("获取市场数据失败: %w", err)
	}

	// 2. 构建 System Prompt（固定规则）和 User Prompt（动态数据）
	systemPrompt := buildSystemPrompt(ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage, ctx.ScanIntervalMinutes)
	userPrompt := buildUserPrompt(ctx)

	// 3. 生成图表截图（仅在使用Gemini且启用截图时）
	var imageData []byte
	if enableScreenshot && mcpClient.Provider == mcp.ProviderGemini {
		var err error
		imageData, err = generateChartScreenshot(ctx)
		if err != nil {
			log.Printf("⚠️ 生成图表截图失败: %v", err)
			// 截图失败不影响主流程，继续使用文本分析
		} else {
			log.Printf("✅ 图表截图生成成功，大小: %d bytes", len(imageData))

			// 可选：保存截图到本地用于调试（取消注释以启用）
			if err := saveScreenshotForDebug(imageData); err != nil {
				log.Printf("⚠️ 保存调试截图失败: %v", err)
			}

			// 在用户提示中添加图表说明
			userPrompt += "\n\n📊 **图表分析**: 我已为你生成了当前市场的K线图表，包含价格走势、成交量。请结合图表进行趋势和支撑阻力分析。\n"
		}

	}

	// 4. 调用AI API（使用 system + user prompt + 可选图像）
	var aiResponse string
	var err error
	if imageData != nil {
		log.Printf("🖼️ 正在调用AI API（包含图像），图像大小: %d bytes", len(imageData))
		aiResponse, err = mcpClient.CallWithMessagesImage(systemPrompt, userPrompt, imageData)
		if err == nil {
			log.Printf("✅ AI API调用成功（图像模式），响应长度: %d 字符", len(aiResponse))
		}
	} else {
		log.Printf("📝 正在调用AI API（纯文本模式）")
		aiResponse, err = mcpClient.CallWithMessages(systemPrompt, userPrompt)
		if err == nil {
			log.Printf("✅ AI API调用成功（文本模式），响应长度: %d 字符", len(aiResponse))
		}
	}
	if err != nil {
		return nil, fmt.Errorf("调用AI API失败: %w", err)
	}

	// 5. 解析AI响应
	decision, err := parseFullDecisionResponse(aiResponse, ctx.Account.TotalEquity, ctx.BTCETHLeverage, ctx.AltcoinLeverage)
	if err != nil {
		// 记录AI响应的前500个字符用于调试
		responsePreview := aiResponse
		if len(responsePreview) > 500 {
			responsePreview = responsePreview[:500] + "..."
		}
		log.Printf("❌ AI响应解析失败，响应预览:\n%s", responsePreview)
		return nil, fmt.Errorf("解析AI响应失败: %w", err)
	}

	decision.Timestamp = time.Now()
	decision.UserPrompt = userPrompt // 保存输入prompt
	return decision, nil
}

// fetchMarketDataForContext 为上下文中的所有币种获取市场数据和OI数据
func fetchMarketDataForContext(ctx *Context) error {
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.OITopDataMap = make(map[string]*OITopData)

	// 收集所有需要获取数据的币种
	symbolSet := make(map[string]bool)

	// 1. 优先获取持仓币种的数据（这是必须的）
	for _, pos := range ctx.Positions {
		symbolSet[pos.Symbol] = true
	}

	// 2. 候选币种数量根据账户状态动态调整
	maxCandidates := calculateMaxCandidates(ctx)
	for i, coin := range ctx.CandidateCoins {
		if i >= maxCandidates {
			break
		}
		symbolSet[coin.Symbol] = true
	}

	// 并发获取市场数据
	// 持仓币种集合（用于判断是否跳过OI检查）
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	for symbol := range symbolSet {
		data, err := market.Get(symbol, ctx.ScanIntervalMinutes) // 使用配置的扫描间隔
		if err != nil {
			// 单个币种失败不影响整体，只记录错误
			fmt.Printf("获取市场数据失败: %s\n", err)
			continue
		}

		// ⚠️ 流动性过滤：持仓价值低于15M USD的币种不做（多空都不做）
		// 持仓价值 = 持仓量 × 当前价格
		// 但现有持仓必须保留（需要决策是否平仓）
		isExistingPosition := positionSymbols[symbol]
		if !isExistingPosition && data.OpenInterest != nil && data.CurrentPrice > 0 {
			// 计算持仓价值（USD）= 持仓量 × 当前价格
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000 // 转换为百万美元单位
			if oiValueInMillions < 15 {
				log.Printf("⚠️  %s 持仓价值过低(%.2fM USD < 15M)，跳过此币种 [持仓量:%.0f × 价格:%.4f]",
					symbol, oiValueInMillions, data.OpenInterest.Latest, data.CurrentPrice)
				continue
			}
		}

		ctx.MarketDataMap[symbol] = data
	}

	// 加载OI Top数据（不影响主流程）
	oiPositions, err := pool.GetOITopPositions()
	if err == nil {
		for _, pos := range oiPositions {
			// 标准化符号匹配
			symbol := pos.Symbol
			ctx.OITopDataMap[symbol] = &OITopData{
				Rank:              pos.Rank,
				OIDeltaPercent:    pos.OIDeltaPercent,
				OIDeltaValue:      pos.OIDeltaValue,
				PriceDeltaPercent: pos.PriceDeltaPercent,
				NetLong:           pos.NetLong,
				NetShort:          pos.NetShort,
			}
		}
	}

	return nil
}

// calculateMaxCandidates 根据账户状态计算需要分析的候选币种数量
func calculateMaxCandidates(ctx *Context) int {
	// 直接返回候选池的全部币种数量
	// 因为候选池已经在 auto_trader.go 中筛选过了
	// 固定分析前20个评分最高的币种（来自AI500）
	return len(ctx.CandidateCoins)
}

// buildSystemPrompt 构建 System Prompt（固定规则，可缓存）
func buildSystemPrompt(accountEquity float64, btcEthLeverage, altcoinLeverage int, scanIntervalMinutes int) string {
	var sb strings.Builder

	// 计算风险敞口和最大仓位 (基于账户净值)
	maxBtcEthPosition := accountEquity * 0.4 * float64(btcEthLeverage)   // 40% 保证金
	maxAltcoinPosition := accountEquity * 0.4 * float64(altcoinLeverage) // 40% 保证金
	standardRiskUSD := accountEquity * 0.015                             // 标准单笔风险 1.5%

	// === 角色设定与核心约束 ===
	sb.WriteString("你是精英短线交易员(Scalper/Day Trader)，专注于捕捉 15分钟(15m) 级别的爆发性波动。\n")
	sb.WriteString("# 🎯 核心逻辑: [4H定方向] + [15m找形态] + [量化评分决策]\n")
	sb.WriteString(fmt.Sprintf("**关键认知**: 你的系统每%d分钟扫描一次，但交易频率应该极低。\n\n", scanIntervalMinutes))

	// === 硬约束 (风控) ===
	sb.WriteString("# ⚖️ 短线硬约束\n")
	sb.WriteString("1. **盈亏比 (R:R)**: 开仓必须 ≥ 1:2。加仓后，整体 R:R 必须 ≥ 1:2。\n")
	sb.WriteString(fmt.Sprintf("2. **单笔风险**: 单笔交易风险 (risk_usd) 不得超过净值的 5%%，即 **$%.2f**。\n", accountEquity*0.05))
	sb.WriteString(fmt.Sprintf("3. **最大仓位**: BTC/ETH ≤ %.0f U; 山寨币 ≤ %.0f U。\n", maxBtcEthPosition, maxAltcoinPosition))
	sb.WriteString("4. **保证金**: 总使用率 ≤ 90%。\n\n")

	// === 📊 短线狙击评分卡 (核心开仓逻辑) ===
	sb.WriteString("# 🧮 评分卡 (开仓/加仓依据)\n")
	sb.WriteString("总分 < 75，强制 `wait`。分数决定仓位大小。\n")
	// (A, B, C, D 评分项不变，沿用用户提供的内容)
	sb.WriteString("## A. 市场背景 (4H Context) [30分]\n")
	sb.WriteString("- **30分**: 15m 信号与 4H 趋势方向完全一致。\n")
	sb.WriteString("- **15分**: 4H 处于强支撑/阻力位，15m 出现逆势反转信号。\n")
	sb.WriteString("- **0分**: 4H 处于无序震荡中间区域 (No Man's Land)。\n\n")

	sb.WriteString("## B. 价格行为 (15m Price Action) [25分]\n")
	sb.WriteString("- **25分**: 出现明确形态：突破回踩确认、2B法则(假突破反向)、或吞没/启明星K线组合。\n")
	sb.WriteString("- **10分**: 形态一般，但没有破坏结构。\n")
	sb.WriteString("- **0分**: 均线纠缠，K线细碎无序。\n\n")

	sb.WriteString("## C. 动能与成交量 (Momentum & Vol) [25分]\n")
	sb.WriteString("- **25分**: 突破关键位时 RVOL > 1.5 (放量)，或出现明确的 RSI 背离。\n")
	sb.WriteString("- **10分**: 量能平平，但指标方向正确。\n")
	sb.WriteString("- **-100分 (一票否决)**: 15m ADX < 25 (死鱼盘)。\n\n")

	sb.WriteString("## D. 止损优势 (Stop Loss Placement) [20分]\n")
	sb.WriteString("- **20分**: 止损位非常清晰且紧凑，R:R 极佳 (≥ 3:1)。\n")
	sb.WriteString("- **0分**: 找不到合理的止损参考点，或 R:R < 1:2。\n\n")

	// === 优化开仓逻辑 (分数与仓位挂钩) ===
	sb.WriteString("# 🎯 优化开仓逻辑 (Score-to-Position)\n")
	sb.WriteString("开仓仓位 (position_size_usd) 必须与评分和风险严格挂钩：\n")
	sb.WriteString("* **总分 90-100**: Confidence 90+。允许最大仓位的 **50%**。\n")
	sb.WriteString("* **总分 80-89**: Confidence 80-89。允许最大仓位的 **30%** (标准开仓量)。\n")
	sb.WriteString("* **总分 75-79**: Confidence 75-79。**极度谨慎**，只允许最大仓位的 **15%** (试探仓)。\n")
	sb.WriteString("* **总分 < 75**: 强制 `wait`。\n\n")

	// === 优化仓位评估和管理逻辑 (加/减仓) ===
	sb.WriteString("# 📈 优化仓位管理逻辑\n")

	sb.WriteString("## 1. 加仓时机 (increase_long/increase_short)\n")
	sb.WriteString("加仓是为了利用趋势，必须严格遵守以下条件：\n")
	sb.WriteString("1. **浮盈锁定**: 原持仓必须至少浮盈 **1R (1倍风险额)**，且已将**整体止损**推至开仓价之上（保本）。\n")
	sb.WriteString("2. **结构确认**: 价格回踩关键支撑位后，15m 再次出现看涨/看跌的结构确认信号 (如吞没 K 线)。\n")
	sb.WriteString("3. **风险计算**: 加仓后，新的 **整体止损位** 必须能确保 **整体 R:R ≥ 1:2**。\n")
	sb.WriteString("4. **仓位控制**: 单次加仓量**不得超过原仓位量的 50%**，且总仓位不超过单币种上限。\n\n")

	sb.WriteString("## 2. 减仓时机 (decrease_long/decrease_short) 和移动止损 (update_loss_profit)\n")
	sb.WriteString("减仓和移动止损必须遵循分步执行，最大化锁定短线利润：\n")
	sb.WriteString("1. **1R 目标**: 价格到达 **1R 目标位**时，**强制**执行 `update_loss_profit`，将止损移至开仓价。\n")
	sb.WriteString("2. **2R 目标**: 价格到达 **2R 目标位**时，执行 `decrease_long/short`，**平仓 30%-50%**，锁定核心利润。\n")
	sb.WriteString("3. **趋势反转**: 15m 出现明显的顶部/底部形态 (如大型吞没、背离)，或价格跌破/突破新的结构支撑/阻力时，可全部平仓。\n\n")

	// === 决策输出 ===
	sb.WriteString("# 📋 决策流程\n")
	sb.WriteString("1. **4H 结构评估**: 判定市场状态 (Trend/Range)，找到关键支撑/阻力。\n")
	sb.WriteString("2. **仓位评估**: 检查现有持仓是否满足加仓/减仓/移动止损条件。\n")
	sb.WriteString("3. **开仓评估**: 严格执行评分卡，Score < 75 一律放弃。\n")
	sb.WriteString("4. **输出决策**: 思维链分析 + JSON 数组。\n\n")

	// === 输出格式 (更新字段说明) ===
	sb.WriteString("# 📤 输出格式\n")
	sb.WriteString("**第一步: 思维链 (必须包含评分与 R:R 计算)**\n")
	sb.WriteString("简洁分析你的思考过程（必须包含对 4H 趋势、15m 扳机、ADX/RVOL 过滤和评分计算）。\n\n")
	sb.WriteString("**第二步: JSON决策数组**\n")
	sb.WriteString("```json\n[\n")
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"entry_price\": 95000, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": %.0f, \"reasoning\": \"Score 85/100, 下跌趋势+反弹至阻力位\", \"invalidation_condition\": \"4h close above 98000 (trend reversal)\"},\n", btcEthLeverage, maxBtcEthPosition*0.3, standardRiskUSD))
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"SOLUSDT\", \"action\": \"increase_long\", \"leverage\": %d, \"position_size_usd\": %.0f, \"entry_price\": 150.0, \"stop_loss\": 145.5, \"take_profit\": 165.0, \"confidence\": 90, \"risk_usd\": %.0f, \"reasoning\": \"原仓位已浮盈 2R，止损已推至保本。加仓后整体R:R 2.5:1\", \"invalidation_condition\": \"15m close below 145.5\"},\n", altcoinLeverage, maxAltcoinPosition*0.15, standardRiskUSD*0.5))
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"ADAUSDT\", \"action\": \"decrease_short\", \"position_size_usd\": %.0f, \"reasoning\": \"价格到达 2R 目标，部分止盈 30%%\", \"invalidation_condition\": \"none\"},\n", maxAltcoinPosition*0.15))
	sb.WriteString("  {\"symbol\": \"BNBUSDT\", \"action\": \"update_loss_profit\", \"stop_loss\": 590.0, \"take_profit\": 650.0, \"reasoning\": \"价格已到达 1R 目标位，将止损移至开仓价 $590.0 保本\", \"invalidation_condition\": \"4h close below 585\"}\n")
	sb.WriteString("]\n```\n\n")
	sb.WriteString("**关键字段说明 (新增)**:\n")
	sb.WriteString("- `action`: 增加了 `increase_long/short` 和 `decrease_long/short`。\n")
	sb.WriteString("- 加仓时：`stop_loss`/`take_profit`/`entry_price` 必须是**加仓后整体**的平均价格和新的止损止盈位。\n")
	sb.WriteString("- 减仓时：`position_size_usd` 填写**需要减少的金额**。\n")
	sb.WriteString("- `risk_usd`: 仅在 `open` 或 `increase` 时填写，表示本次操作新增的美元风险。\n")

	return sb.String()
}

// buildUserPrompt 构建 User Prompt（动态数据）
func buildUserPrompt(ctx *Context) string {
	var sb strings.Builder

	// 系统状态
	sb.WriteString(fmt.Sprintf("**时间**: %s | **周期**: #%d | **运行**: %d分钟\n\n",
		ctx.CurrentTime, ctx.CallCount, ctx.RuntimeMinutes))

	// BTC 市场
	// if btcData, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
	// 	rsi := 0.0
	// 	if btcData.Timeframe1h != nil {
	// 		rsi = btcData.Timeframe1h.RSI
	// 	}
	// 	sb.WriteString(fmt.Sprintf("**BTC**: %.2f | RSI(1h): %.2f\n\n",
	// 		btcData.CurrentPrice, rsi))
	// }

	// 账户
	sb.WriteString(fmt.Sprintf("**账户**: 净值%.2f | 余额%.2f (%.1f%%) | 盈亏%+.2f%% | 保证金%.1f%% | 持仓%d个\n\n",
		ctx.Account.TotalEquity,
		ctx.Account.AvailableBalance,
		(ctx.Account.AvailableBalance/ctx.Account.TotalEquity)*100,
		ctx.Account.TotalPnLPct,
		ctx.Account.MarginUsedPct,
		ctx.Account.PositionCount))

	// 持仓（完整市场数据）
	if len(ctx.Positions) > 0 {
		sb.WriteString("## 当前持仓\n")
		for i, pos := range ctx.Positions {
			// 计算持仓时长
			holdingDuration := ""
			if pos.UpdateTime > 0 {
				durationMs := time.Now().UnixMilli() - pos.UpdateTime
				durationMin := durationMs / (1000 * 60) // 转换为分钟
				if durationMin < 60 {
					holdingDuration = fmt.Sprintf(" | 持仓时长%d分钟", durationMin)
				} else {
					durationHour := durationMin / 60
					durationMinRemainder := durationMin % 60
					holdingDuration = fmt.Sprintf(" | 持仓时长%d小时%d分钟", durationHour, durationMinRemainder)
				}
			}

			// 显示PnL统计信息
			sb.WriteString(fmt.Sprintf("%d. %s %s | 入场价%.4f | 当前价%.4f | 盈亏%+.2f%% | 杠杆%dx | 保证金%.0f | 强平价%.4f%s | 止损价%.4f | 止盈价%.4f | 最高盈利%+.2f%% | 峰值回撤%+.2f%%\n",
				i+1, pos.Symbol, strings.ToUpper(pos.Side),
				pos.EntryPrice, pos.MarkPrice, pos.UnrealizedPnLPct,
				pos.Leverage, pos.MarginUsed, pos.LiquidationPrice, holdingDuration, pos.StopLossPrice, pos.TakeProfitPrice, pos.MaxProfitPct, pos.DrawdownFromPeakPct))

			// // 显示开仓理由（如果有）
			// if pos.Reasoning != "" {
			// 	sb.WriteString(fmt.Sprintf("**开仓理由**: %s\n", pos.Reasoning))
			// }

			// 显示离场条件（如果有）
			if pos.InvalidationCondition != "" {
				sb.WriteString(fmt.Sprintf("**离场条件**: %s\n", pos.InvalidationCondition))
			}
			sb.WriteString("\n")

			// 使用FormatMarketData输出完整市场数据
			if marketData, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(market.Format(marketData))
				sb.WriteString("\n")
			}
		}
	} else {
		sb.WriteString("**当前持仓**: 无\n\n")
	}

	// 候选币种（完整市场数据）- 排除已在持仓中的币种
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	sb.WriteString(fmt.Sprintf("## 候选币种 (%d个)\n\n", len(ctx.MarketDataMap)))
	displayedCount := 0
	for _, coin := range ctx.CandidateCoins {
		// 跳过已在持仓中的币种（避免重复输出）
		if positionSymbols[coin.Symbol] {
			continue
		}

		marketData, hasData := ctx.MarketDataMap[coin.Symbol]
		if !hasData {
			fmt.Printf("coin: %s 无数据", coin.Symbol)
			continue
		}
		displayedCount++

		sourceTags := ""
		if len(coin.Sources) > 1 {
			sourceTags = " (AI500+OI_Top双重信号)"
		} else if len(coin.Sources) == 1 && coin.Sources[0] == "oi_top" {
			sourceTags = " (OI_Top持仓增长)"
		}

		// 使用FormatMarketData输出完整市场数据
		sb.WriteString(fmt.Sprintf("### %d. %s%s\n\n", displayedCount, coin.Symbol, sourceTags))
		sb.WriteString(market.Format(marketData))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// 历史表现分析（提供更直观的指标）
	if ctx.Performance != nil {
		// 从interface{}中提取关键指标
		type PerformanceData struct {
			TotalTrades   int     `json:"total_trades"`
			WinningTrades int     `json:"winning_trades"`
			LosingTrades  int     `json:"losing_trades"`
			WinRate       float64 `json:"win_rate"`
			AvgWin        float64 `json:"avg_win"`
			AvgLoss       float64 `json:"avg_loss"`
			ProfitFactor  float64 `json:"profit_factor"`
			SharpeRatio   float64 `json:"sharpe_ratio"`
		}
		// var perfData PerformanceData
		// if jsonData, err := json.Marshal(ctx.Performance); err == nil {
		// 	if err := json.Unmarshal(jsonData, &perfData); err == nil {
		// 		sb.WriteString("## 📊 近期表现（最近完成的交易）\n\n")

		// 		if perfData.TotalTrades > 0 {
		// 			sb.WriteString(fmt.Sprintf("- 总交易数: %d笔 (%d胜/%d负)\n",
		// 				perfData.TotalTrades, perfData.WinningTrades, perfData.LosingTrades))
		// 			sb.WriteString(fmt.Sprintf("- 胜率: %.1f%% (目标≥50%%)\n", perfData.WinRate))
		// 			sb.WriteString(fmt.Sprintf("- 平均盈利: +%.2f USDT | 平均亏损: %.2f USDT\n",
		// 				perfData.AvgWin, perfData.AvgLoss))
		// 			sb.WriteString(fmt.Sprintf("- 盈亏比: %.2f (目标≥2.0)\n", perfData.ProfitFactor))
		// 			sb.WriteString(fmt.Sprintf("- 夏普比率: %.2f\n", perfData.SharpeRatio))

		// 			// 添加条件性建议
		// 			sb.WriteString("\n**策略建议**:\n")
		// 			if perfData.WinRate < 40 {
		// 				sb.WriteString("⚠️ 胜率偏低(<40%)，建议提高开仓标准，只做高确定性交易\n")
		// 			}

		// 			if perfData.ProfitFactor < 1.5 {
		// 				sb.WriteString("⚠️ 盈亏比偏低(<1.5)，建议扩大止盈空间或收紧止损\n")
		// 			}

		// 			if perfData.SharpeRatio < 0 {
		// 				sb.WriteString("⚠️ 夏普比率为负，策略整体亏损，建议暂停交易或调整策略\n")
		// 			}
		// 		} else {
		// 			sb.WriteString("- 暂无完成的交易记录\n")
		// 		}
		// 		sb.WriteString("\n")
		// 	}
		// }
	}

	sb.WriteString("---\n\n")
	sb.WriteString("现在请分析并输出决策（思维链 + JSON）\n")

	return sb.String()
}

// parseFullDecisionResponse 解析AI的完整决策响应
func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int) (*FullDecision, error) {
	// 1. 提取思维链
	cotTrace := extractCoTTrace(aiResponse)

	// 2. 提取JSON决策列表
	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("提取决策失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	// 3. 验证决策
	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("决策验证失败: %w\n\n=== AI思维链分析 ===\n%s", err, cotTrace)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

// extractCoTTrace 提取思维链分析
func extractCoTTrace(response string) string {
	// 查找JSON数组的开始位置
	jsonStart := strings.Index(response, "[")

	if jsonStart > 0 {
		// 思维链是JSON数组之前的内容
		return strings.TrimSpace(response[:jsonStart])
	}

	// 如果找不到JSON，整个响应都是思维链
	return strings.TrimSpace(response)
}

// extractDecisions 提取JSON决策列表
func extractDecisions(response string) ([]Decision, error) {
	// 直接查找JSON数组 - 找第一个完整的JSON数组
	arrayStart := strings.Index(response, "[")
	if arrayStart == -1 {
		// 显示响应的前200个字符用于调试
		preview := response
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("无法找到JSON数组起始，响应内容: %s", preview)
	}

	// 从 [ 开始，匹配括号找到对应的 ]
	arrayEnd := findMatchingBracket(response, arrayStart)
	if arrayEnd == -1 {
		return nil, fmt.Errorf("无法找到JSON数组结束")
	}

	jsonContent := strings.TrimSpace(response[arrayStart : arrayEnd+1])

	// 🔧 修复常见的JSON格式错误：缺少引号的字段值
	// 匹配: "reasoning": 内容"}  或  "reasoning": 内容}  (没有引号)
	// 修复为: "reasoning": "内容"}
	// 使用简单的字符串扫描而不是正则表达式
	jsonContent = fixMissingQuotes(jsonContent)

	// 解析JSON
	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w\nJSON内容: %s", err, jsonContent)
	}

	return decisions, nil
}

// fixMissingQuotes 替换中文引号为英文引号（避免输入法自动转换）
func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"") // "
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")  // '
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")  // '
	return jsonStr
}

// validateDecisions 验证所有决策（需要账户信息和杠杆配置）
func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage); err != nil {
			return fmt.Errorf("决策 #%d 验证失败: %w", i+1, err)
		}
	}
	return nil
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

// validateDecision 验证单个决策的有效性
func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int) error {
	// 验证action
	validActions := map[string]bool{
		"open_long":          true,
		"open_short":         true,
		"close_long":         true,
		"close_short":        true,
		"increase_long":      true,
		"increase_short":     true,
		"decrease_long":      true,
		"decrease_short":     true,
		"hold":               true,
		"wait":               true,
		"update_loss_profit": true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("无效的action: %s", d.Action)
	}

	// 开仓和加仓操作必须提供完整参数
	if d.Action == "open_long" || d.Action == "open_short" || d.Action == "increase_long" || d.Action == "increase_short" {
		// 根据币种使用配置的杠杆上限
		maxLeverage := altcoinLeverage        // 山寨币使用配置的杠杆
		maxPositionValue := accountEquity * 5 // 山寨币最多1.5倍账户净值
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage          // BTC和ETH使用配置的杠杆
			maxPositionValue = accountEquity * 10 // BTC/ETH最多10倍账户净值
		}

		if d.Leverage <= 0 || d.Leverage > maxLeverage {
			return fmt.Errorf("杠杆必须在1-%d之间（%s，当前配置上限%d倍）: %d", maxLeverage, d.Symbol, maxLeverage, d.Leverage)
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("仓位大小必须大于0: %.2f", d.PositionSizeUSD)
		}
		// 验证仓位价值上限（加1%容差以避免浮点数精度问题）
		tolerance := maxPositionValue * 0.01 // 1%容差
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH单币种仓位价值不能超过%.0f USDT（10倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("山寨币单币种仓位价值不能超过%.0f USDT（1.5倍账户净值），实际: %.0f", maxPositionValue, d.PositionSizeUSD)
			}
		}
		if d.EntryPrice <= 0 {
			return fmt.Errorf("入场价必须大于0: %.2f", d.EntryPrice)
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("止损和止盈必须大于0")
		}
		if strings.TrimSpace(d.InvalidationCondition) == "" {
			actionType := "开仓"
			if d.Action == "increase_long" || d.Action == "increase_short" {
				actionType = "加仓"
			}
			return fmt.Errorf("%s时必须设置离场条件(invalidation_condition)", actionType)
		}

		// 验证止损止盈的合理性
		if d.Action == "open_long" || d.Action == "increase_long" {
			if d.StopLoss >= d.EntryPrice {
				return fmt.Errorf("做多时止损价(%.2f)必须小于入场价(%.2f)", d.StopLoss, d.EntryPrice)
			}
			if d.TakeProfit <= d.EntryPrice {
				return fmt.Errorf("做多时止盈价(%.2f)必须大于入场价(%.2f)", d.TakeProfit, d.EntryPrice)
			}
		} else if d.Action == "open_short" || d.Action == "increase_short" {
			if d.StopLoss <= d.EntryPrice {
				return fmt.Errorf("做空时止损价(%.2f)必须大于入场价(%.2f)", d.StopLoss, d.EntryPrice)
			}
			if d.TakeProfit >= d.EntryPrice {
				return fmt.Errorf("做空时止盈价(%.2f)必须小于入场价(%.2f)", d.TakeProfit, d.EntryPrice)
			}
		}

		// 验证风险回报比（必须≥1:2）
		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" || d.Action == "increase_long" {
			riskPercent = (d.EntryPrice - d.StopLoss) / d.EntryPrice * 100
			rewardPercent = (d.TakeProfit - d.EntryPrice) / d.EntryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else if d.Action == "open_short" || d.Action == "increase_short" {
			riskPercent = (d.StopLoss - d.EntryPrice) / d.EntryPrice * 100
			rewardPercent = (d.EntryPrice - d.TakeProfit) / d.EntryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		// 硬约束：风险回报比必须≥2.0
		if riskRewardRatio < 2.0 {
			return fmt.Errorf("风险回报比过低(%.2f:1)，必须≥2.0:1 [入场:%.2f 止损:%.2f 止盈:%.2f] [风险:%.2f%% 收益:%.2f%%]",
				riskRewardRatio, d.EntryPrice, d.StopLoss, d.TakeProfit, riskPercent, rewardPercent)
		}
	}

	// 减仓操作必须提供仓位大小和理由
	if d.Action == "decrease_long" || d.Action == "decrease_short" {
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("减仓时必须指定减仓金额(position_size_usd): %.2f", d.PositionSizeUSD)
		}
		if strings.TrimSpace(d.Reasoning) == "" {
			return fmt.Errorf("减仓时必须提供reasoning说明原因")
		}
	}

	// update_loss_profit 操作必须提供止损和止盈价格
	if d.Action == "update_loss_profit" {
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("更新止盈止损时，止损和止盈价格必须大于0")
		}
		if strings.TrimSpace(d.Reasoning) == "" {
			return fmt.Errorf("更新止盈止损时必须提供reasoning说明原因")
		}
	}

	return nil
}

// generateChartScreenshot 生成图表截图用于AI分析
func generateChartScreenshot(ctx *Context) ([]byte, error) {
	// 选择主要币种（优先BTC，然后是持仓币种，最后是候选币种）
	var targetSymbol string

	// 1. 优先使用BTC作为市场基准
	if _, hasBTC := ctx.MarketDataMap["BTCUSDT"]; hasBTC {
		targetSymbol = "BTC"
	} else if len(ctx.Positions) > 0 {
		// 2. 如果没有BTC数据，使用第一个持仓币种
		firstPos := ctx.Positions[0]
		// 移除USDT后缀获取基础币种名称
		if strings.HasSuffix(firstPos.Symbol, "USDT") {
			targetSymbol = strings.TrimSuffix(firstPos.Symbol, "USDT")
		} else {
			targetSymbol = firstPos.Symbol
		}
	} else if len(ctx.CandidateCoins) > 0 {
		// 3. 最后使用第一个候选币种
		firstCandidate := ctx.CandidateCoins[0]
		if strings.HasSuffix(firstCandidate.Symbol, "USDT") {
			targetSymbol = strings.TrimSuffix(firstCandidate.Symbol, "USDT")
		} else {
			targetSymbol = firstCandidate.Symbol
		}
	}

	if targetSymbol == "" {
		return nil, fmt.Errorf("没有可用的币种生成图表")
	}

	// 直接从Hyperliquid网页截图
	imageData, err := chart.ScreenshotHyperliquidChart(targetSymbol)
	if err != nil {
		return nil, fmt.Errorf("从Hyperliquid截图失败: %w", err)
	}

	return imageData, nil
}

// saveScreenshotForDebug 保存截图到本地用于调试
func saveScreenshotForDebug(imageData []byte) error {
	// 创建调试目录
	debugDir := "debug_screenshots"
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return err
	}

	// 生成文件名（包含时间戳）
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s/chart_%s.png", debugDir, timestamp)

	// 保存文件
	if err := os.WriteFile(filename, imageData, 0644); err != nil {
		return err
	}

	log.Printf("🔍 调试截图已保存: %s", filename)
	return nil
}
