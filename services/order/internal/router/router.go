// Package router 订单路由。
package router

import (
	"github.com/gin-gonic/gin"

	"go-mall/pkg/middleware"
	"go-mall/services/order/internal/handler"
)

// Register 注册路由。
func Register(r *gin.Engine, h *handler.Handler) {
	v1 := r.Group("/api/v1")

	orders := v1.Group("/orders", middleware.RequireLogin())
	{
		orders.POST("", h.Create)
		orders.GET("", h.List)
		orders.GET("/:order_no", h.Get)
		orders.POST("/:order_no/pay", h.Pay)
		orders.POST("/:order_no/cancel", h.Cancel)
	}

	admin := v1.Group("/admin/orders", middleware.RequireRole(middleware.RoleAdmin))
	{
		admin.GET("", h.AdminList)
	}
}
