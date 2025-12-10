#!/bin/bash

# TradingView Webhook 日志查看脚本

# 配置
LOG_FILE="tradingview_webhook.log"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查日志文件
if [ ! -f "$LOG_FILE" ]; then
    echo -e "${RED}❌ 日志文件不存在: $LOG_FILE${NC}"
    exit 1
fi

# 解析参数
case "$1" in
    -f|--follow)
        echo -e "${GREEN}📋 实时查看日志 (Ctrl+C 退出)${NC}"
        echo ""
        tail -f "$LOG_FILE"
        ;;
    -n|--lines)
        LINES=${2:-50}
        echo -e "${GREEN}📋 最后 $LINES 行日志${NC}"
        echo ""
        tail -n "$LINES" "$LOG_FILE"
        ;;
    -e|--error)
        echo -e "${RED}📋 错误日志${NC}"
        echo ""
        grep -i "error\|failed\|❌" "$LOG_FILE" | tail -n 50
        ;;
    -s|--success)
        echo -e "${GREEN}📋 成功日志${NC}"
        echo ""
        grep -i "success\|✓\|✅" "$LOG_FILE" | tail -n 50
        ;;
    -c|--clear)
        echo -e "${YELLOW}⚠️  确定要清空日志吗? (y/N)${NC}"
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            > "$LOG_FILE"
            echo -e "${GREEN}✅ 日志已清空${NC}"
        else
            echo -e "${YELLOW}已取消${NC}"
        fi
        ;;
    -h|--help|*)
        echo -e "${GREEN}TradingView Webhook 日志查看工具${NC}"
        echo ""
        echo "用法: ./logs.sh [选项]"
        echo ""
        echo "选项:"
        echo "  -f, --follow       实时查看日志"
        echo "  -n, --lines [N]    查看最后 N 行 (默认 50)"
        echo "  -e, --error        只显示错误日志"
        echo "  -s, --success      只显示成功日志"
        echo "  -c, --clear        清空日志文件"
        echo "  -h, --help         显示帮助信息"
        echo ""
        echo "示例:"
        echo "  ./logs.sh -f              # 实时查看"
        echo "  ./logs.sh -n 100          # 查看最后 100 行"
        echo "  ./logs.sh -e              # 只看错误"
        echo "  ./logs.sh                 # 查看最后 50 行"
        ;;
esac
