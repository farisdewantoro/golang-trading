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
WITH base AS (
  SELECT
    *,
    DATE_TRUNC('day', "timestamp") as day,
    evaluation_summary->'technical_analysis_summary'->>'score' AS score,
    evaluation_summary->'technical_analysis_summary'->>'technical_signal' AS technical_signal,
    evaluation_summary->'technical_analysis_summary'->>'status' AS status,
    evaluation_summary->'position_signal' AS position_signal,
    LAG(evaluation_summary->'technical_analysis_summary'->>'score') OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_score,
    LAG(evaluation_summary->'technical_analysis_summary'->>'technical_signal') OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_technical_signal,
    LAG(evaluation_summary->'technical_analysis_summary'->>'status') OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_status,
    LAG(evaluation_summary->'position_signal') OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_position_signal,
    LAG(market_price) OVER (PARTITION BY stock_position_id ORDER BY "timestamp" DESC) AS prev_price
  FROM stock_position_monitorings
  WHERE stock_position_id = ? AND deleted_at IS NULL
),
ranked_per_day AS (
  SELECT *,
         ROW_NUMBER() OVER (PARTITION BY day ORDER BY "timestamp" DESC) AS rn,
         CASE
           WHEN prev_score IS NULL THEN true
           WHEN score IS DISTINCT FROM prev_score THEN true
           WHEN technical_signal IS DISTINCT FROM prev_technical_signal THEN true
           WHEN status IS DISTINCT FROM prev_status THEN true
           WHEN position_signal IS DISTINCT FROM prev_position_signal THEN true
           WHEN market_price IS DISTINCT FROM prev_price THEN true
           ELSE false
         END AS is_changed
  FROM base
)
SELECT *
FROM ranked_per_day
WHERE rn = 1
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
