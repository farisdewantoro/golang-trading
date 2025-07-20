package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TelegramID   int64     `gorm:"not null" json:"telegram_id"`
	Username     string    `gorm:"not null" json:"username"`
	FirstName    string    `gorm:"not null" json:"first_name"`
	LastName     string    `json:"last_name"`
	LanguageCode string    `json:"language_code"`
	IsBot        bool      `gorm:"not null" json:"is_bot"`
	LastActiveAt time.Time `gorm:"not null" json:"last_active_at"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type UserSignalAlert struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null" json:"user_id"`
	Exchange  string         `gorm:"not null" json:"exchange"`
	IsActive  *bool          `gorm:"not null" json:"is_active"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at"`
	User      User           `gorm:"foreignKey:UserID;references:ID"`
}

func (UserSignalAlert) TableName() string {
	return "user_signal_alerts"
}

type GetUserSignalAlertParam struct {
	TelegramID *int64  `json:"telegram_id"`
	IsActive   *bool   `json:"is_active"`
	Exchange   *string `json:"exchange"`
}
