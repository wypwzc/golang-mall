// Package apperr 定义统一的业务错误。
//
// 设计要点：
//  1. 业务错误码（Code）与 HTTP 状态码分离，前端只看 Code，网关只看 HTTP Status；
//  2. 预定义错误是"模板"，WithMessage/WithCause 返回副本，避免并发修改同一实例；
//  3. 任何 error 都能通过 From 转成 *Error，保证响应格式永远统一。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code 业务错误码。0 表示成功，其余按 4xx/5xx 语义分段。
type Code int

const (
	CodeOK                 Code = 0
	CodeInvalidParam       Code = 40001 // 参数校验失败
	CodeStockNotEnough     Code = 40002 // 库存不足
	CodeOrderStatusInvalid Code = 40003 // 订单状态不允许该操作
	CodeUnauthorized       Code = 40101 // 未登录
	CodeTokenInvalid       Code = 40102 // Token 无效或过期
	CodeForbidden          Code = 40301 // 无权限
	CodeNotFound           Code = 40401 // 资源不存在
	CodeMethodNotAllowed   Code = 40501 // 请求方法不支持
	CodeConflict           Code = 40901 // 冲突（重复提交等）
	CodeTooManyRequests    Code = 42901 // 触发限流
	CodeInternal           Code = 50001 // 服务内部错误
	CodeUpstream           Code = 50301 // 依赖服务不可用
)

// Error 业务错误。实现 error 接口，可被 errors.As 识别。
type Error struct {
	Code    Code
	Status  int    // 对应 HTTP 状态码
	Message string // 可直接返回给前端的提示
	cause   error  // 原始错误，只用于日志，不返回给前端
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

// Unwrap 支持 errors.Is / errors.As 向下查找。
func (e *Error) Unwrap() error { return e.cause }

// WithMessage 返回一个改了提示文案的副本，原对象不变。
func (e *Error) WithMessage(format string, args ...any) *Error {
	cp := *e
	cp.Message = fmt.Sprintf(format, args...)
	return &cp
}

// WithCause 返回一个带原始错误的副本。
func (e *Error) WithCause(err error) *Error {
	cp := *e
	cp.cause = err
	return &cp
}

// New 构造一个业务错误。
func New(code Code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// 预定义错误模板，使用时通常配合 WithMessage / WithCause。
var (
	InvalidParam       = New(CodeInvalidParam, http.StatusBadRequest, "参数校验失败")
	StockNotEnough     = New(CodeStockNotEnough, http.StatusConflict, "商品库存不足")
	OrderStatusInvalid = New(CodeOrderStatusInvalid, http.StatusConflict, "订单状态不允许该操作")
	Unauthorized       = New(CodeUnauthorized, http.StatusUnauthorized, "请先登录")
	TokenInvalid       = New(CodeTokenInvalid, http.StatusUnauthorized, "登录已过期，请重新登录")
	Forbidden          = New(CodeForbidden, http.StatusForbidden, "无权限访问")
	NotFound           = New(CodeNotFound, http.StatusNotFound, "资源不存在")
	MethodNotAllowed   = New(CodeMethodNotAllowed, http.StatusMethodNotAllowed, "请求方法不被支持")
	Conflict           = New(CodeConflict, http.StatusConflict, "操作冲突，请勿重复提交")
	TooManyRequests    = New(CodeTooManyRequests, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
	Internal           = New(CodeInternal, http.StatusInternalServerError, "服务内部错误")
	Upstream           = New(CodeUpstream, http.StatusServiceUnavailable, "依赖服务不可用")
)

// From 把任意 error 归一化为 *Error。未知错误一律变成 50001，避免泄漏内部细节。
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal.WithCause(err)
}
