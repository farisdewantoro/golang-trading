package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/helper"
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
	EvaluateSignal(ctx context.Context, latestAnalyses []model.StockAnalysis) (string, error)
	BuyListTradePlan(ctx context.Context, mapSymbolExchangeAnalysis map[string][]model.StockAnalysis) ([]dto.TradePlanResult, error)
	BuildTimeframePivots(analysis *model.StockAnalysis) ([]dto.TimeframePivot, error)
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
		supports    []dto.Level
		resistances []dto.Level
		marketPrice float64
		result      *dto.TradePlanResult
		emaData     []dto.EMAData

		tfMap        map[string]dto.DataTimeframe
		priceBuckets []dto.PriceBucket
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
	plan := s.calculatePlan(float64(marketPrice), supports, resistances, emaData, priceBuckets)

	score, signal, err := helper.EvaluateSignal(ctx, s.log, timeframes, latestAnalyses)
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
	}

	return result, nil
}

func (s *tradingService) calculatePlan(
	marketPrice float64,
	supports []dto.Level,
	resistances []dto.Level,
	emas []dto.EMAData,
	priceBuckets []dto.PriceBucket,
) dto.TradePlan {
	const (
		minTouches     = 1
		maxSLDistance  = 0.03  // 3%
		slFromEMA20Adj = 0.995 // SL 0.5% di bawah EMA20
		targetRR       = 1.0   // target risk reward
	)

	// Step 1: Stop Loss
	var slSupport dto.Level
	for _, s := range supports {
		if s.Price < marketPrice && s.Touches >= minTouches {
			if slSupport.Price == 0 || s.Touches > slSupport.Touches {
				slSupport = s
			}
		}
	}
	if slSupport.Price == 0 {
		return dto.TradePlan{}
	}
	sl := slSupport.Price

	// SL Adjustment dengan EMA
	for _, ema := range emas {
		if !ema.IsMain {
			continue
		}
		if sl < ema.EMA20 {
			diff := ema.EMA20 - sl
			if diff/ema.EMA20 > maxSLDistance {
				sl = ema.EMA20 * slFromEMA20Adj
			}
		}
	}

	risk := marketPrice - sl
	if risk <= 0 {
		return dto.TradePlan{}
	}

	// Step 2: TP dari resistance touch tertinggi
	var tpResistance dto.Level
	for _, r := range resistances {
		if r.Price > marketPrice {
			if tpResistance.Price == 0 || r.Touches > tpResistance.Touches {
				tpResistance = r
			}
		}
	}
	if tpResistance.Price == 0 {
		return dto.TradePlan{}
	}

	tp := tpResistance.Price
	reward := tp - marketPrice
	riskReward := reward / risk

	// Step 3: Fallback jika RR < targetRR
	if riskReward < targetRR {
		// Fallback #2: dari price bucket
		var altTP float64
		maxCount := -1
		for _, pb := range priceBuckets {
			if pb.Bucket > marketPrice && pb.Bucket > tpResistance.Price {
				if pb.Count > maxCount {
					altTP = pb.Bucket
					maxCount = pb.Count
				}
			}
		}

		if altTP > 0 {
			tp = altTP
			reward = tp - marketPrice
			riskReward = reward / risk
		} else {
			// Fallback #3: ambil 2 resistance terdekat di atas tpResistance
			var higherRes []float64
			for _, r := range resistances {
				if r.Price > tpResistance.Price {
					higherRes = append(higherRes, r.Price)
				}
			}
			sort.Float64s(higherRes)
			if len(higherRes) >= 2 {
				mid := (higherRes[0] + higherRes[1]) / 2
				tp = mid
				reward = tp - marketPrice
				riskReward = reward / risk
			} else if len(higherRes) == 1 {
				tp = higherRes[0]
				reward = tp - marketPrice
				riskReward = reward / risk
			}
			// else: fallback gagal, tp tetap tpResistance
		}
	}

	return dto.TradePlan{
		Entry:      marketPrice,
		StopLoss:   sl,
		TakeProfit: tp,
		Risk:       risk,
		Reward:     reward,
		RiskReward: riskReward,
		IsTPValid:  true,
	}
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

func (s *tradingService) EvaluateSignal(ctx context.Context, latestAnalyses []model.StockAnalysis) (string, error) {

	// Ambil konfigurasi bobot timeframe
	dtf, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get default analysis timeframes", logger.ErrorField(err))
		return "", err
	}

	_, signal, err := helper.EvaluateSignal(ctx, s.log, dtf, latestAnalyses)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to evaluate signal", logger.ErrorField(err))
		return "", err
	}
	return signal, nil
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
		scoreI := listTradePlan[i].Score * listTradePlan[i].RiskReward
		scoreJ := listTradePlan[j].Score * listTradePlan[j].RiskReward
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
