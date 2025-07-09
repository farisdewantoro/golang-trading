package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StockAnalysisAI struct {
	ID             uint           `gorm:"primarykey"`
	HashIdentifier string         `gorm:"not null"`
	StockCode      string         `gorm:"not null"`
	Exchange       string         `gorm:"not null"`
	MarketPrice    float64        `gorm:"not null;default:0"`
	Prompt         string         `gorm:"not null"`
	Response       datatypes.JSON `gorm:"type:jsonb"`
	Recommendation string         `gorm:"not null"`
	Score          float64        `gorm:"not null"`
	Confidence     float64        `gorm:"not null"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

func (StockAnalysisAI) TableName() string {
	return "stock_analyses_ai"
}
