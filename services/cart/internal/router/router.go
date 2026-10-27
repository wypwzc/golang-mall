// Package router 购物车路由。所有接口都要求登录。
package router

import (
	"github.com/gin-gonic/gin"

	"go-mall/pkg/middleware"
	"go-mall/services/cart/internal/handler"
)

// Register 注册路由。
func Register(r *gin.Engine, h *handler.Handler) {
	cart := r.Group("/api/v1/cart", middleware.RequireLogin())
	{
		cart.GET("", h.View)
		cart.DELETE("", h.Clear)
		cart.POST("/items", h.Add)
		cart.PUT("/items/:product_id", h.SetQuantity)
		cart.DELETE("/items/:product_id", h.Remove)
	}
}
