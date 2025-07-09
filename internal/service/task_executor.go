package service

import (
	"context"
	"database/sql"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/internal/strategy"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
)

type TaskExecutor interface {
	Execute(ctx context.Context, taskHistory *model.TaskExecutionHistory) error
}

type taskExecutor struct {
	cfg                *config.Config
	log                *logger.Logger
	jobRepo            repository.JobRepository
	executorStrategies map[strategy.JobType]strategy.JobExecutionStrategy
}

func NewTaskExecutor(cfg *config.Config, log *logger.Logger, jobRepo repository.JobRepository, executorStrategies map[strategy.JobType]strategy.JobExecutionStrategy) TaskExecutor {
	return &taskExecutor{
		jobRepo:            jobRepo,
		cfg:                cfg,
		log:                log,
		executorStrategies: executorStrategies,
	}
}

func (t *taskExecutor) Execute(ctx context.Context, taskHistory *model.TaskExecutionHistory) error {
	t.log.InfoContext(ctx, "Processing job", logger.IntField("job_id", int(taskHistory.JobID)), logger.IntField("history_id", int(taskHistory.ID)))

	job, err := t.jobRepo.FindByID(ctx, taskHistory.JobID)
	if err != nil {
		t.log.ErrorContext(ctx, "Failed to find job", logger.ErrorField(err), logger.IntField("job_id", int(taskHistory.JobID)))
		return fmt.Errorf("failed to find job: %w", err)
	}

	strategy := t.executorStrategies[strategy.JobType(job.Type)]
	if strategy == nil {
		t.log.ErrorContext(ctx, "Job type not found", logger.IntField("job_id", int(taskHistory.JobID)))
		taskHistory.Status = model.StatusFailed
		taskHistory.ErrorMessage = sql.NullString{String: "job type not found", Valid: true}
	} else {
		result, err := strategy.Execute(ctx, job)
		if err != nil {
			t.log.ErrorContext(ctx, "Failed to execute job", logger.ErrorField(err), logger.IntField("job_id", int(taskHistory.JobID)))
			taskHistory.Status = model.StatusFailed
			taskHistory.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
		} else {
			taskHistory.Status = model.StatusCompleted
		}
		taskHistory.ExitCode = sql.NullInt32{Int32: result.ExitCode, Valid: true}
		taskHistory.Output = sql.NullString{String: result.Output, Valid: true}
	}

	taskHistory.CompletedAt = sql.NullTime{Time: utils.TimeNowWIB(), Valid: true}
	if err := t.jobRepo.UpdateTaskExecutionHistory(ctx, taskHistory); err != nil {
		t.log.ErrorContext(ctx, "Failed to update task execution history", logger.ErrorField(err), logger.IntField("job_id", int(taskHistory.JobID)))
		return fmt.Errorf("failed to update task execution history: %w", err)
	}

	return nil
}
