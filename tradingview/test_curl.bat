@echo off
REM TradingView Webhook 测试脚本 (Windows CMD)
REM 用于模拟 TradingView 发送的警报信号

setlocal enabledelayedexpansion

REM 配置
set SERVER_URL=http://localhost:9090
set WEBHOOK_SECRET=

echo ========================================
echo   TradingView Webhook 测试脚本
echo ========================================
echo.

REM 测试 1: 健康检查
echo [1/6] 测试健康检查...
curl -s "%SERVER_URL%/health"
echo.
timeout /t 1 /nobreak >nul

REM 测试 2: 开多仓 (BTC, 使用默认参数)
echo [2/6] 测试开多仓 (BTC, 使用默认参数)...
curl -s -X POST "%SERVER_URL%/webhook" ^
  -H "Content-Type: application/json" ^
  -d "{\"action\":\"buy\",\"symbol\":\"BTC\"}"
echo.
timeout /t 3 /nobreak >nul

REM 测试 3: 尝试再次开多仓 (应该被跳过)
echo [3/6] 测试重复开多仓 (应该被跳过)...
curl -s -X POST "%SERVER_URL%/webhook" ^
  -H "Content-Type: application/json" ^
  -d "{\"action\":\"buy\",\"symbol\":\"BTC\"}"
echo.
timeout /t 2 /nobreak >nul

REM 测试 4: 平多仓
echo [4/6] 测试平多仓 (BTC)...
curl -s -X POST "%SERVER_URL%/webhook" ^
  -H "Content-Type: application/json" ^
  -d "{\"action\":\"close_long\",\"symbol\":\"BTC\"}"
echo.
timeout /t 3 /nobreak >nul

REM 测试 5: 开空仓 (ETH, 指定杠杆)
echo [5/6] 测试开空仓 (ETH, 3x杠杆)...
curl -s -X POST "%SERVER_URL%/webhook" ^
  -H "Content-Type: application/json" ^
  -d "{\"action\":\"sell\",\"symbol\":\"ETH\",\"leverage\":3}"
echo.
timeout /t 3 /nobreak >nul

REM 测试 6: 平空仓
echo [6/6] 测试平空仓 (ETH)...
curl -s -X POST "%SERVER_URL%/webhook" ^
  -H "Content-Type: application/json" ^
  -d "{\"action\":\"close_short\",\"symbol\":\"ETH\"}"
echo.

echo ========================================
echo 测试完成！
echo ========================================
echo.
echo 提示：
echo   - 检查服务器日志查看详细信息
echo   - 如果测试失败，确认服务器正在运行
echo   - 如果配置了 webhook_secret，请在脚本中设置
echo.

pause
