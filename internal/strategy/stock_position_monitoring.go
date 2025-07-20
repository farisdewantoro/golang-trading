package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

type StockPositionMonitoringStrategy struct {
	logger                         *logger.Logger
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
	inmemoryCache cache.Cache,
	tradingViewScreenersRepository repository.TradingViewScreenersRepository,
	telegram *telegram.TelegramRateLimiter,
	stockPositionsRepository repository.StockPositionsRepository,
	stockAnalyzer StockAnalyzer,
	stockPositionMonitoringRepo repository.StockPositionMonitoringRepository,
	systemParamRepository repository.SystemParamRepository,
	tradingPositionService contract.TradingPositionContract,
) JobExecutionStrategy {
	return &StockPositionMonitoringStrategy{
		logger:                         logger,
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

	mapStockCode := map[string][]model.StockPosition{}
	for _, stockPosition := range stockPositions {

		symbol := stockPosition.Exchange + ":" + stockPosition.StockCode
		if _, ok := mapStockCode[symbol]; ok {
			mapStockCode[symbol] = append(mapStockCode[symbol], stockPosition)
			continue
		} else {
			mapStockCode[symbol] = []model.StockPosition{stockPosition}
		}

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
			positionAnalysis, err := s.tradingPositionService.EvaluatePositionMonitoring(ctx, &stockPosition, stockAnalyses)
			if err != nil {
				s.logger.ErrorContextWithAlert(ctx, "Failed to evaluate signal", logger.ErrorField(err), logger.StringField("stock_code", stockPosition.StockCode))
				resultData.Errors = err.Error()
				return
			}

			isTrailing := positionAnalysis.TrailingProfitPrice > stockPosition.TrailingProfitPrice ||
				positionAnalysis.TrailingStopPrice > stockPosition.TrailingStopPrice

			summary := model.PositionAnalysisSummary{
				TechnicalAnalysis: model.PositionTechnicalAnalysisSummary{
					Signal:           string(positionAnalysis.TechnicalSignal),
					Score:            positionAnalysis.Score,
					Insight:          positionAnalysis.Insight,
					Status:           string(positionAnalysis.Status),
					IndicatorSummary: positionAnalysis.IndicatorSummary,
				},
				PositionSignal: string(positionAnalysis.Signal),
			}
			if positionAnalysis.HighestPriceSinceTTP > stockPosition.HighestPriceSinceTTP {
				stockPosition.HighestPriceSinceTTP = positionAnalysis.HighestPriceSinceTTP
			}
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

			stockPositionMonitorings := []model.StockPositionMonitoring{}
			sendTelegramToUsers := []model.StockPosition{}
			for _, stockPosition := range mapStockCode[symbol] {
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
				stockPositionMonitorings = append(stockPositionMonitorings, stockPositionMonitoring)

				shouldSendTelegram := (summary.TechnicalAnalysis.Status == string(dto.Warning) && lastScore < stockPosition.FinalScore) ||
					summary.TechnicalAnalysis.Status == string(dto.Dangerous) ||
					isTrailing

				if shouldSendTelegram {
					sendTelegramToUsers = append(sendTelegramToUsers, stockPosition)
				}
			}
			err = s.stockPositionMonitoringRepo.CreateBulk(ctx, stockPositionMonitorings)
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
 - Score : %.2f (%s)
 - Status: %s
`, dto.Signal(summary.PositionSignal), summary.TechnicalAnalysis.Score, summary.TechnicalAnalysis.Signal, dto.PositionStatus(summary.TechnicalAnalysis.Status)))

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
		for _, insight := range summary.TechnicalAnalysis.Insight {
			sb.WriteString(fmt.Sprintf("- %s\n", insight))
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
