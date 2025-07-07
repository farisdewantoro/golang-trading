package middleware

import (
	"context"
	"time"

	"gopkg.in/telebot.v3"
)

func WithContext(rootCtx context.Context, handler func(ctx context.Context, c telebot.Context) error) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		ctx, cancel := context.WithTimeout(rootCtx, 5*time.Minute)
		defer cancel()

		return handler(ctx, c)
	}
}
