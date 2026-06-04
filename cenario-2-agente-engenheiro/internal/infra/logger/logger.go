package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey string

const correlationKey ctxKey = "correlation_id"

func New(level slog.Level) *slog.Logger {
	return slog.New(newHandler(level))
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey, id)
}

func CorrelationIDFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(correlationKey).(string)
	return v
}

type handler struct {
	inner slog.Handler
}

func newHandler(level slog.Level) *handler {
	return &handler{
		inner: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if cid := CorrelationIDFromCtx(ctx); cid != "" {
		r.AddAttrs(slog.String("correlation_id", cid))
	}
	return h.inner.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{inner: h.inner.WithAttrs(attrs)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{inner: h.inner.WithGroup(name)}
}
