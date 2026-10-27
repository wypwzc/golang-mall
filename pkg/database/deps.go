package database

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"go-mall/pkg/config"
)

// Deps 一个服务启动时需要的全部外部依赖。
//
// Redis 是可选的：本地开发没有 Redis 时把 REDIS_ENABLED 设为 false，
// 服务仍能启动，只是用不上限流和缓存（相关功能会退化为无缓存/内存实现）。
type Deps struct {
	DB    *gorm.DB
	Redis *redis.Client
}

// Connect 建立所有依赖连接。任一必需依赖失败则返回错误，由 main 决定是否退出。
func Connect(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Deps, error) {
	d := &Deps{}

	db, err := OpenMySQL(ctx, cfg.MySQL, log)
	if err != nil {
		return nil, err
	}
	d.DB = db

	if cfg.Redis.Enabled {
		rdb, err := OpenRedis(ctx, cfg.Redis, log)
		if err != nil {
			CloseMySQL(db)
			return nil, err
		}
		d.Redis = rdb
	} else {
		log.Warn("REDIS_ENABLED=false，已跳过 Redis，仅适用于本地开发")
	}

	return d, nil
}

// Ready 供 /readyz 调用：所有已连接的依赖都可用才算就绪。
func (d *Deps) Ready(ctx context.Context) error {
	if err := PingMySQL(ctx, d.DB); err != nil {
		return fmt.Errorf("mysql: %w", err)
	}
	if d.Redis != nil {
		if err := PingRedis(ctx, d.Redis); err != nil {
			return fmt.Errorf("redis: %w", err)
		}
	}
	return nil
}

// Close 释放所有连接。
func (d *Deps) Close() {
	CloseRedis(d.Redis)
	CloseMySQL(d.DB)
}
