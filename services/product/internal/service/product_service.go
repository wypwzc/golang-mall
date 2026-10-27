// Package service 商品服务的业务逻辑。
package service

import (
	"context"
	"strings"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/services/product/internal/model"
	"go-mall/services/product/internal/repository"
)

// 仓储抽象，便于在没有数据库时做单元测试。
type (
	// ProductRepo 商品仓储。
	ProductRepo interface {
		Create(ctx context.Context, p *model.Product) error
		ByID(ctx context.Context, id int64) (*model.Product, error)
		ByIDs(ctx context.Context, ids []int64) ([]model.Product, error)
		Update(ctx context.Context, p *model.Product) error
		UpdateStatus(ctx context.Context, id int64, status int8) error
		UpdateStock(ctx context.Context, id int64, stock int) error
		Delete(ctx context.Context, id int64) error
		List(ctx context.Context, f repository.ProductFilter) ([]model.Product, int64, error)
	}

	// CategoryRepo 分类仓储。
	CategoryRepo interface {
		Create(ctx context.Context, c *model.Category) error
		ByID(ctx context.Context, id int64) (*model.Category, error)
		List(ctx context.Context) ([]model.Category, error)
		Update(ctx context.Context, id int64, name string, sort int) error
		Delete(ctx context.Context, id int64) error
	}

	// StockRepo 库存仓储。
	StockRepo interface {
		Deduct(ctx context.Context, orderNo string, items []repository.StockItem) ([]model.Product, error)
		Restore(ctx context.Context, orderNo string) error
	}
)

// Service 商品服务。
type Service struct {
	products   ProductRepo
	categories CategoryRepo
	stock      StockRepo
}

// New 创建服务。
func New(products ProductRepo, categories CategoryRepo, stock StockRepo) *Service {
	return &Service{products: products, categories: categories, stock: stock}
}

// ---------- 入参 ----------

// ListInput 商品列表查询参数。
type ListInput struct {
	Page       int
	PageSize   int
	Keyword    string
	CategoryID int64
	MinPrice   *money.Cents
	MaxPrice   *money.Cents
	Sort       string
}

// ProductInput 新建/修改商品。
type ProductInput struct {
	CategoryID  int64       `json:"category_id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Price       money.Cents `json:"price"`
	Stock       *int        `json:"stock"`
	ImageURL    string      `json:"image_url"`
	Status      *int8       `json:"status"`
}

// CategoryInput 新建/修改分类。
type CategoryInput struct {
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

// StockItemInput 扣减明细。
type StockItemInput struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

// ---------- 查询 ----------

// List 公开列表：只返回上架商品。
func (s *Service) List(ctx context.Context, in ListInput) ([]model.Public, int64, error) {
	onSale := model.StatusOnSale
	page, pageSize := normalizePage(in.Page, in.PageSize)

	list, total, err := s.products.List(ctx, repository.ProductFilter{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    strings.TrimSpace(in.Keyword),
		CategoryID: in.CategoryID,
		MinPrice:   in.MinPrice,
		MaxPrice:   in.MaxPrice,
		Status:     &onSale,
		Sort:       in.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	return model.ToPublicList(list), total, nil
}

// AdminList 管理端列表：包含已下架商品。
func (s *Service) AdminList(ctx context.Context, in ListInput, status *int8) ([]model.Public, int64, error) {
	page, pageSize := normalizePage(in.Page, in.PageSize)

	list, total, err := s.products.List(ctx, repository.ProductFilter{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    strings.TrimSpace(in.Keyword),
		CategoryID: in.CategoryID,
		MinPrice:   in.MinPrice,
		MaxPrice:   in.MaxPrice,
		Status:     status,
		Sort:       in.Sort,
	})
	if err != nil {
		return nil, 0, err
	}
	return model.ToPublicList(list), total, nil
}

// Get 公开详情：已下架的商品对外不可见。
func (s *Service) Get(ctx context.Context, id int64) (*model.Public, error) {
	p, err := s.products.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !p.OnSale() {
		return nil, apperr.NotFound.WithMessage("商品已下架")
	}
	pub := p.ToPublic()
	return &pub, nil
}

// AdminGet 管理端详情：可以看到下架商品。
func (s *Service) AdminGet(ctx context.Context, id int64) (*model.Public, error) {
	p, err := s.products.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	pub := p.ToPublic()
	return &pub, nil
}

// BatchGet 供购物车/订单服务批量取商品快照。
func (s *Service) BatchGet(ctx context.Context, ids []int64) ([]model.Public, error) {
	if len(ids) == 0 {
		return []model.Public{}, nil
	}
	list, err := s.products.ByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return model.ToPublicList(list), nil
}

// ---------- 商品管理 ----------

// Create 新建商品。
func (s *Service) Create(ctx context.Context, in ProductInput) (*model.Public, error) {
	if err := s.validateProduct(ctx, in); err != nil {
		return nil, err
	}

	stock := 0
	if in.Stock != nil {
		stock = *in.Stock
	}
	status := model.StatusOffShelf // 新建默认下架，确认无误后再上架
	if in.Status != nil {
		status = *in.Status
	}

	p := &model.Product{
		CategoryID:  in.CategoryID,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Price:       in.Price,
		Stock:       stock,
		ImageURL:    strings.TrimSpace(in.ImageURL),
		Status:      status,
	}
	if err := s.products.Create(ctx, p); err != nil {
		return nil, err
	}
	pub := p.ToPublic()
	return &pub, nil
}

// Update 修改商品。库存只能通过 Deduct/Restore 或管理端补货接口调整。
func (s *Service) Update(ctx context.Context, id int64, in ProductInput) (*model.Public, error) {
	if err := s.validateProduct(ctx, in); err != nil {
		return nil, err
	}

	p, err := s.products.ByID(ctx, id)
	if err != nil {
		return nil, err
	}

	p.CategoryID = in.CategoryID
	p.Name = strings.TrimSpace(in.Name)
	p.Description = strings.TrimSpace(in.Description)
	p.Price = in.Price
	p.ImageURL = strings.TrimSpace(in.ImageURL)
	if err := s.products.Update(ctx, p); err != nil {
		return nil, err
	}

	// 管理端显式补货/调整库存，单独走一个方法，避免与并发扣减混在一起
	if in.Stock != nil {
		if *in.Stock < 0 {
			return nil, apperr.InvalidParam.WithMessage("库存不能为负数")
		}
		if err := s.products.UpdateStock(ctx, id, *in.Stock); err != nil {
			return nil, err
		}
		p.Stock = *in.Stock
	}

	pub := p.ToPublic()
	return &pub, nil
}

// SetStatus 上架/下架。
func (s *Service) SetStatus(ctx context.Context, id int64, status int8) error {
	if status != model.StatusOnSale && status != model.StatusOffShelf {
		return apperr.InvalidParam.WithMessage("状态只能是 0(下架) 或 1(上架)")
	}
	return s.products.UpdateStatus(ctx, id, status)
}

// Delete 删除商品。
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.products.Delete(ctx, id)
}

func (s *Service) validateProduct(ctx context.Context, in ProductInput) error {
	name := strings.TrimSpace(in.Name)
	if n := len([]rune(name)); n < 1 || n > 128 {
		return apperr.InvalidParam.WithMessage("商品名称长度需在 1-128 个字符之间")
	}
	if !in.Price.Positive() {
		return apperr.InvalidParam.WithMessage("商品价格必须大于 0")
	}
	// DECIMAL(10,2) 最大 99999999.99
	if in.Price > money.Cents(9999999999) {
		return apperr.InvalidParam.WithMessage("商品价格超出上限")
	}
	if in.Stock != nil && *in.Stock < 0 {
		return apperr.InvalidParam.WithMessage("库存不能为负数")
	}
	if len(in.ImageURL) > 255 {
		return apperr.InvalidParam.WithMessage("图片地址过长")
	}
	if in.CategoryID <= 0 {
		return apperr.InvalidParam.WithMessage("请选择商品分类")
	}
	if _, err := s.categories.ByID(ctx, in.CategoryID); err != nil {
		return apperr.InvalidParam.WithMessage("分类不存在")
	}
	return nil
}

// ---------- 分类 ----------

// ListCategories 分类列表。
func (s *Service) ListCategories(ctx context.Context) ([]model.Category, error) {
	return s.categories.List(ctx)
}

// CreateCategory 新建分类。
func (s *Service) CreateCategory(ctx context.Context, in CategoryInput) (*model.Category, error) {
	name := strings.TrimSpace(in.Name)
	if n := len([]rune(name)); n < 1 || n > 64 {
		return nil, apperr.InvalidParam.WithMessage("分类名称长度需在 1-64 个字符之间")
	}
	c := &model.Category{Name: name, Sort: in.Sort}
	if err := s.categories.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// UpdateCategory 修改分类。
func (s *Service) UpdateCategory(ctx context.Context, id int64, in CategoryInput) error {
	name := strings.TrimSpace(in.Name)
	if n := len([]rune(name)); n < 1 || n > 64 {
		return apperr.InvalidParam.WithMessage("分类名称长度需在 1-64 个字符之间")
	}
	return s.categories.Update(ctx, id, name, in.Sort)
}

// DeleteCategory 删除分类。
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	return s.categories.Delete(ctx, id)
}

// ---------- 库存（内部接口） ----------

// Deduct 扣减库存。同一商品出现多次时先合并数量。
func (s *Service) Deduct(ctx context.Context, orderNo string, items []StockItemInput) ([]model.Public, error) {
	if strings.TrimSpace(orderNo) == "" {
		return nil, apperr.InvalidParam.WithMessage("订单号不能为空")
	}
	if len(items) == 0 {
		return nil, apperr.InvalidParam.WithMessage("扣减明细不能为空")
	}
	if len(items) > 50 {
		return nil, apperr.InvalidParam.WithMessage("单次最多处理 50 个商品")
	}

	merged := make(map[int64]int, len(items))
	order := make([]int64, 0, len(items))
	for _, it := range items {
		if it.ProductID <= 0 {
			return nil, apperr.InvalidParam.WithMessage("商品 ID 不合法")
		}
		if it.Quantity <= 0 || it.Quantity > 9999 {
			return nil, apperr.InvalidParam.WithMessage("购买数量需在 1-9999 之间")
		}
		if _, ok := merged[it.ProductID]; !ok {
			order = append(order, it.ProductID)
		}
		merged[it.ProductID] += it.Quantity
	}

	stockItems := make([]repository.StockItem, 0, len(order))
	for _, id := range order {
		stockItems = append(stockItems, repository.StockItem{ProductID: id, Quantity: merged[id]})
	}

	products, err := s.stock.Deduct(ctx, orderNo, stockItems)
	if err != nil {
		return nil, err
	}
	return model.ToPublicList(products), nil
}

// Restore 恢复库存（取消订单）。
func (s *Service) Restore(ctx context.Context, orderNo string) error {
	if strings.TrimSpace(orderNo) == "" {
		return apperr.InvalidParam.WithMessage("订单号不能为空")
	}
	return s.stock.Restore(ctx, orderNo)
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
