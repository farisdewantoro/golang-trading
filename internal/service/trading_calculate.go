package service

import (
	"golang-trading/internal/dto"
	"sort"
)

const (
	minTouches           = 1
	slFromEMAAdj         = 0.995
	targetRiskReward     = 2.0
	maxStopLossPercent   = 0.05
	minStopLossPercent   = 0.02
	maxTakeProfitPercent = 0.07
	minTakeProfitPercent = 0.02
)

type SLSource struct {
	Price  float64
	Type   string
	Reason string
}

type TPSource struct {
	Price  float64
	Type   string
	Reason string
}

func getSLCandidates(marketPrice float64, supports []dto.Level, emas []dto.EMAData, priceBuckets []dto.PriceBucket) []SLSource {
	var result []SLSource

	// 1. SL dari support dengan touches terbanyak
	var bestSupport []dto.Level
	for _, s := range supports {
		if s.Price < marketPrice && s.Touches >= minTouches {
			bestSupport = append(bestSupport, s)
		}
	}

	sort.Slice(bestSupport, func(i, j int) bool {
		return bestSupport[i].Touches > bestSupport[j].Touches
	})

	if len(bestSupport) > 0 {
		max := 3
		for idx, s := range bestSupport {
			if idx+1 >= max {
				break
			}
			result = append(result, SLSource{
				Price:  s.Price,
				Type:   "SL_SUPPORT",
				Reason: "support dengan touches terbanyak",
			})
		}
	}

	// 2. SL support terdekat
	var closestSupport []dto.Level
	for _, s := range supports {
		if len(bestSupport) == 0 {
			break
		}

		for _, bs := range bestSupport {
			if s.Price < marketPrice && s.Price > bs.Price && s.Touches >= minTouches {
				closestSupport = append(closestSupport, s)
			}
		}
	}

	sort.Slice(closestSupport, func(i, j int) bool {
		return closestSupport[i].Touches > closestSupport[j].Touches
	})

	if len(closestSupport) > 0 {
		max := 3
		for idx, s := range closestSupport {
			if idx+1 >= max {
				break
			}
			result = append(result, SLSource{
				Price:  s.Price,
				Type:   "SL_SUPPORT_NEAR",
				Reason: "support lebih dekat dengan market",
			})
		}
	}

	// 3. SL dari EMA20, EMA50, EMA10
	highEMA := []float64{}
	for _, ema := range emas {
		if !ema.IsMain {
			continue
		}
		if ema.EMA20 > 0 {
			valEMA20 := ema.EMA20 * slFromEMAAdj
			result = append(result, SLSource{
				Price:  valEMA20,
				Type:   "SL_EMA20",
				Reason: "0.5% di bawah EMA20",
			})
			highEMA = append(highEMA, valEMA20)
		}
		if ema.EMA50 > 0 {
			valEMA50 := ema.EMA50 * slFromEMAAdj
			result = append(result, SLSource{
				Price:  valEMA50,
				Type:   "SL_EMA50",
				Reason: "0.5% di bawah EMA50",
			})
			highEMA = append(highEMA, valEMA50)
		}

		if ema.EMA10 > 0 {
			valEMA10 := ema.EMA10 * slFromEMAAdj
			result = append(result, SLSource{
				Price:  valEMA10,
				Type:   "SL_EMA10",
				Reason: "0.5% di bawah EMA10",
			})
			highEMA = append(highEMA, valEMA10)
		}
	}
	sort.Slice(highEMA, func(i, j int) bool {
		return highEMA[i] > highEMA[j]
	})
	// 4. SL dari price bucket (di atas EMA dan di bawah market)
	var bestPriceBucket []dto.PriceBucket
	for _, pb := range priceBuckets {
		if pb.Bucket < marketPrice && (len(bestSupport) == 0 || (pb.Bucket > bestSupport[0].Price && (len(highEMA) == 0 || pb.Bucket > highEMA[0]))) {
			bestPriceBucket = append(bestPriceBucket, pb)
		}
	}

	sort.Slice(bestPriceBucket, func(i, j int) bool {
		return bestPriceBucket[i].Count > bestPriceBucket[j].Count
	})
	if len(bestPriceBucket) > 0 {
		max := 5
		for idx, pb := range bestPriceBucket {
			if idx+1 >= max {
				break
			}
			result = append(result, SLSource{
				Price:  pb.Bucket,
				Type:   "SL_BUCKET",
				Reason: "level konsolidasi harga dari bucket",
			})
		}
	}

	return result
}

func getTPCandidates(marketPrice float64, resistances []dto.Level, priceBuckets []dto.PriceBucket) []TPSource {
	var result []TPSource

	// 1. Resistance dengan touches terbanyak
	var bestResistance []dto.Level
	for _, r := range resistances {
		if r.Price > marketPrice && r.Touches > 0 {
			bestResistance = append(bestResistance, r)
		}
	}

	sort.Slice(bestResistance, func(i, j int) bool {
		return bestResistance[i].Touches > bestResistance[j].Touches
	})
	if len(bestResistance) > 0 {
		max := 3
		for idx, r := range bestResistance {
			if idx+1 >= max {
				break
			}
			result = append(result, TPSource{
				Price:  r.Price,
				Type:   "TP_RESISTANCE",
				Reason: "resistance dengan touches terbanyak",
			})
		}
	}

	// 2. Ambil 2 resistance atas dan cari nilai tengah
	var higherRes []float64
	for _, r := range resistances {
		if len(bestResistance) == 0 {
			break
		}
		if r.Price > bestResistance[0].Price {
			higherRes = append(higherRes, r.Price)
		}
	}
	sort.Slice(higherRes, func(i, j int) bool {
		return higherRes[i] > higherRes[j]
	})

	if len(higherRes) >= 2 {
		avg := (higherRes[0] + higherRes[1]) / 2
		result = append(result, TPSource{
			Price:  avg,
			Type:   "TP_MID_RESISTANCE",
			Reason: "nilai tengah 2 resistance terdekat",
		})
	} else if len(higherRes) == 1 {
		result = append(result, TPSource{
			Price:  higherRes[0],
			Type:   "TP_HIGHER_RESISTANCE",
			Reason: "resistance di atas resistance utama",
		})
	}

	// 3. Bucket atas marketPrice dan sedikit di atas resistance
	var bestBucket []dto.PriceBucket
	for _, pb := range priceBuckets {
		if pb.Bucket > marketPrice && (len(higherRes) == 0 || pb.Bucket > higherRes[0]) {
			bestBucket = append(bestBucket, pb)
		}
	}

	sort.Slice(bestBucket, func(i, j int) bool {
		return bestBucket[i].Count > bestBucket[j].Count
	})
	if len(bestBucket) > 0 {
		max := 5
		for idx, pb := range bestBucket {
			if idx+1 >= max {
				break
			}
			result = append(result, TPSource{
				Price:  pb.Bucket,
				Type:   "TP_BUCKET",
				Reason: "level konsolidasi atas dari bucket",
			})
		}
	}

	return result
}

func (s *tradingService) calculatePlan(
	marketPrice float64,
	supports []dto.Level,
	resistances []dto.Level,
	emas []dto.EMAData,
	priceBuckets []dto.PriceBucket,
) dto.TradePlan {
	slCandidates := getSLCandidates(marketPrice, supports, emas, priceBuckets)
	tpCandidates := getTPCandidates(marketPrice, resistances, priceBuckets)

	var tradePlans []dto.TradePlan

	for _, sl := range slCandidates {
		risk := marketPrice - sl.Price
		if risk <= 0 {
			continue
		}

		riskPct := risk / marketPrice
		if riskPct > maxStopLossPercent || riskPct < minStopLossPercent {
			continue
		}

		for _, tp := range tpCandidates {
			reward := tp.Price - marketPrice
			if reward <= 0 {
				continue
			}

			rewardPct := reward / marketPrice
			if rewardPct > maxTakeProfitPercent || rewardPct < minTakeProfitPercent {
				continue
			}

			// Correct Risk Reward Ratio
			riskReward := reward / risk

			tradePlans = append(tradePlans, dto.TradePlan{
				Entry:      marketPrice,
				StopLoss:   sl.Price,
				TakeProfit: tp.Price,
				Risk:       risk,
				Reward:     reward,
				RiskReward: riskReward,
				SLType:     sl.Type,
				SLReason:   sl.Reason,
				TPType:     tp.Type,
				TPReason:   tp.Reason,
			})
		}
	}

	if len(tradePlans) == 0 {
		return dto.TradePlan{}
	}

	// Prioritizing highest Risk Reward Ratio
	sort.SliceStable(tradePlans, func(i, j int) bool {
		return tradePlans[i].RiskReward > tradePlans[j].RiskReward
	})

	return tradePlans[0]
}
