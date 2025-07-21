package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"math"
	"sort"
)

func (s *tradingService) EvaluatePositionMonitoring(
	ctx context.Context,
	stockPosition *model.StockPosition,
	analyses []model.StockAnalysis,
	supports []dto.Level, // Tambahkan parameter supports
	resistances []dto.Level,
) (*dto.PositionAnalysis, error) {

	// --- Inisialisasi dan Pengambilan Data ---
	result := &dto.PositionAnalysis{
		Ticker:          stockPosition.StockCode,
		EntryPrice:      stockPosition.BuyPrice,
		TakeProfitPrice: stockPosition.TakeProfitPrice,
		StopLossPrice:   stockPosition.StopLossPrice,
		Insight:         []dto.Insight{},
		// Inisialisasi dengan nilai LAMA dari database untuk dibaca oleh helper
		TrailingProfitPrice:  stockPosition.TrailingProfitPrice,
		HighestPriceSinceTTP: stockPosition.HighestPriceSinceTTP,
	}

	if stockPosition.TrailingProfitPrice > stockPosition.TakeProfitPrice {
		result.TakeProfitPrice = stockPosition.TrailingProfitPrice
	}

	if stockPosition.TrailingStopPrice > stockPosition.StopLossPrice {
		result.StopLossPrice = stockPosition.TrailingStopPrice
	}

	timeframes, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		return nil, err
	}
	mainData, err := s.findMainAnalysisData(analyses, timeframes)
	if err != nil {
		return nil, err
	}
	if mainData.MainTA == nil {
		return result, fmt.Errorf("no main technical analysis data found for %s", stockPosition.StockCode)
	}
	if len(mainData.MainOHLCV) == 0 {
		return result, fmt.Errorf("no main OHLCV data found for %s", stockPosition.StockCode)
	}

	symbolWithExchange := stockPosition.Exchange + ":" + stockPosition.StockCode
	marketPrice, _ := cache.GetFromCache[float64](fmt.Sprintf(common.KEY_LAST_PRICE, symbolWithExchange))
	if marketPrice == 0 && len(analyses) > 0 {
		marketPrice = analyses[len(analyses)-1].MarketPrice
	}

	//fallback last price from main ohlcv
	if len(mainData.MainOHLCV) > 0 && marketPrice == 0 {
		marketPrice = mainData.MainOHLCV[len(mainData.MainOHLCV)-1].Close
	}
	result.LastPrice = marketPrice

	// Variabel untuk menyimpan keputusan prioritas
	var finalSignal dto.Signal = ""

	// --- Prioritas #1: Cut Loss ---
	if result.LastPrice <= result.StopLossPrice {
		result.Status = dto.Dangerous
		finalSignal = dto.CutLoss
		result.Insight = append(result.Insight, dto.Insight{Text: fmt.Sprintf("SINYAL CUT LOSS: Harga (%.2f) telah menyentuh Stop Loss (%.2f).", result.LastPrice, result.StopLossPrice), Weight: 100})
	}

	// --- Dapatkan Evaluasi Baseline menggunakan Sistem Skor Baru ---
	score, insights, techSignal := s.calculateAdvancedScore(stockPosition, mainData.MainTA, mainData.SecondaryTA, mainData.MainOHLCV, marketPrice, supports, resistances)
	result.Score = score
	result.TechnicalSignal = techSignal
	result.Insight = append(result.Insight, insights...)

	// Prioritas #2: Kelola Trailing Take Profit (TTP)
	// Fungsi ini akan mengubah nilai TTP di dalam 'result' dan bisa menyarankan sinyal exit
	s.evaluateTrailingTakeProfit(result, stockPosition, mainData.MainTA, mainData.MainOHLCV)
	if result.Signal == dto.TakeProfit {
		finalSignal = dto.TakeProfit
	}

	// Evaluasi Trailing Stop Loss
	s.evaluateTrailingStop(result, mainData.MainTA, mainData.SecondaryTA, techSignal, mainData.MainOHLCV)

	// Tentukan status akhir
	s.determineFinalStatus(result)

	// --- Bagian Akhir: Tentukan Sinyal Final ---
	if finalSignal != "" {
		result.Signal = finalSignal
	} else if result.Signal == "" {
		result.Signal = dto.Hold
	}

	// Isi sisa informasi
	summaryStruct := s.CreateIndicatorSummary(mainData.MainTA, mainData.MainOHLCV)
	summaryBytes, err := json.Marshal(summaryStruct)
	if err == nil {
		result.IndicatorSummary = string(summaryBytes)
	}

	// Sort insights by weight descending
	sort.Slice(result.Insight, func(i, j int) bool {
		return result.Insight[i].Weight > result.Insight[j].Weight
	})

	return result, nil
}

// determineFinalStatus menetapkan status akhir posisi (Safe, Warning, Dangerous) berdasarkan skor dan kondisi kritis.
func (s *tradingService) determineFinalStatus(result *dto.PositionAnalysis) {
	// Prioritas 1: Periksa kondisi kritis yang paling berbahaya, seperti jarak ke Stop Loss.
	riskRange := result.LastPrice - result.StopLossPrice
	entryToSLRange := result.EntryPrice - result.StopLossPrice
	if entryToSLRange > 0 && (riskRange/entryToSLRange) < 0.25 {
		result.Status = dto.Dangerous
		// Insight ini sangat penting dan spesifik, jadi kita pertahankan.
		result.Insight = append(result.Insight, dto.Insight{Text: fmt.Sprintf("[Kondisi Bahaya]: Jarak ke Stop Loss < 25%% (Harga: %.2f, SL: %.2f).", result.LastPrice, result.StopLossPrice), Weight: 100})
		return
	}

	// Jangan menimpa status yang lebih kritis yang mungkin sudah diatur oleh logika lain (misalnya, trailing stop).
	if result.Status == dto.Dangerous {
		return
	}

	// Prioritas 2: Tentukan status berdasarkan skor komprehensif.
	switch {
	case result.Score >= 70:
		result.Status = dto.Safe
	case result.Score >= 50:
		result.Status = dto.Safe
	case result.Score >= 30:
		result.Status = dto.Warning
	default:
		result.Status = dto.Dangerous
	}
}

func (s *tradingService) isBearishEngulfing(last, prev dto.StockOHLCV) bool {
	return last.Close < last.Open && prev.Close > prev.Open && last.Open > prev.Close && last.Close < prev.Open
}

func (s *tradingService) calculateAverageVolume(ohlcv []dto.StockOHLCV, period int) float64 {
	if len(ohlcv) < period {
		period = len(ohlcv)
	}
	if period == 0 {
		return 0
	}
	var totalVolume float64
	startIndex := len(ohlcv) - period
	for i := startIndex; i < len(ohlcv); i++ {
		totalVolume += ohlcv[i].Volume
	}
	return float64(totalVolume) / float64(period)
}

func (s *tradingService) findMainAnalysisData(analyses []model.StockAnalysis, timeframes []dto.DataTimeframe) (*dto.MainAnalysisData, error) {
	var result dto.MainAnalysisData
	sort.Slice(timeframes, func(i, j int) bool { return timeframes[i].Weight > timeframes[j].Weight })
	if len(timeframes) < 1 {
		return nil, fmt.Errorf("no timeframes configured")
	}
	mainTF := timeframes[0]
	var secondaryTF dto.DataTimeframe
	if len(timeframes) >= 2 {
		secondaryTF = timeframes[1]
	}
	for _, analysis := range analyses {
		var ta dto.TradingViewScanner
		var ohlcvs []dto.StockOHLCV
		if err := json.Unmarshal([]byte(analysis.OHLCV), &ohlcvs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(analysis.TechnicalData), &ta); err != nil {
			return nil, err
		}
		if analysis.Timeframe == mainTF.Interval {
			result.MainTA = &ta
			result.MainOHLCV = ohlcvs
			result.MainTimeframe = analysis.Timeframe
		}
		if secondaryTF.Interval != "" && analysis.Timeframe == secondaryTF.Interval {
			result.SecondaryTA = &ta
			result.SecondaryOHLCV = ohlcvs
			result.SecondaryTimeframe = analysis.Timeframe
		}
	}
	return &result, nil
}

// evaluateTrailingStop menentukan apakah sinyal Trailing Stop harus diberikan dengan logika yang lebih cerdas.
func (s *tradingService) evaluateTrailingStop(result *dto.PositionAnalysis, mainTA, secondaryTA *dto.TradingViewScanner, technicalSignal string, mainOHLCV []dto.StockOHLCV) {
	totalProfitRange := result.TakeProfitPrice - result.EntryPrice
	currentProfit := result.LastPrice - result.EntryPrice
	hasSignificantProfit := totalProfitRange > 0 && (currentProfit/totalProfitRange) > 0.6
	isSignalStrong := (technicalSignal == dto.SignalStrongBuy || technicalSignal == dto.SignalBuy)
	triggerA := isSignalStrong && hasSignificantProfit
	var breakoutResistanceLevel float64
	triggerB := false
	if secondaryTA != nil {
		resistanceR1_4H := secondaryTA.Value.Pivots.Classic.R1
		if currentProfit > 0 && result.LastPrice > resistanceR1_4H && resistanceR1_4H > result.EntryPrice*1.01 {
			triggerB = true
			breakoutResistanceLevel = resistanceR1_4H
		}
	}
	if !triggerA && !triggerB {
		return
	}
	bestProposedSL := 0.0
	reasonForUpdate := ""
	if result.EntryPrice > bestProposedSL {
		bestProposedSL = result.EntryPrice
		reasonForUpdate = "mengamankan posisi ke breakeven"
	}
	if breakoutResistanceLevel > bestProposedSL {
		bestProposedSL = breakoutResistanceLevel
		reasonForUpdate = fmt.Sprintf("resistance kunci di %s (%.2f) telah ditembus", secondaryTA.Timeframe, breakoutResistanceLevel)
	}
	if len(mainOHLCV) >= 2 {
		dynamicSL := mainOHLCV[len(mainOHLCV)-2].Low
		if dynamicSL > bestProposedSL {
			bestProposedSL = dynamicSL
			reasonForUpdate = fmt.Sprintf("mengikuti support dinamis dari low candle %s", mainTA.Timeframe)
		}
	}
	if bestProposedSL > result.StopLossPrice {
		if result.Signal == dto.Hold || result.Signal == "" {
			result.Signal = dto.TrailingStop
		}
		triggerReason := ""
		if triggerB {
			triggerReason = fmt.Sprintf("karena harga menembus resistance kunci di %s", secondaryTA.Timeframe)
		} else if triggerA {
			triggerReason = fmt.Sprintf("karena posisi profit signifikan dgn sinyal kuat (%s)", technicalSignal)
		}
		insightText := fmt.Sprintf("SINYAL TRAILING STOP: %s. Rekomendasi naikkan SL ke %.2f untuk %s.", triggerReason, bestProposedSL, reasonForUpdate)
		result.Insight = append(result.Insight, dto.Insight{Text: insightText, Weight: 90})
		result.TrailingStopPrice = bestProposedSL
	}
}

// Helper untuk menganalisis potensi di level Take Profit.
func (s *tradingService) evaluatePotentialAtTakeProfit(mainTA *dto.TradingViewScanner, mainOHLCV []dto.StockOHLCV) (bool, dto.Insight, float64) {
	if mainTA == nil || len(mainOHLCV) == 0 {
		return false, dto.Insight{}, 0
	}
	lastCandle := mainOHLCV[len(mainOHLCV)-1]
	isStrongCandle := s.isStrongBullishCandle(lastCandle)
	avgVolume := s.calculateAverageVolume(mainOHLCV, 20)
	isHighVolume := avgVolume > 0 && float64(lastCandle.Volume) > (avgVolume*1.5)
	isRSIHealthy := mainTA.Value.Oscillators.RSI < 85
	if isStrongCandle && isHighVolume && isRSIHealthy {
		nextTarget := mainTA.Value.Pivots.Classic.R2
		if nextTarget == 0 {
			nextTarget = mainTA.Value.Pivots.Fibonacci.R2
		}
		explanation := dto.Insight{
			Text:   fmt.Sprintf("[POTENSI LANJUTAN] Harga menembus TP dengan candle kuat dan volume tinggi (%.0f vs avg %.0f).", float64(lastCandle.Volume), avgVolume),
			Weight: 70, // High importance for TTP decision
		}
		return true, explanation, nextTarget
	}
	return false, dto.Insight{Text: "Momentum tidak cukup kuat untuk melanjutkan kenaikan secara signifikan.", Weight: 50}, 0
}

// calculateAdvancedScore menghitung skor posisi berdasarkan analisis teknikal komprehensif.
func (s *tradingService) calculateAdvancedScore(pos *model.StockPosition, mainTA, secondaryTA *dto.TradingViewScanner, mainOHLCV []dto.StockOHLCV, marketPrice float64, supports, resistances []dto.Level) (float64, []dto.Insight, string) {
	var totalScore float64
	var insights []dto.Insight

	// 1. Analisis Tren (Bobot: 35%)
	trendScore, trendInsights := s.scoreTrend(mainTA)
	totalScore += trendScore * 0.35
	insights = append(insights, trendInsights...)

	// 2. Analisis Momentum (Bobot: 25%)
	momentumScore, momentumInsights := s.scoreMomentum(mainTA)
	totalScore += momentumScore * 0.25
	insights = append(insights, momentumInsights...)

	// 3. Analisis Kondisi Posisi (Bobot: 20%)
	positionScore, positionInsights := s.scorePositionHealth(pos, marketPrice, supports, resistances)
	totalScore += positionScore * 0.20
	insights = append(insights, positionInsights...)

	// 4. Analisis Price Action & Volume (Bobot: 15%)
	priceActionScore, priceActionInsights := s.scorePriceActionAndVolume(mainOHLCV)
	totalScore += priceActionScore * 0.15
	insights = append(insights, priceActionInsights...)

	// 5. Analisis Multi-Timeframe (Bobot: 10%)
	if secondaryTA != nil {
		multiTimeframeScore, multiTimeframeInsights := s.scoreMultiTimeframe(mainTA, secondaryTA)
		totalScore += multiTimeframeScore * 0.10 // Bobotnya lebih kecil karena bersifat konfirmasi
		insights = append(insights, multiTimeframeInsights...)
	}

	// Normalisasi skor ke rentang 0-100
	if totalScore > 100 {
		totalScore = 100
	} else if totalScore < 0 {
		totalScore = 0
	}

	techSignal := dto.MapTradingViewScreenerRecommend(mainTA.Recommend.Global.Summary)

	return totalScore, insights, techSignal
}

// scoreTrend memberikan skor pada kekuatan dan arah tren.
func (s *tradingService) scoreTrend(ta *dto.TradingViewScanner) (float64, []dto.Insight) {
	score := 0.0
	var insights []dto.Insight

	// Skor dari Rekomendasi Global TradingView (Max 50 poin)
	switch dto.GetTrendText(ta.Recommend.Global.Summary) {
	case dto.SignalStrongBuy:
		score += 50
		insights = append(insights, dto.Insight{Text: "TA Summary menunjukkan Sinyal Beli Kuat.", Weight: 20})
	case dto.SignalBuy:
		score += 35
		insights = append(insights, dto.Insight{Text: "TA Summary menunjukkan Sinyal Beli.", Weight: 20})
	case dto.SignalNeutral:
		score += 15
		insights = append(insights, dto.Insight{Text: "TA Summary menunjukkan kondisi Netral.", Weight: 10})
	case dto.SignalSell:
		score -= 35
		insights = append(insights, dto.Insight{Text: "TA Summary menunjukkan Sinyal Jual.", Weight: 50})
	case dto.SignalStrongSell:
		score -= 50
		insights = append(insights, dto.Insight{Text: "TA Summary menunjukkan Sinyal Jual Kuat.", Weight: 50})
	}

	// Skor dari Moving Averages (Max 50 poin)
	maScore := 0.0
	lastPrice := ta.Value.Prices.Close
	if lastPrice > ta.Value.MovingAverages.EMA10 {
		maScore += 5
	}
	if lastPrice > ta.Value.MovingAverages.EMA20 {
		maScore += 10
	}
	if lastPrice > ta.Value.MovingAverages.EMA50 {
		maScore += 15
	}
	if lastPrice > ta.Value.MovingAverages.EMA100 {
		maScore += 20
	} // Bobot lebih tinggi untuk MA jangka panjang

	if maScore > 0 {
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Harga (%.2f) berada di atas EMA utama, mengindikasikan tren bullish.", lastPrice), Weight: 20})
	}

	// Penalti jika harga di bawah MA penting
	if lastPrice < ta.Value.MovingAverages.EMA200 {
		maScore -= 30 // Penalti besar jika di bawah MA 200
		insights = append(insights, dto.Insight{Text: "Peringatan: Harga berada di bawah EMA200, tren jangka panjang mungkin bearish.", Weight: 50})
	}

	// Batasi skor MA antara -30 dan 50
	if maScore > 50 {
		maScore = 50
	}
	if maScore < -30 {
		maScore = -30
	}

	score += maScore

	return score, insights
}

// Utility untuk memeriksa apakah sebuah candle bullish kuat.
// scoreMomentum memberikan skor pada kekuatan momentum pasar.
func (s *tradingService) scoreMomentum(ta *dto.TradingViewScanner) (float64, []dto.Insight) {
	score := 0.0
	var insights []dto.Insight

	// 1. RSI (Relative Strength Index) - Max 40 poin
	rsi := ta.Value.Oscillators.RSI
	if rsi > 70 {
		score -= 20 // Overbought, potensi pembalikan
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("RSI (%.2f) berada di area overbought (>70), waspadai potensi pullback.", rsi), Weight: 50})
	} else if rsi > 50 {
		score += 40 * ((rsi - 50) / 20) // Skor proporsional di zona bullish
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("RSI (%.2f) menunjukkan momentum bullish yang sehat.", rsi), Weight: 20})
	} else if rsi < 30 {
		score -= 10 // Oversold, bisa jadi sinyal beli, tapi momentum masih lemah
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("RSI (%.2f) berada di area oversold (<30), momentum jual kuat.", rsi), Weight: 50})
	} else {
		score += 20 // Netral cenderung bullish
	}

	// 2. MACD (Moving Average Convergence Divergence) - Max 30 poin
	macdLine := ta.Value.Oscillators.MACD.Macd
	signalLine := ta.Value.Oscillators.MACD.Signal
	divergence := macdLine - signalLine

	macdScore := 0.0
	if divergence > 0 {
		macdScore += 15
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("MACD (%.2f) di atas Signal (%.2f), menandakan momentum positif.", macdLine, signalLine), Weight: 20})
	} else {
		macdScore -= 15
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("MACD (%.2f) di bawah Signal (%.2f), menandakan momentum negatif.", macdLine, signalLine), Weight: 50})
	}

	if macdLine > 0 {
		macdScore += 15
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("MACD (%.2f) di atas garis nol, mengkonfirmasi tren bullish.", macdLine), Weight: 10})
	} else {
		macdScore -= 15
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("MACD (%.2f) di bawah garis nol, mengkonfirmasi tren bearish.", macdLine), Weight: 50})
	}

	// Cek divergensi kuat
	if divergence > (math.Abs(macdLine) * 0.1) {
		macdScore += 10 // Bonus untuk divergensi kuat
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Divergensi positif yang kuat (%.2f) menunjukkan momentum beli meningkat.", divergence), Weight: 30})
	} else if divergence < -(math.Abs(macdLine) * 0.1) {
		macdScore -= 10
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Divergensi negatif yang kuat (%.2f) menunjukkan momentum jual meningkat.", divergence), Weight: 60})
	}

	score += macdScore

	// 3. Stochastic Oscillator - Max 30 poin
	stochK := ta.Value.Oscillators.StochK
	if stochK > 80 {
		score -= 10
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Stochastic (%.2f) berada di area overbought (>80).", stochK), Weight: 40})
	} else if stochK > 20 {
		score += 30
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Stochastic (%.2f) menunjukkan momentum naik.", stochK), Weight: 20})
	} else {
		score -= 5
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Stochastic (%.2f) berada di area oversold (<20).", stochK), Weight: 40})
	}

	return score, insights
}

// scorePositionHealth memberikan skor berdasarkan kesehatan posisi trading saat ini.
func (s *tradingService) scorePositionHealth(pos *model.StockPosition, lastPrice float64, supports, resistances []dto.Level) (float64, []dto.Insight) {
	var insights []dto.Insight

	// --- Skor Kesehatan Posisi (Original) ---
	healthScore := 0.0
	profitPercentage := ((lastPrice - pos.BuyPrice) / pos.BuyPrice) * 100

	if profitPercentage > 0 {
		healthScore += 50 + (profitPercentage * 2)
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Posisi sedang profit %.2f%%.", profitPercentage), Weight: 20})
	} else if profitPercentage < 0 {
		healthScore += 50 + (profitPercentage * 5)
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Posisi sedang merugi %.2f%%.", profitPercentage), Weight: 40})
	} else {
		healthScore += 50
	}

	riskRange := lastPrice - pos.StopLossPrice
	entryToSLRange := pos.BuyPrice - pos.StopLossPrice
	if entryToSLRange > 0 {
		riskRatio := riskRange / entryToSLRange
		if riskRatio < 0.25 {
			healthScore -= 50
			insights = append(insights, dto.Insight{Text: "Sangat dekat dengan Stop Loss (<25%% dari rentang risiko).", Weight: 80})
		} else if riskRatio < 0.5 {
			healthScore -= 25
			insights = append(insights, dto.Insight{Text: "Dekat dengan Stop Loss (<50%% dari rentang risiko).", Weight: 50})
		} else {
			healthScore += 20
		}
	}

	// --- Skor Probabilitas Mencapai TP (Baru) ---
	tpReachScore, tpInsights := s.calculateTPReachScore(pos, resistances)
	insights = append(insights, tpInsights...)

	// --- Skor Penempatan SL (Baru) ---
	slPlacementScore, slInsights := s.calculateSLPlacementScore(pos, supports)
	insights = append(insights, slInsights...)

	// --- Gabungkan Skor ---
	// Bobot: 50% kesehatan, 25% probabilitas TP, 25% penempatan SL
	finalScore := (s.normalizeScore(healthScore) * 0.50) + (tpReachScore * 0.25) + (slPlacementScore * 0.25)

	return s.normalizeScore(finalScore), insights
}

// calculateSLPlacementScore menganalisis kualitas penempatan Stop Loss.
func (s *tradingService) calculateSLPlacementScore(pos *model.StockPosition, supports []dto.Level) (float64, []dto.Insight) {
	score := 50.0 // Skor awal netral
	var insights []dto.Insight

	if len(supports) == 0 {
		return score, insights
	}

	// Cari support terdekat
	minDistance := math.MaxFloat64
	var nearestSupport dto.Level
	for _, sup := range supports {
		if sup.Price < pos.BuyPrice { // Hanya pertimbangkan support di bawah harga beli
			distance := math.Abs(pos.StopLossPrice - sup.Price)
			if distance < minDistance {
				minDistance = distance
				nearestSupport = sup
			}
		}
	}

	if nearestSupport.Price > 0 {
		isSLSafe := pos.StopLossPrice < nearestSupport.Price
		proximityFactor := 1.0 - (minDistance / pos.StopLossPrice) // 0-1, 1 sangat dekat
		strengthFactor := float64(nearestSupport.Touches)          // Jumlah sentuhan

		if isSLSafe {
			// Bonus jika SL di bawah support kuat (penempatan aman)
			bonus := (strengthFactor * proximityFactor) * 20
			score += bonus
			insights = append(insights, dto.Insight{Text: fmt.Sprintf("Penempatan SL baik, berada di bawah support kuat (%.2f, disentuh %d kali).", nearestSupport.Price, nearestSupport.Touches), Weight: 15})
		} else {
			// Penalti jika SL di atas support (rawan tersentuh)
			penalty := (strengthFactor * proximityFactor) * 25
			score -= penalty
			insights = append(insights, dto.Insight{Text: fmt.Sprintf("Peringatan: SL berada di atas support (%.2f, disentuh %d kali), rawan tersentuh.", nearestSupport.Price, nearestSupport.Touches), Weight: 40})
		}
	}

	return s.normalizeScore(score), insights
}

// calculateTPReachScore menganalisis probabilitas harga mencapai target Take Profit.
func (s *tradingService) calculateTPReachScore(pos *model.StockPosition, resistances []dto.Level) (float64, []dto.Insight) {
	var insights []dto.Insight

	// 1. Analisis Risk/Reward Ratio (RRR) - Bobot 40%
	rrrScore := 0.0
	entryToTPRange := pos.TakeProfitPrice - pos.BuyPrice
	entryToSLRange := pos.BuyPrice - pos.StopLossPrice

	if entryToSLRange > 0 && entryToTPRange > 0 {
		rrr := entryToTPRange / entryToSLRange
		if rrr >= 2.0 {
			rrrScore = 100
			insights = append(insights, dto.Insight{Text: fmt.Sprintf("Risk/Reward Ratio sangat baik (%.1f:1).", rrr), Weight: 15})
		} else if rrr >= 1.5 {
			rrrScore = 70
			insights = append(insights, dto.Insight{Text: fmt.Sprintf("Risk/Reward Ratio baik (%.1f:1).", rrr), Weight: 10})
		} else {
			rrrScore = 40
			insights = append(insights, dto.Insight{Text: fmt.Sprintf("Risk/Reward Ratio kurang ideal (%.1f:1).", rrr), Weight: 25})
		}
	} else {
		rrrScore = 20 // Penalti jika RRR tidak valid
	}

	// 2. Analisis Posisi TP vs. Resistance - Bobot 60%
	resistanceScore := 50.0 // Skor awal netral
	if len(resistances) > 0 {
		// Cari resistance terdekat
		minDistance := math.MaxFloat64
		var nearestResistance dto.Level
		for _, r := range resistances {
			if r.Price > pos.BuyPrice { // Hanya pertimbangkan resistance di atas harga beli
				distance := math.Abs(r.Price - pos.TakeProfitPrice)
				if distance < minDistance {
					minDistance = distance
					nearestResistance = r
				}
			}
		}

		if nearestResistance.Price > 0 {
			isTPAmbitions := pos.TakeProfitPrice > nearestResistance.Price
			proximityFactor := 1.0 - (minDistance / pos.TakeProfitPrice) // 0-1, 1 sangat dekat
			strengthFactor := float64(nearestResistance.Touches)         // Jumlah sentuhan

			if isTPAmbitions {
				// Penalti jika TP di atas resistance kuat
				penalty := (strengthFactor * proximityFactor) * 20 // Penalti bisa sampai 100+
				resistanceScore -= penalty
				insights = append(insights, dto.Insight{Text: fmt.Sprintf("Target TP ambisius, berada di atas resistance kuat (%.2f, disentuh %d kali).", nearestResistance.Price, nearestResistance.Touches), Weight: 40})
			} else {
				// Bonus jika TP di bawah resistance kuat
				bonus := (strengthFactor * proximityFactor) * 15 // Bonus bisa sampai 75+
				resistanceScore += bonus
				insights = append(insights, dto.Insight{Text: fmt.Sprintf("Target TP realistis, di bawah resistance kuat (%.2f, disentuh %d kali).", nearestResistance.Price, nearestResistance.Touches), Weight: 15})
			}
		}
	}

	// Gabungkan skor RRR dan Resistance
	finalScore := (s.normalizeScore(rrrScore) * 0.4) + (s.normalizeScore(resistanceScore) * 0.6)

	return s.normalizeScore(finalScore), insights
}

// scorePriceActionAndVolume memberikan skor berdasarkan aksi harga dan volume.
func (s *tradingService) scorePriceActionAndVolume(ohlcv []dto.StockOHLCV) (float64, []dto.Insight) {
	if len(ohlcv) < 2 {
		return 50, []dto.Insight{{Text: "Data OHLCV tidak cukup untuk analisis price action.", Weight: 10}}
	}

	score := 50.0 // Mulai dari netral
	var insights []dto.Insight
	lastCandle := ohlcv[len(ohlcv)-1]
	prevCandle := ohlcv[len(ohlcv)-2]
	avgVolume := s.calculateAverageVolume(ohlcv, 20)

	// Analisis Candle Terakhir
	if s.isStrongBullishCandle(lastCandle) {
		score += 30
		insights = append(insights, dto.Insight{Text: "Candle terakhir menunjukkan kekuatan beli yang solid (bullish marubozu/strong).", Weight: 30})
	} else if s.isBearishEngulfing(lastCandle, prevCandle) {
		score -= 40
		insights = append(insights, dto.Insight{Text: "Terbentuk pola Bearish Engulfing, sinyal pembalikan yang kuat.", Weight: 70})
	} else if lastCandle.Close < lastCandle.Open {
		score -= 15
		insights = append(insights, dto.Insight{Text: "Candle terakhir ditutup merah, menunjukkan tekanan jual.", Weight: 40})
	}

	// Analisis Volume
	if avgVolume > 0 {
		volumeRatio := float64(lastCandle.Volume) / avgVolume
		if volumeRatio > 2.0 {
			if lastCandle.Close > lastCandle.Open {
				score += 20 // Konfirmasi volume tinggi pada candle hijau
				insights = append(insights, dto.Insight{Text: fmt.Sprintf("Volume sangat tinggi (%.1fx avg) mengkonfirmasi minat beli.", volumeRatio), Weight: 30})
			} else {
				score -= 30 // Peringatan volume tinggi pada candle merah
				insights = append(insights, dto.Insight{Text: fmt.Sprintf("Volume distribusi sangat tinggi (%.1fx avg) pada candle merah.", volumeRatio), Weight: 70})
			}
		} else if volumeRatio < 0.7 {
			score -= 10
			insights = append(insights, dto.Insight{Text: "Volume rendah, kurangnya konfirmasi dari pasar.", Weight: 40})
		}
	}

	return s.normalizeScore(score), insights
}

// scoreMultiTimeframe memberikan skor berdasarkan keselarasan sinyal di berbagai timeframe.
func (s *tradingService) scoreMultiTimeframe(mainTA, secondaryTA *dto.TradingViewScanner) (float64, []dto.Insight) {
	score := 50.0 // Netral
	var insights []dto.Insight

	mainSignal := dto.MapTradingViewScreenerRecommend(mainTA.Recommend.Global.Summary)
	secondarySignal := dto.MapTradingViewScreenerRecommend(secondaryTA.Recommend.Global.Summary)

	// Kondisi Bullish yang dikonfirmasi
	isMainBuy := mainSignal == dto.SignalStrongBuy || mainSignal == dto.SignalBuy
	isSecondarySupport := secondaryTA == nil || secondarySignal == dto.SignalStrongBuy || secondarySignal == dto.SignalBuy || secondarySignal == dto.SignalNeutral

	// Kondisi Bearish yang diperkuat
	isMainSell := mainSignal == dto.SignalStrongSell || mainSignal == dto.SignalSell
	isSecondaryCorroborate := secondaryTA == nil || secondarySignal == dto.SignalStrongSell || secondarySignal == dto.SignalSell || secondarySignal == dto.SignalNeutral

	// Konflik
	isSecondarySell := secondaryTA != nil && (secondarySignal == dto.SignalStrongSell || secondarySignal == dto.SignalSell)
	isSecondaryBuy := secondaryTA != nil && (secondarySignal == dto.SignalStrongBuy || secondarySignal == dto.SignalBuy)

	if isMainBuy && isSecondarySupport {
		score = 100
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Sinyal bullish dikonfirmasi oleh timeframe %s (%s).", secondaryTA.Timeframe, secondarySignal), Weight: 20})
	} else if isMainSell && isSecondaryCorroborate {
		score = 0
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Sinyal bearish diperkuat oleh timeframe %s (%s).", secondaryTA.Timeframe, secondarySignal), Weight: 70})
	} else if isMainBuy && isSecondarySell {
		score = 20 // Konflik besar
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Peringatan: Konflik sinyal antara timeframe utama (Buy) dan timeframe %s (%s).", secondaryTA.Timeframe, secondarySignal), Weight: 60})
	} else if isMainSell && isSecondaryBuy {
		score = 20 // Konflik besar
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Peringatan: Konflik sinyal antara timeframe utama (Sell) dan timeframe %s (%s).", secondaryTA.Timeframe, secondarySignal), Weight: 60})
	} else {
		insights = append(insights, dto.Insight{Text: fmt.Sprintf("Sinyal tidak selaras: %s di %s vs %s di %s.", mainSignal, mainTA.Timeframe, secondarySignal, secondaryTA.Timeframe), Weight: 40})
	}

	return score, insights
}

func (s *tradingService) isStrongBullishCandle(candle dto.StockOHLCV) bool {
	// Candle harus hijau
	if candle.Close <= candle.Open {
		return false
	}

	bodySize := candle.Close - candle.Open
	totalRange := candle.High - candle.Low

	// Menghindari pembagian dengan nol
	if totalRange == 0 {
		return true // Jika tidak ada range, anggap kuat jika hijau
	}

	// Badan candle harus lebih dari 70% dari total range (artinya ekornya pendek)
	return (bodySize / totalRange) > 0.7
}

func (s *tradingService) evaluateTrailingTakeProfit(
	result *dto.PositionAnalysis,
	pos *model.StockPosition,
	mainTA *dto.TradingViewScanner,
	mainOHLCV []dto.StockOHLCV,
) {
	canGoHigher, explanation, nextTarget := s.evaluatePotentialAtTakeProfit(mainTA, mainOHLCV)
	// --- Logika #1: Aktivasi Mode Trailing Take Profit (TTP) ---
	if result.LastPrice >= pos.TakeProfitPrice && pos.TrailingProfitPrice == 0 {
		if canGoHigher && nextTarget > result.TakeProfitPrice {
			// Aktifkan mode TTP
			result.Signal = dto.TrailingProfit
			result.TrailingProfitPrice = result.TakeProfitPrice // Jaring pengaman awal di TP Price
			result.HighestPriceSinceTTP = result.LastPrice
			result.Insight = append(result.Insight, dto.Insight{Text: fmt.Sprintf("MODE TTP AKTIF: Target profit awal (%.2f) tercapai. Jaring pengaman profit sekarang di %.2f.", pos.TakeProfitPrice, result.TrailingProfitPrice), Weight: 90})
			if explanation.Text != "" {
				result.Insight = append(result.Insight, explanation)
			}
			return // Keluar setelah aktivasi
		} else {
			// Tidak ada potensi, TP biasa
			result.Signal = dto.TakeProfit
			result.Status = dto.Safe
			result.Insight = append(result.Insight, dto.Insight{Text: "SINYAL TAKE PROFIT: Harga telah mencapai target profit awal.", Weight: 90})
			return
		}
	}

	// --- Logika #2: Manajemen Mode TTP yang Sedang Aktif ---
	if pos.TrailingProfitPrice > 0 {
		result.TrailingProfitPrice = pos.TrailingProfitPrice // Bawa nilai TTP lama
		newHighestPrice := math.Max(result.LastPrice, pos.HighestPriceSinceTTP)
		result.HighestPriceSinceTTP = newHighestPrice

		// Tentukan trigger price baru (misal: 3% di bawah harga tertinggi baru)
		newTriggerPrice := newHighestPrice * 0.97

		// Pastikan trigger price tidak pernah turun
		if newTriggerPrice > result.TrailingProfitPrice {
			result.TrailingProfitPrice = newTriggerPrice
		}

		// Kondisi 1: Cek potensi kenaikan lebih lanjut
		if !canGoHigher {
			result.Signal = dto.TakeProfit
			result.Status = dto.Safe
			insightText := "SINYAL TAKE PROFIT (Trailing): Potensi kenaikan lanjutan dinilai rendah."
			if explanation.Text != "" {
				insightText = fmt.Sprintf("SINYAL TAKE PROFIT (Trailing): %s", explanation.Text)
			}
			result.Insight = append(result.Insight, dto.Insight{Text: insightText, Weight: 90})
			return
		}

		// Kondisi 2: Harga menyentuh trigger price
		if result.LastPrice <= result.TrailingProfitPrice {
			result.Signal = dto.TakeProfit
			result.Status = dto.Safe
			result.Insight = append(result.Insight, dto.Insight{Text: fmt.Sprintf("SINYAL TAKE PROFIT (Trailing): Harga (%.2f) telah menyentuh trigger price (%.2f) dari puncaknya (%.2f).", result.LastPrice, result.TrailingProfitPrice, newHighestPrice), Weight: 90})
			return
		}

		// Kondisi 3: Terbentuk pola reversal kuat (misal: Bearish Engulfing)
		if len(mainOHLCV) >= 2 && s.isBearishEngulfing(mainOHLCV[len(mainOHLCV)-1], mainOHLCV[len(mainOHLCV)-2]) {
			result.Signal = dto.TakeProfit
			result.Status = dto.Safe
			result.Insight = append(result.Insight, dto.Insight{Text: "SINYAL TAKE PROFIT (Trailing): Terbentuk pola Bearish Engulfing, mengindikasikan pembalikan momentum.", Weight: 90})
			return
		}

		// Kondisi 4: Sinyal teknikal umum melemah signifikan
		techSignal := dto.MapTradingViewScreenerRecommend(mainTA.Recommend.Global.Summary)
		if techSignal == dto.SignalSell || techSignal == dto.SignalStrongSell {
			result.Signal = dto.TakeProfit
			result.Status = dto.Safe
			result.Insight = append(result.Insight, dto.Insight{Text: "SINYAL TP (Trailing): Sinyal teknikal umum melemah.", Weight: 90})
			return
		}

		// Jika tidak ada sinyal exit, tetap dalam mode TTP
		result.Signal = dto.TrailingProfit
		insightTTPStatus := dto.Insight{Text: fmt.Sprintf("MODE TTP AKTIF: Profit mengambang. Puncak tertinggi: %.2f, Trigger SL: %.2f.", newHighestPrice, result.TrailingProfitPrice), Weight: 30}
		result.Insight = append(result.Insight, insightTTPStatus)
	}
}

func (s *tradingService) normalizeScore(score float64) float64 {
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
