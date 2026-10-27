package service_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go-mall/pkg/apperr"
	"go-mall/pkg/money"
	"go-mall/services/order/internal/client"
	"go-mall/services/order/internal/model"
	"go-mall/services/order/internal/service"
)

// ---------- 假的仓储和商品客户端，测试不连数据库、不发 HTTP ----------

// fakeRepo 内存版订单仓储。
type fakeRepo struct {
	orders   map[string]*model.Order // 按订单号索引
	seq      int64
	createOK bool // 为 false 时模拟写库失败
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{orders: map[string]*model.Order{}, createOK: true}
}

func (f *fakeRepo) Create(_ context.Context, order *model.Order) error {
	if !f.createOK {
		return apperr.Internal.WithMessage("写库失败")
	}
	f.seq++
	cp := *order
	cp.ID = f.seq
	f.orders[order.OrderNo] = &cp
	return nil
}

func (f *fakeRepo) ByOrderNo(_ context.Context, orderNo string) (*model.Order, error) {
	o, ok := f.orders[orderNo]
	if !ok {
		return nil, apperr.NotFound.WithMessage("订单不存在")
	}
	return o, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, userID int64, _, _ int) ([]model.Order, int64, error) {
	out := make([]model.Order, 0, len(f.orders))
	for _, o := range f.orders {
		if o.UserID == userID {
			out = append(out, *o)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) ListAll(_ context.Context, _, _ int, status string) ([]model.Order, int64, error) {
	out := make([]model.Order, 0, len(f.orders))
	for _, o := range f.orders {
		if status == "" || o.Status == status {
			out = append(out, *o)
		}
	}
	return out, int64(len(out)), nil
}

// UpdateStatus 模拟条件更新：只有当前状态等于 from 才改成功。
func (f *fakeRepo) UpdateStatus(_ context.Context, orderNo string, userID int64, from, to string) (bool, error) {
	o, ok := f.orders[orderNo]
	if !ok {
		return false, nil
	}
	if userID > 0 && o.UserID != userID {
		return false, nil
	}
	if o.Status != from {
		return false, nil
	}
	o.Status = to
	return true, nil
}

// fakeProducts 内存版商品服务。
type fakeProducts struct {
	stock     map[int64]int            // 商品 ID -> 库存
	prices    map[int64]money.Cents    // 商品 ID -> 单价
	deducted  map[string][]client.Item // 订单号 -> 扣过的明细，用于校验恢复
	restoreCt int                      // Restore 被调用次数
}

func newFakeProducts() *fakeProducts {
	return &fakeProducts{
		stock:    map[int64]int{1: 10, 2: 5},
		prices:   map[int64]money.Cents{1: yuan("99.90"), 2: yuan("19.90")},
		deducted: map[string][]client.Item{},
	}
}

func (f *fakeProducts) Batch(_ context.Context, ids []int64) (map[int64]client.Product, error) {
	out := make(map[int64]client.Product, len(ids))
	for _, id := range ids {
		out[id] = client.Product{ID: id, Name: "商品" + itoa(id), Price: f.prices[id].String(), Stock: f.stock[id]}
	}
	return out, nil
}

func (f *fakeProducts) Deduct(_ context.Context, orderNo string, items []client.Item) ([]client.Product, error) {
	// 先整体校验库存，任意一项不足就全部失败（和真实服务的语义一致）
	for _, it := range items {
		if f.stock[it.ProductID] < it.Quantity {
			return nil, apperr.StockNotEnough.WithMessage("商品「%d」库存不足", it.ProductID)
		}
	}
	out := make([]client.Product, 0, len(items))
	for _, it := range items {
		f.stock[it.ProductID] -= it.Quantity
		out = append(out, client.Product{
			ID:    it.ProductID,
			Name:  "商品" + itoa(it.ProductID),
			Price: f.prices[it.ProductID].String(),
			Stock: f.stock[it.ProductID],
		})
	}
	f.deducted[orderNo] = items
	return out, nil
}

func (f *fakeProducts) Restore(_ context.Context, orderNo string) error {
	f.restoreCt++
	for _, it := range f.deducted[orderNo] {
		f.stock[it.ProductID] += it.Quantity
	}
	delete(f.deducted, orderNo)
	return nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// yuan 测试辅助：把 "99.90" 这样的字符串转成分，出错直接 panic。
func yuan(s string) money.Cents {
	c, err := money.FromYuan(s)
	if err != nil {
		panic(err)
	}
	return c
}

// newService 组装一个不依赖外部资源的服务实例。
func newService(repo *fakeRepo, products *fakeProducts) *service.Service {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return service.New(repo, products, log)
}

// ---------- 测试用例 ----------

// TestCreateSuccess 正常下单：金额以商品服务价格为准，库存被扣。
func TestCreateSuccess(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)

	got, err := svc.Create(context.Background(), 1, []service.ItemInput{
		{ProductID: 1, Quantity: 2},
		{ProductID: 2, Quantity: 1},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}

	// 99.90 * 2 + 19.90 = 219.70
	if got.TotalAmount != yuan("219.70") {
		t.Errorf("总额 = %s, 期望 219.70", got.TotalAmount)
	}
	if got.Status != model.StatusPending {
		t.Errorf("状态 = %s, 期望 pending", got.Status)
	}
	if len(got.Items) != 2 {
		t.Fatalf("明细数 = %d, 期望 2", len(got.Items))
	}
	if products.stock[1] != 8 || products.stock[2] != 4 {
		t.Errorf("库存 = %d/%d, 期望 8/4", products.stock[1], products.stock[2])
	}
}

// TestCreateMergeDuplicate 同一商品下两次，数量要合并成一条明细。
func TestCreateMergeDuplicate(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)

	got, err := svc.Create(context.Background(), 1, []service.ItemInput{
		{ProductID: 1, Quantity: 1},
		{ProductID: 1, Quantity: 2},
	})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("明细数 = %d, 期望 1（重复商品应合并）", len(got.Items))
	}
	if got.Items[0].Quantity != 3 {
		t.Errorf("数量 = %d, 期望 3", got.Items[0].Quantity)
	}
	if products.stock[1] != 7 {
		t.Errorf("库存 = %d, 期望 7", products.stock[1])
	}
}

// TestCreateOutOfStock 库存不足时直接返回错误，不能产生订单。
func TestCreateOutOfStock(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)

	_, err := svc.Create(context.Background(), 1, []service.ItemInput{{ProductID: 2, Quantity: 99}})
	if err == nil {
		t.Fatal("库存不足应该报错")
	}
	if got := apperr.From(err).Code; got != apperr.CodeStockNotEnough {
		t.Errorf("错误码 = %d, 期望 %d", got, apperr.CodeStockNotEnough)
	}
	if len(repo.orders) != 0 {
		t.Errorf("订单数 = %d, 期望 0", len(repo.orders))
	}
	if products.stock[2] != 5 {
		t.Errorf("库存 = %d, 期望 5（不足时不能扣）", products.stock[2])
	}
}

// TestCreateInvalidInput 数量非法要拦在参数校验阶段。
func TestCreateInvalidInput(t *testing.T) {
	svc := newService(newFakeRepo(), newFakeProducts())

	cases := []struct {
		name  string
		items []service.ItemInput
	}{
		{"空明细", nil},
		{"商品 ID 为 0", []service.ItemInput{{ProductID: 0, Quantity: 1}}},
		{"数量为 0", []service.ItemInput{{ProductID: 1, Quantity: 0}}},
		{"数量超上限", []service.ItemInput{{ProductID: 1, Quantity: 1000}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), 1, c.items); err == nil {
				t.Error("应该返回参数错误")
			}
		})
	}
}

// TestCreateWriteFailedRestoresStock 订单写库失败时，要把已经扣掉的库存还回去。
func TestCreateWriteFailedRestoresStock(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	repo.createOK = false
	svc := newService(repo, products)

	_, err := svc.Create(context.Background(), 1, []service.ItemInput{{ProductID: 1, Quantity: 3}})
	if err == nil {
		t.Fatal("写库失败应该报错")
	}
	if products.restoreCt != 1 {
		t.Errorf("Restore 调用次数 = %d, 期望 1", products.restoreCt)
	}
	if products.stock[1] != 10 {
		t.Errorf("库存 = %d, 期望回到 10", products.stock[1])
	}
}

// TestPay 待支付订单可以支付，重复支付会被拒绝。
func TestPay(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)
	ctx := context.Background()

	created, err := svc.Create(ctx, 1, []service.ItemInput{{ProductID: 1, Quantity: 1}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}

	paid, err := svc.Pay(ctx, 1, created.OrderNo)
	if err != nil {
		t.Fatalf("支付失败: %v", err)
	}
	if paid.Status != model.StatusPaid {
		t.Errorf("状态 = %s, 期望 paid", paid.Status)
	}

	// 第二次支付：状态已经不是 pending，必须失败
	if _, err := svc.Pay(ctx, 1, created.OrderNo); err == nil {
		t.Error("重复支付应该报错")
	}
}

// TestCancelRestoresStock 取消待支付订单后库存要恢复，已支付订单不能取消。
func TestCancelRestoresStock(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)
	ctx := context.Background()

	created, err := svc.Create(ctx, 1, []service.ItemInput{{ProductID: 1, Quantity: 2}})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if products.stock[1] != 8 {
		t.Fatalf("扣减后库存 = %d, 期望 8", products.stock[1])
	}

	cancelled, err := svc.Cancel(ctx, 1, created.OrderNo)
	if err != nil {
		t.Fatalf("取消失败: %v", err)
	}
	if cancelled.Status != model.StatusCancelled {
		t.Errorf("状态 = %s, 期望 cancelled", cancelled.Status)
	}
	if products.stock[1] != 10 {
		t.Errorf("取消后库存 = %d, 期望 10", products.stock[1])
	}

	// 已取消的订单不能再取消
	if _, err := svc.Cancel(ctx, 1, created.OrderNo); err == nil {
		t.Error("重复取消应该报错")
	}
}

// TestCancelPaidRejected 已支付订单不能取消，库存也不能被恢复。
func TestCancelPaidRejected(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)
	ctx := context.Background()

	created, _ := svc.Create(ctx, 1, []service.ItemInput{{ProductID: 1, Quantity: 1}})
	if _, err := svc.Pay(ctx, 1, created.OrderNo); err != nil {
		t.Fatalf("支付失败: %v", err)
	}
	if _, err := svc.Cancel(ctx, 1, created.OrderNo); err == nil {
		t.Fatal("已支付订单不能取消")
	}
	if products.stock[1] != 9 {
		t.Errorf("库存 = %d, 期望 9（已支付不应恢复）", products.stock[1])
	}
}

// TestGetOtherUserOrder 别人的订单查不到，不能泄露存在性。
func TestGetOtherUserOrder(t *testing.T) {
	repo, products := newFakeRepo(), newFakeProducts()
	svc := newService(repo, products)
	ctx := context.Background()

	created, _ := svc.Create(ctx, 1, []service.ItemInput{{ProductID: 1, Quantity: 1}})

	if _, err := svc.Get(ctx, 2, created.OrderNo); err == nil {
		t.Error("别人的订单应该查不到")
	}
	if _, err := svc.Get(ctx, 1, created.OrderNo); err != nil {
		t.Errorf("自己的订单应该能查到: %v", err)
	}
}

// TestAdminListInvalidStatus 管理端只接受三种合法状态。
func TestAdminListInvalidStatus(t *testing.T) {
	svc := newService(newFakeRepo(), newFakeProducts())

	if _, _, err := svc.AdminList(context.Background(), 1, 10, "unknown"); err == nil {
		t.Error("非法状态应该报错")
	}
	if _, _, err := svc.AdminList(context.Background(), 1, 10, model.StatusPaid); err != nil {
		t.Errorf("合法状态不该报错: %v", err)
	}
}
