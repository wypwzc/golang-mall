// Package client 封装对商品服务的调用，购物车不直接读商品库。
package client

import (
	"context"
	"strconv"
	"time"

	"go-mall/pkg/apperr"
	"go-mall/pkg/httpx"
	"go-mall/pkg/money"
)

// Product 商品服务返回的商品快照（只取购物车需要的字段）。
type Product struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Price    string `json:"price"` // 商品服务用字符串返回金额，避免浮点误差
	Stock    int    `json:"stock"`
	Status   int8   `json:"status"` // 1 上架 0 下架
	ImageURL string `json:"image_url"`
}

// PriceValue 把 "99.90" 转成 99.9，解析失败按 0 处理。
func (p Product) PriceValue() float64 {
	v, err := strconv.ParseFloat(p.Price, 64)
	if err != nil {
		return 0
	}
	return v
}

// PriceCents 把 "99.90" 转成分。和订单服务用同一套金额类型，避免浮点误差。
func (p Product) PriceCents() money.Cents {
	c, err := money.FromYuan(p.Price)
	if err != nil {
		return 0
	}
	return c
}

// OnSale 是否在售。
func (p Product) OnSale() bool { return p.Status == 1 }

// ProductClient 商品服务客户端，服务启动时创建一次并复用。
type ProductClient struct {
	c *httpx.Client
}

// NewProductClient 创建客户端。baseURL 来自配置，token 是内部接口凭证。
func NewProductClient(baseURL, internalToken string, timeout time.Duration) *ProductClient {
	return &ProductClient{c: httpx.New(baseURL, internalToken, timeout)}
}

// Batch 按 ID 批量取商品，返回 id -> 商品 的映射，查不到的 ID 不会出现在结果里。
func (pc *ProductClient) Batch(ctx context.Context, ids []int64) (map[int64]Product, error) {
	if len(ids) == 0 {
		return map[int64]Product{}, nil
	}

	var list []Product
	body := map[string]any{"ids": ids}
	if err := pc.c.Post(ctx, "/internal/products/batch", body, &list); err != nil {
		return nil, apperr.Upstream.WithCause(err)
	}

	out := make(map[int64]Product, len(list))
	for _, p := range list {
		out[p.ID] = p
	}
	return out, nil
}
