// Package server 封装 HTTP 服务的通用装配：中间件链、健康检查、优雅关闭。
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/config"
	"go-mall/pkg/jwt"
	"go-mall/pkg/metrics"
	"go-mall/pkg/middleware"
	"go-mall/pkg/response"
)

// NewEngine 组装带标准中间件的 Gin 引擎。
// ready 用于 /readyz：返回 nil 表示依赖就绪。
// tokens 用于直连场景下自行校验 JWT，可为 nil（由网关注入身份头时）。
func NewEngine(cfg *config.Config, log *slog.Logger, m *metrics.Metrics, ready func(context.Context) error, tokens *jwt.Manager) *gin.Engine {
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// 顺序很重要：先有 request_id，日志和指标才能带上它
	r.Use(
		middleware.RequestID(log),
		middleware.Recovery(log),
		middleware.AccessLog(log),
		m.GinMiddleware(cfg.ServiceName),
		middleware.CORS(cfg.CORSOrigins),
		middleware.Auth(tokens),
	)

	// 存活检查：只反映进程是否还在跑，不查依赖
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": cfg.ServiceName})
	})

	// 就绪检查：依赖不可用时返回 503，让编排系统暂时不要打流量进来
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if ready != nil {
			if err := ready(ctx); err != nil {
				log.Warn("就绪检查未通过", "err", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "service": cfg.ServiceName})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "service": cfg.ServiceName})
	})

	r.GET("/metrics", gin.WrapH(m.Handler()))

	// 未匹配到路由时也返回统一格式，避免前端拿到纯文本 404
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) {
		response.Error(c, apperr.NotFound.WithMessage("接口不存在: %s %s", c.Request.Method, c.Request.URL.Path))
	})
	r.NoMethod(func(c *gin.Context) {
		response.Error(c, apperr.MethodNotAllowed.WithMessage("方法不被支持: %s %s", c.Request.Method, c.Request.URL.Path))
	})

	return r
}

// Run 启动 HTTP 服务并处理优雅关闭。
func Run(srv *http.Server, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Info("收到退出信号，开始优雅关闭")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
