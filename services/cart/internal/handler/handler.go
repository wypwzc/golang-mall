// Package handler 购物车的 HTTP 层。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"go-mall/pkg/apperr"
	"go-mall/pkg/middleware"
	"go-mall/pkg/response"
	"go-mall/services/cart/internal/service"
)

// Handler 购物车处理器。
type Handler struct {
	svc *service.Service
}

// New 创建处理器。
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// View 查询购物车。
func (h *Handler) View(c *gin.Context) {
	view, err := h.svc.View(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, view)
}

// Add 加入购物车。
func (h *Handler) Add(c *gin.Context) {
	var body struct {
		ProductID int64 `json:"product_id" binding:"required"`
		Quantity  int   `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}

	if err := h.svc.Add(c.Request.Context(), middleware.UserID(c), body.ProductID, body.Quantity); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已加入购物车"})
}

// SetQuantity 修改数量。
func (h *Handler) SetQuantity(c *gin.Context) {
	productID, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	var body struct {
		Quantity *int `json:"quantity" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}

	if err := h.svc.SetQuantity(c.Request.Context(), middleware.UserID(c), productID, *body.Quantity); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已更新"})
}

// Remove 删除购物车里的某个商品。
func (h *Handler) Remove(c *gin.Context) {
	productID, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.svc.Remove(c.Request.Context(), middleware.UserID(c), productID); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已删除"})
}

// Clear 清空购物车。
func (h *Handler) Clear(c *gin.Context) {
	if err := h.svc.Clear(c.Request.Context(), middleware.UserID(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已清空"})
}

func pathID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("product_id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.InvalidParam.WithMessage("商品 ID 不合法")
	}
	return id, nil
}

func bindErr(err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		f := ve[0]
		return apperr.InvalidParam.WithMessage("字段 %s 校验失败(%s)", f.Field(), f.Tag())
	}
	return apperr.InvalidParam.WithCause(err)
}
