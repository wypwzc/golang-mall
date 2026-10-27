package repository

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-mall/pkg/apperr"
	"go-mall/services/product/internal/model"
)

// StockItem 一次扣减的商品与数量。
type StockItem struct {
	ProductID int64
	Quantity  int
}

// StockRepository 库存操作。这里是整个系统唯一允许修改 products.stock 的地方。
type StockRepository struct{ db *gorm.DB }

// NewStockRepository 创建仓库。
func NewStockRepository(db *gorm.DB) *StockRepository {
	return &StockRepository{db: db}
}

// Deduct 批量扣减库存，返回扣减后的商品快照（含服务端价格，供下单使用）。
//
// 关键点：
//  1. 扣减用单条 "UPDATE ... WHERE stock >= ?"，由 InnoDB 行锁保证原子性，
//     并发请求不会读到同一个旧库存，因此不会超卖；
//  2. 整个操作在一个事务里，任一条失败全部回滚，不会出现"扣了一半"；
//  3. order_no + product_id 唯一索引 + 前置查询，保证重复调用不会重复扣减。
func (r *StockRepository) Deduct(ctx context.Context, orderNo string, items []StockItem) ([]model.Product, error) {
	if orderNo == "" || len(items) == 0 {
		return nil, apperr.InvalidParam.WithMessage("扣减参数不完整")
	}

	var snapshot []model.Product

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 幂等检查：同一订单号已经扣过就直接返回，不再动库存
		var existing []model.StockDeduction
		if err := tx.Where("order_no = ?", orderNo).Find(&existing).Error; err != nil {
			return apperr.Internal.WithCause(err)
		}
		if len(existing) > 0 {
			for _, e := range existing {
				if e.Status == model.DeductionRestored {
					return apperr.Conflict.WithMessage("订单 %s 的库存已恢复，不能重复扣减", orderNo)
				}
			}
			if len(existing) != len(items) {
				return apperr.Conflict.WithMessage("订单 %s 的扣减明细与本次不一致", orderNo)
			}
			return tx.Where("id IN ?", productIDs(items)).Find(&snapshot).Error
		}

		// 逐条条件扣减
		for _, it := range items {
			if it.Quantity <= 0 {
				return apperr.InvalidParam.WithMessage("购买数量必须大于 0")
			}

			res := tx.Exec(
				`UPDATE products SET stock = stock - ?, updated_at = UTC_TIMESTAMP(3)
				 WHERE id = ? AND status = ? AND stock >= ?`,
				it.Quantity, it.ProductID, model.StatusOnSale, it.Quantity,
			)
			if res.Error != nil {
				return apperr.Internal.WithCause(res.Error)
			}
			if res.RowsAffected == 0 {
				// 没更新到，说明商品不存在、已下架或库存不足，查清楚给前端准确提示
				var p model.Product
				if err := tx.Where("id = ?", it.ProductID).First(&p).Error; err != nil {
					return apperr.NotFound.WithMessage("商品 %d 不存在", it.ProductID)
				}
				if p.Status != model.StatusOnSale {
					return apperr.Conflict.WithMessage("商品「%s」已下架", p.Name)
				}
				return apperr.StockNotEnough.WithMessage("商品「%s」库存不足，仅剩 %d 件", p.Name, p.Stock)
			}
		}

		// 写扣减流水，作为幂等与恢复的依据
		for _, it := range items {
			d := model.StockDeduction{
				OrderNo:   orderNo,
				ProductID: it.ProductID,
				Quantity:  it.Quantity,
				Status:    model.DeductionDeducted,
			}
			if err := tx.Create(&d).Error; err != nil {
				return apperr.Internal.WithCause(err)
			}
		}

		return tx.Where("id IN ?", productIDs(items)).Find(&snapshot).Error
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// Restore 按订单号恢复库存（取消订单时调用）。
// 通过行锁 + 状态字段保证同一订单只恢复一次。
func (r *StockRepository) Restore(ctx context.Context, orderNo string) error {
	if orderNo == "" {
		return apperr.InvalidParam.WithMessage("订单号不能为空")
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []model.StockDeduction
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_no = ?", orderNo).
			Find(&rows).Error
		if err != nil {
			return apperr.Internal.WithCause(err)
		}
		if len(rows) == 0 {
			return apperr.NotFound.WithMessage("订单 %s 没有库存扣减记录", orderNo)
		}

		for _, row := range rows {
			if row.Status == model.DeductionRestored {
				continue // 已经恢复过，跳过（幂等）
			}

			if err := tx.Exec(
				`UPDATE products SET stock = stock + ?, updated_at = UTC_TIMESTAMP(3) WHERE id = ?`,
				row.Quantity, row.ProductID,
			).Error; err != nil {
				return apperr.Internal.WithCause(err)
			}

			if err := tx.Model(&model.StockDeduction{}).
				Where("id = ?", row.ID).
				Update("status", model.DeductionRestored).Error; err != nil {
				return apperr.Internal.WithCause(err)
			}
		}
		return nil
	})
}

// DeductionStatus 查询某订单的扣减状态，供排查与对账使用。
func (r *StockRepository) DeductionStatus(ctx context.Context, orderNo string) ([]model.StockDeduction, error) {
	var rows []model.StockDeduction
	if err := r.db.WithContext(ctx).Where("order_no = ?", orderNo).Find(&rows).Error; err != nil {
		return nil, apperr.Internal.WithCause(err)
	}
	return rows, nil
}

func productIDs(items []StockItem) []int64 {
	ids := make([]int64, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ProductID)
	}
	return ids
}
