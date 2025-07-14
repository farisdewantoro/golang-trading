package contract

import (
	"context"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
)

type TradingPositionContract interface {
	EvaluatePositionMonitoring(ctx context.Context, stockPosition *model.StockPosition, analyses []model.StockAnalysis) (*dto.PositionAnalysis, error)
}
