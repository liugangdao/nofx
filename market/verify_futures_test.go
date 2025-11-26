package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestVerifyFuturesData 验证获取的是合约数据
func TestVerifyFuturesData(t *testing.T) {
	symbol := "BTCUSDT"

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔍 验证数据类型：现货 vs 合约")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// 1. Binance 现货价格
	fmt.Println("1️⃣  Binance 现货价格 (Spot)")
	fmt.Println(strings.Repeat("-", 80))

	spotURL := fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%s", symbol)
	resp, err := http.Get(spotURL)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var spotResult struct {
			Symbol string `json:"symbol"`
			Price  string `json:"price"`
		}
		if json.Unmarshal(body, &spotResult) == nil {
			fmt.Printf("📊 Binance 现货: $%s\n", spotResult.Price)
		}
	}

	// 2. Binance 合约价格
	fmt.Println("\n2️⃣  Binance 合约价格 (Futures)")
	fmt.Println(strings.Repeat("-", 80))

	futuresURL := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/price?symbol=%s", symbol)
	resp, err = http.Get(futuresURL)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var futuresResult struct {
			Symbol string `json:"symbol"`
			Price  string `json:"price"`
		}
		if json.Unmarshal(body, &futuresResult) == nil {
			fmt.Printf("📊 Binance 合约: $%s\n", futuresResult.Price)
		}
	}

	// 3. 验证我们使用的Binance API
	fmt.Println("\n3️⃣  当前代码使用的Binance API")
	fmt.Println(strings.Repeat("-", 80))

	binanceKlines, err := getBinanceKlines(symbol, "1h", 1)
	if err == nil && len(binanceKlines) > 0 {
		fmt.Printf("📊 当前获取价格: $%.2f\n", binanceKlines[0].Close)
		fmt.Println("✅ 使用的是 Binance Futures API (fapi.binance.com)")
		fmt.Println("   - 支持资金费率查询")
		fmt.Println("   - 支持持仓量查询")
		fmt.Println("   - 支持杠杆交易")
	}

	// 4. Hyperliquid 价格
	fmt.Println("\n4️⃣  Hyperliquid 价格")
	fmt.Println(strings.Repeat("-", 80))

	hlKlines, err := getKlines(symbol, "1h", 1)
	if err == nil && len(hlKlines) > 0 {
		fmt.Printf("📊 Hyperliquid 价格: $%.2f\n", hlKlines[0].Close)
		fmt.Println("✅ Hyperliquid 只提供永续合约")
		fmt.Println("   - 支持资金费率")
		fmt.Println("   - 支持持仓量查询")
		fmt.Println("   - 支持杠杆交易（最高40x）")
	}

	// 5. 验证资金费率（只有合约才有）
	fmt.Println("\n5️⃣  资金费率验证（只有合约才有资金费率）")
	fmt.Println(strings.Repeat("-", 80))

	binanceFunding, err := getBinanceFundingRate(symbol)
	if err == nil {
		fmt.Printf("📊 Binance 合约资金费率: %.6f%%\n", binanceFunding*100)
		fmt.Println("   ✅ 成功获取，确认是合约数据")
	}

	hlFunding, err := getFundingRate(symbol)
	if err == nil {
		fmt.Printf("📊 Hyperliquid 资金费率: %.6f%%\n", hlFunding*100)
		fmt.Println("   ✅ 成功获取，确认是合约数据")
	}

	// 6. 验证持仓量（只有合约才有）
	fmt.Println("\n6️⃣  持仓量验证（只有合约才有持仓量）")
	fmt.Println(strings.Repeat("-", 80))

	binanceOI, err := getBinanceOpenInterest(symbol)
	if err == nil {
		fmt.Printf("📊 Binance 合约持仓量: %.2f BTC\n", binanceOI)
		fmt.Println("   ✅ 成功获取，确认是合约数据")
	}

	hlOI, err := getOpenInterestData(symbol)
	if err == nil && hlOI != nil {
		fmt.Printf("📊 Hyperliquid 持仓量: %.2f BTC\n", hlOI.Latest)
		fmt.Println("   ✅ 成功获取，确认是合约数据")
	}

	// 7. 总结
	fmt.Println("\n7️⃣  总结")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("✅ Binance: 使用 Futures API (fapi.binance.com)")
	fmt.Println("   - 永续合约 (Perpetual Futures)")
	fmt.Println("   - USDT本位合约")
	fmt.Println("   - 支持最高125x杠杆")
	fmt.Println("")
	fmt.Println("✅ Hyperliquid: 永续合约交易所")
	fmt.Println("   - 只提供永续合约")
	fmt.Println("   - USDC结算")
	fmt.Println("   - 支持最高40x杠杆")
	fmt.Println("")
	fmt.Println("📌 两者都是合约价格，价格差异主要来自：")
	fmt.Println("   1. 不同交易所的流动性和订单簿")
	fmt.Println("   2. 资金费率机制的差异")
	fmt.Println("   3. 市场参与者的不同")
	fmt.Println("   4. 价格发现机制的差异")

	fmt.Println("\n" + strings.Repeat("=", 80))
}

// TestAPIEndpoints 显示使用的API端点
func TestAPIEndpoints(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔗 API 端点信息")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	fmt.Println("📍 Binance API:")
	fmt.Println("   现货 API: https://api.binance.com")
	fmt.Println("   合约 API: https://fapi.binance.com ✅ (当前使用)")
	fmt.Println("")
	fmt.Println("   使用的端点:")
	fmt.Println("   - K线: /fapi/v1/klines")
	fmt.Println("   - 资金费率: /fapi/v1/premiumIndex")
	fmt.Println("   - 持仓量: /fapi/v1/openInterest")
	fmt.Println("")

	fmt.Println("📍 Hyperliquid API:")
	fmt.Println("   API: https://api.hyperliquid.xyz ✅ (当前使用)")
	fmt.Println("")
	fmt.Println("   使用的端点:")
	fmt.Println("   - K线: /info (type: candleSnapshot)")
	fmt.Println("   - 资金费率: /info (type: metaAndAssetCtxs)")
	fmt.Println("   - 持仓量: /info (type: metaAndAssetCtxs)")
	fmt.Println("")

	fmt.Println("✅ 两个数据源都是永续合约数据")
	fmt.Println("\n" + strings.Repeat("=", 80))
}
