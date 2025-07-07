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
	SchedulerService SchedulerService
	TaskExecutor     TaskExecutor
}

func NewService(
	cfg *config.Config,
	log *logger.Logger,
	repo *repository.Repository,
	inmemoryCache cache.Cache,
	telegram *telegram.TelegramRateLimiter,
) *Service {
	executorStrategies := make(map[strategy.JobType]strategy.JobExecutionStrategy)
	executorStrategies[strategy.JobTypeStockPriceAlert] = strategy.NewStockPriceAlertStrategy(log, inmemoryCache, repo.TradingViewScannerRepo, telegram, repo.StockPositionsRepo)
	taskExecutor := NewTaskExecutor(cfg, log, repo.JobRepo, executorStrategies)

	schedulerService := NewSchedulerService(cfg, log, repo.JobRepo, taskExecutor)
	return &Service{
		SchedulerService: schedulerService,
		TaskExecutor:     taskExecutor,
	}
}
