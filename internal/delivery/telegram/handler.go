package telegram

import (
	"context"
	"golang-trading/internal/dto"
	"golang-trading/pkg/logger"
	"net/http"

	"github.com/labstack/echo/v4"
	"gopkg.in/telebot.v3"
)

func (t *TelegramBotHandler) WithContext(handler func(ctx context.Context, c telebot.Context) error) func(c telebot.Context) error {
	return func(c telebot.Context) error {
		ctx, cancel := context.WithTimeout(t.ctx, t.cfg.Telegram.TimeoutDuration)
		defer cancel()

		return handler(ctx, c)
	}
}

func (t *TelegramBotHandler) RegisterHandlers() {
	t.echo.POST("/api/v1/telegram/webhook", func(c echo.Context) error {
		var update telebot.Update
		if err := c.Bind(&update); err != nil {
			t.log.ErrorContext(t.ctx, "Cannot bind JSON", logger.ErrorField(err))
			badRequest := dto.NewBadRequestResponse(err.Error())
			return c.JSON(http.StatusBadRequest, badRequest)
		}
		t.bot.ProcessUpdate(update)
		return c.JSON(http.StatusOK, dto.NewBaseResponse(http.StatusOK, "ok", nil))
	})

	t.bot.Handle("/start", t.WithContext(t.handleStart))
	t.bot.Handle("/help", t.WithContext(t.handleHelp))
	t.bot.Handle("/analyze", t.WithContext(t.handleStartAnalyze))

	t.bot.Handle(telebot.OnText, t.WithContext(t.handleConversation))

	t.bot.Handle(&btnAskAIAnalyzer, t.WithContext(t.handleAskAIAnalyzer))

}
