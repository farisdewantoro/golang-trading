package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/logger"
	"sort"
)

const (
	minTouches               = 1
	slFromEMAAdj             = 0.995
	targetRiskReward         = 1.5
	targetRiskRewardFallback = 1.0

	maxStopLossPercent   = 0.05
	minStopLossPercent   = 0.02
	maxTakeProfitPercent = 0.07
	minTakeProfitPercent = 0.02

	fallbackMaxTakeProfitPercent = 0.14
	fallbackMinTakeProfitPercent = 0.01
	fallbackMaxStopLossPercent   = 0.10
	fallbackMinStopLossPercent   = 0.01
)

type SLSource struct {
	Price  float64
	Type   string
	Reason string
	Score  float64
}

type TPSource struct {
	Price  float64
	Type   string
	Reason string
	Score  float64
}

// getSLCandidates gathers all potential SL levels from various sources (supports, EMAs, price buckets)
// and returns them as a sorted slice of SLSource.
func getSLCandidates(marketPrice float64, supports []dto.Level, emas []dto.EMAData, priceBuckets []dto.PriceBucket, atr float64) []SLSource {
	var candidates []SLSource
	uniquePrices := make(map[float64]struct{})

	// Helper to add a candidate if its price is unique and below market price
	addCandidate := func(price float64, sourceType, reason string, score float64) {
		adjustedPrice := price - atr
		if adjustedPrice >= marketPrice {
			return // Skip if the adjusted SL is at or above the market price
		}
		if _, exists := uniquePrices[adjustedPrice]; !exists {
			candidates = append(candidates, SLSource{Price: adjustedPrice, Type: sourceType, Reason: reason, Score: score})
			uniquePrices[adjustedPrice] = struct{}{}
		}
	}

	// 1. Add from Supports
	for _, s := range supports {
		addCandidate(s.Price, "SL_SUPPORT", "Support Level", float64(s.Touches))
	}

	// 2. Add from EMAs
	for _, ema := range emas {
		if ema.IsMain {
			// The score for EMAs can be constant or based on their period (e.g., longer-term EMA is stronger)
			addCandidate(ema.EMA10, "SL_EMA10", "Below EMA10", 1.5)
			addCandidate(ema.EMA20, "SL_EMA20", "Below EMA20", 2.0)
			addCandidate(ema.EMA50, "SL_EMA50", "Below EMA50", 2.5)
		}
	}

	// 3. Add from Price Buckets
	for _, pb := range priceBuckets {
		addCandidate(pb.Bucket, "SL_BUCKET", "Price Consolidation", float64(pb.Count)/10.0) // Normalize score
	}

	// Sort candidates by price, descending. The best SL is the highest one below the market price.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Price > candidates[j].Price
	})

	return candidates
}

// getTPCandidates gathers all potential TP levels from resistances and price buckets.
func getTPCandidates(marketPrice float64, resistances []dto.Level, priceBuckets []dto.PriceBucket, atr float64) []TPSource {
	var candidates []TPSource
	uniquePrices := make(map[float64]struct{})

	addCandidate := func(price float64, sourceType, reason string, score float64) {
		adjustedPrice := price - atr
		if adjustedPrice <= marketPrice {
			return
		}
		if _, exists := uniquePrices[adjustedPrice]; !exists {
			candidates = append(candidates, TPSource{Price: adjustedPrice, Type: sourceType, Reason: reason, Score: score})
			uniquePrices[adjustedPrice] = struct{}{}
		}
	}

	// 1. Add from Resistances
	for _, r := range resistances {
		addCandidate(r.Price, "TP_RESISTANCE", "Resistance Level", float64(r.Touches))
	}

	// 2. Add from Price Buckets
	for _, pb := range priceBuckets {
		addCandidate(pb.Bucket, "TP_BUCKET", "Price Consolidation", float64(pb.Count)/10.0)
	}

	// Sort candidates by price, ascending. The best TP is the lowest one above the market price.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Price < candidates[j].Price
	})

	return candidates
}

// findBestPlanForRRR adalah fungsi pembantu yang mencari trade plan terbaik
// untuk target Risk/Reward Ratio (RRR) TERTENTU.
func (s *tradingService) findBestPlanForRRR(
	marketPrice float64,
	slCandidates []SLSource,
	tpCandidates []TPSource,
	config dto.TradeConfig,

) (dto.TradePlan, bool) { // Mengembalikan plan dan boolean 'ditemukan'

	var bestPlan dto.TradePlan
	highestScore := -1.0
	found := false

	for _, sl := range slCandidates {
		risk := marketPrice - sl.Price
		if risk <= 0 {
			continue
		}
		riskPct := risk / marketPrice
		if riskPct > config.MaxStopLossPercent || riskPct < config.MinStopLossPercent {
			continue
		}

		numTPToCheck := len(tpCandidates)
		if numTPToCheck > 5 {
			numTPToCheck = 5
		}

		for i := 0; i < len(tpCandidates); i++ {
			tp := tpCandidates[i]
			reward := tp.Price - marketPrice
			if reward <= 0 {
				continue
			}
			rewardPct := reward / marketPrice
			if rewardPct > config.MaxTakeProfitPercent {
				continue
			}

			riskRewardRatio := reward / risk
			if riskRewardRatio < config.TargetRiskReward {
				continue
			}

			currentScore := (riskRewardRatio * 0.5) + (sl.Score * 0.3) + (tp.Score * 0.2) + config.Score
			if currentScore > highestScore {
				highestScore = currentScore
				found = true
				bestPlan = dto.TradePlan{
					Entry: marketPrice, StopLoss: sl.Price, TakeProfit: tp.Price,
					Risk: risk, Reward: reward, RiskReward: riskRewardRatio,
					SLType: sl.Type, SLReason: sl.Reason, TPType: tp.Type, TPReason: tp.Reason,
					Score:    currentScore,
					PlanType: config.Type,
				}
			}
		}
	}
	return bestPlan, found
}

// findIdealPlan mencari trade plan terbaik berdasarkan level teknis (support/resistance)
func (s *tradingService) findIdealPlan(
	marketPrice float64,
	slCandidates []SLSource,
	tpCandidates []TPSource,
) dto.TradePlan {

	config := dto.TradeConfig{
		TargetRiskReward:     targetRiskReward,
		MaxStopLossPercent:   maxStopLossPercent,
		MinStopLossPercent:   minStopLossPercent,
		MaxTakeProfitPercent: maxTakeProfitPercent,
		MinTakeProfitPercent: minTakeProfitPercent,
		Type:                 dto.PlanTypePrimary,
		Score:                3,
	}
	if bestPlan, found := s.findBestPlanForRRR(marketPrice, slCandidates, tpCandidates, config); found {
		return bestPlan
	}

	config.TargetRiskReward = targetRiskRewardFallback
	config.Type = dto.PlanTypeSecondary
	config.Score = 2
	if bestPlan, found := s.findBestPlanForRRR(marketPrice, slCandidates, tpCandidates, config); found {
		return bestPlan
	}

	config.MaxTakeProfitPercent = fallbackMaxTakeProfitPercent
	config.MinTakeProfitPercent = fallbackMinTakeProfitPercent
	config.MaxStopLossPercent = fallbackMaxStopLossPercent
	config.MinStopLossPercent = fallbackMinStopLossPercent
	config.Type = dto.PlanTypeFallback
	config.Score = 0
	if bestPlan, found := s.findBestPlanForRRR(marketPrice, slCandidates, tpCandidates, config); found {
		return bestPlan
	}

	return dto.TradePlan{}
}

// calculatePlan evaluates all possible SL/TP combinations and selects the best one based on a scoring system.
func (s *tradingService) calculatePlan(
	marketPrice float64,
	supports []dto.Level,
	resistances []dto.Level,
	emas []dto.EMAData,
	priceBuckets []dto.PriceBucket,
	atr float64,
	slATRMultiplier float64,
) dto.TradePlan {
	slDistance := atr * slATRMultiplier
	tpDistance := atr * 0.1 // 10% of ATR
	slCandidates := getSLCandidates(marketPrice, supports, emas, priceBuckets, slDistance)
	tpCandidates := getTPCandidates(marketPrice, resistances, priceBuckets, tpDistance)

	return s.findIdealPlan(marketPrice, slCandidates, tpCandidates)
}

func (s *tradingService) CalculateSummary(ctx context.Context, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (float64, int, error) {
	var totalScore float64

	mapWeight := make(map[string]int)
	mainTrend := ""
	maxWeight := 0

	for _, tf := range dtf {
		mapWeight[tf.Interval] = tf.Weight
		if tf.Weight > maxWeight {
			maxWeight = tf.Weight
			mainTrend = tf.Interval
		}
	}

	mainTrendScore := -999 // Flag awal jika belum ditemukan

	for _, analysis := range latestAnalyses {
		weight, ok := mapWeight[analysis.Timeframe]
		if !ok {
			s.log.WarnContext(ctx, "Unknown timeframe in analysis", logger.StringField("timeframe", analysis.Timeframe))
			continue
		}

		var technicalData dto.TradingViewScanner
		if err := json.Unmarshal([]byte(analysis.TechnicalData), &technicalData); err != nil {
			s.log.ErrorContext(ctx, "Failed to unmarshal technical data", logger.ErrorField(err))
			continue
		}

		score := technicalData.Recommend.Global.Summary
		totalScore += float64(weight) * (float64(score) + 0.05)

		if analysis.Timeframe == mainTrend {
			mainTrendScore = score
		}
	}

	// Pastikan main trend score ditemukan
	if mainTrendScore == -999 {
		err := fmt.Errorf("mainTrendScore for timeframe %s not found", mainTrend)
		s.log.ErrorContext(ctx, "Main trend score not found", logger.ErrorField(err))
		return 0, mainTrendScore, err
	}
	return totalScore, mainTrendScore, nil
}

func (s *tradingService) evaluateSignal(ctx context.Context, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (score float64, signal string, err error) {

	totalScore, mainTrendScore, err := s.CalculateSummary(ctx, dtf, latestAnalyses)
	if err != nil {
		return 0, "", err
	}

	// Evaluasi sinyal akhir
	switch {
	case totalScore >= 9 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.SignalStrongBuy, nil
	case totalScore >= 6 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.SignalBuy, nil
	case totalScore >= 3 && mainTrendScore >= dto.TradingViewSignalNeutral:
		return totalScore, dto.SignalNeutral, nil
	default:
		return totalScore, dto.SignalSell, nil
	}
}

func (s *tradingService) EvaluatePosition(ctx context.Context, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (float64, dto.Evaluation, error) {

	totalScore, mainTrendScore, err := s.CalculateSummary(ctx, dtf, latestAnalyses)
	if err != nil {
		return 0, dto.Evaluation(""), err
	}

	// Evaluasi sinyal akhir
	switch {
	case totalScore >= 9 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.EvalVeryStrong, nil
	case totalScore >= 6 && mainTrendScore >= dto.TradingViewSignalBuy:
		return totalScore, dto.EvalStrong, nil
	case totalScore >= 3 && mainTrendScore >= dto.TradingViewSignalNeutral:
		return totalScore, dto.EvalNeutral, nil
	case totalScore >= 0:
		return totalScore, dto.EvalWeak, nil
	default:
		return totalScore, dto.EvalVeryWeak, nil
	}

}
