# 快速开始：选择运行模式

## 🎯 两种模式

```
TM (Trading Machine)     →  自动寻找机会并开仓
PM (Position Manager)    →  只管理现有仓位
```

## ⚡ 快速配置

### 1. 编辑 config.json

```json
{
  "traders": [
    {
      "id": "my_bot",
      "name": "My Bot",
      "enabled": true,
      "mode": "tm",  // 👈 改为 "pm" 使用仓位管理器
      "ai_model": "deepseek",
      "exchange": "binance",
      ...
    }
  ]
}
```

### 2. 运行

```bash
./nofx.exe
```

## 📊 模式选择

### 使用 TM (交易机器人) 如果你想：

✅ 全自动交易  
✅ 系统自动寻找机会  
✅ 24/7 运行  
✅ 多币种监控  

**配置**: `"mode": "tm"`

### 使用 PM (仓位管理器) 如果你想：

✅ 手动开仓，AI管理  
✅ 只管理现有仓位  
✅ 两阶段移动止盈  
✅ 基于超级趋势止损  

**配置**: `"mode": "pm"`

## 🔧 关键配置

| 字段 | TM | PM | 说明 |
|------|----|----|------|
| `mode` | `"tm"` | `"pm"` | 运行模式 |
| `scan_interval_minutes` | 3-5 | 5-10 | 扫描间隔 |
| `initial_balance` | 实际余额 | 实际余额 | 初始余额 |

## 💡 使用场景

### 场景 1：完全自动化
```json
{
  "mode": "tm",
  "scan_interval_minutes": 3
}
```
系统自动寻找机会并交易

### 场景 2：半自动化
```json
{
  "mode": "pm",
  "scan_interval_minutes": 5
}
```
你手动开仓，AI帮你管理

### 场景 3：混合模式
```json
{
  "traders": [
    {"id": "trader", "mode": "tm", ...},
    {"id": "manager", "mode": "pm", ...}
  ]
}
```
同时运行交易机器人和仓位管理器

## 📝 启动日志

### TM 启动
```
🚀 [Auto Trader] 自动交易系统启动
🪙 币种池: 8个币种
📊 开始扫描市场机会...
```

### PM 启动
```
🚀 [Position Manager] 仓位管理系统启动
📊 只管理现有仓位，不会开新仓
📋 [BTCUSDT long] 读取到现有订单 - 止损: 95000, 止盈: 105000
```

## ⚠️ 注意事项

### TM
- ⚠️ 全自动交易有风险
- ⚠️ 建议先小资金测试
- ⚠️ 设置风险控制参数

### PM
- ⚠️ 必须有现有仓位才工作
- ⚠️ 首次启动会读取止盈止损
- ⚠️ 不会开新仓

## 📚 详细文档

- [MODE_CONFIGURATION_GUIDE.md](MODE_CONFIGURATION_GUIDE.md) - 完整配置指南
- [TWO_STAGE_PROFIT_TAKING.md](TWO_STAGE_PROFIT_TAKING.md) - 两阶段止盈策略
- [config.json.example](config.json.example) - 配置示例
