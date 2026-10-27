// Package router 集中声明本服务的路由与权限，一眼能看出哪些接口是公开的。
package router

import (
	"github.com/gin-gonic/gin"

	"go-mall/pkg/middleware"
	"go-mall/services/user/internal/handler"
	"go-mall/services/user/internal/model"
)

// Register 注册用户服务的全部路由。
func Register(r *gin.Engine, h *handler.Handler) {
	v1 := r.Group("/api/v1")

	// 公开接口
	auth := v1.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
	}

	// 登录用户
	me := v1.Group("/users", middleware.RequireLogin())
	{
		me.GET("/me", h.Me)
		me.PUT("/me", h.UpdateProfile)
		me.PUT("/me/password", h.ChangePassword)
	}

	// 管理员
	admin := v1.Group("/admin/users", middleware.RequireRole(model.RoleAdmin))
	{
		admin.GET("", h.AdminList)
		admin.PUT("/:id/status", h.AdminSetStatus)
		admin.PUT("/:id/role", h.AdminSetRole)
	}
}
