package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/common"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"math"
	"time"

	"gopkg.in/telebot.v3"
)

// StockPriceAlertStrategy defines the strategy for scraping stock news.
type StockPriceAlertStrategy struct {
	logger                         *logger.Logger
	inmemoryCache                  cache.Cache
	tradingViewScreenersRepository repository.TradingViewScreenersRepository
	telegram                       *telegram.TelegramRateLimiter
	stockPositionsRepository       repository.StockPositionsRepository
	candleRepository               repository.CandleRepository
}

// StockPriceAlertPayload defines the payload for stock price alert.
type StockPriceAlertPayload struct {
	DataInterval                string  `json:"data_interval"`
	DataRange                   string  `json:"data_range"`
	AlertCacheDuration          string  `json:"alert_cache_duration"`
	AlertResendThresholdPercent float64 `json:"alert_resend_threshold_percent"`
}

// StockPriceAlertResult defines the result for stock price alert.
type StockPriceAlertResult struct {
	StockCode string `json:"stock_code"`
	Errors    string `json:"errors"`
}

// NewStockPriceAlertStrategy creates a new instance of StockPriceAlertStrategy.
func NewStockPriceAlertStrategy(
	cfg *config.Config,
	logger *logger.Logger,
	inmemoryCache cache.Cache,
	tradingViewScreenersRepository repository.TradingViewScreenersRepository,
	telegram *telegram.TelegramRateLimiter,
	stockPositionsRepository repository.StockPositionsRepository,
	candleRepository repository.CandleRepository) JobExecutionStrategy {
	return &StockPriceAlertStrategy{
		logger:                         logger,
		inmemoryCache:                  inmemoryCache,
		tradingViewScreenersRepository: tradingViewScreenersRepository,
		telegram:                       telegram,
		stockPositionsRepository:       stockPositionsRepository,
		candleRepository:               candleRepository,
	}
}

// GetType returns the job type this strategy handles.
func (s *StockPriceAlertStrategy) GetType() JobType {
	return JobTypeStockPriceAlert
}

// Execute runs the stock alert job.
func (s *StockPriceAlertStrategy) Execute(ctx context.Context, job *model.Job) (JobResult, error) {
	s.logger.DebugContext(ctx, "Executing stock alert job", logger.IntField("job_id", int(job.ID)))

	var (
		payload StockPriceAlertPayload
		results []StockPriceAlertResult
	)
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.logger.Error("Failed to unmarshal job payload", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to unmarshal job payload: %v", err)}, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	alertCacheDuration, err := time.ParseDuration(payload.AlertCacheDuration)
	if err != nil {
		s.logger.Error("Failed to parse alert_cache_duration", logger.ErrorField(err), logger.StringField("alert_cache_duration", payload.AlertCacheDuration), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to parse alert_cache_duration: %v", err)}, fmt.Errorf("failed to parse alert_cache_duration: %w", err)
	}

	stockPositions, err := s.stockPositionsRepository.Get(ctx, dto.GetStockPositionsParam{
		PriceAlert: utils.ToPointer(true),
		IsActive:   utils.ToPointer(true),
	})
	if err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to get stocks positions: %v", err)}, err
	}

	for _, stockPosition := range stockPositions {

		resultData := StockPriceAlertResult{
			StockCode: stockPosition.StockCode,
		}

		s.logger.DebugContext(ctx, "Processing stock alert", logger.StringField("stock_code", stockPosition.StockCode))
		stockData, err := s.candleRepository.Get(ctx, dto.GetStockDataParam{
			StockCode: stockPosition.StockCode,
			Range:     payload.DataRange,
			Interval:  payload.DataInterval,
			Exchange:  stockPosition.Exchange,
		})
		if err != nil {
			s.logger.Error("Failed to get stock data", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			resultData.Errors = err.Error()
			results = append(results, resultData)
			continue
		}

		// set last price in Redis
		stockCodeWithExchange := stockPosition.Exchange + ":" + stockPosition.StockCode
		key := fmt.Sprintf(common.KEY_LAST_PRICE, stockCodeWithExchange)
		s.inmemoryCache.Set(key, stockData.MarketPrice, alertCacheDuration)

		isSendAlert := false
		// check if market price already reach take profit or stop loss
		targetTP := stockPosition.TakeProfitPrice
		targetSL := stockPosition.StopLossPrice
		targetTrailingProfit := stockPosition.TrailingProfitPrice
		targetTrailingStop := stockPosition.TrailingStopPrice

		if stockData.MarketPrice >= targetTP && targetTrailingProfit == 0 {
			isSendAlert = true
			err = s.sendTelegramMessageAlert(
				ctx,
				&stockPosition,
				telegram.TakeProfit,
				stockData.MarketPrice,
				targetTP,
				utils.TimeNowWIB().Unix(),
				alertCacheDuration,
				payload.AlertResendThresholdPercent,
			)
		} else if targetTrailingProfit > 0 && stockData.MarketPrice <= targetTrailingProfit {
			isSendAlert = true
			err = s.sendTelegramMessageAlert(
				ctx,
				&stockPosition,
				telegram.TrailingProfit,
				stockData.MarketPrice,
				targetTrailingProfit,
				utils.TimeNowWIB().Unix(),
				alertCacheDuration,
				payload.AlertResendThresholdPercent,
			)
		} else if targetTrailingStop > 0 && stockData.MarketPrice <= targetTrailingStop {
			isSendAlert = true
			err = s.sendTelegramMessageAlert(
				ctx,
				&stockPosition,
				telegram.TrailingStop,
				stockData.MarketPrice,
				targetTrailingStop,
				utils.TimeNowWIB().Unix(),
				alertCacheDuration,
				payload.AlertResendThresholdPercent,
			)
		} else if stockData.MarketPrice <= targetSL && targetTrailingStop == 0 {
			isSendAlert = true
			err = s.sendTelegramMessageAlert(
				ctx,
				&stockPosition,
				telegram.StopLoss,
				stockData.MarketPrice,
				targetSL,
				utils.TimeNowWIB().Unix(),
				alertCacheDuration,
				payload.AlertResendThresholdPercent,
			)
		}

		if isSendAlert {
			stockPosition.LastPriceAlertAt = utils.ToPointer(utils.TimeNowWIB())
			errSql := s.stockPositionsRepository.Update(ctx, stockPosition)
			if errSql != nil {
				s.logger.Error("Failed to update stock position", logger.ErrorField(errSql), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = errSql.Error()
				results = append(results, resultData)
			}
		}

		// set result
		if err != nil {
			s.logger.Error("Failed to send stock alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
			resultData.Errors = err.Error()
			results = append(results, resultData)
		} else if isSendAlert {
			results = append(results, resultData)
		} else {
			results = append(results, resultData)
		}
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to marshal results: %v", err)}, fmt.Errorf("failed to marshal results: %w", err)
	}

	return JobResult{ExitCode: JOB_EXIT_CODE_SUCCESS, Output: string(resultJSON)}, nil
}

func (s *StockPriceAlertStrategy) sendTelegramMessageAlert(ctx context.Context,
	stockPosition *model.StockPosition,
	alertType telegram.AlertType,
	triggerPrice float64,
	targetPrice float64,
	timestamp int64,
	cacheDuration time.Duration,
	alertResendThresholdPercent float64) error {
	ok, err := s.shouldTriggerAlert(ctx, stockPosition, triggerPrice, alertType, alertResendThresholdPercent)
	if err != nil {
		s.logger.Error("Failed to check alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
		return err
	}
	if !ok {
		return nil
	}

	message := telegram.FormatStockAlertResultForTelegram(alertType, stockPosition.StockCode, triggerPrice, targetPrice, timestamp)

	menu := &telebot.ReplyMarkup{}
	menu.Inline(
		menu.Row(menu.Data("🔍 Detail Posisi", "btn_detail_stock_position", fmt.Sprintf("%d", stockPosition.ID)), menu.Data("📤 Keluar dari Posisi", "btn_exit_stock_position", fmt.Sprintf("%s|%d", stockPosition.Exchange+":"+stockPosition.StockCode, stockPosition.ID))),
		menu.Row(menu.Data("🗑️ Hapus Pesan", "btn_delete_message")),
	)

	err = s.telegram.SendMessageUser(ctx, message, stockPosition.User.TelegramID, menu, telebot.ModeHTML)
	if err != nil {
		s.logger.Error("Failed to send alert", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
	}

	s.logger.Debug("Send alert", logger.StringField("stock_code", stockPosition.StockCode), logger.StringField("alert_type", string(alertType)))

	s.inmemoryCache.Set(fmt.Sprintf(common.KEY_STOCK_PRICE_ALERT, alertType, stockPosition.StockCode), triggerPrice, cacheDuration)
	return nil
}

func (s *StockPriceAlertStrategy) getLastAlertPrice(ctx context.Context, stockPosition *model.StockPosition, alertType telegram.AlertType) (float64, error) {
	lastAlertPrice, ok := s.inmemoryCache.Get(fmt.Sprintf(common.KEY_STOCK_PRICE_ALERT, alertType, stockPosition.StockCode))
	if !ok || lastAlertPrice == nil {
		return 0, nil // belum pernah ada alert
	}

	lastAlertPriceFloat, ok := lastAlertPrice.(float64)
	if !ok {
		return 0, fmt.Errorf("failed to get last alert price")
	}
	return lastAlertPriceFloat, nil
}

func (s *StockPriceAlertStrategy) shouldTriggerAlert(ctx context.Context,
	stockPosition *model.StockPosition,
	triggerPrice float64,
	alertType telegram.AlertType,
	alertResendThresholdPercent float64) (bool, error) {

	lastAlertPrice, err := s.getLastAlertPrice(ctx, stockPosition, alertType)
	if err != nil {
		return false, err
	}

	if lastAlertPrice == 0 {
		// Belum ada alert sebelumnya, trigger
		return true, nil
	}

	// Hitung selisih persentase
	diff := math.Abs(triggerPrice - lastAlertPrice)
	percentChange := (diff / lastAlertPrice) * 100

	if percentChange >= alertResendThresholdPercent {
		s.logger.Debug("Trigger Resend alert", logger.StringField("stock_code", stockPosition.StockCode), logger.IntField("trigger_price", int(triggerPrice)), logger.IntField("last_alert_price", int(lastAlertPrice)), logger.IntField("percent_change", int(percentChange)))
		return true, nil
	}

	s.logger.Debug("Skip Resend alert", logger.StringField("stock_code", stockPosition.StockCode), logger.IntField("trigger_price", int(triggerPrice)), logger.IntField("last_alert_price", int(lastAlertPrice)), logger.IntField("percent_change", int(percentChange)))

	return false, nil
}
