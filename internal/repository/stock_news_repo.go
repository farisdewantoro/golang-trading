package repository

import (
	"context"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StockNewsRepository stores news that has already been seen, for de-duplication.
type StockNewsRepository interface {
	// InsertIfNotExists stores the news and reports whether it was new.
	InsertIfNotExists(ctx context.Context, news *model.StockNews, opts ...utils.DBOption) (bool, error)
}

type stockNewsRepository struct {
	db *gorm.DB
}

func NewStockNewsRepository(db *gorm.DB) StockNewsRepository {
	return &stockNewsRepository{db: db}
}

func (r *stockNewsRepository) InsertIfNotExists(ctx context.Context, news *model.StockNews, opts ...utils.DBOption) (bool, error) {
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "hash_identifier"}},
		DoNothing: true,
	}).Create(news)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// NewsWatchlistRepository manages the stocks whose news a user follows.
type NewsWatchlistRepository interface {
	Get(ctx context.Context, param model.GetUserNewsWatchlistParam, opts ...utils.DBOption) ([]model.UserNewsWatchlist, error)
	Upsert(ctx context.Context, watchlist *model.UserNewsWatchlist, opts ...utils.DBOption) error
}

type newsWatchlistRepository struct {
	db *gorm.DB
}

func NewNewsWatchlistRepository(db *gorm.DB) NewsWatchlistRepository {
	return &newsWatchlistRepository{db: db}
}

func (r *newsWatchlistRepository) Get(ctx context.Context, param model.GetUserNewsWatchlistParam, opts ...utils.DBOption) ([]model.UserNewsWatchlist, error) {
	var result []model.UserNewsWatchlist
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)

	qFilter := []string{}
	qParam := []interface{}{}

	if param.TelegramID != nil {
		qFilter = append(qFilter, "users.telegram_id = ?")
		qParam = append(qParam, *param.TelegramID)
	}
	if param.StockCode != nil {
		qFilter = append(qFilter, "user_news_watchlists.stock_code = ?")
		qParam = append(qParam, *param.StockCode)
	}
	if param.Exchange != nil {
		qFilter = append(qFilter, "user_news_watchlists.exchange = ?")
		qParam = append(qParam, *param.Exchange)
	}
	if param.IsActive != nil {
		qFilter = append(qFilter, "user_news_watchlists.is_active = ?")
		qParam = append(qParam, *param.IsActive)
	}

	tx = tx.Joins("JOIN users ON user_news_watchlists.user_id = users.id").
		Preload("User").
		Order("user_news_watchlists.stock_code ASC")
	if len(qFilter) > 0 {
		tx = tx.Where(strings.Join(qFilter, " AND "), qParam...)
	}

	if err := tx.Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

func (r *newsWatchlistRepository) Upsert(ctx context.Context, watchlist *model.UserNewsWatchlist, opts ...utils.DBOption) error {
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)
	return tx.Omit("User").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "stock_code"}, {Name: "exchange"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"is_active": watchlist.IsActive, "updated_at": utils.TimeNowWIB()}),
	}).Create(watchlist).Error
}
