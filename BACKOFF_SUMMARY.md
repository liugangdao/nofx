# 幂级避让功能实现总结

## 实现方式

采用**三重验证**的方式：开仓时记录止损价格，持仓消失时通过价格比对+账户总额变化判断是否触发止损。

### 核心逻辑

1. **开仓时**：记录止损价格到 `PnLTracking` 结构
2. **每个周期**：
   - 记录当前账户总额
   - 检查持仓是否消失
3. **持仓消失时**（三重验证）：
   - ✅ 获取当前市场价格，与止损价格比对
   - ✅ 检查账户总额是否减少
   - ✅ 确认持仓已消失
4. **触发后**：启动幂级避让，暂停交易

### 判断标准

**多仓止损**：
```
当前价格 ≤ 止损价格 × 1.05  AND  账户总额减少
```

**空仓止损**：
```
当前价格 ≥ 止损价格 × 0.95  AND  账户总额减少
```

**优势**：
- 三重验证，准确率极高
- 避免止盈误判为止损
- 考虑滑点和市场波动（5%容差）
- 通过账户总额变化确认是亏损

## 代码改动

### 1. 配置结构 (config/config.go)

添加幂级避让配置字段：
```go
EnableExponentialBackoff bool    // 是否启用
BackoffBaseMinutes       int     // 基础休息时间
BackoffMultiplier        float64 // 倍数
BackoffMaxMinutes        int     // 最大休息时间
BackoffResetHours        int     // 重置时间
```

### 2. 交易器结构 (trader/auto_trader.go)

添加状态跟踪字段：
```go
stopLossCount    int       // 止损次数
lastStopLossTime time.Time // 最后止损时间
backoffUntil     time.Time // 暂停到的时间
```

修改 `PnLTracking` 结构，添加止损价格记录：
```go
StopLossPrice float64 // 止损价格
```

### 3. 核心方法

**checkStopLossTriggered()**：
- 检查持仓消失
- 获取市场价格
- 比对止损价格
- 触发幂级避让

**开仓方法修改**：
- 初始化 `PnLTracking` 时记录止损价格

## 配置示例

```json
{
  "enable_exponential_backoff": true,
  "backoff_base_minutes": 45,
  "backoff_multiplier": 2.67,
  "backoff_max_minutes": 360,
  "backoff_reset_hours": 24
}
```

## 优势

1. **简单可靠**：不依赖复杂的API调用
2. **通用性强**：适用于所有交易所
3. **准确度极高**：三重验证机制，避免误判
4. **区分止损止盈**：通过账户总额变化判断
5. **易于调试**：逻辑清晰，日志详细

## 使用场景

- ✅ 防止连续止损导致的情绪化交易
- ✅ 市场剧烈波动时自动减少交易频率
- ✅ 给AI更多时间观察市场变化
- ✅ 保护账户免受连续亏损

## 文档

- `README_BACKOFF.md` - 快速上手指南
- `EXPONENTIAL_BACKOFF.md` - 详细技术文档
- `config.json.backoff_example` - 配置示例
