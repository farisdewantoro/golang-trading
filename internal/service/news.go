package service

import (
	"context"
	"fmt"
	"golang-trading/config"
	"golang-trading/internal/dto"
	"golang-trading/internal/model"
	"golang-trading/internal/repository"
	"golang-trading/internal/strategy"
	"golang-trading/pkg/logger"
	"golang-trading/pkg/utils"
	"regexp"
	"strings"
	"time"
)

const (
	newsLatestMaxAge = 7 * 24 * time.Hour
	newsLatestLimit  = 7
)

var stockCodePattern = regexp.MustCompile(`^[A-Z0-9.\-]{1,15}$`)

type NewsService interface {
	GetWatchlist(ctx context.Context, telegramID int64) ([]model.UserNewsWatchlist, error)
	WatchNews(ctx context.Context, userTelegram *dto.RequestUserTelegram, exchange, stockCode string) error
	UnwatchNews(ctx context.Context, telegramID int64, exchange, stockCode string) error
	GetLatestNews(ctx context.Context, stockCode string) ([]dto.NewsItem, error)
}

type newsService struct {
	cfg               *config.Config
	log               *logger.Logger
	newsRepo          repository.NewsRepository
	newsWatchlistRepo repository.NewsWatchlistRepository
	userRepo          repository.UserRepository
}

func NewNewsService(cfg *config.Config, log *logger.Logger, repo *repository.Repository) NewsService {
	return &newsService{
		cfg:               cfg,
		log:               log,
		newsRepo:          repo.NewsRepo,
		newsWatchlistRepo: repo.NewsWatchlistRepo,
		userRepo:          repo.UserRepo,
	}
}

// NormalizeStockCode upper-cases and validates a stock code typed by a user.
func NormalizeStockCode(stockCode string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(stockCode))
	if !stockCodePattern.MatchString(code) {
		return "", fmt.Errorf("invalid stock code: %q", stockCode)
	}
	return code, nil
}

func (s *newsService) GetWatchlist(ctx context.Context, telegramID int64) ([]model.UserNewsWatchlist, error) {
	return s.newsWatchlistRepo.Get(ctx, model.GetUserNewsWatchlistParam{
		TelegramID: &telegramID,
		IsActive:   utils.ToPointer(true),
	})
}

func (s *newsService) WatchNews(ctx context.Context, userTelegram *dto.RequestUserTelegram, exchange, stockCode string) error {
	code, err := NormalizeStockCode(stockCode)
	if err != nil {
		return err
	}

	user, err := s.userRepo.GetUserByTelegramID(ctx, userTelegram.ID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		user = userTelegram.ToUserEntity()
		if err := s.userRepo.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return s.newsWatchlistRepo.Upsert(ctx, &model.UserNewsWatchlist{
		UserID:    user.ID,
		StockCode: code,
		Exchange:  exchange,
		IsActive:  utils.ToPointer(true),
	})
}

func (s *newsService) UnwatchNews(ctx context.Context, telegramID int64, exchange, stockCode string) error {
	code, err := NormalizeStockCode(stockCode)
	if err != nil {
		return err
	}

	user, err := s.userRepo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil
	}

	return s.newsWatchlistRepo.Upsert(ctx, &model.UserNewsWatchlist{
		UserID:    user.ID,
		StockCode: code,
		Exchange:  exchange,
		IsActive:  utils.ToPointer(false),
	})
}

func (s *newsService) GetLatestNews(ctx context.Context, stockCode string) ([]dto.NewsItem, error) {
	code, err := NormalizeStockCode(stockCode)
	if err != nil {
		return nil, err
	}

	items, err := s.newsRepo.Search(ctx, dto.SearchNewsParam{
		Query:    strategy.BuildNewsQuery("", code),
		Language: "id",
		Country:  "ID",
	})
	if err != nil {
		return nil, err
	}

	return strategy.FilterRecentNews(items, utils.TimeNowWIB(), newsLatestMaxAge, newsLatestLimit), nil
}
