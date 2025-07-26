package dto

type Evaluation string

const (
	EvalVeryStrong Evaluation = "Sangat Kuat & Potensi Naik"
	EvalStrong     Evaluation = "Cukup Kuat tapi Perlu Waspada"
	EvalNeutral    Evaluation = "Netral / Lemah"
	EvalVeryWeak   Evaluation = "Sangat Lemah / Potensi Breakdown"
	EvalWeak       Evaluation = "Lemah / Tidak Stabil"
)

const (
	TradingViewInterval1Min   string = "1"   // 1 minute
	TradingViewInterval5Min   string = "5"   // 5 minutes
	TradingViewInterval15Min  string = "15"  // 15 minutes
	TradingViewInterval30Min  string = "30"  // 30 minutes
	TradingViewInterval1Hour  string = "60"  // 1 hour
	TradingViewInterval2Hour  string = "120" // 2 hours
	TradingViewInterval4Hour  string = "240" // 4 hours
	TradingViewInterval1Day   string = "1D"  // 1 day
	TradingViewInterval1Week  string = "1W"  // 1 week
	TradingViewInterval1Month string = "1M"  // 1 month

	TradingViewSignalStrongBuy  int = 2  // STRONG_BUY
	TradingViewSignalBuy        int = 1  // BUY
	TradingViewSignalNeutral    int = 0  // NEUTRAL
	TradingViewSignalSell       int = -1 // SELL
	TradingViewSignalStrongSell int = -2 // STRONG_SELL

	Interval30Min string = "30m"
	Interval1Hour string = "1h"
	Interval4Hour string = "4h"
	Interval1Day  string = "1d"
	Interval1Week string = "1w"

	SignalStrongBuy  = "STRONG_BUY"
	SignalBuy        = "BUY"
	SignalNeutral    = "NEUTRAL"
	SignalSell       = "SELL"
	SignalStrongSell = "STRONG_SELL"
	SignalHold       = "HOLD"

	TrendBullish string = "Bullish"
	TrendBearish string = "Bearish"
	TrendNeutral string = "Neutral"

	OverBought string = "OverBought"
	OverSold   string = "OverSold"
	Normal     string = "Normal"

	LevelClassic   = "Classic"
	LevelFibonacci = "Fibonacci"
)

type PositionStatus string

const (
	Safe      PositionStatus = "safe"
	Warning   PositionStatus = "warning"
	Dangerous PositionStatus = "dangerous"
)

func (p PositionStatus) String() string {
	switch p {
	case Safe:
		return "✅ Safe"
	case Warning:
		return "⚠️ Warning"
	case Dangerous:
		return "❌ Dangerous"
	default:
		return "Unknown"
	}
}

type Signal string

const (
	TakeProfit     Signal = "take_profit"
	CutLoss        Signal = "cut_loss"
	TrailingStop   Signal = "trailing_stop"
	TrailingProfit Signal = "trailing_profit"
	Hold           Signal = "hold"
)

func (s Signal) String() string {
	switch s {
	case TakeProfit:
		return "🟢 Take Profit"
	case CutLoss:
		return "🔴 Cut Loss"
	case TrailingStop:
		return "🟠 Trailing Stop"
	case TrailingProfit:
		return "🟠 Trailing Profit"
	case Hold:
		return "🟡 Hold"
	default:
		return "Unknown"
	}
}

func TASignalText(signal string) string {
	switch signal {
	case SignalStrongBuy:
		return "🟢 Strong Buy"
	case SignalBuy:
		return "🟢 Buy"
	case SignalNeutral:
		return "🟡 Neutral"
	case SignalSell:
		return "🔴 Sell"
	case SignalStrongSell:
		return "🔴 Strong Sell"
	default:
		return "⚪ Unknown"
	}
}
