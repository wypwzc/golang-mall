package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"go-mall/pkg/config"
)

// OpenRedis 创建 Redis 客户端，同样带启动重试。
func OpenRedis(ctx context.Context, cfg config.Redis, log *slog.Logger) (*redis.Client, error) {
	// 客户端只创建一次，重试期间复用
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	err := retry(ctx, log, "redis", 30, 2*time.Second, func(ctx context.Context) error {
		return rdb.Ping(ctx).Err()
	})
	if err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}
	return rdb, nil
}

// PingRedis 供 /readyz 使用。
func PingRedis(ctx context.Context, rdb *redis.Client) error {
	return rdb.Ping(ctx).Err()
}

// CloseRedis 关闭客户端。
func CloseRedis(rdb *redis.Client) {
	if rdb != nil {
		_ = rdb.Close()
	}
}
