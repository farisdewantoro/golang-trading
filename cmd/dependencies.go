package cmd

import (
	"context"
	"golang-trading/config"
	"golang-trading/pkg/cache"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/postgres"
	"golang-trading/pkg/telegram"
	"time"

	goValidator "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
)

type AppDependency struct {
	db          *postgres.DB
	cfg         *config.Config
	log         *logger.Logger
	validator   *goValidator.Validate
	echo        *echo.Echo
	cache       cache.Cache
	telegram    *telegram.TelegramRateLimiter
	telegramBot *telebot.Bot
}

func NewAppDependency(ctx context.Context) (*AppDependency, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	log, err := logger.New(cfg.Log.Level, cfg.Log.Encoding)
	if err != nil {
		return nil, err
	}

	db, err := postgres.NewDB(cfg.DB, log)
	if err != nil {
		log.Error("Failed to connect to database", zap.Error(err))
		return nil, err
	}

	pref := telebot.Settings{
		Token:  cfg.Telegram.BotToken,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
		OnError: func(err error, c telebot.Context) {
			log.Error("Telegram bot error", zap.Error(err))
		},
	}
	telebot, err := telebot.NewBot(pref)
	if err != nil {
		log.Error("Failed to create telegram bot", zap.Error(err))
		return nil, err
	}
	e := echo.New()
	return &AppDependency{
		cfg:         cfg,
		log:         log,
		validator:   goValidator.New(),
		db:          db,
		echo:        e,
		cache:       cache.NewCache(cfg.Cache.DefaultExpiration, cfg.Cache.CleanupInterval),
		telegram:    telegram.NewTelegramRateLimiter(&cfg.Telegram, log, telebot),
		telegramBot: telebot,
	}, nil
}

func (d *AppDependency) Close() error {
	d.log.Info("Closing app dependency")
	if d.db != nil {

		return d.db.Close()
	}
	return nil
}
