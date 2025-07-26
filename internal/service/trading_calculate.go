package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/logger"
	"math"
	"sort"
)

const (
	minTouches       = 1
	slFromEMAAdj     = 0.995
	targetRiskReward = 1.0

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
		addCandidate(s.Price, "SL_SUPPORT", fmt.Sprintf("Support Level (%d touches)", s.Touches), float64(s.Touches))
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

	// // 3. Add from Price Buckets
	// for _, pb := range priceBuckets {
	// 	addCandidate(pb.Bucket, "SL_BUCKET", "Price Consolidation", float64(pb.Count)/10.0) // Normalize score
	// }

	// Sort candidates by price, descending. The best SL is the highest one below the market price.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
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

		addCandidate(r.Price, "TP_RESISTANCE", fmt.Sprintf("Resistance Level (%d touches)", r.Touches), float64(r.Touches))
	}

	// 2. Add from Price Buckets
	for _, pb := range priceBuckets {
		addCandidate(pb.Bucket, "TP_BUCKET", fmt.Sprintf("Price Consolidation (%d touches)", pb.Count), float64(pb.Count)/10.0)
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
	entryQualityScore int,
) (dto.TradePlan, bool) { // Mengembalikan plan dan boolean 'ditemukan'

	var bestPlan dto.TradePlan
	highestScore := -1.0
	found := false

	normalize := func(value, max float64) float64 {
		if value <= 0 {
			return 0.0
		}
		if value >= max {
			return 1.0
		}
		return value / max
	}

	for _, sl := range slCandidates {
		risk := marketPrice - sl.Price
		if risk <= 0 {
			continue
		}
		riskPct := risk / marketPrice
		if riskPct > config.MaxStopLossPercent || riskPct < config.MinStopLossPercent {
			continue
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

			// --- New Scoring Logic (0-100 scale) ---
			// 1. RRR Score (Weight: 30%) - A great RRR is considered >= 4.0
			rrrScore := normalize(riskRewardRatio, 4.0) * 30.0

			// 2. SL Quality Score (Weight: 20%) - A great SL source score is considered >= 5.0
			slScore := normalize(sl.Score, 5.0) * 20.0

			// 3. TP Quality Score (Weight: 20%) - A great TP source score is considered >= 5.0
			tpScore := normalize(tp.Score, 5.0) * 20.0

			// 4. Entry Quality Score (Weight: 20%) - Max score from calculateEntryQualityScore is 50
			entryScore := normalize(float64(entryQualityScore), 50.0) * 20.0

			// 5. Plan Type Bonus (Weight: 10%) - Based on config (Primary/Secondary/Fallback)
			planTypeScore := normalize(config.Score, 3.0) * 10.0

			currentScore := rrrScore + slScore + tpScore + entryScore + planTypeScore

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
	entryQualityScore int,
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
	if bestPlan, found := s.findBestPlanForRRR(marketPrice, slCandidates, tpCandidates, config, entryQualityScore); found {
		return bestPlan
	}

	config.MaxTakeProfitPercent = fallbackMaxTakeProfitPercent
	config.MinTakeProfitPercent = fallbackMinTakeProfitPercent
	config.MaxStopLossPercent = fallbackMaxStopLossPercent
	config.MinStopLossPercent = fallbackMinStopLossPercent
	config.Type = dto.PlanTypeSecondary
	config.Score = 0
	if bestPlan, found := s.findBestPlanForRRR(marketPrice, slCandidates, tpCandidates, config, entryQualityScore); found {
		return bestPlan
	}

	return dto.TradePlan{}
}

// calculateEntryQualityScore calculates a score based on the quality of the entry price.
func (s *tradingService) calculateEntryQualityScore(marketPrice float64, technicalData *dto.TradingViewScanner) int {
	if technicalData == nil {
		return 0
	}

	score := 0
	ema20 := technicalData.Value.MovingAverages.EMA20
	ema50 := technicalData.Value.MovingAverages.EMA50
	rsi := technicalData.Value.Oscillators.RSI

	// 1. Proximity to Support (the closer to EMA20, the better for a pullback)
	if ema20 > 0 {
		distanceToEMA20 := (marketPrice - ema20) / ema20
		if distanceToEMA20 <= 0.01 { // Within 1% of EMA20
			score += 20
		} else if distanceToEMA20 <= 0.03 { // Within 3% of EMA20
			score += 10
		} else if distanceToEMA20 > 0.05 { // More than 5% away, potentially chasing price
			score -= 10
		}
	}

	// 2. RSI Condition
	if rsi >= 50 && rsi <= 65 {
		score += 15 // Healthy momentum
	} else if rsi > 70 {
		score -= 10 // Overbought, higher risk
	}

	// 3. Trend Confirmation
	if ema20 > ema50 {
		score += 15 // Solid uptrend confirmation
	}

	return score
}

// createATRBasedPlan is a fallback function to create a simple trade plan based on ATR.
// This is used when no suitable plan can be found from support/resistance levels.
func (s *tradingService) createATRBasedPlan(marketPrice, atr float64, slATRMultiplier float64) dto.TradePlan {
	if atr <= 0 {
		return dto.TradePlan{}
	}

	// Define SL and TP based on ATR multipliers. Example: SL=2*ATR, TP=3*ATR for a 1.5 RRR.
	stopLoss := marketPrice - (slATRMultiplier * atr)
	takeProfit := marketPrice + (3 * atr)

	risk := marketPrice - stopLoss
	reward := takeProfit - marketPrice

	if risk <= 0 || reward <= 0 {
		return dto.TradePlan{}
	}

	plan := dto.TradePlan{
		Entry:      marketPrice,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
		Risk:       risk,
		Reward:     reward,
		RiskReward: reward / risk,
		PlanType:   dto.PlanTypeATR,
		SLType:     "ATR_FALLBACK",
		SLReason:   fmt.Sprintf("Fallback based on %fx ATR (%.2f)", slATRMultiplier, atr),
		TPType:     "ATR_FALLBACK",
		TPReason:   fmt.Sprintf("Fallback based on 3x ATR (%.2f)", atr),
		Score:      0.5, // Low score to indicate it's a fallback plan
	}

	return plan
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
	technicalData *dto.TradingViewScanner,
) dto.TradePlan {
	slDistance := atr * slATRMultiplier
	tpDistance := atr * 0.1 // 10% of ATR
	slCandidates := getSLCandidates(marketPrice, supports, emas, priceBuckets, slDistance)
	tpCandidates := getTPCandidates(marketPrice, resistances, priceBuckets, tpDistance)

	// Calculate the entry quality score
	entryQualityScore := s.calculateEntryQualityScore(marketPrice, technicalData)

	// First, try to find the ideal plan from technical levels
	plan := s.findIdealPlan(marketPrice, slCandidates, tpCandidates, entryQualityScore)

	// If no plan is found, use the ATR-based fallback
	if plan.Entry == 0 {
		s.log.Info("No ideal plan found, creating ATR-based fallback plan.")
		plan = s.createATRBasedPlan(marketPrice, atr, slATRMultiplier)
	}

	return plan
}

// calculateSmartEntry determines the optimal entry price using advanced technical analysis
// This enhanced version considers volume, candlestick patterns, volatility, and momentum
func (s *tradingService) calculateSmartEntry(
	marketPrice float64,
	technicalData *dto.TradingViewScanner,
	candles []dto.StockOHLCV,
	supports []dto.Level,
	resistances []dto.Level,
	emaData []dto.EMAData,
) EntryResult {
	// Start with market price as baseline
	smartEntry := marketPrice
	entryReason := "Entry di harga pasar saat ini"

	// Validate input data
	if len(candles) == 0 || technicalData == nil {
		return EntryResult{Price: smartEntry, Reason: "Entry di harga pasar - data teknikal tidak lengkap"}
	}

	currentPrice := marketPrice

	// Calculate ATR for volatility-based adjustments
	atr := s.calculateATR(candles, 14)
	// Analyze overall market conditions
	marketConditions := s.analyzeMarketConditions(technicalData, candles)

	// Get trend score with enhanced analysis
	trendScore := s.calculateEnhancedTrendScore(technicalData, candles)

	// Analyze volume confirmation
	volumeConfirmation := s.analyzeVolumeConfirmation(candles)

	// Detect candlestick patterns
	candlestickSignal := s.detectCandlestickPatterns(candles)

	// Calculate momentum score
	momentumScore := s.calculateMomentumScore(technicalData)

	// For bullish trend with confirmations
	if trendScore > 0 {
		entryOptions := s.getBullishEntryOptions(currentPrice, supports, emaData, technicalData, atr)

		// Score each entry option based on multiple factors
		bestEntry := s.selectBestEntry(entryOptions, volumeConfirmation, candlestickSignal, momentumScore, marketConditions)

		if bestEntry.Price > 0 && bestEntry.Price <= currentPrice {
			// Apply volatility adjustment
			volatilityAdjustment := s.calculateVolatilityAdjustment(atr, marketConditions.Volatility)
			smartEntry = bestEntry.Price * (1 - volatilityAdjustment)
			entryReason = fmt.Sprintf("Trend naik - %s dengan penyesuaian volatilitas %.1f%%", bestEntry.Reason, volatilityAdjustment*100)

			// Ensure entry is within acceptable range
			if (currentPrice-smartEntry)/currentPrice <= 0.04 { // Max 4% below for bullish
				// Apply final momentum adjustment
				if momentumScore > 5 {
					smartEntry = smartEntry * 1.002 // Slightly higher entry for strong momentum
					entryReason += " + momentum kuat"
				}
			} else {
				smartEntry = currentPrice * 0.98 // Conservative fallback
				entryReason = "Trend naik - entry konservatif 2% di bawah harga pasar"
			}
		}
	}

	// For bearish trend - more conservative approach
	if trendScore < 0 {
		// In bearish conditions, use resistance-based entry or current price with discount
		bearishEntry := s.getBearishEntry(currentPrice, resistances, technicalData, atr)

		if bearishEntry > 0 {
			smartEntry = bearishEntry
			entryReason = "Trend turun - entry di dekat resistance dengan diskon kecil"
		} else {
			// Apply bearish discount based on trend strength
			bearishDiscount := math.Min(0.015, math.Abs(float64(trendScore))*0.002) // Max 1.5% discount
			smartEntry = currentPrice * (1 - bearishDiscount)
			entryReason = fmt.Sprintf("Trend turun - entry dengan diskon %.1f%% dari harga pasar", bearishDiscount*100)
		}
	}

	// For sideways market - use range-based entry
	if math.Abs(float64(trendScore)) <= 2 {
		rangeEntry := s.getRangeBasedEntry(currentPrice, supports, resistances, technicalData)
		if rangeEntry > 0 {
			smartEntry = rangeEntry
			entryReason = "Pasar sideways - entry di dekat level support"
		}
	}

	// Apply final safety checks and adjustments
	finalEntry := s.applyFinalEntryAdjustments(smartEntry, currentPrice, marketConditions, volumeConfirmation)

	// Update reason if final adjustments were significant
	if math.Abs(finalEntry-smartEntry)/smartEntry > 0.01 { // More than 1% adjustment
		entryReason += " (disesuaikan untuk keamanan)"
	}

	return EntryResult{Price: finalEntry, Reason: entryReason}
}

// calculateTrendScore analyzes multiple indicators to determine overall trend strength
func (s *tradingService) calculateTrendScore(technicalData *dto.TradingViewScanner) int {
	score := 0

	// Global summary (strongest weight)
	score += technicalData.Recommend.Global.Summary * 3

	// Moving averages trend
	score += technicalData.Recommend.Global.MA * 2

	// Oscillators (for momentum)
	score += technicalData.Recommend.Global.Oscillators

	// MACD trend analysis
	if technicalData.Value.Oscillators.MACD.Macd > technicalData.Value.Oscillators.MACD.Signal {
		score += 1
	} else {
		score -= 1
	}

	// RSI analysis (avoid overbought/oversold)
	rsi := technicalData.Value.Oscillators.RSI
	if rsi > 70 {
		score -= 2 // Overbought
	} else if rsi < 30 {
		score += 1 // Oversold can be opportunity
	}

	return score
}

// findBestSupportEntry finds the best support level for entry
func (s *tradingService) findBestSupportEntry(currentPrice float64, supports []dto.Level, emaData []dto.EMAData) float64 {
	bestEntry := 0.0
	bestScore := 0.0

	// Check support levels
	for _, support := range supports {
		if support.Price < currentPrice && support.Price > currentPrice*0.95 { // Within 5% below current price
			// Score based on touches and proximity
			proximityScore := 1.0 - math.Abs(currentPrice-support.Price)/currentPrice
			totalScore := float64(support.Touches) * proximityScore

			if totalScore > bestScore {
				bestScore = totalScore
				bestEntry = support.Price
			}
		}
	}

	// Check EMA levels as potential support
	for _, ema := range emaData {
		if ema.IsMain { // Only consider main timeframe EMAs
			// Check EMA20 and EMA50 as potential support
			emas := []float64{ema.EMA20, ema.EMA50}
			for _, emaValue := range emas {
				if emaValue > 0 && emaValue < currentPrice && emaValue > currentPrice*0.97 { // Within 3% below
					proximityScore := 1.0 - math.Abs(currentPrice-emaValue)/currentPrice
					totalScore := 5.0 * proximityScore // EMAs get higher base score

					if totalScore > bestScore {
						bestScore = totalScore
						bestEntry = emaValue
					}
				}
			}
		}
	}

	return bestEntry
}

// findMovingAverageEntry finds entry opportunities near key moving averages
func (s *tradingService) findMovingAverageEntry(currentPrice float64, technicalData *dto.TradingViewScanner) float64 {
	// Check if price is near key moving averages
	emas := []float64{
		technicalData.Value.MovingAverages.EMA10,
		technicalData.Value.MovingAverages.EMA20,
		technicalData.Value.MovingAverages.EMA50,
	}

	for _, ema := range emas {
		if ema > 0 {
			// If price is within 1% of EMA and EMA is below current price
			if ema < currentPrice && math.Abs(currentPrice-ema)/currentPrice <= 0.01 {
				return ema
			}
		}
	}

	return 0
}

// EntryOption represents a potential entry point with its score
type EntryOption struct {
	Price  float64
	Type   string
	Score  float64
	Reason string
}

// MarketConditions represents overall market analysis
type MarketConditions struct {
	Trend      string  // "bullish", "bearish", "sideways"
	Volatility float64 // 0-1 scale
	Strength   float64 // 0-10 scale
}

// EntryResult represents the result of smart entry calculation
type EntryResult struct {
	Price  float64
	Reason string
}

// analyzeMarketConditions analyzes overall market conditions
func (s *tradingService) analyzeMarketConditions(technicalData *dto.TradingViewScanner, candles []dto.StockOHLCV) MarketConditions {
	var conditions MarketConditions

	// Determine trend
	globalSummary := technicalData.Recommend.Global.Summary
	if globalSummary > 2 {
		conditions.Trend = "bullish"
	} else if globalSummary < -2 {
		conditions.Trend = "bearish"
	} else {
		conditions.Trend = "sideways"
	}

	// Calculate volatility using recent price movements
	if len(candles) >= 10 {
		var priceChanges []float64
		for i := len(candles) - 9; i < len(candles); i++ {
			change := math.Abs((candles[i].Close - candles[i-1].Close) / candles[i-1].Close)
			priceChanges = append(priceChanges, change)
		}

		var avgChange float64
		for _, change := range priceChanges {
			avgChange += change
		}
		avgChange /= float64(len(priceChanges))

		// Normalize volatility to 0-1 scale
		conditions.Volatility = math.Min(1.0, avgChange*50)
	}

	// Calculate trend strength
	conditions.Strength = math.Abs(float64(globalSummary))

	return conditions
}

// calculateEnhancedTrendScore provides more sophisticated trend analysis
func (s *tradingService) calculateEnhancedTrendScore(technicalData *dto.TradingViewScanner, candles []dto.StockOHLCV) int {
	score := s.calculateTrendScore(technicalData)

	// Add price action confirmation
	if len(candles) >= 3 {
		latest := candles[len(candles)-1]
		previous := candles[len(candles)-2]

		// Higher highs and higher lows for bullish
		if latest.High > previous.High && latest.Low > previous.Low {
			score += 1
		}
		// Lower highs and lower lows for bearish
		if latest.High < previous.High && latest.Low < previous.Low {
			score -= 1
		}
	}

	// Add volume-price relationship
	if len(candles) >= 2 {
		latest := candles[len(candles)-1]
		previous := candles[len(candles)-2]

		priceUp := latest.Close > previous.Close
		volumeUp := latest.Volume > previous.Volume

		// Volume confirms price movement
		if (priceUp && volumeUp) || (!priceUp && volumeUp) {
			if priceUp {
				score += 1
			} else {
				score -= 1
			}
		}
	}

	return score
}

// analyzeVolumeConfirmation analyzes volume patterns for entry confirmation
func (s *tradingService) analyzeVolumeConfirmation(candles []dto.StockOHLCV) float64 {
	if len(candles) < 10 {
		return 0.5 // Neutral if insufficient data
	}

	// Calculate average volume for last 10 periods
	var avgVolume float64
	for i := len(candles) - 10; i < len(candles); i++ {
		avgVolume += float64(candles[i].Volume)
	}
	avgVolume /= 10

	latestVolume := float64(candles[len(candles)-1].Volume)
	volumeRatio := latestVolume / avgVolume

	// Score based on volume relative to average
	if volumeRatio > 1.5 {
		return 0.9 // High volume confirmation
	} else if volumeRatio > 1.2 {
		return 0.7 // Good volume
	} else if volumeRatio > 0.8 {
		return 0.5 // Average volume
	} else {
		return 0.3 // Low volume warning
	}
}

// detectCandlestickPatterns detects bullish/bearish candlestick patterns
func (s *tradingService) detectCandlestickPatterns(candles []dto.StockOHLCV) float64 {
	if len(candles) < 3 {
		return 0
	}

	latest := candles[len(candles)-1]
	previous := candles[len(candles)-2]
	previous2 := candles[len(candles)-3]

	score := 0.0

	// Hammer pattern (bullish)
	body := math.Abs(latest.Close - latest.Open)
	lowerShadow := math.Min(latest.Open, latest.Close) - latest.Low
	upperShadow := latest.High - math.Max(latest.Open, latest.Close)

	if lowerShadow > body*2 && upperShadow < body*0.5 {
		score += 0.3 // Hammer pattern
	}

	// Doji pattern (indecision)
	if body < (latest.High-latest.Low)*0.1 {
		score += 0.1 // Doji - neutral to slightly positive
	}

	// Engulfing patterns
	prevBody := math.Abs(previous.Close - previous.Open)
	if body > prevBody*1.2 {
		if latest.Close > latest.Open && previous.Close < previous.Open {
			score += 0.4 // Bullish engulfing
		} else if latest.Close < latest.Open && previous.Close > previous.Open {
			score -= 0.4 // Bearish engulfing
		}
	}

	// Three consecutive candles pattern
	if latest.Close > previous.Close && previous.Close > previous2.Close {
		score += 0.2 // Three bullish candles
	} else if latest.Close < previous.Close && previous.Close < previous2.Close {
		score -= 0.2 // Three bearish candles
	}

	return score
}

// calculateMomentumScore calculates momentum based on multiple indicators
func (s *tradingService) calculateMomentumScore(technicalData *dto.TradingViewScanner) int {
	score := 0

	// RSI momentum
	rsi := technicalData.Value.Oscillators.RSI
	if rsi > 50 && rsi < 70 {
		score += 2 // Good bullish momentum
	} else if rsi > 70 {
		score += 1 // Overbought but still bullish
	} else if rsi < 50 && rsi > 30 {
		score -= 1 // Bearish momentum
	} else if rsi < 30 {
		score += 1 // Oversold, potential bounce
	}

	// MACD momentum
	macd := technicalData.Value.Oscillators.MACD
	if macd.Macd > macd.Signal {
		score += 2
	} else {
		score -= 1
	}

	// Stochastic momentum (using StochK as approximation)
	stochK := technicalData.Value.Oscillators.StochK
	if stochK > 50 && stochK < 80 {
		score += 1
	} else if stochK < 50 {
		score -= 1
	}

	return score
}

// getBullishEntryOptions gets potential entry options for bullish scenarios
func (s *tradingService) getBullishEntryOptions(currentPrice float64, supports []dto.Level, emaData []dto.EMAData, technicalData *dto.TradingViewScanner, atr float64) []EntryOption {
	var options []EntryOption

	// Market price entry
	options = append(options, EntryOption{
		Price:  currentPrice,
		Type:   "market",
		Score:  5.0,
		Reason: "harga pasar saat ini",
	})

	// Support level entries
	for _, support := range supports {
		if support.Price < currentPrice && support.Price > currentPrice*0.95 {
			proximityScore := 1.0 - math.Abs(currentPrice-support.Price)/currentPrice
			score := float64(support.Touches)*2 + proximityScore*5

			options = append(options, EntryOption{
				Price:  support.Price,
				Type:   "support",
				Score:  score,
				Reason: fmt.Sprintf("level support yang sudah teruji %d kali", support.Touches),
			})
		}
	}

	// EMA entries
	for _, ema := range emaData {
		if ema.IsMain {
			emas := []struct {
				value float64
				name  string
			}{
				{ema.EMA20, "EMA20"},
				{ema.EMA50, "EMA50"},
			}

			for _, e := range emas {
				if e.value > 0 && e.value < currentPrice && e.value > currentPrice*0.97 {
					proximityScore := 1.0 - math.Abs(currentPrice-e.value)/currentPrice
					score := 6.0 + proximityScore*4

					options = append(options, EntryOption{
						Price:  e.value,
						Type:   "ema",
						Score:  score,
						Reason: fmt.Sprintf("%s sebagai support dinamis", e.name),
					})
				}
			}
		}
	}

	// ATR-based pullback entry
	atrEntry := currentPrice - (atr * 0.5)
	if atrEntry > currentPrice*0.97 {
		options = append(options, EntryOption{
			Price:  atrEntry,
			Type:   "atr_pullback",
			Score:  4.0,
			Reason: "entry saat pullback berdasarkan volatilitas (ATR)",
		})
	}

	return options
}

// selectBestEntry selects the best entry option based on multiple factors
func (s *tradingService) selectBestEntry(options []EntryOption, volumeConfirmation, candlestickSignal float64, momentumScore int, conditions MarketConditions) EntryOption {
	if len(options) == 0 {
		return EntryOption{}
	}

	bestOption := options[0]
	bestScore := 0.0

	for _, option := range options {
		// Base score from option
		score := option.Score

		// Volume confirmation bonus
		score *= volumeConfirmation

		// Candlestick pattern bonus
		if candlestickSignal > 0 {
			score += candlestickSignal * 2
		} else {
			score += candlestickSignal * 1
		}

		// Momentum bonus
		if momentumScore > 0 {
			score += float64(momentumScore) * 0.5
		} else {
			score += float64(momentumScore) * 0.3
		}

		// Market conditions adjustment
		if conditions.Trend == "bullish" {
			score += 1.0
		} else if conditions.Trend == "bearish" {
			score -= 1.0
		}

		// Volatility adjustment
		if conditions.Volatility > 0.7 {
			score -= 1.0 // Penalize high volatility
		} else if conditions.Volatility < 0.3 {
			score += 0.5 // Bonus for low volatility
		}

		if score > bestScore {
			bestScore = score
			bestOption = option
		}
	}

	return bestOption
}

// calculateVolatilityAdjustment calculates entry adjustment based on volatility
func (s *tradingService) calculateVolatilityAdjustment(atr, volatility float64) float64 {
	// Base adjustment from ATR (normalized)
	adjustment := (atr / 100) * 0.01 // Assume price around 100 for normalization

	// Scale by volatility
	adjustment *= volatility

	// Cap the adjustment
	return math.Min(0.02, adjustment) // Max 2% adjustment
}

// getBearishEntry gets entry for bearish scenarios
func (s *tradingService) getBearishEntry(currentPrice float64, resistances []dto.Level, technicalData *dto.TradingViewScanner, atr float64) float64 {
	// In bearish trend, look for resistance levels to short or reduce position
	for _, resistance := range resistances {
		if resistance.Price > currentPrice && resistance.Price < currentPrice*1.03 {
			// Use resistance level with small discount
			return resistance.Price * 0.998
		}
	}

	// Fallback: use current price with ATR-based discount
	return currentPrice - (atr * 0.3)
}

// getRangeBasedEntry gets entry for sideways market
func (s *tradingService) getRangeBasedEntry(currentPrice float64, supports []dto.Level, resistances []dto.Level, technicalData *dto.TradingViewScanner) float64 {
	// In sideways market, buy near support, sell near resistance
	var nearestSupport, nearestResistance float64

	// Find nearest support below current price
	for _, support := range supports {
		if support.Price < currentPrice {
			if nearestSupport == 0 || support.Price > nearestSupport {
				nearestSupport = support.Price
			}
		}
	}

	// Find nearest resistance above current price
	for _, resistance := range resistances {
		if resistance.Price > currentPrice {
			if nearestResistance == 0 || resistance.Price < nearestResistance {
				nearestResistance = resistance.Price
			}
		}
	}

	// If we're closer to support, use support + small buffer
	if nearestSupport > 0 && nearestResistance > 0 {
		supportDistance := currentPrice - nearestSupport
		resistanceDistance := nearestResistance - currentPrice

		if supportDistance < resistanceDistance {
			return nearestSupport * 1.005 // 0.5% above support
		}
	}

	return 0 // No suitable range entry found
}

// applyFinalEntryAdjustments applies final safety checks and adjustments
func (s *tradingService) applyFinalEntryAdjustments(smartEntry, currentPrice float64, conditions MarketConditions, volumeConfirmation float64) float64 {
	// Ensure entry is not too far from current price
	maxDeviation := 0.05 // 5% max deviation
	if conditions.Volatility > 0.7 {
		maxDeviation = 0.03 // Reduce to 3% in high volatility
	}

	if math.Abs(smartEntry-currentPrice)/currentPrice > maxDeviation {
		if smartEntry < currentPrice {
			smartEntry = currentPrice * (1 - maxDeviation)
		} else {
			smartEntry = currentPrice * (1 + maxDeviation)
		}
	}

	// Volume confirmation adjustment
	if volumeConfirmation < 0.4 {
		// Low volume - be more conservative
		if smartEntry < currentPrice {
			smartEntry = (smartEntry + currentPrice) / 2 // Move closer to current price
		}
	}

	// Final sanity check
	if smartEntry <= 0 {
		smartEntry = currentPrice
	}

	return smartEntry
}

func (s *tradingService) CalculateSummary(ctx context.Context, dtf []dto.DataTimeframe, latestAnalyses []model.StockAnalysis) (int, error) {
	if len(latestAnalyses) == 0 {
		return 0, fmt.Errorf("cannot calculate summary without analysis data")
	}

	mapWeight := make(map[string]int)
	totalWeight := 0
	for _, tf := range dtf {
		mapWeight[tf.Interval] = tf.Weight
		totalWeight += tf.Weight
	}

	if totalWeight == 0 {
		return 0, fmt.Errorf("total weight for timeframes is zero")
	}

	var weightedScoreSum float64
	for _, analysis := range latestAnalyses {
		weight, ok := mapWeight[analysis.Timeframe]
		if !ok {
			s.log.WarnContext(ctx, "Unknown timeframe in analysis, skipping", logger.StringField("timeframe", analysis.Timeframe))
			continue
		}

		var technicalData dto.TradingViewScanner
		if err := json.Unmarshal([]byte(analysis.TechnicalData), &technicalData); err != nil {
			s.log.ErrorContext(ctx, "Failed to unmarshal technical data", logger.ErrorField(err), logger.StringField("timeframe", analysis.Timeframe))
			continue // Skip this analysis if data is corrupted
		}

		score := technicalData.Recommend.Global.Summary
		weightedScoreSum += float64(weight) * float64(score)
	}

	const scale = 25.0
	weightedAverage := weightedScoreSum / float64(totalWeight)
	finalScore := int((weightedAverage + 2) * scale)
	return finalScore, nil
}
