package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/contract"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"strings"
	"sync"

	"gopkg.in/telebot.v3"
)

type PositionMonitoringEvaluator interface {
	JobExecutionStrategy
	EvaluateStockPosition(ctx context.Context, stockPositions []model.StockPosition) ([]StockPositionMonitoringResult, error)
}
type StockPositionMonitoringStrategy struct {
	logger                         *logger.Logger
	cfg                            *config.Config
	inmemoryCache                  cache.Cache
	tradingViewScreenersRepository repository.TradingViewScreenersRepository
	telegram                       *telegram.TelegramRateLimiter
	stockPositionsRepo             repository.StockPositionsRepository
	stockAnalyzer                  StockAnalyzer
	stockPositionMonitoringRepo    repository.StockPositionMonitoringRepository
	systemParamRepository          repository.SystemParamRepository
	tradingPositionService         contract.TradingPositionContract
}

type StockPositionMonitoringResult struct {
	StockCode string `json:"stock_code"`
	Errors    string `json:"errors"`
}

func NewStockPositionMonitoringStrategy(
	logger *logger.Logger,
	cfg *config.Config,
	inmemoryCache cache.Cache,
	tradingViewScreenersRepository repository.TradingViewScreenersRepository,
	telegram *telegram.TelegramRateLimiter,
	stockPositionsRepository repository.StockPositionsRepository,
	stockAnalyzer StockAnalyzer,
	stockPositionMonitoringRepo repository.StockPositionMonitoringRepository,
	systemParamRepository repository.SystemParamRepository,
	tradingPositionService contract.TradingPositionContract,
) PositionMonitoringEvaluator {
	return &StockPositionMonitoringStrategy{
		logger:                         logger,
		cfg:                            cfg,
		inmemoryCache:                  inmemoryCache,
		tradingViewScreenersRepository: tradingViewScreenersRepository,
		telegram:                       telegram,
		stockPositionsRepo:             stockPositionsRepository,
		stockAnalyzer:                  stockAnalyzer,
		stockPositionMonitoringRepo:    stockPositionMonitoringRepo,
		systemParamRepository:          systemParamRepository,
		tradingPositionService:         tradingPositionService,
	}
}

func (s *StockPositionMonitoringStrategy) GetType() JobType {
	return JobTypeStockPositionMonitor
}

func (s *StockPositionMonitoringStrategy) Execute(ctx context.Context, job *model.Job) (JobResult, error) {
	var (
		stocks  []dto.StockInfo
		results []StockPositionMonitoringResult
	)

	stockPositions, err := s.stockPositionsRepo.Get(ctx, dto.GetStockPositionsParam{
		MonitorPosition: utils.ToPointer(true),
		IsActive:        utils.ToPointer(true),
	})
	if err != nil {
		s.logger.Error("Failed to get stocks positions", logger.ErrorField(err))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to get stocks positions: %v", err)}, fmt.Errorf("failed to get stocks positions: %w", err)
	}

	s.logger.Info("Stock position monitoring completed", logger.IntField("total_stock", len(stocks)))

	if len(stockPositions) == 0 {
		return JobResult{ExitCode: JOB_EXIT_CODE_SKIPPED, Output: "no stocks positions found"}, nil
	}

	results, err = s.EvaluateStockPosition(ctx, stockPositions)
	if err != nil {
		s.logger.Error("Failed to evaluate stock position", logger.ErrorField(err))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to evaluate stock position: %v", err)}, fmt.Errorf("failed to evaluate stock position: %w", err)
	}

	if len(results) == 0 {
		return JobResult{ExitCode: JOB_EXIT_CODE_SKIPPED, Output: "result is empty no evaluation"}, nil
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to marshal results: %v", err)}, fmt.Errorf("failed to marshal results: %w", err)
	}

	return JobResult{ExitCode: JOB_EXIT_CODE_SUCCESS, Output: string(resultJSON)}, nil
}

func (s *StockPositionMonitoringStrategy) EvaluateStockPosition(ctx context.Context, stockPositions []model.StockPosition) ([]StockPositionMonitoringResult, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []StockPositionMonitoringResult
	)

	for _, sp := range stockPositions {

		stockPosition := sp // Create a copy of the stockPosition to avoid data
		wg.Add(1)
		utils.GoSafe(func() {
			defer wg.Done()
			resultData := StockPositionMonitoringResult{
				StockCode: stockPosition.Exchange + ":" + stockPosition.StockCode,
			}
			defer func() {
				mu.Lock()
				results = append(results, resultData)
				mu.Unlock()
			}()
			stockAnalyses, err := s.stockAnalyzer.AnalyzeStock(ctx, dto.StockInfo{
				StockCode: stockPosition.StockCode,
				Exchange:  stockPosition.Exchange,
			})
			if err != nil {
				s.logger.Error("Failed to analyze stock", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = err.Error()
				return
			}
			supports, resistances, err := s.tradingPositionService.CalculateSupportResistance(ctx, stockAnalyses)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to calculate S/R for position monitoring", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = err.Error()
				return
			}

			positionAnalysis, err := s.tradingPositionService.EvaluatePositionMonitoring(ctx, &stockPosition, stockAnalyses, supports, resistances)
			if err != nil {
				s.logger.ErrorContextWithAlert(ctx, "Failed to evaluate signal", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = err.Error()
				return
			}

			isTrailing := positionAnalysis.TrailingProfitPrice > stockPosition.TrailingProfitPrice ||
				positionAnalysis.TrailingStopPrice > stockPosition.TrailingStopPrice

			// Convert []dto.Insight to []string
			var insights []model.Insight
			for _, insight := range positionAnalysis.Insight {
				insights = append(insights, model.Insight{
					Text:   insight.Text,
					Weight: insight.Weight,
				})
			}

			// Unmarshal IndicatorSummary from string to struct
			var indicatorSummary model.IndicatorSummary
			if positionAnalysis.IndicatorSummary != "" {
				if err := json.Unmarshal([]byte(positionAnalysis.IndicatorSummary), &indicatorSummary); err != nil {
					s.logger.ErrorContextWithAlert(ctx, "Failed to unmarshal indicator summary", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
					// Handle error appropriately, maybe skip this part or set a default
					return
				}
			}

			summary := model.PositionAnalysisSummary{
				TechnicalAnalysis: model.PositionTechnicalAnalysisSummary{
					Signal:           string(positionAnalysis.TechnicalSignal),
					Score:            positionAnalysis.Score,
					Insight:          insights,
					Status:           string(positionAnalysis.Status),
					IndicatorSummary: indicatorSummary,
				},
				PositionSignal: string(positionAnalysis.Signal),
			}
			if positionAnalysis.HighestPriceSinceTTP > stockPosition.HighestPriceSinceTTP {
				stockPosition.HighestPriceSinceTTP = positionAnalysis.HighestPriceSinceTTP
			}

			/// terdapat potensi bug FIX IT, jadi yang di send ke user trailing stop nya yg lama karena di set di loop
			stockPosition.TrailingProfitPrice = positionAnalysis.TrailingProfitPrice
			stockPosition.TrailingStopPrice = positionAnalysis.TrailingStopPrice
			if stockPosition.InitialScore == 0 {
				stockPosition.InitialScore = positionAnalysis.Score
			}
			lastScore := stockPosition.FinalScore
			stockPosition.FinalScore = positionAnalysis.Score
			errUpdate := s.stockPositionsRepo.Update(ctx, stockPosition)
			if errUpdate != nil {
				s.logger.ErrorContextWithAlert(ctx, "Failed to update stock position", logger.ErrorField(errUpdate), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = errUpdate.Error()
				return
			}

			jsonSummary, err := json.Marshal(summary)
			if err != nil {
				s.logger.ErrorContextWithAlert(ctx, "Failed to marshal summary", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = err.Error()
				return
			}

			sendTelegramToUsers := []model.StockPosition{}
			stockPositionMonitoring := model.StockPositionMonitoring{
				StockPositionID:   stockPosition.ID,
				EvaluationSummary: jsonSummary,
				MarketPrice:       stockAnalyses[0].MarketPrice,
				Timestamp:         utils.TimeNowWIB(),
			}
			stockPositionMonitoring.HashIdentifier = s.GenerateHashIdentifier(&stockPositionMonitoring)

			for _, stockAnalysis := range stockAnalyses {
				stockPositionMonitoring.StockPositionMonitoringAnalysisRefs = append(stockPositionMonitoring.StockPositionMonitoringAnalysisRefs, model.StockPositionMonitoringAnalysisRef{
					StockPositionMonitoringID: stockPositionMonitoring.ID,
					StockAnalysisID:           stockAnalysis.ID,
				})
			}

			shouldSendTelegram := (summary.TechnicalAnalysis.Status == string(dto.Warning) && lastScore < stockPosition.FinalScore) ||
				summary.TechnicalAnalysis.Status == string(dto.Dangerous) ||
				isTrailing

			if shouldSendTelegram {
				sendTelegramToUsers = append(sendTelegramToUsers, stockPosition)
			}
			err = s.stockPositionMonitoringRepo.Create(ctx, &stockPositionMonitoring)
			if err != nil {
				s.logger.ErrorContextWithAlert(ctx, "Failed to create stock position monitoring", logger.ErrorField(err))
				resultData.Errors = err.Error()
				return
			}

			if len(sendTelegramToUsers) > 0 {
				s.SendMessageUser(ctx, sendTelegramToUsers, stockAnalyses, summary)
			}

		}).Run()

	}

	wg.Wait()

	return results, nil
}

func (s *StockPositionMonitoringStrategy) GenerateHashIdentifier(data *model.StockPositionMonitoring) string {
	parts := []string{
		data.StockPosition.StockCode,
		data.StockPosition.Exchange,
		fmt.Sprintf("%d", data.Timestamp.Unix()),
		fmt.Sprintf("%f", data.MarketPrice),
	}

	hashInput := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

func (s *StockPositionMonitoringStrategy) SendMessageUser(ctx context.Context, stockPositions []model.StockPosition, stockAnalyses []model.StockAnalysis, summary model.PositionAnalysisSummary) error {

	marketPrice := stockAnalyses[0].MarketPrice
	for _, stockPosition := range stockPositions {
		sb := strings.Builder{}
		if summary.PositionSignal == string(dto.TrailingStop) && summary.TechnicalAnalysis.Status == string(dto.Safe) {
			sb.WriteString(fmt.Sprintf("<b>💰 Posisi Saham %s:%s amankan profit!</b>\n", stockPosition.Exchange, stockPosition.StockCode))
		} else if summary.PositionSignal == string(dto.TrailingProfit) && summary.TechnicalAnalysis.Status == string(dto.Safe) {
			sb.WriteString(fmt.Sprintf("<b>💰 Posisi Saham %s:%s naikkan profit!</b>\n", stockPosition.Exchange, stockPosition.StockCode))
		} else {
			sb.WriteString(fmt.Sprintf("<b>⚠️ Posisi Saham %s:%s mulai melemah!</b>\n", stockPosition.Exchange, stockPosition.StockCode))
		}
		sb.WriteString(fmt.Sprintf("<i>📅 Update :%s</i>\n", utils.PrettyDate(utils.TimeNowWIB())))
		sb.WriteString(fmt.Sprintf(`
<b>📊 Evaluasi Terbaru</b>
 - Position Signal: %s	
 - Score : %.2f ⮕ %.2f (%s)
 - Status: %s
`, dto.Signal(summary.PositionSignal), stockPosition.InitialScore, summary.TechnicalAnalysis.Score, summary.TechnicalAnalysis.Signal, dto.PositionStatus(summary.TechnicalAnalysis.Status)))

		sb.WriteString(fmt.Sprintf(`
🧾 <b>Informasi Posisi</b>
 - Current Price: %.2f
 - Entry Price: %.2f
 - Target Price: %.2f (%s)
 - Stop Loss: %.2f (%s)
 - PnL: %s`, marketPrice,
			stockPosition.BuyPrice,
			stockPosition.TakeProfitPrice,
			utils.FormatChange(float64(stockPosition.BuyPrice),
				float64(stockPosition.TakeProfitPrice)),
			stockPosition.StopLossPrice,
			utils.FormatChange(float64(stockPosition.BuyPrice),
				float64(stockPosition.StopLossPrice)),
			utils.FormatChangeWithIcon(float64(stockPosition.BuyPrice),
				float64(marketPrice))))

		sb.WriteString("\n")

		if summary.PositionSignal == string(dto.TrailingStop) {
			sb.WriteString(fmt.Sprintf(" - Trailing Stop: %.2f (%s)\n", stockPosition.TrailingStopPrice, utils.FormatChange(float64(stockPosition.BuyPrice),
				float64(stockPosition.TrailingStopPrice))))
		}

		if summary.PositionSignal == string(dto.TrailingProfit) {
			sb.WriteString(fmt.Sprintf(" - Trailing Take Profit: %.2f (%s)\n", stockPosition.TrailingProfitPrice, utils.FormatChange(float64(stockPosition.BuyPrice),
				float64(stockPosition.TrailingProfitPrice))))
		}

		sb.WriteString("\n<b>🧠 Insight:</b>\n")
		counter := 0
		for _, insight := range summary.TechnicalAnalysis.Insight {
			if counter >= s.cfg.Telegram.MaxShowAnalyzeInsight {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", insight.Text))
			counter++
		}

		menu := &telebot.ReplyMarkup{}

		btnDetail := menu.Data("🔍 Detail Posisi", "btn_detail_stock_position", fmt.Sprintf("%d", stockPosition.ID))
		btnAskAI := menu.Data("🤖 Analisa oleh AI", "btn_position_ask_ai_analyzer", fmt.Sprintf("%d", stockPosition.ID))
		btnExitPosition := menu.Data("📤 Keluar dari Posisi", "btn_exit_stock_position", fmt.Sprintf("%s|%d", stockPosition.Exchange+":"+stockPosition.StockCode, stockPosition.ID))
		btnDeleteMessage := menu.Data("🗑️ Hapus Pesan", "btn_delete_message")
		menu.Inline(
			menu.Row(btnDetail, btnAskAI),
			menu.Row(btnExitPosition, btnDeleteMessage),
		)

		err := s.telegram.SendMessageUser(ctx, sb.String(), stockPosition.User.TelegramID, menu, telebot.ModeHTML)
		if err != nil {
			s.logger.Error("Failed to send message to user", logger.ErrorField(err))
		}
	}

	return nil
}
