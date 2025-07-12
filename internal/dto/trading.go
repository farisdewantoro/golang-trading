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

type Level struct {
	Price     float64
	Timeframe string // "1D", "4H", "1H"
	Touches   int    // seberapa sering level disentuh (opsional, default 0)
	Type      string // "Classic", "Fibonacci"
}

type TimeframePivot struct {
	Timeframe string
	PivotData []PivotData
}

type PivotData struct {
	Support    []Level
	Resistance []Level
	Type       string
}

type EMAData struct {
	Timeframe string
	EMA10     float64
	EMA20     float64
	EMA50     float64
	IsMain    bool
}

type PriceBucket struct {
	Bucket float64 `json:"bucket"`
	Count  int     `json:"count"`
}
