package requestid

import "context"

type ctxKey struct{}

// WithContext 把请求 ID 放进 context，供服务间调用透传。
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext 取出请求 ID，没有则返回空字符串。
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(ctxKey{}).(string); ok {
		return id
	}
	return ""
}
