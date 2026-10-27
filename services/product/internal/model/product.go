// Package model 商品服务的持久化模型。
package model

import (
	"time"

	"go-mall/pkg/money"
)

// 商品上下架状态
const (
	StatusOffShelf int8 = 0
	StatusOnSale   int8 = 1
)

// 库存扣减流水状态
const (
	DeductionDeducted = "deducted"
	DeductionRestored = "restored"
)

// Category 商品分类。
type Category struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name;size:64" json:"name"`
	Sort      int       `gorm:"column:sort" json:"sort"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 表名。
func (Category) TableName() string { return "categories" }

// Product 商品。
// Price 用 money.Cents，读写数据库时自动在 DECIMAL 与"分"之间转换。
type Product struct {
	ID          int64       `gorm:"column:id;primaryKey;autoIncrement"`
	CategoryID  int64       `gorm:"column:category_id"`
	Name        string      `gorm:"column:name;size:128"`
	Description string      `gorm:"column:description"`
	Price       money.Cents `gorm:"column:price"`
	Stock       int         `gorm:"column:stock"`
	ImageURL    string      `gorm:"column:image_url;size:255"`
	Status      int8        `gorm:"column:status"`
	CreatedAt   time.Time   `gorm:"column:created_at"`
	UpdatedAt   time.Time   `gorm:"column:updated_at"`
}

// TableName 表名。
func (Product) TableName() string { return "products" }

// OnSale 是否在售。
func (p *Product) OnSale() bool { return p.Status == StatusOnSale }

// StockDeduction 库存扣减流水。一个订单里每个商品一行。
type StockDeduction struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	OrderNo   string    `gorm:"column:order_no;size:32"`
	ProductID int64     `gorm:"column:product_id"`
	Quantity  int       `gorm:"column:quantity"`
	Status    string    `gorm:"column:status;size:16"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName 表名。
func (StockDeduction) TableName() string { return "stock_deductions" }

// Public 商品对外结构。价格以字符串返回，前端直接展示。
type Public struct {
	ID          int64       `json:"id"`
	CategoryID  int64       `json:"category_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Price       money.Cents `json:"price"`
	Stock       int         `json:"stock"`
	ImageURL    string      `json:"image_url"`
	Status      int8        `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ToPublic 转换。
func (p *Product) ToPublic() Public {
	return Public{
		ID:          p.ID,
		CategoryID:  p.CategoryID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		ImageURL:    p.ImageURL,
		Status:      p.Status,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// ToPublicList 批量转换。
func ToPublicList(products []Product) []Public {
	out := make([]Public, 0, len(products))
	for i := range products {
		out = append(out, products[i].ToPublic())
	}
	return out
}
