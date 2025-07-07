package strategy

import (
	"context"
	"golang-trading/internal/model"
)

const (
	JOB_STATUS_SUCCESS = "success"
	JOB_STATUS_FAILED  = "failed"
	JOB_STATUS_SKIPPED = "skipped"
)

type JobType string

const (
	JobTypeHTTP                 JobType = "http_request"
	JobTypeStockNewsScraper     JobType = "stock_news_scraper"
	JobTypeStockNewsSummary     JobType = "stock_news_summary"
	JobTypeStockPriceAlert      JobType = "stock_price_alert"
	JobTypeStockAnalyzer        JobType = "stock_analyzer"
	JobTypeStockPositionMonitor JobType = "stock_position_monitor"
)

// JobExecutionStrategy defines the interface for different job execution strategies.
type JobExecutionStrategy interface {
	Execute(ctx context.Context, job *model.Job) (string, error)
	GetType() JobType
}
