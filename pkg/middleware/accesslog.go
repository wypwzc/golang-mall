package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/requestid"
)

// AccessLog 记录访问日志。健康检查与指标接口不打日志，避免刷屏。
func AccessLog(log *slog.Logger) gin.HandlerFunc {
	skip := map[string]bool{
		"/healthz": true,
		"/readyz":  true,
		"/metrics": true,
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if skip[c.Request.URL.Path] {
			return
		}

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"request_id", c.GetString(requestid.ContextKey),
		}
		// 只记录用户 ID，绝不记录密码、完整 JWT 等敏感信息
		if uid := UserID(c); uid > 0 {
			attrs = append(attrs, "user_id", uid)
		}

		switch {
		case status >= 500:
			log.Error("请求失败", attrs...)
		case status >= 400:
			log.Warn("请求异常", attrs...)
		default:
			log.Info("请求完成", attrs...)
		}
	}
}
