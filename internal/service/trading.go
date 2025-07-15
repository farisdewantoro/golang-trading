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
	"math"
	"sort"
)

type TradingService interface {
	CreateTradePlan(ctx context.Context, latestAnalyses []model.StockAnalysis) (*dto.TradePlanResult, error)
	BuyListTradePlan(ctx context.Context, mapSymbolExchangeAnalysis map[string][]model.StockAnalysis) ([]dto.TradePlanResult, error)
	BuildTimeframePivots(analysis *model.StockAnalysis) ([]dto.TimeframePivot, error)
	contract.TradingPositionContract
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
		supports      []dto.Level
		resistances   []dto.Level
		emaData       []dto.EMAData
		priceBuckets  []dto.PriceBucket
		mainTFCandles []dto.StockOHLCV
		marketPrice   float64
		result        *dto.TradePlanResult
		tfMap         map[string]dto.DataTimeframe
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
		if isMainTF {
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

	plan := s.calculatePlan(float64(marketPrice), supports, resistances, emaData, priceBuckets, atr14)

	score, signal, err := s.evaluateSignal(ctx, timeframes, latestAnalyses)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to evaluate signal", logger.ErrorField(err))
		return nil, err
	}

	result = &dto.TradePlanResult{
		CurrentMarketPrice: float64(marketPrice),
		Symbol:             stockCodeWithExchange,
		Entry:              plan.Entry,
		StopLoss:           plan.StopLoss,
		TakeProfit:         plan.TakeProfit,
		RiskReward:         plan.RiskReward,
		Status:             signal,
		Score:              float64(score),
		IsBuySignal:        signal == dto.SignalStrongBuy || signal == dto.SignalBuy,
		SLReason:           plan.SLReason,
		TPReason:           plan.TPReason,
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
		scoreWeight := 0.75
		riskRewardWeight := 0.25

		scoreI := (listTradePlan[i].Score * scoreWeight) + (listTradePlan[i].RiskReward * riskRewardWeight)
		scoreJ := (listTradePlan[j].Score * scoreWeight) + (listTradePlan[j].RiskReward * riskRewardWeight)
		return scoreI > scoreJ
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

	for _, s := range support {
		if s.Type == dto.LevelClassic {
			classicSupport = append(classicSupport, s)
		}
		if s.Type == dto.LevelFibonacci {
			fibonacciSupport = append(fibonacciSupport, s)
		}
	}

	for _, r := range resistance {
		if r.Type == dto.LevelClassic {
			classicResistance = append(classicResistance, r)
		}
		if r.Type == dto.LevelFibonacci {
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
