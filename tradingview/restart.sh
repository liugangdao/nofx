#!/bin/bash

# TradingView Webhook 重启脚本

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  TradingView Webhook 重启脚本${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# 停止服务
echo -e "${YELLOW}1️⃣ 停止服务...${NC}"
./stop.sh

# 等待一下
sleep 2

# 启动服务
echo ""
echo -e "${YELLOW}2️⃣ 启动服务...${NC}"
./start.sh
