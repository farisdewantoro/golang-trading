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
	GetRecentDistinctMonitorings(ctx context.Context, param model.StockPositionMonitoringQueryParam, opts ...utils.DBOption) ([]model.StockPositionMonitoring, error)
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

func (r *stockPositionMonitoringRepository) GetRecentDistinctMonitorings(ctx context.Context, param model.StockPositionMonitoringQueryParam, opts ...utils.DBOption) ([]model.StockPositionMonitoring, error) {
	var results []model.StockPositionMonitoring

	query := `
	WITH ranked AS (
	  SELECT
	    id,
	    stock_position_id,
	    evaluation_summary,
	    "timestamp",
	    created_at,
	    market_price,
	    evaluation_summary->>'technical_score' as technical_score,
	    LAG(evaluation_summary->>'technical_score') OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_tech_score,
	    LAG(market_price) OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_price

	  FROM stock_position_monitorings
	  WHERE stock_position_id = ? AND deleted_at IS NULL
	),
	filtered AS (
	  SELECT *
	  FROM ranked
	  WHERE
	    prev_tech_score IS NULL OR
	    technical_score IS DISTINCT FROM prev_tech_score OR
	    market_price IS DISTINCT FROM prev_price
	)
	SELECT *
	FROM filtered
	ORDER BY created_at DESC
	LIMIT ?
	`

	err := utils.ApplyOptions(r.db.WithContext(ctx), opts...).Raw(query, param.StockPositionID, param.Limit).Scan(&results).Error

	if err != nil {
		return nil, err
	}

	stockMonitoringIds := make([]uint, len(results))
	stockMonitoringMapIdx := make(map[uint]int)
	for i, result := range results {
		stockMonitoringIds[i] = result.ID
		stockMonitoringMapIdx[result.ID] = i
	}

	if param.WithStockAnalysis != nil && *param.WithStockAnalysis {
		var stockAnalysesRef []model.StockPositionMonitoringAnalysisRef
		err = utils.ApplyOptions(r.db.WithContext(ctx), opts...).Preload("StockAnalysis").Where("stock_position_monitoring_id IN (?)", stockMonitoringIds).Find(&stockAnalysesRef).Error
		if err != nil {
			return nil, err
		}

		for _, stockAnalysisRef := range stockAnalysesRef {
			stockMonitoringIdx, ok := stockMonitoringMapIdx[stockAnalysisRef.StockPositionMonitoringID]
			if !ok {
				continue
			}

			results[stockMonitoringIdx].StockPositionMonitoringAnalysisRefs = append(results[stockMonitoringIdx].StockPositionMonitoringAnalysisRefs, stockAnalysisRef)
		}

	}
	return results, err
}
