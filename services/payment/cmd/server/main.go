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
	"go-mall/pkg/server"
)

func main() {
	cfg, err := config.Load("payment")
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

	tokens := jwt.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	m := metrics.New()
	r := server.NewEngine(cfg, log, m, deps.Ready, tokens)

	// 阶段七在这里注册：/api/v1/payments/**

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
