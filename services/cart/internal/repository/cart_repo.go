// Package repository 购物车的数据访问。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-mall/pkg/apperr"
	"go-mall/services/cart/internal/model"
)

// CartRepository 购物车仓库。
type CartRepository struct {
	db *gorm.DB
}

// NewCartRepository 创建仓库。
func NewCartRepository(db *gorm.DB) *CartRepository {
	return &CartRepository{db: db}
}

// List 查询某个用户的全部购物车条目，按加入时间正序。
func (r *CartRepository) List(ctx context.Context, userID int64) ([]model.CartItem, error) {
	var items []model.CartItem
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id ASC").
		Find(&items).Error
	if err != nil {
		return nil, apperr.Internal.WithCause(err)
	}
	return items, nil
}

// Get 查询单个条目，不存在时返回 nil（不当作错误，调用方按"还没加购"处理）。
func (r *CartRepository) Get(ctx context.Context, userID, productID int64) (*model.CartItem, error) {
	var item model.CartItem
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, apperr.Internal.WithCause(err)
	}
	return &item, nil
}

// Add 加购。已存在则累加数量，靠唯一索引 + ON DUPLICATE KEY UPDATE 一条语句完成。
func (r *CartRepository) Add(ctx context.Context, userID, productID int64, quantity int) error {
	item := model.CartItem{
		UserID:    userID,
		ProductID: productID,
		Quantity:  quantity,
	}

	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "product_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"quantity":   gorm.Expr("quantity + ?", quantity),
			"updated_at": time.Now().UTC(),
		}),
	}).Create(&item).Error

	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// SetQuantity 直接改数量。
func (r *CartRepository) SetQuantity(ctx context.Context, userID, productID int64, quantity int) error {
	res := r.db.WithContext(ctx).Model(&model.CartItem{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Updates(map[string]any{"quantity": quantity, "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return apperr.Internal.WithCause(res.Error)
	}
	if res.RowsAffected == 0 {
		// 数量没变化时 RowsAffected 也是 0，这里再确认一次记录是否存在
		item, err := r.Get(ctx, userID, productID)
		if err != nil {
			return err
		}
		if item == nil {
			return apperr.NotFound.WithMessage("购物车里没有这个商品")
		}
	}
	return nil
}

// Delete 删除一个商品。
func (r *CartRepository) Delete(ctx context.Context, userID, productID int64) error {
	res := r.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&model.CartItem{})
	if res.Error != nil {
		return apperr.Internal.WithCause(res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound.WithMessage("购物车里没有这个商品")
	}
	return nil
}

// Clear 清空购物车。
func (r *CartRepository) Clear(ctx context.Context, userID int64) error {
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&model.CartItem{}).Error
	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}
