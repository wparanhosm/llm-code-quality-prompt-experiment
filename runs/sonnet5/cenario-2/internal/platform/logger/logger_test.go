package logger_test

import (
	"log/slog"
	"testing"

	"p2p-transfer/internal/platform/logger"
)

func TestNew(t *testing.T) {
	t.Parallel()

	log := logger.New(slog.LevelDebug)
	if log == nil {
		t.Fatal("New não deveria retornar logger nil")
	}
	if !log.Enabled(nil, slog.LevelDebug) {
		t.Fatal("logger deveria ter nível DEBUG habilitado")
	}
}

func TestNew_RespectsMinimumLevel(t *testing.T) {
	t.Parallel()

	log := logger.New(slog.LevelError)
	if log.Enabled(nil, slog.LevelInfo) {
		t.Fatal("logger com nível ERROR não deveria ter INFO habilitado")
	}
	if !log.Enabled(nil, slog.LevelError) {
		t.Fatal("logger com nível ERROR deveria ter ERROR habilitado")
	}
}
