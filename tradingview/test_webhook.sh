#!/bin/bash

# TradingView Webhook 测试脚本

SERVER_URL="http://localhost:9090"
WEBHOOK_SECRET="your_secret_key"

echo "🧪 TradingView Webhook 测试脚本"
echo "================================"
echo ""

# 测试健康检查
echo "1️⃣ 测试健康检查..."
curl -s "$SERVER_URL/health" | jq .
echo ""
echo ""

# 测试开多仓
echo "2️⃣ 测试开多仓 (BTC)..."
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: $WEBHOOK_SECRET" \
  -d '{
    "action": "buy",
    "symbol": "BTCUSDT",
    "quantity": 0.001,
    "leverage": 3
  }' | jq .
echo ""
echo ""

# 等待3秒
echo "⏳ 等待3秒..."
sleep 3
echo ""

# 测试平多仓
echo "3️⃣ 测试平多仓 (BTC)..."
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: $WEBHOOK_SECRET" \
  -d '{
    "action": "close_long",
    "symbol": "BTCUSDT",
    "quantity": 0
  }' | jq .
echo ""
echo ""

# 等待3秒
echo "⏳ 等待3秒..."
sleep 3
echo ""

# 测试开空仓
echo "4️⃣ 测试开空仓 (ETH)..."
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: $WEBHOOK_SECRET" \
  -d '{
    "action": "sell",
    "symbol": "ETHUSDT",
    "quantity": 0.01,
    "leverage": 5
  }' | jq .
echo ""
echo ""

# 等待3秒
echo "⏳ 等待3秒..."
sleep 3
echo ""

# 测试平空仓
echo "5️⃣ 测试平空仓 (ETH)..."
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: $WEBHOOK_SECRET" \
  -d '{
    "action": "close_short",
    "symbol": "ETHUSDT",
    "quantity": 0
  }' | jq .
echo ""
echo ""

echo "✅ 测试完成！"
