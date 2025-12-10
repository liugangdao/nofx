# TradingView → Hyperliquid 快速开始指南

## 核心特性

✅ **智能资金管理**：自动使用账户资金的指定百分比（默认 5%）开仓
✅ **仓位保护**：如果该币种已有仓位，自动跳过开仓信号，避免重复开仓
✅ **灵活配置**：支持固定数量模式和资金百分比模式
✅ **完整日志**：详细的操作日志，方便监控和调试

## 1. 准备工作

### 1.1 获取 Hyperliquid 钱包信息

1. 准备一个以太坊钱包（MetaMask 等）
2. 获取钱包地址（如：`0x1234...`）
3. 导出私钥（**注意安全！**）

### 1.2 安装 Golang

如果还没有安装 Golang，请访问 https://golang.org/dl/ 下载安装。

## 2. 配置

### 2.1 复制配置文件

```bash
cd tradingview
cp tradingview_config.json.example tradingview_config.json
```

### 2.2 编辑配置文件

打开 `tradingview_config.json`，填写你的信息：

```json
{
  "port": 9090,
  "private_key": "你的私钥（不带0x前缀）",
  "wallet_addr": "0x你的钱包地址",
  "testnet": false,
  "default_quantity": 0,
  "position_size_percent": 5.0,
  "default_leverage": 5,
  "webhook_secret": "my_secret_123"
}
```

**配置说明：**
- `default_quantity: 0` 表示使用资金百分比模式
- `position_size_percent: 5.0` 表示每次开仓使用账户资金的 5%
- 如果想使用固定数量，可以设置 `default_quantity: 0.01`（此时 `position_size_percent` 会被忽略）

**重要参数说明：**

- `port`: Webhook 服务器端口（默认 9090）
- `private_key`: 以太坊私钥，**不要带 0x 前缀**
- `wallet_addr`: 以太坊钱包地址
- `testnet`: 是否使用测试网（建议先用测试网测试）
- `default_quantity`: 固定下单数量（如果为 0 则使用资金百分比模式）
- `position_size_percent`: 资金百分比模式的百分比（默认 5%，即每次用账户资金的 5% 开仓）
- `default_leverage`: 默认杠杆倍数（1-50）
- `webhook_secret`: Webhook 密钥（用于验证请求，可选但推荐）

**下单模式说明：**

1. **固定数量模式**：设置 `default_quantity` 为具体数值（如 0.01），每次开仓使用固定数量
2. **资金百分比模式**（推荐）：设置 `default_quantity` 为 0，配置 `position_size_percent`（如 5.0），每次开仓自动使用账户资金的指定百分比

## 3. 运行

### 方法 1：直接运行（推荐用于测试）

```bash
go run main.go -config tradingview_config.json
```

### 方法 2：编译后运行

```bash
# 编译
go build -o tradingview_webhook main.go

# 运行
./tradingview_webhook -config tradingview_config.json
```

### 方法 3：使用 Makefile

```bash
# 编译
make build

# 运行
make run
```

## 4. 网络配置（如果需要外部访问）

服务器默认监听 `0.0.0.0:9090`，可以从外部访问。

### 4.1 开放防火墙端口

**Linux (UFW):**
```bash
sudo ufw allow 9090/tcp
```

**Windows PowerShell (管理员):**
```powershell
New-NetFirewallRule -DisplayName "TradingView Webhook" -Direction Inbound -LocalPort 9090 -Protocol TCP -Action Allow
```

**云服务器：** 在安全组中开放 9090 端口

详细配置请查看 [网络配置指南](网络配置指南.md)

### 4.2 获取服务器 IP

```bash
# 查看公网 IP
curl ifconfig.me
```

## 5. 配置 TradingView

### 5.1 创建 Alert

1. 在 TradingView 图表上右键 → "Add alert"
2. 在 Alert 设置中：
   - Condition: 设置你的触发条件
   - Options: 勾选 "Webhook URL"
   - Webhook URL: `http://你的服务器IP:9090/webhook`
   - 或使用域名：`https://trading.example.com/webhook`（推荐）

### 5.2 设置 Alert Message

在 "Message" 字段中输入 JSON 格式的信号：

**开多仓示例：**
```json
{
  "action": "buy",
  "symbol": "BTCUSDT",
  "quantity": 0.01,
  "leverage": 5
}
```

**开空仓示例：**
```json
{
  "action": "sell",
  "symbol": "ETHUSDT",
  "quantity": 0.05,
  "leverage": 3
}
```

**平多仓示例：**
```json
{
  "action": "close_long",
  "symbol": "BTCUSDT"
}
```

**平空仓示例：**
```json
{
  "action": "close_short",
  "symbol": "ETHUSDT"
}
```

### 5.3 添加 Webhook 密钥（如果配置了）

TradingView 不支持自定义 HTTP Header，所以如果你配置了 `webhook_secret`，需要在生产环境中使用反向代理（如 Nginx）来添加 Header。

**临时测试方案：** 将配置文件中的 `webhook_secret` 设置为空字符串 `""`。

## 6. 测试

### 5.1 健康检查

```bash
curl http://localhost:9090/health
```

应该返回：
```json
{"status":"ok"}
```

### 5.2 手动测试 Webhook

**开多仓：**
```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "buy",
    "symbol": "BTCUSDT",
    "quantity": 0.001,
    "leverage": 3
  }'
```

**平多仓：**
```bash
curl -X POST http://localhost:9090/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "action": "close_long",
    "symbol": "BTCUSDT"
  }'
```

### 5.3 使用测试脚本

**Linux/Mac：**
```bash
bash test_webhook.sh
```

**Windows PowerShell：**
```powershell
.\test_webhook.ps1
```

## 7. 部署到服务器

### 6.1 使用 systemd（Linux）

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

### 6.2 配置防火墙

```bash
# Ubuntu/Debian
sudo ufw allow 9090/tcp

# CentOS/RHEL
sudo firewall-cmd --permanent --add-port=9090/tcp
sudo firewall-cmd --reload
```

### 6.3 使用 Nginx 反向代理（推荐）

安装 SSL 证书后，配置 Nginx：

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
        proxy_set_header X-Webhook-Secret "your_secret_key";
    }
}
```

然后在 TradingView 中使用：
```
https://your-domain.com/webhook
```

## 8. 常见问题

### Q1: 连接失败

**检查：**
- 服务器是否正常运行？
- 防火墙是否开放端口？
- 网络是否可达？

### Q2: 私钥错误

**确保：**
- 私钥不带 `0x` 前缀
- 私钥长度为 64 个十六进制字符
- 私钥对应的钱包地址正确

### Q3: 下单失败

**可能原因：**
- 账户余额不足
- 数量太小（低于最小下单量）
- 杠杆设置不合理
- 网络问题

**解决方法：**
- 查看日志输出
- 先在测试网测试
- 调整 `position_size_percent` 参数

### Q4: 收到信号但没有开仓

**可能原因：**
- 该币种已有仓位（系统会自动跳过，避免重复开仓）
- 账户余额不足
- 计算出的数量低于最小下单量

**日志示例：**
```
⚠️ BTCUSDT 已有 long 仓位，跳过开仓
```

这是正常的保护机制，确保不会在同一币种上重复开仓。

### Q5: TradingView Alert 不触发

**检查：**
- Webhook URL 是否正确
- 服务器是否可从外网访问
- Alert 条件是否满足
- TradingView 账户是否支持 Webhook（需要 Pro 账户）

## 9. 安全建议

1. **私钥安全**
   - 不要将配置文件提交到 Git
   - 使用环境变量存储敏感信息
   - 定期更换密钥

2. **网络安全**
   - 使用 HTTPS（SSL/TLS）
   - 配置 Webhook 密钥验证
   - 限制 IP 白名单

3. **资金安全**
   - 先在测试网测试
   - 使用小额资金测试
   - 设置合理的杠杆倍数
   - 定期检查交易记录

## 10. 日志查看

程序运行时会输出详细日志：

```
✓ Hyperliquid交易器初始化成功 (testnet=false, wallet=0xYourAddress)
🌐 TradingView Webhook服务器启动在 http://localhost:9090
📡 Webhook端点: POST http://localhost:9090/webhook

📨 收到TradingView信号: {action:buy symbol:BTCUSDT quantity:0.01 leverage:5}
📈 开多仓: BTCUSDT 数量: 0.0100 杠杆: 5x
  ✓ BTCUSDT 杠杆已设置为 5x
  📏 数量: 0.01000000 -> 0.01000000
  💰 价格: 45578.12340000 -> 45578.00000000
✓ 开多仓成功: BTCUSDT 数量: 0.0100
```

## 11. 获取帮助

如果遇到问题：

1. 查看日志输出
2. 检查配置文件
3. 使用测试脚本验证
4. 在测试网环境测试
5. 提交 Issue 到 GitHub

---

**祝交易顺利！** 🚀
