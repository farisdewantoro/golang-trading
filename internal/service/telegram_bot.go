package service

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/helper"
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
	AnalyzeStock(ctx context.Context, c telebot.Context) ([]model.StockAnalysis, error)
	AnalyzeStockAI(ctx context.Context, c telebot.Context) (*dto.AIAnalyzeStockResponse, error)
	EvaluateSignal(ctx context.Context, latestAnalyses []model.StockAnalysis) (string, error)
	SetStockPosition(ctx context.Context, data *dto.RequestSetPositionData) error
	GetStockPositions(ctx context.Context, param dto.GetStockPositionsParam) ([]model.StockPosition, error)
}

type telegramBotService struct {
	log                     *logger.Logger
	cfg                     *config.Config
	telegram                *telegram.TelegramRateLimiter
	inmemoryCache           cache.Cache
	stockAnalysisRepository repository.StockAnalysisRepository
	systemParamRepository   repository.SystemParamRepository
	stockAnalyzer           strategy.StockAnalyzer
	aiRepository            repository.AIRepository
	userRepo                repository.UserRepository
	stockPositionRepository repository.StockPositionsRepository
	uow                     repository.UnitOfWork
}

func NewTelegramBotService(
	log *logger.Logger,
	cfg *config.Config,
	telegram *telegram.TelegramRateLimiter,
	inmemoryCache cache.Cache,
	stockAnalysisRepository repository.StockAnalysisRepository,
	systemParamRepository repository.SystemParamRepository,
	stockAnalyzer strategy.StockAnalyzer,
	aiRepository repository.AIRepository,
	userRepo repository.UserRepository,
	stockPositionRepository repository.StockPositionsRepository,
	uow repository.UnitOfWork,
) TelegramBotService {
	return &telegramBotService{
		log:                     log,
		cfg:                     cfg,
		telegram:                telegram,
		inmemoryCache:           inmemoryCache,
		stockAnalysisRepository: stockAnalysisRepository,
		systemParamRepository:   systemParamRepository,
		stockAnalyzer:           stockAnalyzer,
		aiRepository:            aiRepository,
		userRepo:                userRepo,
		stockPositionRepository: stockPositionRepository,
		uow:                     uow,
	}
}

func (s *telegramBotService) AnalyzeStock(ctx context.Context, c telebot.Context) ([]model.StockAnalysis, error) {
	symbol := c.Text()

	symbol = strings.ToUpper(symbol)
	stockCode, exchange, err := utils.ParseStockSymbol(symbol)

	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse stock symbol", logger.ErrorField(err))
		return nil, err
	}

	return s.GetLatestAnalyses(ctx, stockCode, exchange)
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

func (s *telegramBotService) AnalyzeStockAI(ctx context.Context, c telebot.Context) (*dto.AIAnalyzeStockResponse, error) {
	var result dto.AIAnalyzeStockResponse

	data := c.Data()
	stockCode, exchange, err := utils.ParseStockSymbol(data)

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

func (s *telegramBotService) EvaluateSignal(ctx context.Context, latestAnalyses []model.StockAnalysis) (string, error) {

	// Ambil konfigurasi bobot timeframe
	dtf, err := s.systemParamRepository.GetDefaultAnalysisTimeframes(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get default analysis timeframes", logger.ErrorField(err))
		return "", err
	}

	_, signal, err := helper.EvaluateSignal(ctx, s.log, dtf, latestAnalyses)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to evaluate signal", logger.ErrorField(err))
		return "", err
	}
	return signal, nil
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
