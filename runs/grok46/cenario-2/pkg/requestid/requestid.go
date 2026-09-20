package requestid

import "context"

type ctxKey struct{}

// FromContext returns the request id stored by HTTP middleware, if any.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// WithValue stores a request id. Used by tests and adapters that need a portable key.
func WithValue(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}
