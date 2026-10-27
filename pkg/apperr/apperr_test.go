package apperr_test

import (
	"errors"
	"testing"

	"go-mall/pkg/apperr"
)

func TestFromUnknownError(t *testing.T) {
	e := apperr.From(errors.New("database down"))
	if e == nil {
		t.Fatal("From 不应返回 nil")
	}
	if e.Code != apperr.CodeInternal {
		t.Fatalf("未知错误应转成 %d，实际 %d", apperr.CodeInternal, e.Code)
	}
	// 原始错误应保留在 cause 里，供日志排查
	if !errors.Is(e, e.Unwrap()) {
		t.Fatal("应保留原始错误")
	}
}

func TestFromNil(t *testing.T) {
	if e := apperr.From(nil); e != nil {
		t.Fatal("nil 应返回 nil")
	}
}

func TestWithMessageDoesNotMutateTemplate(t *testing.T) {
	custom := apperr.InvalidParam.WithMessage("商品名称不能为空")
	if custom.Message != "商品名称不能为空" {
		t.Fatalf("自定义文案不生效: %s", custom.Message)
	}
	// 模板本身不能被改掉，否则并发场景下会互相污染
	if apperr.InvalidParam.Message != "参数校验失败" {
		t.Fatalf("模板被意外修改: %s", apperr.InvalidParam.Message)
	}
	if custom.Code != apperr.CodeInvalidParam {
		t.Fatalf("错误码应保持不变")
	}
}

func TestWrappedErrorStillRecognized(t *testing.T) {
	inner := apperr.StockNotEnough.WithCause(errors.New("row affected 0"))
	wrapped := errors.Join(errors.New("order create failed"), inner)

	e := apperr.From(wrapped)
	if e.Code != apperr.CodeStockNotEnough {
		t.Fatalf("应识别出库存不足错误，实际 %d", e.Code)
	}
}
