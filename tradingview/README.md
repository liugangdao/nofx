# TradingView Webhook → Hyperliquid 自动交易模块

这是一个独立的 Golang 模块，用于接收 TradingView 的 Webhook 信号并自动在 Hyperliquid 交易所执行交易。

## 功能特性

- ✅ 接收 TradingView Webhook 信号
- ✅ 支持做多、做空、平仓操作
- ✅ 智能资金管理：自动使用账户资金的指定百分比（默认 5%）
- ✅ 仓位保护：自动检测现有仓位，避免重复开仓
- ✅ 自动处理价格和数量精度
- ✅ 支持自定义杠杆倍数
- ✅ Webhook 密钥验证（可选）
- ✅ 完整的错误处理和日志记录
- ✅ 支持主网和测试网

## 快速开始

### 1. 配置文件

复制配置文件模板并填写你的信息：

```bash
cp tradingview_config.json.example tradingview_config.json
```

编辑 `tradingview_config.json`：

```json
{
  "port": 9090,
  "private_key": "你的以太坊私钥（不带0x前缀）",
  "wallet_addr": "0x你的以太坊地址",
  "testnet": false,
  "default_quantity": 0.01,
  "default_leverage": 5,
  "webhook_secret": "你的webhook密钥（可选）"
}
```

### 2. 编译运行

```bash
# 编译
go build -o tradingview_webhook main.go

# 运行
./tradingview_webhook -config tradingview_config.json
```

或者直接运行：

```bash
go run main.go -config tradingview_config.json
```

### 3. 配置 TradingView Alert

在 TradingView 中创建 Alert，Webhook URL 设置为：

```
http://你的服务器IP:9090/webhook
```

如果配置了 `webhook_secret`，需要在 TradingView Alert 的 Message 中添加 Header：

```
X-Webhook-Secret: 你的webhook密钥
```

## TradingView 信号格式

### 做多（开多仓）

```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.01,
  "leverage": 5
}
```

或者使用 `"action": "long"`

### 做空（开空仓）

```json
{
  "action": "sell",
  "symbol": "ETHUSDT",
  "quantity": 0.05,
  "leverage": 3
}
```

或者使用 `"action": "short"`

### 平多仓

```json
{
  "action": "close_long",
  "symbol": "BTCUSDT",
  "quantity": 0
}
```

`quantity` 为 0 时会自动平掉所有多仓。也可以使用 `"action": "close"`

### 平空仓

```json
{
  "action": "close_short",
  "symbol": "ETHUSDT",
  "quantity": 0
}
```

`quantity` 为 0 时会自动平掉所有空仓。

## 参数说明

### 必填参数

- `action`: 操作类型
  - `buy` / `long`: 开多仓
  - `sell` / `short`: 开空仓
  - `close_long` / `close`: 平多仓
  - `close_short`: 平空仓

- `symbol`: 交易对（如 `BTCUSDT`、`ETHUSDT`）

### 可选参数

- `quantity`: 下单数量（默认使用配置文件中的 `default_quantity`）
- `leverage`: 杠杆倍数（默认使用配置文件中的 `default_leverage`）
- `price`: 当前价格（仅用于日志记录，不影响实际下单）

## API 端点

### POST /webhook

接收 TradingView Webhook 信号

**请求示例：**

```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -H "X-Webhook-Secret: your_secret_key" \
  -d '{
    "action": "buy",
    "symbol": "BTCUSDT",
    "quantity": 0.01,
    "leverage": 5
  }'
```

**响应示例：**

```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.01,
  "price": 45123.45,
  "leverage": 5,
  "status": "success"
}
```

### GET /health

健康检查端点

```bash
curl http://localhost:9090/health
```

## 安全建议

1. **使用 Webhook 密钥**：在配置文件中设置 `webhook_secret`，防止未授权的请求
2. **使用 HTTPS**：在生产环境中使用反向代理（如 Nginx）配置 HTTPS
3. **IP 白名单**：限制只允许 TradingView 的 IP 访问
4. **私钥安全**：妥善保管配置文件，不要提交到 Git 仓库

## 部署建议

### 使用 systemd（Linux）

创建服务文件 `/etc/systemd/system/tradingview-webhook.service`：

```ini
[Unit]
Description=TradingView Webhook Service
After=network.target

[Service]
Type=simple
User=your_user
WorkingDirectory=/path/to/tradingview
ExecStart=/path/to/tradingview/tradingview_webhook -config /path/to/tradingview/tradingview_config.json
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable tradingview-webhook
sudo systemctl start tradingview-webhook
sudo systemctl status tradingview-webhook
```

### 使用 Docker

创建 `Dockerfile`：

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o tradingview_webhook main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/tradingview_webhook .
COPY tradingview_config.json .
EXPOSE 9090
CMD ["./tradingview_webhook", "-config", "tradingview_config.json"]
```

构建并运行：

```bash
docker build -t tradingview-webhook .
docker run -d -p 9090:9090 --name tradingview-webhook tradingview-webhook
```

### 使用 Nginx 反向代理（HTTPS）

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location /webhook {
        proxy_pass http://localhost:9090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 日志示例

```
✓ Hyperliquid交易器初始化成功 (testnet=false, wallet=0xYourAddress)
🌐 TradingView Webhook服务器启动在 http://localhost:9090
📡 Webhook端点: POST http://localhost:9090/webhook
💡 TradingView Alert配置示例:
  {
    "action": "buy",
    "symbol": "BTCUSDT",
    "quantity": 0.01,
    "leverage": 5
  }

📨 收到TradingView信号: {action:buy symbol:BTCUSDT quantity:0.01 leverage:5}
📈 开多仓: BTCUSDT 数量: 0.0100 杠杆: 5x
  ✓ BTCUSDT 杠杆已设置为 5x
  📏 数量: 0.01000000 -> 0.01000000
  💰 价格: 45578.12340000 -> 45578.00000000
✓ 开多仓成功: BTCUSDT 数量: 0.0100
✓ 信号处理成功: map[action:buy leverage:5 price:45578 quantity:0.01 status:success symbol:BTCUSDT]
```

## 故障排查

### 1. 连接失败

检查网络连接和 Hyperliquid API 状态：

```bash
curl https://api.hyperliquid.xyz/info
```

### 2. 私钥错误

确保私钥格式正确（不带 `0x` 前缀），长度为 64 个十六进制字符。

### 3. 精度错误

模块会自动处理价格和数量精度，如果仍然出错，检查 Hyperliquid 的最新精度要求。

### 4. Webhook 未收到

- 检查防火墙设置
- 确认端口已开放
- 使用 `curl` 测试本地连接
- 检查 TradingView Alert 配置

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
