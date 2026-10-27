package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/middleware"
)

func newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Auth(nil))
	r.GET("/me", middleware.RequireLogin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": middleware.UserID(c), "role": middleware.Role(c)})
	})
	r.GET("/admin", middleware.RequireRole(middleware.RoleAdmin), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/internal", middleware.RequireInternalToken("secret-token"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func do(r *gin.Engine, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequireLoginRejectsAnonymous(t *testing.T) {
	w := do(newRouter(), "/me", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录应返回 401，实际 %d", w.Code)
	}
}

func TestRequireRoleRejectsNormalUser(t *testing.T) {
	w := do(newRouter(), "/admin", map[string]string{
		middleware.HeaderUserID:   "2",
		middleware.HeaderUserRole: middleware.RoleUser,
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("普通用户访问管理接口应返回 403，实际 %d", w.Code)
	}
}

func TestRequireRoleAllowsAdmin(t *testing.T) {
	w := do(newRouter(), "/admin", map[string]string{
		middleware.HeaderUserID:   "1",
		middleware.HeaderUserRole: middleware.RoleAdmin,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("管理员应通过，实际 %d", w.Code)
	}
}

func TestRequireInternalToken(t *testing.T) {
	r := newRouter()

	if w := do(r, "/internal", nil); w.Code != http.StatusForbidden {
		t.Fatalf("缺少内部令牌应返回 403，实际 %d", w.Code)
	}
	if w := do(r, "/internal", map[string]string{middleware.HeaderInternalToken: "wrong"}); w.Code != http.StatusForbidden {
		t.Fatalf("错误令牌应返回 403，实际 %d", w.Code)
	}
	if w := do(r, "/internal", map[string]string{middleware.HeaderInternalToken: "secret-token"}); w.Code != http.StatusOK {
		t.Fatalf("正确令牌应通过，实际 %d", w.Code)
	}
}

func TestAuthIgnoresInvalidUserID(t *testing.T) {
	w := do(newRouter(), "/me", map[string]string{
		middleware.HeaderUserID: "not-a-number",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("非法用户 ID 应视为未登录，实际 %d", w.Code)
	}
}
