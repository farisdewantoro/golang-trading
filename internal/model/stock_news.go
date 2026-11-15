package model

import "time"

type StockNews struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	StockCode      string     `gorm:"not null" json:"stock_code"`
	Exchange       string     `gorm:"not null" json:"exchange"`
	Title          string     `gorm:"not null" json:"title"`
	Link           string     `gorm:"not null" json:"link"`
	Source         string     `json:"source"`
	PublishedAt    *time.Time `json:"published_at"`
	HashIdentifier string     `gorm:"not null;uniqueIndex" json:"hash_identifier"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (StockNews) TableName() string {
	return "stock_news"
}

type UserNewsWatchlist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	StockCode string    `gorm:"not null" json:"stock_code"`
	Exchange  string    `gorm:"not null" json:"exchange"`
	IsActive  *bool     `gorm:"not null" json:"is_active"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	User      User      `gorm:"foreignKey:UserID;references:ID"`
}

func (UserNewsWatchlist) TableName() string {
	return "user_news_watchlists"
}

type GetUserNewsWatchlistParam struct {
	TelegramID *int64
	StockCode  *string
	Exchange   *string
	IsActive   *bool
}
