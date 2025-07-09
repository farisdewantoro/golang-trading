package model

import (
	"time"

	"gorm.io/datatypes"
)

type StockAnalysis struct {
	ID                uint           `gorm:"primarykey"`
	StockAnalysisAIID *uint          `gorm:"null"`
	HashIdentifier    string         `gorm:"not null"`
	StockCode         string         `gorm:"not null"`
	Exchange          string         `gorm:"not null"`
	Timeframe         string         `gorm:"not null"`
	Timestamp         time.Time      `gorm:"not null"`
	MarketPrice       float64        `gorm:"not null"`
	OHLCV             datatypes.JSON `gorm:"type:jsonb"`
	TechnicalData     datatypes.JSON `gorm:"type:jsonb"`
	Recommendation    string         `gorm:"not null"`
	CreatedAt         time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime" json:"updated_at"`

	StockAnalysisAI *StockAnalysisAI `gorm:"foreignKey:StockAnalysisAIID"`
}

func (StockAnalysis) TableName() string {
	return "stock_analyses"
}

type GetLatestAnalysisParam struct {
	StockCode       string
	Exchange        string
	TimestampAfter  time.Time
	ExpectedTFCount int
}

type UpdateStockAnalysisFilterParam struct {
	StockAnalysesAIID *uint
	HashIdentifier    *string
	StockCode         *string
}

type UpdateStockAnalysisValueParam struct {
	HashIdentifier    *string
	StockAnalysesAIID *uint
}

type UpdateStockAnalysisParam struct {
	Filter UpdateStockAnalysisFilterParam
	Value  UpdateStockAnalysisValueParam
}
