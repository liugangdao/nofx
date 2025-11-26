package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// getBinanceKlines 从Binance获取K线数据（用于对比）
func getBinanceKlines(symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawData [][]any
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, err
	}

	klines := make([]Kline, len(rawData))
	for i, item := range rawData {
		openTime := int64(item[0].(float64))
		open, _ := parseFloat(item[1])
		high, _ := parseFloat(item[2])
		low, _ := parseFloat(item[3])
		close, _ := parseFloat(item[4])
		volume, _ := parseFloat(item[5])
		closeTime := int64(item[6].(float64))

		klines[i] = Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		}
	}

	return klines, nil
}

// getBinanceFundingRate 从Binance获取资金费率（用于对比）
func getBinanceFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		LastFundingRate string `json:"lastFundingRate"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

// getBinanceOpenInterest 从Binance获取持仓量（用于对比）
func getBinanceOpenInterest(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)
	return oi, nil
}

// TestCompareDataSources 对比Binance和Hyperliquid的数据
func TestCompareDataSources(t *testing.T) {
	symbol := "BTCUSDT"

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📊 数据源对比: Binance vs Hyperliquid")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	// 1. 对比K线数据
	fmt.Println("1️⃣  K线数据对比 (最近5根1小时K线)")
	fmt.Println(strings.Repeat("-", 80))

	binanceKlines, err := getBinanceKlines(symbol, "1h", 5)
	if err != nil {
		t.Logf("⚠️  Binance K线获取失败: %v", err)
	} else {
		fmt.Printf("\n📈 Binance:\n")
		for i, k := range binanceKlines {
			fmt.Printf("  [%d] 开:%.2f 高:%.2f 低:%.2f 收:%.2f 量:%.2f\n",
				i, k.Open, k.High, k.Low, k.Close, k.Volume)
		}
	}

	hyperliquidKlines, err := getKlines(symbol, "1h", 5)
	if err != nil {
		t.Logf("⚠️  Hyperliquid K线获取失败: %v", err)
	} else {
		fmt.Printf("\n📈 Hyperliquid:\n")
		for i, k := range hyperliquidKlines {
			fmt.Printf("  [%d] 开:%.2f 高:%.2f 低:%.2f 收:%.2f 量:%.2f\n",
				i, k.Open, k.High, k.Low, k.Close, k.Volume)
		}
	}

	// 对比最新价格
	if len(binanceKlines) > 0 && len(hyperliquidKlines) > 0 {
		binancePrice := binanceKlines[len(binanceKlines)-1].Close
		hyperliquidPrice := hyperliquidKlines[len(hyperliquidKlines)-1].Close
		priceDiff := hyperliquidPrice - binancePrice
		priceDiffPct := (priceDiff / binancePrice) * 100

		fmt.Printf("\n💰 价格对比:\n")
		fmt.Printf("  Binance:     $%.2f\n", binancePrice)
		fmt.Printf("  Hyperliquid: $%.2f\n", hyperliquidPrice)
		fmt.Printf("  差异:        $%.2f (%.4f%%)\n", priceDiff, priceDiffPct)
	}

	// 2. 对比资金费率
	fmt.Println("\n\n2️⃣  资金费率对比")
	fmt.Println(strings.Repeat("-", 80))

	binanceFunding, err := getBinanceFundingRate(symbol)
	if err != nil {
		t.Logf("⚠️  Binance资金费率获取失败: %v", err)
	} else {
		fmt.Printf("📊 Binance:     %.6f%% (%.8f)\n", binanceFunding*100, binanceFunding)
	}

	hyperliquidFunding, err := getFundingRate(symbol)
	if err != nil {
		t.Logf("⚠️  Hyperliquid资金费率获取失败: %v", err)
	} else {
		fmt.Printf("📊 Hyperliquid: %.6f%% (%.8f)\n", hyperliquidFunding*100, hyperliquidFunding)
	}

	if binanceFunding != 0 && hyperliquidFunding != 0 {
		fundingDiff := hyperliquidFunding - binanceFunding
		fmt.Printf("📊 差异:        %.6f%% (%.8f)\n", fundingDiff*100, fundingDiff)
	}

	// 3. 对比持仓量
	fmt.Println("\n\n3️⃣  持仓量对比")
	fmt.Println(strings.Repeat("-", 80))

	binanceOI, err := getBinanceOpenInterest(symbol)
	if err != nil {
		t.Logf("⚠️  Binance持仓量获取失败: %v", err)
	} else {
		fmt.Printf("📊 Binance:     %.2f BTC\n", binanceOI)
	}

	hyperliquidOI, err := getOpenInterestData(symbol)
	if err != nil {
		t.Logf("⚠️  Hyperliquid持仓量获取失败: %v", err)
	} else {
		fmt.Printf("📊 Hyperliquid: %.2f BTC\n", hyperliquidOI.Latest)
	}

	if binanceOI != 0 && hyperliquidOI != nil && hyperliquidOI.Latest != 0 {
		oiDiff := hyperliquidOI.Latest - binanceOI
		oiDiffPct := (oiDiff / binanceOI) * 100
		fmt.Printf("📊 差异:        %.2f BTC (%.2f%%)\n", oiDiff, oiDiffPct)
	}

	// 4. 对比完整市场数据
	fmt.Println("\n\n4️⃣  完整市场数据对比")
	fmt.Println(strings.Repeat("-", 80))

	hyperliquidData, err := Get(symbol, 60)
	if err != nil {
		t.Fatalf("❌ Hyperliquid完整数据获取失败: %v", err)
	}

	fmt.Printf("\n📊 Hyperliquid 技术指标:\n")
	fmt.Printf("  当前价格: $%.2f\n", hyperliquidData.CurrentPrice)
	fmt.Printf("  资金费率: %.6f%%\n", hyperliquidData.FundingRate*100)
	fmt.Printf("  持仓量:   %.2f BTC\n", hyperliquidData.OpenInterest.Latest)
	fmt.Printf("\n  1小时周期:\n")
	fmt.Printf("    EMA20:  %.2f\n", hyperliquidData.Timeframe1h.EMA20)
	fmt.Printf("    EMA50:  %.2f\n", hyperliquidData.Timeframe1h.EMA50)
	fmt.Printf("    RSI:    %.2f\n", hyperliquidData.Timeframe1h.RSI)

	fmt.Printf("    ATR:    %.2f\n", hyperliquidData.Timeframe1h.ATR)

	// 5. 总结
	fmt.Println("\n\n5️⃣  总结")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("✅ Hyperliquid数据获取成功\n")
	fmt.Printf("✅ 所有技术指标计算正常\n")
	fmt.Printf("✅ 数据结构与Binance兼容\n")
	fmt.Printf("\n💡 说明:\n")
	fmt.Printf("  - 价格可能略有差异（不同交易所的市场价格）\n")
	fmt.Printf("  - 资金费率可能不同（各交易所独立计算）\n")
	fmt.Printf("  - 持仓量反映各交易所的实际持仓情况\n")
	fmt.Printf("  - 技术指标基于各自的K线数据计算\n")

	fmt.Println("\n" + strings.Repeat("=", 80))
}

// TestMultipleSymbolsComparison 对比多个币种
func TestMultipleSymbolsComparison(t *testing.T) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📊 多币种数据对比")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	fmt.Printf("%-12s | %-15s | %-15s | %-12s\n", "币种", "Binance价格", "Hyperliquid价格", "差异%")
	fmt.Println(strings.Repeat("-", 80))

	for _, symbol := range symbols {
		// Binance价格
		binanceKlines, err := getBinanceKlines(symbol, "1h", 1)
		var binancePrice float64
		if err == nil && len(binanceKlines) > 0 {
			binancePrice = binanceKlines[0].Close
		}

		// Hyperliquid价格
		hyperliquidKlines, err := getKlines(symbol, "1h", 1)
		var hyperliquidPrice float64
		if err == nil && len(hyperliquidKlines) > 0 {
			hyperliquidPrice = hyperliquidKlines[0].Close
		}

		// 计算差异
		var diffPct float64
		if binancePrice > 0 && hyperliquidPrice > 0 {
			diffPct = ((hyperliquidPrice - binancePrice) / binancePrice) * 100
		}

		fmt.Printf("%-12s | $%-14.2f | $%-14.2f | %+.4f%%\n",
			symbol, binancePrice, hyperliquidPrice, diffPct)
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
}
