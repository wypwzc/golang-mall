package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go-mall/pkg/config"
	"go-mall/pkg/database"
	"go-mall/pkg/jwt"
	"go-mall/pkg/logger"
	"go-mall/pkg/metrics"
	"go-mall/pkg/migrate"
	"go-mall/pkg/server"
	"go-mall/services/product/internal/handler"
	"go-mall/services/product/internal/repository"
	"go-mall/services/product/internal/router"
	"go-mall/services/product/internal/service"
	"go-mall/services/product/migrations"
)

func main() {
	cfg, err := config.Load("product")
	if err != nil {
		slog.Error("加载配置失败", "err", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel, cfg.Env)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	deps, err := database.Connect(ctx, cfg, log)
	if err != nil {
		log.Error("初始化依赖失败", "err", err)
		os.Exit(1)
	}
	defer deps.Close()

	if err := migrate.Run(deps.DB, migrations.FS, "."); err != nil {
		log.Error("数据库迁移失败", "err", err)
		os.Exit(1)
	}

	// 依赖装配：仓库 -> 服务 -> 处理器 -> 路由
	productRepo := repository.NewProductRepository(deps.DB)
	categoryRepo := repository.NewCategoryRepository(deps.DB)
	stockRepo := repository.NewStockRepository(deps.DB)
	svc := service.New(productRepo, categoryRepo, stockRepo)

	tokens := jwt.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	m := metrics.New()
	r := server.NewEngine(cfg, log, m, deps.Ready, tokens)
	router.Register(r, handler.New(svc), cfg.InternalToken)

	// 商品图片静态托管。目录不存在时先建出来，避免第一次访问就 404。
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		log.Error("创建图片目录失败", "err", err)
		os.Exit(1)
	}
	r.Static("/uploads", "uploads")

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Info("服务启动", "service", cfg.ServiceName, "addr", cfg.HTTPAddr, "env", cfg.Env)
	if err := server.Run(srv, log); err != nil {
		log.Error("服务退出", "err", err)
		os.Exit(1)
	}
}
