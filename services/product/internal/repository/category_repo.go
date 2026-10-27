// Package repository 商品服务的数据访问层。
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"go-mall/pkg/apperr"
	"go-mall/services/product/internal/model"
)

// CategoryRepository 分类数据访问。
type CategoryRepository struct{ db *gorm.DB }

// NewCategoryRepository 创建仓库。
func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// Create 新建分类。
func (r *CategoryRepository) Create(ctx context.Context, c *model.Category) error {
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		if isDuplicateKey(err) {
			return apperr.Conflict.WithMessage("分类名称已存在").WithCause(err)
		}
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// ByID 按 ID 查询。
func (r *CategoryRepository) ByID(ctx context.Context, id int64) (*model.Category, error) {
	var c model.Category
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&c).Error; err != nil {
		return nil, wrapFindErr(err, "分类不存在")
	}
	return &c, nil
}

// List 全部分类，按 sort、id 排序。
func (r *CategoryRepository) List(ctx context.Context) ([]model.Category, error) {
	var list []model.Category
	if err := r.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, apperr.Internal.WithCause(err)
	}
	return list, nil
}

// Update 修改分类。
func (r *CategoryRepository) Update(ctx context.Context, id int64, name string, sort int) error {
	if _, err := r.ByID(ctx, id); err != nil {
		return err
	}
	err := r.db.WithContext(ctx).Model(&model.Category{}).
		Where("id = ?", id).
		Updates(map[string]any{"name": name, "sort": sort, "updated_at": time.Now().UTC()}).Error
	if err != nil {
		if isDuplicateKey(err) {
			return apperr.Conflict.WithMessage("分类名称已存在").WithCause(err)
		}
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// Delete 删除分类。分类下还有商品时拒绝，避免产生孤儿商品。
func (r *CategoryRepository) Delete(ctx context.Context, id int64) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Product{}).Where("category_id = ?", id).Count(&count).Error; err != nil {
		return apperr.Internal.WithCause(err)
	}
	if count > 0 {
		return apperr.Conflict.WithMessage("该分类下还有 %d 个商品，请先移动或删除", count)
	}

	res := r.db.WithContext(ctx).Delete(&model.Category{}, id)
	if res.Error != nil {
		return apperr.Internal.WithCause(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound.WithMessage("分类不存在")
	}
	return nil
}

// wrapFindErr 翻译 GORM 的未找到错误。
func wrapFindErr(err error, msg string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound.WithMessage("%s", msg)
	}
	return apperr.Internal.WithCause(err)
}

// isDuplicateKey 判断唯一键冲突。
func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return strings.Contains(err.Error(), "Duplicate entry")
}
