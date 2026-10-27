package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"go-mall/pkg/ratelimit"
)

func TestMemoryAllowsUpToLimit(t *testing.T) {
	l := ratelimit.NewMemory(ratelimit.Config{Limit: 3, Window: time.Minute})
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		allowed, _, err := l.Allow(ctx, "login:alice")
		if err != nil {
			t.Fatalf("第 %d 次出错: %v", i, err)
		}
		if !allowed {
			t.Fatalf("第 %d 次应放行", i)
		}
	}

	allowed, retryAfter, err := l.Allow(ctx, "login:alice")
	if err != nil {
		t.Fatalf("出错: %v", err)
	}
	if allowed {
		t.Fatal("超过阈值后应拒绝")
	}
	if retryAfter <= 0 {
		t.Fatalf("拒绝时应返回等待时间，实际 %v", retryAfter)
	}
}

func TestMemoryIsolatesKeys(t *testing.T) {
	l := ratelimit.NewMemory(ratelimit.Config{Limit: 1, Window: time.Minute})
	ctx := context.Background()

	if allowed, _, _ := l.Allow(ctx, "login:alice"); !allowed {
		t.Fatal("alice 首次应放行")
	}
	if allowed, _, _ := l.Allow(ctx, "login:alice"); allowed {
		t.Fatal("alice 第二次应拒绝")
	}
	// 不同 key 之间不能互相影响
	if allowed, _, _ := l.Allow(ctx, "login:bob"); !allowed {
		t.Fatal("bob 不应被 alice 的计数影响")
	}
}

func TestMemoryReset(t *testing.T) {
	l := ratelimit.NewMemory(ratelimit.Config{Limit: 1, Window: time.Minute})
	ctx := context.Background()

	l.Allow(ctx, "login:alice")
	if allowed, _, _ := l.Allow(ctx, "login:alice"); allowed {
		t.Fatal("应被限流")
	}

	if err := l.Reset(ctx, "login:alice"); err != nil {
		t.Fatalf("重置失败: %v", err)
	}
	if allowed, _, _ := l.Allow(ctx, "login:alice"); !allowed {
		t.Fatal("重置后应重新放行")
	}
}

func TestMemoryWindowExpires(t *testing.T) {
	l := ratelimit.NewMemory(ratelimit.Config{Limit: 1, Window: 50 * time.Millisecond})
	ctx := context.Background()

	l.Allow(ctx, "login:alice")
	if allowed, _, _ := l.Allow(ctx, "login:alice"); allowed {
		t.Fatal("窗口内应拒绝")
	}

	time.Sleep(60 * time.Millisecond)
	if allowed, _, _ := l.Allow(ctx, "login:alice"); !allowed {
		t.Fatal("窗口过期后应重新放行")
	}
}
