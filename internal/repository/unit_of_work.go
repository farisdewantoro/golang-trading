package repository

import (
	"fmt"
	"golang-trading/pkg/utils"

	"gorm.io/gorm"
)

type UnitOfWork interface {
	Begin() *gorm.DB
	Commit() error
	Rollback() error
	Run(fn func(opts ...utils.DBOption) error) (err error)
}

type unitOfWork struct {
	db *gorm.DB
}

func NewUnitOfWork(db *gorm.DB) UnitOfWork {
	return &unitOfWork{
		db: db,
	}
}

func (u *unitOfWork) Begin() *gorm.DB {
	return u.db.Begin()
}

func (u *unitOfWork) Commit() error {
	return u.db.Commit().Error
}

func (u *unitOfWork) Rollback() error {
	return u.db.Rollback().Error
}

func (u *unitOfWork) Run(fn func(opts ...utils.DBOption) error) (err error) {
	tx := u.db.Begin()

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
		if err != nil {
			_ = tx.Rollback()
		} else {
			if commitErr := tx.Commit().Error; commitErr != nil {
				err = fmt.Errorf("commit failed: %w", commitErr)
			}
		}
	}()

	err = fn(utils.WithTx(tx))
	return
}
