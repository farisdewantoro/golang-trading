package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
)

type DataCleaner interface {
	JobExecutionStrategy
}

type DataCleanUpPayload struct {
	Days int `json:"days"`
}

type DataCleanUpStrategy struct {
	cfg               *config.Config
	log               *logger.Logger
	StockAnalysisRepo repository.StockAnalysisRepository
}

func NewDataCleanUpStrategy(cfg *config.Config, log *logger.Logger, stockAnalysisRepo repository.StockAnalysisRepository) DataCleaner {
	return &DataCleanUpStrategy{
		cfg:               cfg,
		log:               log,
		StockAnalysisRepo: stockAnalysisRepo,
	}
}

func (s *DataCleanUpStrategy) Execute(ctx context.Context, job *model.Job) (JobResult, error) {
	s.log.InfoContext(ctx, "Starting data clean up")

	var (
		payload DataCleanUpPayload
	)
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.log.ErrorContext(ctx, "Failed to unmarshal job payload", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to unmarshal job payload: %v", err)}, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	date := utils.TimeNowWIB().AddDate(0, 0, -payload.Days)
	totalDeleted, err := s.StockAnalysisRepo.DeleteOlderThan(ctx, date)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to delete older than", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to delete older than: %v", err)}, fmt.Errorf("failed to delete older than: %w", err)
	}
	return JobResult{ExitCode: JOB_EXIT_CODE_SUCCESS, Output: fmt.Sprintf("deleted older than: %v, total deleted: %v", date, totalDeleted)}, nil
}

func (s *DataCleanUpStrategy) GetType() JobType {
	return JobTypeDataCleanUp
}
