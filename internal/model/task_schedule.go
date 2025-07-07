package model

import (
	"database/sql"
	"time"
)

type TaskSchedule struct {
	ID             uint   `gorm:"primaryKey"`
	JobID          uint   `gorm:"not null"`
	CronExpression string `gorm:"type:varchar(100)"`
	NextExecution  sql.NullTime
	LastExecution  sql.NullTime
	IsActive       bool      `gorm:"default:true"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`

	Job Job `gorm:"foreignKey:JobID;references:ID"`
}

func (TaskSchedule) TableName() string {
	return "task_schedules"
}
