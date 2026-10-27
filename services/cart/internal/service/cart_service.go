// Package service 购物车业务逻辑。
package service

import (
	"context"

	"go-mall/pkg/apperr"
	"go-mall/services/cart/internal/client"
	"go-mall/services/cart/internal/model"
)

// CartRepo 购物车仓储抽象，便于单元测试。
type CartRepo interface {
	List(ctx context.Context, userID int64) ([]model.CartItem, error)
	Get(ctx context.Context, userID, productID int64) (*model.CartItem, error)
	Add(ctx context.Context, userID, productID int64, quantity int) error
	SetQuantity(ctx context.Context, userID, productID int64, quantity int) error
	Delete(ctx context.Context, userID, productID int64) error
	Clear(ctx context.Context, userID int64) error
}

// ProductClient 商品服务抽象。
type ProductClient interface {
	Batch(ctx context.Context, ids []int64) (map[int64]client.Product, error)
}

// Service 购物车服务。
type Service struct {
	repo     CartRepo
	products ProductClient
}

// New 创建服务。
func New(repo CartRepo, products ProductClient) *Service {
	return &Service{repo: repo, products: products}
}

// View 查询购物车。
// 价格与库存每次实时向商品服务取，购物车里只存商品 ID 和数量。
func (s *Service) View(ctx context.Context, userID int64) (*model.View, error) {
	rows, err := s.repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}

	view := &model.View{Items: make([]model.Item, 0, len(rows))}
	if len(rows) == 0 {
		return view, nil
	}

	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProductID)
	}

	products, err := s.products.Batch(ctx, ids)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		p, ok := products[row.ProductID]
		item := model.Item{
			ID:        row.ID,
			ProductID: row.ProductID,
			Quantity:  row.Quantity,
		}

		if ok {
			item.Name = p.Name
			item.ImageURL = p.ImageURL
			item.Price = p.PriceCents()
			item.Stock = p.Stock
			item.Subtotal = item.Price.Mul(row.Quantity)
			item.Available = p.OnSale() && p.Stock >= row.Quantity
		} else {
			// 商品已被删除，仍然展示出来让用户自己删掉
			item.Name = "商品已下架"
			item.Available = false
		}

		if item.Available {
			view.Total = view.Total.Add(item.Subtotal)
			view.Count += item.Quantity
		}
		view.Items = append(view.Items, item)
	}

	return view, nil
}

// Add 加购。会校验商品是否存在、是否在售、库存是否够。
func (s *Service) Add(ctx context.Context, userID, productID int64, quantity int) error {
	if quantity < 1 || quantity > 999 {
		return apperr.InvalidParam.WithMessage("数量需在 1-999 之间")
	}

	products, err := s.products.Batch(ctx, []int64{productID})
	if err != nil {
		return err
	}
	p, ok := products[productID]
	if !ok {
		return apperr.NotFound.WithMessage("商品不存在")
	}
	if !p.OnSale() {
		return apperr.Conflict.WithMessage("商品已下架")
	}

	// 已经在购物车里的数量要一起算，避免分多次加购绕过库存校验
	existing, err := s.repo.Get(ctx, userID, productID)
	if err != nil {
		return err
	}
	total := quantity
	if existing != nil {
		total += existing.Quantity
	}
	if total > p.Stock {
		return apperr.StockNotEnough.WithMessage("库存不足，最多可购买 %d 件", p.Stock)
	}

	return s.repo.Add(ctx, userID, productID, quantity)
}

// SetQuantity 修改数量。
func (s *Service) SetQuantity(ctx context.Context, userID, productID int64, quantity int) error {
	if quantity < 1 || quantity > 999 {
		return apperr.InvalidParam.WithMessage("数量需在 1-999 之间")
	}

	products, err := s.products.Batch(ctx, []int64{productID})
	if err != nil {
		return err
	}
	if p, ok := products[productID]; ok && quantity > p.Stock {
		return apperr.StockNotEnough.WithMessage("库存不足，最多可购买 %d 件", p.Stock)
	}

	return s.repo.SetQuantity(ctx, userID, productID, quantity)
}

// Remove 删除一个商品。
func (s *Service) Remove(ctx context.Context, userID, productID int64) error {
	return s.repo.Delete(ctx, userID, productID)
}

// Clear 清空购物车。
func (s *Service) Clear(ctx context.Context, userID int64) error {
	return s.repo.Clear(ctx, userID)
}
