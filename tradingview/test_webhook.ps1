# TradingView Webhook 测试脚本 (PowerShell)
# 用于模拟 TradingView 发送的警报信号

# 配置
$SERVER_URL = "http://localhost:9090"
$WEBHOOK_SECRET = ""  # 如果配置了 webhook_secret，在这里填写

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "  TradingView Webhook 测试脚本" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

# 构建 Headers
$headers = @{
    "Content-Type" = "application/json"
}
if ($WEBHOOK_SECRET) {
    $headers["X-Webhook-Secret"] = $WEBHOOK_SECRET
}

# 测试 1: 健康检查
Write-Host "[1/6] 测试健康检查..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$SERVER_URL/health" -Method Get
    Write-Host "✅ 服务器正常运行" -ForegroundColor Green
    $response | ConvertTo-Json
} catch {
    Write-Host "❌ 健康检查失败: $_" -ForegroundColor Red
}
Write-Host ""
Start-Sleep -Seconds 1

# 测试 2: 开多仓 (BTC, 使用默认参数)
Write-Host "[2/6] 测试开多仓 (BTC, 使用默认参数)..." -ForegroundColor Yellow
$body = @{
    action = "buy"
    symbol = "BTC"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$SERVER_URL/webhook" -Method Post -Body $body -Headers $headers
    Write-Host "✅ 请求成功" -ForegroundColor Green
    $response | ConvertTo-Json
} catch {
    Write-Host "❌ 错误: $_" -ForegroundColor Red
}
Write-Host ""
Start-Sleep -Seconds 3

# 测试 3: 尝试再次开多仓 (应该被跳过)
Write-Host "[3/6] 测试重复开多仓 (应该被跳过)..." -ForegroundColor Yellow
$body = @{
    action = "buy"
    symbol = "BTC"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$SERVER_URL/webhook" -Method Post -Body $body -Headers $headers
    if ($response.status -eq "skipped") {
        Write-Host "✅ 仓位保护生效，跳过重复开仓" -ForegroundColor Green
    }
    $response | ConvertTo-Json
} catch {
    Write-Host "❌ 错误: $_" -ForegroundColor Red
}
Write-Host ""
Start-Sleep -Seconds 2

# 测试 4: 平多仓
Write-Host "[4/6] 测试平多仓 (BTC)..." -ForegroundColor Yellow
$body = @{
    action = "close_long"
    symbol = "BTC"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$SERVER_URL/webhook" -Method Post -Body $body -Headers $headers
    Write-Host "✅ 请求成功" -ForegroundColor Green
    $response | ConvertTo-Json
} catch {
    Write-Host "❌ 错误: $_" -ForegroundColor Red
}
Write-Host ""
Start-Sleep -Seconds 3

# 测试 5: 开空仓 (ETH, 指定杠杆)
Write-Host "[5/6] 测试开空仓 (ETH, 3x杠杆)..." -ForegroundColor Yellow
$body = @{
    action = "sell"
    symbol = "ETH"
    leverage = 3
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$SERVER_URL/webhook" -Method Post -Body $body -Headers $headers
    Write-Host "✅ 请求成功" -ForegroundColor Green
    $response | ConvertTo-Json
} catch {
    Write-Host "❌ 错误: $_" -ForegroundColor Red
}
Write-Host ""
Start-Sleep -Seconds 3

# 测试 6: 平空仓
Write-Host "[6/6] 测试平空仓 (ETH)..." -ForegroundColor Yellow
$body = @{
    action = "close_short"
    symbol = "ETH"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$SERVER_URL/webhook" -Method Post -Body $body -Headers $headers
    Write-Host "✅ 请求成功" -ForegroundColor Green
    $response | ConvertTo-Json
} catch {
    Write-Host "❌ 错误: $_" -ForegroundColor Red
}
Write-Host ""

Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "✅ 测试完成！" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host ""
Write-Host "提示：" -ForegroundColor Cyan
Write-Host "  • 检查服务器日志查看详细信息" -ForegroundColor White
Write-Host "  • 如果测试失败，确认服务器正在运行" -ForegroundColor White
Write-Host "  • 如果配置了 webhook_secret，请在脚本中设置" -ForegroundColor White
Write-Host ""
