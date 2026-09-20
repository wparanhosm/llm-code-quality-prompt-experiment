package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestGetEnv(t *testing.T) {
	t.Run("retorna valor da variável de ambiente quando definida", func(t *testing.T) {
		t.Setenv("P2P_TEST_VAR", "valor-customizado")
		if got := getEnv("P2P_TEST_VAR", "fallback"); got != "valor-customizado" {
			t.Errorf("getEnv = %q, want %q", got, "valor-customizado")
		}
	})

	t.Run("retorna fallback quando variável não definida", func(t *testing.T) {
		if got := getEnv("P2P_TEST_VAR_INEXISTENTE", "fallback"); got != "fallback" {
			t.Errorf("getEnv = %q, want %q", got, "fallback")
		}
	})

	t.Run("retorna fallback quando variável definida como vazia", func(t *testing.T) {
		t.Setenv("P2P_TEST_VAR_VAZIA", "")
		if got := getEnv("P2P_TEST_VAR_VAZIA", "fallback"); got != "fallback" {
			t.Errorf("getEnv = %q, want %q", got, "fallback")
		}
	})
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		raw  string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"nivel-invalido-qualquer", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.raw, func(t *testing.T) {
			if got := parseLogLevel(tt.raw); got != tt.want {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLoadConfig_UsesDefaults(t *testing.T) {
	cfg := loadConfig()
	if cfg.httpAddr == "" {
		t.Error("httpAddr não deveria ser vazio")
	}
	if cfg.databaseDSN == "" {
		t.Error("databaseDSN não deveria ser vazio")
	}
}

func TestLoadConfig_ReadsEnvVars(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9999")
	t.Setenv("DATABASE_DSN", "postgres://custom")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := loadConfig()

	if cfg.httpAddr != ":9999" {
		t.Errorf("httpAddr = %q, want :9999", cfg.httpAddr)
	}
	if cfg.databaseDSN != "postgres://custom" {
		t.Errorf("databaseDSN = %q, want postgres://custom", cfg.databaseDSN)
	}
	if cfg.logLevel != slog.LevelDebug {
		t.Errorf("logLevel = %v, want Debug", cfg.logLevel)
	}
}

func TestRun_ReturnsErrorWhenDatabaseUnreachable(t *testing.T) {
	// Não roda em paralelo pois manipula variáveis de ambiente globais do
	// processo. Exercita o caminho de erro de run() ao tentar conectar em
	// um host de banco de dados inalcançável, sem depender de um Postgres
	// real disponível no ambiente de testes.
	t.Setenv("DATABASE_DSN", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")
	t.Setenv("HTTP_ADDR", ":0")

	err := run()
	if err == nil {
		t.Fatal("esperava erro de run() ao conectar em banco de dados inalcançável")
	}
}

func TestNewServer_WiresDependenciesCorrectly(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("erro ao criar sqlmock: %v", err)
	}
	defer db.Close()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config{httpAddr: ":0", databaseDSN: "unused", logLevel: slog.LevelInfo}

	server := newServer(db, cfg, log)
	if server == nil {
		t.Fatal("newServer não deveria retornar nil")
	}
	if server.Addr != ":0" {
		t.Errorf("Addr = %q, want :0", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("Handler não deveria ser nil")
	}

	// Verifica que as rotas foram registradas corretamente exercitando o
	// handler diretamente.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /healthz status = %d, want 200", rec.Code)
	}
}

func TestServe_GracefulShutdownOnContextCancel(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	server := &http.Server{Addr: "127.0.0.1:0", Handler: mux}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- serve(ctx, server, log, server.Addr)
	}()

	// Aguarda o servidor estar pronto para aceitar conexões antes de
	// cancelar o contexto.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve retornou erro inesperado no graceful shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout esperando graceful shutdown")
	}
}

func TestServe_ReturnsErrorOnListenFailure(t *testing.T) {
	// Ocupa a porta propositalmente para forçar falha de bind no segundo
	// servidor, exercitando o branch de erro de ListenAndServe.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("erro ao criar listener: %v", err)
	}
	defer listener.Close()
	addr := listener.Addr().String()

	server := &http.Server{Addr: addr, Handler: http.NewServeMux()}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	err = serve(context.Background(), server, log, addr)
	if err == nil {
		t.Fatal("esperava erro ao tentar escutar em porta já ocupada")
	}
}

func TestHandleHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	handleHealthz(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != `{"status":"ok"}` {
		t.Fatalf("body = %q, inesperado", rec.Body.String())
	}
}
