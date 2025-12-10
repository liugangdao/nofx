# 仓位管理AI机器人 (Position Manager)

## 简介

仓位管理AI机器人是一个专门用于管理现有持仓的智能系统。它不会开新仓，只会根据市场K线数据和技术指标，对现有仓位进行加仓、减仓、平仓或移动止损操作。

## 核心功能

### 🎯 专注仓位管理
- ✅ 只管理现有持仓，不开新仓
- ✅ 如果没有持仓，自动跳过
- ✅ 可与开仓机器人配合使用

### 📊 智能分析
- 分析15分钟、1小时、4小时K线
- 使用RSI、MACD、ADX、成交量等技术指标
- 识别趋势延续、反转、震荡等市场状态

### 🔧 灵活操作
- **加仓**: 趋势延续时增加仓位
- **减仓**: 部分止盈或风险增加时减少仓位
- **平仓**: 趋势反转或达到目标时完全平仓
- **移动止损**: 保护利润，调整止盈止损位置

## 快速开始

### 1. 创建配置

```go
pmConfig := trader.PositionManagerConfig{
    ID:                  "pm_1",
    Name:                "Position Manager",
    AIModel:             "deepseek",
    Exchange:            "binance",
    ScanInterval:        3 * time.Minute,
    ScanIntervalMinutes: 3,
    InitialBalance:      1000.0,
    BTCETHLeverage:      5,
    AltcoinLeverage:     3,
    
    BinanceAPIKey:    "your_api_key",
    BinanceSecretKey: "your_secret_key",
    DeepSeekKey:      "your_deepseek_key",
}
```

### 2. 创建并运行

```go
pm, err := trader.NewPositionManager(pmConfig)
if err != nil {
    log.Fatal(err)
}

pm.Run()
```

### 3. 运行示例

```bash
# 设置环境变量
export BINANCE_API_KEY="your_api_key"
export BINANCE_SECRET_KEY="your_secret_key"
export DEEPSEEK_API_KEY="your_deepseek_key"

# 运行示例
go run examples/position_manager_example.go
```

## 决策逻辑

### 加仓 (Increase)
- 趋势延续：价格突破关键阻力/支撑
- 技术指标确认：MACD金叉、RSI合理区间
- 成交量放大：确认突破有效性
- 原持仓已浮盈且止损已移至保本

### 减仓 (Decrease)
- 价格到达2R目标位
- 趋势减弱：ADX下降、成交量萎缩
- 出现反转信号：吞没K线、十字星
- 峰值回撤超过30%

### 平仓 (Close)
- 趋势明确反转：MACD死叉、破位
- 触发离场条件：价格突破关键支撑/阻力
- 风险急剧增加：接近止损价

### 移动止损 (Update)
- 浮盈达到1R：移动止损至保本价
- 浮盈达到2R：移动止损至1R位置
- 趋势延续：跟随价格移动止损

## 配置说明

### 支持的交易平台
- **Binance** (币安合约)
- **Hyperliquid**
- **Aster**

### 支持的AI模型
- **DeepSeek** (推荐，性价比高)
- **Qwen** (阿里云通义千问)
- **Gemini** (Google，支持图表分析)
- **Custom** (自定义API)

## 与开仓机器人配合

可以同时运行开仓机器人和仓位管理机器人：

```go
// 开仓机器人（负责寻找机会开仓）
autoTrader, _ := trader.NewAutoTrader(autoTraderConfig)
go autoTrader.Run()

// 仓位管理机器人（负责管理现有仓位）
positionManager, _ := trader.NewPositionManager(pmConfig)
go positionManager.Run()
```

**分工明确**:
- 开仓机器人：扫描市场，寻找新机会，开仓频率较低
- 仓位管理器：管理现有仓位，扫描频率较高，不开新仓

## 日志和监控

决策日志保存在 `decision_logs/{manager_id}/` 目录下，包含：
- AI思维链分析
- 具体决策和执行结果
- 持仓状态和盈亏统计
- 错误和警告信息

## 文件结构

```
trader/
├── position_manager.go          # 仓位管理器核心代码
├── auto_trader.go               # 开仓机器人（原有）
├── interface.go                 # 交易器接口
├── binance_futures.go           # 币安交易器
├── hyperliquid_trader.go        # Hyperliquid交易器
└── aster_trader.go              # Aster交易器

examples/
└── position_manager_example.go  # 使用示例

POSITION_MANAGER_USAGE.md        # 详细使用说明
POSITION_MANAGER_README.md       # 本文件
```

## 最佳实践

1. **扫描间隔**: 短线3-5分钟，中线15-30分钟
2. **杠杆设置**: BTC/ETH 5-10倍，山寨币3-5倍
3. **风险控制**: 确保初始余额配置正确
4. **日志监控**: 定期检查决策日志
5. **独立运行**: 可单独运行或与开仓机器人配合

## 注意事项

⚠️ **重要提示**:
1. 仓位管理器不会开新仓
2. 需要账户有现有持仓才会工作
3. 注意交易所API调用频率限制
4. 确保网络稳定，避免价格偏差

## 技术支持

详细文档请参考:
- [详细使用说明](POSITION_MANAGER_USAGE.md)
- [示例代码](examples/position_manager_example.go)
- 决策日志: `decision_logs/{manager_id}/`

## 许可证

与主项目相同
