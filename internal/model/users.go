package model

import "time"

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
