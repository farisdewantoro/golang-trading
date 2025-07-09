package repository

import (
	"context"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"

	"gorm.io/gorm"
)

type UserRepository interface {
	GetUserByTelegramID(ctx context.Context, telegramID int64, opts ...utils.DBOption) (*model.User, error)
	CreateUser(ctx context.Context, user *model.User, opts ...utils.DBOption) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		db: db,
	}
}

func (r *userRepository) GetUserByTelegramID(ctx context.Context, telegramID int64, opts ...utils.DBOption) (*model.User, error) {
	var user model.User
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)

	result := tx.Where("telegram_id = ?", telegramID).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}

		return nil, result.Error
	}

	return &user, nil
}

func (r *userRepository) CreateUser(ctx context.Context, user *model.User, opts ...utils.DBOption) error {
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)
	return tx.Create(user).Error
}
