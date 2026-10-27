package repository_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"go-mall/pkg/migrate"
	"go-mall/pkg/money"
	"go-mall/services/product/internal/model"
	"go-mall/services/product/internal/repository"
	"go-mall/services/product/migrations"
)

// 这些测试需要真实 MySQL（InnoDB 行锁是并发正确性的前提，mock 不出来）。
//
// 运行方式：
//
//	TEST_MYSQL_DSN="mall_product:product_dev_2026@tcp(127.0.0.1:3306)/mall_product?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true" \
//	  go test ./services/product/internal/repository/ -run TestStock -v
//
// 未设置 DSN 时自动跳过，保证 go test ./... 在没有数据库的环境也能通过。
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("未设置 TEST_MYSQL_DSN，跳过需要 MySQL 的集成测试")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:  gormlogger.Default.LogMode(gormlogger.Silent),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	if err := migrate.Run(db, migrations.FS, "."); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

// createProduct 造一个测试商品，返回 ID，测试结束自动删除。
func createProduct(t *testing.T, db *gorm.DB, stock int) int64 {
	t.Helper()

	p := &model.Product{
		CategoryID: 1,
		Name:       fmt.Sprintf("测试商品-%d", time.Now().UnixNano()),
		Price:      money.Cents(1000),
		Stock:      stock,
		Status:     model.StatusOnSale,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("创建测试商品失败: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM stock_deductions WHERE product_id = ?", p.ID)
		db.Exec("DELETE FROM products WHERE id = ?", p.ID)
	})
	return p.ID
}

func currentStock(t *testing.T, db *gorm.DB, id int64) int {
	t.Helper()
	var p model.Product
	if err := db.Where("id = ?", id).First(&p).Error; err != nil {
		t.Fatalf("查询商品失败: %v", err)
	}
	return p.Stock
}

// TestStockDeductNoOversell 是整套系统最关键的一条测试：
// 100 个并发请求抢 10 件库存，必须恰好成功 10 个，库存归零且不能为负。
func TestStockDeductNoOversell(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStockRepository(db)

	productID := createProduct(t, db, 10)

	const goroutines = 100
	var success, failed int64

	var wg sync.WaitGroup
	start := make(chan struct{}) // 让所有 goroutine 尽量同时起跑

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start

			orderNo := fmt.Sprintf("T%d-%d", time.Now().UnixNano(), n)
			_, err := repo.Deduct(context.Background(), orderNo, []repository.StockItem{
				{ProductID: productID, Quantity: 1},
			})
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			atomic.AddInt64(&success, 1)
		}(i)
	}

	close(start)
	wg.Wait()

	if success != 10 {
		t.Fatalf("库存 10 件，应恰好成功 10 次，实际成功 %d 次（失败 %d 次）", success, failed)
	}
	if got := currentStock(t, db, productID); got != 0 {
		t.Fatalf("扣减后库存应为 0，实际 %d", got)
	}
}

// TestStockDeductIdempotent 同一订单号重复扣减只能生效一次。
func TestStockDeductIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStockRepository(db)

	productID := createProduct(t, db, 5)
	orderNo := fmt.Sprintf("IDEM-%d", time.Now().UnixNano())
	items := []repository.StockItem{{ProductID: productID, Quantity: 2}}

	for i := 0; i < 3; i++ {
		if _, err := repo.Deduct(context.Background(), orderNo, items); err != nil {
			t.Fatalf("第 %d 次扣减失败: %v", i+1, err)
		}
	}

	if got := currentStock(t, db, productID); got != 3 {
		t.Fatalf("重复扣减应只生效一次，库存应为 3，实际 %d", got)
	}
}

// TestStockRestoreIdempotent 取消订单恢复库存，重复调用不能重复加库存。
func TestStockRestoreIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStockRepository(db)

	productID := createProduct(t, db, 10)
	orderNo := fmt.Sprintf("RESTORE-%d", time.Now().UnixNano())

	if _, err := repo.Deduct(context.Background(), orderNo, []repository.StockItem{
		{ProductID: productID, Quantity: 4},
	}); err != nil {
		t.Fatalf("扣减失败: %v", err)
	}
	if got := currentStock(t, db, productID); got != 6 {
		t.Fatalf("扣减后应为 6，实际 %d", got)
	}

	for i := 0; i < 3; i++ {
		if err := repo.Restore(context.Background(), orderNo); err != nil {
			t.Fatalf("第 %d 次恢复失败: %v", i+1, err)
		}
	}

	if got := currentStock(t, db, productID); got != 10 {
		t.Fatalf("重复恢复应只生效一次，库存应回到 10，实际 %d", got)
	}
}

// TestStockDeductInsufficient 库存不足时整单失败，不能部分扣减。
func TestStockDeductInsufficient(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStockRepository(db)

	enough := createProduct(t, db, 10)
	scarce := createProduct(t, db, 1)

	orderNo := fmt.Sprintf("INSUFF-%d", time.Now().UnixNano())
	_, err := repo.Deduct(context.Background(), orderNo, []repository.StockItem{
		{ProductID: enough, Quantity: 2},
		{ProductID: scarce, Quantity: 5},
	})
	if err == nil {
		t.Fatal("库存不足必须报错")
	}

	// 关键：第一个商品也不能被扣掉
	if got := currentStock(t, db, enough); got != 10 {
		t.Fatalf("事务应整体回滚，库存应仍为 10，实际 %d", got)
	}
}

// TestStockDeductOffShelf 下架商品不能下单。
func TestStockDeductOffShelf(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewStockRepository(db)

	productID := createProduct(t, db, 10)
	db.Model(&model.Product{}).Where("id = ?", productID).Update("status", model.StatusOffShelf)

	_, err := repo.Deduct(context.Background(), fmt.Sprintf("OFF-%d", time.Now().UnixNano()), []repository.StockItem{
		{ProductID: productID, Quantity: 1},
	})
	if err == nil {
		t.Fatal("下架商品必须拒绝下单")
	}
	if got := currentStock(t, db, productID); got != 10 {
		t.Fatalf("下架商品库存不应变化，实际 %d", got)
	}
}
