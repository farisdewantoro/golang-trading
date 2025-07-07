package repository

import (
	"golang-trading/config"
	"golang-trading/pkg/logger"

	"gorm.io/gorm"
)

type Repository struct {
	JobRepo                JobRepository
	StockPositionsRepo     StockPositionsRepository
	TradingViewScannerRepo TradingViewScannerRepository
}

func NewRepository(cfg *config.Config, db *gorm.DB, log *logger.Logger) *Repository {
	return &Repository{
		JobRepo:                NewJobRepository(db),
		StockPositionsRepo:     NewStockPositionsRepository(db),
		TradingViewScannerRepo: NewTradingViewScannerRepository(cfg, log),
	}
}
