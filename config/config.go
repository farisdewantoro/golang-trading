package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Log         Logger         `mapstructure:"logger"`
	DB          Database       `mapstructure:"database"`
	API         API            `mapstructure:"api"`
	Scheduler   Scheduler      `mapstructure:"scheduler"`
	TradingView TradingView    `mapstructure:"tradingview"`
	Cache       Cache          `mapstructure:"cache"`
	Telegram    TelegramConfig `mapstructure:"telegram"`
}

type Logger struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

type Database struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"name"`
	SSLMode         string `mapstructure:"ssl_mode"`
	TimeZone        string `mapstructure:"time_zone"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
	LogLevel        string `mapstructure:"log_level"`
}

type Scheduler struct {
	MaxConcurrency  int           `mapstructure:"max_concurrency"`
	TimeoutDuration time.Duration `mapstructure:"timeout_duration"`
}

type API struct {
	Port int `mapstructure:"port"`
}

type TradingView struct {
	BaseURLScanner   string        `mapstructure:"base_url_scanner"`
	BaseTimeout      time.Duration `mapstructure:"base_timeout"`
	MaxRequestPerMin int           `mapstructure:"max_request_per_min"`
}

type Cache struct {
	DefaultExpiration time.Duration `mapstructure:"default_expiration"`
	CleanupInterval   time.Duration `mapstructure:"cleanup_interval"`
}

type TelegramConfig struct {
	BotToken                  string        `mapstructure:"bot_token"`
	ChatID                    string        `mapstructure:"chat_id"`
	WebhookURL                string        `mapstructure:"webhook_url"`
	TimeoutDuration           time.Duration `mapstructure:"timeout_duration"`
	TimeoutBuyListDuration    time.Duration `mapstructure:"timeout_buy_list_duration"`
	MaxGlobalRequestPerSecond int           `mapstructure:"max_global_request_per_second"`
	MaxUserRequestPerSecond   int           `mapstructure:"max_user_request_per_second"`
	MaxEditMessagePerSecond   int           `mapstructure:"max_edit_message_per_second"`
	RatelimitExpireDuration   time.Duration `mapstructure:"ratelimit_expire_duration"`
	RateLimitCleanupDuration  time.Duration `mapstructure:"rate_limit_cleanup_duration"`
	FeatureNewsMaxAgeInDays   int           `mapstructure:"feature_news_max_age_in_days"`
	FeatureNewsLimitStockNews int           `mapstructure:"feature_news_limit_stock_news"`
	MaxShowHistoryAnalysis    int           `mapstructure:"max_show_history_analysis"`
}

func Load() (*Config, error) {
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("No config file loaded:", err)
	}

	fmt.Println("Viper settings for environment variables:")
	for _, key := range viper.AllKeys() {
		if viper.IsSet(key) {
			fmt.Printf("  Key: %s, Value: %v\n", key, viper.Get(key))
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
