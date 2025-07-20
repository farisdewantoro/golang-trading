package dto

import "golang-trading/internal/model"

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
	TechnicalSignal    string
	Exchange           string
	PlanType           PlanType

	SLType   string // jenis SL: support / ema-adjust
	SLReason string // alasan SL

	TPType           string // jenis TP: resistance / price-bucket / avg-resistance
	TPReason         string // alasan TP
	IndicatorSummary model.IndicatorSummary
	Insights         []string
}

type TradePlan struct {
	CurrentMarketPrice float64
	Entry              float64
	StopLoss           float64
	TakeProfit         float64
	Risk               float64
	Reward             float64
	RiskReward         float64
	Score              float64
	PlanType           PlanType

	SLType   string // jenis SL: support / ema-adjust
	SLReason string // alasan SL

	TPType   string // jenis TP: resistance / price-bucket / avg-resistance
	TPReason string // alasan TP
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

type PositionAnalysis struct {
	Ticker               string
	EntryPrice           float64
	LastPrice            float64
	TakeProfitPrice      float64
	StopLossPrice        float64
	Status               PositionStatus
	Signal               Signal
	Insight              []string
	Score                float64
	TechnicalSignal      string
	IndicatorSummary     model.IndicatorSummary
	TrailingStopPrice    float64
	TrailingProfitPrice  float64
	HighestPriceSinceTTP float64
}

type MainAnalysisData struct {
	MainTA             *TradingViewScanner
	MainOHLCV          []StockOHLCV
	MainTimeframe      string
	SecondaryTA        *TradingViewScanner
	SecondaryOHLCV     []StockOHLCV
	SecondaryTimeframe string
}

type TradeConfig struct {
	TargetRiskReward     float64
	MaxStopLossPercent   float64
	MinStopLossPercent   float64
	MaxTakeProfitPercent float64
	MinTakeProfitPercent float64
	Type                 PlanType
	Score                float64
}

type PlanType string

const (
	PlanTypePrimary   PlanType = "PRIMARY"
	PlanTypeSecondary PlanType = "SECONDARY"
	PlanTypeFallback  PlanType = "FALLBACK"
)

func (pt PlanType) String() string {
	switch pt {
	case PlanTypePrimary:
		return "🥇 Primary"
	case PlanTypeSecondary:
		return "🥈 Secondary"
	case PlanTypeFallback:
		return "🚨 Fallback"
	default:
		return "❓ Unknown"
	}
}
