#!/bin/bash

# TradingView Webhook 测试脚本 (使用 curl)
# 用于模拟 TradingView 发送的警报信号

# 配置
SERVER_URL="http://localhost:9090"
WEBHOOK_SECRET=""  # 如果配置了 webhook_secret，在这里填写

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  TradingView Webhook 测试脚本${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# 构建 Header
if [ -n "$WEBHOOK_SECRET" ]; then
    HEADERS="-H \"Content-Type: application/json\" -H \"X-Webhook-Secret: $WEBHOOK_SECRET\""
else
    HEADERS="-H \"Content-Type: application/json\""
fi

# 测试 1: 健康检查
echo -e "${YELLOW}[1/6] 测试健康检查...${NC}"
curl -s "$SERVER_URL/health" | jq . || echo -e "${RED}❌ 健康检查失败${NC}"
echo ""
sleep 1

# 测试 2: 开多仓 (BTC, 使用默认参数)
echo -e "${YELLOW}[2/6] 测试开多仓 (BTC, 使用默认参数)...${NC}"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  $([ -n "$WEBHOOK_SECRET" ] && echo "-H \"X-Webhook-Secret: $WEBHOOK_SECRET\"") \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }' | jq .
echo ""
sleep 3

# 测试 3: 尝试再次开多仓 (应该被跳过)
echo -e "${YELLOW}[3/6] 测试重复开多仓 (应该被跳过)...${NC}"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  $([ -n "$WEBHOOK_SECRET" ] && echo "-H \"X-Webhook-Secret: $WEBHOOK_SECRET\"") \
  -d '{
    "action": "buy",
    "symbol": "BTC"
  }' | jq .
echo ""
sleep 2

# 测试 4: 平多仓
echo -e "${YELLOW}[4/6] 测试平多仓 (BTC)...${NC}"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  $([ -n "$WEBHOOK_SECRET" ] && echo "-H \"X-Webhook-Secret: $WEBHOOK_SECRET\"") \
  -d '{
    "action": "close_long",
    "symbol": "BTC"
  }' | jq .
echo ""
sleep 3

# 测试 5: 开空仓 (ETH, 指定杠杆)
echo -e "${YELLOW}[5/6] 测试开空仓 (ETH, 3x杠杆)...${NC}"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  $([ -n "$WEBHOOK_SECRET" ] && echo "-H \"X-Webhook-Secret: $WEBHOOK_SECRET\"") \
  -d '{
    "action": "sell",
    "symbol": "ETH",
    "leverage": 3
  }' | jq .
echo ""
sleep 3

# 测试 6: 平空仓
echo -e "${YELLOW}[6/6] 测试平空仓 (ETH)...${NC}"
curl -s -X POST "$SERVER_URL/webhook" \
  -H "Content-Type: application/json" \
  $([ -n "$WEBHOOK_SECRET" ] && echo "-H \"X-Webhook-Secret: $WEBHOOK_SECRET\"") \
  -d '{
    "action": "close_short",
    "symbol": "ETH"
  }' | jq .
echo ""

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✅ 测试完成！${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}提示：${NC}"
echo -e "  • 检查服务器日志查看详细信息"
echo -e "  • 如果测试失败，确认服务器正在运行"
echo -e "  • 如果配置了 webhook_secret，请在脚本中设置"
echo ""
