// Package database 负责 MySQL / Redis 连接的创建与健康检查。
//
// 连接池只在服务启动时创建一次，绝不在请求里新建连接。
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"go-mall/pkg/config"
)

// OpenMySQL 创建 MySQL 连接池。
// 容器启动时数据库可能还没就绪，因此带重试：默认最多等 60 秒。
func OpenMySQL(ctx context.Context, cfg config.MySQL, log *slog.Logger) (*gorm.DB, error) {
	var db *gorm.DB

	err := retry(ctx, log, "mysql", 30, 2*time.Second, func(ctx context.Context) error {
		d, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
			// 关闭 GORM 的默认事务：单条写操作不需要额外 BEGIN/COMMIT，需要事务时显式 db.Transaction
			SkipDefaultTransaction: true,
			Logger:                 gormlogger.Default.LogMode(gormlogger.Warn),
			NowFunc:                func() time.Time { return time.Now().UTC() },
		})
		if err != nil {
			return err
		}

		sqlDB, err := d.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

		if err := sqlDB.PingContext(ctx); err != nil {
			_ = sqlDB.Close()
			return err
		}
		db = d
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	return db, nil
}

// PingMySQL 供 /readyz 使用。
func PingMySQL(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// CloseMySQL 关闭连接池。
func CloseMySQL(db *gorm.DB) {
	if db == nil {
		return
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
