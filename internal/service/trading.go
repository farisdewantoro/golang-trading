package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/contract"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"math"
	"sort"
)

type TradingService interface {
	BuyListTradePlan(ctx context.Context, mapSymbolExchangeAnalysis map[string][]model.StockAnalysis) ([]dto.TradePlanResult, error)
	BuildTimeframePivots(analysis *model.StockAnalysis) ([]dto.TimeframePivot, error)
	contract.TradingPositionContract
	contract.TradingPlanContract
}

type tradingService struct {
	cfg                   *config.Config
	log                   *logger.Logger
	systemParamRepository repository.SystemParamRepository
}

func NewTradingService(
	cfg *config.Config,
	log *logger.Logger,
	systemParamRepository repository.SystemParamRepository,
) TradingService {
	return &tradingService{
		cfg:                   cfg,
		log:                   log,
		systemParamRepository: systemParamRepository,
	}
}

func (s *tradingService) CreateTradePlan(ctx context.Context, latestAnalyses []model.StockAnalysis) (*dto.TradePlanResult, error) {
	var (
		supports               []dto.Level
		resistances            []dto.Level
		emaData                []dto.EMAData
		priceBuckets           []dto.PriceBucket
		mainTFCandles          []dto.StockOHLCV
		marketPrice            float64
		result                 *dto.TradePlanResult
		tfMap                  map[string]dto.DataTimeframe
		tfHighestTechnicalData dto.TradingViewScanner
		highestWeightTF        string
		highestWeightTFScore   int
	)

	if len(latestAnalyses) == 0 {
		return nil, fmt.Errorf("no latest analysis")
	}

	timeframes, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get default analysis timeframes", logger.ErrorField(err))
		return nil, err
	}

	tfMap = make(map[string]dto.DataTimeframe)
	for _, tf := range timeframes {
		tfMap[tf.Interval] = tf
		if tf.Weight > highestWeightTFScore {
			highestWeightTF = tf.Interval
			highestWeightTFScore = tf.Weight
		}
	}

	lastAnalysis := latestAnalyses[len(latestAnalyses)-1]

	stockCodeWithExchange := lastAnalysis.Exchange + ":" + lastAnalysis.StockCode
	cacheKey := fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange)
	marketPrice, _ = cache.GetFromCache[float64](cacheKey)

	if marketPrice == 0 {
		marketPrice = lastAnalysis.MarketPrice
	}

	for _, analysis := range latestAnalyses {
		var (
			technicalData dto.TradingViewScanner
			candles       []dto.StockOHLCV
		)
		if err := json.Unmarshal(analysis.TechnicalData, &technicalData); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(analysis.OHLCV, &candles); err != nil {
			return nil, err
		}

		isMainTF := tfMap[analysis.Timeframe].IsMain

		if analysis.Timeframe == highestWeightTF {
			tfHighestTechnicalData = technicalData
			mainTFCandles = candles
		}

		emaData = append(emaData, dto.EMAData{
			Timeframe: analysis.Timeframe,
			EMA10:     technicalData.Value.MovingAverages.EMA10,
			EMA20:     technicalData.Value.MovingAverages.EMA20,
			EMA50:     technicalData.Value.MovingAverages.EMA50,
			IsMain:    isMainTF,
		})

		priceBuckets = append(priceBuckets, s.GetClosePriceBuckets(candles)...)

		supports = append(supports, s.buildPivots(technicalData, candles, analysis.Timeframe, true)...)

		resistances = append(resistances, s.buildPivots(technicalData, candles, analysis.Timeframe, false)...)
	}

	s.log.DebugContext(ctx, "Create Trade Plan", logger.StringField("stock_code", stockCodeWithExchange))

	// Calculate ATR using the main timeframe's candles
	atr14 := s.calculateATR(mainTFCandles, 14)
	slAtrMultiplier := s.getATRMultiplierForSL(&tfHighestTechnicalData)

	plan := s.calculatePlan(float64(marketPrice), supports, resistances, emaData, priceBuckets, atr14, slAtrMultiplier)

	positionAnalysis, err := s.EvaluatePositionMonitoring(ctx, &model.StockPosition{
		StockCode:       lastAnalysis.StockCode,
		Exchange:        lastAnalysis.Exchange,
		BuyPrice:        plan.Entry,
		TakeProfitPrice: plan.TakeProfit,
		StopLossPrice:   plan.StopLoss,
	}, latestAnalyses)

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to evaluate position monitoring when create trade plan", logger.ErrorField(err))
		return nil, err
	}

	result = &dto.TradePlanResult{
		CurrentMarketPrice: float64(marketPrice),
		Symbol:             lastAnalysis.StockCode,
		Entry:              plan.Entry,
		StopLoss:           plan.StopLoss,
		TakeProfit:         plan.TakeProfit,
		RiskReward:         plan.RiskReward,
		Status:             string(positionAnalysis.Status),
		TechnicalSignal:    string(positionAnalysis.TechnicalSignal),
		Score:              positionAnalysis.Score,
		IsBuySignal:        positionAnalysis.TechnicalSignal == dto.SignalBuy || positionAnalysis.TechnicalSignal == dto.SignalStrongBuy,
		SLReason:           plan.SLReason,
		TPReason:           plan.TPReason,
		IndicatorSummary:   s.CreateIndicatorSummary(&tfHighestTechnicalData, mainTFCandles),
		Insights:           positionAnalysis.Insight,
		Exchange:           lastAnalysis.Exchange,
		PlanType:           plan.PlanType,
	}
	s.log.DebugContext(ctx, "Finished create trade plan", logger.StringField("stock_code", stockCodeWithExchange))

	return result, nil
}

func (s *tradingService) BuyListTradePlan(ctx context.Context, mapSymbolExchangeAnalysis map[string][]model.StockAnalysis) ([]dto.TradePlanResult, error) {
	var (
		listTradePlan = []dto.TradePlanResult{}
	)

	for _, analyses := range mapSymbolExchangeAnalysis {
		tradePlan, err := s.CreateTradePlan(ctx, analyses)
		if err != nil {
			return nil, err
		}

		if tradePlan == nil {
			continue
		}

		if !tradePlan.IsBuySignal {
			continue
		}

		listTradePlan = append(listTradePlan, *tradePlan)
	}

	// add sort by score to return buylistresult
	sort.Slice(listTradePlan, func(i, j int) bool {
		return listTradePlan[i].Score > listTradePlan[j].Score
	})

	return listTradePlan, nil

}

func (s *tradingService) BuildTimeframePivots(analysis *model.StockAnalysis) ([]dto.TimeframePivot, error) {
	var (
		technicalData dto.TradingViewScanner
		candles       []dto.StockOHLCV
		result        []dto.TimeframePivot

		classicResistance []dto.Level
		classicSupport    []dto.Level

		fibonacciResistance []dto.Level
		fibonacciSupport    []dto.Level
	)
	if err := json.Unmarshal(analysis.TechnicalData, &technicalData); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(analysis.OHLCV, &candles); err != nil {
		return nil, err
	}

	support := s.buildPivots(technicalData, candles, analysis.Timeframe, true)
	resistance := s.buildPivots(technicalData, candles, analysis.Timeframe, false)

	sort.Slice(support, func(i, j int) bool {
		return support[i].Price < support[j].Price && support[i].Touches > support[j].Touches
	})

	sort.Slice(resistance, func(i, j int) bool {
		return resistance[i].Price > resistance[j].Price && resistance[i].Touches > resistance[j].Touches
	})

	showMax := 2
	for _, s := range support {
		if s.Type == dto.LevelClassic && len(classicSupport) < showMax {
			classicSupport = append(classicSupport, s)
		}
		if s.Type == dto.LevelFibonacci && len(fibonacciSupport) < showMax {
			fibonacciSupport = append(fibonacciSupport, s)
		}
	}

	for _, r := range resistance {
		if r.Type == dto.LevelClassic && len(classicResistance) < showMax {
			classicResistance = append(classicResistance, r)
		}
		if r.Type == dto.LevelFibonacci && len(fibonacciResistance) < showMax {
			fibonacciResistance = append(fibonacciResistance, r)
		}
	}

	result = append(result, dto.TimeframePivot{
		Timeframe: analysis.Timeframe,
		PivotData: []dto.PivotData{
			{
				Support:    classicSupport,
				Resistance: classicResistance,
				Type:       dto.LevelClassic,
			},
			{
				Support:    fibonacciSupport,
				Resistance: fibonacciResistance,
				Type:       dto.LevelFibonacci,
			},
		},
	})

	return result, nil

}

func (s *tradingService) buildPivots(technicalData dto.TradingViewScanner, candles []dto.StockOHLCV, timeframe string, isSupp bool) []dto.Level {
	result := []dto.Level{}
	if isSupp {
		result = append(result,
			dto.Level{
				Price:     technicalData.Value.Pivots.Classic.S1,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.S1, isSupp),
				Type:      dto.LevelClassic,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Classic.S2,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.S2, isSupp),
				Type:      dto.LevelClassic,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Classic.S3,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.S3, isSupp),
				Type:      dto.LevelClassic,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Fibonacci.S1,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.S1, isSupp),
				Type:      dto.LevelFibonacci,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Fibonacci.S2,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.S2, isSupp),
				Type:      dto.LevelFibonacci,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Fibonacci.S3,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.S3, isSupp),
				Type:      dto.LevelFibonacci,
			})
	} else {
		result = append(result,
			dto.Level{
				Price:     technicalData.Value.Pivots.Classic.R1,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.R1, isSupp),
				Type:      dto.LevelClassic,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Classic.R2,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.R2, isSupp),
				Type:      dto.LevelClassic,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Classic.R3,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.R3, isSupp),
				Type:      dto.LevelClassic,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Fibonacci.R1,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.R1, isSupp),
				Type:      dto.LevelFibonacci,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Fibonacci.R2,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.R2, isSupp),
				Type:      dto.LevelFibonacci,
			},
			dto.Level{
				Price:     technicalData.Value.Pivots.Fibonacci.R3,
				Timeframe: timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.R3, isSupp),
				Type:      dto.LevelFibonacci,
			},
		)
	}
	return result
}

func (s *tradingService) GetClosePriceBuckets(candles []dto.StockOHLCV) []dto.PriceBucket {
	if len(candles) == 0 {
		return nil
	}

	// Hitung rata-rata range (High - Low)
	var totalRange float64
	for _, candle := range candles {
		totalRange += candle.High - candle.Low
	}
	avgRange := totalRange / float64(len(candles))

	// Tentukan bucketSize berdasarkan avg range
	bucketSize := avgRange / 3
	if bucketSize < 0.1 {
		bucketSize = 0.1 // batas bawah agar tidak terlalu kecil
	}

	// Hitung frekuensi harga close ke dalam bucket
	priceFreq := map[float64]int{}
	for _, candle := range candles {
		bucket := math.Round(candle.Close/bucketSize) * bucketSize
		priceFreq[bucket]++
	}

	// Ubah map menjadi slice of dto.PriceBucket
	var result []dto.PriceBucket
	for bucket, count := range priceFreq {
		result = append(result, dto.PriceBucket{
			Bucket: bucket,
			Count:  count,
		})
	}

	return result
}

func (s *tradingService) calculateATR(candles []dto.StockOHLCV, period int) float64 {
	if len(candles) <= period {
		return 0 // Not enough data
	}

	trueRanges := make([]float64, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		high := candles[i].High
		low := candles[i].Low
		prevClose := candles[i-1].Close

		tr1 := high - low
		tr2 := math.Abs(high - prevClose)
		tr3 := math.Abs(low - prevClose)

		trueRanges[i-1] = math.Max(tr1, math.Max(tr2, tr3))
	}

	// Smoothed Moving Average (Wilder's Smoothing)
	atr := 0.0
	for i := 0; i < period; i++ {
		atr += trueRanges[i]
	}
	atr /= float64(period)

	for i := period; i < len(trueRanges); i++ {
		atr = (atr*float64(period-1) + trueRanges[i]) / float64(period)
	}

	return atr
}

func (s *tradingService) getATRMultiplierForSL(ta *dto.TradingViewScanner) float64 {
	// =========================================================================
	// Bagian 1: Konfigurasi Bobot dan Rentang Multiplier
	// =========================================================================

	// Bobot menentukan seberapa penting setiap indikator dalam pengambilan keputusan.
	const adxWeight float64 = 0.50 // ADX (kekuatan tren) dianggap paling penting.
	const rsiWeight float64 = 0.30 // RSI (momentum/jenuh) memiliki bobot lebih rendah.
	totalWeight := adxWeight + rsiWeight

	// Rentang multiplier yang diinginkan.
	const multiplierMax float64 = 2.25 // SL Terlebar: Digunakan saat pasar lemah/ranging (skor -1.0).
	const multiplierMin float64 = 1.25 // SL Terketat: Digunakan saat tren kuat (skor +1.0).

	// =========================================================================
	// Bagian 2: Penilaian (Scoring) Indikator
	// =========================================================================
	// Skor dikonversi menjadi rentang [-1.0, 1.0] untuk normalisasi.
	// -1.0 berarti "kondisi butuh SL lebar".
	// +1.0 berarti "kondisi memungkinkan SL ketat".

	var adxScore, rsiScore float64

	// Skor ADX: Tren kuat (ADX > 25) mendukung SL yang lebih ketat.
	if ta.Value.Oscillators.ADX.Value > 25 {
		adxScore = 1.0
	} else {
		adxScore = -1.0
	}

	// Skor RSI: Pasar jenuh/ekstrem (RSI > 70 atau < 30) memerlukan SL yang lebih lebar
	// untuk bertahan dari potensi koreksi tajam.
	if ta.Value.Oscillators.RSI > 70 || ta.Value.Oscillators.RSI < 30 {
		rsiScore = -1.0
	} else {
		rsiScore = 1.0
	}

	// =========================================================================
	// Bagian 3: Perhitungan Skor Akhir dan Pemetaan
	// =========================================================================

	// Hitung skor akhir dengan menggabungkan skor individu sesuai bobotnya.
	finalScore := (adxScore*adxWeight + rsiScore*rsiWeight) / totalWeight

	// Normalisasi skor akhir dari rentang [-1, 1] menjadi persentase [0, 1].
	// Skor -1.0 akan menjadi 0%, dan skor +1.0 akan menjadi 100%.
	percentage := (finalScore + 1.0) / 2.0

	// Lakukan interpolasi linear untuk memetakan persentase ke rentang multiplier.
	// Rumus ini memastikan bahwa:
	// - Persentase 0% (skor -1.0) menghasilkan multiplierMax.
	// - Persentase 100% (skor +1.0) menghasilkan multiplierMin.
	finalMultiplier := multiplierMax - percentage*(multiplierMax-multiplierMin)

	return finalMultiplier
}
func (s *tradingService) countTouches(candles []dto.StockOHLCV, level float64, isSupport bool) int {
	tolerancePercent := 0.5
	tolerance := level * tolerancePercent / 100.0
	low := level - tolerance
	high := level + tolerance
	touches := 0

	for _, c := range candles {
		if isSupport {
			if c.Low <= high && c.High >= low {
				touches++
			}
		} else {
			if c.High >= high && c.Low <= low {
				touches++
			}
		}
	}
	return touches
}

func (s *tradingService) CreateIndicatorSummary(technicalData *dto.TradingViewScanner, candles []dto.StockOHLCV) model.IndicatorSummary {

	return model.IndicatorSummary{
		RSI:    dto.GetRSIStatus(int(technicalData.Value.Oscillators.RSI)),
		Volume: utils.FormatVolume(candles[len(candles)-1].Volume),
		MACD:   dto.GetTrendText(technicalData.Recommend.Oscillators.MACD),
		MA:     dto.GetTrendText(technicalData.Recommend.Global.MA),
		Osc:    dto.GetSignalText(technicalData.Recommend.Global.Oscillators),
	}
}
