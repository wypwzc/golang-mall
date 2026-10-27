package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/services/product/internal/model"
)

// ProductRepository 商品数据访问。
type ProductRepository struct{ db *gorm.DB }

// NewProductRepository 创建仓库。
func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// ProductFilter 商品列表查询条件。
type ProductFilter struct {
	Page       int
	PageSize   int
	Keyword    string
	CategoryID int64
	MinPrice   *money.Cents
	MaxPrice   *money.Cents
	Status     *int8 // nil 表示不限制（管理端用），公开列表固定传 1
	Sort       string
}

// 排序白名单。绝不能把前端传来的字符串直接拼进 ORDER BY，那是 SQL 注入。
var sortableFields = map[string]string{
	"":           "id DESC",
	"newest":     "created_at DESC, id DESC",
	"price_asc":  "price ASC, id DESC",
	"price_desc": "price DESC, id DESC",
}

// Create 新建商品。
func (r *ProductRepository) Create(ctx context.Context, p *model.Product) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// ByID 按 ID 查询。
func (r *ProductRepository) ByID(ctx context.Context, id int64) (*model.Product, error) {
	var p model.Product
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&p).Error; err != nil {
		return nil, wrapFindErr(err, "商品不存在")
	}
	return &p, nil
}

// ByIDs 批量查询，用于下单时取商品快照。
func (r *ProductRepository) ByIDs(ctx context.Context, ids []int64) ([]model.Product, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []model.Product
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, apperr.Internal.WithCause(err)
	}
	return list, nil
}

// Update 更新商品。库存不在这里改，必须走扣减/恢复接口，避免绕过并发控制。
func (r *ProductRepository) Update(ctx context.Context, p *model.Product) error {
	if _, err := r.ByID(ctx, p.ID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ?", p.ID).
		Updates(map[string]any{
			"category_id": p.CategoryID,
			"name":        p.Name,
			"description": p.Description,
			"price":       p.Price,
			"image_url":   p.ImageURL,
			"updated_at":  time.Now().UTC(),
		}).Error
}

// UpdateStatus 上架/下架。
func (r *ProductRepository) UpdateStatus(ctx context.Context, id int64, status int8) error {
	if _, err := r.ByID(ctx, id); err != nil {
		return err
	}
	err := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": time.Now().UTC()}).Error
	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// Delete 删除商品。有库存扣减记录的商品不允许物理删除，避免破坏历史订单的库存依据。
func (r *ProductRepository) Delete(ctx context.Context, id int64) error {
	var used int64
	if err := r.db.WithContext(ctx).Model(&model.StockDeduction{}).Where("product_id = ?", id).Count(&used).Error; err != nil {
		return apperr.Internal.WithCause(err)
	}
	if used > 0 {
		return apperr.Conflict.WithMessage("该商品已有订单记录，请改为下架")
	}

	res := r.db.WithContext(ctx).Delete(&model.Product{}, id)
	if res.Error != nil {
		return apperr.Internal.WithCause(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound.WithMessage("商品不存在")
	}
	return nil
}

// List 分页查询。
func (r *ProductRepository) List(ctx context.Context, f ProductFilter) ([]model.Product, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Product{})

	if f.Keyword != "" {
		q = q.Where("name LIKE ?", "%"+f.Keyword+"%")
	}
	if f.CategoryID > 0 {
		q = q.Where("category_id = ?", f.CategoryID)
	}
	if f.MinPrice != nil {
		q = q.Where("price >= ?", *f.MinPrice)
	}
	if f.MaxPrice != nil {
		q = q.Where("price <= ?", *f.MaxPrice)
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}

	order, ok := sortableFields[f.Sort]
	if !ok {
		order = sortableFields[""]
	}

	var list []model.Product
	err := q.Order(order).
		Offset((f.Page - 1) * f.PageSize).
		Limit(f.PageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}
	return list, total, nil
}

// UpdateStock 直接设置库存。仅供管理端补货使用，购买流程必须走 StockRepository。
func (r *ProductRepository) UpdateStock(ctx context.Context, id int64, stock int) error {
	if _, err := r.ByID(ctx, id); err != nil {
		return err
	}
	err := r.db.WithContext(ctx).Model(&model.Product{}).
		Where("id = ?", id).
		Updates(map[string]any{"stock": stock, "updated_at": time.Now().UTC()}).Error
	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}
