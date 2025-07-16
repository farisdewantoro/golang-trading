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
)

func (s *tradingService) EvaluatePositionMonitoring(
	ctx context.Context,
	stockPosition *model.StockPosition,
	analyses []model.StockAnalysis,
) (*dto.PositionAnalysis, error) {

	result := &dto.PositionAnalysis{
		Ticker:          stockPosition.StockCode,
		EntryPrice:      stockPosition.BuyPrice,
		TakeProfitPrice: stockPosition.TakeProfitPrice,
		StopLossPrice:   stockPosition.StopLossPrice,
		Insight:         []string{},
	}

	timeframes, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		return nil, err
	}
	mainData, err := s.findMainAnalysisData(analyses, timeframes)
	if err != nil {
		return nil, err
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

	// --- Prioritas #1: Hard Exits ---
	if result.LastPrice <= result.StopLossPrice {
		result.Status = dto.Dangerous
		result.Signal = dto.CutLoss
		result.Insight = append(result.Insight, fmt.Sprintf("SINYAL CUT LOSS: Harga (%.2f) telah menyentuh Stop Loss (%.2f).", result.LastPrice, result.StopLossPrice))
		// return result, nil
	}

	if result.LastPrice >= result.TakeProfitPrice {
		// Panggil helper untuk menganalisis apakah ada potensi kenaikan lebih lanjut
		hasPotential, explanation := s.evaluatePotentialAtTakeProfit(mainData.MainTA, mainData.MainOHLCV)

		if hasPotential {
			// Jika ada potensi, JANGAN keluar. Beri sinyal untuk Trailing Stop.
			result.Status = dto.Safe
			result.Signal = dto.TrailingStop
			result.Insight = append(result.Insight, fmt.Sprintf("LEVEL TAKE PROFIT TERCAPAI (%.2f), NAMUN...", result.TakeProfitPrice))
			result.Insight = append(result.Insight, explanation)
			result.Insight = append(result.Insight, "AKSI: Pertimbangkan untuk take profit sebagian & naikkan Stop Loss secara agresif untuk mengikuti sisa tren.")
			// Lanjutkan ke sisa analisis untuk penentuan status akhir
		} else {
			// Jika tidak ada potensi kuat, keluar sesuai rencana.
			result.Status = dto.Safe
			result.Signal = dto.TakeProfit
			result.Insight = append(result.Insight, fmt.Sprintf("SINYAL TAKE PROFIT: Harga (%.2f) telah mencapai Target Profit (%.2f) tanpa momentum lanjutan yang kuat.", result.LastPrice, result.TakeProfitPrice))
			// return result, nil
		}
	}

	// --- Dapatkan Evaluasi Baseline ---
	score, techSignal, err := s.evaluateSignal(ctx, timeframes, analyses)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get baseline evaluation", logger.ErrorField(err))
		return nil, err
	}
	result.Score = score
	result.TechnicalSignal = techSignal

	if mainData.MainTA == nil {
		return result, fmt.Errorf("no main technical analysis data found")
	}

	if len(mainData.MainOHLCV) == 0 {
		return result, fmt.Errorf("no main OHLCV data found")
	}

	// --- Logika Analisis Mendalam ---
	result.Insight = append(result.Insight, fmt.Sprintf("Analisis mendalam menggunakan data timeframe %s.", mainData.MainTimeframe))
	// Analisis dari indikator (RSI, MACD, Pivot)
	s.analyzeMultiTimeframeConditions(result, mainData.MainTA, mainData.SecondaryTA)
	// Analisis dari price action & volume menggunakan OHLCV dari timeframe utama
	s.analyzePriceActionAndVolume(result, mainData.MainOHLCV, mainData.MainTA)

	// Tentukan sinyal aksi (Trailing Stop/Hold)
	s.evaluateTrailingStop(result, mainData.MainTA, mainData.SecondaryTA, techSignal, mainData.MainOHLCV)
	// Tentukan status akhir berdasarkan semua data
	s.determineFinalStatus(result, mainData.MainTA, techSignal)

	// Pastikan sinyal terisi (default ke Hold)
	if result.Signal == "" {
		result.Signal = dto.Hold
	}

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
	var totalVolume int64
	startIndex := len(ohlcv) - period
	for i := startIndex; i < len(ohlcv); i++ {
		totalVolume += ohlcv[i].Volume
	}
	return float64(totalVolume) / float64(period)
}

func (s *tradingService) findMainAnalysisData(analyses []model.StockAnalysis, timeframes []dto.DataTimeframe) (*dto.MainAnalysisData, error) {

	var (
		result dto.MainAnalysisData
	)
	sort.Slice(timeframes, func(i, j int) bool {
		return timeframes[i].Weight > timeframes[j].Weight
	})

	if len(timeframes) < 2 {
		return nil, fmt.Errorf("not enough timeframes")
	}

	mainTF := timeframes[0]
	secondaryTF := timeframes[1]

	for _, analysis := range analyses {
		var (
			ta     dto.TradingViewScanner
			ohlcvs []dto.StockOHLCV
		)
		if err := json.Unmarshal([]byte(analysis.OHLCV), &ohlcvs); err != nil {
			return nil, err
		}
		// Ekstrak data Indikator
		if err := json.Unmarshal([]byte(analysis.TechnicalData), &ta); err != nil {
			return nil, err
		}
		if analysis.Timeframe == mainTF.Interval {
			result.MainTA = &ta
			result.MainOHLCV = ohlcvs
			result.MainTimeframe = analysis.Timeframe
		}

		if analysis.Timeframe == secondaryTF.Interval {
			result.SecondaryTA = &ta
			result.SecondaryOHLCV = ohlcvs
			result.SecondaryTimeframe = analysis.Timeframe
		}
	}
	// Jika loop selesai tanpa menemukan apa pun
	return &result, nil
}

func (s *tradingService) analyzeMultiTimeframeConditions(result *dto.PositionAnalysis, mainTA, secondaryTA *dto.TradingViewScanner) {
	// Jika data 4H tidak ada, jalankan logika lama pada data 1D
	if secondaryTA == nil {
		s.log.WarnContext(context.Background(), fmt.Sprintf("Data teknikal secondary tidak tersedia, analisis hanya berdasarkan %s.", mainTA.Timeframe))
		s.checkMomentumAndResistance(result, mainTA) // Panggil versi lama
		return
	}

	mainRSI := mainTA.Value.Oscillators.RSI
	secondaryRSI := secondaryTA.Value.Oscillators.RSI
	isMainBullish := mainTA.Value.Global.Summary > 0.1 // Strong/Buy

	// Skenario 1: Konfirmasi Kekuatan
	// 1D bullish dan 4H juga bullish (RSI sehat, tidak overbought)
	if isMainBullish && secondaryRSI > 50 && secondaryRSI < 70 {
		result.Insight = append(result.Insight, fmt.Sprintf("[AMAN] Konfirmasi MTA: Tren utama %s (Bullish) dikonfirmasi oleh momentum kuat di timeframe %s.", mainTA.Timeframe, secondaryTA.Timeframe))
	}

	// Skenario 2: Divergensi / Peringatan Kelelahan
	// 1D masih bullish, tapi 4H sudah sangat overbought. Ini adalah sinyal `Warning` klasik.
	if isMainBullish && secondaryRSI > 75 {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Divergensi MTA: Tren %s kuat, namun timeframe %s menunjukkan kondisi sangat jenuh beli (RSI: %.2f). Waspada potensi pullback/koreksi jangka pendek.", mainTA.Timeframe, secondaryTA.Timeframe, secondaryRSI))
	}

	// Skenario 3: Peringatan Momentum Lemah di Timeframe Bawah
	// 1D bullish, tapi 4H MACD-nya bearish.
	if isMainBullish && secondaryTA.Value.Oscillators.MACD.Macd < secondaryTA.Value.Oscillators.MACD.Signal {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Pelemahan Momentum: Tren utama %s masih naik, namun MACD pada timeframe %s telah cross ke bawah, sinyal pelemahan momentum.", mainTA.Timeframe, secondaryTA.Timeframe))
	}

	// Periksa juga kondisi overbought pada timeframe utama
	if mainRSI > 75 {
		result.Status = dto.Warning
		result.Insight = append(result.Insight, fmt.Sprintf("[WASPADA] Kondisi Jenuh Beli %s: Timeframe utama menunjukkan kondisi sangat jenuh beli (RSI: %.2f).", mainTA.Timeframe, mainRSI))
	}
}

// evaluateTrailingStop diperbarui untuk mempertimbangkan breakout di 4H
func (s *tradingService) evaluateTrailingStop(result *dto.PositionAnalysis, mainTA, secondaryTA *dto.TradingViewScanner, technicalSignal string, mainOHLCV []dto.StockOHLCV) {
	totalProfitRange := result.TakeProfitPrice - result.EntryPrice
	currentProfit := result.LastPrice - result.EntryPrice

	// Aturan 1: Kuat & Profit Signifikan (dari evaluasi umum)
	if (technicalSignal == dto.SignalStrongBuy || technicalSignal == dto.SignalBuy) && totalProfitRange > 0 && (currentProfit/totalProfitRange) > 0.6 {
		result.Signal = dto.TrailingStop
		result.Insight = append(result.Insight, fmt.Sprintf("SINYAL TRAILING STOP: Posisi kuat (technical signal: %s) & profit signifikan.", technicalSignal))
	}

	// Aturan 2 (BARU): Breakout di timeframe 4H
	if secondaryTA != nil {
		resistanceR1_4H := secondaryTA.Value.Pivots.Classic.R1
		// Jika harga menembus resistance R1 di 4H
		if currentProfit > 0 && result.LastPrice > resistanceR1_4H && resistanceR1_4H > result.EntryPrice*1.01 {
			result.Signal = dto.TrailingStop
			result.Insight = append(result.Insight, fmt.Sprintf("SINYAL TRAILING STOP: Harga menembus resistance kunci di timeframe %s (R1 @ %.2f).", secondaryTA.Timeframe, resistanceR1_4H))
		}
	}

	if result.Signal == dto.TrailingStop && len(mainOHLCV) >= 2 {
		dynamicSL := mainOHLCV[len(mainOHLCV)-2].Low
		result.Insight = append(result.Insight, fmt.Sprintf("REKOMENDASI LEVEL SL: Naikkan Stop Loss ke support dinamis di %.2f (low candle %s sebelumnya).", dynamicSL, mainTA.Timeframe))
	}
}

// Helper untuk menganalisis potensi di level Take Profit.
func (s *tradingService) evaluatePotentialAtTakeProfit(mainTA *dto.TradingViewScanner, mainOHLCV []dto.StockOHLCV) (bool, string) {
	if mainTA == nil || len(mainOHLCV) == 0 {
		return false, "" // Tidak bisa dianalisis jika data tidak ada
	}

	lastCandle := mainOHLCV[len(mainOHLCV)-1]

	// Kondisi 1: Candle harus menunjukkan kekuatan (bukan penolakan/rejection)
	isStrongCandle := s.isStrongBullishCandle(lastCandle)

	// Kondisi 2: Volume harus mengkonfirmasi pergerakan
	avgVolume := s.calculateAverageVolume(mainOHLCV, 20)
	isHighVolume := avgVolume > 0 && float64(lastCandle.Volume) > (avgVolume*1.5)

	// Kondisi 3: RSI belum 'terlalu panas' (memberi ruang untuk naik)
	isRSIHealthy := mainTA.Value.Oscillators.RSI < 85

	if isStrongCandle && isHighVolume && isRSIHealthy {
		// Semua kondisi terpenuhi, ada potensi!
		nextTarget := mainTA.Value.Pivots.Classic.R2
		explanation := fmt.Sprintf("[POTENSI LANJUTAN] Harga menembus TP dengan candle kuat dan volume tinggi (%.0f vs avg %.0f). Target potensial berikutnya adalah R2 di %.2f.", float64(lastCandle.Volume), avgVolume, nextTarget)
		return true, explanation
	}

	return false, "Momentum tidak cukup kuat untuk melanjutkan kenaikan secara signifikan."
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
