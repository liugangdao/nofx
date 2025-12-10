# 更新日志 - 两阶段移动止盈策略

## 版本信息
- 日期: 2024-12-05
- 功能: 基于超级趋势的两阶段移动止盈策略

## 新增功能

### 1. 超级趋势指标 (Supertrend Indicator)
- 新增 `SupertrendData` 结构体
- 实现超级趋势计算算法
- 支持自定义 ATR 周期和倍数
- 自动识别上升/下降趋势
- 提供动态支撑位和阻力位

### 2. 两阶段止盈策略
**第一阶段 - 固定目标止盈**:
- 浮盈达到 2R 时自动触发
- 平仓 50% 仓位锁定利润
- 移动止损至入场价（保本）
- 自动进入第二阶段

**第二阶段 - 移动止盈**:
- 使用超级趋势线作为动态止损
- 多头：跟随超级趋势支撑位
- 空头：跟随超级趋势阻力位
- 最大化趋势利润

### 3. 增强的仓位追踪
- 新增 `Stage` 字段：追踪止盈阶段
- 新增 `PartialTakenAt` 字段：记录部分止盈价格
- 新增 `RemainingQuantity` 字段：追踪剩余仓位百分比

## 修改的文件

### 核心文件
1. **market/data.go**
   - 新增 `SupertrendData` 结构体
   - 新增 `calculateSupertrend()` 函数
   - 在 `TimeframeData` 中添加 `Supertrend` 字段

2. **market/data_format.go**
   - 添加超级趋势信息显示

3. **trader/auto_trader.go**
   - 扩展 `PnLTracking` 结构体
   - 添加阶段追踪字段

4. **trader/position_manager.go**
   - 更新 System Prompt：添加两阶段策略说明
   - 更新 User Prompt：显示阶段信息和超级趋势数据
   - 更新 `executeDecreaseLong/Short()`：自动更新阶段
   - 更新 `buildTradingContext()`：初始化阶段信息

### 测试文件
5. **market/supertrend_test.go** (新增)
   - 测试上升趋势识别
   - 测试下降趋势识别
   - 测试数据不足情况

### 文档文件
6. **TWO_STAGE_PROFIT_TAKING.md** (新增)
   - 完整的策略说明文档
   - 使用示例和配置说明

7. **CHANGELOG_TWO_STAGE_PROFIT.md** (本文件)
   - 更新日志

## 技术细节

### 超级趋势计算公式
```
HL2 = (最高价 + 最低价) / 2
基础上轨 = HL2 + (ATR × 倍数)
基础下轨 = HL2 - (ATR × 倍数)

最终上轨规则:
  if 当前基础上轨 < 前一根最终上轨 OR 前一根收盘价 > 前一根最终上轨:
    最终上轨 = 当前基础上轨
  else:
    最终上轨 = 前一根最终上轨

最终下轨规则:
  if 当前基础下轨 > 前一根最终下轨 OR 前一根收盘价 < 前一根最终下轨:
    最终下轨 = 当前基础下轨
  else:
    最终下轨 = 前一根最终下轨

趋势判断:
  if 前一根是上升趋势:
    if 收盘价 <= 最终下轨: 转为下降趋势
  else:
    if 收盘价 >= 最终上轨: 转为上升趋势
```

### 默认参数
- ATR 周期: 10
- ATR 倍数: 3.0
- 第一阶段触发: 浮盈 ≥ 2R
- 部分止盈比例: 50%

## 使用方法

### AI 决策示例

#### 达到 2R 目标，进入第一阶段
```json
[
  {
    "symbol": "BTCUSDT",
    "action": "decrease_long",
    "position_size_usd": 500,
    "reasoning": "浮盈达到2R(+20%)，部分止盈50%锁定利润"
  },
  {
    "symbol": "BTCUSDT",
    "action": "update_loss_profit",
    "stop_loss": 95000,
    "take_profit": 110000,
    "reasoning": "移动止损至入场价保本，进入第二阶段"
  }
]
```

#### 第二阶段，使用超级趋势移动止损
```json
[
  {
    "symbol": "BTCUSDT",
    "action": "update_loss_profit",
    "stop_loss": 98000,
    "take_profit": 115000,
    "reasoning": "根据超级趋势支撑位98000移动止损"
  }
]
```

## 测试结果

所有测试通过：
```
✓ TestCalculateSupertrend - 上升趋势识别
✓ TestCalculateSupertrendDowntrend - 下降趋势识别
✓ TestCalculateSupertrendInsufficientData - 数据不足处理
```

编译成功：
```
go build -o nofx.exe .
```

## 向后兼容性

- 所有现有功能保持不变
- 新字段使用默认值初始化
- 不影响现有的自动交易器 (auto_trader)
- 仅在仓位管理器 (position_manager) 中启用新策略

## 未来改进建议

1. 支持自定义部分止盈比例（30%、40%、60%）
2. 多阶段止盈（1R、2R、3R 分别止盈）
3. 结合其他指标优化止损位（布林带、斐波那契）
4. 添加回测功能验证策略效果
5. 支持不同时间周期的超级趋势组合

## 相关文档

- [TWO_STAGE_PROFIT_TAKING.md](TWO_STAGE_PROFIT_TAKING.md) - 完整策略文档
- [POSITION_MANAGER_README.md](POSITION_MANAGER_README.md) - 仓位管理器说明
- [POSITION_MANAGER_USAGE.md](POSITION_MANAGER_USAGE.md) - 使用指南
