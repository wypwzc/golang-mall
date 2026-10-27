// Package handler 商品服务的 HTTP 层。
package handler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/pkg/response"
	"go-mall/services/product/internal/service"
)

// Handler 商品服务处理器。
type Handler struct {
	svc *service.Service
}

// New 创建处理器。
func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// ---------- 分类 ----------

// ListCategories 分类列表。
func (h *Handler) ListCategories(c *gin.Context) {
	list, err := h.svc.ListCategories(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// CreateCategory 新建分类（管理端）。
func (h *Handler) CreateCategory(c *gin.Context) {
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	cat, err := h.svc.CreateCategory(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, cat)
}

// UpdateCategory 修改分类（管理端）。
func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.CategoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	if err := h.svc.UpdateCategory(c.Request.Context(), id, in); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已更新"})
}

// DeleteCategory 删除分类（管理端）。
func (h *Handler) DeleteCategory(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.svc.DeleteCategory(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已删除"})
}

// ---------- 商品 ----------

// List 公开商品列表，只含上架商品。
func (h *Handler) List(c *gin.Context) {
	in, err := parseListInput(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	list, total, err := h.svc.List(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, normalizePage(in.Page), normalizePageSize(in.PageSize))
}

// Get 公开商品详情。
func (h *Handler) Get(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	p, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// AdminList 管理端商品列表，包含下架商品。
func (h *Handler) AdminList(c *gin.Context) {
	in, err := parseListInput(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	var status *int8
	if raw := c.Query("status"); raw != "" {
		v, convErr := strconv.Atoi(raw)
		if convErr != nil || (v != 0 && v != 1) {
			response.Error(c, apperr.InvalidParam.WithMessage("status 只能是 0 或 1"))
			return
		}
		s := int8(v)
		status = &s
	}

	list, total, err := h.svc.AdminList(c.Request.Context(), in, status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, normalizePage(in.Page), normalizePageSize(in.PageSize))
}

// AdminGet 管理端详情。
func (h *Handler) AdminGet(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	p, err := h.svc.AdminGet(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// Upload 上传商品图片。
//
// 演示项目直接把文件存到本地 uploads/ 目录，返回一个相对 URL。
// 生产环境应换成对象存储（OSS/S3），并做图片压缩、病毒扫描。
func (h *Handler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperr.InvalidParam.WithMessage("请选择要上传的图片"))
		return
	}
	if file.Size > 2<<20 {
		response.Error(c, apperr.InvalidParam.WithMessage("图片不能超过 2MB"))
		return
	}

	// 只按扩展名放行常见图片格式，避免上传可执行文件
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		response.Error(c, apperr.InvalidParam.WithMessage("只支持 jpg / png / webp / gif"))
		return
	}

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		response.Error(c, apperr.Internal.WithCause(err))
		return
	}

	// 用纳秒时间戳做文件名，避免同名覆盖
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	if err := c.SaveUploadedFile(file, filepath.Join(uploadDir, name)); err != nil {
		response.Error(c, apperr.Internal.WithCause(err))
		return
	}

	response.OK(c, gin.H{"url": "/uploads/" + name})
}

// Create 新建商品（管理端）。
func (h *Handler) Create(c *gin.Context) {
	var in service.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	p, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, p)
}

// Update 修改商品（管理端）。
func (h *Handler) Update(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	var in service.ProductInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	p, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// SetStatus 上下架（管理端）。
func (h *Handler) SetStatus(c *gin.Context) {
	id, err := pathID(c)
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
	if err := h.svc.SetStatus(c.Request.Context(), id, *body.Status); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "状态已更新"})
}

// Delete 删除商品（管理端）。
func (h *Handler) Delete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "已删除"})
}

// ---------- 内部接口 ----------

// BatchGet 批量取商品快照。
func (h *Handler) BatchGet(c *gin.Context) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	list, err := h.svc.BatchGet(c.Request.Context(), body.IDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// Deduct 扣减库存。
func (h *Handler) Deduct(c *gin.Context) {
	var body struct {
		OrderNo string                   `json:"order_no"`
		Items   []service.StockItemInput `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	list, err := h.svc.Deduct(c.Request.Context(), body.OrderNo, body.Items)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// Restore 恢复库存。
func (h *Handler) Restore(c *gin.Context) {
	var body struct {
		OrderNo string `json:"order_no"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, bindErr(err))
		return
	}
	if err := h.svc.Restore(c.Request.Context(), body.OrderNo); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"message": "库存已恢复"})
}

// ---------- 工具 ----------

func pathID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.InvalidParam.WithMessage("ID 不合法")
	}
	return id, nil
}

func parseListInput(c *gin.Context) (service.ListInput, error) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	categoryID, _ := strconv.ParseInt(c.DefaultQuery("category_id", "0"), 10, 64)

	in := service.ListInput{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    c.Query("keyword"),
		CategoryID: categoryID,
		Sort:       c.Query("sort"),
	}

	if raw := c.Query("min_price"); raw != "" {
		v, err := money.FromYuan(raw)
		if err != nil {
			return in, apperr.InvalidParam.WithMessage("min_price 格式不正确")
		}
		in.MinPrice = &v
	}
	if raw := c.Query("max_price"); raw != "" {
		v, err := money.FromYuan(raw)
		if err != nil {
			return in, apperr.InvalidParam.WithMessage("max_price 格式不正确")
		}
		in.MaxPrice = &v
	}
	if in.MinPrice != nil && in.MaxPrice != nil && *in.MinPrice > *in.MaxPrice {
		return in, apperr.InvalidParam.WithMessage("min_price 不能大于 max_price")
	}
	return in, nil
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizePageSize(size int) int {
	if size < 1 {
		return 10
	}
	if size > 100 {
		return 100
	}
	return size
}

func bindErr(err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) && len(ve) > 0 {
		f := ve[0]
		return apperr.InvalidParam.WithMessage("字段 %s 校验失败(%s)", f.Field(), f.Tag())
	}
	return apperr.InvalidParam.WithCause(err)
}

// uploadDir 图片存放目录，相对于服务启动时的工作目录。
const uploadDir = "uploads"
