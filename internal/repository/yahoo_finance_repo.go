package repository

import (
	"context"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/pkg/httpclient"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type YahooFinanceRepository interface {
	Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error)
}

// yahooFinanceRepository is an implementation of NewsAnalyzerRepository that uses the Google Gemini API.
type yahooFinanceRepository struct {
	httpClient     httpclient.HTTPClient
	cfg            *config.Config
	logger         *logger.Logger
	requestLimiter *rate.Limiter
	mu             sync.Mutex
}

// NewYahooFinanceRepository creates a new instance of yahooFinanceRepository.
func NewYahooFinanceRepository(cfg *config.Config, log *logger.Logger) YahooFinanceRepository {
	secondsPerRequest := time.Minute / time.Duration(cfg.YahooFinance.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	return &yahooFinanceRepository{
		httpClient:     httpclient.New(log, cfg.YahooFinance.BaseURL, cfg.YahooFinance.Timeout, ""),
		cfg:            cfg,
		logger:         log,
		requestLimiter: requestLimiter,
		mu:             sync.Mutex{},
	}
}

func (r *yahooFinanceRepository) Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error) {

	r.mu.Lock()
	if !r.requestLimiter.Allow() {
		r.logger.WarnContext(ctx, "Yahoo Finance API request limit exceeded",
			logger.IntField("max_request_per_minute", r.cfg.YahooFinance.MaxRequestPerMinute),
			logger.IntField("available_tokens_at", int(r.requestLimiter.TokensAt(utils.TimeNowWIB()))),
		)
	}
	if err := r.requestLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	r.mu.Unlock()

	if param.Exchange == dto.ExchangeIDX {
		param.StockCode = fmt.Sprintf("%s.JK", param.StockCode)
	}

	// Build URL with query parameters
	endpoint := "/" + param.StockCode

	period1, period2 := r.MapPeriodeStringToUnix(param.Range)
	if period1 == 0 || period2 == 0 {
		return nil, fmt.Errorf("invalid period")
	}
	queryParams := map[string]string{
		"period1":        fmt.Sprintf("%d", period1),
		"period2":        fmt.Sprintf("%d", period2),
		"interval":       param.Interval,
		"includePrePost": "false",
		"events":         "div,split",
	}

	headers := map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "en-US,en;q=0.9",
		"Accept-Encoding": "gzip, deflate, br",
		"Connection":      "keep-alive",
		"Referer":         "https://finance.yahoo.com/",
	}

	var yahooResp dto.YahooFinanceResponse
	resp, err := r.httpClient.Get(ctx, endpoint, queryParams, headers, &yahooResp)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data from yahoo finance: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		r.logger.Error("Yahoo Finance API returned Non-OK status",
			logger.IntField("status_code", resp.StatusCode),
			logger.StringField("body", string(resp.Body)))
		return nil, fmt.Errorf("yahoo finance api returned status: %d", resp.StatusCode)
	}

	// Check for API errors
	if yahooResp.Chart.Error != nil {
		return nil, fmt.Errorf("yahoo finance api error: %v", yahooResp.Chart.Error)
	}

	// Check if we have results
	if len(yahooResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data returned for symbol: %s", param.StockCode)
	}

	result := yahooResp.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no quote data available for symbol: %s", param.StockCode)
	}

	quote := result.Indicators.Quote[0]

	// Convert to OHLCVData format
	var ohlcvData []dto.StockOHLCV
	for i, timestamp := range result.Timestamp {
		// Skip if any required data is missing
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) ||
			i >= len(quote.Close) || i >= len(quote.Volume) {
			continue
		}

		// Skip if any value is 0 (missing data)
		if quote.Open[i] == 0 || quote.High[i] == 0 || quote.Low[i] == 0 ||
			quote.Close[i] == 0 || quote.Volume[i] == 0 {
			continue
		}

		ohlcvData = append(ohlcvData, dto.StockOHLCV{
			Timestamp: timestamp,
			Open:      quote.Open[i],
			High:      quote.High[i],
			Low:       quote.Low[i],
			Close:     quote.Close[i],
			Volume:    quote.Volume[i],
		})
	}

	if len(ohlcvData) == 0 {
		return nil, fmt.Errorf("no valid OHLCV data found for symbol: %s", param.StockCode)
	}

	marketPrice := 0.0

	if len(yahooResp.Chart.Result) > 0 && yahooResp.Chart.Result[0].Meta.RegularMarketPrice > 0 {
		marketPrice = yahooResp.Chart.Result[0].Meta.RegularMarketPrice
	}

	return &dto.StockData{
		MarketPrice: marketPrice,
		OHLCV:       ohlcvData,
		Range:       param.Range,
		Interval:    param.Interval,
	}, nil
}

// MapPeriodeStringToUnix convert days to unix timestamp
func (r *yahooFinanceRepository) MapPeriodeStringToUnix(periode string) (int64, int64) {

	now := utils.TimeNowWIB()
	switch periode {
	case "1d":
		return now.AddDate(0, 0, -1).Unix(), now.Unix()
	case "14d":
		return now.AddDate(0, 0, -14).Unix(), now.Unix()
	case "1w":
		return now.AddDate(0, 0, -7).Unix(), now.Unix()
	case "1m":
		return now.AddDate(0, 0, -30).Unix(), now.Unix()
	case "2m":
		return now.AddDate(0, 0, -60).Unix(), now.Unix()
	case "3m":
		return now.AddDate(0, 0, -90).Unix(), now.Unix()
	case "6m":
		return now.AddDate(0, 0, -180).Unix(), now.Unix()
	case "1y":
		return now.AddDate(0, 0, -365).Unix(), now.Unix()
	default:
		return 0, 0
	}
}
