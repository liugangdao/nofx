package market

import (
	"fmt"
	"testing"
)

// TestGetMarketData 测试获取市场数据
func TestGetMarketData(t *testing.T) {
	// 测试BTC数据
	symbol := "BTCUSDT"
	interval := 60 // 60分钟扫描间隔

	fmt.Printf("\n🔍 测试获取 %s 市场数据...\n", symbol)

	data, err := Get(symbol, interval)
	if err != nil {
		t.Fatalf("❌ 获取市场数据失败: %v", err)
	}

	// 验证基本数据
	if data.Symbol != symbol {
		t.Errorf("❌ Symbol不匹配: 期望 %s, 实际 %s", symbol, data.Symbol)
	}

	if data.CurrentPrice <= 0 {
		t.Errorf("❌ 当前价格无效: %.2f", data.CurrentPrice)
	} else {
		fmt.Printf("✅ 当前价格: $%.2f\n", data.CurrentPrice)
	}

	// 验证资金费率
	fmt.Printf("✅ 资金费率: %.6f%%\n", data.FundingRate*100)

	// 验证OI数据
	if data.OpenInterest != nil {
		fmt.Printf("✅ 持仓量: %.2f (平均: %.2f)\n", data.OpenInterest.Latest, data.OpenInterest.Average)
	}

	// 验证12小时数据
	if data.Timeframe12h == nil {
		t.Error("❌ 12小时数据为空")
	} else {
		fmt.Printf("\n📊 12小时周期数据:\n")
		fmt.Printf("  EMA20: %.2f\n", data.Timeframe12h.EMA20)
		fmt.Printf("  EMA50: %.2f\n", data.Timeframe12h.EMA50)
		fmt.Printf("  EMA200: %.2f\n", data.Timeframe12h.EMA200)
		fmt.Printf("  RSI: %.2f\n", data.Timeframe12h.RSI)
		fmt.Printf("  市场结构: %s\n", data.Timeframe12h.MarketStructure)
		fmt.Printf("  POC: %.2f\n", data.Timeframe12h.POC)
	}

	// 验证4小时数据
	if data.Timeframe4h == nil {
		t.Error("❌ 4小时数据为空")
	} else {
		fmt.Printf("\n📊 4小时周期数据:\n")
		fmt.Printf("  EMA20: %.2f\n", data.Timeframe4h.EMA20)
		fmt.Printf("  EMA50: %.2f\n", data.Timeframe4h.EMA50)
		fmt.Printf("  RSI: %.2f\n", data.Timeframe4h.RSI)

		fmt.Printf("  ATR: %.2f\n", data.Timeframe4h.ATR)
	}

	// 验证1小时数据
	if data.Timeframe1h == nil {
		t.Error("❌ 1小时数据为空")
	} else {
		fmt.Printf("\n📊 1小时周期数据:\n")
		fmt.Printf("  EMA20: %.2f\n", data.Timeframe1h.EMA20)
		fmt.Printf("  RSI: %.2f\n", data.Timeframe1h.RSI)

		fmt.Printf("  ATR: %.2f\n", data.Timeframe1h.ATR)
		fmt.Printf("  价格序列长度: %d\n", len(data.Timeframe1h.PriceSeries))
	}

	fmt.Printf("\n✅ 所有测试通过!\n")
}

// TestGetKlines 测试获取K线数据
func TestGetKlines(t *testing.T) {
	symbol := "BTCUSDT"
	interval := "1h"
	limit := 100

	fmt.Printf("\n🔍 测试获取 %s K线数据 (周期: %s, 数量: %d)...\n", symbol, interval, limit)

	klines, err := getKlines(symbol, interval, limit)
	if err != nil {
		t.Fatalf("❌ 获取K线失败: %v", err)
	}

	if len(klines) == 0 {
		t.Fatal("❌ K线数据为空")
	}

	fmt.Printf("✅ 获取到 %d 根K线\n", len(klines))

	// 显示最近5根K线
	fmt.Printf("\n📊 最近5根K线:\n")
	start := len(klines) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		k := klines[i]
		fmt.Printf("  [%d] O:%.2f H:%.2f L:%.2f C:%.2f V:%.2f\n",
			i, k.Open, k.High, k.Low, k.Close, k.Volume)
	}

	// 验证K线数据完整性
	lastKline := klines[len(klines)-1]
	if lastKline.Close <= 0 {
		t.Errorf("❌ 最新K线收盘价无效: %.2f", lastKline.Close)
	}
	if lastKline.High < lastKline.Low {
		t.Errorf("❌ K线数据异常: 最高价(%.2f) < 最低价(%.2f)", lastKline.High, lastKline.Low)
	}

	fmt.Printf("\n✅ K线数据验证通过!\n")
}

// TestGetOpenInterest 测试获取持仓量
func TestGetOpenInterest(t *testing.T) {
	symbol := "BTCUSDT"

	fmt.Printf("\n🔍 测试获取 %s 持仓量...\n", symbol)

	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		t.Fatalf("❌ 获取持仓量失败: %v", err)
	}

	if oiData == nil {
		t.Fatal("❌ 持仓量数据为空")
	}

	fmt.Printf("✅ 最新持仓量: %.2f\n", oiData.Latest)
	fmt.Printf("✅ 平均持仓量: %.2f\n", oiData.Average)
}

// TestGetFundingRate 测试获取资金费率
func TestGetFundingRate(t *testing.T) {
	symbol := "BTCUSDT"

	fmt.Printf("\n🔍 测试获取 %s 资金费率...\n", symbol)

	rate, err := getFundingRate(symbol)
	if err != nil {
		t.Fatalf("❌ 获取资金费率失败: %v", err)
	}

	fmt.Printf("✅ 资金费率: %.6f%% (%.8f)\n", rate*100, rate)
}

// TestMultipleSymbols 测试多个币种
func TestMultipleSymbols(t *testing.T) {
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}

	fmt.Printf("\n🔍 测试多个币种数据获取...\n")

	for _, symbol := range symbols {
		fmt.Printf("\n--- %s ---\n", symbol)

		data, err := Get(symbol, 60)
		if err != nil {
			t.Errorf("❌ %s 获取失败: %v", symbol, err)
			continue
		}

		fmt.Printf("✅ 价格: $%.2f\n", data.CurrentPrice)
		fmt.Printf("✅ RSI(1h): %.2f\n", data.Timeframe1h.RSI)
		fmt.Printf("✅ 资金费率: %.6f%%\n", data.FundingRate*100)
	}
}
