package dto

type DataTimeframe struct {
	Interval string `json:"interval"`
	Range    string `json:"range"`
}

func (d *DataTimeframe) ToTradingViewScreenersInterval() string {
	switch d.Interval {
	case Interval30Min:
		return TradingViewInterval30Min
	case Interval1Hour:
		return TradingViewInterval1Hour
	case Interval4Hour:
		return TradingViewInterval4Hour
	case Interval1Day:
		return TradingViewInterval1Day
	case Interval1Week:
		return TradingViewInterval1Week
	default:
		return TradingViewInterval1Day
	}
}

func MapTradingViewScreenerRecommend(val int) string {
	switch val {
	case TradingViewSignalStrongBuy:
		return SignalBuy
	case TradingViewSignalBuy:
		return SignalBuy
	case TradingViewSignalNeutral:
		return SignalNeutral
	case TradingViewSignalSell:
		return SignalSell
	case TradingViewSignalStrongSell:
		return SignalStrongSell
	default:
		return "NOT_FOUND"
	}
}

type TradePlan struct {
	Entry      float64
	StopLoss   float64
	TakeProfit float64
	Risk       float64
	Reward     float64
	RiskReward float64
	IsTPValid  bool
}

// HitungTPnSL menghitung SL dan TP berdasarkan RR dan level support/resistance
func CreateTradePlan(entry float64, supports []float64, resistances []float64, riskReward float64) TradePlan {
	// Cari support terdekat di bawah entry
	var nearestSupport float64 = 0
	for _, s := range supports {
		if s < entry && s > nearestSupport {
			nearestSupport = s
		}
	}

	// Jika tidak ditemukan support valid
	if nearestSupport == 0 {
		return TradePlan{}
	}

	risk := entry - nearestSupport
	reward := risk * riskReward
	tp := entry + reward

	// Cari resistance terdekat di atas TP
	isTPValid := false
	for _, r := range resistances {
		if tp <= r {
			isTPValid = true
			break
		}
	}

	return TradePlan{
		Entry:      entry,
		StopLoss:   nearestSupport,
		TakeProfit: tp,
		Risk:       risk,
		Reward:     reward,
		RiskReward: reward / risk,
		IsTPValid:  isTPValid,
	}
}
