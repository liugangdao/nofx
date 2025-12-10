# TradingView 信号示例

## 基础示例

### 1. 简单做多（使用默认参数）

```json
{
  "action": "buy",
  "symbol": "BTC"
}
```

这将使用配置文件中的设置：
- 如果配置了 `position_size_percent`（如 5%），则自动使用账户资金的 5% 开仓
- 如果配置了 `default_quantity`（如 0.01），则使用固定数量开仓
- 使用 `default_leverage` 作为杠杆倍数

**智能保护：** 如果 BTC 已有仓位（多仓或空仓），系统会自动跳过此信号，避免重复开仓。

### 2. 指定数量和杠杆的做多

```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.01,
  "leverage": 5
}
```

### 3. 做空

```json
{
  "action": "sell",
  "symbol": "ETHUSDT",
  "quantity": 0.05,
  "leverage": 3
}
```

### 4. 平多仓（自动获取持仓数量）

```json
{
  "action": "close_long",
  "symbol": "BTCUSDT"
}
```

### 5. 平空仓（指定数量）

```json
{
  "action": "close_short",
  "symbol": "ETHUSDT",
  "quantity": 0.05
}
```

## 高级示例

### 策略 1: 突破做多

**TradingView Pine Script:**
```pine
//@version=5
strategy("Breakout Long", overlay=true)

// 参数
length = input.int(20, "Length")
leverage = input.int(5, "Leverage")
quantity = input.float(0.01, "Quantity")

// 计算突破
highest_high = ta.highest(high, length)
breakout = close > highest_high[1]

// 开仓信号
if breakout and strategy.position_size == 0
    alert('{"action":"buy","symbol":"' + syminfo.ticker + '","quantity":' + str.tostring(quantity) + ',"leverage":' + str.tostring(leverage) + '}')
    strategy.entry("Long", strategy.long)

// 平仓信号（止损）
if strategy.position_size > 0 and close < strategy.position_avg_price * 0.98
    alert('{"action":"close_long","symbol":"' + syminfo.ticker + '"}')
    strategy.close("Long")
```

### 策略 2: RSI 反转

**TradingView Alert Message:**

做多信号：
```json
{
  "action": "buy",
  "symbol": "{{ticker}}",
  "quantity": 0.01,
  "leverage": 3
}
```

做空信号：
```json
{
  "action": "sell",
  "symbol": "{{ticker}}",
  "quantity": 0.01,
  "leverage": 3
}
```

平仓信号：
```json
{
  "action": "close",
  "symbol": "{{ticker}}"
}
```

### 策略 3: 多币种轮动

**BTC 做多：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "leverage": 5
}
```

**ETH 做多：**
```json
{
  "action": "buy",
  "symbol": "ETHUSDT",
  "leverage": 5
}
```

**SOL 做多：**
```json
{
  "action": "buy",
  "symbol": "SOLUSDT",
  "leverage": 5
}
```

**说明：** 不指定 `quantity`，系统会自动使用账户资金的配置百分比（如 5%）为每个币种开仓。

### 策略 4: 仓位保护示例

系统会自动检测现有仓位，避免重复开仓：

**场景 1：BTC 已有多仓**
- 收到信号：`{"action": "buy", "symbol": "BTCUSDT"}`
- 系统响应：跳过开仓，返回 `{"status": "skipped", "reason": "已有long仓位"}`

**场景 2：BTC 已有空仓**
- 收到信号：`{"action": "buy", "symbol": "BTCUSDT"}`
- 系统响应：跳过开仓，返回 `{"status": "skipped", "reason": "已有short仓位"}`

**场景 3：BTC 无仓位**
- 收到信号：`{"action": "buy", "symbol": "BTCUSDT"}`
- 系统响应：正常开仓，返回 `{"status": "success"}`

这个机制确保：
- 不会在同一币种上重复开多仓
- 不会在已有空仓时再开多仓（避免对冲）
- 需要先平仓才能反向开仓

## 实用技巧

### 技巧 1: 使用 TradingView 变量

TradingView 支持在 Alert Message 中使用变量：

```json
{
  "action": "buy",
  "symbol": "{{ticker}}",
  "quantity": {{close}},
  "leverage": 5,
  "price": {{close}}
}
```

可用变量：
- `{{ticker}}`: 交易对名称
- `{{close}}`: 收盘价
- `{{open}}`: 开盘价
- `{{high}}`: 最高价
- `{{low}}`: 最低价
- `{{volume}}`: 成交量
- `{{time}}`: 时间戳

### 技巧 2: 条件平仓

**止盈平仓：**
```json
{
  "action": "close_long",
  "symbol": "BTCUSDT",
  "quantity": 0
}
```

**止损平仓：**
```json
{
  "action": "close_long",
  "symbol": "BTCUSDT",
  "quantity": 0
}
```

### 技巧 3: 分批建仓

**第一批（30%）：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.003,
  "leverage": 5
}
```

**第二批（30%）：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.003,
  "leverage": 5
}
```

**第三批（40%）：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.004,
  "leverage": 5
}
```

## 测试建议

1. **先在测试网测试**
   - 配置文件中设置 `"testnet": true`
   - 使用测试网钱包和资金

2. **使用小额资金**
   - 初始测试使用最小数量
   - 逐步增加仓位

3. **监控日志**
   - 观察服务器日志输出
   - 确认订单执行情况

4. **设置合理的杠杆**
   - 新手建议 1-3x
   - 有经验者可以 3-10x
   - 避免过高杠杆（>10x）

## 常见错误

### 错误 1: 数量太小

```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.00001
}
```

**解决：** 增加数量到最小下单量以上（BTC 通常 0.001+）

### 错误 2: Symbol 格式错误

```json
{
  "action": "buy",
  "symbol": "BTC/USDT"
}
```

**解决：** 使用正确格式 `"BTCUSDT"` 或 `"BTC"`

### 错误 3: 杠杆超限

```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.01,
  "leverage": 100
}
```

**解决：** 使用合理杠杆（1-50x）

## 更多资源

- [TradingView Pine Script 文档](https://www.tradingview.com/pine-script-docs/)
- [Hyperliquid API 文档](https://hyperliquid.gitbook.io/)
- [项目 README](README.md)
- [快速开始指南](QUICK_START.zh-CN.md)
