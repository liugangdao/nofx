package market

import (
	"testing"
)

func TestCalculateSupertrend(t *testing.T) {
	// 创建测试K线数据（模拟上升趋势）
	klines := []Kline{
		{Open: 100, High: 105, Low: 99, Close: 104, Volume: 1000},
		{Open: 104, High: 108, Low: 103, Close: 107, Volume: 1100},
		{Open: 107, High: 112, Low: 106, Close: 111, Volume: 1200},
		{Open: 111, High: 115, Low: 110, Close: 114, Volume: 1300},
		{Open: 114, High: 118, Low: 113, Close: 117, Volume: 1400},
		{Open: 117, High: 122, Low: 116, Close: 121, Volume: 1500},
		{Open: 121, High: 125, Low: 120, Close: 124, Volume: 1600},
		{Open: 124, High: 128, Low: 123, Close: 127, Volume: 1700},
		{Open: 127, High: 132, Low: 126, Close: 131, Volume: 1800},
		{Open: 131, High: 135, Low: 130, Close: 134, Volume: 1900},
		{Open: 134, High: 138, Low: 133, Close: 137, Volume: 2000},
	}

	st := calculateSupertrend(klines, 10, 3.0)

	if st == nil {
		t.Fatal("calculateSupertrend returned nil")
	}

	// 验证基本字段
	if st.Trend == "" {
		t.Error("Trend should not be empty")
	}

	if st.Value == 0 {
		t.Error("Value should not be zero")
	}

	if st.SupportLevel == 0 {
		t.Error("SupportLevel should not be zero")
	}

	if st.ResistanceLevel == 0 {
		t.Error("ResistanceLevel should not be zero")
	}

	if st.ATRMultiplier != 3.0 {
		t.Errorf("Expected ATRMultiplier 3.0, got %.2f", st.ATRMultiplier)
	}

	// 在上升趋势中，应该识别为UPTREND
	if st.Trend != "UPTREND" {
		t.Errorf("Expected UPTREND in rising market, got %s", st.Trend)
	}

	// 支撑位应该低于当前价格
	currentPrice := klines[len(klines)-1].Close
	if st.SupportLevel >= currentPrice {
		t.Errorf("Support level (%.2f) should be below current price (%.2f)", st.SupportLevel, currentPrice)
	}

	t.Logf("Supertrend Result:")
	t.Logf("  Trend: %s", st.Trend)
	t.Logf("  Value: %.2f", st.Value)
	t.Logf("  Support: %.2f", st.SupportLevel)
	t.Logf("  Resistance: %.2f", st.ResistanceLevel)
	t.Logf("  Current Price: %.2f", currentPrice)
}

func TestCalculateSupertrendDowntrend(t *testing.T) {
	// 创建测试K线数据（模拟下降趋势）
	klines := []Kline{
		{Open: 140, High: 142, Low: 135, Close: 136, Volume: 1000},
		{Open: 136, High: 138, Low: 131, Close: 132, Volume: 1100},
		{Open: 132, High: 134, Low: 127, Close: 128, Volume: 1200},
		{Open: 128, High: 130, Low: 123, Close: 124, Volume: 1300},
		{Open: 124, High: 126, Low: 119, Close: 120, Volume: 1400},
		{Open: 120, High: 122, Low: 115, Close: 116, Volume: 1500},
		{Open: 116, High: 118, Low: 111, Close: 112, Volume: 1600},
		{Open: 112, High: 114, Low: 107, Close: 108, Volume: 1700},
		{Open: 108, High: 110, Low: 103, Close: 104, Volume: 1800},
		{Open: 104, High: 106, Low: 99, Close: 100, Volume: 1900},
		{Open: 100, High: 102, Low: 95, Close: 96, Volume: 2000},
	}

	st := calculateSupertrend(klines, 10, 3.0)

	if st == nil {
		t.Fatal("calculateSupertrend returned nil")
	}

	// 在下降趋势中，应该识别为DOWNTREND
	if st.Trend != "DOWNTREND" {
		t.Errorf("Expected DOWNTREND in falling market, got %s", st.Trend)
	}

	// 阻力位应该高于当前价格
	currentPrice := klines[len(klines)-1].Close
	if st.ResistanceLevel <= currentPrice {
		t.Errorf("Resistance level (%.2f) should be above current price (%.2f)", st.ResistanceLevel, currentPrice)
	}

	t.Logf("Supertrend Result (Downtrend):")
	t.Logf("  Trend: %s", st.Trend)
	t.Logf("  Value: %.2f", st.Value)
	t.Logf("  Support: %.2f", st.SupportLevel)
	t.Logf("  Resistance: %.2f", st.ResistanceLevel)
	t.Logf("  Current Price: %.2f", currentPrice)
}

func TestCalculateSupertrendInsufficientData(t *testing.T) {
	// 测试数据不足的情况
	klines := []Kline{
		{Open: 100, High: 105, Low: 99, Close: 104, Volume: 1000},
		{Open: 104, High: 108, Low: 103, Close: 107, Volume: 1100},
	}

	st := calculateSupertrend(klines, 10, 3.0)

	if st == nil {
		t.Fatal("calculateSupertrend returned nil")
	}

	if st.Trend != "UNKNOWN" {
		t.Errorf("Expected UNKNOWN trend with insufficient data, got %s", st.Trend)
	}

	if st.Description != "数据不足" {
		t.Errorf("Expected '数据不足' description, got %s", st.Description)
	}
}
