package repository

import (
	"golang-trading/config"
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
}

func NewRepository(cfg *config.Config, db *gorm.DB, log *logger.Logger) (*Repository, error) {
	uow := NewUnitOfWork(db)
	geminiAIRepo, err := NewGeminiAIRepository(db, cfg, log)
	if err != nil {
		return nil, err
	}

	return &Repository{
		JobRepo:                  NewJobRepository(db),
		StockPositionsRepo:       NewStockPositionsRepository(db),
		TradingViewScreenersRepo: NewTradingViewScreenersRepository(cfg, log),
		YahooFinanceRepo:         NewYahooFinanceRepository(cfg, log),
		StockAnalysisRepo:        NewStockAnalysisRepository(db),
		SystemParamRepo:          NewSystemParamRepository(db),
		GeminiAIRepo:             geminiAIRepo,
		UnitOfWork:               uow,
	}, nil
}
