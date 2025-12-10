# 仓位管理AI机器人使用说明

## 概述

仓位管理AI机器人（Position Manager）是一个专门用于管理现有持仓的AI系统。它不会开新仓，只会对现有仓位进行以下操作：

- ✅ **加仓** (increase_long/short): 趋势延续时增加仓位
- ✅ **减仓** (decrease_long/short): 部分止盈或风险增加时减少仓位
- ✅ **平仓** (close_long/short): 趋势反转或达到目标时完全平仓
- ✅ **移动止损** (update_loss_profit): 保护利润，调整止盈止损位置
- ✅ **持有** (hold): 继续持有当前仓位

## 核心特点

### 1. 专注仓位管理
- 不会开新仓，只管理现有持仓
- 如果没有持仓，自动跳过本周期
- 适合与开仓机器人配合使用

### 2. 基于K线和指标分析
- 分析15分钟、1小时、4小时K线
- 使用RSI、MACD、ADX、成交量等技术指标
- 识别趋势延续、反转、震荡等市场状态

### 3. 智能盈亏管理
- 跟踪最大盈利和峰值回撤
- 浮盈>2R时考虑部分止盈
- 浮盈>1R时移动止损至保本
- 峰值回撤>30%时考虑减仓

## 使用方法

### 方法1: 在main.go中集成

```go
package main

import (
	"log"
	"nofx/trader"
	"time"
)

func main() {
	// 创建仓位管理器配置
	pmConfig := trader.PositionManagerConfig{
		ID:                  "position_manager_1",
		Name:                "BTC Position Manager",
		AIModel:             "deepseek",
		Exchange:            "binance",
		EnableScreenshot:    false,
		ScanInterval:        3 * time.Minute,
		ScanIntervalMinutes: 3,
		InitialBalance:      1000.0,
		BTCETHLeverage:      5,
		AltcoinLeverage:     3,
		
		// 币安配置
		BinanceAPIKey:    "your_api_key",
		BinanceSecretKey: "your_secret_key",
		
		// AI配置
		DeepSeekKey: "your_deepseek_key",
	}

	// 创建仓位管理器
	pm, err := trader.NewPositionManager(pmConfig)
	if err != nil {
		log.Fatalf("创建仓位管理器失败: %v", err)
	}

	// 运行仓位管理器
	if err := pm.Run(); err != nil {
		log.Fatalf("运行仓位管理器失败: %v", err)
	}
}
```

### 方法2: 与现有Trader配合使用

可以同时运行开仓机器人和仓位管理机器人：

```go
// 开仓机器人（负责寻找机会开仓）
autoTrader, err := trader.NewAutoTrader(autoTraderConfig)
go autoTrader.Run()

// 仓位管理机器人（负责管理现有仓位）
positionManager, err := trader.NewPositionManager(pmConfig)
go positionManager.Run()
```

## 配置参数说明

### 基础配置
- `ID`: 管理器唯一标识
- `Name`: 管理器显示名称
- `AIModel`: AI模型 ("deepseek", "qwen", "gemini", "custom")
- `Exchange`: 交易平台 ("binance", "hyperliquid", "aster")

### 扫描配置
- `ScanInterval`: 扫描间隔（建议3-5分钟）
- `ScanIntervalMinutes`: 扫描间隔分钟数（用于K线数据）

### 账户配置
- `InitialBalance`: 初始余额（用于计算盈亏）
- `BTCETHLeverage`: BTC/ETH杠杆倍数
- `AltcoinLeverage`: 山寨币杠杆倍数

### 交易平台配置
根据选择的交易平台，配置相应的API密钥：

**币安 (Binance)**:
```go
BinanceAPIKey:    "your_api_key"
BinanceSecretKey: "your_secret_key"
```

**Hyperliquid**:
```go
HyperliquidPrivateKey: "your_private_key"
HyperliquidWalletAddr: "your_wallet_address"
HyperliquidTestnet:    false
```

**Aster**:
```go
AsterUser:       "your_main_wallet"
AsterSigner:     "your_api_wallet"
AsterPrivateKey: "your_api_private_key"
```

### AI配置
根据选择的AI模型，配置相应的API密钥：

```go
DeepSeekKey:     "your_deepseek_key"  // DeepSeek
QwenKey:         "your_qwen_key"      // 阿里云Qwen
GeminiKey:       "your_gemini_key"    // Google Gemini
CustomAPIURL:    "https://api.xxx"    // 自定义API
CustomAPIKey:    "your_custom_key"
CustomModelName: "gpt-4"
```

## AI决策逻辑

### 1. 加仓条件
- 趋势延续：价格突破关键阻力/支撑
- 技术指标确认：MACD金叉、RSI合理区间
- 成交量放大：确认突破有效性
- 原持仓已浮盈且止损已移至保本

### 2. 减仓条件
- 价格到达2R目标位
- 趋势减弱：ADX下降、成交量萎缩
- 出现反转信号：吞没K线、十字星
- 峰值回撤超过30%

### 3. 平仓条件
- 趋势明确反转：MACD死叉、破位
- 触发离场条件：价格突破关键支撑/阻力
- 风险急剧增加：接近止损价
- 持仓时间过长且无明显盈利

### 4. 移动止损条件
- 浮盈达到1R：移动止损至保本价
- 浮盈达到2R：移动止损至1R位置
- 趋势延续：跟随价格移动止损
- 保护利润：防止回撤过大

## 日志和监控

仓位管理器会生成详细的决策日志，保存在 `decision_logs/position_manager_1/` 目录下：

- 每个周期的AI思维链分析
- 具体的决策和执行结果
- 持仓状态和盈亏统计
- 错误和警告信息

## 最佳实践

### 1. 扫描间隔设置
- 短线交易：3-5分钟
- 中线交易：15-30分钟
- 长线交易：1-4小时

### 2. 与开仓机器人配合
- 开仓机器人：负责寻找新机会，开仓频率较低
- 仓位管理器：负责管理现有仓位，扫描频率较高
- 两者独立运行，互不干扰

### 3. 风险控制
- 设置合理的杠杆倍数
- 确保初始余额配置正确
- 定期检查决策日志
- 监控账户盈亏情况

### 4. AI模型选择
- DeepSeek：性价比高，推荐用于生产环境
- Qwen：国内访问快，适合中文环境
- Gemini：支持图表分析（需启用EnableScreenshot）
- Custom：可接入任何兼容OpenAI API的模型

## 注意事项

1. **不会开新仓**：仓位管理器只管理现有持仓，不会主动开新仓
2. **需要现有持仓**：如果账户没有持仓，会自动跳过本周期
3. **独立运行**：可以单独运行，也可以与开仓机器人配合使用
4. **API限制**：注意交易所API调用频率限制
5. **网络延迟**：确保网络稳定，避免价格偏差过大

## 示例输出

```
======================================================================
⏰ 2024-01-15 10:30:00 - [BTC Position Manager] 仓位管理周期 #5
======================================================================
📊 当前持仓数量: 2
📊 账户净值: 1050.00 USDT | 可用: 800.00 USDT | 持仓: 2
🤖 正在请求AI分析仓位并决策...
✅ AI API调用成功，响应长度: 1234 字符

----------------------------------------------------------------------
💭 AI思维链分析:
----------------------------------------------------------------------
BTCUSDT多头持仓分析：
- 当前价格43500，入场价42000，浮盈+3.57%
- 4H趋势向上，15m出现回踩确认
- RSI 65（合理区间），MACD金叉延续
- 建议：移动止损至保本价42000，保护利润

ETHUSDT空头持仓分析：
- 当前价格2250，入场价2300，浮盈+2.17%
- 价格已到达2R目标位2200附近
- 15m出现反弹信号，建议部分止盈
- 建议：减仓30%，锁定核心利润
----------------------------------------------------------------------

📋 AI决策列表 (2 个):
  [1] BTCUSDT: update_loss_profit - 移动止损至保本价
  [2] ETHUSDT: decrease_short - 价格到达2R目标，部分止盈30%

  🔄 更新止盈止损: BTCUSDT
  ✓ 止盈止损更新成功 - 新止损: 42000.0000, 新止盈: 45000.0000
  📈 减空仓: ETHUSDT
  ✓ 减仓成功，数量: 0.1333 (剩余: 0.3111)
```

## 故障排查

### 问题1: 无法获取持仓
- 检查API密钥是否正确
- 确认交易所账户有持仓
- 查看日志中的错误信息

### 问题2: AI决策失败
- 检查AI API密钥是否有效
- 确认网络连接正常
- 查看AI响应的原始内容

### 问题3: 执行决策失败
- 检查账户余额是否充足
- 确认持仓数量是否正确
- 查看交易所返回的错误信息

## 技术支持

如有问题，请查看：
1. 决策日志：`decision_logs/position_manager_1/`
2. 系统日志：控制台输出
3. 配置文件：确认所有参数正确
