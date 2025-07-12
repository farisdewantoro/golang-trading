package service

import (
	"context"
	"database/sql"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"time"

	"github.com/robfig/cron/v3"
)

type SchedulerService interface {
	Execute(ctx context.Context) error
}

type schedulerService struct {
	cfg          *config.Config
	log          *logger.Logger
	cronParser   cron.Parser
	jobRepo      repository.JobRepository
	taskExecutor TaskExecutor
}

func NewSchedulerService(
	cfg *config.Config,
	log *logger.Logger,
	jobRepo repository.JobRepository,
	taskExecutor TaskExecutor,
) *schedulerService {
	return &schedulerService{
		cfg:          cfg,
		log:          log,
		jobRepo:      jobRepo,
		cronParser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
		taskExecutor: taskExecutor,
	}
}

func (s *schedulerService) Execute(ctx context.Context) error {
	jobs, err := s.jobRepo.FindJobsToSchedule(ctx, utils.WithPreload("Job"))
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to find jobs to schedule", logger.ErrorField(err))
		return fmt.Errorf("failed to find jobs to schedule: %w", err)
	}

	semaphore := make(chan struct{}, s.cfg.Scheduler.MaxConcurrency)

	if len(jobs) == 0 {
		s.log.InfoContext(ctx, "No jobs to schedule")
		return nil
	}
	s.log.InfoContext(ctx, "Start running jobs",
		logger.IntField("job_count", len(jobs)),
		logger.IntField("max_concurrency", s.cfg.Scheduler.MaxConcurrency),
	)

	for _, job := range jobs {
		if ctx.Err() != nil {
			s.log.WarnContext(ctx, "Job execution cancelled", logger.ErrorField(ctx.Err()))
			return nil
		}

		err := s.executeJob(ctx, job, semaphore)
		if err != nil {
			s.log.ErrorContext(ctx, "Failed to execute job",
				logger.ErrorField(err),
				logger.IntField("job_id", int(job.JobID)),
				logger.IntField("schedule_id", int(job.ID)),
			)
		}

		s.log.InfoContext(ctx, "Job execution completed",
			logger.IntField("job_id", int(job.JobID)),
			logger.IntField("schedule_id", int(job.ID)),
			logger.StringField("job_name", job.Job.Name),
		)
	}

	return nil
}

func (s *schedulerService) executeJob(ctx context.Context, task model.TaskSchedule, semaphore chan struct{}) error {
	now := utils.TimeNowWIB()
	history := &model.TaskExecutionHistory{
		JobID:      task.JobID,
		ScheduleID: task.ID,
		Status:     model.StatusRunning,
		StartedAt:  now,
	}

	if err := s.jobRepo.CreateTaskExecutionHistory(ctx, history); err != nil {
		s.log.ErrorContext(ctx, "Failed to create task history", logger.ErrorField(err), logger.IntField("schedule_id", int(task.ID)))
		return fmt.Errorf("failed to create task history: %w", err)
	}
	semaphore <- struct{}{}

	utils.GoSafe(func() {
		defer func() {
			<-semaphore
		}()

		newCtx, cancel := context.WithTimeout(context.Background(), time.Duration(task.Job.Timeout)*time.Second)
		defer cancel()
		if err := s.taskExecutor.Execute(newCtx, history); err != nil {
			s.log.ErrorContext(newCtx, "Failed to execute task", logger.ErrorField(err), logger.IntField("schedule_id", int(task.ID)))
		}
	}).Run()

	// Update schedule for next run
	cronSchedule, err := s.cronParser.Parse(task.CronExpression)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to parse cron expression", logger.ErrorField(err), logger.IntField("schedule_id", int(task.ID)))
		return fmt.Errorf("failed to parse cron expression: %w", err)
	}

	task.LastExecution = sql.NullTime{Time: now, Valid: true}
	task.NextExecution = sql.NullTime{Time: cronSchedule.Next(now), Valid: true}

	if err := s.jobRepo.UpdateTaskSchedule(ctx, &task); err != nil {
		s.log.ErrorContext(ctx, "Failed to update task schedule", logger.ErrorField(err), logger.IntField("schedule_id", int(task.ID)))
		return fmt.Errorf("failed to update task schedule: %w", err)
	}
	return nil
}
