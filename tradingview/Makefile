.PHONY: build run test clean help

# 默认目标
help:
	@echo "TradingView Webhook 构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  make build   - 编译程序"
	@echo "  make run     - 运行程序"
	@echo "  make test    - 运行测试"
	@echo "  make clean   - 清理编译文件"
	@echo "  make install - 安装依赖"

# 编译
build:
	@echo "🔨 编译 TradingView Webhook..."
	go build -o tradingview_webhook main.go
	@echo "✅ 编译完成！"

# 运行
run:
	@echo "🚀 启动 TradingView Webhook..."
	go run main.go -config tradingview_config.json

# 测试
test:
	@echo "🧪 运行测试..."
	@if [ -f test_webhook.sh ]; then \
		bash test_webhook.sh; \
	else \
		echo "❌ 测试脚本不存在"; \
	fi

# 清理
clean:
	@echo "🧹 清理编译文件..."
	rm -f tradingview_webhook
	@echo "✅ 清理完成！"

# 安装依赖
install:
	@echo "📦 安装依赖..."
	go mod tidy
	@echo "✅ 依赖安装完成！"

# 编译 Windows 版本
build-windows:
	@echo "🔨 编译 Windows 版本..."
	GOOS=windows GOARCH=amd64 go build -o tradingview_webhook.exe main.go
	@echo "✅ Windows 版本编译完成！"

# 编译 Linux 版本
build-linux:
	@echo "🔨 编译 Linux 版本..."
	GOOS=linux GOARCH=amd64 go build -o tradingview_webhook_linux main.go
	@echo "✅ Linux 版本编译完成！"

# 编译所有平台
build-all: build-windows build-linux
	@echo "✅ 所有平台编译完成！"

# 启动服务（后台）
start:
	@echo "🚀 启动服务..."
	@chmod +x start.sh
	@./start.sh

# 停止服务
stop:
	@echo "🛑 停止服务..."
	@chmod +x stop.sh
	@./stop.sh

# 重启服务
restart:
	@echo "🔄 重启服务..."
	@chmod +x restart.sh
	@./restart.sh

# 查看状态
status:
	@chmod +x status.sh
	@./status.sh

# 查看日志
logs:
	@chmod +x logs.sh
	@./logs.sh -f

# 添加执行权限
chmod:
	@echo "📝 添加执行权限..."
	@chmod +x start.sh stop.sh restart.sh status.sh logs.sh test_curl.sh
	@echo "✅ 权限设置完成！"
