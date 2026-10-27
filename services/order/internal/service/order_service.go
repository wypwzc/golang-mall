// Package service 订单业务逻辑。
package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/services/order/internal/client"
	"go-mall/services/order/internal/model"
)

// OrderRepo 订单仓储抽象。
type OrderRepo interface {
	Create(ctx context.Context, order *model.Order) error
	ByOrderNo(ctx context.Context, orderNo string) (*model.Order, error)
	ListByUser(ctx context.Context, userID int64, page, pageSize int) ([]model.Order, int64, error)
	ListAll(ctx context.Context, page, pageSize int, status string) ([]model.Order, int64, error)
	UpdateStatus(ctx context.Context, orderNo string, userID int64, from, to string) (bool, error)
}

// ProductClient 商品服务抽象。
type ProductClient interface {
	Batch(ctx context.Context, ids []int64) (map[int64]client.Product, error)
	Deduct(ctx context.Context, orderNo string, items []client.Item) ([]client.Product, error)
	Restore(ctx context.Context, orderNo string) error
}

// Service 订单服务。
type Service struct {
	repo     OrderRepo
	products ProductClient
	log      *slog.Logger
}

// New 创建服务。
func New(repo OrderRepo, products ProductClient, log *slog.Logger) *Service {
	return &Service{repo: repo, products: products, log: log}
}

// ItemInput 下单明细。
type ItemInput struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required"`
}

// Create 创建订单。
//
// 流程：先让商品服务扣库存并返回服务端价格，再用这份价格算金额、写订单。
// 价格永远以商品服务为准，前端传来的价格一律忽略。
func (s *Service) Create(ctx context.Context, userID int64, items []ItemInput) (*model.Public, error) {
	if len(items) == 0 {
		return nil, apperr.InvalidParam.WithMessage("请选择要购买的商品")
	}
	if len(items) > 50 {
		return nil, apperr.InvalidParam.WithMessage("单次最多购买 50 种商品")
	}

	// 同一商品出现多次时合并数量
	merged := make(map[int64]int, len(items))
	order := make([]int64, 0, len(items))
	for _, it := range items {
		if it.ProductID <= 0 {
			return nil, apperr.InvalidParam.WithMessage("商品 ID 不合法")
		}
		if it.Quantity < 1 || it.Quantity > 999 {
			return nil, apperr.InvalidParam.WithMessage("购买数量需在 1-999 之间")
		}
		if _, ok := merged[it.ProductID]; !ok {
			order = append(order, it.ProductID)
		}
		merged[it.ProductID] += it.Quantity
	}

	deductItems := make([]client.Item, 0, len(order))
	for _, id := range order {
		deductItems = append(deductItems, client.Item{ProductID: id, Quantity: merged[id]})
	}

	orderNo := newOrderNo()

	// 扣库存。库存不足会在这里直接返回 40002，不会产生订单
	products, err := s.products.Deduct(ctx, orderNo, deductItems)
	if err != nil {
		return nil, err
	}

	// 用商品服务返回的价格算总额
	snapshots := make(map[int64]client.Product, len(products))
	for _, p := range products {
		snapshots[p.ID] = p
	}

	var total money.Cents
	orderItems := make([]model.OrderItem, 0, len(deductItems))
	for _, it := range deductItems {
		p, ok := snapshots[it.ProductID]
		if !ok {
			// 理论上不会发生：扣减成功却没返回快照
			s.restoreAsync(orderNo)
			return nil, apperr.Internal.WithMessage("商品信息不完整，请重试")
		}

		price := p.PriceCents()
		subtotal := price.Mul(it.Quantity)
		total = total.Add(subtotal)

		orderItems = append(orderItems, model.OrderItem{
			ProductID:    p.ID,
			ProductName:  p.Name,
			ProductImage: p.ImageURL,
			Price:        price,
			Quantity:     it.Quantity,
			Subtotal:     subtotal,
		})
	}

	o := &model.Order{
		OrderNo:     orderNo,
		UserID:      userID,
		TotalAmount: total,
		Status:      model.StatusPending,
		Items:       orderItems,
	}

	if err := s.repo.Create(ctx, o); err != nil {
		// 订单没写成功，把刚扣掉的库存还回去，避免"扣了库存却没有订单"
		s.restoreAsync(orderNo)
		return nil, err
	}

	pub := o.ToPublic()
	return &pub, nil
}

// List 我的订单。
func (s *Service) List(ctx context.Context, userID int64, page, pageSize int) ([]model.Public, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	list, total, err := s.repo.ListByUser(ctx, userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return model.ToPublicList(list), total, nil
}

// Get 订单详情。只能查自己的订单。
func (s *Service) Get(ctx context.Context, userID int64, orderNo string) (*model.Public, error) {
	o, err := s.repo.ByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if o.UserID != userID {
		// 不返回"无权查看"，避免暴露订单是否存在
		return nil, apperr.NotFound.WithMessage("订单不存在")
	}
	pub := o.ToPublic()
	return &pub, nil
}

// Pay 模拟支付：把待支付订单改成已支付。
func (s *Service) Pay(ctx context.Context, userID int64, orderNo string) (*model.Public, error) {
	ok, err := s.repo.UpdateStatus(ctx, orderNo, userID, model.StatusPending, model.StatusPaid)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.OrderStatusInvalid.WithMessage("订单不存在或不是待支付状态")
	}

	o, err := s.repo.ByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	pub := o.ToPublic()
	return &pub, nil
}

// Cancel 取消订单：先改状态，成功后再恢复库存。
func (s *Service) Cancel(ctx context.Context, userID int64, orderNo string) (*model.Public, error) {
	ok, err := s.repo.UpdateStatus(ctx, orderNo, userID, model.StatusPending, model.StatusCancelled)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.OrderStatusInvalid.WithMessage("订单不存在或已支付，不能取消")
	}

	// 状态已经改成功，这里恢复库存失败也不能回滚订单，
	// 记录日志由人工处理（演示项目不做自动重试）
	if err := s.products.Restore(ctx, orderNo); err != nil {
		s.log.Error("恢复库存失败", "order_no", orderNo, "err", err)
	}

	o, err := s.repo.ByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	pub := o.ToPublic()
	return &pub, nil
}

// AdminList 管理端查询全部订单。
func (s *Service) AdminList(ctx context.Context, page, pageSize int, status string) ([]model.Public, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	if status != "" && status != model.StatusPending && status != model.StatusPaid && status != model.StatusCancelled {
		return nil, 0, apperr.InvalidParam.WithMessage("状态只能是 pending / paid / cancelled")
	}
	list, total, err := s.repo.ListAll(ctx, page, pageSize, status)
	if err != nil {
		return nil, 0, err
	}
	return model.ToPublicList(list), total, nil
}

// restoreAsync 补偿：订单没建成时把库存还回去。
func (s *Service) restoreAsync(orderNo string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.products.Restore(ctx, orderNo); err != nil {
		s.log.Error("补偿恢复库存失败，需要人工处理", "order_no", orderNo, "err", err)
	}
}

// newOrderNo 生成订单号：ORD + 时间 + 4 位随机数。
func newOrderNo() string {
	return fmt.Sprintf("ORD%s%04d", time.Now().Format("20060102150405"), time.Now().UnixNano()%10000)
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
