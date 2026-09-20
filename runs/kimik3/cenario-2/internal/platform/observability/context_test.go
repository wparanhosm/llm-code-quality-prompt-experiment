package observability_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"wallet-p2p/internal/platform/observability"
)

func TestContextWithRequestID_RoundTrip(t *testing.T) {
	t.Parallel()

	ctx := observability.ContextWithRequestID(context.Background(), "req-123")
	assert.Equal(t, "req-123", observability.RequestIDFromContext(ctx))
}

func TestRequestIDFromContext_Absent(t *testing.T) {
	t.Parallel()

	assert.Empty(t, observability.RequestIDFromContext(context.Background()))
}

func TestRequestIDFromContext_WrongType(t *testing.T) {
	t.Parallel()

	// Um valor de tipo diferente sob a mesma chave não deve causar pânico.
	type otherKey string
	ctx := context.WithValue(context.Background(), otherKey("request_id"), 42)
	assert.Empty(t, observability.RequestIDFromContext(ctx))
}

func TestLogAttrs_WithRequestID(t *testing.T) {
	t.Parallel()

	ctx := observability.ContextWithRequestID(context.Background(), "req-abc")
	attrs := observability.LogAttrs(ctx)
	assert.Len(t, attrs, 1)
}

func TestLogAttrs_WithoutRequestID(t *testing.T) {
	t.Parallel()

	assert.Empty(t, observability.LogAttrs(context.Background()))
}

func TestLogAttrs_CompatibleWithSlog(t *testing.T) {
	t.Parallel()

	ctx := observability.ContextWithRequestID(context.Background(), "req-xyz")
	// Garante que os atributos podem ser usados diretamente com slog.
	logger := slog.Default()
	logger.InfoContext(ctx, "teste", observability.LogAttrs(ctx)...)
}
