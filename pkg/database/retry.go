package database

import (
	"context"
	"log/slog"
	"time"
)

// retry 带固定间隔的重试，用于等待依赖服务就绪。
// 只处理"依赖还没起来"这类瞬时失败；ctx 取消或重试次数用尽即返回最后一次错误。
func retry(ctx context.Context, log *slog.Logger, name string, attempts int, interval time.Duration, fn func(context.Context) error) error {
	var err error
	for i := 1; i <= attempts; i++ {
		// 单次尝试也要有超时，避免某次拨号卡死拖垮整个启动流程
		attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = fn(attemptCtx)
		cancel()

		if err == nil {
			if i > 1 {
				log.Info("依赖已就绪", "target", name, "attempt", i)
			}
			return nil
		}

		log.Warn("依赖未就绪，准备重试", "target", name, "attempt", i, "err", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return err
}
