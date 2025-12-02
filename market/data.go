package market

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Data 市场数据结构
type Data struct {
	Symbol              string
	CurrentPrice        float64
	OpenInterest        *OIData
	FundingRate         float64
	Timeframe12h        *TimeframeData // 12小时周期数据
	Timeframe4h         *TimeframeData // 4小时周期数据
	Timeframe1h         *TimeframeData // 1小时周期数据
	ScanIntervalMinutes int            // 扫描间隔分钟数
}

// OIData Open Interest数据
type OIData struct {
	Latest  float64
	Average float64
}

// TimeframeData 时间周期数据
type TimeframeData struct {
	Timeframe       string             // "12h", "4h", "1h"
	Price           float64            // 当前价格
	EMA20           float64            // 20周期EMA
	EMA50           float64            // 50周期EMA
	EMA200          float64            // 200周期EMA
	RSI             float64            // RSI指标
	MarketStructure string             // 市场结构: "HH", "HL", "LH", "LL", "RANGING"
	POC             float64            // Point of Control
	ATR             float64            // ATR指标
	ADX             float64            // 平均趋向指数
	BBWidth         float64            // 布林带带宽
	RVOL            float64            // 相对成交量
	VolumeProfile   *VolumeProfileData // 成交量分布
	StructureDetail *MarketStructure   // 详细市场结构
	RSIDivergence   *RSIDivergence     // RSI背离信号
	CandleReversal  *CandleReversal    // K线反转信号

	// 时间序列数据 (最近10个数据点，从旧到新)
	PriceSeries []float64 // 价格序列
	EMA20Series []float64 // EMA20序列
	RSISeries   []float64 // RSI序列
}

// VolumeProfileData 成交量分布数据
type VolumeProfileData struct {
	POC           float64             // Point of Control - 成交量最大的价格
	ValueAreaHigh float64             // 价值区域上限 (70%成交量)
	ValueAreaLow  float64             // 价值区域下限 (70%成交量)
	PriceLevels   []float64           // 价格档位
	VolumeAtPrice map[float64]float64 // 每个价格档位的成交量
}

// SwingPoint 摆动点
type SwingPoint struct {
	Index  int     // K线索引
	Price  float64 // 价格
	IsHigh bool    // true=摆动高点, false=摆动低点
}

// MarketStructure 市场结构
type MarketStructure struct {
	Trend       string       // "UPTREND" (上升趋势), "DOWNTREND" (下降趋势), "RANGING" (震荡)
	SwingHighs  []SwingPoint // 摆动高点列表
	SwingLows   []SwingPoint // 摆动低点列表
	LastPattern string       // 最近的结构模式: "HH", "HL", "LH", "LL"
	Description string       // 市场结构描述
}

// RSIDivergence RSI背离信号
type RSIDivergence struct {
	Type          string  // "BULLISH" (看涨背离), "BEARISH" (看跌背离), "NONE" (无背离)
	Strength      string  // "REGULAR" (常规背离), "HIDDEN" (隐藏背离)
	Description   string  // 背离描述
	PeriodsAgo    int     // 背离出现在第几个周期前 (0=当前周期)
	ValidityLeft  int     // 剩余有效周期数
	PricePoint1   float64 // 价格点1
	PricePoint2   float64 // 价格点2
	RSIPoint1     float64 // RSI点1
	RSIPoint2     float64 // RSI点2
	DetectedIndex int     // 检测到背离的K线索引
}

// CandleReversal K线反转信号
type CandleReversal struct {
	SingleCandle *SingleCandlePattern // 单K线反转形态
	DoubleCandle *DoubleCandlePattern // 双K线反转形态
}

// SingleCandlePattern 单K线反转形态
type SingleCandlePattern struct {
	Type        string  // "BULLISH_HAMMER" (看涨锤子线), "BEARISH_SHOOTING_STAR" (看跌流星线), "BULLISH_ENGULFING" (看涨吞没), "BEARISH_ENGULFING" (看跌吞没), "NONE"
	Description string  // 形态描述
	Strength    float64 // 信号强度 (0-1)
}

// DoubleCandlePattern 双K线反转形态
type DoubleCandlePattern struct {
	Type        string  // "BULLISH_ENGULFING" (看涨吞没), "BEARISH_ENGULFING" (看跌吞没), "BULLISH_PIERCING" (看涨刺透), "BEARISH_DARK_CLOUD" (看跌乌云盖顶), "NONE"
	Description string  // 形态描述
	Strength    float64 // 信号强度 (0-1)
}

// Kline K线数据
type Kline struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// Get 获取指定代币的市场数据
func Get(symbol string, interval int) (*Data, error) {
	// 标准化symbol
	symbol = Normalize(symbol)

	// 获取12小时K线数据
	klines12h, err := getKlines(symbol, "12h", 250) // 需要足够数据计算EMA200
	if err != nil {
		return nil, fmt.Errorf("获取12小时K线失败: %v", err)
	}

	// 获取4小时K线数据
	klines4h, err := getKlines(symbol, "4h", 250)
	if err != nil {
		return nil, fmt.Errorf("获取4小时K线失败: %v", err)
	}

	// 获取1小时K线数据
	klines1h, err := getKlines(symbol, "1h", 250)
	if err != nil {
		return nil, fmt.Errorf("获取1小时K线失败: %v", err)
	}

	// 当前价格
	currentPrice := klines1h[len(klines1h)-1].Close

	// 获取OI数据
	oiData, err := getOpenInterestData(symbol)
	if err != nil {
		// OI失败不影响整体,使用默认值
		oiData = &OIData{Latest: 0, Average: 0}
	}

	// 获取Funding Rate
	fundingRate, _ := getFundingRate(symbol)

	// 计算各时间周期数据
	timeframe12h := calculateTimeframeData(klines12h, "12h", currentPrice, false)
	timeframe4h := calculateTimeframeData(klines4h, "4h", currentPrice, true)
	timeframe1h := calculateTimeframeData(klines1h, "1h", currentPrice, true)

	return &Data{
		Symbol:              symbol,
		CurrentPrice:        currentPrice,
		OpenInterest:        oiData,
		FundingRate:         fundingRate,
		Timeframe12h:        timeframe12h,
		Timeframe4h:         timeframe4h,
		Timeframe1h:         timeframe1h,
		ScanIntervalMinutes: interval,
	}, nil
}

// getKlines 从Hyperliquid获取K线数据
func getKlines(symbol, interval string, limit int) ([]Kline, error) {
	// 转换symbol格式: BTCUSDT -> BTC
	coin := convertSymbolToHyperliquid(symbol)

	// 转换时间间隔格式: 1h -> 1h, 4h -> 4h, 12h -> 12h
	// Hyperliquid支持: 1m, 15m, 1h, 4h, 1d
	hlInterval := interval

	// 计算开始时间 (毫秒)
	endTime := time.Now().Unix() * 1000
	var intervalMs int64
	switch interval {
	case "1h":
		intervalMs = 3600 * 1000
	case "4h":
		intervalMs = 4 * 3600 * 1000
	case "12h":
		intervalMs = 12 * 3600 * 1000
	default:
		intervalMs = 3600 * 1000
	}
	startTime := endTime - int64(limit)*intervalMs

	// 构建请求
	url := "https://api.hyperliquid.xyz/info"
	requestBody := map[string]any{
		"type": "candleSnapshot",
		"req": map[string]any{
			"coin":      coin,
			"interval":  hlInterval,
			"startTime": startTime,
			"endTime":   endTime,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// Hyperliquid返回格式: [{"t": timestamp_ms, "T": close_timestamp_ms, "o": "open", "h": "high", "l": "low", "c": "close", "v": "volume", "n": num_trades}]
	var rawData []map[string]any
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	klines := make([]Kline, 0, len(rawData))
	for _, item := range rawData {
		openTime := int64(item["t"].(float64))
		closeTime := int64(item["T"].(float64))
		open, _ := parseFloat(item["o"])
		high, _ := parseFloat(item["h"])
		low, _ := parseFloat(item["l"])
		close, _ := parseFloat(item["c"])
		volume, _ := parseFloat(item["v"])

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	return klines, nil
}

// convertSymbolToHyperliquid 将标准symbol转换为Hyperliquid格式
// 例如: "BTCUSDT" -> "BTC"
func convertSymbolToHyperliquid(symbol string) string {
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}

// calculateEMA 计算EMA
func calculateEMA(klines []Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA作为初始EMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// 计算EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

// calculateRSI 计算RSI
func calculateRSI(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	gains := 0.0
	losses := 0.0

	// 计算初始平均涨跌幅
	for i := 1; i <= period; i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// 使用Wilder平滑方法计算后续RSI
	for i := period + 1; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			avgGain = (avgGain*float64(period-1) + change) / float64(period)
			avgLoss = (avgLoss * float64(period-1)) / float64(period)
		} else {
			avgGain = (avgGain * float64(period-1)) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + (-change)) / float64(period)
		}
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateVolumeProfile 计算成交量分布
func calculateVolumeProfile(klines []Kline, period int) *VolumeProfileData {
	if len(klines) < period {
		period = len(klines)
	}

	// 使用最近period根K线
	startIdx := len(klines) - period
	relevantKlines := klines[startIdx:]

	// 找出价格范围
	minPrice := relevantKlines[0].Low
	maxPrice := relevantKlines[0].High
	for _, k := range relevantKlines {
		if k.Low < minPrice {
			minPrice = k.Low
		}
		if k.High > maxPrice {
			maxPrice = k.High
		}
	}

	// 将价格范围分成100个档位
	numBins := 100
	priceStep := (maxPrice - minPrice) / float64(numBins)
	if priceStep == 0 {
		priceStep = 1
	}

	// 初始化成交量分布
	volumeAtPrice := make(map[float64]float64)
	priceLevels := make([]float64, 0, numBins)

	// 计算每个价格档位的成交量
	for i := 0; i < numBins; i++ {
		priceLevel := minPrice + float64(i)*priceStep
		priceLevels = append(priceLevels, priceLevel)
		volumeAtPrice[priceLevel] = 0
	}

	// 分配成交量到价格档位
	for _, k := range relevantKlines {
		// 将K线的成交量按价格范围分配
		klineRange := k.High - k.Low
		if klineRange == 0 {
			// 如果没有价格变化,分配到收盘价所在档位
			binIdx := int((k.Close - minPrice) / priceStep)
			if binIdx >= numBins {
				binIdx = numBins - 1
			}
			if binIdx < 0 {
				binIdx = 0
			}
			priceLevel := priceLevels[binIdx]
			volumeAtPrice[priceLevel] += k.Volume
		} else {
			// 按价格范围比例分配成交量
			for i, priceLevel := range priceLevels {
				nextLevel := maxPrice
				if i < len(priceLevels)-1 {
					nextLevel = priceLevels[i+1]
				}

				// 计算这个档位与K线的重叠部分
				overlapLow := math.Max(priceLevel, k.Low)
				overlapHigh := math.Min(nextLevel, k.High)

				if overlapHigh > overlapLow {
					overlapRatio := (overlapHigh - overlapLow) / klineRange
					volumeAtPrice[priceLevel] += k.Volume * overlapRatio
				}
			}
		}
	}

	// 找出POC (成交量最大的价格)
	poc := priceLevels[0]
	maxVolume := volumeAtPrice[poc]
	for _, priceLevel := range priceLevels {
		if volumeAtPrice[priceLevel] > maxVolume {
			maxVolume = volumeAtPrice[priceLevel]
			poc = priceLevel
		}
	}

	// 计算总成交量
	totalVolume := 0.0
	for _, vol := range volumeAtPrice {
		totalVolume += vol
	}

	// 计算价值区域 (包含70%成交量的价格范围)
	targetVolume := totalVolume * 0.70
	valueAreaHigh := poc
	valueAreaLow := poc
	accumulatedVolume := volumeAtPrice[poc]

	// 从POC向上下扩展,直到包含70%成交量
	for accumulatedVolume < targetVolume {
		// 找到POC上方和下方最近的价格档位
		var upperPrice, lowerPrice float64
		upperVolume, lowerVolume := 0.0, 0.0

		for _, priceLevel := range priceLevels {
			if priceLevel > valueAreaHigh {
				if upperPrice == 0 || priceLevel < upperPrice {
					upperPrice = priceLevel
					upperVolume = volumeAtPrice[priceLevel]
				}
			}
			if priceLevel < valueAreaLow {
				if lowerPrice == 0 || priceLevel > lowerPrice {
					lowerPrice = priceLevel
					lowerVolume = volumeAtPrice[priceLevel]
				}
			}
		}

		// 选择成交量更大的方向扩展
		if upperVolume >= lowerVolume && upperPrice > 0 {
			valueAreaHigh = upperPrice
			accumulatedVolume += upperVolume
		} else if lowerPrice > 0 {
			valueAreaLow = lowerPrice
			accumulatedVolume += lowerVolume
		} else {
			break
		}
	}

	return &VolumeProfileData{
		POC:           poc,
		ValueAreaHigh: valueAreaHigh,
		ValueAreaLow:  valueAreaLow,
		PriceLevels:   priceLevels,
		VolumeAtPrice: volumeAtPrice,
	}
}

// calculateMarketStructure 计算市场结构
func calculateMarketStructure(klines []Kline, lookback int) *MarketStructure {
	if len(klines) < lookback*2+1 {
		return &MarketStructure{
			Trend:       "UNKNOWN",
			Description: "数据不足以分析市场结构",
		}
	}

	// 识别摆动高点和低点
	// 摆动高点: 当前K线的高点高于左右各lookback根K线的高点
	// 摆动低点: 当前K线的低点低于左右各lookback根K线的低点
	swingHighs := []SwingPoint{}
	swingLows := []SwingPoint{}

	for i := lookback; i < len(klines)-lookback; i++ {
		isSwingHigh := true
		isSwingLow := true

		// 检查左右各lookback根K线
		for j := 1; j <= lookback; j++ {
			// 检查摆动高点
			if klines[i].High <= klines[i-j].High || klines[i].High <= klines[i+j].High {
				isSwingHigh = false
			}
			// 检查摆动低点
			if klines[i].Low >= klines[i-j].Low || klines[i].Low >= klines[i+j].Low {
				isSwingLow = false
			}
		}

		if isSwingHigh {
			swingHighs = append(swingHighs, SwingPoint{
				Index:  i,
				Price:  klines[i].High,
				IsHigh: true,
			})
		}
		if isSwingLow {
			swingLows = append(swingLows, SwingPoint{
				Index:  i,
				Price:  klines[i].Low,
				IsHigh: false,
			})
		}
	}

	// 分析市场结构模式
	trend := "RANGING"
	lastPattern := ""
	description := ""

	// 需要至少2个摆动高点和2个摆动低点来判断趋势
	if len(swingHighs) >= 2 && len(swingLows) >= 2 {
		// 获取最近的摆动点
		lastHigh := swingHighs[len(swingHighs)-1]
		prevHigh := swingHighs[len(swingHighs)-2]
		lastLow := swingLows[len(swingLows)-1]
		prevLow := swingLows[len(swingLows)-2]

		// 判断高点模式
		highPattern := ""
		if lastHigh.Price > prevHigh.Price {
			highPattern = "HH" // Higher High
		} else {
			highPattern = "LH" // Lower High
		}

		// 判断低点模式
		lowPattern := ""
		if lastLow.Price > prevLow.Price {
			lowPattern = "HL" // Higher Low
		} else {
			lowPattern = "LL" // Lower Low
		}

		// 判断趋势
		if highPattern == "HH" && lowPattern == "HL" {
			trend = "UPTREND"
			lastPattern = "HH+HL"
			description = "形成更高的高点(HH)和更高的低点(HL)"
		} else if highPattern == "LH" && lowPattern == "LL" {
			trend = "DOWNTREND"
			lastPattern = "LH+LL"
			description = " 形成更低的高点(LH)和更低的低点(LL)"
		} else if highPattern == "HH" && lowPattern == "LL" {
			trend = "RANGING"
			lastPattern = "HH+LL"
			description = "高点上升但低点下降,可能处于盘整或趋势转换"
		} else if highPattern == "LH" && lowPattern == "HL" {
			trend = "RANGING"
			lastPattern = "LH+HL"
			description = "高点下降但低点上升,价格收敛可能突破"
		}

		// 添加具体价格信息
		description += fmt.Sprintf(" | 最近高点: %.2f (前高: %.2f), 最近低点: %.2f (前低: %.2f)",
			lastHigh.Price, prevHigh.Price, lastLow.Price, prevLow.Price)
	} else {
		description = fmt.Sprintf("摆动点不足: 高点%d个, 低点%d个", len(swingHighs), len(swingLows))
	}

	return &MarketStructure{
		Trend:       trend,
		SwingHighs:  swingHighs,
		SwingLows:   swingLows,
		LastPattern: lastPattern,
		Description: description,
	}
}

// calculateATR 计算ATR
func calculateATR(klines []Kline, period int) float64 {
	if len(klines) <= period {
		return 0
	}

	trs := make([]float64, len(klines))
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trs[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算初始ATR
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += trs[i]
	}
	atr := sum / float64(period)

	// Wilder平滑
	for i := period + 1; i < len(klines); i++ {
		atr = (atr*float64(period-1) + trs[i]) / float64(period)
	}

	return atr
}

// calculateADX 计算ADX (平均趋向指数)
func calculateADX(klines []Kline, period int) float64 {
	if len(klines) <= period*2 {
		return 0
	}

	// 计算+DM和-DM
	plusDM := make([]float64, len(klines))
	minusDM := make([]float64, len(klines))
	tr := make([]float64, len(klines))

	for i := 1; i < len(klines); i++ {
		highDiff := klines[i].High - klines[i-1].High
		lowDiff := klines[i-1].Low - klines[i].Low

		if highDiff > lowDiff && highDiff > 0 {
			plusDM[i] = highDiff
		}
		if lowDiff > highDiff && lowDiff > 0 {
			minusDM[i] = lowDiff
		}

		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close
		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)
		tr[i] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// 计算平滑的+DI和-DI
	smoothPlusDM := 0.0
	smoothMinusDM := 0.0
	smoothTR := 0.0

	for i := 1; i <= period; i++ {
		smoothPlusDM += plusDM[i]
		smoothMinusDM += minusDM[i]
		smoothTR += tr[i]
	}

	plusDI := make([]float64, len(klines))
	minusDI := make([]float64, len(klines))
	dx := make([]float64, len(klines))

	for i := period; i < len(klines); i++ {
		if i > period {
			smoothPlusDM = smoothPlusDM - smoothPlusDM/float64(period) + plusDM[i]
			smoothMinusDM = smoothMinusDM - smoothMinusDM/float64(period) + minusDM[i]
			smoothTR = smoothTR - smoothTR/float64(period) + tr[i]
		}

		if smoothTR != 0 {
			plusDI[i] = 100 * smoothPlusDM / smoothTR
			minusDI[i] = 100 * smoothMinusDM / smoothTR
		}

		diSum := plusDI[i] + minusDI[i]
		if diSum != 0 {
			dx[i] = 100 * math.Abs(plusDI[i]-minusDI[i]) / diSum
		}
	}

	// 计算ADX (DX的平滑移动平均)
	adx := 0.0
	for i := period; i < period*2 && i < len(klines); i++ {
		adx += dx[i]
	}
	adx = adx / float64(period)

	for i := period * 2; i < len(klines); i++ {
		adx = (adx*float64(period-1) + dx[i]) / float64(period)
	}

	return adx
}

// calculateBBWidth 计算布林带带宽
func calculateBBWidth(klines []Kline, period int, stdDev float64) float64 {
	if len(klines) < period {
		return 0
	}

	// 计算SMA
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	sma := sum / float64(period)

	// 计算标准差
	variance := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		diff := klines[i].Close - sma
		variance += diff * diff
	}
	stdDeviation := math.Sqrt(variance / float64(period))

	// 布林带带宽 = (上轨 - 下轨) / 中轨
	upper := sma + (stdDev * stdDeviation)
	lower := sma - (stdDev * stdDeviation)

	if sma != 0 {
		return (upper - lower) / sma
	}
	return 0
}

// calculateRVOL 计算相对成交量
func calculateRVOL(klines []Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	// 计算过去period根K线的平均成交量
	sum := 0.0
	for i := len(klines) - period - 1; i < len(klines)-1; i++ {
		sum += klines[i].Volume
	}
	avgVolume := sum / float64(period)

	// 当前K线成交量 / 平均成交量
	currentVolume := klines[len(klines)-1].Volume
	if avgVolume != 0 {
		return currentVolume / avgVolume
	}
	return 0
}

// calculateTimeframeData 计算时间周期数据
func calculateTimeframeData(klines []Kline, timeframe string, currentPrice float64, includeATR bool) *TimeframeData {
	data := &TimeframeData{
		Timeframe:   timeframe,
		Price:       currentPrice,
		PriceSeries: make([]float64, 0, 10),
		EMA20Series: make([]float64, 0, 10),
		RSISeries:   make([]float64, 0, 10),
	}

	// 计算EMA
	data.EMA20 = calculateEMA(klines, 20)
	data.EMA50 = calculateEMA(klines, 50)
	data.EMA200 = calculateEMA(klines, 200)

	// 计算RSI
	data.RSI = calculateRSI(klines, 14)

	// 计算成交量分布和POC
	data.VolumeProfile = calculateVolumeProfile(klines, 50)
	if data.VolumeProfile != nil {
		data.POC = data.VolumeProfile.POC
	}

	// 计算市场结构
	data.StructureDetail = calculateMarketStructure(klines, 3)
	if data.StructureDetail != nil {
		data.MarketStructure = data.StructureDetail.LastPattern
		if data.MarketStructure == "" {
			data.MarketStructure = data.StructureDetail.Trend
		}
	}

	// 计算RSI背离 (用于震荡区间交易) - 检测最近5个周期内的背离
	data.RSIDivergence = calculateRSIDivergence(klines, 14, 10, 5)

	// 计算K线反转信号 (仅对1h和4h周期)
	if timeframe == "1h" || timeframe == "4h" {
		data.CandleReversal = detectCandleReversal(klines)
	}

	// 计算ATR (波动率指标)
	if includeATR {
		data.ATR = calculateATR(klines, 14)
	}

	// 计算ADX (平均趋向指数)
	data.ADX = calculateADX(klines, 14)

	// 计算布林带带宽
	data.BBWidth = calculateBBWidth(klines, 20, 2.0)

	// 计算相对成交量 (当前成交量 / 过去20根K线平均成交量)
	data.RVOL = calculateRVOL(klines, 20)

	// 计算时间序列数据 (最近10个数据点)
	seriesStart := len(klines) - 10
	if seriesStart < 0 {
		seriesStart = 0
	}

	for i := seriesStart; i < len(klines); i++ {
		// 价格序列
		data.PriceSeries = append(data.PriceSeries, klines[i].Close)

		// EMA20序列 (需要至少20个数据点)
		if i >= 19 {
			ema20 := calculateEMA(klines[:i+1], 20)
			data.EMA20Series = append(data.EMA20Series, ema20)
		}

		// RSI序列 (需要至少15个数据点)
		if i >= 14 {
			rsi := calculateRSI(klines[:i+1], 14)
			data.RSISeries = append(data.RSISeries, rsi)
		}
	}

	return data
}

// getOpenInterestData 获取OI数据
func getOpenInterestData(symbol string) (*OIData, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// 构建请求获取meta信息
	url := "https://api.hyperliquid.xyz/info"
	requestBody := map[string]any{
		"type": "metaAndAssetCtxs",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应 - Hyperliquid返回 [meta, assetCtxs]
	var result []any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// 查找对应币种的OI数据
	if len(result) >= 2 {
		// 解析meta获取币种列表
		metaMap, ok := result[0].(map[string]any)
		if !ok {
			return &OIData{Latest: 0, Average: 0}, nil
		}

		universe, ok := metaMap["universe"].([]any)
		if !ok {
			return &OIData{Latest: 0, Average: 0}, nil
		}

		// 查找币种索引
		coinIndex := -1
		for i, asset := range universe {
			assetMap, ok := asset.(map[string]any)
			if !ok {
				continue
			}
			if assetMap["name"] == coin {
				coinIndex = i
				break
			}
		}

		if coinIndex == -1 {
			return &OIData{Latest: 0, Average: 0}, nil
		}

		// 获取对应的assetCtx
		assetCtxs, ok := result[1].([]any)
		if !ok || coinIndex >= len(assetCtxs) {
			return &OIData{Latest: 0, Average: 0}, nil
		}

		ctxMap, ok := assetCtxs[coinIndex].(map[string]any)
		if !ok {
			return &OIData{Latest: 0, Average: 0}, nil
		}

		oiStr, _ := ctxMap["openInterest"].(string)
		oi, _ := strconv.ParseFloat(oiStr, 64)

		return &OIData{
			Latest:  oi,
			Average: oi * 0.999, // 近似平均值
		}, nil
	}

	// 如果没找到，返回默认值
	return &OIData{
		Latest:  0,
		Average: 0,
	}, nil
}

// getFundingRate 获取资金费率
func getFundingRate(symbol string) (float64, error) {
	coin := convertSymbolToHyperliquid(symbol)

	// 构建请求获取meta信息
	url := "https://api.hyperliquid.xyz/info"
	requestBody := map[string]any{
		"type": "metaAndAssetCtxs",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return 0, fmt.Errorf("构建请求失败: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// 解析响应 - Hyperliquid返回 [meta, assetCtxs]
	var result []any
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	// 查找对应币种的funding rate
	if len(result) >= 2 {
		// 解析meta获取币种列表
		metaMap, ok := result[0].(map[string]any)
		if !ok {
			return 0, nil
		}

		universe, ok := metaMap["universe"].([]any)
		if !ok {
			return 0, nil
		}

		// 查找币种索引
		coinIndex := -1
		for i, asset := range universe {
			assetMap, ok := asset.(map[string]any)
			if !ok {
				continue
			}
			if assetMap["name"] == coin {
				coinIndex = i
				break
			}
		}

		if coinIndex == -1 {
			return 0, nil
		}

		// 获取对应的assetCtx
		assetCtxs, ok := result[1].([]any)
		if !ok || coinIndex >= len(assetCtxs) {
			return 0, nil
		}

		ctxMap, ok := assetCtxs[coinIndex].(map[string]any)
		if !ok {
			return 0, nil
		}

		fundingStr, _ := ctxMap["funding"].(string)
		rate, _ := strconv.ParseFloat(fundingStr, 64)
		return rate, nil
	}

	return 0, nil
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// calculateRSIDivergence 计算RSI背离 (检测最近validPeriods个周期内的背离)
// 优化版本：预先计算所有需要的RSI值，避免重复计算
func calculateRSIDivergence(klines []Kline, rsiPeriod int, lookback int, validPeriods int) *RSIDivergence {
	if len(klines) < rsiPeriod+lookback*2 {
		return &RSIDivergence{
			Type:        "NONE",
			Description: "数据不足以检测背离",
		}
	}

	// 预先计算所有需要的RSI值（只计算一次）
	rsiCache := make([]float64, len(klines))
	for i := rsiPeriod; i < len(klines); i++ {
		rsiCache[i] = calculateRSI(klines[:i+1], rsiPeriod)
	}

	// 从最近的K线开始，向前检测validPeriods个周期
	for periodsAgo := 0; periodsAgo < validPeriods && periodsAgo < len(klines)-rsiPeriod-lookback*2; periodsAgo++ {
		// 计算检测点的索引
		checkIndex := len(klines) - 1 - periodsAgo
		if checkIndex < rsiPeriod+lookback*2 {
			break
		}

		// 计算当前点的RSI和价格
		currentRSI := rsiCache[checkIndex]
		currentPrice := klines[checkIndex].Close

		// 获取lookback周期内的最高价和最低价
		highestPrice := klines[checkIndex].High
		lowestPrice := klines[checkIndex].Low
		highestRSI := currentRSI
		lowestRSI := currentRSI

		// 计算lookback周期内的历史最高/最低价格和RSI（使用缓存的RSI）
		for i := checkIndex - lookback + 1; i <= checkIndex; i++ {
			if klines[i].High > highestPrice {
				highestPrice = klines[i].High
			}
			if klines[i].Low < lowestPrice {
				lowestPrice = klines[i].Low
			}

			if rsiCache[i] > highestRSI {
				highestRSI = rsiCache[i]
			}
			if rsiCache[i] < lowestRSI {
				lowestRSI = rsiCache[i]
			}
		}

		// 获取lookback周期前的历史最高/最低价格和RSI
		prevHighestPrice := klines[checkIndex-lookback].High
		prevLowestPrice := klines[checkIndex-lookback].Low
		prevHighestRSI := rsiCache[checkIndex-lookback]
		prevLowestRSI := prevHighestRSI

		for i := checkIndex - lookback*2 + 1; i < checkIndex-lookback && i >= 0; i++ {
			if klines[i].High > prevHighestPrice {
				prevHighestPrice = klines[i].High
			}
			if klines[i].Low < prevLowestPrice {
				prevLowestPrice = klines[i].Low
			}

			if rsiCache[i] > prevHighestRSI {
				prevHighestRSI = rsiCache[i]
			}
			if rsiCache[i] < prevLowestRSI {
				prevLowestRSI = rsiCache[i]
			}
		}

		// 检测看跌背离: 价格创新高但RSI未创新高
		priceHigherHigh := currentPrice == highestPrice && highestPrice > prevHighestPrice
		rsiHigherHigh := currentRSI == highestRSI && highestRSI > prevHighestRSI

		if priceHigherHigh && !rsiHigherHigh {
			validityLeft := validPeriods - periodsAgo
			description := fmt.Sprintf("看跌背离: 价格创新高(%.2f > %.2f)，RSI未创新高(%.2f vs %.2f)",
				highestPrice, prevHighestPrice, currentRSI, prevHighestRSI)
			if periodsAgo > 0 {
				description = fmt.Sprintf("前%d周期%s", periodsAgo, description)
			}

			return &RSIDivergence{
				Type:          "BEARISH",
				Strength:      "REGULAR",
				Description:   description,
				PeriodsAgo:    periodsAgo,
				ValidityLeft:  validityLeft,
				PricePoint1:   prevHighestPrice,
				PricePoint2:   highestPrice,
				RSIPoint1:     prevHighestRSI,
				RSIPoint2:     currentRSI,
				DetectedIndex: checkIndex,
			}
		}

		// 检测看涨背离: 价格创新低但RSI未创新低
		priceLowerLow := currentPrice == lowestPrice && lowestPrice < prevLowestPrice
		rsiLowerLow := currentRSI == lowestRSI && lowestRSI < prevLowestRSI

		if priceLowerLow && !rsiLowerLow {
			validityLeft := validPeriods - periodsAgo
			description := fmt.Sprintf("看涨背离: 价格创新低(%.2f < %.2f)，RSI未创新低(%.2f vs %.2f)",
				lowestPrice, prevLowestPrice, currentRSI, prevLowestRSI)
			if periodsAgo > 0 {
				description = fmt.Sprintf("前%d周期%s", periodsAgo, description)
			}

			return &RSIDivergence{
				Type:          "BULLISH",
				Strength:      "REGULAR",
				Description:   description,
				PeriodsAgo:    periodsAgo,
				ValidityLeft:  validityLeft,
				PricePoint1:   prevLowestPrice,
				PricePoint2:   lowestPrice,
				RSIPoint1:     prevLowestRSI,
				RSIPoint2:     currentRSI,
				DetectedIndex: checkIndex,
			}
		}
	}

	return &RSIDivergence{
		Type:        "NONE",
		Description: "未检测到背离信号",
	}
}

// parseFloat 解析float值
func parseFloat(v any) (float64, error) {
	switch val := v.(type) {
	case string:
		return strconv.ParseFloat(val, 64)
	case float64:
		return val, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}

// detectCandleReversal 检测K线反转形态
func detectCandleReversal(klines []Kline) *CandleReversal {
	if len(klines) < 2 {
		return &CandleReversal{
			SingleCandle: &SingleCandlePattern{Type: "NONE", Description: "数据不足"},
			DoubleCandle: &DoubleCandlePattern{Type: "NONE", Description: "数据不足"},
		}
	}

	return &CandleReversal{
		SingleCandle: detectSingleCandlePattern(klines),
		DoubleCandle: detectDoubleCandlePattern(klines),
	}
}

// detectSingleCandlePattern 检测单K线反转形态
func detectSingleCandlePattern(klines []Kline) *SingleCandlePattern {
	if len(klines) < 1 {
		return &SingleCandlePattern{Type: "NONE", Description: "数据不足"}
	}

	current := klines[len(klines)-1]
	body := math.Abs(current.Close - current.Open)
	totalRange := current.High - current.Low

	if totalRange == 0 {
		return &SingleCandlePattern{Type: "NONE", Description: "无价格波动"}
	}

	upperShadow := current.High - math.Max(current.Open, current.Close)
	lowerShadow := math.Min(current.Open, current.Close) - current.Low

	// 看涨锤子线 (Bullish Hammer)
	// 特征: 下影线长(至少是实体的2倍), 上影线很短或没有, 实体在上部
	if lowerShadow >= body*2 && upperShadow <= body*0.3 && current.Close > current.Open {
		strength := math.Min(lowerShadow/body/3, 1.0) // 下影线越长，信号越强
		return &SingleCandlePattern{
			Type:        "BULLISH_HAMMER",
			Description: fmt.Sprintf("看涨锤子线: 下影线%.2f%%, 实体%.2f%%", lowerShadow/totalRange*100, body/totalRange*100),
			Strength:    strength,
		}
	}

	// 看涨倒锤子线 (Bullish Inverted Hammer)
	// 特征: 上影线长(至少是实体的2倍), 下影线很短或没有, 实体在下部
	if upperShadow >= body*2 && lowerShadow <= body*0.3 && current.Close > current.Open {
		strength := math.Min(upperShadow/body/3, 1.0)
		return &SingleCandlePattern{
			Type:        "BULLISH_INVERTED_HAMMER",
			Description: fmt.Sprintf("看涨倒锤子线: 上影线%.2f%%, 实体%.2f%%", upperShadow/totalRange*100, body/totalRange*100),
			Strength:    strength,
		}
	}

	// 看跌流星线 (Bearish Shooting Star)
	// 特征: 上影线长(至少是实体的2倍), 下影线很短或没有, 实体在下部
	if upperShadow >= body*2 && lowerShadow <= body*0.3 && current.Close < current.Open {
		strength := math.Min(upperShadow/body/3, 1.0)
		return &SingleCandlePattern{
			Type:        "BEARISH_SHOOTING_STAR",
			Description: fmt.Sprintf("看跌流星线: 上影线%.2f%%, 实体%.2f%%", upperShadow/totalRange*100, body/totalRange*100),
			Strength:    strength,
		}
	}

	// 看跌上吊线 (Bearish Hanging Man)
	// 特征: 下影线长(至少是实体的2倍), 上影线很短或没有, 实体在上部
	if lowerShadow >= body*2 && upperShadow <= body*0.3 && current.Close < current.Open {
		strength := math.Min(lowerShadow/body/3, 1.0)
		return &SingleCandlePattern{
			Type:        "BEARISH_HANGING_MAN",
			Description: fmt.Sprintf("看跌上吊线: 下影线%.2f%%, 实体%.2f%%", lowerShadow/totalRange*100, body/totalRange*100),
			Strength:    strength,
		}
	}

	// 看涨十字星 (Bullish Doji) - 实体很小
	if body/totalRange <= 0.1 && len(klines) >= 2 {
		prev := klines[len(klines)-2]
		if prev.Close < prev.Open { // 前一根是阴线
			strength := 0.5 // 十字星信号相对较弱
			return &SingleCandlePattern{
				Type:        "BULLISH_DOJI",
				Description: fmt.Sprintf("看涨十字星: 实体仅%.2f%%", body/totalRange*100),
				Strength:    strength,
			}
		}
	}

	// 看跌十字星 (Bearish Doji)
	if body/totalRange <= 0.1 && len(klines) >= 2 {
		prev := klines[len(klines)-2]
		if prev.Close > prev.Open { // 前一根是阳线
			strength := 0.5
			return &SingleCandlePattern{
				Type:        "BEARISH_DOJI",
				Description: fmt.Sprintf("看跌十字星: 实体仅%.2f%%", body/totalRange*100),
				Strength:    strength,
			}
		}
	}

	return &SingleCandlePattern{Type: "NONE", Description: "未检测到单K线反转形态"}
}

// detectDoubleCandlePattern 检测双K线反转形态
func detectDoubleCandlePattern(klines []Kline) *DoubleCandlePattern {
	if len(klines) < 2 {
		return &DoubleCandlePattern{Type: "NONE", Description: "数据不足"}
	}

	prev := klines[len(klines)-2]
	current := klines[len(klines)-1]

	prevBody := math.Abs(prev.Close - prev.Open)
	currentBody := math.Abs(current.Close - current.Open)
	prevRange := prev.High - prev.Low
	currentRange := current.High - current.Low

	if prevRange == 0 || currentRange == 0 {
		return &DoubleCandlePattern{Type: "NONE", Description: "无价格波动"}
	}

	// 看涨吞没 (Bullish Engulfing)
	// 特征: 前一根阴线，当前阳线完全吞没前一根
	if prev.Close < prev.Open && current.Close > current.Open {
		if current.Open <= prev.Close && current.Close >= prev.Open {
			engulfRatio := currentBody / prevBody
			strength := math.Min(engulfRatio/2, 1.0) // 吞没程度越大，信号越强
			return &DoubleCandlePattern{
				Type:        "BULLISH_ENGULFING",
				Description: fmt.Sprintf("看涨吞没: 当前阳线吞没前阴线%.1f倍", engulfRatio),
				Strength:    strength,
			}
		}
	}

	// 看跌吞没 (Bearish Engulfing)
	// 特征: 前一根阳线，当前阴线完全吞没前一根
	if prev.Close > prev.Open && current.Close < current.Open {
		if current.Open >= prev.Close && current.Close <= prev.Open {
			engulfRatio := currentBody / prevBody
			strength := math.Min(engulfRatio/2, 1.0)
			return &DoubleCandlePattern{
				Type:        "BEARISH_ENGULFING",
				Description: fmt.Sprintf("看跌吞没: 当前阴线吞没前阳线%.1f倍", engulfRatio),
				Strength:    strength,
			}
		}
	}

	// 看涨刺透形态 (Bullish Piercing Pattern)
	// 特征: 前一根阴线，当前阳线开盘低于前收盘，收盘在前实体中部以上
	if prev.Close < prev.Open && current.Close > current.Open {
		prevMidpoint := (prev.Open + prev.Close) / 2
		if current.Open < prev.Close && current.Close > prevMidpoint && current.Close < prev.Open {
			penetration := (current.Close - prev.Close) / prevBody
			strength := math.Min(penetration, 1.0)
			return &DoubleCandlePattern{
				Type:        "BULLISH_PIERCING",
				Description: fmt.Sprintf("看涨刺透: 刺入前阴线%.1f%%", penetration*100),
				Strength:    strength,
			}
		}
	}

	// 看跌乌云盖顶 (Bearish Dark Cloud Cover)
	// 特征: 前一根阳线，当前阴线开盘高于前收盘，收盘在前实体中部以下
	if prev.Close > prev.Open && current.Close < current.Open {
		prevMidpoint := (prev.Open + prev.Close) / 2
		if current.Open > prev.Close && current.Close < prevMidpoint && current.Close > prev.Open {
			penetration := (prev.Close - current.Close) / prevBody
			strength := math.Min(penetration, 1.0)
			return &DoubleCandlePattern{
				Type:        "BEARISH_DARK_CLOUD",
				Description: fmt.Sprintf("看跌乌云盖顶: 覆盖前阳线%.1f%%", penetration*100),
				Strength:    strength,
			}
		}
	}

	// 看涨孕线 (Bullish Harami)
	// 特征: 前一根大阴线，当前小阳线完全在前一根实体内
	if prev.Close < prev.Open && current.Close > current.Open {
		if current.Open >= prev.Close && current.Close <= prev.Open && currentBody < prevBody*0.5 {
			strength := 0.6 // 孕线信号相对中等
			return &DoubleCandlePattern{
				Type:        "BULLISH_HARAMI",
				Description: fmt.Sprintf("看涨孕线: 小阳线在大阴线内(%.1f%%)", currentBody/prevBody*100),
				Strength:    strength,
			}
		}
	}

	// 看跌孕线 (Bearish Harami)
	// 特征: 前一根大阳线，当前小阴线完全在前一根实体内
	if prev.Close > prev.Open && current.Close < current.Open {
		if current.Open <= prev.Close && current.Close >= prev.Open && currentBody < prevBody*0.5 {
			strength := 0.6
			return &DoubleCandlePattern{
				Type:        "BEARISH_HARAMI",
				Description: fmt.Sprintf("看跌孕线: 小阴线在大阳线内(%.1f%%)", currentBody/prevBody*100),
				Strength:    strength,
			}
		}
	}

	// 看涨启明星 (需要检查前面是否有下跌趋势)
	if len(klines) >= 3 {
		prevPrev := klines[len(klines)-3]
		// 前两根: 大阴线 + 小实体(跳空低开) + 大阳线(收盘在第一根中部以上)
		if prevPrev.Close < prevPrev.Open && current.Close > current.Open {
			prevPrevBody := math.Abs(prevPrev.Close - prevPrev.Open)
			if prevBody < prevPrevBody*0.3 && currentBody > prevPrevBody*0.5 {
				if prev.High < prevPrev.Close && current.Close > (prevPrev.Open+prevPrev.Close)/2 {
					strength := 0.8 // 启明星是较强的反转信号
					return &DoubleCandlePattern{
						Type:        "BULLISH_MORNING_STAR",
						Description: "看涨启明星: 三K线底部反转形态",
						Strength:    strength,
					}
				}
			}
		}
	}

	// 看跌黄昏星
	if len(klines) >= 3 {
		prevPrev := klines[len(klines)-3]
		// 前两根: 大阳线 + 小实体(跳空高开) + 大阴线(收盘在第一根中部以下)
		if prevPrev.Close > prevPrev.Open && current.Close < current.Open {
			prevPrevBody := math.Abs(prevPrev.Close - prevPrev.Open)
			if prevBody < prevPrevBody*0.3 && currentBody > prevPrevBody*0.5 {
				if prev.Low > prevPrev.Close && current.Close < (prevPrev.Open+prevPrev.Close)/2 {
					strength := 0.8
					return &DoubleCandlePattern{
						Type:        "BEARISH_EVENING_STAR",
						Description: "看跌黄昏星: 三K线顶部反转形态",
						Strength:    strength,
					}
				}
			}
		}
	}

	return &DoubleCandlePattern{Type: "NONE", Description: "未检测到双K线反转形态"}
}
