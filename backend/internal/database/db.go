package database

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tzwall/backend/internal/config"
)

// Open 建立 PostgreSQL 连接池。
func Open(cfg *config.Config, log *slog.Logger) (*gorm.DB, error) {
	gormLog := logger.New(
		slogWriter{},
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // 权限/配置回源查不到属正常路径，避免日志噪音
			Colorful:                  false,
		},
	)
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{Logger: gormLog})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	return db, nil
}

// slogWriter 将 GORM 日志接入结构化日志。
type slogWriter struct{}

func (slogWriter) Printf(format string, args ...interface{}) {
	slog.Default().Info(fmt.Sprintf(format, args...))
}
