package repository

import (
	"context"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"time"

	"gorm.io/gorm"
)

type JobRepository interface {
	FindJobsToSchedule(ctx context.Context, opts ...utils.DBOption) ([]model.TaskSchedule, error)
	CreateTaskExecutionHistory(ctx context.Context, history *model.TaskExecutionHistory, opts ...utils.DBOption) error
	UpdateTaskSchedule(ctx context.Context, schedule *model.TaskSchedule, opts ...utils.DBOption) error
	FindByID(ctx context.Context, id uint) (*model.Job, error)
	UpdateTaskExecutionHistory(ctx context.Context, history *model.TaskExecutionHistory, opts ...utils.DBOption) error
	Get(ctx context.Context, param *model.GetJobParam, opts ...utils.DBOption) ([]model.Job, error)
	DeleteTaskHistoryOlderThan(ctx context.Context, date time.Time, opts ...utils.DBOption) (int64, error)
}

type jobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) JobRepository {
	return &jobRepository{db: db}
}

// FindJobsToSchedule finds all active jobs with schedules that need to be run.
func (r *jobRepository) FindJobsToSchedule(ctx context.Context, opts ...utils.DBOption) ([]model.TaskSchedule, error) {
	var schedules []model.TaskSchedule
	// Find jobs with active schedules that are due
	err := utils.ApplyOptions(r.db.WithContext(ctx), opts...).
		Where("is_active = ? AND (next_execution IS NULL OR next_execution <= ?)", true, utils.TimeNowWIB()).
		Find(&schedules).Error
	if err != nil {
		return nil, err
	}
	return schedules, nil
}

func (r *jobRepository) CreateTaskExecutionHistory(ctx context.Context, history *model.TaskExecutionHistory, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Create(history).Error
}

func (r *jobRepository) UpdateTaskSchedule(ctx context.Context, schedule *model.TaskSchedule, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Updates(schedule).Error
}

func (r *jobRepository) FindByID(ctx context.Context, id uint) (*model.Job, error) {
	var job model.Job
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *jobRepository) UpdateTaskExecutionHistory(ctx context.Context, history *model.TaskExecutionHistory, opts ...utils.DBOption) error {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Updates(history).Error
}

func (r *jobRepository) Get(ctx context.Context, param *model.GetJobParam, opts ...utils.DBOption) ([]model.Job, error) {
	var jobs []model.Job
	db := utils.ApplyOptions(r.db.WithContext(ctx), opts...)
	db = db.Model(&model.Job{}).Joins("LEFT JOIN task_schedules ON task_schedules.job_id = jobs.id")
	if param.IsActive != nil {
		db = db.Where("task_schedules.is_active = ?", *param.IsActive)
	}
	if len(param.IDs) > 0 {
		db = db.Where("jobs.id IN ?", param.IDs)
	}
	if param.Limit != nil {
		db = db.Limit(*param.Limit)
	}
	if param.WithTaskHistory != nil {
		db = db.Preload("Histories", func(db *gorm.DB) *gorm.DB {
			db = db.Order("created_at DESC")
			if param.WithTaskHistory.Limit != nil {
				db = db.Limit(*param.WithTaskHistory.Limit)
			}
			return db
		})
	}
	result := db.Preload("Schedules.Job").Find(&jobs)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return jobs, nil
}

func (r *jobRepository) DeleteTaskHistoryOlderThan(ctx context.Context, date time.Time, opts ...utils.DBOption) (int64, error) {
	return utils.ApplyOptions(r.db.WithContext(ctx), opts...).Where("created_at < ?", date).Delete(&model.TaskExecutionHistory{}).RowsAffected, nil
}
