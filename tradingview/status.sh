#!/bin/bash

# TradingView Webhook 状态检查脚本

# 配置
PID_FILE="tradingview_webhook.pid"
LOG_FILE="tradingview_webhook.log"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  TradingView Webhook 状态${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# 检查 PID 文件
if [ ! -f "$PID_FILE" ]; then
    echo -e "${RED}❌ 服务未运行${NC}"
    echo -e "${YELLOW}提示: 运行 ./start.sh 启动服务${NC}"
    exit 1
fi

# 读取 PID
PID=$(cat "$PID_FILE")

# 检查进程是否存在
if ! ps -p "$PID" > /dev/null 2>&1; then
    echo -e "${RED}❌ 服务未运行 (PID 文件存在但进程不存在)${NC}"
    echo -e "${YELLOW}提示: 运行 ./start.sh 启动服务${NC}"
    exit 1
fi

# 服务正在运行
echo -e "${GREEN}✅ 服务正在运行${NC}"
echo ""

# 显示进程信息
echo -e "${YELLOW}进程信息:${NC}"
echo -e "  PID: ${GREEN}$PID${NC}"
ps -p "$PID" -o pid,ppid,%cpu,%mem,etime,cmd --no-headers | while read line; do
    echo -e "  $line"
done
echo ""

# 显示端口监听
echo -e "${YELLOW}端口监听:${NC}"
netstat -tlnp 2>/dev/null | grep "$PID" | grep -v "127.0.0.1" || echo -e "  ${YELLOW}(需要 root 权限查看详细信息)${NC}"
echo ""

# 显示日志文件信息
if [ -f "$LOG_FILE" ]; then
    echo -e "${YELLOW}日志文件:${NC}"
    echo -e "  文件: ${GREEN}$LOG_FILE${NC}"
    echo -e "  大小: ${GREEN}$(du -h "$LOG_FILE" | cut -f1)${NC}"
    echo -e "  最后修改: ${GREEN}$(stat -c %y "$LOG_FILE" 2>/dev/null || stat -f "%Sm" "$LOG_FILE")${NC}"
    echo ""
    
    echo -e "${YELLOW}最新日志 (最后 15 行):${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    tail -n 15 "$LOG_FILE"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
else
    echo -e "${YELLOW}⚠️  日志文件不存在${NC}"
fi

echo ""
echo -e "${YELLOW}常用命令:${NC}"
echo -e "  查看实时日志: ${GREEN}tail -f $LOG_FILE${NC}"
echo -e "  停止服务: ${GREEN}./stop.sh${NC}"
echo -e "  重启服务: ${GREEN}./restart.sh${NC}"
echo -e "  测试服务: ${GREEN}curl http://localhost:9090/health${NC}"
