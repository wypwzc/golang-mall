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
	"go-mall/pkg/ratelimit"
	"go-mall/pkg/server"
	"go-mall/services/user/internal/handler"
	"go-mall/services/user/internal/repository"
	"go-mall/services/user/internal/router"
	"go-mall/services/user/internal/service"
	"go-mall/services/user/migrations"
)

func main() {
	cfg, err := config.Load("user")
	if err != nil {
		slog.Error("加载配置失败", "err", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel, cfg.Env)

	// 启动阶段最多等 60 秒，等 MySQL / Redis 就绪（容器编排下数据库往往比业务慢）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	deps, err := database.Connect(ctx, cfg, log)
	if err != nil {
		log.Error("初始化依赖失败", "err", err)
		os.Exit(1)
	}
	defer deps.Close()

	// 建表脚本随二进制一起发布，启动时自动执行，已执行过的不会重复跑
	if err := migrate.Run(deps.DB, migrations.FS, "."); err != nil {
		log.Error("数据库迁移失败", "err", err)
		os.Exit(1)
	}

	// 依赖装配：仓库 -> 服务 -> 处理器 -> 路由
	tokens := jwt.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	limiter := newLimiter(cfg, deps, log)
	repo := repository.NewUserRepository(deps.DB)
	svc := service.New(repo, tokens, limiter, log)

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

// newLimiter 有 Redis 就用 Redis（多实例共享计数），没有就退化为进程内计数。
func newLimiter(cfg *config.Config, deps *database.Deps, log *slog.Logger) ratelimit.Limiter {
	c := ratelimit.Config{Limit: cfg.LoginMaxFails, Window: cfg.LoginFailWindow}
	if deps.Redis != nil {
		log.Info("登录限流使用 Redis 实现")
		return ratelimit.NewRedis(deps.Redis, c)
	}
	log.Info("登录限流使用内存实现（仅单实例有效）")
	return ratelimit.NewMemory(c)
}
