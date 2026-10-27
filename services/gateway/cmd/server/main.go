package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/config"
	"go-mall/pkg/jwt"
	"go-mall/pkg/logger"
	"go-mall/pkg/metrics"
	"go-mall/pkg/response"
	"go-mall/pkg/server"
	"go-mall/services/gateway/internal/proxy"
	"go-mall/services/gateway/internal/web"
)

// 网关自身不连数据库，只做路由与鉴权，因此没有 MySQL/Redis 初始化。
func main() {
	cfg, err := config.Load("gateway")
	if err != nil {
		slog.Error("加载配置失败", "err", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel, cfg.Env)

	tokens := jwt.New(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.TTL)
	m := metrics.New()

	// 网关无外部依赖，就绪检查恒通过
	ready := func(ctx context.Context) error { return nil }
	r := server.NewEngine(cfg, log, m, ready, tokens)

	// 反向代理：/api/v1/** 按前缀分发给各业务服务
	table, err := proxy.New(cfg.Upstreams, tokens, log)
	if err != nil {
		log.Error("初始化网关路由失败", "err", err)
		os.Exit(1)
	}
	r.Any("/api/v1", table.ServeHTTP)
	r.Any("/api/v1/*path", table.ServeHTTP)

	// 商品图片同样转发给商品服务，它负责把 uploads 目录里的文件读出来
	r.Any("/uploads/*path", table.ServeHTTP)

	// 前端静态文件。API 路径没匹配上仍然返回 JSON 404，其余交给静态文件处理器
	webHandler, err := web.Handler()
	if err != nil {
		log.Error("加载前端资源失败", "err", err)
		os.Exit(1)
	}
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/uploads/") {
			response.Error(c, apperr.NotFound.WithMessage("接口不存在: %s %s", c.Request.Method, p))
			return
		}
		webHandler.ServeHTTP(c.Writer, c.Request)
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second, // 比业务服务稍长，留出上游耗时
		IdleTimeout:       60 * time.Second,
	}

	log.Info("服务启动", "service", cfg.ServiceName, "addr", cfg.HTTPAddr, "env", cfg.Env)
	if err := server.Run(srv, log); err != nil {
		log.Error("服务退出", "err", err)
		os.Exit(1)
	}
}
