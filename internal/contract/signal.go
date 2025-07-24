package contract

import (
	"context"
	"golang-trading/internal/model"
)

type SignalContract interface {
	SendBuySignal(ctx context.Context, analyses []model.StockAnalysis, minScore float64) (bool, error)
}
