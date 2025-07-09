package strategy

import (
	"context"
	"golang-trading/internal/model"
)

const (
	JOB_EXIT_CODE_SUCCESS         = 200
	JOB_EXIT_CODE_FAILED          = 500
	JOB_EXIT_CODE_SKIPPED         = 204
	JOB_EXIT_CODE_PARTIAL_SUCCESS = 206
)

type JobType string

const (
	JobTypeHTTP                   JobType = "http_request"
	JobTypeStockNewsScraper       JobType = "stock_news_scraper"
	JobTypeStockNewsSummary       JobType = "stock_news_summary"
	JobTypeStockPriceAlert        JobType = "stock_price_alert"
	JobTypeStockAnalyzer          JobType = "stock_analyzer"
	JobTypeStockPositionMonitor   JobType = "stock_position_monitor"
	JobTypeStockTechnicalAnalysis JobType = "stock_technical_analysis"
)

type JobResult struct {
	ExitCode int32  `json:"exit_code"`
	Output   string `json:"output"`
}

// JobExecutionStrategy defines the interface for different job execution strategies.
type JobExecutionStrategy interface {
	Execute(ctx context.Context, job *model.Job) (JobResult, error)
	GetType() JobType
}
