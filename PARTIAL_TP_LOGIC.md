# 分仓止盈逻辑说明

## 核心思想

分仓止盈基于AI给出的止盈价格，自动分两档执行：
- **第一档（50%目标）**：平仓50%
- **第二档（100%目标）**：平仓剩余50%

这样既能锁定部分利润，又能保留部分仓位追求更高收益。

## 计算逻辑

### 多仓（Long）

假设：
- 开仓价：`entryPrice`
- AI止盈价：`takeProfitPrice`（高于开仓价）

计算：
```
价格移动距离 = takeProfitPrice - entryPrice
50%目标价 = entryPrice + 价格移动距离 × 0.5
100%目标价 = takeProfitPrice
```

触发条件：
- 当前价 >= 50%目标价 → 平仓50%
- 当前价 >= 100%目标价 → 平仓剩余50%

### 空仓（Short）

假设：
- 开仓价：`entryPrice`
- AI止盈价：`takeProfitPrice`（低于开仓价）

计算：
```
价格移动距离 = entryPrice - takeProfitPrice
50%目标价 = entryPrice - 价格移动距离 × 0.5
100%目标价 = takeProfitPrice
```

触发条件：
- 当前价 <= 50%目标价 → 平仓50%
- 当前价 <= 100%目标价 → 平仓剩余50%

## 实际案例

### 案例1：BTC多仓

```
开仓价：50000 USDT
AI止盈价：55000 USDT
杠杆：10x

计算：
价格移动 = 55000 - 50000 = 5000
50%目标 = 50000 + 5000 × 0.5 = 52500 USDT
100%目标 = 55000 USDT

执行：
1. BTC涨到52500时，自动平仓50%（盈利5%，杠杆后50%）
2. BTC涨到55000时，自动平仓剩余50%（盈利10%，杠杆后100%）
```

### 案例2：ETH空仓

```
开仓价：3000 USDT
AI止盈价：2700 USDT
杠杆：10x

计算：
价格移动 = 3000 - 2700 = 300
50%目标 = 3000 - 300 × 0.5 = 2850 USDT
100%目标 = 2700 USDT

执行：
1. ETH跌到2850时，自动平仓50%（盈利5%，杠杆后50%）
2. ETH跌到2700时，自动平仓剩余50%（盈利10%，杠杆后100%）
```

## 与交易所止盈单的关系

### 未启用分仓止盈

系统会在交易所设置止盈单：
```go
trader.SetTakeProfit(symbol, side, quantity, takeProfitPrice)
```

### 启用分仓止盈

系统**不会**在交易所设置止盈单，而是：
1. 保存AI给出的止盈价格到内存
2. 每个交易周期检查价格是否达到50%或100%目标
3. 达到目标时通过程序自动执行平仓

这样可以实现灵活的分仓止盈策略。

## 代码实现要点

### 1. 开仓时保存止盈价格

```go
if at.config.EnablePartialTakeProfit {
    if tracking, exists := at.positionPnLTracking[posKey]; exists {
        tracking.TakeProfitPrice = decision.TakeProfit
        tracking.EntryPrice = marketData.CurrentPrice
        tracking.PartialTP50Executed = false
        tracking.PartialTP100Executed = false
    }
}
```

### 2. 每个周期检查止盈条件

```go
// 计算目标价格
if pos.Side == "long" {
    priceMove := tracking.TakeProfitPrice - tracking.EntryPrice
    target50Price = tracking.EntryPrice + priceMove*0.5
    target100Price = tracking.TakeProfitPrice
} else {
    priceMove := tracking.EntryPrice - tracking.TakeProfitPrice
    target50Price = tracking.EntryPrice - priceMove*0.5
    target100Price = tracking.TakeProfitPrice
}

// 检查是否达到目标
if !tracking.PartialTP50Executed {
    if (pos.Side == "long" && pos.MarkPrice >= target50Price) ||
       (pos.Side == "short" && pos.MarkPrice <= target50Price) {
        // 执行50%平仓
        executePartialTakeProfit(&pos, 0.5, record)
        tracking.PartialTP50Executed = true
    }
}
```

### 3. 避免重复执行

使用布尔标志记录执行状态：
- `PartialTP50Executed`：是否已执行50%止盈
- `PartialTP100Executed`：是否已执行100%止盈

每个档位只会触发一次。

## 优势

1. **智能化**：止盈目标由AI根据市场分析动态设定，而非固定百分比
2. **灵活性**：既能锁定部分利润，又能保留部分仓位追求更高收益
3. **简单配置**：只需一个开关，无需设置复杂的价格水平和比例
4. **自动管理**：系统自动计算目标价格并执行，无需人工干预

## 与移动止盈的配合

两种策略可以同时使用：

1. **分仓止盈先执行**：达到50%和100%目标时分批平仓
2. **移动止盈后续跟踪**：在剩余仓位上继续跟踪最高盈利点
3. **回撤触发平仓**：如果从峰值回撤超过设定距离，平掉剩余仓位

这样可以实现更完善的止盈策略。
