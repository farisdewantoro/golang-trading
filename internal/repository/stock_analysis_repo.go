package repository

import (
	"context"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

type StockAnalysisRepository interface {
	Create(ctx context.Context, stockAnalysis *model.StockAnalysis) error
	CreateBulk(ctx context.Context, stockAnalyses []model.StockAnalysis) error
	GetLatestAnalyses(ctx context.Context, param model.GetLatestAnalysisParam) ([]model.StockAnalysis, error)
	DeleteOlderThan(ctx context.Context, date time.Time) (int64, error)
	GetHistoricalAnalyses(ctx context.Context, stockCode, exchange string, startDate, endDate time.Time) ([]model.StockAnalysis, error)
}

type stockAnalysisRepository struct {
	db *gorm.DB
}

func NewStockAnalysisRepository(db *gorm.DB) StockAnalysisRepository {
	return &stockAnalysisRepository{db: db}
}

func (s *stockAnalysisRepository) Create(ctx context.Context, stockAnalysis *model.StockAnalysis) error {
	return s.db.WithContext(ctx).Create(stockAnalysis).Error
}

func (s *stockAnalysisRepository) CreateBulk(ctx context.Context, stockAnalyses []model.StockAnalysis) error {
	return s.db.WithContext(ctx).CreateInBatches(stockAnalyses, 100).Error
}

func (s *stockAnalysisRepository) GetLatestAnalyses(ctx context.Context, param model.GetLatestAnalysisParam) ([]model.StockAnalysis, error) {
	var latestHash []string

	var queryBuilder strings.Builder
	args := []interface{}{}

	queryBuilder.WriteString(`WITH ranked_hashes AS (SELECT hash_identifier, stock_code, ROW_NUMBER() OVER(PARTITION BY stock_code ORDER BY MAX(timestamp) DESC) as rn FROM stock_analyses`)

	whereClauses := []string{}
	if param.StockCode != "" {
		whereClauses = append(whereClauses, "stock_code = ?")
		args = append(args, param.StockCode)
	}
	if param.Exchange != "" {
		whereClauses = append(whereClauses, "exchange = ?")
		args = append(args, param.Exchange)
	}
	if !param.TimestampAfter.IsZero() {
		whereClauses = append(whereClauses, "timestamp >= ?")
		args = append(args, param.TimestampAfter)
	}

	if len(whereClauses) > 0 {
		queryBuilder.WriteString(" WHERE " + strings.Join(whereClauses, " AND "))
	}

	queryBuilder.WriteString(" GROUP BY stock_code, hash_identifier")

	if param.ExpectedTFCount > 0 {
		queryBuilder.WriteString(" HAVING COUNT(DISTINCT timeframe) >= ?")
		args = append(args, param.ExpectedTFCount)
	}

	queryBuilder.WriteString(") SELECT hash_identifier FROM ranked_hashes WHERE rn = 1")

	err := s.db.Debug().Raw(queryBuilder.String(), args...).Scan(&latestHash).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	if len(latestHash) == 0 {
		return nil, nil
	}

	// Fetch the full analysis data for the selected hash_identifiers
	query := s.db.Debug().Where("hash_identifier IN ?", latestHash)

	var analyses []model.StockAnalysis
	err = query.Order("stock_code ASC, timeframe DESC").
		Preload("StockAnalysisAI").
		Find(&analyses).Error

	if err != nil {
		return nil, err
	}

	return analyses, nil
}

func (s *stockAnalysisRepository) GetHistoricalAnalyses(ctx context.Context, stockCode, exchange string, startDate, endDate time.Time) ([]model.StockAnalysis, error) {
	var analyses []model.StockAnalysis

	query := s.db.WithContext(ctx).Debug().
		Where("stock_code = ?", stockCode).
		Where("exchange = ?", exchange).
		Where("timestamp >= ?", startDate).
		Where("timestamp <= ?", endDate).
		Order("timestamp ASC, created_at ASC")

	err := query.Find(&analyses).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not an error, just no data
		}
		return nil, err
	}

	return analyses, nil
}

func (s *stockAnalysisRepository) DeleteOlderThan(ctx context.Context, date time.Time) (int64, error) {
	db := s.db.WithContext(ctx).Where("created_at < ?", date).Delete(&model.StockAnalysis{})
	if db.Error != nil {
		return 0, db.Error
	}
	return db.RowsAffected, nil
}

func (s *stockAnalysisRepository) Update(ctx context.Context, param model.UpdateStockAnalysisParam, opts ...utils.DBOption) error {
	db := utils.ApplyOptions(s.db.WithContext(ctx), opts...)

	qFilter := []string{}
	qFilterValues := []interface{}{}

	sValue := []string{}

	if param.Filter.StockAnalysesAIID != nil {
		qFilter = append(qFilter, "stock_analyses_ai_id = ?")
		qFilterValues = append(qFilterValues, *param.Filter.StockAnalysesAIID)
	}

	if param.Filter.HashIdentifier != nil {
		qFilter = append(qFilter, "hash_identifier = ?")
		qFilterValues = append(qFilterValues, *param.Filter.HashIdentifier)
	}

	if param.Value.StockAnalysesAIID != nil {
		sValue = append(sValue, "stock_analyses_ai_id = ?")
	}

	if param.Value.HashIdentifier != nil {
		sValue = append(sValue, "hash_identifier = ?")
	}

	return db.Model(&model.StockAnalysis{}).Where(strings.Join(qFilter, " AND "), qFilterValues...).Updates(strings.Join(sValue, ", ")).Error
}
