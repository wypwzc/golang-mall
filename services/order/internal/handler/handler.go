// Package handler 订单的 HTTP 层。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"go-mall/pkg/apperr"
	"go-mall/pkg/middleware"
	"go-mall/pkg/response"
	"go-mall/services/order/internal/service"
)

// Handler 订单处理器。
type Handler struct {
	svc *service.Service
}

// New 创建处理器。
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Create 下单。
func (h *Handler) Create(c *gin.Context) {
	var body struct {
		Items []service.ItemInput `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}

	order, err := h.svc.Create(c.Request.Context(), middleware.UserID(c), body.Items)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, order)
}

// List 我的订单。
func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	list, total, err := h.svc.List(c.Request.Context(), middleware.UserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, page, pageSize)
}

// Get 订单详情。
func (h *Handler) Get(c *gin.Context) {
	order, err := h.svc.Get(c.Request.Context(), middleware.UserID(c), c.Param("order_no"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, order)
}

// Pay 模拟支付。
func (h *Handler) Pay(c *gin.Context) {
	order, err := h.svc.Pay(c.Request.Context(), middleware.UserID(c), c.Param("order_no"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, order)
}

// Cancel 取消订单。
func (h *Handler) Cancel(c *gin.Context) {
	order, err := h.svc.Cancel(c.Request.Context(), middleware.UserID(c), c.Param("order_no"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, order)
}

// AdminList 管理端订单列表。
func (h *Handler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	list, total, err := h.svc.AdminList(c.Request.Context(), page, pageSize, c.Query("status"))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func bindErr(err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		f := ve[0]
		return apperr.InvalidParam.WithMessage("字段 %s 校验失败(%s)", f.Field(), f.Tag())
	}
	return apperr.InvalidParam.WithCause(err)
}
