// Package logger 统一日志初始化。
//
// dev 环境用可读的文本格式并带源码位置，prod 用 JSON 方便采集。
// 请求级日志通过 context 传递，自动带上 request_id。
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

// New 创建日志器，并设置为全局默认（GORM 等库会用到）。
func New(level, env string) *slog.Logger {
	var lv slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     lv,
		AddSource: env != "prod", // 生产环境不加源码位置，减少体积
	}

	var h slog.Handler
	if env == "prod" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	l := slog.New(h)
	slog.SetDefault(l)
	return l
}

// WithContext 把日志器放进 context，供下层函数取用。
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext 取出带请求上下文的日志器，没有则退回默认日志器。
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
