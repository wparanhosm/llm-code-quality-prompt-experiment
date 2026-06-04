package logger_test

import (
	"context"
	"log/slog"
	"testing"

	"p2p-wallet/internal/infra/logger"
)

func TestNew(t *testing.T) {
	l := logger.New(slog.LevelInfo)
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestWithCorrelationID(t *testing.T) {
	ctx := logger.WithCorrelationID(context.Background(), "abc-123")
	got := logger.CorrelationIDFromCtx(ctx)
	if got != "abc-123" {
		t.Fatalf("expected abc-123, got %s", got)
	}
}

func TestCorrelationIDFromCtx_Empty(t *testing.T) {
	got := logger.CorrelationIDFromCtx(context.Background())
	if got != "" {
		t.Fatalf("expected empty string, got %s", got)
	}
}

func TestHandler_LogWithCorrelation(t *testing.T) {
	l := logger.New(slog.LevelInfo)
	ctx := logger.WithCorrelationID(context.Background(), "test-cid")
	l.InfoContext(ctx, "test message")
}

func TestHandler_WithAttrs(t *testing.T) {
	l := logger.New(slog.LevelInfo)
	child := l.With(slog.String("key", "val"))
	if child == nil {
		t.Fatal("expected non-nil logger after WithAttrs")
	}
	child.Info("test with attrs")
}

func TestHandler_WithGroup(t *testing.T) {
	l := logger.New(slog.LevelInfo)
	child := l.WithGroup("grp")
	if child == nil {
		t.Fatal("expected non-nil logger after WithGroup")
	}
	child.Info("test with group")
}
