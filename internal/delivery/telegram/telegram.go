package telegram

import (
	"context"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/repository"
	"golang-trading/internal/service"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/httpclient"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"sync"
	"time"

	goValidator "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"gopkg.in/telebot.v3"
)

type TelegramBotHandler struct {
	mu            sync.Mutex
	ctx           context.Context
	cfg           *config.Config
	bot           *telebot.Bot
	log           *logger.Logger
	telegram      *telegram.TelegramRateLimiter
	echo          *echo.Echo
	validator     *goValidator.Validate
	service       *service.Service
	httpClient    httpclient.HTTPClient
	inmemoryCache cache.Cache
	sysParam      repository.SystemParamRepository
}

func NewTelegramBotHandler(
	ctx context.Context,
	cfg *config.Config,
	log *logger.Logger,
	bot *telebot.Bot,
	telegram *telegram.TelegramRateLimiter,
	echo *echo.Echo,
	validator *goValidator.Validate,
	service *service.Service,
	inmemoryCache cache.Cache,
	sysParam repository.SystemParamRepository) *TelegramBotHandler {
	return &TelegramBotHandler{
		ctx:           ctx,
		mu:            sync.Mutex{},
		cfg:           cfg,
		log:           log,
		bot:           bot,
		telegram:      telegram,
		echo:          echo,
		validator:     validator,
		service:       service,
		httpClient:    httpclient.New(log, cfg.Telegram.WebhookURL, cfg.Telegram.TimeoutDuration, ""),
		inmemoryCache: inmemoryCache,
		sysParam:      sysParam,
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

func (t *TelegramBotHandler) RegisterMiddleware() {
	t.bot.Use(t.LoggingMiddleware)
	t.bot.Use(t.RecoverMiddleware())
	t.bot.Use(t.LogErrorMiddleware)
}

func (t *TelegramBotHandler) LoggingMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		now := utils.TimeNowWIB()
		userID := c.Sender().ID
		err := next(c)
		t.log.Debug("Processed message from user",
			logger.StringField("timestamp", now.Format("2006-01-02 15:04:05")),
			logger.IntField("user_id", int(userID)),
			logger.ErrorField(err),
			logger.StringField("duration", time.Since(now).String()),
			logger.StringField("message", c.Message().Text))

		return err
	}
}

func (t *TelegramBotHandler) RecoverMiddleware() telebot.MiddlewareFunc {
	return func(next telebot.HandlerFunc) telebot.HandlerFunc {
		return func(c telebot.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					t.log.Error("Recovered from panic: ", logger.IntField("user_id", int(c.Sender().ID)), logger.ErrorField(fmt.Errorf("%v", r)))
					_ = c.Send("⚠️ Terjadi kesalahan internal. Mohon coba lagi nanti.")
				}
			}()
			return next(c)
		}
	}
}

func (t *TelegramBotHandler) LogErrorMiddleware(next telebot.HandlerFunc) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		err := next(c)
		if err != nil {
			// Log ke Zap
			t.log.ErrorContextWithAlert(t.ctx, "Unhandled Telegram Bot Error",
				logger.StringField("user", c.Sender().Username),
				logger.StringField("text", c.Text()),
				logger.ErrorField(err),
			)
		}
		return err
	}
}
