# 两阶段移动止盈策略

## 概述

仓位管理器现在支持基于超级趋势指标的两阶段移动止盈策略，旨在最大化趋势利润同时保护已实现收益。

## 策略原理

### 第一阶段：固定目标止盈

当浮盈达到 **2R**（2倍初始止损距离）时：

1. **部分止盈**：平仓 50% 仓位锁定利润
   - 使用 `decrease_long` 或 `decrease_short` 操作
   - 锁定一半利润，降低风险敞口

2. **移动止损至保本**：将剩余 50% 仓位的止损移至入场价
   - 使用 `update_loss_profit` 操作
   - 确保即使价格回撤，整体交易不会亏损

3. **进入第二阶段**：系统自动标记仓位进入第二阶段

### 第二阶段：移动止盈

剩余 50% 仓位使用**超级趋势线**作为动态止损，追逐最大趋势利润：

#### 多头仓位
- 止损设在**超级趋势支撑位** (`Supertrend.SupportLevel`)
- 随着价格上涨，支撑位自动上移
- 当价格跌破超级趋势支撑位时平仓

#### 空头仓位
- 止损设在**超级趋势阻力位** (`Supertrend.ResistanceLevel`)
- 随着价格下跌，阻力位自动下移
- 当价格突破超级趋势阻力位时平仓

#### 备选方案：ATR移动止损
如果不使用超级趋势，可以使用 ATR 移动止损：
- 多头：止损 = 当前价 - 2×ATR
- 空头：止损 = 当前价 + 2×ATR

## 技术实现

### 数据结构更新

#### PnLTracking 结构体
```go
type PnLTracking struct {
    MaxProfitPct      float64 // 最大盈利百分比
    MaxLossPct        float64 // 最大亏损百分比
    TakeProfitPrice   float64 // 止盈价格
    StopLossPrice     float64 // 止损价格
    EntryPrice        float64 // 开仓价格
    Stage             int     // 止盈阶段: 1=第一阶段, 2=第二阶段
    PartialTakenAt    float64 // 部分止盈时的价格
    RemainingQuantity float64 // 剩余仓位百分比 (0-1)
}
```

#### SupertrendData 结构体
```go
type SupertrendData struct {
    Trend           string  // "UPTREND" 或 "DOWNTREND"
    Value           float64 // 超级趋势线值
    SupportLevel    float64 // 支撑位（多头使用）
    ResistanceLevel float64 // 阻力位（空头使用）
    ATRMultiplier   float64 // ATR倍数（默认3.0）
    Description     string  // 描述
}
```

### 超级趋势计算

超级趋势指标基于 ATR（平均真实波幅）计算：

```
基础上轨 = (最高价 + 最低价) / 2 + 3×ATR
基础下轨 = (最高价 + 最低价) / 2 - 3×ATR

趋势判断：
- 收盘价 > 下轨 → 上升趋势（使用下轨作为支撑）
- 收盘价 < 上轨 → 下降趋势（使用上轨作为阻力）
```

### AI Prompt 更新

System Prompt 中添加了详细的两阶段策略说明：

```
## 3. 两阶段移动止盈策略
**第一阶段 (固定目标止盈)**:
- 当浮盈达到2R (2倍初始止损距离)时:
  * 使用 decrease_long/short 平仓50%仓位锁定利润
  * 使用 update_loss_profit 将剩余50%仓位的止损移至入场价(保本)
  * 标记进入第二阶段

**第二阶段 (移动止盈)**:
- 剩余50%仓位使用超级趋势线作为移动止损:
  * 多头: 止损设在超级趋势支撑位 (Supertrend.SupportLevel)
  * 空头: 止损设在超级趋势阻力位 (Supertrend.ResistanceLevel)
  * 当价格突破超级趋势线时平仓离场
  * 或使用 ATR 移动止损: 止损距离 = 当前价 ± 2*ATR
```

User Prompt 中显示：
- 当前止盈阶段（阶段1 或 阶段2）
- 剩余仓位百分比
- 4小时和15分钟超级趋势数据

## 使用示例

### AI 决策示例

#### 第一阶段：达到2R目标
```json
[
  {
    "symbol": "BTCUSDT",
    "action": "decrease_long",
    "position_size_usd": 500,
    "reasoning": "浮盈达到2R目标(+20%)，部分止盈50%锁定利润",
    "invalidation_condition": "none"
  },
  {
    "symbol": "BTCUSDT",
    "action": "update_loss_profit",
    "stop_loss": 95000,
    "take_profit": 110000,
    "reasoning": "移动止损至入场价95000保本，进入第二阶段移动止盈",
    "invalidation_condition": "4h close below 94000"
  }
]
```

#### 第二阶段：使用超级趋势移动止损
```json
[
  {
    "symbol": "BTCUSDT",
    "action": "update_loss_profit",
    "stop_loss": 98000,
    "take_profit": 115000,
    "reasoning": "价格继续上涨，根据超级趋势支撑位98000移动止损",
    "invalidation_condition": "15m close below supertrend support"
  }
]
```

#### 第二阶段：触发超级趋势止损
```json
[
  {
    "symbol": "BTCUSDT",
    "action": "close_long",
    "reasoning": "价格跌破超级趋势支撑位，趋势反转，平仓离场",
    "invalidation_condition": "none"
  }
]
```

## 优势

1. **风险控制**：第一阶段锁定50%利润，确保盈利交易
2. **利润最大化**：第二阶段追逐趋势，捕捉更大行情
3. **动态调整**：超级趋势自动适应市场波动
4. **心理优势**：部分止盈后心态更轻松，更容易持有剩余仓位

## 参数配置

### 超级趋势参数
- **ATR周期**：10（默认）
- **ATR倍数**：3.0（默认）
- 可在 `market/data.go` 的 `calculateTimeframeData` 函数中调整

### 止盈阶段触发条件
- **第一阶段触发**：浮盈 ≥ 2R
- **部分止盈比例**：50%（可在代码中调整为40%-60%）
- **第二阶段判断**：剩余仓位在40%-60%之间

## 监控和日志

系统会自动记录：
- 止盈阶段转换
- 部分止盈价格
- 剩余仓位百分比
- 超级趋势支撑/阻力位变化

日志示例：
```
📊 进入第二阶段移动止盈 (剩余仓位: 50%)
**超级趋势(4h)**: UPTREND | 支撑98000.00 | 阻力102000.00
**超级趋势(15m)**: UPTREND | 支撑97500.00 | 阻力101500.00
```

## 注意事项

1. **市场条件**：超级趋势在趋势市场中表现最佳，震荡市场可能产生频繁信号
2. **时间周期**：建议同时参考4小时和15分钟超级趋势，多周期确认
3. **手动干预**：AI可以根据市场情况灵活调整，不必严格遵循50%比例
4. **风险管理**：即使在第二阶段，也要关注峰值回撤和反转信号

## 相关文件

- `trader/position_manager.go` - 仓位管理器主逻辑
- `trader/auto_trader.go` - PnLTracking 结构体定义
- `market/data.go` - 超级趋势计算
- `market/data_format.go` - 市场数据格式化
- `decision/engine.go` - 决策引擎（如需更新）

## 未来改进

1. 支持自定义部分止盈比例（30%、40%、60%等）
2. 多阶段止盈（例如：1R止盈25%，2R止盈25%，剩余50%移动止盈）
3. 结合其他指标（如布林带、斐波那契回撤）优化止损位
4. 回测功能，验证策略效果
