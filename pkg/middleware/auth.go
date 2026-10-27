package middleware

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/jwt"
	"go-mall/pkg/response"
)

const (
	// HeaderUserID 网关验签后注入的用户 ID。
	HeaderUserID = "X-User-Id"
	// HeaderUserRole 网关验签后注入的角色。
	HeaderUserRole = "X-User-Role"

	ctxUserID   = "auth_user_id"
	ctxUserRole = "auth_user_role"

	// RoleAdmin 管理员角色。
	RoleAdmin = "admin"
	// RoleUser 普通用户角色。
	RoleUser = "user"
)

// Auth 解析请求身份，写入 gin.Context。
//
// 两条路径：
//  1. 生产环境：网关验签后注入 X-User-Id / X-User-Role，服务直接信任（端口不对外暴露）；
//  2. 本地直连：没有网关时，退化为自己校验 Authorization: Bearer <JWT>。
//
// 本中间件不拦截请求，是否必须登录由 RequireLogin / RequireRole 决定。
func Auth(tokens *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 路径一：网关注入的身份头
		if raw := c.GetHeader(HeaderUserID); raw != "" {
			if uid, err := strconv.ParseInt(raw, 10, 64); err == nil && uid > 0 {
				c.Set(ctxUserID, uid)
				c.Set(ctxUserRole, c.GetHeader(HeaderUserRole))
				c.Next()
				return
			}
		}

		// 路径二：直连时自己验 JWT
		if tokens != nil {
			if token := bearerToken(c); token != "" {
				claims, err := tokens.Parse(token)
				if err == nil {
					c.Set(ctxUserID, claims.UserID)
					c.Set(ctxUserRole, claims.Role)
				}
			}
		}
		c.Next()
	}
}

// bearerToken 从 Authorization 头里取出 token，格式必须是 "Bearer xxx"。
func bearerToken(c *gin.Context) string {
	raw := c.GetHeader("Authorization")
	if raw == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(raw) <= len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(raw[len(prefix):])
}

// RequireLogin 要求已登录。
func RequireLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if UserID(c) <= 0 {
			response.Error(c, apperr.Unauthorized)
			return
		}
		c.Next()
	}
}

// RequireRole 要求指定角色。管理端接口必须挂这个中间件。
// 注意：前端路由守卫只是体验优化，真正的权限判断必须在后端。
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if UserID(c) <= 0 {
			response.Error(c, apperr.Unauthorized)
			return
		}
		if Role(c) != role {
			response.Error(c, apperr.Forbidden)
			return
		}
		c.Next()
	}
}

// UserID 取当前登录用户 ID，未登录返回 0。
func UserID(c *gin.Context) int64 {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return 0
	}
	id, _ := v.(int64)
	return id
}

// Role 取当前用户角色，未登录返回空字符串。
func Role(c *gin.Context) string {
	return c.GetString(ctxUserRole)
}

// IsAdmin 当前用户是否管理员。
func IsAdmin(c *gin.Context) bool { return Role(c) == RoleAdmin }
