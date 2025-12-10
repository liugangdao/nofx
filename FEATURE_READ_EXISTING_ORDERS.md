# 功能更新：读取现有止盈止损订单

## 概述

仓位管理器现在可以从交易所读取现有的止盈止损委托单，并在初始化 `PnLTracking` 时自动填充止损和止盈价格。

## 问题背景

之前的实现中，当仓位管理器启动时：
- `StopLossPrice` 和 `TakeProfitPrice` 都初始化为 0
- 即使交易所已经有止盈止损订单，系统也无法感知
- AI 无法基于现有的止盈止损价格做出决策

## 解决方案

### 1. 新增 Trader 接口方法

在 `trader/interface.go` 中添加：

```go
// GetOpenOrders 获取指定币种的所有未完成订单
GetOpenOrders(symbol string) ([]map[string]interface{}, error)
```

### 2. 各交易所实现

#### 币安 (Binance)
```go
func (t *FuturesTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
    orders, err := t.client.NewListOpenOrdersService().
        Symbol(symbol).
        Do(context.Background())
    
    // 返回订单信息，包括：
    // - orderId: 订单ID
    // - type: 订单类型 (STOP_MARKET, TAKE_PROFIT_MARKET等)
    // - side: 买卖方向
    // - positionSide: 持仓方向 (LONG/SHORT)
    // - stopPrice: 触发价格
    // - origQty: 原始数量
}
```

#### Hyperliquid
```go
func (t *HyperliquidTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
    allOrders, err := t.exchange.Info().OpenOrders(t.ctx, t.walletAddr)
    
    // 过滤指定币种的订单
    // 返回订单信息，包括：
    // - orderId: 订单ID (Oid)
    // - coin: 币种
    // - side: 方向
    // - limitPx: 限价
}
```

#### Aster
```go
func (t *AsterTrader) GetOpenOrders(symbol string) ([]map[string]interface{}, error) {
    // TODO: Aster暂未实现
    return []map[string]interface{}{}, nil
}
```

### 3. 仓位管理器集成

在 `trader/position_manager.go` 的 `buildTradingContext()` 函数中：

```go
if _, exists := pm.positionPnLTracking[posKey]; !exists {
    // 初始化追踪数据
    tracking := &PnLTracking{
        MaxProfitPct:      pnlPct,
        MaxLossPct:        pnlPct,
        Stage:             1,
        RemainingQuantity: 1.0,
        EntryPrice:        entryPrice,
    }

    // 🆕 从交易所读取现有订单
    orders, err := pm.trader.GetOpenOrders(symbol)
    if err != nil {
        log.Printf("⚠️  获取 %s 的委托单失败: %v", symbol, err)
    } else {
        // 解析止盈止损价格
        for _, order := range orders {
            stopPrice := getStopPrice(order) // 从订单中提取触发价格
            
            if stopPrice > 0 {
                // 根据持仓方向和价格关系判断是止损还是止盈
                if side == "long" {
                    if stopPrice < markPrice {
                        tracking.StopLossPrice = stopPrice  // 多头止损
                    } else {
                        tracking.TakeProfitPrice = stopPrice // 多头止盈
                    }
                } else {
                    if stopPrice > markPrice {
                        tracking.StopLossPrice = stopPrice  // 空头止损
                    } else {
                        tracking.TakeProfitPrice = stopPrice // 空头止盈
                    }
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
```

## 判断逻辑

### 多头仓位
```
当前价格: 100
止损订单触发价: 95  → 识别为止损 (低于当前价)
止盈订单触发价: 110 → 识别为止盈 (高于当前价)
```

### 空头仓位
```
当前价格: 100
止损订单触发价: 105 → 识别为止损 (高于当前价)
止盈订单触发价: 90  → 识别为止盈 (低于当前价)
```

### 订单类型判断（币安）
```go
// 也可以通过订单类型判断
if orderType == "STOP_MARKET" || orderType == "STOP" {
    // 止损单
} else if orderType == "TAKE_PROFIT_MARKET" || orderType == "TAKE_PROFIT" {
    // 止盈单
}
```

## 使用效果

### 启动日志示例

```
🚀 [Position Manager] 仓位管理系统启动
💰 初始余额: 10000.00 USDT
⚙️  扫描间隔: 5m0s
📊 只管理现有仓位，不会开新仓

⏰ 2024-12-05 15:30:00 - [Position Manager] 仓位管理周期 #1
======================================================================
📊 当前持仓数量: 2

📋 [BTCUSDT long] 读取到现有订单 - 止损: 95000.0000, 止盈: 105000.0000
📋 [ETHUSDT short] 读取到现有订单 - 止损: 3500.0000, 止盈: 3200.0000

📊 账户净值: 10500.00 USDT | 可用: 5000.00 USDT | 持仓: 2
```

### AI 可见信息

AI 现在可以看到完整的止盈止损信息：

```
## 当前持仓
1. BTCUSDT LONG | 入场价95000.0000 | 当前价98000.0000 | 盈亏+3.16% | 杠杆10x | 阶段1 | 持仓时长2小时30分钟
   止损价95000.0000 | 止盈价105000.0000 | 最高盈利+3.16% | 峰值回撤+0.00%
   **超级趋势(4h)**: UPTREND | 支撑96000.0000 | 阻力102000.0000
```

## 优势

1. **状态连续性**: 重启后能恢复之前设置的止盈止损
2. **决策准确性**: AI 基于真实的止盈止损价格做决策
3. **风险可见**: 用户可以看到系统识别的止盈止损价格
4. **多交易所支持**: 币安和 Hyperliquid 都已实现

## 注意事项

### Hyperliquid 限制
- Hyperliquid SDK 的 `OpenOrder` 结构体字段有限
- 触发价格可能需要额外解析
- 当前实现返回基本信息（orderId, coin, side, limitPx）

### Aster 限制
- Aster 暂未实现 `GetOpenOrders`
- 返回空数组，不影响正常运行

### 订单识别
- 主要通过触发价格与当前价格的关系判断
- 币安可以通过订单类型辅助判断
- 如果有多个止损/止盈订单，会使用最后一个

## 测试

编译测试通过：
```bash
go build -o nofx.exe .
# Exit Code: 0 ✅
```

## 相关文件

- `trader/interface.go` - Trader 接口定义
- `trader/binance_futures.go` - 币安实现
- `trader/hyperliquid_trader.go` - Hyperliquid 实现
- `trader/aster_trader.go` - Aster 实现
- `trader/position_manager.go` - 仓位管理器集成

## 未来改进

1. **Hyperliquid 增强**: 解析完整的触发单信息
2. **Aster 实现**: 添加获取订单功能
3. **订单缓存**: 避免频繁查询交易所
4. **订单验证**: 检查订单数量是否与持仓匹配
5. **多订单处理**: 支持部分止盈的多个订单
