package model

import (
	"time"

	"gorm.io/datatypes"
)

type Job struct {
	ID          uint                   `gorm:"primaryKey"`
	Name        string                 `gorm:"type:varchar(255);not null"`
	Description string                 `gorm:"type:text"`
	Type        string                 `gorm:"type:varchar(50);not null"`
	Payload     datatypes.JSON         `gorm:"type:jsonb;not null"`
	RetryPolicy datatypes.JSON         `gorm:"type:jsonb"`
	Timeout     int                    `gorm:"default:60"`
	CreatedAt   time.Time              `gorm:"autoCreateTime"`
	UpdatedAt   time.Time              `gorm:"autoUpdateTime"`
	Schedules   []TaskSchedule         `gorm:"foreignKey:JobID"`
	Histories   []TaskExecutionHistory `gorm:"foreignKey:JobID"`
}

func (Job) TableName() string {
	return "jobs"
}

type GetJobParam struct {
	IDs             []uint                        `json:"ids"`
	IsActive        *bool                         `json:"is_active"`
	Limit           *int                          `json:"limit"`
	WithTaskHistory *GetTaskExecutionHistoryParam `json:"with_task_history"`
}
type GetTaskExecutionHistoryParam struct {
	Limit *int `json:"limit"`
}
