package repository

import (
	"context"
	"encoding/json"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"

	"gorm.io/gorm"
)

type SystemParamRepository interface {
	Get(ctx context.Context, name string, destValue interface{}) error
	GetDefaultAnalysisTimeframes(ctx context.Context) ([]dto.DataTimeframe, error)
}

type systemParamRepository struct {
	db *gorm.DB
}

func NewSystemParamRepository(db *gorm.DB) SystemParamRepository {
	return &systemParamRepository{db: db}
}

func (s *systemParamRepository) Get(ctx context.Context, name string, destValue interface{}) error {
	var param model.SystemParameter
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&param).Error; err != nil {
		return err
	}
	return json.Unmarshal(param.Value, destValue)
}

func (s *systemParamRepository) GetDefaultAnalysisTimeframes(ctx context.Context) ([]dto.DataTimeframe, error) {
	var destValue []dto.DataTimeframe
	if err := s.Get(ctx, model.SysParamDefaultAnalysisTimeframes, &destValue); err != nil {
		return nil, err
	}
	return destValue, nil
}
