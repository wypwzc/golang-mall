// Package metrics 提供 Prometheus 指标。
//
// 只暴露三类基础指标：请求总数、请求耗时、错误数。
// 使用独立 Registry 而不是默认全局 Registry，避免测试之间互相污染。
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics 持有本服务的指标集合。
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	errors   *prometheus.CounterVec
}

// New 创建指标集合并注册 Go 运行时指标。
func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "HTTP 请求总数",
		}, []string{"service", "method", "path", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP 请求耗时（秒）",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"service", "method", "path"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "HTTP 5xx 错误数",
		}, []string{"service", "path"}),
	}

	reg.MustRegister(m.requests, m.duration, m.errors)
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// GinMiddleware 记录每次请求的指标。
// path 使用 c.FullPath()（路由模板），避免把订单号之类的变量写进标签导致基数爆炸。
func (m *Metrics) GinMiddleware(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())

		m.requests.WithLabelValues(service, c.Request.Method, path, status).Inc()
		m.duration.WithLabelValues(service, c.Request.Method, path).Observe(time.Since(start).Seconds())
		if c.Writer.Status() >= http.StatusInternalServerError {
			m.errors.WithLabelValues(service, path).Inc()
		}
	}
}

// Handler 返回 /metrics 的 http.Handler。
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
