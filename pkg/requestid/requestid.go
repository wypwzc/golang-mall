// Package requestid 统一请求 ID 的头名称与上下文 key，
// 避免 middleware 与 response 互相 import 造成循环依赖。
package requestid

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	// Header 网关与业务服务之间传递请求 ID 的头。
	Header = "X-Request-Id"
	// ContextKey gin.Context 中保存请求 ID 的 key。
	ContextKey = "request_id"
)

// New 生成一个 16 字节的随机 ID，不依赖第三方 uuid 库。
func New() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
