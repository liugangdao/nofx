package market

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
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
	BBUpper         float64            // 布林带上轨 (仅1h)
	BBMiddle        float64            // 布林带中轨 (仅1h)
	BBLower         float64            // 布林带下轨 (仅1h)
	ATR             float64            // ATR指标 (仅1h)
	VolumeProfile   *VolumeProfileData // 成交量分布
	StructureDetail *MarketStructure   // 详细市场结构

	// 时间序列数据 (最近10个数据点，从旧到新)
	PriceSeries  []float64 // 价格序列
	EMA20Series  []float64 // EMA20序列
	RSISeries    []float64 // RSI序列
	VolumeSeries []float64 // 成交量序列
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
	timeframe4h := calculateTimeframeData(klines4h, "4h", currentPrice, false)
	timeframe1h := calculateTimeframeData(klines1h, "1h", currentPrice, true) // 1h包含BB和ATR

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

// getKlines 从Binance获取K线数据
func getKlines(symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rawData [][]interface{}
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

// calculateBollingerBands 计算布林带
// 返回值: upper, middle, lower
func calculateBollingerBands(klines []Kline, period int, stdDev float64) (float64, float64, float64) {
	if len(klines) < period {
		return 0, 0, 0
	}

	// 计算中轨 (SMA)
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	middle := sum / float64(period)

	// 计算标准差
	variance := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		diff := klines[i].Close - middle
		variance += diff * diff
	}
	stdDeviation := math.Sqrt(variance / float64(period))

	// 计算上轨和下轨
	upper := middle + (stdDev * stdDeviation)
	lower := middle - (stdDev * stdDeviation)

	return upper, middle, lower
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

// calculateTimeframeData 计算时间周期数据
func calculateTimeframeData(klines []Kline, timeframe string, currentPrice float64, includeBBAndATR bool) *TimeframeData {
	data := &TimeframeData{
		Timeframe:    timeframe,
		Price:        currentPrice,
		PriceSeries:  make([]float64, 0, 10),
		EMA20Series:  make([]float64, 0, 10),
		RSISeries:    make([]float64, 0, 10),
		VolumeSeries: make([]float64, 0, 10),
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
	data.StructureDetail = calculateMarketStructure(klines, 5)
	if data.StructureDetail != nil {
		data.MarketStructure = data.StructureDetail.LastPattern
		if data.MarketStructure == "" {
			data.MarketStructure = data.StructureDetail.Trend
		}
	}

	// 仅1小时周期计算布林带和ATR
	if includeBBAndATR {
		upper, middle, lower := calculateBollingerBands(klines, 20, 2.0)
		data.BBUpper = upper
		data.BBMiddle = middle
		data.BBLower = lower
		data.ATR = calculateATR(klines, 14)
	}

	// 计算时间序列数据 (最近10个数据点)
	seriesStart := len(klines) - 10
	if seriesStart < 0 {
		seriesStart = 0
	}

	for i := seriesStart; i < len(klines); i++ {
		// 价格序列
		data.PriceSeries = append(data.PriceSeries, klines[i].Close)

		// 成交量序列
		data.VolumeSeries = append(data.VolumeSeries, klines[i].Volume)

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
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/openInterest?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		OpenInterest string `json:"openInterest"`
		Symbol       string `json:"symbol"`
		Time         int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	oi, _ := strconv.ParseFloat(result.OpenInterest, 64)

	return &OIData{
		Latest:  oi,
		Average: oi * 0.999, // 近似平均值
	}, nil
}

// getFundingRate 获取资金费率
func getFundingRate(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/premiumIndex?symbol=%s", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Symbol          string `json:"symbol"`
		MarkPrice       string `json:"markPrice"`
		IndexPrice      string `json:"indexPrice"`
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
		InterestRate    string `json:"interestRate"`
		Time            int64  `json:"time"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	rate, _ := strconv.ParseFloat(result.LastFundingRate, 64)
	return rate, nil
}

// Normalize 标准化symbol,确保是USDT交易对
func Normalize(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasSuffix(symbol, "USDT") {
		return symbol
	}
	return symbol + "USDT"
}

// parseFloat 解析float值
func parseFloat(v interface{}) (float64, error) {
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
