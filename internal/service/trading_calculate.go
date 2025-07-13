package service

import (
	"golang-trading/internal/dto"
	"sort"
)

const (
	minTouches         = 1
	slFromEMAAdj       = 0.995
	targetRiskReward   = 2.0
	maxStopLossPercent = 0.05
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
	var bestSupport dto.Level
	for _, s := range supports {
		if s.Price < marketPrice && s.Touches >= minTouches {
			if bestSupport.Price == 0 || s.Touches > bestSupport.Touches {
				bestSupport = s
			}
		}
	}
	if bestSupport.Price > 0 {
		result = append(result, SLSource{
			Price:  bestSupport.Price,
			Type:   "SL_SUPPORT",
			Reason: "support dengan touches terbanyak",
		})
	}

	// 2. SL support terdekat
	var closestSupport dto.Level
	for _, s := range supports {
		if s.Price < marketPrice && s.Price > bestSupport.Price && s.Touches >= minTouches {
			if closestSupport.Price == 0 || s.Price > closestSupport.Price {
				closestSupport = s
			}
		}
	}
	if closestSupport.Price > 0 {
		result = append(result, SLSource{
			Price:  closestSupport.Price,
			Type:   "SL_SUPPORT_NEAR",
			Reason: "support lebih dekat dengan market",
		})
	}

	// 3. SL dari EMA20 dan EMA50
	for _, ema := range emas {
		if !ema.IsMain {
			continue
		}
		if ema.EMA20 > 0 {
			result = append(result, SLSource{
				Price:  ema.EMA20 * slFromEMAAdj,
				Type:   "SL_EMA20",
				Reason: "0.5% di bawah EMA20",
			})
		}
		if ema.EMA50 > 0 {
			result = append(result, SLSource{
				Price:  ema.EMA50 * slFromEMAAdj,
				Type:   "SL_EMA50",
				Reason: "0.5% di bawah EMA50",
			})
		}
	}

	// 4. SL dari price bucket (di atas EMA dan di bawah market)
	for _, pb := range priceBuckets {
		if pb.Bucket < marketPrice && (bestSupport.Price == 0 || pb.Bucket > bestSupport.Price) {
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
	var bestResistance dto.Level
	for _, r := range resistances {
		if r.Price > marketPrice {
			if bestResistance.Price == 0 || r.Touches > bestResistance.Touches {
				bestResistance = r
			}
		}
	}
	if bestResistance.Price > 0 {
		result = append(result, TPSource{
			Price:  bestResistance.Price,
			Type:   "TP_RESISTANCE",
			Reason: "resistance dengan touches terbanyak",
		})
	}

	// 2. Bucket atas marketPrice
	var bestBucket dto.PriceBucket
	for _, pb := range priceBuckets {
		if pb.Bucket > marketPrice && pb.Count > bestBucket.Count {
			bestBucket = pb
		}
	}
	if bestBucket.Bucket > 0 {
		result = append(result, TPSource{
			Price:  bestBucket.Bucket,
			Type:   "TP_BUCKET",
			Reason: "level konsolidasi atas dari bucket",
		})
	}

	// 3. Ambil 2 resistance atas dan cari nilai tengah
	var higherRes []float64
	for _, r := range resistances {
		if r.Price > bestResistance.Price {
			higherRes = append(higherRes, r.Price)
		}
	}
	sort.Float64s(higherRes)
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

	// Cari kombinasi SL/TP terbaik
	for _, sl := range slCandidates {
		risk := marketPrice - sl.Price
		riskPct := risk / marketPrice
		if risk <= 0 || riskPct > maxStopLossPercent {
			continue
		}
		for _, tp := range tpCandidates {
			reward := tp.Price - marketPrice
			if reward <= 0 {
				continue
			}
			rr := reward / risk
			if rr >= 1.0 { // minimal acceptable RR
				return dto.TradePlan{
					Entry:      marketPrice,
					StopLoss:   sl.Price,
					TakeProfit: tp.Price,
					Risk:       risk,
					Reward:     reward,
					RiskReward: rr,
					SLType:     sl.Type,
					SLReason:   sl.Reason,
					TPType:     tp.Type,
					TPReason:   tp.Reason,
				}
			}
		}
	}

	// Jika tidak ada kombinasi valid
	return dto.TradePlan{}
}
