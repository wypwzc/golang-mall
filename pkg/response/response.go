// Package response 统一 HTTP 响应格式。
//
// 成功：{"code":0,"message":"success","data":...}
// 失败：{"code":40001,"message":"...","data":null,"request_id":"..."}
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-mall/pkg/apperr"
	"go-mall/pkg/requestid"
)

// Body 统一响应体。
type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id,omitempty"`
}

// PageData 分页数据。所有列表接口都用这个结构，前端只写一套分页逻辑。
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// OK 返回 200 成功。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data, RequestID: requestID(c)})
}

// Created 返回 201，用于创建资源成功。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{Code: 0, Message: "success", Data: data, RequestID: requestID(c)})
}

// Error 把 error 转成统一错误响应。未知错误统一变成 50001，不泄漏内部细节。
func Error(c *gin.Context, err error) {
	e := apperr.From(err)
	if e == nil {
		OK(c, nil)
		return
	}
	c.AbortWithStatusJSON(e.Status, Body{Code: int(e.Code), Message: e.Message, Data: nil, RequestID: requestID(c)})
}

// Page 返回分页数据。
func Page(c *gin.Context, list any, total int64, page, pageSize int) {
	OK(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

func requestID(c *gin.Context) string {
	return c.GetString(requestid.ContextKey)
}
