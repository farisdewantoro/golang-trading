package service

import (
	"golang-trading/config"
	"golang-trading/internal/repository"
	"golang-trading/internal/strategy"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
)

type Service struct {
	SchedulerService   SchedulerService
	TaskExecutor       TaskExecutor
	TelegramBotService TelegramBotService
	TradingService     TradingService
	BacktestService    BacktestService
	SendSignalService  SendSignalService
}

func NewService(
	cfg *config.Config,
	log *logger.Logger,
	repo *repository.Repository,
	inmemoryCache cache.Cache,
	telegram *telegram.TelegramRateLimiter,
) *Service {
	tradingService := NewTradingService(cfg, log, repo.SystemParamRepo)
	signalService := NewSendSignalService(cfg, log, telegram, repo.StockPositionsRepo, repo.UserSignalAlertRepo, tradingService, inmemoryCache)

	analyzerStrategy := strategy.NewStockAnalyzerStrategy(cfg, log, inmemoryCache, repo.StockPositionsRepo, repo.TradingViewScreenersRepo, repo.CandleRepo, repo.StockAnalysisRepo, repo.SystemParamRepo, repo.UserSignalAlertRepo, telegram, tradingService, signalService)
	buySignalGeneratorStrategy := strategy.NewBuySignalGeneratorStrategy(cfg, log, repo.CandleRepo, inmemoryCache, signalService, repo.StockAnalysisRepo)
	stockPositionMonitoringStrategy := strategy.NewStockPositionMonitoringStrategy(log, cfg, inmemoryCache, repo.TradingViewScreenersRepo, telegram, repo.StockPositionsRepo, analyzerStrategy, repo.StockPositionMonitoringRepo, repo.SystemParamRepo, tradingService)
	executorStrategies := make(map[strategy.JobType]strategy.JobExecutionStrategy)
	executorStrategies[strategy.JobTypeStockPriceAlert] = strategy.NewStockPriceAlertStrategy(cfg, log, inmemoryCache, repo.TradingViewScreenersRepo, telegram, repo.StockPositionsRepo, repo.CandleRepo)
	executorStrategies[strategy.JobTypeStockAnalyzer] = analyzerStrategy
	executorStrategies[strategy.JobTypeBuySignalGenerator] = buySignalGeneratorStrategy
	executorStrategies[strategy.JobTypeStockPositionMonitor] = stockPositionMonitoringStrategy
	executorStrategies[strategy.JobTypeDataCleanUp] = strategy.NewDataCleanUpStrategy(cfg, log, repo.StockAnalysisRepo, repo.JobRepo)

	taskExecutor := NewTaskExecutor(cfg, log, repo.JobRepo, executorStrategies)

	schedulerService := NewSchedulerService(cfg, log, repo.JobRepo, taskExecutor)
	telegramBotService := NewTelegramBotService(log, cfg, telegram, inmemoryCache, repo.StockAnalysisRepo, repo.SystemParamRepo, analyzerStrategy, stockPositionMonitoringStrategy, repo.GeminiAIRepo, repo.UserRepo, repo.StockPositionsRepo, repo.StockPositionMonitoringRepo, repo.UnitOfWork, repo.UserSignalAlertRepo)
	backtestService := NewBacktestService(log, tradingService, repo.StockAnalysisRepo)

	return &Service{
		SchedulerService:   schedulerService,
		TaskExecutor:       taskExecutor,
		TelegramBotService: telegramBotService,
		TradingService:     tradingService,
		BacktestService:    backtestService,
		SendSignalService:  signalService,
	}
}
