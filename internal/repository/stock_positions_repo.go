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
	Get(ctx context.Context, param dto.GetStockPositionsParam, opts ...utils.DBOption) ([]model.StockPosition, error)
	Update(ctx context.Context, stockPosition model.StockPosition, opts ...utils.DBOption) error
	Create(ctx context.Context, stockPosition *model.StockPosition, opts ...utils.DBOption) error
	Delete(ctx context.Context, stockPosition *model.StockPosition, opts ...utils.DBOption) error
}

type stockPositionsRepository struct {
	db *gorm.DB
}

func NewStockPositionsRepository(db *gorm.DB) StockPositionsRepository {
	return &stockPositionsRepository{
		db: db,
	}
}

func (r *stockPositionsRepository) Get(ctx context.Context, param dto.GetStockPositionsParam, opts ...utils.DBOption) ([]model.StockPosition, error) {
	var stockPositions []model.StockPosition

	qFilter := []string{}
	qFilterParam := []interface{}{}

	db := utils.ApplyOptions(r.db.WithContext(ctx), opts...)

	if param.TelegramID != nil {
		db = db.Debug().Joins("JOIN users ON stock_positions.user_id = users.id").
			Where("users.telegram_id = ?", *param.TelegramID)
	}

	if len(param.IDs) > 0 {
		qFilter = append(qFilter, "stock_positions.id IN (?)")
		qFilterParam = append(qFilterParam, param.IDs)
	}

	if param.PriceAlert != nil {
		qFilter = append(qFilter, "stock_positions.price_alert = ?")
		qFilterParam = append(qFilterParam, *param.PriceAlert)
	}

	if len(param.StockCodes) > 0 {
		qFilter = append(qFilter, "stock_positions.stock_code IN (?)")
		qFilterParam = append(qFilterParam, param.StockCodes)
	}

	if param.MonitorPosition != nil {
		qFilter = append(qFilter, "stock_positions.monitor_position = ?")
		qFilterParam = append(qFilterParam, *param.MonitorPosition)
	}

	if param.IsActive != nil {
		qFilter = append(qFilter, "stock_positions.is_active = ?")
		qFilterParam = append(qFilterParam, *param.IsActive)
	}

	if param.Exchange != nil {
		qFilter = append(qFilter, "stock_positions.exchange = ?")
		qFilterParam = append(qFilterParam, *param.Exchange)
	}

	if param.UserID != nil {
		qFilter = append(qFilter, "stock_positions.user_id = ?")
		qFilterParam = append(qFilterParam, *param.UserID)
	}

	if param.IsExit != nil {
		qFilter = append(qFilter, "stock_positions.is_active = false and stock_positions.exit_price is not null")
	}

	if len(qFilter) == 0 {
		return nil, fmt.Errorf("no filter provided")
	}

	if param.Monitoring != nil {
		// Preload monitoring dengan order by
		db = db.Preload("StockPositionMonitorings", func(pDb *gorm.DB) *gorm.DB {

			if param.Monitoring.ShowNewest != nil && *param.Monitoring.ShowNewest {
				pDb = pDb.Order("created_at DESC")
			}

			if param.Monitoring.Limit != nil && *param.Monitoring.Limit > 0 {
				pDb = pDb.Limit(*param.Monitoring.Limit)
			}
			return pDb
		})
		db = db.Preload("StockPositionMonitorings.StockAnalysisAI")
	}

	if param.SortBy != nil && param.SortOrder != nil {
		if *param.SortBy == "exit_date" {
			db = db.Order("stock_positions.exit_date " + *param.SortOrder)
		}
	}

	if err := db.Preload("User").Debug().Where(strings.Join(qFilter, " AND "), qFilterParam...).Find(&stockPositions).Error; err != nil {
		return nil, err
	}

	return stockPositions, nil
}

func (r *stockPositionsRepository) Update(ctx context.Context, stockPosition model.StockPosition, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Updates(&stockPosition).Error
}

func (r *stockPositionsRepository) Create(ctx context.Context, stockPosition *model.StockPosition, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Create(&stockPosition).Error
}

func (r *stockPositionsRepository) Delete(ctx context.Context, stockPosition *model.StockPosition, opts ...utils.DBOption) error {
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)
	return tx.Delete(stockPosition).Error
}
