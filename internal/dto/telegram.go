package dto

import (
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"time"

	"gopkg.in/telebot.v3"
)

type RequestSetPositionData struct {
	StockCode    string
	Exchange     string
	BuyPrice     float64
	BuyDate      string
	TakeProfit   float64
	StopLoss     float64
	MaxHolding   int
	AlertPrice   bool
	AlertMonitor bool
	UserTelegram *RequestUserTelegram
	SourceType   string
}

func (r *RequestSetPositionData) ToStockPositionEntity() *model.StockPosition {
	return &model.StockPosition{
		StockCode:       r.StockCode,
		BuyPrice:        r.BuyPrice,
		BuyDate:         utils.MustParseDate(r.BuyDate),
		TakeProfitPrice: r.TakeProfit,
		StopLossPrice:   r.StopLoss,
		PriceAlert:      utils.ToPointer(r.AlertPrice),
		MonitorPosition: utils.ToPointer(r.AlertMonitor),
		Exchange:        r.Exchange,
		SourceType:      r.SourceType,
	}
}

type RequestUserTelegram struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	LanguageCode string    `json:"language_code"`
	IsBot        bool      `json:"is_bot"`
	LastActiveAt time.Time `json:"last_active_at"`
}

func ToRequestUserTelegram(user *telebot.User) *RequestUserTelegram {
	return &RequestUserTelegram{
		ID:           user.ID,
		Username:     user.Username,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		LanguageCode: user.LanguageCode,
		IsBot:        user.IsBot,
		LastActiveAt: utils.TimeNowWIB(),
	}
}

func (r *RequestUserTelegram) ToUserEntity() *model.User {
	return &model.User{
		TelegramID:   r.ID,
		Username:     r.Username,
		FirstName:    r.FirstName,
		LastName:     r.LastName,
		LanguageCode: r.LanguageCode,
		IsBot:        r.IsBot,
		LastActiveAt: r.LastActiveAt,
	}
}

type GetStockPositionsUserTelegramParam struct {
	TelegramID *int64
	IsActive   *bool
}

type RequestExitPositionData struct {
	Symbol          string
	ExitPrice       float64
	ExitDate        time.Time
	StockPositionID uint
}
