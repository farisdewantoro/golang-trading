package model

import "time"

type StockPosition struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	UserID                uint       `gorm:"not null" json:"user_id"`
	StockCode             string     `gorm:"not null" json:"stock_code"`
	Exchange              string     `gorm:"not null" json:"exchange"`
	BuyPrice              float64    `gorm:"not null" json:"buy_price"`
	TakeProfitPrice       float64    `gorm:"not null" json:"take_profit_price"`
	StopLossPrice         float64    `gorm:"not null" json:"stop_loss_price"`
	BuyDate               time.Time  `gorm:"not null" json:"buy_date"`
	MaxHoldingPeriodDays  int        `gorm:"not null" json:"max_holding_period_days"`
	IsActive              *bool      `gorm:"not null" json:"is_active"`
	ExitPrice             *float64   `json:"exit_price"`
	ExitDate              *time.Time `json:"exit_date"`
	PriceAlert            *bool      `json:"price_alert"`
	LastPriceAlertAt      *time.Time `json:"last_price_alert_at"`
	MonitorPosition       *bool      `json:"monitor_position"`
	LastMonitorPositionAt *time.Time `json:"last_monitor_position_at"`
	User                  User       `gorm:"foreignKey:UserID;references:ID"`
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (StockPosition) TableName() string {
	return "stock_positions"
}
