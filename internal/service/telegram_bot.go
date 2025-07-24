package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/internal/strategy"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"strings"

	"gopkg.in/telebot.v3"
)

type TelegramBotService interface {
	ExecuteStockAnalyzer(ctx context.Context, symbol string) ([]model.StockAnalysis, error)
	AnalyzeStock(ctx context.Context, c telebot.Context, symbol string) ([]model.StockAnalysis, error)
	AnalyzeStockAI(ctx context.Context, c telebot.Context, symbol string) (*dto.AIAnalyzeStockResponse, error)
	SetStockPosition(ctx context.Context, data *dto.RequestSetPositionData) error
	GetStockPositions(ctx context.Context, param dto.GetStockPositionsParam) ([]model.StockPosition, error)
	DeleteStockPositionTelegramUser(ctx context.Context, telegramID int64, stockPositionID uint) error
	GetDetailStockPosition(ctx context.Context, telegramID int64, stockPositionID uint) (*model.StockPosition, error)
	ExitStockPosition(ctx context.Context, telegramID int64, data *dto.RequestExitPositionData) error
	GetAllLatestAnalyses(ctx context.Context, exchange string) ([]model.StockAnalysis, error)
	GetAlertSignal(ctx context.Context, telegramID int64) ([]model.UserSignalAlert, error)
	SetAlertSignal(ctx context.Context, telegramID int64, exchange string, isActive bool) error
	AnalyzePosition(ctx context.Context, stockPosition model.StockPosition) error
}

type telegramBotService struct {
	log                               *logger.Logger
	cfg                               *config.Config
	telegram                          *telegram.TelegramRateLimiter
	inmemoryCache                     cache.Cache
	stockAnalysisRepository           repository.StockAnalysisRepository
	systemParamRepository             repository.SystemParamRepository
	stockAnalyzer                     strategy.StockAnalyzer
	positionMonitoringStrategy        strategy.PositionMonitoringEvaluator
	aiRepository                      repository.AIRepository
	userRepo                          repository.UserRepository
	stockPositionRepository           repository.StockPositionsRepository
	stockPositionMonitoringRepository repository.StockPositionMonitoringRepository
	uow                               repository.UnitOfWork
	userSignalAlertRepository         repository.UserSignalAlertRepository
}

func NewTelegramBotService(
	log *logger.Logger,
	cfg *config.Config,
	telegram *telegram.TelegramRateLimiter,
	inmemoryCache cache.Cache,
	stockAnalysisRepository repository.StockAnalysisRepository,
	systemParamRepository repository.SystemParamRepository,
	stockAnalyzer strategy.StockAnalyzer,
	positionMonitoringStrategy strategy.PositionMonitoringEvaluator,
	aiRepository repository.AIRepository,
	userRepo repository.UserRepository,
	stockPositionRepository repository.StockPositionsRepository,
	stockPositionMonitoringRepository repository.StockPositionMonitoringRepository,
	uow repository.UnitOfWork,
	userSignalAlertRepository repository.UserSignalAlertRepository,
) TelegramBotService {
	return &telegramBotService{
		log:                               log,
		cfg:                               cfg,
		telegram:                          telegram,
		inmemoryCache:                     inmemoryCache,
		stockAnalysisRepository:           stockAnalysisRepository,
		systemParamRepository:             systemParamRepository,
		stockAnalyzer:                     stockAnalyzer,
		positionMonitoringStrategy:        positionMonitoringStrategy,
		aiRepository:                      aiRepository,
		userRepo:                          userRepo,
		stockPositionRepository:           stockPositionRepository,
		stockPositionMonitoringRepository: stockPositionMonitoringRepository,
		uow:                               uow,
		userSignalAlertRepository:         userSignalAlertRepository,
	}
}

func (s *telegramBotService) AnalyzeStock(ctx context.Context, c telebot.Context, symbol string) ([]model.StockAnalysis, error) {

	symbol = strings.ToUpper(symbol)
	stockCode, exchange, err := utils.ParseStockSymbol(symbol)

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse stock symbol", logger.ErrorField(err))
		return nil, err
	}

	return s.GetLatestAnalyses(ctx, stockCode, exchange)
}

func (s *telegramBotService) ExecuteStockAnalyzer(ctx context.Context, symbol string) ([]model.StockAnalysis, error) {
	symbol = strings.ToUpper(symbol)
	stockCode, exchange, err := utils.ParseStockSymbol(symbol)

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse stock symbol", logger.ErrorField(err))
		return nil, err
	}

	latestAnalyses, err := s.stockAnalyzer.AnalyzeStock(ctx, dto.StockInfo{
		StockCode: stockCode,
		Exchange:  exchange,
	})
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to analyze stock", logger.ErrorField(err))
		return nil, err
	}

	return latestAnalyses, nil
}

func (s *telegramBotService) GetLatestAnalyses(ctx context.Context, stockCode string, exchange string) ([]model.StockAnalysis, error) {
	latestAnalyses, err := s.stockAnalysisRepository.GetLatestAnalyses(ctx, model.GetLatestAnalysisParam{
		StockCode:       stockCode,
		Exchange:        exchange,
		TimestampAfter:  utils.TimeNowWIB().Add(-s.cfg.Telegram.FeatureStockAnalyze.AfterTimestampDuration),
		ExpectedTFCount: s.cfg.Telegram.FeatureStockAnalyze.ExpectedTFCount,
	})

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get latest analyses", logger.ErrorField(err))
		return nil, err
	}
	if len(latestAnalyses) == 0 {
		latestAnalyses, err = s.stockAnalyzer.AnalyzeStock(ctx, dto.StockInfo{
			StockCode: stockCode,
			Exchange:  exchange,
		})
		if err != nil {
			s.log.ErrorContext(ctx, "Failed to analyze stock", logger.ErrorField(err))
			return nil, err
		}
	}

	s.log.DebugContext(ctx, "Found latest analyses", logger.IntField("count", len(latestAnalyses)), logger.StringField("stock_code", stockCode), logger.StringField("exchange", exchange))
	return latestAnalyses, nil
}

func (s *telegramBotService) AnalyzeStockAI(ctx context.Context, c telebot.Context, symbol string) (*dto.AIAnalyzeStockResponse, error) {
	var result dto.AIAnalyzeStockResponse

	stockCode, exchange, err := utils.ParseStockSymbol(symbol)

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse stock symbol", logger.ErrorField(err))
		return nil, err
	}

	latestAnalyses, err := s.GetLatestAnalyses(ctx, stockCode, exchange)

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get latest analyses", logger.ErrorField(err))
		return nil, err
	}

	if len(latestAnalyses) == 0 {
		err := fmt.Errorf("no data when analyze stock")
		s.log.ErrorContext(ctx, "Failed to analyze stock no data", logger.ErrorField(err))
		return nil, err
	}

	existStockAnalysisAI := latestAnalyses[0].StockAnalysisAI
	if existStockAnalysisAI != nil {
		err := json.Unmarshal(existStockAnalysisAI.Response, &result)
		if err != nil {
			s.log.ErrorContext(ctx, "Failed to unmarshal stock analysis AI", logger.ErrorField(err))
			return nil, err
		}
		return &result, nil
	}

	return s.aiRepository.AnalyzeStock(ctx, latestAnalyses)
}

func (s *telegramBotService) SetStockPosition(ctx context.Context, data *dto.RequestSetPositionData) error {
	user, err := s.userRepo.GetUserByTelegramID(ctx, data.UserTelegram.ID)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get user", logger.ErrorField(err))
		return fmt.Errorf("failed to get user: %w", err)
	}

	positions, err := s.stockPositionRepository.Get(ctx, dto.GetStockPositionsParam{
		TelegramID: &data.UserTelegram.ID,
		StockCodes: []string{data.StockCode},
		Exchange:   &data.Exchange,
		IsActive:   utils.ToPointer(true),
	})
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get stock position", logger.ErrorField(err))
		return fmt.Errorf("failed to get stock position: %w", err)
	}
	if len(positions) > 0 {
		s.log.WarnContext(ctx, "position already exists", logger.IntField("count", len(positions)))
		return fmt.Errorf("position already exists")
	}

	err = s.uow.Run(func(opts ...utils.DBOption) error {
		if user == nil {
			user = data.UserTelegram.ToUserEntity()
			err = s.userRepo.CreateUser(ctx, user, opts...)
			if err != nil {
				s.log.ErrorContext(ctx, "Failed to create user", logger.ErrorField(err))
				return fmt.Errorf("failed to create user: %w", err)
			}
		}

		stockPosition := data.ToStockPositionEntity()
		stockPosition.UserID = user.ID
		stockPosition.IsActive = utils.ToPointer(true)

		err = s.stockPositionRepository.Create(ctx, stockPosition, opts...)
		if err != nil {
			s.log.ErrorContext(ctx, "Failed to create stock position", logger.ErrorField(err))
			return fmt.Errorf("failed to create stock position: %w", err)
		}
		return nil
	})
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to create stock position", logger.ErrorField(err))
		return fmt.Errorf("failed to create stock position: %w", err)
	}

	return nil
}

func (s *telegramBotService) GetStockPositions(ctx context.Context, param dto.GetStockPositionsParam) ([]model.StockPosition, error) {
	positions, err := s.stockPositionRepository.Get(ctx, param)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get stock positions", logger.ErrorField(err))
		return nil, fmt.Errorf("failed to get stock positions: %w", err)
	}
	return positions, nil
}

func (s *telegramBotService) DeleteStockPositionTelegramUser(ctx context.Context, telegramID int64, stockPositionID uint) error {
	positions, err := s.stockPositionRepository.Get(ctx, dto.GetStockPositionsParam{
		TelegramID: &telegramID,
		IDs:        []uint{stockPositionID},
	})
	if err != nil {
		return fmt.Errorf("failed to get stock positions: %w", err)
	}

	if len(positions) == 0 {
		return fmt.Errorf("position not found")
	}

	return s.stockPositionRepository.Delete(ctx, &positions[0])
}

func (s *telegramBotService) GetDetailStockPosition(ctx context.Context, telegramID int64, stockPositionID uint) (*model.StockPosition, error) {
	positions, err := s.stockPositionRepository.Get(ctx, dto.GetStockPositionsParam{
		TelegramID: &telegramID,
		IDs:        []uint{stockPositionID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get stock positions: %w", err)
	}

	if len(positions) == 0 {
		return nil, fmt.Errorf("position not found")
	}

	monitorings, err := s.stockPositionMonitoringRepository.GetRecentDistinctMonitorings(ctx, model.StockPositionMonitoringQueryParam{
		StockPositionID:   positions[0].ID,
		Limit:             utils.ToPointer(s.cfg.Telegram.FeatureMyPosition.LimitRecentMonitoring),
		WithStockAnalysis: utils.ToPointer(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get stock position monitorings: %w", err)
	}
	positions[0].StockPositionMonitorings = monitorings

	return &positions[0], nil
}

func (s *telegramBotService) ExitStockPosition(ctx context.Context, telegramID int64, data *dto.RequestExitPositionData) error {
	positions, err := s.stockPositionRepository.Get(ctx, dto.GetStockPositionsParam{
		TelegramID: &telegramID,
		IDs:        []uint{data.StockPositionID},
		Monitoring: &dto.StockPositionMonitoringQueryParam{
			ShowNewest: utils.ToPointer(true),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get stock positions: %w", err)
	}

	if len(positions) == 0 {
		return fmt.Errorf("position not found")
	}

	if !data.ExitDate.IsZero() {
		positions[0].ExitDate = &data.ExitDate
	}

	if data.ExitPrice > 0 {
		positions[0].ExitPrice = &data.ExitPrice
	}

	if len(positions[0].StockPositionMonitorings) > 0 {
		monitoring := positions[0].StockPositionMonitorings[0]
		var evalSummary model.PositionTechnicalAnalysisSummary
		err := json.Unmarshal(monitoring.EvaluationSummary, &evalSummary)
		if err != nil {
			return fmt.Errorf("failed to unmarshal eval summary: %w", err)
		}
		positions[0].FinalScore = evalSummary.Score
	}

	positions[0].IsActive = utils.ToPointer(false)

	return s.stockPositionRepository.Update(ctx, positions[0])
}

func (s *telegramBotService) GetAllLatestAnalyses(ctx context.Context, exchange string) ([]model.StockAnalysis, error) {

	latestAnalyses, err := s.stockAnalysisRepository.GetLatestAnalyses(ctx, model.GetLatestAnalysisParam{
		TimestampAfter:  utils.TimeNowWIB().Add(-s.cfg.Telegram.FeatureStockAnalyze.AfterTimestampDuration),
		ExpectedTFCount: s.cfg.Telegram.FeatureStockAnalyze.ExpectedTFCount,
		Exchange:        exchange,
	})

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get latest analyses", logger.ErrorField(err))
		return nil, err
	}
	if len(latestAnalyses) == 0 {
		return nil, nil
	}

	s.log.DebugContext(ctx, "Found latest analyses", logger.IntField("count", len(latestAnalyses)))
	return latestAnalyses, nil
}

func (s *telegramBotService) GetAlertSignal(ctx context.Context, telegramID int64) ([]model.UserSignalAlert, error) {
	return s.userSignalAlertRepository.Get(ctx, &model.GetUserSignalAlertParam{
		TelegramID: &telegramID,
	})
}

func (s *telegramBotService) SetAlertSignal(ctx context.Context, telegramID int64, exchange string, isActive bool) error {
	userSignalAlerts, err := s.userSignalAlertRepository.Get(ctx, &model.GetUserSignalAlertParam{
		TelegramID: &telegramID,
		Exchange:   &exchange,
	})
	if err != nil {
		return err
	}
	if len(userSignalAlerts) == 0 {
		return fmt.Errorf("user signal alert not found")
	}

	userSignalAlerts[0].IsActive = utils.ToPointer(isActive)
	return s.userSignalAlertRepository.Update(ctx, &userSignalAlerts[0])
}

func (s *telegramBotService) AnalyzePosition(ctx context.Context, stockPosition model.StockPosition) error {
	_, err := s.positionMonitoringStrategy.EvaluateStockPosition(ctx, []model.StockPosition{stockPosition})
	return err
}
