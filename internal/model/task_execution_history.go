package model

import (
	"database/sql"
	"time"
)

type TaskExecutionStatus string

const (
	StatusRunning   TaskExecutionStatus = "running"
	StatusCompleted TaskExecutionStatus = "completed"
	StatusFailed    TaskExecutionStatus = "failed"
	StatusTimeout   TaskExecutionStatus = "timeout"
)

type TaskExecutionHistory struct {
	ID           uint      `gorm:"primaryKey"`
	JobID        uint      `gorm:"not null"`
	ScheduleID   uint      `gorm:"not null"`
	StartedAt    time.Time `gorm:"not null"`
	CompletedAt  sql.NullTime
	Status       TaskExecutionStatus `gorm:"type:varchar(50);not null"`
	ExitCode     sql.NullInt32
	Output       sql.NullString `gorm:"type:text"`
	ErrorMessage sql.NullString `gorm:"type:text"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
}

func (TaskExecutionHistory) TableName() string {
	return "task_execution_history"
}
