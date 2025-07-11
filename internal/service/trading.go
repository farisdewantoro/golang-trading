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
	"sort"
)

type TradingService interface {
	CreateTradePlan(ctx context.Context, latestAnalyses []model.StockAnalysis) (*dto.TradePlanResult, error)
	EvaluateSignal(ctx context.Context, latestAnalyses []model.StockAnalysis) (string, error)
	BuyListTradePlan(ctx context.Context, mapSymbolExchangeAnalysis map[string][]model.StockAnalysis) ([]dto.TradePlanResult, error)
}

type tradingService struct {
	cfg                   *config.Config
	log                   *logger.Logger
	systemParamRepository repository.SystemParamRepository
}

type Level struct {
	Price     float64
	Timeframe string // "1D", "4H", "1H"
	Touches   int    // seberapa sering level disentuh (opsional, default 0)
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
		supports    []Level
		resistances []Level
		marketPrice int
		result      *dto.TradePlanResult

		// mainTrend string
		// maxWeight int
	)

	if len(latestAnalyses) == 0 {
		return nil, fmt.Errorf("no latest analysis")
	}

	timeframes, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get default analysis timeframes", logger.ErrorField(err))
		return nil, err
	}

	// for _, tf := range timeframes {
	// 	if tf.Weight > maxWeight {
	// 		maxWeight = tf.Weight
	// 		mainTrend = tf.Interval
	// 	}
	// }

	lastAnalysis := latestAnalyses[len(latestAnalyses)-1]

	stockCodeWithExchange := lastAnalysis.StockCode + ":" + lastAnalysis.Exchange
	cacheKey := fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange)
	marketPrice, _ = cache.GetFromCache[int](cacheKey)

	if marketPrice == 0 {
		marketPrice = int(lastAnalysis.MarketPrice)
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
		// if analysis.Timeframe != mainTrend {
		// 	continue
		// }

		tolerancePercent := 0.05
		supports = append(supports,
			Level{
				Price:     technicalData.Value.Pivots.Classic.S1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.S1, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Classic.S2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.S2, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Classic.S3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.S3, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Camarilla.S1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Camarilla.S1, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Camarilla.S2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Camarilla.S2, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Camarilla.S3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Camarilla.S3, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Demark.S1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Demark.S1, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Fibonacci.S1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.S1, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Fibonacci.S2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.S2, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Fibonacci.S3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.S3, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Woodie.S1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Woodie.S1, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Woodie.S2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Woodie.S2, tolerancePercent, true),
			},
			Level{
				Price:     technicalData.Value.Pivots.Woodie.S3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Woodie.S3, tolerancePercent, true),
			},
		)
		resistances = append(resistances,
			Level{
				Price:     technicalData.Value.Pivots.Classic.R1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.R1, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Classic.R2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.R2, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Classic.R3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Classic.R3, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Camarilla.R1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Camarilla.R1, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Camarilla.R2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Camarilla.R2, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Camarilla.R3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Camarilla.R3, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Demark.R1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Demark.R1, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Fibonacci.R1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.R1, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Fibonacci.R2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.R2, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Fibonacci.R3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Fibonacci.R3, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Woodie.R1,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Woodie.R1, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Woodie.R2,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Woodie.R2, tolerancePercent, false),
			},
			Level{
				Price:     technicalData.Value.Pivots.Woodie.R3,
				Timeframe: analysis.Timeframe,
				Touches:   s.countTouches(candles, technicalData.Value.Pivots.Woodie.R3, tolerancePercent, false),
			},
		)
	}

	s.log.DebugContext(ctx, "Create Trade Plan", logger.StringField("stock_code", stockCodeWithExchange))
	plan := s.calculatePlan(float64(marketPrice), supports, resistances)

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
	supports []Level,
	resistances []Level,
) dto.TradePlan {
	const minTouches = 2

	// SL: support dengan touches terbanyak, di bawah marketPrice
	var slSupport Level
	for _, s := range supports {
		if s.Price < marketPrice && s.Touches >= minTouches {
			if slSupport.Price == 0 || s.Touches > slSupport.Touches {
				slSupport = s
			}
		}
	}
	if slSupport.Price == 0 {
		return dto.TradePlan{} // Tidak ada SL valid
	}
	sl := slSupport.Price
	risk := marketPrice - sl
	if risk <= 0 {
		return dto.TradePlan{}
	}

	// TP: resistance dengan touches terbanyak, di atas marketPrice
	var tpResistance Level
	for _, r := range resistances {
		if r.Price > marketPrice && r.Touches >= minTouches {
			if tpResistance.Price == 0 || r.Touches > tpResistance.Touches {
				tpResistance = r
			}
		}
	}
	if tpResistance.Price == 0 {
		return dto.TradePlan{} // Tidak ada TP valid
	}
	tp := tpResistance.Price
	reward := tp - marketPrice
	riskReward := reward / risk

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

func (s *tradingService) countTouches(candles []dto.StockOHLCV, level float64, tolerancePercent float64, isSupport bool) int {
	tolerance := level * tolerancePercent / 100.0
	touches := 0

	for _, c := range candles {
		if isSupport {
			if c.Low <= level+tolerance && c.High >= level-tolerance {
				touches++
			}
		} else {
			if c.High >= level-tolerance && c.Low <= level+tolerance {
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

		if !tradePlan.IsBuySignal {
			continue
		}

		listTradePlan = append(listTradePlan, *tradePlan)
	}

	// add sort by score to return buylistresult
	sort.Slice(listTradePlan, func(i, j int) bool {
		return listTradePlan[i].Score > listTradePlan[j].Score && listTradePlan[i].RiskReward > listTradePlan[j].RiskReward
	})

	return listTradePlan, nil

}
