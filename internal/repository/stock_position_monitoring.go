package repository

import (
	"context"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"

	"gorm.io/gorm"
)

type StockPositionMonitoringRepository interface {
	Create(ctx context.Context, stockPosition *model.StockPositionMonitoring, opts ...utils.DBOption) error
	CreateBulk(ctx context.Context, stockPositions []model.StockPositionMonitoring, opts ...utils.DBOption) error
}

type stockPositionMonitoringRepository struct {
	db *gorm.DB
}

func NewStockPositionMonitoringRepository(db *gorm.DB) StockPositionMonitoringRepository {
	return &stockPositionMonitoringRepository{
		db: db,
	}
}

func (r *stockPositionMonitoringRepository) Create(ctx context.Context, stockPosition *model.StockPositionMonitoring, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Create(&stockPosition).Error
}

func (r *stockPositionMonitoringRepository) CreateBulk(ctx context.Context, stockPositions []model.StockPositionMonitoring, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Create(&stockPositions).Error
}
