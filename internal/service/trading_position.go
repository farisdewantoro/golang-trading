package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"sort"
	"strings"
)

func (s *tradingService) EvaluatePositionMonitoring(
	ctx context.Context,
	stockPosition *model.StockPosition,
	analyses []model.StockAnalysis,
) (*dto.PositionAnalysis, error) {

	// --- Inisialisasi dan Pengambilan Data ---
	result := &dto.PositionAnalysis{
		Ticker:          stockPosition.StockCode,
		EntryPrice:      stockPosition.BuyPrice,
		TakeProfitPrice: stockPosition.TakeProfitPrice,
		StopLossPrice:   stockPosition.StopLossPrice,
		Insight:         []string{},
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
		result.Insight = append(result.Insight, fmt.Sprintf("SINYAL CUT LOSS: Harga (%.2f) telah menyentuh Stop Loss (%.2f).", result.LastPrice, result.StopLossPrice))
	}

	// --- Dapatkan Evaluasi Baseline ---
	score, techSignal, err := s.evaluateSignal(ctx, timeframes, analyses)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get baseline evaluation", logger.ErrorField(err))
		return nil, err
	}
	result.Score = score
	result.TechnicalSignal = techSignal

	// Prioritas #2: Kelola Trailing Take Profit (TTP)
	// Fungsi ini akan mengubah nilai TTP di dalam 'result' dan bisa menyarankan sinyal exit
	s.evaluateTrailingTakeProfit(result, stockPosition, mainData.MainTA, mainData.SecondaryTA, mainData.MainOHLCV)
	if result.Signal == dto.TakeProfit {
		finalSignal = dto.TakeProfit
	}

	// Analisis kondisi pasar
	s.analyzeMultiTimeframeConditions(result, mainData.MainTA, mainData.SecondaryTA)
	s.analyzePriceActionAndVolume(result, mainData.MainOHLCV, mainData.MainTA)

	// Evaluasi Trailing Stop Loss
	s.evaluateTrailingStop(result, mainData.MainTA, mainData.SecondaryTA, techSignal, mainData.MainOHLCV)

	// Tentukan status akhir
	s.determineFinalStatus(result, mainData.MainTA, techSignal)

	// --- Bagian Akhir: Tentukan Sinyal Final ---
	if finalSignal != "" {
		result.Signal = finalSignal
	} else if result.Signal == "" {
		result.Signal = dto.Hold
	}

	// Isi sisa informasi
	result.IndicatorSummary = s.CreateIndicatorSummary(mainData.MainTA, mainData.MainOHLCV)

	return result, nil
}

func (s *tradingService) checkMomentumAndResistance(result *dto.PositionAnalysis, ta *dto.TradingViewScanner) {
	if ta.Value.Oscillators.RSI > 70 {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Momentum: RSI (%.2f) memasuki area overbought.", ta.Value.Oscillators.RSI))
	}
	if ta.Value.Oscillators.MACD.Macd < ta.Value.Oscillators.MACD.Signal {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, "[WASPADA] Momentum: MACD cross ke bawah, indikasi pelemahan bullish.")
	}
	resistanceR1 := ta.Value.Pivots.Classic.R1
	if resistanceR1 > 0 && result.LastPrice > (resistanceR1*0.99) && result.LastPrice <= resistanceR1 {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Struktur Pasar: Harga mendekati resistance R1 di %.2f.", resistanceR1))
	}
}

func (s *tradingService) analyzePriceActionAndVolume(result *dto.PositionAnalysis, ohlcv []dto.StockOHLCV, ta *dto.TradingViewScanner) {
	if len(ohlcv) < 2 {
		return
	}
	lastCandle := ohlcv[len(ohlcv)-1]
	prevCandle := ohlcv[len(ohlcv)-2]
	if s.isBearishEngulfing(lastCandle, prevCandle) {
		resistanceLevel := ta.Value.Pivots.Classic.R1
		if resistanceLevel > 0 && lastCandle.High >= (resistanceLevel*0.99) {
			result.Status = dto.Dangerous
			result.Insight = append(result.Insight, fmt.Sprintf("[BAHAYA] Price Action: Pola Bearish Engulfing terbentuk dekat resistance R1 (%.2f).", resistanceLevel))
		} else {
			result.Status = dto.Warning
			result.Insight = append(result.Insight, "[WASPADA] Price Action: Terbentuk pola Bearish Engulfing.")
		}
	}
	avgVolume := s.calculateAverageVolume(ohlcv, 20)
	if avgVolume == 0 {
		return
	}
	isRedCandle := lastCandle.Close < lastCandle.Open
	isHighVolume := float64(lastCandle.Volume) > (avgVolume * 2.0)
	if isRedCandle && isHighVolume {
		result.Status = dto.Dangerous
		result.Insight = append(result.Insight, fmt.Sprintf("[BAHAYA] Volume: Tekanan jual masif (Volume %.0f vs rata-rata %.0f).", float64(lastCandle.Volume), avgVolume))
	}
}

func (s *tradingService) determineFinalStatus(result *dto.PositionAnalysis, ta *dto.TradingViewScanner, technicalSignal string) {
	riskRange := result.LastPrice - result.StopLossPrice
	entryToSLRange := result.EntryPrice - result.StopLossPrice
	if entryToSLRange > 0 && (riskRange/entryToSLRange) < 0.25 {
		result.Status = dto.Dangerous
		result.Insight = append(result.Insight, fmt.Sprintf("[Kondisi Bahaya]: Jarak ke Stop Loss < 25%% (Harga: %.2f, SL: %.2f).", result.LastPrice, result.StopLossPrice))
		return
	}
	if result.Status == dto.Dangerous || result.Status == dto.Warning {
		return
	}
	switch technicalSignal {
	case dto.SignalStrongBuy, dto.SignalBuy:
		result.Status = dto.Safe
	case dto.SignalNeutral:
		if ta.Value.Oscillators.ADX.Value < 20 {
			result.Status = dto.Warning
			result.Insight = append(result.Insight, "[Kondisi Waspada]: Pasar ranging (ADX < 20), kekuatan tren lemah.")
		} else {
			result.Status = dto.Safe
		}
	case dto.SignalSell, dto.SignalStrongSell:
		result.Status = dto.Warning
		result.Insight = append(result.Insight, "[Kondisi Waspada]: Evaluasi umum menunjukkan kelemahan.")
	default:
		result.Status = dto.Warning
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

func (s *tradingService) analyzeMultiTimeframeConditions(result *dto.PositionAnalysis, mainTA, secondaryTA *dto.TradingViewScanner) {
	if secondaryTA == nil {
		s.checkMomentumAndResistance(result, mainTA)
		result.Insight = append(result.Insight, fmt.Sprintf("Analisis mendalam menggunakan data timeframe %s tanpa secondary timeframe.", mainTA.Timeframe))
		return
	}
	isMainBullish := mainTA.Recommend.Global.Summary == dto.TradingViewSignalBuy || mainTA.Recommend.Global.Summary == dto.TradingViewSignalStrongBuy
	isSecondaryBullish := secondaryTA.Recommend.Global.Summary == dto.TradingViewSignalBuy || secondaryTA.Recommend.Global.Summary == dto.TradingViewSignalStrongBuy
	secondaryRSI := secondaryTA.Value.Oscillators.RSI
	if isMainBullish && secondaryRSI > 50 && secondaryRSI < 70 {
		result.Insight = append(result.Insight, fmt.Sprintf("[AMAN] Konfirmasi MTA: Tren utama %s dikonfirmasi oleh momentum sehat di %s.", mainTA.Timeframe, secondaryTA.Timeframe))
	} else if isMainBullish {
		if isSecondaryBullish {
			result.Insight = append(result.Insight, fmt.Sprintf("[AMAN] Tren pada %s dan %s bullish.", mainTA.Timeframe, secondaryTA.Timeframe))
		} else {
			result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Tren utama %s bullish tanpa konfirmasi dari %s.", mainTA.Timeframe, secondaryTA.Timeframe))
		}
	}
	if isMainBullish && secondaryRSI > 75 {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Divergensi MTA: Tren %s kuat, namun %s jenuh beli (RSI: %.2f).", mainTA.Timeframe, secondaryTA.Timeframe, secondaryRSI))
	}
	if isMainBullish && secondaryTA.Value.Oscillators.MACD.Macd < secondaryTA.Value.Oscillators.MACD.Signal {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Pelemahan Momentum: Tren utama %s naik, namun MACD di %s cross ke bawah.", mainTA.Timeframe, secondaryTA.Timeframe))
	}
	if mainTA.Value.Oscillators.RSI > 75 {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Kondisi Jenuh Beli %s: Timeframe utama sangat jenuh beli (RSI: %.2f).", mainTA.Timeframe, mainTA.Value.Oscillators.RSI))
	}
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
		insight := fmt.Sprintf("SINYAL TRAILING STOP: %s. Rekomendasi naikkan SL ke %.2f untuk %s.", triggerReason, bestProposedSL, reasonForUpdate)
		result.Insight = append(result.Insight, insight)
		result.TrailingStopPrice = bestProposedSL
	}
}

// Helper untuk menganalisis potensi di level Take Profit.
func (s *tradingService) evaluatePotentialAtTakeProfit(mainTA *dto.TradingViewScanner, mainOHLCV []dto.StockOHLCV) (bool, string, float64) {
	if mainTA == nil || len(mainOHLCV) == 0 {
		return false, "", 0
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
		explanation := fmt.Sprintf("[POTENSI LANJUTAN] Harga menembus TP dengan candle kuat dan volume tinggi (%.0f vs avg %.0f).", float64(lastCandle.Volume), avgVolume)
		return true, explanation, nextTarget
	}
	return false, "Momentum tidak cukup kuat untuk melanjutkan kenaikan secara signifikan.", 0
}

// Utility untuk memeriksa apakah sebuah candle bullish kuat.
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

	// Badan candle harus lebih dari 65% dari total range (artinya ekornya pendek)
	return (bodySize / totalRange) > 0.65
}

func (s *tradingService) evaluateTrailingTakeProfit(
	result *dto.PositionAnalysis, // Target DTO yang akan diisi dengan state baru.
	pos *model.StockPosition, // State LAMA dari database (hanya untuk dibaca).
	mainTA *dto.TradingViewScanner,
	secondaryTA *dto.TradingViewScanner,
	mainOHLCV []dto.StockOHLCV,
) {
	// Cek apakah TTP sudah aktif dengan membaca state LAMA dari database.
	isTTPActive := pos.TrailingProfitPrice > 0

	// --- FASE 1: Logika Aktivasi TTP ---
	if !isTTPActive {
		isBeyondOriginalTP := result.LastPrice >= pos.TakeProfitPrice
		hasPotential, explanation, _ := s.evaluatePotentialAtTakeProfit(mainTA, mainOHLCV)

		if explanation != "" {
			result.Insight = append(result.Insight, explanation)
		}

		if isBeyondOriginalTP && !hasPotential {
			result.Signal = dto.TakeProfit
			return
		}

		if isBeyondOriginalTP && hasPotential {
			// --- AKTIVASI TTP ---
			s.log.Info(fmt.Sprintf("TTP Activation Triggered for %s", pos.StockCode))

			// 1. Harga puncak awal adalah harga saat ini.
			initialHighestPrice := result.LastPrice
			result.HighestPriceSinceTTP = initialHighestPrice

			// 2. Hitung kandidat trigger TTP (turun 3% dari puncak).
			candidateTriggerPrice := initialHighestPrice * (1.0 - 0.03)

			// 3. ATURAN EMAS: Trigger TTP tidak boleh lebih rendah dari TP awal.
			//    Pilih mana yang lebih tinggi antara kandidat trigger dan TP awal.
			//    Ini memastikan kita tidak pernah keluar di bawah target profit awal kita.
			if candidateTriggerPrice > pos.TakeProfitPrice {
				result.TrailingProfitPrice = candidateTriggerPrice
			} else {
				result.TrailingProfitPrice = pos.TakeProfitPrice
			}

			result.Signal = dto.TrailingProfit
			result.Insight = append(result.Insight, fmt.Sprintf("MODE TTP AKTIF: Target profit awal (%.2f) tercapai dengan momentum kuat. Jaring pengaman profit sekarang di %.2f.", pos.TakeProfitPrice, result.TrailingProfitPrice))
		}
		return
	}

	// Inisialisasi nilai BARU dengan nilai LAMA sebagai dasar.
	newHighestPrice := pos.HighestPriceSinceTTP
	// Gunakan trigger lama sebagai dasar sementara
	newTriggerPrice := pos.TrailingProfitPrice

	// Perbarui harga puncak dan harga trigger jika rekor baru tercapai.
	if result.LastPrice > newHighestPrice {
		newHighestPrice = result.LastPrice

		// Hitung ulang kandidat trigger dinamis.
		candidateTriggerPrice := newHighestPrice * (1.0 - 0.03)

		// ATURAN EMAS: Trigger baru tidak boleh lebih rendah dari trigger LAMA.
		// Ini memastikan jaring pengaman tidak pernah turun.
		if candidateTriggerPrice > pos.TrailingProfitPrice {
			newTriggerPrice = candidateTriggerPrice
		}
		// Jika kandidat baru lebih rendah (misal, karena koreksi kecil),
		// kita tetap menggunakan trigger lama (pos.TrailingProfitPrice).
		// Jadi, 'newTriggerPrice' akan sama dengan 'pos.TrailingProfitPrice'.
	}

	// Tulis nilai BARU yang sudah dihitung ke DTO 'result' agar bisa disimpan nanti.
	result.HighestPriceSinceTTP = newHighestPrice
	result.TrailingProfitPrice = newTriggerPrice

	// Aturan Eksekusi 1: Harga menyentuh trigger price BARU.
	if result.LastPrice < newTriggerPrice {
		result.Signal = dto.TakeProfit
		result.Status = dto.Safe
		result.Insight = append(result.Insight, fmt.Sprintf("SINYAL TAKE PROFIT (Trailing): Harga (%.2f) telah menyentuh trigger price (%.2f) dari puncaknya (%.2f).", result.LastPrice, newTriggerPrice, newHighestPrice))
		return
	}

	// Aturan Eksekusi 2: Muncul pola candlestick pembalikan yang kuat.
	if len(mainOHLCV) >= 2 && s.isBearishEngulfing(mainOHLCV[len(mainOHLCV)-1], mainOHLCV[len(mainOHLCV)-2]) {
		result.Signal = dto.TakeProfit
		result.Status = dto.Safe
		result.Insight = append(result.Insight, "SINYAL TAKE PROFIT (Trailing): Terbentuk pola Bearish Engulfing, mengindikasikan pembalikan momentum.")
		return
	}

	// Aturan Eksekusi 3: Cross di Bawah MA Jangka Pendek
	ema20 := mainTA.Value.MovingAverages.EMA20
	if ema20 > 0 && result.LastPrice < ema20 {
		result.Signal = dto.TakeProfit
		result.Status = dto.Safe
		result.Insight = append(result.Insight, fmt.Sprintf("SINYAL TP (Trailing): Harga cross ke bawah EMA20 (%.2f).", ema20))
		return
	}

	// Aturan Eksekusi 4: Sinyal Teknikal Melemah
	isWeakSignal := (mainTA.Recommend.Global.Summary != dto.TradingViewSignalBuy && mainTA.Recommend.Global.Summary != dto.TradingViewSignalStrongBuy) ||
		(secondaryTA != nil && (secondaryTA.Recommend.Global.Summary != dto.TradingViewSignalBuy && secondaryTA.Recommend.Global.Summary != dto.TradingViewSignalStrongBuy))
	if isWeakSignal {
		result.Signal = dto.TakeProfit
		result.Status = dto.Safe
		result.Insight = append(result.Insight, "SINYAL TP (Trailing): Sinyal teknikal umum melemah.")
		return
	}

	// Jika tidak ada sinyal eksekusi, maka set sinyal ke 'TrailingProfit'
	// dan berikan insight status TTP saat ini.
	result.Signal = dto.TrailingProfit // Menggunakan sinyal yang lebih deskriptif daripada 'Hold'.
	insightTTPStatus := fmt.Sprintf("STATUS TTP: Mode aktif. Harga trigger keluar saat ini: %.2f (berdasarkan puncak %.2f).", newTriggerPrice, newHighestPrice)

	// Hapus insight status TTP yang mungkin ada dari iterasi sebelumnya agar tidak menumpuk.
	var newInsights []string
	for _, insight := range result.Insight {
		if !strings.Contains(insight, "STATUS TTP:") && !strings.Contains(insight, "MODE TTP AKTIF") {
			newInsights = append(newInsights, insight)
		}
	}
	result.Insight = newInsights
	result.Insight = append(result.Insight, insightTTPStatus)
}
