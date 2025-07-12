package dto

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

	EvalVeryStrong = "✅ Sangat Kuat & Potensi Naik"
	EvalStrong     = "⚠️ Cukup Kuat tapi Perlu Waspada"
	EvalNeutral    = "⚠️ Netral / Lemah"
	EvalVeryWeak   = "❌ Sangat Lemah / Potensi Breakdown"
	EvalWeak       = "❌ Lemah / Tidak Stabil"

	TrendBullish string = "Bullish"
	TrendBearish string = "Bearish"
	TrendNeutral string = "Neutral"

	OverBought string = "OverBought"
	OverSold   string = "OverSold"
	Normal     string = "Normal"

	LevelClassic   = "Classic"
	LevelFibonacci = "Fibonacci"
)
