package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/response"
)

// Recovery 捕获 panic，返回统一的 50001 错误，并把堆栈写进日志。
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("请求处理发生 panic",
					"panic", rec,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"stack", string(debug.Stack()),
				)
				response.Error(c, apperr.Internal)
			}
		}()
		c.Next()
	}
}
