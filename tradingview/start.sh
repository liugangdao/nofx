#!/bin/bash

# TradingView Webhook 启动脚本
# 使用 nohup 在后台运行

# 配置
CONFIG_FILE="tradingview_config.json"
LOG_FILE="tradingview_webhook.log"
PID_FILE="tradingview_webhook.pid"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  TradingView Webhook 启动脚本${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ 配置文件不存在: $CONFIG_FILE${NC}"
    echo -e "${YELLOW}请先复制并配置: cp tradingview_config.json.example $CONFIG_FILE${NC}"
    exit 1
fi

# 检查可执行文件
if [ ! -f "tradingview_webhook" ]; then
    echo -e "${YELLOW}⚠️  可执行文件不存在，正在编译...${NC}"
    go build -o tradingview_webhook main.go
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ 编译失败${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ 编译成功${NC}"
fi

# 检查是否已经在运行
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if ps -p "$OLD_PID" > /dev/null 2>&1; then
        echo -e "${YELLOW}⚠️  服务已在运行 (PID: $OLD_PID)${NC}"
        echo -e "${YELLOW}如需重启，请先运行: ./stop.sh${NC}"
        exit 1
    else
        echo -e "${YELLOW}⚠️  发现旧的 PID 文件，正在清理...${NC}"
        rm -f "$PID_FILE"
    fi
fi

# 启动服务
echo -e "${YELLOW}🚀 正在启动服务...${NC}"
nohup ./tradingview_webhook -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
PID=$!

# 保存 PID
echo $PID > "$PID_FILE"

# 等待服务启动
sleep 2

# 检查服务是否成功启动
if ps -p "$PID" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 服务启动成功！${NC}"
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "  PID: ${GREEN}$PID${NC}"
    echo -e "  日志文件: ${GREEN}$LOG_FILE${NC}"
    echo -e "  PID 文件: ${GREEN}$PID_FILE${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "${YELLOW}常用命令:${NC}"
    echo -e "  查看日志: ${GREEN}tail -f $LOG_FILE${NC}"
    echo -e "  查看状态: ${GREEN}./status.sh${NC}"
    echo -e "  停止服务: ${GREEN}./stop.sh${NC}"
    echo -e "  重启服务: ${GREEN}./restart.sh${NC}"
    echo ""
    
    # 显示最后几行日志
    echo -e "${YELLOW}最新日志:${NC}"
    tail -n 10 "$LOG_FILE"
else
    echo -e "${RED}❌ 服务启动失败${NC}"
    echo -e "${YELLOW}请查看日志: cat $LOG_FILE${NC}"
    rm -f "$PID_FILE"
    exit 1
fi
