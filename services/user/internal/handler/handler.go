// Package handler HTTP 处理层：只负责参数绑定、调用 service、返回统一响应。
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"go-mall/pkg/apperr"
	"go-mall/pkg/middleware"
	"go-mall/pkg/response"
	"go-mall/services/user/internal/service"
)

// Handler 汇总本服务的所有 HTTP 处理函数。
type Handler struct {
	svc *service.Service
}

// New 创建 Handler。
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// ---------- 认证 ----------

// Register 注册。
func (h *Handler) Register(c *gin.Context) {
	var in service.RegisterInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	user, err := h.svc.Register(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, user)
}

// Login 登录，返回 JWT。
func (h *Handler) Login(c *gin.Context) {
	var in service.LoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	result, err := h.svc.Login(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

// ---------- 当前用户 ----------

// Me 查询当前登录用户。
func (h *Handler) Me(c *gin.Context) {
	user, err := h.svc.Me(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// UpdateProfile 修改昵称与邮箱。
func (h *Handler) UpdateProfile(c *gin.Context) {
	var in service.UpdateProfileInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	user, err := h.svc.UpdateProfile(c.Request.Context(), middleware.UserID(c), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// ChangePassword 修改密码。
func (h *Handler) ChangePassword(c *gin.Context) {
	var in service.ChangePasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), middleware.UserID(c), in); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "密码修改成功"})
}

// ---------- 管理端 ----------

// AdminList 用户分页查询。
func (h *Handler) AdminList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	var status *int8
	if raw := c.Query("status"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || (v != 0 && v != 1) {
			response.Error(c, apperr.InvalidParam.WithMessage("status 只能是 0 或 1"))
			return
		}
		s := int8(v)
		status = &s
	}

	users, total, err := h.svc.AdminList(c.Request.Context(), page, pageSize, c.Query("keyword"), c.Query("role"), status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, users, total, page, pageSize)
}

// AdminSetStatus 启用/禁用账号。
func (h *Handler) AdminSetStatus(c *gin.Context) {
	targetID, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	var body struct {
		Status *int8 `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}

	if err := h.svc.AdminSetStatus(c.Request.Context(), middleware.UserID(c), targetID, *body.Status); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "状态已更新"})
}

// AdminSetRole 修改角色。
func (h *Handler) AdminSetRole(c *gin.Context) {
	targetID, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	var body struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}

	if err := h.svc.AdminSetRole(c.Request.Context(), middleware.UserID(c), targetID, body.Role); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "角色已更新"})
}

// ---------- 工具 ----------

func pathID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.InvalidParam.WithMessage("用户 ID 不合法")
	}
	return id, nil
}

// bindErr 把 validator 的字段错误翻译成可读的业务错误。
func bindErr(err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		f := ve[0]
		switch f.Tag() {
		case "required":
			return apperr.InvalidParam.WithMessage("缺少必填字段: %s", f.Field())
		default:
			return apperr.InvalidParam.WithMessage("字段 %s 校验失败(%s)", f.Field(), f.Tag())
		}
	}
	return apperr.InvalidParam.WithCause(err)
}
