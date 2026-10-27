// 购物车服务入口：装配依赖并启动 HTTP 服务。
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
	"go-mall/services/cart/internal/client"
	"go-mall/services/cart/internal/handler"
	"go-mall/services/cart/internal/repository"
	"go-mall/services/cart/internal/router"
	"go-mall/services/cart/internal/service"
	"go-mall/services/cart/migrations"
)

func main() {
	cfg, err := config.Load("cart")
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

	productClient := client.NewProductClient(cfg.Upstreams["product"], cfg.InternalToken, 3*time.Second)
	repo := repository.NewCartRepository(deps.DB)
	svc := service.New(repo, productClient)

	tokens := jwt.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	m := metrics.New()
	r := server.NewEngine(cfg, log, m, deps.Ready, tokens)
	router.Register(r, handler.New(svc))

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
