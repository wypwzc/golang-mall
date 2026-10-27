package service_test

import (
	"context"
	"testing"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/services/product/internal/model"
	"go-mall/services/product/internal/repository"
	"go-mall/services/product/internal/service"
)

// ---------- 内存仓储 ----------

type fakeProductRepo struct {
	byID   map[int64]*model.Product
	nextID int64
}

func newFakeProductRepo() *fakeProductRepo {
	return &fakeProductRepo{byID: map[int64]*model.Product{}, nextID: 1}
}

func (f *fakeProductRepo) Create(_ context.Context, p *model.Product) error {
	p.ID = f.nextID
	f.nextID++
	f.byID[p.ID] = p
	return nil
}

func (f *fakeProductRepo) ByID(_ context.Context, id int64) (*model.Product, error) {
	if p, ok := f.byID[id]; ok {
		return p, nil
	}
	return nil, apperr.NotFound.WithMessage("商品不存在")
}

func (f *fakeProductRepo) ByIDs(_ context.Context, ids []int64) ([]model.Product, error) {
	out := make([]model.Product, 0, len(ids))
	for _, id := range ids {
		if p, ok := f.byID[id]; ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeProductRepo) Update(_ context.Context, p *model.Product) error {
	if _, ok := f.byID[p.ID]; !ok {
		return apperr.NotFound.WithMessage("商品不存在")
	}
	f.byID[p.ID] = p
	return nil
}

func (f *fakeProductRepo) UpdateStatus(_ context.Context, id int64, status int8) error {
	if p, ok := f.byID[id]; ok {
		p.Status = status
		return nil
	}
	return apperr.NotFound.WithMessage("商品不存在")
}

func (f *fakeProductRepo) UpdateStock(_ context.Context, id int64, stock int) error {
	if p, ok := f.byID[id]; ok {
		p.Stock = stock
		return nil
	}
	return apperr.NotFound.WithMessage("商品不存在")
}

func (f *fakeProductRepo) Delete(_ context.Context, id int64) error {
	delete(f.byID, id)
	return nil
}

func (f *fakeProductRepo) List(_ context.Context, filter repository.ProductFilter) ([]model.Product, int64, error) {
	out := make([]model.Product, 0)
	for _, p := range f.byID {
		if filter.Status != nil && p.Status != *filter.Status {
			continue
		}
		if filter.CategoryID > 0 && p.CategoryID != filter.CategoryID {
			continue
		}
		out = append(out, *p)
	}
	return out, int64(len(out)), nil
}

type fakeCategoryRepo struct {
	byID map[int64]*model.Category
}

func newFakeCategoryRepo() *fakeCategoryRepo {
	return &fakeCategoryRepo{byID: map[int64]*model.Category{1: {ID: 1, Name: "默认分类"}}}
}

func (f *fakeCategoryRepo) Create(_ context.Context, c *model.Category) error {
	c.ID = int64(len(f.byID) + 1)
	f.byID[c.ID] = c
	return nil
}

func (f *fakeCategoryRepo) ByID(_ context.Context, id int64) (*model.Category, error) {
	if c, ok := f.byID[id]; ok {
		return c, nil
	}
	return nil, apperr.NotFound.WithMessage("分类不存在")
}

func (f *fakeCategoryRepo) List(_ context.Context) ([]model.Category, error) {
	out := make([]model.Category, 0, len(f.byID))
	for _, c := range f.byID {
		out = append(out, *c)
	}
	return out, nil
}

func (f *fakeCategoryRepo) Update(_ context.Context, id int64, name string, sort int) error {
	if c, ok := f.byID[id]; ok {
		c.Name, c.Sort = name, sort
		return nil
	}
	return apperr.NotFound.WithMessage("分类不存在")
}

func (f *fakeCategoryRepo) Delete(_ context.Context, id int64) error {
	delete(f.byID, id)
	return nil
}

type fakeStockRepo struct {
	deductCalls int
	lastItems   []repository.StockItem
}

func (f *fakeStockRepo) Deduct(_ context.Context, _ string, items []repository.StockItem) ([]model.Product, error) {
	f.deductCalls++
	f.lastItems = items
	return nil, nil
}

func (f *fakeStockRepo) Restore(_ context.Context, _ string) error { return nil }

// ---------- 脚手架 ----------

func newService() (*service.Service, *fakeProductRepo, *fakeStockRepo) {
	p := newFakeProductRepo()
	s := &fakeStockRepo{}
	return service.New(p, newFakeCategoryRepo(), s), p, s
}

func codeOf(err error) apperr.Code {
	if err == nil {
		return apperr.CodeOK
	}
	return apperr.From(err).Code
}

func intPtr(v int) *int { return &v }

// ---------- 商品校验 ----------

func TestCreateRejectsInvalidPrice(t *testing.T) {
	svc, _, _ := newService()

	for _, price := range []money.Cents{0, -100} {
		_, err := svc.Create(context.Background(), service.ProductInput{
			CategoryID: 1,
			Name:       "测试",
			Price:      price,
		})
		if codeOf(err) != apperr.CodeInvalidParam {
			t.Fatalf("价格 %d 应被拒绝，实际 %v", price, err)
		}
	}
}

func TestCreateRejectsMissingCategory(t *testing.T) {
	svc, _, _ := newService()

	_, err := svc.Create(context.Background(), service.ProductInput{
		CategoryID: 999,
		Name:       "测试",
		Price:      money.Cents(100),
	})
	if codeOf(err) != apperr.CodeInvalidParam {
		t.Fatalf("分类不存在应被拒绝，实际 %v", err)
	}
}

func TestCreateDefaultsToOffShelf(t *testing.T) {
	svc, _, _ := newService()

	p, err := svc.Create(context.Background(), service.ProductInput{
		CategoryID: 1,
		Name:       "新商品",
		Price:      money.Cents(1990),
		Stock:      intPtr(5),
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if p.Status != model.StatusOffShelf {
		t.Fatalf("新建商品默认应下架，实际 %d", p.Status)
	}
	if p.Price.String() != "19.90" {
		t.Fatalf("价格转换错误: %s", p.Price.String())
	}
}

// ---------- 公开可见性 ----------

func TestGetHidesOffShelfProduct(t *testing.T) {
	svc, repo, _ := newService()

	p, _ := svc.Create(context.Background(), service.ProductInput{
		CategoryID: 1, Name: "已下架商品", Price: money.Cents(100),
	})
	repo.byID[p.ID].Status = model.StatusOffShelf

	if _, err := svc.Get(context.Background(), p.ID); codeOf(err) != apperr.CodeNotFound {
		t.Fatalf("下架商品对公开接口应不可见，实际 %v", err)
	}
	// 管理端仍可见
	if _, err := svc.AdminGet(context.Background(), p.ID); err != nil {
		t.Fatalf("管理端应能看到下架商品: %v", err)
	}
}

func TestListOnlyReturnsOnSale(t *testing.T) {
	svc, repo, _ := newService()

	svc.Create(context.Background(), service.ProductInput{CategoryID: 1, Name: "在售", Price: money.Cents(100)})
	svc.Create(context.Background(), service.ProductInput{CategoryID: 1, Name: "下架", Price: money.Cents(100)})
	for _, p := range repo.byID {
		if p.Name == "在售" {
			p.Status = model.StatusOnSale
		}
	}

	list, total, err := svc.List(context.Background(), service.ListInput{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 1 || len(list) != 1 || list[0].Name != "在售" {
		t.Fatalf("公开列表应只含在售商品，实际 total=%d list=%+v", total, list)
	}
}

// ---------- 状态 ----------

func TestSetStatusRejectsInvalidValue(t *testing.T) {
	svc, _, _ := newService()

	if err := svc.SetStatus(context.Background(), 1, 9); codeOf(err) != apperr.CodeInvalidParam {
		t.Fatalf("非法状态应被拒绝，实际 %v", err)
	}
}

// ---------- 库存扣减入参 ----------

func TestDeductMergesDuplicateProducts(t *testing.T) {
	svc, _, stock := newService()

	_, err := svc.Deduct(context.Background(), "ORDER-1", []service.StockItemInput{
		{ProductID: 7, Quantity: 2},
		{ProductID: 7, Quantity: 3},
		{ProductID: 8, Quantity: 1},
	})
	if err != nil {
		t.Fatalf("扣减失败: %v", err)
	}
	if stock.deductCalls != 1 {
		t.Fatalf("应只调用一次仓储，实际 %d 次", stock.deductCalls)
	}
	if len(stock.lastItems) != 2 {
		t.Fatalf("重复商品应合并，期望 2 条明细，实际 %d", len(stock.lastItems))
	}
	for _, it := range stock.lastItems {
		if it.ProductID == 7 && it.Quantity != 5 {
			t.Fatalf("商品 7 的数量应合并为 5，实际 %d", it.Quantity)
		}
	}
}

func TestDeductRejectsBadQuantity(t *testing.T) {
	svc, _, _ := newService()

	for _, qty := range []int{0, -1, 10000} {
		_, err := svc.Deduct(context.Background(), "ORDER-2", []service.StockItemInput{
			{ProductID: 1, Quantity: qty},
		})
		if codeOf(err) != apperr.CodeInvalidParam {
			t.Fatalf("数量 %d 应被拒绝，实际 %v", qty, err)
		}
	}
}

func TestDeductRequiresOrderNo(t *testing.T) {
	svc, _, _ := newService()

	_, err := svc.Deduct(context.Background(), "  ", []service.StockItemInput{{ProductID: 1, Quantity: 1}})
	if codeOf(err) != apperr.CodeInvalidParam {
		t.Fatalf("订单号为空应被拒绝，实际 %v", err)
	}
}
