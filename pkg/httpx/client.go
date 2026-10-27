// Package httpx 是服务间调用的 HTTP 客户端。
//
// 统一做三件事：带上内部令牌、透传 request_id、把对端的错误响应还原成 *apperr.Error。
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-mall/pkg/apperr"
	"go-mall/pkg/middleware"
	"go-mall/pkg/requestid"
)

// Client 指向某个上游服务的客户端，应在服务启动时创建一次并复用。
type Client struct {
	baseURL string
	token   string
	hc      *http.Client
}

// New 创建客户端。timeout 建议 3~5 秒，避免上游卡死拖垮自己。
func New(baseURL, internalToken string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   internalToken,
		hc:      &http.Client{Timeout: timeout},
	}
}

// apiResponse 与 pkg/response.Body 对应。
type apiResponse struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	Data      json.RawMessage `json:"data"`
	RequestID string          `json:"request_id"`
}

// Do 发起一次调用。in 为请求体（可为 nil），out 为解析目标（可为 nil）。
func (c *Client) Do(ctx context.Context, method, path string, in, out any) error {
	var reader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return apperr.Internal.WithCause(err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return apperr.Internal.WithCause(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.HeaderInternalToken, c.token)
	if rid := requestid.FromContext(ctx); rid != "" {
		req.Header.Set(requestid.Header, rid)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return apperr.Upstream.WithCause(fmt.Errorf("%s %s: %w", method, path, err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apperr.Upstream.WithCause(err)
	}

	var parsed apiResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &parsed); err != nil {
			return apperr.Upstream.WithCause(fmt.Errorf("解析上游响应失败: %w", err))
		}
	}

	if resp.StatusCode >= http.StatusBadRequest {
		msg := parsed.Message
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return apperr.New(apperr.Code(parsed.Code), resp.StatusCode, msg)
	}

	if out != nil && len(parsed.Data) > 0 {
		if err := json.Unmarshal(parsed.Data, out); err != nil {
			return apperr.Internal.WithCause(err)
		}
	}
	return nil
}

// Get 便捷方法。
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, nil, out)
}

// Post 便捷方法。
func (c *Client) Post(ctx context.Context, path string, in, out any) error {
	return c.Do(ctx, http.MethodPost, path, in, out)
}
