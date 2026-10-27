package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisLimiter 基于 Redis 的固定窗口计数器，多实例共享同一份计数。
type redisLimiter struct {
	rdb *redis.Client
	cfg Config
}

// NewRedis 创建 Redis 限流器。
func NewRedis(rdb *redis.Client, cfg Config) Limiter {
	return &redisLimiter{rdb: rdb, cfg: cfg.normalized()}
}

// 用 Lua 保证 INCR 与 PEXPIRE 的原子性：
// 如果分两条命令发，进程在中间崩溃会留下永不过期的 key，导致该用户被永久锁定。
const incrScript = `
local c = redis.call('INCR', KEYS[1])
if c == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
return {c, ttl}
`

func (l *redisLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	res, err := l.rdb.Eval(ctx, incrScript, []string{key}, l.cfg.Window.Milliseconds()).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis 限流失败: %w", err)
	}

	values, ok := res.([]any)
	if !ok || len(values) != 2 {
		return false, 0, fmt.Errorf("redis 限流返回格式异常: %v", res)
	}

	count, _ := values[0].(int64)
	ttlMs, _ := values[1].(int64)

	if count > int64(l.cfg.Limit) {
		if ttlMs < 0 {
			ttlMs = 0
		}
		return false, time.Duration(ttlMs) * time.Millisecond, nil
	}
	return true, 0, nil
}

func (l *redisLimiter) Reset(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, key).Err()
}
