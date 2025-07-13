package dto

// TradingView Payload Data
type TradingViewScanner struct {
	Recommend struct {
		Global struct {
			Summary     int // Summary recommendation
			Oscillators int // Oscillators recommendation
			MA          int // Moving Averages recommendation
		}
		Oscillators struct {
			RSI      int // Relative Strength Index (14)
			StochK   int // Stochastic %K (14, 3, 3)
			CCI      int // Commodity Channel Index (20)
			ADX      int // Average Directional Index (14)
			AO       int // Awesome Oscillator
			Mom      int // Momentum (10)
			MACD     int // MACD Level (12, 26)
			StochRSI int // Stochastic RSI Fast (3, 3, 14, 14)
			WR       int // Williams Percent Range (14)
			BBP      int // Bull Bear Power
			UO       int // Ultimate Oscillator (7, 14, 28)
		}
		MovingAverages struct {
			EMA10    int // Exponential Moving Average (EMA10)
			SMA10    int // Simple Moving Average (SMA10)
			EMA20    int // Exponential Moving Average (EMA20)
			SMA20    int // Simple Moving Average (SMA20)
			EMA30    int // Exponential Moving Average (EMA30)
			SMA30    int // Simple Moving Average (SMA30)
			EMA50    int // Exponential Moving Average (EMA50)
			SMA50    int // Simple Moving Average (SMA50)
			EMA100   int // Exponential Moving Average (EMA100)
			SMA100   int // Simple Moving Average (SMA100)
			EMA200   int // Exponential Moving Average (EMA200)
			SMA200   int // Simple Moving Average (SMA200)
			Ichimoku int // Ichimoku Base Line (9, 26, 52, 26)
			VWMA     int // Volume Weighted Moving Average (20)
			HullMA   int // Hull Moving Average (HullMA9)
		}
	}
	Value struct {
		Global struct {
			Summary     float64 // Summary recommendation
			Oscillators float64 // Oscillators recommendation
			MA          float64 // Moving Averages recommendation
		}
		Oscillators struct {
			RSI    float64  // Relative Strength Index (14)
			StochK float64  // Stochastic %K (14, 3, 3)
			CCI    float64  // Commodity Channel Index (20)
			ADX    struct { // Average Directional Index (14)
				Value    float64 // ADX Value
				PlusDI   float64 // ADX+DI
				MinusDI  float64 // ADX-DI
				PlusDI1  float64 // ADX+DI[1]
				MinusDI1 float64 // ADX-DI[1]
			}
			AO struct { // Awesome Oscillator
				Value float64 // AO current value
				Prev1 float64 // AO[1]
				Prev2 float64 // AO[2]
			}
			Mom  float64  // Momentum (10)
			MACD struct { // MACD Level (12, 26)
				Macd   float64 // MACD line
				Signal float64 // Signal line
			}
			StochRSI float64 // Stochastic RSI Fast (3, 3, 14, 14)
			WR       float64 // Williams Percent Range (14)
			BBP      float64 // Bull Bear Power
			UO       float64 // Ultimate Oscillator (7, 14, 28)
		}
		MovingAverages struct {
			EMA10    float64 // Exponential Moving Average (EMA10)
			SMA10    float64 // Simple Moving Average (SMA10)
			EMA20    float64 // Exponential Moving Average (EMA20)
			SMA20    float64 // Simple Moving Average (SMA20)
			EMA30    float64 // Exponential Moving Average (EMA30)
			SMA30    float64 // Simple Moving Average (SMA30)
			EMA50    float64 // Exponential Moving Average (EMA50)
			SMA50    float64 // Simple Moving Average (SMA50)
			EMA100   float64 // Exponential Moving Average (EMA100)
			SMA100   float64 // Simple Moving Average (SMA100)
			EMA200   float64 // Exponential Moving Average (EMA200)
			SMA200   float64 // Simple Moving Average (SMA200)
			Ichimoku float64 // Ichimoku Base Line (9, 26, 52, 26)
			VWMA     float64 // Volume Weighted Moving Average (20)
			HullMA   float64 // Hull Moving Average (HullMA9)
		}
		Pivots struct {
			Classic struct {
				Middle float64 // Classic Pivot Middle (Pivot.M.Classic.Middle)
				R1     float64 // Resistance 1 (Pivot.M.Classic.R1)
				R2     float64 // Resistance 2 (Pivot.M.Classic.R2)
				R3     float64 // Resistance 3 (Pivot.M.Classic.R3)
				S1     float64 // Support 1 (Pivot.M.Classic.S1)
				S2     float64 // Support 2 (Pivot.M.Classic.S2)
				S3     float64 // Support 3 (Pivot.M.Classic.S3)
			}
			Fibonacci struct {
				Middle float64 // Fibonacci Pivot Middle (Pivot.M.Fibonacci.Middle)
				R1     float64 // Resistance 1 (Pivot.M.Fibonacci.R1)
				R2     float64 // Resistance 2 (Pivot.M.Fibonacci.R2)
				R3     float64 // Resistance 3 (Pivot.M.Fibonacci.R3)
				S1     float64 // Support 1 (Pivot.M.Fibonacci.S1)
				S2     float64 // Support 2 (Pivot.M.Fibonacci.S2)
				S3     float64 // Support 3 (Pivot.M.Fibonacci.S3)
			}
			Camarilla struct {
				Middle float64 // Camarilla Pivot Middle (Pivot.M.Camarilla.Middle)
				R1     float64 // Resistance 1 (Pivot.M.Camarilla.R1)
				R2     float64 // Resistance 2 (Pivot.M.Camarilla.R2)
				R3     float64 // Resistance 3 (Pivot.M.Camarilla.R3)
				S1     float64 // Support 1 (Pivot.M.Camarilla.S1)
				S2     float64 // Support 2 (Pivot.M.Camarilla.S2)
				S3     float64 // Support 3 (Pivot.M.Camarilla.S3)
			}
			Woodie struct {
				Middle float64 // Woodie Pivot Middle (Pivot.M.Woodie.Middle)
				R1     float64 // Resistance 1 (Pivot.M.Woodie.R1)
				R2     float64 // Resistance 2 (Pivot.M.Woodie.R2)
				R3     float64 // Resistance 3 (Pivot.M.Woodie.R3)
				S1     float64 // Support 1 (Pivot.M.Woodie.S1)
				S2     float64 // Support 2 (Pivot.M.Woodie.S2)
				S3     float64 // Support 3 (Pivot.M.Woodie.S3)
			}
			Demark struct {
				Middle float64 // Demark Pivot Middle (Pivot.M.Demark.Middle)
				R1     float64 // Resistance 1 (Pivot.M.Demark.R1)
				S1     float64 // Support 1 (Pivot.M.Demark.S1)
			}
		}
		Prices struct {
			Close float64 // Closing price
			High  float64 // Highest price
			Low   float64 // Lowest price
		}
	}
}

func (s *TradingViewScanner) GetTrendMACD() string {
	if s.Recommend.Oscillators.MACD == TradingViewSignalBuy {
		return TrendBullish
	}
	if s.Recommend.Oscillators.MACD == TradingViewSignalSell {
		return TrendBearish
	}
	return TrendNeutral
}

func GetSignalText(signal int) string {
	switch signal {
	case TradingViewSignalStrongBuy:
		return "Strong Buy"
	case TradingViewSignalBuy:
		return "Buy"
	case TradingViewSignalSell:
		return "Sell"
	case TradingViewSignalStrongSell:
		return "Strong Sell"
	case TradingViewSignalNeutral:
		return "Neutral"
	default:
		return "Unknown"
	}
}

func GetRSIStatus(rsi int) string {
	switch {
	case rsi < 30:
		return "Oversold"
	case rsi > 70:
		return "Overbought"
	default:
		return "Normal"
	}
}

type TradingViewBuyListResponse struct {
	TotalCount int                              `json:"totalCount"`
	Data       []TradingViewBuyListDataResponse `json:"data"`
}

type TradingViewBuyListDataResponse struct {
	// assume column only :
	//   "columns": [
	//       "Recommend.All"
	//   ],
	StockCode       string    `json:"s"`
	TechnicalRating []float64 `json:"d"`
}


