// Package middleware 存放各服务共用的 Gin 中间件。
package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/logger"
	"go-mall/pkg/requestid"
)

// RequestID 为每个请求分配唯一 ID，并放进日志上下文与响应头。
// 网关会先生成并透传，业务服务优先复用网关传来的值。
func RequestID(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestid.Header)
		if id == "" {
			id = requestid.New()
		}
		c.Set(requestid.ContextKey, id)
		c.Request.Header.Set(requestid.Header, id)
		c.Header(requestid.Header, id)

		// 把带 request_id 的日志器塞进 request context，下层可直接用
		l := log.With("request_id", id)
		ctx := logger.WithContext(c.Request.Context(), l)
		ctx = requestid.WithContext(ctx, id) // 服务间调用时透传
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
