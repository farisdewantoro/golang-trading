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
	RetentionDays int `json:"retention_days"`
}

type DataCleanUpResult struct {
	Table string `json:"table"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

type DataCleanUpStrategy struct {
	cfg               *config.Config
	log               *logger.Logger
	StockAnalysisRepo repository.StockAnalysisRepository
	JobRepo           repository.JobRepository
}

func NewDataCleanUpStrategy(cfg *config.Config, log *logger.Logger, stockAnalysisRepo repository.StockAnalysisRepository, jobRepo repository.JobRepository) DataCleaner {
	return &DataCleanUpStrategy{
		cfg:               cfg,
		log:               log,
		StockAnalysisRepo: stockAnalysisRepo,
		JobRepo:           jobRepo,
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

	date := utils.TimeNowWIB().AddDate(0, 0, -payload.RetentionDays)
	totalDeleted, err := s.StockAnalysisRepo.DeleteOlderThan(ctx, date)
	outputMsg := []DataCleanUpResult{}
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to delete older than", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		outputMsg = append(outputMsg, DataCleanUpResult{
			Table: "stock_analysis",
			Total: totalDeleted,
			Error: fmt.Sprintf("failed to delete data stock analysis older than %v: %v", date, err),
		})
	} else {
		outputMsg = append(outputMsg, DataCleanUpResult{
			Table: "stock_analysis",
			Total: totalDeleted,
		})
	}

	totalDeletedTask, err := s.JobRepo.DeleteTaskHistoryOlderThan(ctx, date)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to update job history", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		outputMsg = append(outputMsg, DataCleanUpResult{
			Table: "job_history",
			Total: totalDeletedTask,
			Error: fmt.Sprintf("failed to delete data job history older than %v: %v", date, err),
		})
	} else {
		outputMsg = append(outputMsg, DataCleanUpResult{
			Table: "job_history",
			Total: totalDeletedTask,
		})
	}

	res, err := json.Marshal(outputMsg)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to marshal output message", logger.ErrorField(err), logger.IntField("job_id", int(job.ID)))
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to marshal output message: %v", err)}, fmt.Errorf("failed to marshal output message: %w", err)
	}
	return JobResult{ExitCode: JOB_EXIT_CODE_SUCCESS, Output: string(res)}, nil
}

func (s *DataCleanUpStrategy) GetType() JobType {
	return JobTypeDataCleanUp
}
