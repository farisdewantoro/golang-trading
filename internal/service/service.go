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
}

func NewService(
	cfg *config.Config,
	log *logger.Logger,
	repo *repository.Repository,
	inmemoryCache cache.Cache,
	telegram *telegram.TelegramRateLimiter,
) *Service {
	analyzerStrategy := strategy.NewStockAnalyzerStrategy(cfg, log, inmemoryCache, repo.StockPositionsRepo, repo.TradingViewScreenersRepo, repo.YahooFinanceRepo, repo.StockAnalysisRepo, repo.SystemParamRepo)
	stockPositionMonitoringStrategy := strategy.NewStockPositionMonitoringStrategy(log, inmemoryCache, repo.TradingViewScreenersRepo, telegram, repo.StockPositionsRepo, analyzerStrategy, repo.StockPositionMonitoringRepo, repo.SystemParamRepo)
	executorStrategies := make(map[strategy.JobType]strategy.JobExecutionStrategy)
	executorStrategies[strategy.JobTypeStockPriceAlert] = strategy.NewStockPriceAlertStrategy(log, inmemoryCache, repo.TradingViewScreenersRepo, telegram, repo.StockPositionsRepo)
	executorStrategies[strategy.JobTypeStockAnalyzer] = analyzerStrategy
	executorStrategies[strategy.JobTypeStockPositionMonitor] = stockPositionMonitoringStrategy

	taskExecutor := NewTaskExecutor(cfg, log, repo.JobRepo, executorStrategies)

	schedulerService := NewSchedulerService(cfg, log, repo.JobRepo, taskExecutor)
	telegramBotService := NewTelegramBotService(log, cfg, telegram, inmemoryCache, repo.StockAnalysisRepo, repo.SystemParamRepo, analyzerStrategy, repo.GeminiAIRepo, repo.UserRepo, repo.StockPositionsRepo, repo.UnitOfWork)
	return &Service{
		SchedulerService:   schedulerService,
		TaskExecutor:       taskExecutor,
		TelegramBotService: telegramBotService,
	}
}
