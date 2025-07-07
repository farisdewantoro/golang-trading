package telegram

import (
	"context"
	"golang-trading/config"
	"golang-trading/internal/service"
	"golang-trading/pkg/httpclient"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"time"

	goValidator "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"gopkg.in/telebot.v3"
)

type TelegramBotHandler struct {
	ctx        context.Context
	cfg        *config.Config
	bot        *telebot.Bot
	log        *logger.Logger
	telegram   *telegram.TelegramRateLimiter
	echo       *echo.Echo
	validator  *goValidator.Validate
	service    *service.Service
	httpClient httpclient.HTTPClient
}

func NewTelegramBotHandler(
	ctx context.Context,
	cfg *config.Config,
	log *logger.Logger,
	bot *telebot.Bot,
	telegram *telegram.TelegramRateLimiter,
	echo *echo.Echo,
	validator *goValidator.Validate,
	service *service.Service) *TelegramBotHandler {
	return &TelegramBotHandler{
		ctx:        ctx,
		cfg:        cfg,
		log:        log,
		bot:        bot,
		telegram:   telegram,
		echo:       echo,
		validator:  validator,
		service:    service,
		httpClient: httpclient.New(cfg.Telegram.WebhookURL, cfg.Telegram.TimeoutDuration, ""),
	}
}

func (t *TelegramBotHandler) Start() {
	t.log.Info("Starting Telegram bot...")

	if t.cfg.Telegram.WebhookURL == "" {
		t.log.Info("Telegram webhook is disabled")
		return
	}

	t.log.Info("Setting webhook URL", logger.StringField("webhook_url", t.cfg.Telegram.WebhookURL))
	t.bot.SetWebhook(&telebot.Webhook{
		Endpoint: &telebot.WebhookEndpoint{
			PublicURL: t.cfg.Telegram.WebhookURL,
		},
	})

	t.RegisterHandlers()
}

func (t *TelegramBotHandler) Stop() {
	t.log.Info("Stopping Telegram bot...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
	defer cancel()

	// Stop the bot with timeout
	stopDone := make(chan error, 1)
	go func() {
		// Use a separate goroutine to avoid blocking
		t.bot.Stop()
		stopDone <- nil
	}()

	// Wait for bot to stop with timeout
	select {
	case <-stopDone:
		t.log.Info("Telegram bot stopped successfully")
	case <-ctx.Done():
		t.log.Warn("Timeout while stopping bot, forcing shutdown")
	}

	t.log.Info("Telegram bot shutdown completed")
}
