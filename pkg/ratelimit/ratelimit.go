// Package ratelimit 提供固定窗口计数器，用于登录失败限流等场景。
//
// 为什么抽成接口：本地开发往往没有 Redis，而生产必须用 Redis 才能多实例共享计数。
// 抽一个只有两个方法的接口，就能让同一份业务代码在两种环境下都跑得通，
// 又不至于引入一整套"限流框架"。
package ratelimit

import (
	"context"
	"time"
)

// Limiter 限流器。
type Limiter interface {
	// Allow 记一次操作并判断是否放行。
	// 返回 false 时，retryAfter 表示还需要等待多久。
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
	// Reset 清零计数，例如登录成功后清除该用户的失败记录。
	Reset(ctx context.Context, key string) error
}

// Config 限流参数。
type Config struct {
	Limit  int           // 窗口内允许的最大次数
	Window time.Duration // 窗口长度
}

func (c Config) normalized() Config {
	if c.Limit <= 0 {
		c.Limit = 5
	}
	if c.Window <= 0 {
		c.Window = 5 * time.Minute
	}
	return c
}
