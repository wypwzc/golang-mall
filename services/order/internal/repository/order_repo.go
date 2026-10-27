// Package repository 订单的数据访问。
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-mall/pkg/apperr"
	"go-mall/services/order/internal/model"
)

// OrderRepository 订单仓库。
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository 创建仓库。
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create 在一个事务里写入订单主表和明细，任何一步失败都整体回滚。
func (r *OrderRepository) Create(ctx context.Context, order *model.Order) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 必须 Omit(clause.Associations)：否则 GORM 会在插主表时级联插入明细，
		// 把明细的 ID 提前占掉，下面再插一次就会主键冲突。
		if err := tx.Omit(clause.Associations).Create(order).Error; err != nil {
			return err
		}
		// 明细需要订单 ID，所以先插主表再插明细
		for i := range order.Items {
			order.Items[i].OrderID = order.ID
		}
		if len(order.Items) > 0 {
			if err := tx.Create(&order.Items).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	return nil
}

// ByOrderNo 按订单号查询，带上明细。
func (r *OrderRepository) ByOrderNo(ctx context.Context, orderNo string) (*model.Order, error) {
	var order model.Order
	err := r.db.WithContext(ctx).
		Preload("Items").
		Where("order_no = ?", orderNo).
		First(&order).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound.WithMessage("订单不存在")
	}
	if err != nil {
		return nil, apperr.Internal.WithCause(err)
	}
	return &order, nil
}

// ListByUser 查询某个用户的订单。
func (r *OrderRepository) ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{}).Where("user_id = ?", userID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}

	var list []model.Order
	err := q.Preload("Items").
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}
	return list, total, nil
}

// ListAll 管理端查询全部订单。
func (r *OrderRepository) ListAll(ctx context.Context, page, pageSize int, status string) ([]model.Order, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Order{})
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}

	var list []model.Order
	err := q.Preload("Items").
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, apperr.Internal.WithCause(err)
	}
	return list, total, nil
}

// UpdateStatus 条件更新状态。
// 只有当前状态等于 from 时才会更新成功，用它来保证"已支付的订单不能被取消"这类规则。
// 返回 false 表示状态不匹配（订单不存在或已被别人改过）。
func (r *OrderRepository) UpdateStatus(ctx context.Context, orderNo string, userID int64, from, to string) (bool, error) {
	now := time.Now().UTC()
	fields := map[string]any{"status": to, "updated_at": now}
	switch to {
	case model.StatusPaid:
		fields["paid_at"] = now
	case model.StatusCancelled:
		fields["cancelled_at"] = now
	}

	q := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("order_no = ? AND status = ?", orderNo, from)
	// userID 为 0 表示管理端操作，不限制归属
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}

	res := q.Updates(fields)
	if res.Error != nil {
		return false, apperr.Internal.WithCause(res.Error)
	}
	return res.RowsAffected > 0, nil
}

// AdminUpdateStatus 管理端直接改状态（只允许在合法状态之间切换，由 service 校验）。
func (r *OrderRepository) AdminUpdateStatus(ctx context.Context, orderNo, from, to string) (bool, error) {
	return r.UpdateStatus(ctx, orderNo, 0, from, to)
}
