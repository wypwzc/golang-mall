// Package model 订单的数据结构。
package model

import (
	"time"

	"go-mall/pkg/money"
)

// 订单状态。只有这三种，状态之间的转换在 service 里用条件更新控制。
const (
	StatusPending   = "pending"   // 待支付
	StatusPaid      = "paid"      // 已支付
	StatusCancelled = "cancelled" // 已取消
)

// Order 订单主表。
type Order struct {
	ID          int64       `gorm:"column:id;primaryKey;autoIncrement"`
	OrderNo     string      `gorm:"column:order_no;size:32"`
	UserID      int64       `gorm:"column:user_id"`
	TotalAmount money.Cents `gorm:"column:total_amount"`
	Status      string      `gorm:"column:status;size:16"`
	PaidAt      *time.Time  `gorm:"column:paid_at"`
	CancelledAt *time.Time  `gorm:"column:cancelled_at"`
	CreatedAt   time.Time   `gorm:"column:created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at"`

	Items []OrderItem `gorm:"foreignKey:OrderID"`
}

// TableName 表名。
func (Order) TableName() string { return "orders" }

// OrderItem 订单明细，保存下单时的商品快照。
type OrderItem struct {
	ID           int64       `gorm:"column:id;primaryKey;autoIncrement"`
	OrderID      int64       `gorm:"column:order_id"`
	ProductID    int64       `gorm:"column:product_id"`
	ProductName  string      `gorm:"column:product_name;size:128"`
	ProductImage string      `gorm:"column:product_image;size:255"`
	Price        money.Cents `gorm:"column:price"`
	Quantity     int         `gorm:"column:quantity"`
	Subtotal     money.Cents `gorm:"column:subtotal"`
}

// TableName 表名。
func (OrderItem) TableName() string { return "order_items" }

// Public 返回给前端的订单结构。
type Public struct {
	OrderNo     string       `json:"order_no"`
	TotalAmount money.Cents  `json:"total_amount"`
	Status      string       `json:"status"`
	PaidAt      *time.Time   `json:"paid_at"`
	CancelledAt *time.Time   `json:"cancelled_at"`
	CreatedAt   time.Time    `json:"created_at"`
	Items       []PublicItem `json:"items"`
}

// PublicItem 返回给前端的明细结构。
type PublicItem struct {
	ProductID    int64       `json:"product_id"`
	ProductName  string      `json:"product_name"`
	ProductImage string      `json:"product_image"`
	Price        money.Cents `json:"price"`
	Quantity     int         `json:"quantity"`
	Subtotal     money.Cents `json:"subtotal"`
}

// ToPublic 转换成对外结构。
func (o *Order) ToPublic() Public {
	items := make([]PublicItem, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, PublicItem{
			ProductID:    it.ProductID,
			ProductName:  it.ProductName,
			ProductImage: it.ProductImage,
			Price:        it.Price,
			Quantity:     it.Quantity,
			Subtotal:     it.Subtotal,
		})
	}
	return Public{
		OrderNo:     o.OrderNo,
		TotalAmount: o.TotalAmount,
		Status:      o.Status,
		PaidAt:      o.PaidAt,
		CancelledAt: o.CancelledAt,
		CreatedAt:   o.CreatedAt,
		Items:       items,
	}
}

// ToPublicList 批量转换。
func ToPublicList(orders []Order) []Public {
	out := make([]Public, 0, len(orders))
	for i := range orders {
		out = append(out, orders[i].ToPublic())
	}
	return out
}
