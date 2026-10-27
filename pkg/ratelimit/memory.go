package ratelimit

import (
	"context"
	"sync"
	"time"
)

// memoryLimiter 进程内固定窗口计数器。
//
// 只适合单实例部署（本地开发、单机小流量）。
// 多实例时每个进程各算各的，实际放行次数会翻倍，生产必须换 Redis 实现。
type memoryLimiter struct {
	cfg Config
	mu  sync.Mutex
	m   map[string]*entry
}

type entry struct {
	count    int
	expireAt time.Time
}

// NewMemory 创建内存限流器。
func NewMemory(cfg Config) Limiter {
	return &memoryLimiter{
		cfg: cfg.normalized(),
		m:   make(map[string]*entry),
	}
}

func (l *memoryLimiter) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.m[key]
	if !ok || now.After(e.expireAt) {
		// 窗口不存在或已过期，重新开始计数
		e = &entry{expireAt: now.Add(l.cfg.Window)}
		l.m[key] = e
	}

	e.count++
	if e.count > l.cfg.Limit {
		return false, e.expireAt.Sub(now), nil
	}

	// 顺手清理过期 key，避免大量不同用户名把内存撑爆
	if len(l.m) > 10000 {
		for k, v := range l.m {
			if now.After(v.expireAt) {
				delete(l.m, k)
			}
		}
	}
	return true, 0, nil
}

func (l *memoryLimiter) Reset(_ context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.m, key)
	return nil
}
