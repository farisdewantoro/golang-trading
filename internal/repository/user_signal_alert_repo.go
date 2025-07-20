package repository

import (
	"context"
	"fmt"
	"golang-trading/internal/model"
	"golang-trading/pkg/utils"
	"strings"

	"gorm.io/gorm"
)

type UserSignalAlertRepository interface {
	Get(ctx context.Context, param *model.GetUserSignalAlertParam, opts ...utils.DBOption) ([]model.UserSignalAlert, error)
	Update(ctx context.Context, userSignalAlert *model.UserSignalAlert, opts ...utils.DBOption) error
}

type userSignalAlertRepository struct {
	db *gorm.DB
}

func NewUserSignalAlertRepository(db *gorm.DB) UserSignalAlertRepository {
	return &userSignalAlertRepository{
		db: db,
	}
}

func (r *userSignalAlertRepository) Get(ctx context.Context, param *model.GetUserSignalAlertParam, opts ...utils.DBOption) ([]model.UserSignalAlert, error) {
	var userSignalAlerts []model.UserSignalAlert
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)

	qFilter := []string{}
	qFilterParam := []interface{}{}

	if param.TelegramID != nil {
		qFilter = append(qFilter, "users.telegram_id = ?")
		qFilterParam = append(qFilterParam, *param.TelegramID)
	}

	if param.Exchange != nil {
		qFilter = append(qFilter, "user_signal_alerts.exchange = ?")
		qFilterParam = append(qFilterParam, *param.Exchange)
	}

	if param.IsActive != nil {
		qFilter = append(qFilter, "user_signal_alerts.is_active = ?")
		qFilterParam = append(qFilterParam, *param.IsActive)
	}

	if len(qFilter) == 0 {
		return nil, fmt.Errorf("no filter provided")
	}

	if err := tx.Joins("JOIN users ON user_signal_alerts.user_id = users.id").
		Where(strings.Join(qFilter, " AND "), qFilterParam...).
		Find(&userSignalAlerts).Error; err != nil {
		return nil, err
	}

	return userSignalAlerts, nil
}

func (r *userSignalAlertRepository) Update(ctx context.Context, userSignalAlert *model.UserSignalAlert, opts ...utils.DBOption) error {
	tx := utils.ApplyOptions(r.db.WithContext(ctx), opts...)

	if err := tx.Save(userSignalAlert).Error; err != nil {
		return err
	}

	return nil
}
