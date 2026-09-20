package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/walter/p2p-wallet-transfer/pkg/logger"
)

func TestContextHandler_CorrelationID(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	baseHandler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	handler := &logger.ContextHandler{Handler: baseHandler}
	l := slog.New(handler)

	// Context with correlation ID
	ctx := logger.WithCorrelationID(context.Background(), "trace-xyz-123")
	l.InfoContext(ctx, "test message")

	var logData map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logData); err != nil {
		t.Fatalf("failed unmarshaling log output: %v", err)
	}

	if logData["correlation_id"] != "trace-xyz-123" {
		t.Errorf("expected correlation_id 'trace-xyz-123', got %v", logData["correlation_id"])
	}

	if logData["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got %v", logData["msg"])
	}

	// Helper tests
	cid := logger.GetCorrelationID(ctx)
	if cid != "trace-xyz-123" {
		t.Errorf("expected 'trace-xyz-123', got %s", cid)
	}

	emptyCid := logger.GetCorrelationID(context.Background())
	if emptyCid != "" {
		t.Errorf("expected empty string, got %s", emptyCid)
	}

	emptyNilCtx := logger.GetCorrelationID(nil)
	if emptyNilCtx != "" {
		t.Errorf("expected empty string for nil context, got %s", emptyNilCtx)
	}
}

func TestNewLogger(t *testing.T) {
	t.Parallel()
	l := logger.New()
	if l == nil {
		t.Fatalf("expected non-nil logger")
	}
}
