package service

import (
	"golang-trading/internal/dto"
	"sort"
)

const (
	minTouches           = 1
	slFromEMAAdj         = 0.995
	targetRiskReward     = 1.0
	maxStopLossPercent   = 0.05
	minStopLossPercent   = 0.02
	maxTakeProfitPercent = 0.07
	minTakeProfitPercent = 0.02
	atrMultiplier        = 1.0 // ATR multiplier for SL/TP adjustment
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
		adjustedPrice := price - (atr * atrMultiplier)
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
		adjustedPrice := price + (atr * atrMultiplier)
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

// calculatePlan evaluates all possible SL/TP combinations and selects the best one based on a scoring system.
func (s *tradingService) calculatePlan(
	marketPrice float64,
	supports []dto.Level,
	resistances []dto.Level,
	emas []dto.EMAData,
	priceBuckets []dto.PriceBucket,
	atr float64,
) dto.TradePlan {
	slCandidates := getSLCandidates(marketPrice, supports, emas, priceBuckets, atr)
	tpCandidates := getTPCandidates(marketPrice, resistances, priceBuckets, atr)

	var bestPlan dto.TradePlan
	highestScore := -1.0

	for _, sl := range slCandidates {
		risk := marketPrice - sl.Price
		if risk <= 0 {
			continue
		}
		riskPct := risk / marketPrice
		if riskPct > maxStopLossPercent || riskPct < minStopLossPercent {
			continue
		}

		// We only need to check the most realistic TP candidates (the first few)
		for _, tp := range tpCandidates {

			reward := tp.Price - marketPrice
			if reward <= 0 {
				continue
			}
			rewardPct := reward / marketPrice
			if rewardPct > maxTakeProfitPercent {
				continue
			}

			riskRewardRatio := reward / risk
			if riskRewardRatio < targetRiskReward {
				continue
			}

			// Scoring: Combines RRR with the strength of the SL and TP levels.
			// This is a simple scoring model, can be enhanced further.
			currentScore := (riskRewardRatio * 0.5) + (sl.Score * 0.3) + (tp.Score * 0.2)

			if currentScore > highestScore {
				highestScore = currentScore
				bestPlan = dto.TradePlan{
					Entry:      marketPrice,
					StopLoss:   sl.Price,
					TakeProfit: tp.Price,
					Risk:       risk,
					Reward:     reward,
					RiskReward: riskRewardRatio,
					SLType:     sl.Type,
					SLReason:   sl.Reason,
					TPType:     tp.Type,
					TPReason:   tp.Reason,
					Score:      currentScore,
				}
			}
		}
	}

	return bestPlan
}
