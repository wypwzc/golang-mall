package service_test

import (
	"context"
	"testing"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/services/cart/internal/client"
	"go-mall/services/cart/internal/model"
	"go-mall/services/cart/internal/service"
)

type fakeRepo struct {
	items map[int64]*model.CartItem
	seq   int64
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[int64]*model.CartItem{}} }

func (f *fakeRepo) List(_ context.Context, userID int64) ([]model.CartItem, error) {
	out := make([]model.CartItem, 0, len(f.items))
	for _, it := range f.items {
		if it.UserID == userID {
			out = append(out, *it)
		}
	}
	return out, nil
}

func (f *fakeRepo) Get(_ context.Context, userID, productID int64) (*model.CartItem, error) {
	for _, it := range f.items {
		if it.UserID == userID && it.ProductID == productID {
			return it, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) Add(_ context.Context, userID, productID int64, quantity int) error {
	for _, it := range f.items {
		if it.UserID == userID && it.ProductID == productID {
			it.Quantity += quantity
			return nil
		}
	}
	f.seq++
	f.items[f.seq] = &model.CartItem{ID: f.seq, UserID: userID, ProductID: productID, Quantity: quantity}
	return nil
}

func (f *fakeRepo) SetQuantity(_ context.Context, userID, productID int64, quantity int) error {
	for _, it := range f.items {
		if it.UserID == userID && it.ProductID == productID {
			it.Quantity = quantity
			return nil
		}
	}
	return apperr.NotFound.WithMessage("购物车里没有这个商品")
}

func (f *fakeRepo) Delete(_ context.Context, userID, productID int64) error {
	for k, it := range f.items {
		if it.UserID == userID && it.ProductID == productID {
			delete(f.items, k)
			return nil
		}
	}
	return nil
}

func (f *fakeRepo) Clear(_ context.Context, userID int64) error {
	for k, it := range f.items {
		if it.UserID == userID {
			delete(f.items, k)
		}
	}
	return nil
}

type fakeClient struct {
	products map[int64]client.Product
}

func (f *fakeClient) Batch(_ context.Context, ids []int64) (map[int64]client.Product, error) {
	out := map[int64]client.Product{}
	for _, id := range ids {
		if p, ok := f.products[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

func newService() (*service.Service, *fakeRepo) {
	repo := newFakeRepo()
	c := &fakeClient{products: map[int64]client.Product{
		1: {ID: 1, Name: "商品A", Price: "99.90", Stock: 10, Status: 1},
		2: {ID: 2, Name: "商品B", Price: "19.90", Stock: 2, Status: 0},
	}}
	return service.New(repo, c), repo
}

func codeOf(err error) apperr.Code {
	if err == nil {
		return apperr.CodeOK
	}
	return apperr.From(err).Code
}

func TestAddAndView(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	if err := svc.Add(ctx, 1, 1, 2); err != nil {
		t.Fatalf("加购失败: %v", err)
	}
	if err := svc.Add(ctx, 1, 1, 1); err != nil {
		t.Fatalf("重复加购失败: %v", err)
	}

	view, err := svc.View(ctx, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(view.Items) != 1 {
		t.Fatalf("应合并为一条，实际 %d 条", len(view.Items))
	}
	if view.Items[0].Quantity != 3 {
		t.Fatalf("数量应为 3，实际 %d", view.Items[0].Quantity)
	}
	if view.Total != money.Cents(29970) {
		t.Fatalf("合计应为 299.70，实际 %v", view.Total)
	}
}

func TestAddRejectsOffShelf(t *testing.T) {
	svc, _ := newService()

	err := svc.Add(context.Background(), 1, 2, 1)
	if codeOf(err) != apperr.CodeConflict {
		t.Fatalf("下架商品应拒绝加购，实际 %v", err)
	}
}

func TestAddRespectsExistingQuantity(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	if err := svc.Add(ctx, 1, 1, 8); err != nil {
		t.Fatalf("加购失败: %v", err)
	}
	if err := svc.Add(ctx, 1, 1, 5); codeOf(err) != apperr.CodeStockNotEnough {
		t.Fatalf("累计 13 件超过库存 10，应拒绝，实际 %v", err)
	}
}

func TestViewMarksUnavailable(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	repo.Add(ctx, 1, 2, 1)
	view, err := svc.View(ctx, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(view.Items) != 1 || view.Items[0].Available {
		t.Fatal("下架商品应标记为不可购买")
	}
	if view.Total != 0 || view.Count != 0 {
		t.Fatal("不可购买的商品不应计入合计")
	}
}

func TestViewHandlesDeletedProduct(t *testing.T) {
	svc, repo := newService()
	ctx := context.Background()

	repo.Add(ctx, 1, 999, 1)
	view, err := svc.View(ctx, 1)
	if err != nil {
		t.Fatalf("商品被删除时不应报错: %v", err)
	}
	if len(view.Items) != 1 || view.Items[0].Name != "商品已下架" {
		t.Fatalf("应展示为已下架，实际 %+v", view.Items)
	}
}

func TestSetQuantityValidates(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	svc.Add(ctx, 1, 1, 1)

	if err := svc.SetQuantity(ctx, 1, 1, 0); codeOf(err) != apperr.CodeInvalidParam {
		t.Fatalf("数量 0 应被拒绝，实际 %v", err)
	}
	if err := svc.SetQuantity(ctx, 1, 1, 11); codeOf(err) != apperr.CodeStockNotEnough {
		t.Fatalf("超过库存应被拒绝，实际 %v", err)
	}
	if err := svc.SetQuantity(ctx, 1, 1, 5); err != nil {
		t.Fatalf("正常改数量失败: %v", err)
	}
}
