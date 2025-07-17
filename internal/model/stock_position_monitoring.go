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
	StockAnalysis             StockAnalysis `gorm:"foreignKey:StockAnalysisID;references:ID"`
}

func (StockPositionMonitoringAnalysisRef) TableName() string {
	return "stock_position_monitoring_analysis_refs"
}

type PositionAnalysisSummary struct {
	TechnicalAnalysis PositionTechnicalAnalysisSummary `json:"technical_analysis_summary"`
	PositionSignal    string                           `json:"position_signal"`
}

type PositionTechnicalAnalysisSummary struct {
	Signal           string           `json:"signal"`
	Status           string           `json:"status"`
	Score            float64          `json:"score"`
	Insight          []string         `json:"insight"`
	IndicatorSummary IndicatorSummary `json:"indicator_summary"`
}

type IndicatorSummary struct {
	Volume string `json:"volume"`
	RSI    string `json:"rsi"`
	MACD   string `json:"macd"`
	MA     string `json:"ma"`
	Osc    string `json:"osc"`
}

type StockPositionMonitoringQueryParam struct {
	Limit             *int  `json:"limit"`
	StockPositionID   uint  `json:"stock_position_id"`
	WithStockAnalysis *bool `json:"with_stock_analysis"`
}
