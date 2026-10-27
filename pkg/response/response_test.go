package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/response"
)

func newContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func TestOKShape(t *testing.T) {
	c, w := newContext()
	response.OK(c, gin.H{"id": 1})

	if w.Code != http.StatusOK {
		t.Fatalf("状态码应为 200，实际 %d", w.Code)
	}

	var body response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if body.Code != 0 || body.Message != "success" {
		t.Fatalf("成功响应格式不对: %+v", body)
	}
}

func TestErrorShape(t *testing.T) {
	c, w := newContext()
	response.Error(c, apperr.StockNotEnough.WithMessage("库存不足，仅剩 2 件"))

	if w.Code != http.StatusConflict {
		t.Fatalf("库存不足应返回 409，实际 %d", w.Code)
	}

	var body response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if body.Code != int(apperr.CodeStockNotEnough) {
		t.Fatalf("业务码应为 %d，实际 %d", apperr.CodeStockNotEnough, body.Code)
	}
	if body.Message != "库存不足，仅剩 2 件" {
		t.Fatalf("错误文案不对: %s", body.Message)
	}
	if body.Data != nil {
		t.Fatal("错误响应的 data 必须为 null")
	}
}

func TestPageShape(t *testing.T) {
	c, w := newContext()
	response.Page(c, []string{"a", "b"}, 42, 2, 10)

	var body struct {
		Code int               `json:"code"`
		Data response.PageData `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if body.Data.Total != 42 || body.Data.Page != 2 || body.Data.PageSize != 10 {
		t.Fatalf("分页字段不对: %+v", body.Data)
	}
}
