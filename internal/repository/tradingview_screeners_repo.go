package repository

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/pkg/httpclient"
	"golang-trading/pkg/logger"

	"golang.org/x/time/rate"
)

type TradingViewScreenersRepository interface {
	Get(ctx context.Context, symbol string, interval string) (*dto.TradingViewScanner, error)
	GetBuyList(ctx context.Context, payload map[string]interface{}) ([]dto.StockInfo, error)
}

type tradingViewScreenersRepository struct {
	cfg            *config.Config
	log            *logger.Logger
	httpClient     httpclient.HTTPClient
	requestLimiter *rate.Limiter
	mu             sync.Mutex
}

func NewTradingViewScreenersRepository(cfg *config.Config, log *logger.Logger) *tradingViewScreenersRepository {
	secondsPerRequest := time.Minute / time.Duration(cfg.TradingView.MaxRequestPerMin)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	return &tradingViewScreenersRepository{
		cfg:            cfg,
		httpClient:     httpclient.New(log, cfg.TradingView.BaseURLScanner, cfg.TradingView.BaseTimeout, ""),
		log:            log,
		requestLimiter: requestLimiter,
		mu:             sync.Mutex{},
	}
}

// Get formats and sends a GET request to TradingView's scanner to retrieve market data
// for a given symbol and timeframe.
//
// The symbol should be in the format "EXCHANGE:SYMBOL" (e.g., "BINANCE:BTCUSDT").
// The interval parameter defines the timeframe (e.g., "1min", "5min", "1hour", etc.).
//
// Parameters:
//
//	symbol  - string: the exchange and symbol in the format "EXCHANGE:SYMBOL".
//	interval - string: the timeframe/interval for the data.
//
// Returns:
//
//	error - returns an error if the request fails or the symbol/interval is invalid.
func (t *tradingViewScreenersRepository) Get(ctx context.Context, symbol string, interval string) (*dto.TradingViewScanner, error) {
	t.mu.Lock()
	if !t.requestLimiter.Allow() {
		t.log.WarnContext(ctx, "TradingView Screeners API request limit exceeded",
			logger.IntField("max_request_per_minute", t.cfg.TradingView.MaxRequestPerMin),
		)
	}
	if err := t.requestLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	t.mu.Unlock()

	// Validate symbol parameter to ensure it has the correct format EXCHANGE:SYMBOL
	if strings.Count(symbol, ":") != 1 {
		return nil, fmt.Errorf("symbol parameter is not valid")
	}

	// Map interval input to appropriate TradingView format
	var dataInterval string
	switch interval {
	case dto.TradingViewInterval1Min: // 1 minute
		dataInterval = "|1"
	case dto.TradingViewInterval5Min: // 5 minutes
		dataInterval = "|5"
	case dto.TradingViewInterval15Min: // 15 minutes
		dataInterval = "|15"
	case dto.TradingViewInterval30Min: // 30 minutes
		dataInterval = "|30"
	case dto.TradingViewInterval1Hour: // 1 hour
		dataInterval = "|60"
	case dto.TradingViewInterval2Hour: // 2 hours
		dataInterval = "|120"
	case dto.TradingViewInterval4Hour: // 4 hours
		dataInterval = "|240"
	case dto.TradingViewInterval1Day: // 1 day
		dataInterval = ""
	case dto.TradingViewInterval1Week: // 1 week
		dataInterval = "|1W"
	case dto.TradingViewInterval1Month: // 1 month
		dataInterval = "|1M"
	default: // 1 day
		dataInterval = ""
	}

	// Construct the fields array which includes technical indicators for the specified interval
	fields := []string{
		fmt.Sprintf("Recommend.All%s", dataInterval),
		fmt.Sprintf("Recommend.Other%s", dataInterval),
		fmt.Sprintf("Recommend.MA%s", dataInterval),
		fmt.Sprintf("RSI%s", dataInterval),
		fmt.Sprintf("RSI[1]%s", dataInterval),
		fmt.Sprintf("Stoch.K%s", dataInterval),
		fmt.Sprintf("Stoch.D%s", dataInterval),
		fmt.Sprintf("Stoch.K[1]%s", dataInterval),
		fmt.Sprintf("Stoch.D[1]%s", dataInterval),
		fmt.Sprintf("CCI20%s", dataInterval),
		fmt.Sprintf("CCI20[1]%s", dataInterval),
		fmt.Sprintf("ADX%s", dataInterval),
		fmt.Sprintf("ADX+DI%s", dataInterval),
		fmt.Sprintf("ADX-DI%s", dataInterval),
		fmt.Sprintf("ADX+DI[1]%s", dataInterval),
		fmt.Sprintf("ADX-DI[1]%s", dataInterval),
		fmt.Sprintf("AO%s", dataInterval),
		fmt.Sprintf("AO[1]%s", dataInterval),
		fmt.Sprintf("AO[2]%s", dataInterval),
		fmt.Sprintf("MACD.macd%s", dataInterval),
		fmt.Sprintf("MACD.signal%s", dataInterval),
		fmt.Sprintf("Mom%s", dataInterval),
		fmt.Sprintf("Mom[1]%s", dataInterval),
		fmt.Sprintf("Rec.Stoch.RSI%s", dataInterval),
		fmt.Sprintf("Stoch.RSI.K%s", dataInterval),
		fmt.Sprintf("Rec.WR%s", dataInterval),
		fmt.Sprintf("W.R%s", dataInterval),
		fmt.Sprintf("Rec.BBPower%s", dataInterval),
		fmt.Sprintf("BBPower%s", dataInterval),
		fmt.Sprintf("Rec.UO%s", dataInterval),
		fmt.Sprintf("UO%s", dataInterval),
		fmt.Sprintf("EMA10%s", dataInterval),
		fmt.Sprintf("SMA10%s", dataInterval),
		fmt.Sprintf("EMA20%s", dataInterval),
		fmt.Sprintf("SMA20%s", dataInterval),
		fmt.Sprintf("EMA30%s", dataInterval),
		fmt.Sprintf("SMA30%s", dataInterval),
		fmt.Sprintf("EMA50%s", dataInterval),
		fmt.Sprintf("SMA50%s", dataInterval),
		fmt.Sprintf("EMA100%s", dataInterval),
		fmt.Sprintf("SMA100%s", dataInterval),
		fmt.Sprintf("EMA200%s", dataInterval),
		fmt.Sprintf("SMA200%s", dataInterval),
		fmt.Sprintf("Rec.Ichimoku%s", dataInterval),
		fmt.Sprintf("Ichimoku.BLine%s", dataInterval),
		fmt.Sprintf("Rec.VWMA%s", dataInterval),
		fmt.Sprintf("VWMA%s", dataInterval),
		fmt.Sprintf("Rec.HullMA9%s", dataInterval),
		fmt.Sprintf("HullMA9%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.S3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.S2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.S1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.Middle%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.R1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.R2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Classic.R3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.S3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.S2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.S1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.Middle%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.R1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.R2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Fibonacci.R3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.S3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.S2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.S1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.Middle%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.R1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.R2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Camarilla.R3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.S3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.S2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.S1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.Middle%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.R1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.R2%s", dataInterval),
		fmt.Sprintf("Pivot.M.Woodie.R3%s", dataInterval),
		fmt.Sprintf("Pivot.M.Demark.S1%s", dataInterval),
		fmt.Sprintf("Pivot.M.Demark.Middle%s", dataInterval),
		fmt.Sprintf("Pivot.M.Demark.R1%s", dataInterval),
		fmt.Sprintf("close%s", dataInterval),
		fmt.Sprintf("high%s", dataInterval),
		fmt.Sprintf("low%s", dataInterval),
	}

	// Build the URL for GET request by encoding the parameters
	params := map[string]string{
		"symbol": symbol,
		"fields": strings.Join(fields, ","),
	}
	responseMap := make(map[string]float64)
	baseResponse, err := t.httpClient.Get(ctx, "/symbol", params, nil, &responseMap)
	if err != nil {
		return nil, err
	}

	if baseResponse.StatusCode != http.StatusOK {
		t.log.WarnContext(ctx, "Return NON-200 response",
			logger.IntField("status_code", baseResponse.StatusCode),
		)
		return nil, fmt.Errorf("failed to get data: %v", baseResponse.Body)
	}

	ta := &dto.TradingViewScanner{}

	// Recommendations
	// Summary recommendation
	ta.Recommend.Global.Summary = tvComputeRecommend(responseMap[key("Recommend.All%s", dataInterval)])
	ta.Value.Global.Summary = responseMap[key("Recommend.All%s", dataInterval)]

	// Oscillators recommendation
	ta.Recommend.Global.Oscillators = tvComputeRecommend(responseMap[key("Recommend.Other%s", dataInterval)])
	ta.Value.Global.Oscillators = responseMap[key("Recommend.Other%s", dataInterval)]

	// Moving Averages recommendation
	ta.Recommend.Global.MA = tvComputeRecommend(responseMap[key("Recommend.MA%s", dataInterval)])
	ta.Value.Global.MA = responseMap[key("Recommend.MA%s", dataInterval)]

	// Oscillators
	// Relative Strength Index (14)
	ta.Recommend.Oscillators.RSI = tvRsi(responseMap[key("RSI%s", dataInterval)], responseMap[key("RSI[1]%s", dataInterval)])
	ta.Value.Oscillators.RSI = responseMap[key("RSI%s", dataInterval)]

	// Stochastic %K (14, 3, 3)
	ta.Recommend.Oscillators.StochK = tvStoch(responseMap[key("Stoch.K%s", dataInterval)], responseMap[key("Stoch.D%s", dataInterval)], responseMap[key("Stoch.K[1]%s", dataInterval)], responseMap[key("Stoch.D[1]%s", dataInterval)])
	ta.Value.Oscillators.StochK = responseMap[key("Stoch.K%s", dataInterval)]

	// Commodity Channel Index (20)
	ta.Recommend.Oscillators.CCI = tvCci20(responseMap[key("CCI20%s", dataInterval)], responseMap[key("CCI20[1]%s", dataInterval)])
	ta.Value.Oscillators.CCI = responseMap[key("CCI20%s", dataInterval)]

	// Average Directional Index (14)
	ta.Recommend.Oscillators.ADX = tvAdx(responseMap[key("ADX%s", dataInterval)], responseMap[key("ADX+DI%s", dataInterval)], responseMap[key("ADX+DI%s", dataInterval)], responseMap[key("ADX+DI[1]%s", dataInterval)], responseMap[key("ADX-DI[1]%s", dataInterval)])
	ta.Value.Oscillators.ADX.Value = responseMap[key("ADX%s", dataInterval)]          // ADX Value
	ta.Value.Oscillators.ADX.PlusDI = responseMap[key("ADX+DI%s", dataInterval)]      // ADX +DI
	ta.Value.Oscillators.ADX.MinusDI = responseMap[key("ADX-DI%s", dataInterval)]     // ADX -DI
	ta.Value.Oscillators.ADX.PlusDI1 = responseMap[key("ADX+DI[1]%s", dataInterval)]  // ADX +DI[1]
	ta.Value.Oscillators.ADX.MinusDI1 = responseMap[key("ADX-DI[1]%s", dataInterval)] // ADX -DI[1]

	// Awesome Oscillator
	ta.Recommend.Oscillators.AO = tvAo(responseMap[key("AO%s", dataInterval)], responseMap[key("AO[1]%s", dataInterval)], responseMap[key("AO[2]%s", dataInterval)])
	ta.Value.Oscillators.AO.Value = responseMap[key("AO%s", dataInterval)]    // AO current value
	ta.Value.Oscillators.AO.Prev1 = responseMap[key("AO[1]%s", dataInterval)] // AO previous 1 value
	ta.Value.Oscillators.AO.Prev2 = responseMap[key("AO[2]%s", dataInterval)] // AO previous 2 value

	// Momentum (10)
	ta.Recommend.Oscillators.Mom = tvMom(responseMap[key("Mom%s", dataInterval)], responseMap[key("Mom[1]%s", dataInterval)])
	ta.Value.Oscillators.Mom = responseMap[key("Mom%s", dataInterval)]

	// MACD Level (12, 26)
	ta.Recommend.Oscillators.MACD = tvMacd(responseMap[key("MACD.macd%s", dataInterval)], responseMap[key("MACD.signal%s", dataInterval)])
	ta.Value.Oscillators.MACD.Macd = responseMap[key("MACD.macd%s", dataInterval)]     // MACD line
	ta.Value.Oscillators.MACD.Signal = responseMap[key("MACD.signal%s", dataInterval)] // Signal line

	// Stochastic RSI Fast (3, 3, 14, 14)
	ta.Recommend.Oscillators.StochRSI = tvSimple(responseMap[key("Rec.Stoch.RSI%s", dataInterval)])
	ta.Value.Oscillators.StochRSI = responseMap[key("Stoch.RSI.K%s", dataInterval)]

	// Williams Percent Range (14)
	ta.Recommend.Oscillators.WR = tvSimple(responseMap[key("Rec.WR%s", dataInterval)])
	ta.Value.Oscillators.WR = responseMap[key("W.R%s", dataInterval)]

	// Bull Bear Power
	ta.Recommend.Oscillators.BBP = tvSimple(responseMap[key("Rec.BBPower%s", dataInterval)])
	ta.Value.Oscillators.BBP = responseMap[key("BBPower%s", dataInterval)]

	// Ultimate Oscillator (7, 14, 28)
	ta.Recommend.Oscillators.UO = tvSimple(responseMap[key("Rec.UO%s", dataInterval)])
	ta.Value.Oscillators.UO = responseMap[key("UO%s", dataInterval)]

	// Moving Averages
	// Exponential Moving Average (EMA)
	ta.Recommend.MovingAverages.EMA10 = tvMa(responseMap[key("EMA10%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.EMA10 = responseMap[key("EMA10%s", dataInterval)]

	ta.Recommend.MovingAverages.EMA20 = tvMa(responseMap[key("EMA20%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.EMA20 = responseMap[key("EMA20%s", dataInterval)]

	ta.Recommend.MovingAverages.EMA30 = tvMa(responseMap[key("EMA30%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.EMA30 = responseMap[key("EMA30%s", dataInterval)]

	ta.Recommend.MovingAverages.EMA50 = tvMa(responseMap[key("EMA50%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.EMA50 = responseMap[key("EMA50%s", dataInterval)]

	ta.Recommend.MovingAverages.EMA100 = tvMa(responseMap[key("EMA100%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.EMA100 = responseMap[key("EMA100%s", dataInterval)]

	ta.Recommend.MovingAverages.EMA200 = tvMa(responseMap[key("EMA200%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.EMA200 = responseMap[key("EMA200%s", dataInterval)]

	// Simple Moving Average (SMA)
	ta.Recommend.MovingAverages.SMA10 = tvMa(responseMap[key("SMA10%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.SMA10 = responseMap[key("SMA10%s", dataInterval)]

	ta.Recommend.MovingAverages.SMA20 = tvMa(responseMap[key("SMA20%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.SMA20 = responseMap[key("SMA20%s", dataInterval)]

	ta.Recommend.MovingAverages.SMA30 = tvMa(responseMap[key("SMA30%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.SMA30 = responseMap[key("SMA30%s", dataInterval)]

	ta.Recommend.MovingAverages.SMA50 = tvMa(responseMap[key("SMA50%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.SMA50 = responseMap[key("SMA50%s", dataInterval)]

	ta.Recommend.MovingAverages.SMA100 = tvMa(responseMap[key("SMA100%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.SMA100 = responseMap[key("SMA100%s", dataInterval)]

	ta.Recommend.MovingAverages.SMA200 = tvMa(responseMap[key("SMA200%s", dataInterval)], responseMap[key("close%s", dataInterval)])
	ta.Value.MovingAverages.SMA200 = responseMap[key("SMA200%s", dataInterval)]

	// Ichimoku Base Line (9, 26, 52, 26)
	ta.Recommend.MovingAverages.Ichimoku = tvSimple(responseMap[key("Rec.Ichimoku%s", dataInterval)])
	ta.Value.MovingAverages.Ichimoku = responseMap[key("Ichimoku.BLine%s", dataInterval)]

	// Volume Weighted Moving Average (20)
	ta.Recommend.MovingAverages.VWMA = tvSimple(responseMap[key("Rec.VWMA%s", dataInterval)])
	ta.Value.MovingAverages.VWMA = responseMap[key("VWMA%s", dataInterval)]

	// Hull Moving Average (9)
	ta.Recommend.MovingAverages.HullMA = tvSimple(responseMap[key("Rec.HullMA9%s", dataInterval)])
	ta.Value.MovingAverages.HullMA = responseMap[key("HullMA9%s", dataInterval)]

	// Pivots
	// Pivots - Classic
	ta.Value.Pivots.Classic.Middle = responseMap[key("Pivot.M.Classic.Middle%s", dataInterval)]
	ta.Value.Pivots.Classic.R1 = responseMap[key("Pivot.M.Classic.R1%s", dataInterval)]
	ta.Value.Pivots.Classic.R2 = responseMap[key("Pivot.M.Classic.R2%s", dataInterval)]
	ta.Value.Pivots.Classic.R3 = responseMap[key("Pivot.M.Classic.R3%s", dataInterval)]
	ta.Value.Pivots.Classic.S1 = responseMap[key("Pivot.M.Classic.S1%s", dataInterval)]
	ta.Value.Pivots.Classic.S2 = responseMap[key("Pivot.M.Classic.S2%s", dataInterval)]
	ta.Value.Pivots.Classic.S3 = responseMap[key("Pivot.M.Classic.S3%s", dataInterval)]

	// Pivots - Fibonacci
	ta.Value.Pivots.Fibonacci.Middle = responseMap[key("Pivot.M.Fibonacci.Middle%s", dataInterval)]
	ta.Value.Pivots.Fibonacci.R1 = responseMap[key("Pivot.M.Fibonacci.R1%s", dataInterval)]
	ta.Value.Pivots.Fibonacci.R2 = responseMap[key("Pivot.M.Fibonacci.R2%s", dataInterval)]
	ta.Value.Pivots.Fibonacci.R3 = responseMap[key("Pivot.M.Fibonacci.R3%s", dataInterval)]
	ta.Value.Pivots.Fibonacci.S1 = responseMap[key("Pivot.M.Fibonacci.S1%s", dataInterval)]
	ta.Value.Pivots.Fibonacci.S2 = responseMap[key("Pivot.M.Fibonacci.S2%s", dataInterval)]
	ta.Value.Pivots.Fibonacci.S3 = responseMap[key("Pivot.M.Fibonacci.S3%s", dataInterval)]

	// Pivots - Camarilla
	ta.Value.Pivots.Camarilla.Middle = responseMap[key("Pivot.M.Camarilla.Middle%s", dataInterval)]
	ta.Value.Pivots.Camarilla.R1 = responseMap[key("Pivot.M.Camarilla.R1%s", dataInterval)]
	ta.Value.Pivots.Camarilla.R2 = responseMap[key("Pivot.M.Camarilla.R2%s", dataInterval)]
	ta.Value.Pivots.Camarilla.R3 = responseMap[key("Pivot.M.Camarilla.R3%s", dataInterval)]
	ta.Value.Pivots.Camarilla.S1 = responseMap[key("Pivot.M.Camarilla.S1%s", dataInterval)]
	ta.Value.Pivots.Camarilla.S2 = responseMap[key("Pivot.M.Camarilla.S2%s", dataInterval)]
	ta.Value.Pivots.Camarilla.S3 = responseMap[key("Pivot.M.Camarilla.S3%s", dataInterval)]

	// Pivots - Woodie
	ta.Value.Pivots.Woodie.Middle = responseMap[key("Pivot.M.Woodie.Middle%s", dataInterval)]
	ta.Value.Pivots.Woodie.R1 = responseMap[key("Pivot.M.Woodie.R1%s", dataInterval)]
	ta.Value.Pivots.Woodie.R2 = responseMap[key("Pivot.M.Woodie.R2%s", dataInterval)]
	ta.Value.Pivots.Woodie.R3 = responseMap[key("Pivot.M.Woodie.R3%s", dataInterval)]
	ta.Value.Pivots.Woodie.S1 = responseMap[key("Pivot.M.Woodie.S1%s", dataInterval)]
	ta.Value.Pivots.Woodie.S2 = responseMap[key("Pivot.M.Woodie.S2%s", dataInterval)]
	ta.Value.Pivots.Woodie.S3 = responseMap[key("Pivot.M.Woodie.S3%s", dataInterval)]

	// Pivots - Demark
	ta.Value.Pivots.Demark.Middle = responseMap[key("Pivot.M.Demark.Middle%s", dataInterval)]
	ta.Value.Pivots.Demark.R1 = responseMap[key("Pivot.M.Demark.R1%s", dataInterval)]
	ta.Value.Pivots.Demark.S1 = responseMap[key("Pivot.M.Demark.S1%s", dataInterval)]

	// Prices
	ta.Value.Prices.Close = responseMap[key("close%s", dataInterval)]
	ta.Value.Prices.High = responseMap[key("high%s", dataInterval)]
	ta.Value.Prices.Low = responseMap[key("low%s", dataInterval)]

	return ta, nil
}

// key creates a key for accessing data by concatenating the indicator name and the dataInterval.
func key(indicator, dataInterval string) string {
	return fmt.Sprintf(indicator, dataInterval)
}

// tvComputeRecommend - Compute Recommend
func tvComputeRecommend(v float64) int {
	switch {
	case v > 0.1 && v <= 0.5:
		return dto.TradingViewSignalBuy // BUY
	case v > 0.5 && v <= 1:
		return dto.TradingViewSignalStrongBuy // STRONG_BUY
	case v >= -0.1 && v <= 0.1:
		return dto.TradingViewSignalNeutral // NEUTRAL
	case v >= -1 && v < -0.5:
		return dto.TradingViewSignalStrongSell // STRONG_SELL
	case v >= -0.5 && v < -0.1:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvRsi - Compute Relative Strength Index
func tvRsi(rsi, rsi1 float64) int {
	switch {
	case rsi < 30 && rsi1 < rsi:
		return dto.TradingViewSignalBuy // BUY
	case rsi > 70 && rsi1 > rsi:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvStoch - Compute Stochastic
func tvStoch(k, d, k1, d1 float64) int {
	switch {
	case k < 20 && d < 20 && k > d && k1 < d1:
		return dto.TradingViewSignalBuy // BUY
	case k > 80 && d > 80 && k < d && k1 > d1:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvCci20 - Compute Commodity Channel Index 20
func tvCci20(cci20, cci201 float64) int {
	switch {
	case cci20 < -100 && cci20 > cci201:
		return dto.TradingViewSignalBuy // BUY
	case cci20 > 100 && cci20 < cci201:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvAdx - Compute Average Directional Index
func tvAdx(adx, adxpdi, adxndi, adxpdi1, adxndi1 float64) int {
	switch {
	case adx > 20 && adxpdi1 < adxndi1 && adxpdi > adxndi:
		return dto.TradingViewSignalBuy // BUY
	case adx > 20 && adxpdi1 > adxndi1 && adxpdi < adxndi:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvAo - Compute Awesome Oscillator
func tvAo(ao, ao1, ao2 float64) int {
	switch {
	case (ao > 0 && ao1 < 0) || (ao > 0 && ao1 > 0 && ao > ao1 && ao2 > ao1):
		return dto.TradingViewSignalBuy // BUY
	case (ao < 0 && ao1 > 0) || (ao < 0 && ao1 < 0 && ao < ao1 && ao2 < ao1):
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvMom - Compute Momentum
func tvMom(mom, mom1 float64) int {
	switch {
	case mom > mom1:
		return dto.TradingViewSignalBuy // BUY
	case mom < mom1:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvMacd - Compute Moving Average Convergence/Divergence
func tvMacd(macd, s float64) int {
	switch {
	case macd > s:
		return dto.TradingViewSignalBuy // BUY
	case macd < s:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvSimple - Compute Simple
func tvSimple(v float64) int {
	switch {
	case v == 1:
		return dto.TradingViewSignalBuy // BUY
	case v == -1:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

// tvMa - Compute Moving Average
func tvMa(ma, close float64) int {
	switch {
	case ma < close:
		return dto.TradingViewSignalBuy // BUY
	case ma > close:
		return dto.TradingViewSignalSell // SELL
	default:
		return dto.TradingViewSignalNeutral // NEUTRAL
	}
}

func (t *tradingViewScreenersRepository) GetBuyList(ctx context.Context, payload map[string]interface{}) ([]dto.StockInfo, error) {
	if payload["markets"] == nil {
		return nil, fmt.Errorf("markets is required")
	}

	markets := payload["markets"].([]interface{})
	if len(markets) == 0 {
		return nil, fmt.Errorf("markets is required")
	}

	url := fmt.Sprintf("/%s/scan?label-product=screener-stock", markets[0])
	var response dto.TradingViewBuyListResponse
	resp, err := t.httpClient.Post(ctx, url, payload, nil, &response)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get data: %v", resp.Body)
	}

	var result []dto.StockInfo
	for _, data := range response.Data {
		if len(result) >= t.cfg.TradingView.BuyListMaxStockAnalyze {
			break
		}

		if data.StockCode == "" {
			continue
		}

		if len(data.TechnicalRating) == 0 {
			continue
		}

		valueParse := strings.Split(data.StockCode, ":")
		if len(valueParse) < 2 {
			continue
		}
		result = append(result, dto.StockInfo{
			StockCode: valueParse[1],
			Exchange:  valueParse[0],
		})
	}
	return result, nil
}
