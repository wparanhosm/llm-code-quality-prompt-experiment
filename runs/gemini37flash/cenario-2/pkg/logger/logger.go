package logger

import (
	"context"
	"log/slog"
	"os"
)

type correlationIDKeyType struct{}

// CorrelationIDKey is the context key for tracing/request ID
var CorrelationIDKey = correlationIDKeyType{}

// ContextHandler wraps an slog.Handler to inject context values (like correlation_id) into log records.
type ContextHandler struct {
	slog.Handler
}

// Handle appends context correlation_id if present.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		if correlationID, ok := ctx.Value(CorrelationIDKey).(string); ok && correlationID != "" {
			r.AddAttrs(slog.String("correlation_id", correlationID))
		}
	}
	return h.Handler.Handle(ctx, r)
}

// WithCorrelationID stores correlation ID in context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetCorrelationID extracts correlation ID from context.
func GetCorrelationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if val, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return val
	}
	return ""
}

// New creates a structured JSON logger with context enrichment.
func New() *slog.Logger {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler := &ContextHandler{Handler: baseHandler}
	return slog.New(handler)
}
