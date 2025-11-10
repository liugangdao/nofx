# 移动止盈Bug修复说明

## 问题描述

原始实现中，移动止盈的检查逻辑存在一个严重bug：

```go
// 原始代码（有bug）
if pos.UnrealizedPnLPct >= at.config.TrailingStopActivation*100 {
    // 检查回撤
    drawdownFromPeak := (tracking.MaxProfitPct - pos.UnrealizedPnLPct) / 100
    if drawdownFromPeak >= at.config.TrailingStopDistance {
        // 触发止盈
    }
}
```

### Bug场景

假设配置：
- 激活条件：5%
- 回撤距离：3%

执行过程：
1. 持仓盈利达到10%，激活移动止盈，开始跟踪峰值
2. 盈利继续上涨到15%（新峰值）
3. 盈利回撤到3%（低于激活条件5%）
4. **Bug出现**：外层 `if` 条件不满足（3% < 5%），不会检查回撤
5. 即使从峰值15%回撤了12%（远超3%的设定），也不会触发止盈
6. 可能导致盈利变成亏损

### 问题根源

移动止盈的激活应该是**一次性的状态改变**，而不是每次都重新判断。一旦激活，就应该持续跟踪，不管当前盈利是否还满足激活条件。

## 修复方案

### 1. 添加激活状态标志

```go
type PnLTracking struct {
    MaxProfitPct          float64 // 最大盈利百分比
    MaxLossPct            float64 // 最大亏损百分比（负数）
    TakeProfitPrice       float64 // AI设置的止盈价格
    EntryPrice            float64 // 开仓价格
    PartialTP50Executed   bool    // 是否已执行50%止盈
    PartialTP100Executed  bool    // 是否已执行100%止盈
    TrailingStopActivated bool    // 移动止盈是否已激活（新增）
}
```

### 2. 修改检查逻辑

```go
// 修复后的代码
if at.config.EnableTrailingStop {
    // 检查是否达到激活条件（只需要达到一次）
    if !tracking.TrailingStopActivated && pos.UnrealizedPnLPct >= at.config.TrailingStopActivation*100 {
        tracking.TrailingStopActivated = true
        log.Printf("✨ [移动止盈激活] %s %s: 盈利%.2f%% 达到激活条件%.2f%%, 开始跟踪峰值",
            pos.Symbol, pos.Side, pos.UnrealizedPnLPct, at.config.TrailingStopActivation*100)
    }

    // 如果已激活，检查是否触发移动止盈
    if tracking.TrailingStopActivated {
        drawdownFromPeak := (tracking.MaxProfitPct - pos.UnrealizedPnLPct) / 100
        if drawdownFromPeak >= at.config.TrailingStopDistance {
            // 触发止盈
        }
    }
}
```

## 修复效果

使用相同的场景测试：

假设配置：
- 激活条件：5%
- 回撤距离：3%

执行过程：
1. 持仓盈利达到10%，激活移动止盈（`TrailingStopActivated = true`）
2. 盈利继续上涨到15%（新峰值）
3. 盈利回撤到3%（低于激活条件5%）
4. **修复后**：因为 `TrailingStopActivated = true`，继续检查回撤
5. 回撤 = 15% - 3% = 12% > 3%（设定距离）
6. ✅ 触发移动止盈，成功平仓锁定利润

## 关键改进

1. **状态持久化**：激活状态一旦设置就不会改变（直到持仓平仓）
2. **逻辑分离**：激活检查和回撤检查分开处理
3. **日志增强**：添加激活日志，便于监控和调试
4. **防止亏损**：确保盈利不会因为回撤过大而变成亏损

## 测试场景

### 场景1：正常止盈

```
配置：激活5%，回撤3%
1. 盈利5% → 激活移动止盈
2. 盈利8% → 更新峰值
3. 盈利5% → 回撤3%，触发止盈 ✅
```

### 场景2：大幅回撤（修复前会失败）

```
配置：激活5%，回撤3%
1. 盈利10% → 激活移动止盈
2. 盈利15% → 更新峰值
3. 盈利3% → 回撤12%，触发止盈 ✅
   （修复前：因为3% < 5%，不会触发 ❌）
```

### 场景3：未达到激活条件

```
配置：激活5%，回撤3%
1. 盈利3% → 未激活
2. 盈利4% → 未激活
3. 盈利1% → 未激活，不触发止盈 ✅
   （因为从未达到5%的激活条件）
```

## 总结

这个bug修复确保了移动止盈功能的正确性和可靠性，防止了因为逻辑错误导致的利润回吐。修复后的逻辑更符合移动止盈的设计初衷：**一旦激活就持续保护利润**。
