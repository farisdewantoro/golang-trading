package repository

import (
	"context"
	"encoding/xml"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/pkg/logger"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// NewsRepository fetches news articles from an external news source.
type NewsRepository interface {
	Search(ctx context.Context, param dto.SearchNewsParam) ([]dto.NewsItem, error)
}

// googleNewsRepository fetches news from the Google News RSS search feed.
type googleNewsRepository struct {
	cfg            *config.Config
	logger         *logger.Logger
	client         *http.Client
	requestLimiter *rate.Limiter
	mu             sync.Mutex
}

func NewGoogleNewsRepository(cfg *config.Config, log *logger.Logger) NewsRepository {
	perRequest := time.Minute / time.Duration(cfg.News.MaxRequestPerMinute)
	return &googleNewsRepository{
		cfg:            cfg,
		logger:         log,
		client:         &http.Client{Timeout: cfg.News.Timeout},
		requestLimiter: rate.NewLimiter(rate.Every(perRequest), 1),
	}
}

func (r *googleNewsRepository) Search(ctx context.Context, param dto.SearchNewsParam) ([]dto.NewsItem, error) {
	if strings.TrimSpace(param.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	r.mu.Lock()
	err := r.requestLimiter.Wait(ctx)
	r.mu.Unlock()
	if err != nil {
		return nil, err
	}

	lang := param.Language
	if lang == "" {
		lang = "id"
	}
	country := param.Country
	if country == "" {
		country = "ID"
	}

	q := url.Values{}
	q.Set("q", param.Query)
	q.Set("hl", lang)
	q.Set("gl", country)
	q.Set("ceid", fmt.Sprintf("%s:%s", country, lang))
	endpoint := r.cfg.News.BaseURL + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build news request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; golang-trading-bot/1.0)")
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read news response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		r.logger.ErrorContext(ctx, "News source returned non-OK status",
			logger.IntField("status_code", resp.StatusCode),
			logger.StringField("query", param.Query))
		return nil, fmt.Errorf("news source returned status: %d", resp.StatusCode)
	}

	return ParseGoogleNewsRSS(body)
}

// ParseGoogleNewsRSS parses a Google News RSS document into news items, newest first.
func ParseGoogleNewsRSS(body []byte) ([]dto.NewsItem, error) {
	var feed dto.GoogleNewsRSS
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse news rss: %w", err)
	}

	items := make([]dto.NewsItem, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		link := strings.TrimSpace(it.Link)
		if link == "" {
			continue
		}
		source := strings.TrimSpace(it.Source.Name)
		title := strings.TrimSpace(it.Title)
		// Google News appends " - <source>" to every title.
		if source != "" {
			title = strings.TrimSpace(strings.TrimSuffix(title, " - "+source))
		}

		item := dto.NewsItem{
			Title:  title,
			Link:   link,
			GUID:   strings.TrimSpace(it.GUID),
			Source: source,
		}
		if pub, err := parseRSSDate(it.PubDate); err == nil {
			item.PublishedAt = pub
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})

	return items, nil
}

func parseRSSDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC822, time.RFC822Z, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown date format: %q", s)
}
