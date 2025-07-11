package dto

type TradePlanResult struct {
	CurrentMarketPrice float64
	Entry              float64
	StopLoss           float64
	TakeProfit         float64
	RiskReward         float64
	Score              float64
	Confidence         float64
	Status             string
	IsBuySignal        bool
	Symbol             string
}

type TradePlan struct {
	CurrentMarketPrice float64
	Entry              float64
	StopLoss           float64
	TakeProfit         float64
	Risk               float64
	Reward             float64
	RiskReward         float64
	IsTPValid          bool // TP realistis dan RR valid
	IsTPIdeal          bool // TP berasal dari resistance ideal
	Confidence         int  // Skor 1–3 (1 = low, 3 = high)
}
