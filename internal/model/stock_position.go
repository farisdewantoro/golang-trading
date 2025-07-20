package model

import "time"

const (
	StockPositionSourceTypeManual    = "MANUAL"
	StockPositionSourceTypeAI        = "AI"
	StockPositionSourceTypeTechnical = "TECHNICAL"
)

type StockPosition struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	UserID                uint       `gorm:"not null" json:"user_id"`
	StockCode             string     `gorm:"not null" json:"stock_code"`
	Exchange              string     `gorm:"not null" json:"exchange"`
	BuyPrice              float64    `gorm:"not null" json:"buy_price"`
	TakeProfitPrice       float64    `gorm:"not null" json:"take_profit_price"`
	StopLossPrice         float64    `gorm:"not null" json:"stop_loss_price"`
	HighestPriceSinceTTP  float64    `gorm:"default:0" json:"highest_price_since_ttp"`
	TrailingProfitPrice   float64    `gorm:"not null" json:"trailing_profit_price"`
	TrailingStopPrice     float64    `gorm:"not null" json:"trailing_stop_price"`
	BuyDate               time.Time  `gorm:"not null" json:"buy_date"`
	IsActive              *bool      `gorm:"not null" json:"is_active"`
	ExitPrice             *float64   `json:"exit_price"`
	ExitDate              *time.Time `json:"exit_date"`
	PriceAlert            *bool      `json:"price_alert"`
	LastPriceAlertAt      *time.Time `json:"last_price_alert_at"`
	MonitorPosition       *bool      `json:"monitor_position"`
	SourceType            string     `json:"source_type" gorm:"default:'MANUAL'"`
	LastMonitorPositionAt *time.Time `json:"last_monitor_position_at"`
	User                  User       `gorm:"foreignKey:UserID;references:ID"`
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	InitialScore          float64    `json:"initial_score"`
	FinalScore            float64    `json:"final_score"`

	StockPositionMonitorings []StockPositionMonitoring
}

func (StockPosition) TableName() string {
	return "stock_positions"
}
