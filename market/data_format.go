package market

import (
	"fmt"
	"strings"
)

// Format 格式化输出市场数据
func Format(data *Data) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Current Price: %.2f\n\n", data.CurrentPrice))

	if data.OpenInterest != nil {
		sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f, Average: %.2f\n\n",
			data.OpenInterest.Latest, data.OpenInterest.Average))
	}

	sb.WriteString(fmt.Sprintf("Funding Rate: %.6f\n\n", data.FundingRate))

	// 12小时周期
	if data.Timeframe12h != nil {
		sb.WriteString("=== 12-Hour Timeframe ===\n\n")
		formatTimeframeData(&sb, data.Timeframe12h, data.CurrentPrice)
	}

	// 4小时周期
	if data.Timeframe4h != nil {
		sb.WriteString("=== 4-Hour Timeframe ===\n\n")
		formatTimeframeData(&sb, data.Timeframe4h, data.CurrentPrice)
	}

	// 1小时周期
	if data.Timeframe1h != nil {
		sb.WriteString("=== 1-Hour Timeframe ===\n\n")
		formatTimeframeData(&sb, data.Timeframe1h, data.CurrentPrice)
	}

	return sb.String()
}

// formatTimeframeData 格式化单个时间周期数据
func formatTimeframeData(sb *strings.Builder, tf *TimeframeData, currentPrice float64) {
	sb.WriteString(fmt.Sprintf("Price: %.2f\n", tf.Price))
	sb.WriteString(fmt.Sprintf("EMA20: %.2f, EMA50: %.2f, EMA200: %.2f\n", tf.EMA20, tf.EMA50, tf.EMA200))
	sb.WriteString(fmt.Sprintf("RSI: %.2f\n", tf.RSI))
	sb.WriteString(fmt.Sprintf("Market Structure: %s\n", tf.MarketStructure))
	sb.WriteString(fmt.Sprintf("POC: %.2f\n", tf.POC))

	// 显示波动率和趋势指标
	if tf.ATR > 0 {
		sb.WriteString(fmt.Sprintf("ATR: %.2f, ADX: %.2f, BB Width: %.4f, RVOL: %.2f\n", tf.ATR, tf.ADX, tf.BBWidth, tf.RVOL))
	} else {
		sb.WriteString(fmt.Sprintf("ADX: %.2f, BB Width: %.4f, RVOL: %.2f\n", tf.ADX, tf.BBWidth, tf.RVOL))
	}

	// 显示RSI背离信号
	if tf.RSIDivergence != nil {
		sb.WriteString(fmt.Sprintf("RSI Divergence: [%s-%s] %s\n", tf.RSIDivergence.Type, tf.RSIDivergence.Strength, tf.RSIDivergence.Description))
	}

	// 显示时间序列数据 (最近10个数据点，从旧到新)
	if len(tf.PriceSeries) > 0 {
		sb.WriteString(fmt.Sprintf("Price Series (oldest→latest): %s\n", formatFloatSlice(tf.PriceSeries)))
	}
	if len(tf.EMA20Series) > 0 {
		sb.WriteString(fmt.Sprintf("EMA20 Series: %s\n", formatFloatSlice(tf.EMA20Series)))
	}
	if len(tf.RSISeries) > 0 {
		sb.WriteString(fmt.Sprintf("RSI Series: %s\n", formatFloatSlice(tf.RSISeries)))
	}

	// 显示市场结构详情
	if tf.StructureDetail != nil {
		ms := tf.StructureDetail
		sb.WriteString(fmt.Sprintf("Trend: %s | %s\n", ms.Trend, ms.Description))

		// 显示最近的摆动点
		if len(ms.SwingHighs) > 0 {
			recentHighs := ms.SwingHighs
			if len(recentHighs) > 3 {
				recentHighs = recentHighs[len(recentHighs)-3:]
			}
			sb.WriteString("Recent Swing Highs: ")
			for i, sh := range recentHighs {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%.2f", sh.Price))
			}
			sb.WriteString("\n")
		}

		if len(ms.SwingLows) > 0 {
			recentLows := ms.SwingLows
			if len(recentLows) > 3 {
				recentLows = recentLows[len(recentLows)-3:]
			}
			sb.WriteString("Recent Swing Lows: ")
			for i, sl := range recentLows {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(fmt.Sprintf("%.2f", sl.Price))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
}

// formatFloatSlice 格式化float64切片为字符串
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.2f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}
