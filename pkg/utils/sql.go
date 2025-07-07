package utils

import "gorm.io/gorm"

type DBOption func(*gorm.DB) *gorm.DB

func ApplyOptions(db *gorm.DB, opts ...DBOption) *gorm.DB {
	for _, opt := range opts {
		db = opt(db)
	}
	return db
}

func WithTx(tx *gorm.DB) DBOption {
	return func(_ *gorm.DB) *gorm.DB {
		return tx
	}
}

func WithPreload(column string) DBOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(column)
	}
}

func WithWhere(query interface{}, args ...interface{}) DBOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(query, args...)
	}
}
