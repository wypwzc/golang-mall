package middleware

import (
	"crypto/subtle"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/response"
)

// HeaderInternalToken 服务间内部接口的共享令牌头。
const HeaderInternalToken = "X-Internal-Token"

// RequireInternalToken 保护 /internal/** 接口。
// 业务服务的端口不对外暴露，这一层是"防止内网误调用"的第二道锁。
func RequireInternalToken(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := c.GetHeader(HeaderInternalToken)
		// 常量时间比较，避免时序攻击
		if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			response.Error(c, apperr.Forbidden.WithMessage("内部接口凭证无效"))
			return
		}
		c.Next()
	}
}
