package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type StockPositionMonitoring struct {
	ID                uint           `gorm:"primaryKey"`
	StockPositionID   uint           `gorm:"not null"`
	StockAnalysisAIID *uint          `gorm:"null"`
	EvaluationSummary datatypes.JSON `gorm:"type:jsonb"`
	MarketPrice       float64        `gorm:"not null"`
	Timestamp         time.Time      `gorm:"not null"`
	HashIdentifier    string         `gorm:"not null"`
	CreatedAt         time.Time      `gorm:"autoCreateTime"`
	UpdatedAt         time.Time      `gorm:"autoUpdateTime"`
	DeletedAt         gorm.DeletedAt

	StockPositionMonitoringAnalysisRefs []StockPositionMonitoringAnalysisRef `gorm:"foreignKey:StockPositionMonitoringID;references:ID"`
	StockPosition                       StockPosition                        `gorm:"foreignKey:StockPositionID"`
	StockAnalysisAI                     *StockAnalysisAI                     `gorm:"foreignKey:StockAnalysisAIID;references:ID"`
}

func (StockPositionMonitoring) TableName() string {
	return "stock_position_monitorings"
}

type StockPositionMonitoringAnalysisRef struct {
	ID                        uint      `gorm:"primaryKey"`
	StockPositionMonitoringID uint      `gorm:"not null"`
	StockAnalysisID           uint      `gorm:"not null"`
	CreatedAt                 time.Time `gorm:"autoCreateTime"`
	UpdatedAt                 time.Time `gorm:"autoUpdateTime"`
	DeletedAt                 gorm.DeletedAt
}

func (StockPositionMonitoringAnalysisRef) TableName() string {
	return "stock_position_monitoring_analysis_refs"
}

type EvaluationSummaryData struct {
	TechnicalRecommendation string `json:"technical_recommendation"`
	TechnicalScore          int    `json:"technical_score"`
}

type StockPositionMonitoringQueryParam struct {
	Limit           *int `json:"limit"`
	StockPositionID uint `json:"stock_position_id"`
}
