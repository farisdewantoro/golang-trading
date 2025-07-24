package repository

import (
	"context"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/pkg/common"
	"golang-trading/pkg/httpclient"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"net/http"
	"strings"
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
	time.Sleep(100 * time.Millisecond)

	if !r.requestLimiter.Allow() {
		r.logger.WarnContext(ctx, "Yahoo Finance API request limit exceeded",
			logger.IntField("max_request_per_minute", r.cfg.YahooFinance.MaxRequestPerMinute),
		)
	}

	r.mu.Lock()
	err := r.requestLimiter.Wait(ctx)
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}

	symbol, err := r.parseSymbol(param.StockCode, param.Exchange)
	if err != nil {
		return nil, err
	}

	param.StockCode = symbol

	// Build URL with query parameters
	endpoint := "/" + param.StockCode

	period1, period2 := utils.MapPeriodeStringToUnix(param.Range)
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
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "en-US,en;q=0.9",
		"Accept-Encoding":           "gzip, deflate, br, zstd",
		"Referer":                   "https://finance.yahoo.com/",
		"Cache-Control":             "max-age=0",
		"Priority":                  "u=0, i",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Ch-UA":                 `"Not)A;Brand";v="8", "Chromium";v="138", "Google Chrome";v="138"`,
		"Sec-Ch-UA-Mobile":          "?0",
		"Sec-Ch-UA-Platform":        "macOS",
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
			Volume:    float64(quote.Volume[i]),
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

func (r *yahooFinanceRepository) parseSymbol(symbol string, exchange string) (string, error) {
	symbol = strings.ToUpper(symbol)

	if exchange == common.EXCHANGE_IDX {
		return symbol + ".JK", nil
	}

	return symbol, nil
}
