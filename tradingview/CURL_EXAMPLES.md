# Curl 命令示例

本文档提供使用 curl 命令测试 TradingView Webhook 的详细示例。

## 基础命令

### 1. 健康检查

```bash
curl http://localhost:9090/health
```

**预期响应：**
```json
{
  "status": "ok"
}
```

## 开仓命令

### 2. 开多仓（使用默认参数）

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }'
```

**说明：**
- 使用配置文件中的 `position_size_percent`（默认 5%）
- 使用配置文件中的 `default_leverage`

**预期响应：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.001,
  "price": 45123.45,
  "leverage": 5,
  "status": "success"
}
```

### 3. 开多仓（指定数量和杠杆）

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "buy",
    "symbol": "BTCUSDT",
    "quantity": 0.01,
    "leverage": 3
  }'
```

### 4. 开空仓

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "sell",
    "symbol": "ETHUSDT",
    "leverage": 5
  }'
```

**或使用 "short"：**
```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "short",
    "symbol": "ETH"
  }'
```

## 平仓命令

### 5. 平多仓（自动获取持仓数量）

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "close_long",
    "symbol": "BTCUSDT"
  }'
```

**或使用 "close"：**
```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "close",
    "symbol": "BTC"
  }'
```

### 6. 平空仓

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "close_short",
    "symbol": "ETHUSDT"
  }'
```

### 7. 平仓（指定数量）

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "close_long",
    "symbol": "BTCUSDT",
    "quantity": 0.005
  }'
```

## 带 Webhook 密钥的命令

如果配置了 `webhook_secret`，需要添加 `X-Webhook-Secret` Header：

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your_secret_key" \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }'
```

## 仓位保护测试

### 测试重复开仓（应该被跳过）

```bash
# 第一次开仓（成功）
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }'

# 等待几秒

# 第二次开仓（应该被跳过）
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }'
```

**第二次的预期响应：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "status": "skipped",
  "reason": "已有long仓位",
  "message": "BTCUSDT 已有 long 仓位，跳过开多仓"
}
```

## 多币种测试

### 同时测试多个币种

```bash
# BTC 开多
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}'

# ETH 开多
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"ETH"}'

# SOL 开多
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"SOL"}'
```

## Windows CMD 命令

在 Windows CMD 中使用 curl：

```cmd
curl -X POST http://localhost:9090/webhook ^
  -H "Content-Type: application/json" ^
  -d "{\"action\":\"buy\",\"symbol\":\"BTC\"}"
```

**注意：** Windows CMD 中需要：
- 使用 `^` 作为行继续符
- 使用 `\"` 转义双引号

## PowerShell 命令

在 PowerShell 中使用 curl（实际是 Invoke-WebRequest 的别名）：

```powershell
$body = @{
    action = "buy"
    symbol = "BTC"
} | ConvertTo-Json

Invoke-RestMethod -Uri "http://localhost:9090/webhook" `
  -Method Post `
  -Body $body `
  -ContentType "application/json"
```

## 完整测试流程

```bash
#!/bin/bash

SERVER_URL="http://localhost:9090"

echo "1. 健康检查"
curl -s "$SERVER_URL/health" | jq .

echo -e "\n2. 开多仓 (BTC)"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}' | jq .

sleep 3

echo -e "\n3. 尝试重复开仓 (应该被跳过)"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}' | jq .

sleep 2

echo -e "\n4. 平多仓 (BTC)"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -d '{"action":"close_long","symbol":"BTC"}' | jq .

sleep 3

echo -e "\n5. 开空仓 (ETH)"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -d '{"action":"sell","symbol":"ETH","leverage":3}' | jq .

sleep 3

echo -e "\n6. 平空仓 (ETH)"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -d '{"action":"close_short","symbol":"ETH"}' | jq .

echo -e "\n✅ 测试完成！"
```

## 错误处理示例

### 无效的 action

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "invalid_action",
    "symbol": "BTC"
  }'
```

**响应：**
```json
{
  "error": "未知的操作: invalid_action"
}
```

### 缺少必需字段

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "buy"
  }'
```

**响应：**
```json
{
  "error": "无效的信号格式"
}
```

### Webhook 密钥错误

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: wrong_secret" \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }'
```

**响应：**
```json
{
  "error": "无效的webhook密钥"
}
```

## 调试技巧

### 1. 查看完整的 HTTP 响应

```bash
curl -v -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}'
```

### 2. 保存响应到文件

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}' \
  -o response.json
```

### 3. 显示响应时间

```bash
curl -w "\nTime: %{time_total}s\n" \
  -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}'
```

### 4. 使用 jq 格式化输出

```bash
curl -s -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}' | jq .
```

## 常见问题

### Q: curl 命令返回 "Connection refused"

**A:** 确认服务器正在运行：
```bash
# 检查进程
ps aux | grep tradingview_webhook

# 检查端口
netstat -an | grep 9090
```

### Q: 如何测试远程服务器？

**A:** 将 `localhost` 替换为服务器 IP 或域名：
```bash
curl -X POST http://your-server-ip:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{"action":"buy","symbol":"BTC"}'
```

### Q: 如何在 TradingView 中使用？

**A:** 在 TradingView Alert 设置中：
1. Webhook URL: `http://your-server-ip:9090/webhook`
2. Message: 
```json
{
  "action": "buy",
  "symbol": "{{ticker}}"
}
```

## 更多资源

- [快速开始指南](QUICK_START.zh-CN.md)
- [示例文档](EXAMPLES.md)
- [README](README.md)
