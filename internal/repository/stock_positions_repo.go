package repository

import (
	"context"
	"fmt"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"strings"

	"gorm.io/gorm"
)

type StockPositionsRepository interface {
	Get(ctx context.Context, param dto.GetStockPositionsParam) ([]model.StockPosition, error)
	Update(ctx context.Context, stockPosition model.StockPosition, opts ...utils.DBOption) error
}

type stockPositionsRepository struct {
	db *gorm.DB
}

func NewStockPositionsRepository(db *gorm.DB) StockPositionsRepository {
	return &stockPositionsRepository{
		db: db,
	}
}

func (r *stockPositionsRepository) Get(ctx context.Context, param dto.GetStockPositionsParam) ([]model.StockPosition, error) {
	var stockPositions []model.StockPosition

	qFilter := []string{}
	qFilterParam := []interface{}{}

	if len(param.IDs) > 0 {
		qFilter = append(qFilter, "id IN (?)")
		qFilterParam = append(qFilterParam, param.IDs)
	}

	if param.PriceAlert != nil {
		qFilter = append(qFilter, "price_alert = ?")
		qFilterParam = append(qFilterParam, *param.PriceAlert)
	}

	if len(param.StockCodes) > 0 {
		qFilter = append(qFilter, "stock_code IN (?)")
		qFilterParam = append(qFilterParam, param.StockCodes)
	}

	if param.MonitorPosition != nil {
		qFilter = append(qFilter, "monitor_position = ?")
		qFilterParam = append(qFilterParam, *param.MonitorPosition)
	}

	if param.IsActive != nil {
		qFilter = append(qFilter, "is_active = ?")
		qFilterParam = append(qFilterParam, *param.IsActive)
	}

	if len(qFilter) == 0 {
		return nil, fmt.Errorf("no filter provided")
	}

	if err := r.db.WithContext(ctx).Preload("User").Where(strings.Join(qFilter, " AND "), qFilterParam...).Find(&stockPositions).Error; err != nil {
		return nil, err
	}

	return stockPositions, nil
}

func (r *stockPositionsRepository) Update(ctx context.Context, stockPosition model.StockPosition, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Updates(&stockPosition).Error
}
