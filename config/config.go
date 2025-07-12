package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Log           Logger
	DB            Database
	API           API
	Scheduler     Scheduler
	TradingView   TradingView
	Cache         Cache
	Telegram      TelegramConfig
	YahooFinance  YahooFinance
	Gemini        Gemini
	Trading       Trading
	StockAnalyzer StockAnalyzer
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
	BaseURLScanner         string
	BaseTimeout            time.Duration
	MaxRequestPerMin       int
	BuyListMaxStockAnalyze int
}

type Cache struct {
	DefaultExpiration        time.Duration
	CleanupInterval          time.Duration
	SysParamExpDuration      time.Duration
	TelegramStateExpDuration time.Duration
}

type TelegramConfig struct {
	BotToken                  string
	ChatID                    string
	WebhookURL                string
	TimeoutDuration           time.Duration
	TimeoutAsyncDuration      time.Duration
	MaxGlobalRequestPerSecond int
	MaxUserRequestPerSecond   int
	MaxEditMessagePerSecond   int
	RatelimitExpireDuration   time.Duration
	RateLimitCleanupDuration  time.Duration

	FeatureStockAnalyze TelegramFeatureStockAnalyze
}

type YahooFinance struct {
	BaseURL             string
	Timeout             time.Duration
	MaxRequestPerMinute int
}

type TelegramFeatureStockAnalyze struct {
	AfterTimestampDuration time.Duration
	ExpectedTFCount        int
}

type Gemini struct {
	APIKey              string
	BaseModel           string
	MaxRequestPerMinute int
	MaxTokenPerMinute   int
	BaseURL             string
	Timeout             time.Duration
}

type Trading struct {
	RiskRewardRatio float64
	MaxBuyList      int
}

type StockAnalyzer struct {
	MaxConcurrency int
}

func Load() (*Config, error) {
	// Load .env file. It's okay if it doesn't exist.
	cfgName := "env-config"
	err := godotenv.Load(cfgName)
	if err != nil {
		dir, errWd := os.Getwd()
		if errWd != nil {
			fmt.Println(fmt.Sprintf("Failed to get current directory %v", errWd))
		}
		fmt.Printf("Failed to load %v file %v -> current directory: %v\n", cfgName, err, dir)
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
			BaseURLScanner:         viper.GetString("TRADINGVIEW_BASE_URL_SCANNER"),
			BaseTimeout:            viper.GetDuration("TRADINGVIEW_BASE_TIMEOUT"),
			MaxRequestPerMin:       viper.GetInt("TRADINGVIEW_MAX_REQUEST_PER_MIN"),
			BuyListMaxStockAnalyze: viper.GetInt("TRADINGVIEW_BUY_LIST_MAX_STOCK_ANALYZE"),
		},
		Cache: Cache{
			DefaultExpiration:        viper.GetDuration("CACHE_DEFAULT_EXPIRATION"),
			CleanupInterval:          viper.GetDuration("CACHE_CLEANUP_INTERVAL"),
			SysParamExpDuration:      viper.GetDuration("CACHE_SYS_PARAM_EXPIRATION_DURATION"),
			TelegramStateExpDuration: viper.GetDuration("CACHE_TELEGRAM_STATE_EXPIRATION_DURATION"),
		},
		Telegram: TelegramConfig{
			BotToken:                  viper.GetString("TELEGRAM_BOT_TOKEN"),
			ChatID:                    viper.GetString("TELEGRAM_CHAT_ID"),
			WebhookURL:                viper.GetString("TELEGRAM_WEBHOOK_URL"),
			TimeoutDuration:           viper.GetDuration("TELEGRAM_TIMEOUT_DURATION"),
			MaxGlobalRequestPerSecond: viper.GetInt("TELEGRAM_MAX_GLOBAL_REQUEST_PER_SECOND"),
			MaxUserRequestPerSecond:   viper.GetInt("TELEGRAM_MAX_USER_REQUEST_PER_SECOND"),
			MaxEditMessagePerSecond:   viper.GetInt("TELEGRAM_MAX_EDIT_MESSAGE_PER_SECOND"),
			RatelimitExpireDuration:   viper.GetDuration("TELEGRAM_RATELIMIT_EXPIRE_DURATION"),
			RateLimitCleanupDuration:  viper.GetDuration("TELEGRAM_RATELIMIT_CLEANUP_DURATION"),
			FeatureStockAnalyze: TelegramFeatureStockAnalyze{
				AfterTimestampDuration: viper.GetDuration("TELEGRAM_FEATURE_STOCK_ANALYZE_AFTER_TIMESTAMP_DURATION"),
				ExpectedTFCount:        viper.GetInt("TELEGRAM_FEATURE_STOCK_ANALYZE_EXPECTED_TF_COUNT"),
			},
			TimeoutAsyncDuration: viper.GetDuration("TELEGRAM_TIMEOUT_ASYNC_DURATION"),
		},
		YahooFinance: YahooFinance{
			BaseURL:             viper.GetString("YAHOO_FINANCE_BASE_URL"),
			Timeout:             viper.GetDuration("YAHOO_FINANCE_TIMEOUT"),
			MaxRequestPerMinute: viper.GetInt("YAHOO_FINANCE_MAX_REQUEST_PER_MINUTE"),
		},
		Gemini: Gemini{
			APIKey:              viper.GetString("GEMINI_API_KEY"),
			BaseModel:           viper.GetString("GEMINI_BASE_MODEL"),
			MaxRequestPerMinute: viper.GetInt("GEMINI_MAX_REQUEST_PER_MINUTE"),
			MaxTokenPerMinute:   viper.GetInt("GEMINI_MAX_TOKEN_PER_MINUTE"),
			BaseURL:             viper.GetString("GEMINI_BASE_URL"),
			Timeout:             viper.GetDuration("GEMINI_TIMEOUT"),
		},
		Trading: Trading{
			RiskRewardRatio: viper.GetFloat64("TRADING_RISK_REWARD_RATIO"),
			MaxBuyList:      viper.GetInt("TRADING_MAX_BUY_LIST"),
		},
		StockAnalyzer: StockAnalyzer{
			MaxConcurrency: viper.GetInt("STOCK_ANALYZER_MAX_CONCURRENCY"),
		},
	}

	return &cfg, nil
}
