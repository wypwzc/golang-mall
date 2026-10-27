// Package router 商品服务路由。
package router

import (
	"github.com/gin-gonic/gin"

	"go-mall/pkg/middleware"
	"go-mall/services/product/internal/handler"
)

// Register 注册路由。
func Register(r *gin.Engine, h *handler.Handler, internalToken string) {
	v1 := r.Group("/api/v1")

	// 公开：浏览与搜索
	v1.GET("/categories", h.ListCategories)
	v1.GET("/products", h.List)
	v1.GET("/products/:id", h.Get)

	// 管理端
	admin := v1.Group("/admin", middleware.RequireRole(middleware.RoleAdmin))
	{
		admin.POST("/categories", h.CreateCategory)
		admin.PUT("/categories/:id", h.UpdateCategory)
		admin.DELETE("/categories/:id", h.DeleteCategory)

		admin.GET("/products", h.AdminList)
		admin.GET("/products/:id", h.AdminGet)
		admin.POST("/products", h.Create)
		admin.PUT("/products/:id", h.Update)
		admin.PUT("/products/:id/status", h.SetStatus)
		admin.DELETE("/products/:id", h.Delete)

		// 图片上传单独一个前缀：gin 的路由树里 /products/:id 和 /products/upload 会冲突
		admin.POST("/uploads", h.Upload)
	}

	// 内部接口：只允许带内部令牌的服务调用，不经过网关
	internal := r.Group("/internal/products", middleware.RequireInternalToken(internalToken))
	{
		internal.POST("/batch", h.BatchGet)
		internal.POST("/stock/deduct", h.Deduct)
		internal.POST("/stock/restore", h.Restore)
	}
}
