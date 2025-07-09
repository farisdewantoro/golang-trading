package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	SysParamDefaultAnalysisTimeframes = "DEFAULT_ANALYSIS_TIMEFRAMES"
)

type SystemParameter struct {
	Name        string         `gorm:"column:name;type:varchar(100);primaryKey" json:"name"`
	Value       datatypes.JSON `gorm:"column:value;type:jsonb" json:"value"`
	Description string         `gorm:"column:description;type:text" json:"description"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at"`
	CreatedAt   time.Time      `gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime"`
}

func (SystemParameter) TableName() string {
	return "system_parameters"
}
