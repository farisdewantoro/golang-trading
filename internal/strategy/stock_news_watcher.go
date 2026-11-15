package strategy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/telegram"
	"golang-trading/pkg/utils"
	"sort"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

const (
	defaultNewsQueryTemplate    = `"%s" saham`
	defaultNewsMaxAge           = 24 * time.Hour
	defaultNewsMaxItemsPerStock = 5
)

// StockNewsWatcherPayload is the job payload for the news watcher.
type StockNewsWatcherPayload struct {
	Exchange         string `json:"exchange"`
	MaxAge           string `json:"max_age"`
	MaxItemsPerStock int    `json:"max_items_per_stock"`
	IncludePositions bool   `json:"include_positions"`
	QueryTemplate    string `json:"query_template"`
	Language         string `json:"language"`
	Country          string `json:"country"`
}

type StockNewsWatcherResult struct {
	StockCode string `json:"stock_code"`
	Fetched   int    `json:"fetched"`
	NewNews   int    `json:"new_news"`
	Notified  int    `json:"notified"`
	Errors    string `json:"errors,omitempty"`
}

// StockNewsWatcherStrategy watches news for stocks users follow (news watchlist
// and active positions) and notifies them on Telegram when new articles appear.
type StockNewsWatcherStrategy struct {
	cfg               *config.Config
	logger            *logger.Logger
	telegram          *telegram.TelegramRateLimiter
	newsRepo          repository.NewsRepository
	stockNewsRepo     repository.StockNewsRepository
	newsWatchlistRepo repository.NewsWatchlistRepository
	stockPositionRepo repository.StockPositionsRepository
}

func NewStockNewsWatcherStrategy(
	cfg *config.Config,
	log *logger.Logger,
	telegram *telegram.TelegramRateLimiter,
	newsRepo repository.NewsRepository,
	stockNewsRepo repository.StockNewsRepository,
	newsWatchlistRepo repository.NewsWatchlistRepository,
	stockPositionRepo repository.StockPositionsRepository,
) JobExecutionStrategy {
	return &StockNewsWatcherStrategy{
		cfg:               cfg,
		logger:            log,
		telegram:          telegram,
		newsRepo:          newsRepo,
		stockNewsRepo:     stockNewsRepo,
		newsWatchlistRepo: newsWatchlistRepo,
		stockPositionRepo: stockPositionRepo,
	}
}

func (s *StockNewsWatcherStrategy) GetType() JobType {
	return JobTypeStockNewsWatcher
}

func (s *StockNewsWatcherStrategy) Execute(ctx context.Context, job *model.Job) (JobResult, error) {
	var payload StockNewsWatcherPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to unmarshal job payload: %v", err)}, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	maxAge := defaultNewsMaxAge
	if payload.MaxAge != "" {
		d, err := time.ParseDuration(payload.MaxAge)
		if err != nil {
			return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("invalid max_age: %v", err)}, fmt.Errorf("invalid max_age: %w", err)
		}
		maxAge = d
	}
	if payload.MaxItemsPerStock <= 0 {
		payload.MaxItemsPerStock = defaultNewsMaxItemsPerStock
	}

	subscribers, err := s.collectSubscribers(ctx, payload)
	if err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to collect subscribers: %v", err)}, err
	}
	if len(subscribers) == 0 {
		return JobResult{ExitCode: JOB_EXIT_CODE_SKIPPED, Output: "no stock to watch"}, nil
	}

	stockCodes := make([]string, 0, len(subscribers))
	for code := range subscribers {
		stockCodes = append(stockCodes, code)
	}
	sort.Strings(stockCodes)

	results := make([]StockNewsWatcherResult, 0, len(stockCodes))
	hasError := false

	for _, stockCode := range stockCodes {
		if !utils.ShouldContinue(ctx, s.logger) {
			break
		}

		result := s.processStock(ctx, payload, stockCode, maxAge, subscribers[stockCode])
		if result.Errors != "" {
			hasError = true
		}
		results = append(results, result)
	}

	output, err := json.Marshal(results)
	if err != nil {
		return JobResult{ExitCode: JOB_EXIT_CODE_FAILED, Output: fmt.Sprintf("failed to marshal results: %v", err)}, err
	}

	exitCode := int32(JOB_EXIT_CODE_SUCCESS)
	if hasError {
		exitCode = JOB_EXIT_CODE_PARTIAL_SUCCESS
	}
	return JobResult{ExitCode: exitCode, Output: string(output)}, nil
}

// collectSubscribers returns stock code -> set of telegram IDs that follow its news.
func (s *StockNewsWatcherStrategy) collectSubscribers(ctx context.Context, payload StockNewsWatcherPayload) (map[string]map[int64]struct{}, error) {
	subscribers := make(map[string]map[int64]struct{})
	add := func(stockCode string, telegramID int64) {
		stockCode = strings.ToUpper(strings.TrimSpace(stockCode))
		if stockCode == "" || telegramID == 0 {
			return
		}
		if _, ok := subscribers[stockCode]; !ok {
			subscribers[stockCode] = make(map[int64]struct{})
		}
		subscribers[stockCode][telegramID] = struct{}{}
	}

	watchlists, err := s.newsWatchlistRepo.Get(ctx, model.GetUserNewsWatchlistParam{
		Exchange: utils.ToPointer(payload.Exchange),
		IsActive: utils.ToPointer(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get news watchlist: %w", err)
	}
	for _, w := range watchlists {
		add(w.StockCode, w.User.TelegramID)
	}

	if payload.IncludePositions {
		positions, err := s.stockPositionRepo.Get(ctx, dto.GetStockPositionsParam{
			IsActive: utils.ToPointer(true),
			Exchange: utils.ToPointer(payload.Exchange),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get stock positions: %w", err)
		}
		for _, p := range positions {
			add(p.StockCode, p.User.TelegramID)
		}
	}

	return subscribers, nil
}

func (s *StockNewsWatcherStrategy) processStock(ctx context.Context, payload StockNewsWatcherPayload, stockCode string, maxAge time.Duration, telegramIDs map[int64]struct{}) StockNewsWatcherResult {
	result := StockNewsWatcherResult{StockCode: stockCode}

	items, err := s.newsRepo.Search(ctx, dto.SearchNewsParam{
		Query:    BuildNewsQuery(payload.QueryTemplate, stockCode),
		Language: payload.Language,
		Country:  payload.Country,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to fetch news", logger.ErrorField(err), logger.StringField("stock_code", stockCode))
		result.Errors = err.Error()
		return result
	}

	items = FilterRecentNews(items, utils.TimeNowWIB(), maxAge, payload.MaxItemsPerStock)
	result.Fetched = len(items)

	newItems := make([]dto.NewsItem, 0, len(items))
	for _, item := range items {
		news := &model.StockNews{
			StockCode:      stockCode,
			Exchange:       payload.Exchange,
			Title:          item.Title,
			Link:           item.Link,
			Source:         item.Source,
			HashIdentifier: NewsHash(payload.Exchange, stockCode, item),
		}
		if !item.PublishedAt.IsZero() {
			news.PublishedAt = utils.ToPointer(item.PublishedAt)
		}

		isNew, err := s.stockNewsRepo.InsertIfNotExists(ctx, news)
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to store news", logger.ErrorField(err), logger.StringField("stock_code", stockCode))
			result.Errors = err.Error()
			continue
		}
		if isNew {
			newItems = append(newItems, item)
		}
	}
	result.NewNews = len(newItems)

	if len(newItems) == 0 {
		return result
	}

	message := telegram.FormatStockNewsForTelegram("📰 Berita Terbaru", stockCode, ToNewsLines(newItems))
	menu := &telebot.ReplyMarkup{}
	menu.Inline(
		menu.Row(
			menu.Data("🔕 Berhenti Pantau", "btn_unwatch_news", fmt.Sprintf("%s:%s", payload.Exchange, stockCode)),
			menu.Data("🗑️ Hapus Pesan", "btn_delete_message"),
		),
	)

	for telegramID := range telegramIDs {
		if err := s.telegram.SendMessageUser(ctx, message, telegramID, menu, telebot.ModeHTML, telebot.NoPreview); err != nil {
			s.logger.ErrorContext(ctx, "Failed to send news", logger.ErrorField(err), logger.StringField("stock_code", stockCode))
			result.Errors = err.Error()
			continue
		}
		result.Notified++
	}

	return result
}

// BuildNewsQuery builds the search query for a stock code.
func BuildNewsQuery(template, stockCode string) string {
	if template == "" || !strings.Contains(template, "%s") {
		template = defaultNewsQueryTemplate
	}
	return fmt.Sprintf(template, strings.ToUpper(stockCode))
}

// FilterRecentNews keeps news published within maxAge of now (newest first), capped at limit.
// News without a publish date is dropped because its freshness is unknown.
func FilterRecentNews(items []dto.NewsItem, now time.Time, maxAge time.Duration, limit int) []dto.NewsItem {
	cutoff := now.Add(-maxAge)
	filtered := make([]dto.NewsItem, 0, len(items))
	for _, item := range items {
		if item.PublishedAt.IsZero() || item.PublishedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].PublishedAt.After(filtered[j].PublishedAt)
	})
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

// NewsHash identifies a news article for a given stock, used for de-duplication.
func NewsHash(exchange, stockCode string, item dto.NewsItem) string {
	key := item.GUID
	if key == "" {
		key = item.Link
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(exchange) + "|" + strings.ToUpper(stockCode) + "|" + key))
	return hex.EncodeToString(sum[:])
}

func ToNewsLines(items []dto.NewsItem) []telegram.NewsLine {
	lines := make([]telegram.NewsLine, 0, len(items))
	for _, item := range items {
		lines = append(lines, telegram.NewsLine{
			Title:       item.Title,
			Link:        item.Link,
			Source:      item.Source,
			PublishedAt: item.PublishedAt,
		})
	}
	return lines
}
