// Package client 调用商品服务的内部接口（取快照、扣库存、恢复库存）。
package client

import (
	"context"
	"time"

	"go-mall/pkg/apperr"
	"go-mall/pkg/httpx"
	"go-mall/pkg/money"
)

// Product 商品快照，只取订单需要的字段。
type Product struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Price    string `json:"price"` // 商品服务用字符串返回金额
	Stock    int    `json:"stock"`
	Status   int8   `json:"status"`
	ImageURL string `json:"image_url"`
}

// PriceCents 把 "99.90" 转成分。解析失败按 0 处理，由调用方决定是否报错。
func (p Product) PriceCents() money.Cents {
	v, err := money.FromYuan(p.Price)
	if err != nil {
		return 0
	}
	return v
}

// OnSale 是否在售。
func (p Product) OnSale() bool { return p.Status == 1 }

// Item 扣减明细。
type Item struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

// ProductClient 商品服务客户端。
type ProductClient struct {
	c *httpx.Client
}

// NewProductClient 创建客户端。
func NewProductClient(baseURL, internalToken string, timeout time.Duration) *ProductClient {
	return &ProductClient{c: httpx.New(baseURL, internalToken, timeout)}
}

// Batch 批量取商品快照。
func (pc *ProductClient) Batch(ctx context.Context, ids []int64) (map[int64]Product, error) {
	if len(ids) == 0 {
		return map[int64]Product{}, nil
	}

	var list []Product
	if err := pc.c.Post(ctx, "/internal/products/batch", map[string]any{"ids": ids}, &list); err != nil {
		return nil, apperr.Upstream.WithCause(err)
	}

	out := make(map[int64]Product, len(list))
	for _, p := range list {
		out[p.ID] = p
	}
	return out, nil
}

// Deduct 扣减库存，返回扣减后的商品快照（含服务端价格，用它算订单金额）。
func (pc *ProductClient) Deduct(ctx context.Context, orderNo string, items []Item) ([]Product, error) {
	body := map[string]any{"order_no": orderNo, "items": items}

	var list []Product
	if err := pc.c.Post(ctx, "/internal/products/stock/deduct", body, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// Restore 恢复库存，下单失败或取消订单时调用。
func (pc *ProductClient) Restore(ctx context.Context, orderNo string) error {
	return pc.c.Post(ctx, "/internal/products/stock/restore", map[string]any{"order_no": orderNo}, nil)
}
