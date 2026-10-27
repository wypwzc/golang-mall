// Package proxy 网关的反向代理：按路径前缀把请求转发给对应的业务服务。
//
// 网关只做三件事：
//  1. 按前缀分发（不重写路径，业务服务看到的还是原始 URL）；
//  2. 校验 JWT 后把用户身份写进 X-User-Id / X-User-Role，业务服务直接信任；
//  3. 上游挂了返回统一格式的 503，避免前端拿到纯文本。
package proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/jwt"
	"go-mall/pkg/middleware"
	"go-mall/pkg/requestid"
	"go-mall/pkg/response"
)

// route 一条转发规则。
type route struct {
	prefix  string // 路径前缀
	service string // 目标服务名，只用于日志
	proxy   *httputil.ReverseProxy
}

// Table 路由表。
//
// 匹配规则是"最长前缀优先"，所以 /api/v1/admin/orders 一定走订单服务，
// 不会被更短的 /api/v1/orders 抢走。
type Table struct {
	routes []*route
	tokens *jwt.Manager // 网关自己验签，不用中间件解析出来的身份
	log    *slog.Logger
}

// rules 前缀到服务名的映射。新增接口时在这里补一行即可。
//
// 注意 /api/v1/internal 不在这里：内部接口只允许服务之间直连，
// 不通过网关暴露出去。
var rules = []struct {
	prefix  string
	service string
}{
	{"/api/v1/auth", "user"},
	{"/api/v1/users", "user"},
	{"/api/v1/admin/users", "user"},
	{"/api/v1/categories", "product"},
	{"/api/v1/products", "product"},
	{"/api/v1/admin/categories", "product"},
	{"/api/v1/admin/products", "product"},
	{"/api/v1/admin/uploads", "product"},
	{"/api/v1/cart", "cart"},
	{"/api/v1/orders", "order"},
	{"/api/v1/admin/orders", "order"},

	// 商品图片也是商品服务在托管
	{"/uploads", "product"},
}

// New 根据上游地址表创建路由表。upstreams 的 key 是服务名，value 形如 http://127.0.0.1:8081。
func New(upstreams map[string]string, tokens *jwt.Manager, log *slog.Logger) (*Table, error) {
	t := &Table{tokens: tokens, log: log}

	for _, r := range rules {
		base, ok := upstreams[r.service]
		if !ok || base == "" {
			return nil, fmt.Errorf("缺少上游配置: %s", r.service)
		}
		target, err := url.Parse(base)
		if err != nil {
			return nil, fmt.Errorf("上游地址不合法 %s=%s: %w", r.service, base, err)
		}
		t.routes = append(t.routes, &route{
			prefix:  r.prefix,
			service: r.service,
			proxy:   newReverseProxy(target, r.service, log),
		})
	}

	// 长前缀排前面，保证先匹配到更具体的规则
	sort.Slice(t.routes, func(i, j int) bool {
		return len(t.routes[i].prefix) > len(t.routes[j].prefix)
	})
	return t, nil
}

// newReverseProxy 构造单个反向代理。
func newReverseProxy(target *url.URL, service string, log *slog.Logger) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		// 只改目标地址，路径原样保留
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = target.Host
		},
		// 上游返回什么就透传什么，不做二次包装
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("X-Upstream-Service", service)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// 连不上上游（服务没启动、端口不通、超时）
			log.Error("转发失败", "service", service, "path", r.URL.Path, "err", err)
			writeJSON(w, http.StatusServiceUnavailable, response.Body{
				Code:      int(apperr.CodeUpstream),
				Message:   fmt.Sprintf("依赖服务不可用: %s", service),
				Data:      nil,
				RequestID: r.Header.Get(requestid.Header),
			})
		},
		FlushInterval: 100 * time.Millisecond,
	}
}

// ServeHTTP 实现 gin.HandlerFunc，挂在 /api/v1/*path 上。
func (t *Table) ServeHTTP(c *gin.Context) {
	path := c.Request.URL.Path

	rt := t.match(path)
	if rt == nil {
		response.Error(c, apperr.NotFound.WithMessage("接口不存在: %s %s", c.Request.Method, path))
		return
	}

	// 身份头只能由网关自己生成。先删掉客户端可能伪造的值，
	// 否则任何人只要加一个 X-User-Id: 1 就能冒充管理员。
	h := c.Request.Header
	h.Del(middleware.HeaderUserID)
	h.Del(middleware.HeaderUserRole)
	h.Del("X-Internal-Token")

	// 只认 Authorization 里的 JWT。不能直接用 middleware.UserID(c)，
	// 因为 Auth 中间件会优先信任请求里已有的 X-User-Id，那正好是客户端伪造的。
	if uid, role, ok := t.identity(c.Request); ok {
		h.Set(middleware.HeaderUserID, strconv.FormatInt(uid, 10))
		h.Set(middleware.HeaderUserRole, role)
	}

	// 让业务服务的访问日志能和网关日志对上
	if rid := c.GetString(requestid.ContextKey); rid != "" {
		h.Set(requestid.Header, rid)
	}

	rt.proxy.ServeHTTP(c.Writer, c.Request)
}

// identity 从 Authorization: Bearer <JWT> 里解析用户身份。
// 没有 token、格式不对、验签失败都当作未登录，交给业务服务的 RequireLogin 处理。
func (t *Table) identity(r *http.Request) (int64, string, bool) {
	if t.tokens == nil {
		return 0, "", false
	}
	raw := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(raw) <= len(prefix) || !strings.EqualFold(raw[:len(prefix)], prefix) {
		return 0, "", false
	}
	claims, err := t.tokens.Parse(strings.TrimSpace(raw[len(prefix):]))
	if err != nil {
		return 0, "", false
	}
	return claims.UserID, claims.Role, true
}

// match 找最长匹配前缀。前缀按整段匹配，避免 /api/v1/products 误伤 /api/v1/productsX。
func (t *Table) match(path string) *route {
	for _, r := range t.routes {
		if path == r.prefix || strings.HasPrefix(path, r.prefix+"/") {
			return r
		}
	}
	return nil
}

// writeJSON 直接往 ResponseWriter 写统一格式，用于代理层出错时（此时没有 gin.Context 可用）。
func writeJSON(w http.ResponseWriter, status int, body response.Body) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
