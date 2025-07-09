package repository

import (
	"context"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"strings"

	"gorm.io/gorm"
)

type StockAnalysisRepository interface {
	Create(ctx context.Context, stockAnalysis *model.StockAnalysis) error
	CreateBulk(ctx context.Context, stockAnalyses []model.StockAnalysis) error
	GetLatestAnalyses(ctx context.Context, param model.GetLatestAnalysisParam) ([]model.StockAnalysis, error)
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
	var latestHash string

	sub := s.db.Model(&model.StockAnalysis{}).
		Select("hash_identifier").
		Where("stock_code = ? AND exchange = ? AND timestamp >= ?", param.StockCode, param.Exchange, param.TimestampAfter).
		Group("hash_identifier").
		Having("COUNT(DISTINCT timeframe) >= ?", param.ExpectedTFCount).
		Order("MAX(timestamp) DESC").
		Limit(1)

	err := sub.Take(&latestHash).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	var analyses []model.StockAnalysis
	err = s.db.
		Where("stock_code = ? AND exchange = ? AND hash_identifier = ?", param.StockCode, param.Exchange, latestHash).
		Order("timeframe DESC").
		Preload("StockAnalysisAI").
		Find(&analyses).Error
	if err != nil {
		return nil, err
	}

	return analyses, nil
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
