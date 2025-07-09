package repository

import (
	"context"
	"encoding/json"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/pkg/cache"

	"gorm.io/gorm"
)

type SystemParamRepository interface {
	Get(ctx context.Context, name string, destValue interface{}) error
	GetDefaultAnalysisTimeframes(ctx context.Context) ([]dto.DataTimeframe, error)
}

type systemParamRepository struct {
	cfg           *config.Config
	inmemoryCache cache.Cache
	db            *gorm.DB
}

func NewSystemParamRepository(cfg *config.Config, inmemoryCache cache.Cache, db *gorm.DB) SystemParamRepository {
	return &systemParamRepository{cfg: cfg, inmemoryCache: inmemoryCache, db: db}
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
	if val, found := cache.GetFromCache[[]dto.DataTimeframe](model.SysParamDefaultAnalysisTimeframes); found {
		return val, nil
	}
	if err := s.Get(ctx, model.SysParamDefaultAnalysisTimeframes, &destValue); err != nil {
		return nil, err
	}
	s.inmemoryCache.Set(model.SysParamDefaultAnalysisTimeframes, destValue, s.cfg.Cache.SysParamExpDuration)
	return destValue, nil
}
