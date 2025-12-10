#!/bin/bash

# TradingView Webhook 停止脚本

# 配置
PID_FILE="tradingview_webhook.pid"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}  TradingView Webhook 停止脚本${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# 检查 PID 文件
if [ ! -f "$PID_FILE" ]; then
    echo -e "${RED}❌ PID 文件不存在，服务可能未运行${NC}"
    exit 1
fi

# 读取 PID
PID=$(cat "$PID_FILE")

# 检查进程是否存在
if ! ps -p "$PID" > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  进程不存在 (PID: $PID)${NC}"
    echo -e "${YELLOW}正在清理 PID 文件...${NC}"
    rm -f "$PID_FILE"
    exit 0
fi

# 停止进程
echo -e "${YELLOW}🛑 正在停止服务 (PID: $PID)...${NC}"
kill "$PID"

# 等待进程结束
for i in {1..10}; do
    if ! ps -p "$PID" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ 服务已停止${NC}"
        rm -f "$PID_FILE"
        exit 0
    fi
    sleep 1
done

# 如果进程还在运行，强制终止
echo -e "${YELLOW}⚠️  进程未响应，正在强制终止...${NC}"
kill -9 "$PID"
sleep 1

if ! ps -p "$PID" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 服务已强制停止${NC}"
    rm -f "$PID_FILE"
else
    echo -e "${RED}❌ 无法停止服务${NC}"
    exit 1
fi
