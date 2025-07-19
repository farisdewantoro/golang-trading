package repository

import (
	"context"
	"golang-trading/internal/dto"
	"golang-trading/pkg/common"
)

type CandleRepository interface {
	Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error)
}

type candleRepository struct {
	binanceRepo BinanceRepository
	yahooRepo   YahooFinanceRepository
}

func NewCandleRepository(binanceRepo BinanceRepository, yahooRepo YahooFinanceRepository) CandleRepository {
	return &candleRepository{
		binanceRepo: binanceRepo,
		yahooRepo:   yahooRepo,
	}
}

func (r *candleRepository) Get(ctx context.Context, param dto.GetStockDataParam) (*dto.StockData, error) {
	if param.Exchange == common.EXCHANGE_BINANCE {
		return r.binanceRepo.Get(ctx, param)
	}

	return r.yahooRepo.Get(ctx, param)
}
