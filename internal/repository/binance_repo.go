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
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type BinanceRepository interface {
	GetKlines(ctx context.Context, symbol string, interval string, limit int, startTime, endTime int64) ([]dto.BinanceKlines, error)
	GetLastPrice(ctx context.Context, symbol string) (*dto.BinancePrice, error)
	Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error)
}

type binanceRepository struct {
	httpClient     httpclient.HTTPClient
	cfg            *config.Config
	logger         *logger.Logger
	requestLimiter *rate.Limiter
	mu             sync.Mutex
}

func NewBinanceRepository(cfg *config.Config, log *logger.Logger) BinanceRepository {
	secondsPerRequest := time.Minute / time.Duration(cfg.Binance.MaxRequestPerMinute)
	requestLimiter := rate.NewLimiter(rate.Every(secondsPerRequest), 1)

	return &binanceRepository{
		httpClient:     httpclient.New(log, cfg.Binance.BaseURL, cfg.Binance.Timeout, ""),
		cfg:            cfg,
		logger:         log,
		requestLimiter: requestLimiter,
		mu:             sync.Mutex{},
	}
}

func (r *binanceRepository) GetKlines(ctx context.Context, symbol string, interval string, limit int, startTime, endTime int64) ([]dto.BinanceKlines, error) {
	if err := r.requestLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	endpoint := "/api/v3/klines"
	queryParams := map[string]string{
		"symbol":    symbol,
		"interval":  interval,
		"limit":     strconv.Itoa(limit),
		"startTime": strconv.FormatInt(startTime, 10),
		"endTime":   strconv.FormatInt(endTime, 10),
	}

	var klines [][]interface{}
	resp, err := r.httpClient.Get(ctx, endpoint, queryParams, nil, &klines)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch klines from binance: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		r.logger.Error("Binance API returned Non-OK status for klines",
			logger.IntField("status_code", resp.StatusCode),
			logger.StringField("body", string(resp.Body)))
		return nil, fmt.Errorf("binance api returned status: %d", resp.StatusCode)
	}

	var result []dto.BinanceKlines
	for _, k := range klines {
		openTime, _ := k[0].(float64)
		open, _ := strconv.ParseFloat(k[1].(string), 64)
		high, _ := strconv.ParseFloat(k[2].(string), 64)
		low, _ := strconv.ParseFloat(k[3].(string), 64)
		closePrice, _ := strconv.ParseFloat(k[4].(string), 64)
		volume, _ := strconv.ParseFloat(k[5].(string), 64)
		closeTime, _ := k[6].(float64)
		quoteAssetVolume, _ := strconv.ParseFloat(k[7].(string), 64)
		trades, _ := k[8].(float64)
		takerBuyBaseAssetVolume, _ := strconv.ParseFloat(k[9].(string), 64)
		takerBuyQuoteAssetVolume, _ := strconv.ParseFloat(k[10].(string), 64)

		result = append(result, dto.BinanceKlines{
			OpenTime:                 int64(openTime),
			Open:                     open,
			High:                     high,
			Low:                      low,
			Close:                    closePrice,
			Volume:                   volume,
			CloseTime:                int64(closeTime),
			QuoteAssetVolume:         quoteAssetVolume,
			NumberOfTrades:           int64(trades),
			TakerBuyBaseAssetVolume:  takerBuyBaseAssetVolume,
			TakerBuyQuoteAssetVolume: takerBuyQuoteAssetVolume,
		})
	}

	return result, nil
}

func (r *binanceRepository) Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error) {
	startTime, endTime := utils.MapPeriodeStringToUnixMs(param.Range)

	klines, err := r.GetKlines(ctx, param.StockCode, param.Interval, 1000, startTime, endTime)
	if err != nil {
		return nil, err
	}

	var ohlcvData []dto.StockOHLCV
	for _, k := range klines {
		ohlcvData = append(ohlcvData, dto.StockOHLCV{
			Timestamp: k.OpenTime, // dalam ms tpi di yahoo finance second
			Open:      k.Open,
			High:      k.High,
			Low:       k.Low,
			Close:     k.Close,
			Volume:    k.Volume,
		})
	}

	lastPrice, err := r.GetLastPrice(ctx, param.StockCode)
	if err != nil {
		return nil, err
	}

	return &dto.StockData{
		OHLCV:       ohlcvData,
		Range:       param.Range,
		Interval:    param.Interval,
		MarketPrice: lastPrice.Price,
	}, nil
}

func (r *binanceRepository) GetLastPrice(ctx context.Context, symbol string) (*dto.BinancePrice, error) {
	if err := r.requestLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	endpoint := "/api/v3/ticker/price"
	queryParams := map[string]string{
		"symbol": symbol,
	}

	var respData map[string]string
	resp, err := r.httpClient.Get(ctx, endpoint, queryParams, nil, &respData)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch last price from binance: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		r.logger.Error("Binance API returned Non-OK status for price",
			logger.IntField("status_code", resp.StatusCode),
			logger.StringField("body", string(resp.Body)))
		return nil, fmt.Errorf("binance api returned status: %d", resp.StatusCode)
	}

	price, err := strconv.ParseFloat(respData["price"], 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price from binance: %w", err)
	}

	return &dto.BinancePrice{
		Symbol: symbol,
		Price:  price,
	}, nil
}
