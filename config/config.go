package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Log         Logger
	DB          Database
	API         API
	Scheduler   Scheduler
	TradingView TradingView
	Cache       Cache
	Telegram    TelegramConfig
}

type Logger struct {
	Level    string
	Encoding string
}

type Database struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	SSLMode         string
	TimeZone        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime string
	LogLevel        string
}

type Scheduler struct {
	MaxConcurrency  int
	TimeoutDuration time.Duration
}

type API struct {
	Port int
}

type TradingView struct {
	BaseURLScanner   string
	BaseTimeout      time.Duration
	MaxRequestPerMin int
}

type Cache struct {
	DefaultExpiration time.Duration
	CleanupInterval   time.Duration
}

type TelegramConfig struct {
	BotToken                  string
	ChatID                    string
	WebhookURL                string
	TimeoutDuration           time.Duration
	TimeoutBuyListDuration    time.Duration
	MaxGlobalRequestPerSecond int
	MaxUserRequestPerSecond   int
	MaxEditMessagePerSecond   int
	RatelimitExpireDuration   time.Duration
	RateLimitCleanupDuration  time.Duration
	FeatureNewsMaxAgeInDays   int
	FeatureNewsLimitStockNews int
	MaxShowHistoryAnalysis    int
}

func Load() (*Config, error) {
	// Load .env file. It's okay if it doesn't exist.
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Failed to load .env file %v\n", err)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	var cfg Config

	cfg = Config{
		Log: Logger{
			Level:    viper.GetString("LOGGER_LEVEL"),
			Encoding: viper.GetString("LOGGER_ENCODING"),
		},
		DB: Database{
			Host:            viper.GetString("DATABASE_HOST"),
			Port:            viper.GetInt("DATABASE_PORT"),
			User:            viper.GetString("DATABASE_USER"),
			Password:        viper.GetString("DATABASE_PASSWORD"),
			DBName:          viper.GetString("DATABASE_NAME"),
			SSLMode:         viper.GetString("DATABASE_SSL_MODE"),
			TimeZone:        viper.GetString("DATABASE_TIMEZONE"),
			MaxIdleConns:    viper.GetInt("DATABASE_MAX_IDLE_CONNS"),
			MaxOpenConns:    viper.GetInt("DATABASE_MAX_OPEN_CONNS"),
			ConnMaxLifetime: viper.GetString("DATABASE_CONN_MAX_LIFETIME"),
			LogLevel:        viper.GetString("DATABASE_LOG_LEVEL"),
		},
		API: API{
			Port: viper.GetInt("API_PORT"),
		},
		Scheduler: Scheduler{
			MaxConcurrency:  viper.GetInt("SCHEDULER_MAX_CONCURRENCY"),
			TimeoutDuration: viper.GetDuration("SCHEDULER_TIMEOUT_DURATION"),
		},
		TradingView: TradingView{
			BaseURLScanner:   viper.GetString("TRADINGVIEW_BASE_URL_SCANNER"),
			BaseTimeout:      viper.GetDuration("TRADINGVIEW_BASE_TIMEOUT"),
			MaxRequestPerMin: viper.GetInt("TRADINGVIEW_MAX_REQUEST_PER_MIN"),
		},
		Cache: Cache{
			DefaultExpiration: viper.GetDuration("CACHE_DEFAULT_EXPIRATION"),
			CleanupInterval:   viper.GetDuration("CACHE_CLEANUP_INTERVAL"),
		},
		Telegram: TelegramConfig{
			BotToken:                  viper.GetString("TELEGRAM_BOT_TOKEN"),
			ChatID:                    viper.GetString("TELEGRAM_CHAT_ID"),
			WebhookURL:                viper.GetString("TELEGRAM_WEBHOOK_URL"),
			TimeoutDuration:           viper.GetDuration("TELEGRAM_TIMEOUT_DURATION"),
			TimeoutBuyListDuration:    viper.GetDuration("TELEGRAM_TIMEOUT_BUY_LIST_DURATION"),
			MaxGlobalRequestPerSecond: viper.GetInt("TELEGRAM_MAX_GLOBAL_REQUEST_PER_SECOND"),
			MaxUserRequestPerSecond:   viper.GetInt("TELEGRAM_MAX_USER_REQUEST_PER_SECOND"),
			MaxEditMessagePerSecond:   viper.GetInt("TELEGRAM_MAX_EDIT_MESSAGE_PER_SECOND"),
			RatelimitExpireDuration:   viper.GetDuration("TELEGRAM_RATELIMIT_EXPIRE_DURATION"),
			RateLimitCleanupDuration:  viper.GetDuration("TELEGRAM_RATELIMIT_CLEANUP_DURATION"),
			FeatureNewsMaxAgeInDays:   viper.GetInt("TELEGRAM_FEATURE_NEWS_MAX_AGE_IN_DAYS"),
			FeatureNewsLimitStockNews: viper.GetInt("TELEGRAM_FEATURE_NEWS_LIMIT_STOCK_NEWS"),
			MaxShowHistoryAnalysis:    viper.GetInt("TELEGRAM_MAX_SHOW_HISTORY_ANALYSIS"),
		},
	}

	return &cfg, nil
}
