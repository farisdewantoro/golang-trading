package repository

import (
	"golang-trading/config"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"

	"gorm.io/gorm"
)

type Repository struct {
	JobRepo                  JobRepository
	StockPositionsRepo       StockPositionsRepository
	TradingViewScreenersRepo TradingViewScreenersRepository
	YahooFinanceRepo         YahooFinanceRepository
	StockAnalysisRepo        StockAnalysisRepository
	SystemParamRepo          SystemParamRepository
	GeminiAIRepo             AIRepository
	UnitOfWork               UnitOfWork
	UserRepo                 UserRepository
}

func NewRepository(cfg *config.Config, inmemoryCache cache.Cache, db *gorm.DB, log *logger.Logger) (*Repository, error) {
	uow := NewUnitOfWork(db)
	geminiAIRepo, err := NewGeminiAIRepository(db, cfg, log)
	if err != nil {
		return nil, err
	}
	userRepo := NewUserRepository(db)

	return &Repository{
		JobRepo:                  NewJobRepository(db),
		StockPositionsRepo:       NewStockPositionsRepository(db),
		TradingViewScreenersRepo: NewTradingViewScreenersRepository(cfg, log),
		YahooFinanceRepo:         NewYahooFinanceRepository(cfg, log),
		StockAnalysisRepo:        NewStockAnalysisRepository(db),
		SystemParamRepo:          NewSystemParamRepository(cfg, inmemoryCache, db),
		GeminiAIRepo:             geminiAIRepo,
		UnitOfWork:               uow,
		UserRepo:                 userRepo,
	}, nil
}
