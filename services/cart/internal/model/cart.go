// Package model 购物车的数据结构与对外返回格式。
package model

import (
	"time"

	"go-mall/pkg/money"
)

// CartItem 对应 cart_items 表。
// (user_id, product_id) 唯一，同一个商品在购物车里只有一行，重复加购只累加数量。
type CartItem struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id"`
	ProductID int64     `gorm:"column:product_id"`
	Quantity  int       `gorm:"column:quantity"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName 显式指定表名。
func (CartItem) TableName() string { return "cart_items" }

// Item 返回给前端的购物车条目。
// 名称、价格、库存都来自商品服务，购物车表里不存这些字段，避免价格过期。
type Item struct {
	ID        int64       `json:"id"`
	ProductID int64       `json:"product_id"`
	Name      string      `json:"name"`
	ImageURL  string      `json:"image_url"`
	Price     money.Cents `json:"price"`
	Stock     int         `json:"stock"`
	Quantity  int         `json:"quantity"`
	Subtotal  money.Cents `json:"subtotal"`
	// Available 商品是否还能买（存在、已上架、库存够）
	Available bool `json:"available"`
}

// View 整个购物车。
type View struct {
	Items []Item      `json:"items"`
	Total money.Cents `json:"total"`
	Count int         `json:"count"`
}
