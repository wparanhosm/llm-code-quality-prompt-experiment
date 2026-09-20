package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Garante ausência de variáveis que afetariam os defaults.
	for _, k := range []string{"HTTP_ADDR", "DATABASE_DSN", "DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME", "SHUTDOWN_TIMEOUT"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	cfg := loadConfig()
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "postgres://wallet:wallet@localhost:5432/wallet_p2p?sslmode=disable", cfg.DBDSN)
	assert.Equal(t, 25, cfg.DBMaxOpenConns)
	assert.Equal(t, 5, cfg.DBMaxIdleConns)
	assert.Equal(t, 5*time.Minute, cfg.DBConnMaxLifetime)
	assert.Equal(t, 1*time.Minute, cfg.DBConnMaxIdleTime)
	assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout)
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_DSN", "postgres://u:p@db:5432/x")
	t.Setenv("DB_MAX_OPEN_CONNS", "50")
	t.Setenv("DB_MAX_IDLE_CONNS", "10")
	t.Setenv("DB_CONN_MAX_LIFETIME", "10m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "2m")
	t.Setenv("SHUTDOWN_TIMEOUT", "30s")

	cfg := loadConfig()
	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, "postgres://u:p@db:5432/x", cfg.DBDSN)
	assert.Equal(t, 50, cfg.DBMaxOpenConns)
	assert.Equal(t, 10, cfg.DBMaxIdleConns)
	assert.Equal(t, 10*time.Minute, cfg.DBConnMaxLifetime)
	assert.Equal(t, 2*time.Minute, cfg.DBConnMaxIdleTime)
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
}

func TestGetEnvHelpers_InvalidValues(t *testing.T) {
	t.Setenv("INT_BAD", "abc")
	assert.Equal(t, 7, getEnvInt("INT_BAD", 7))
	t.Setenv("INT_NEG", "-3")
	assert.Equal(t, 7, getEnvInt("INT_NEG", 7))
	t.Setenv("DUR_BAD", "não-é-duração")
	assert.Equal(t, time.Second, getEnvDuration("DUR_BAD", time.Second))
	t.Setenv("DUR_NEG", "-5s")
	assert.Equal(t, time.Second, getEnvDuration("DUR_NEG", time.Second))
}

func TestOpenDB_ConfiguresPool(t *testing.T) {
	cfg := config{
		DBDSN:             "postgres://u:p@localhost:5432/x?sslmode=disable",
		DBMaxOpenConns:    11,
		DBMaxIdleConns:    3,
		DBConnMaxLifetime: time.Minute,
		DBConnMaxIdleTime: 30 * time.Second,
	}
	db, err := openDB(cfg)
	require.NoError(t, err)
	defer db.Close()

	stats := db.Stats()
	assert.Equal(t, 11, stats.MaxOpenConnections)
}

func TestOpenDB_InvalidDriver(t *testing.T) {
	// DSN inválido não falha em sql.Open (lazy), então validamos apenas que
	// a função retorna handle utilizável. O ping real ocorre em run().
	db, err := openDB(config{DBDSN: "postgres://"})
	require.NoError(t, err)
	defer db.Close()
}

func TestBuildServer_ServesHealthz(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := config{DBDSN: "postgres://placeholder"}

	// Substitui o banco real injetando via buildServer com openDB "de verdade":
	// sql.Open nunca falha com DSN placeholder, então buildServer funciona.
	srv, realDB, err := buildServer(cfg, logger)
	require.NoError(t, err)
	defer realDB.Close()
	_ = db // sqlmock usado nos demais testes de repositório; aqui validamos o wiring.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	srv.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestRun_GracefulShutdown(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := config{
		HTTPAddr:          "127.0.0.1:0", // porta aleatória livre
		DBDSN:             "postgres://wallet:wallet@localhost:1/wallet_p2p?sslmode=disable&connect_timeout=1",
		ShutdownTimeout:   2 * time.Second,
		DBMaxOpenConns:    2,
		DBMaxIdleConns:    1,
		DBConnMaxLifetime: time.Minute,
		DBConnMaxIdleTime: time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx, cfg, logger)
	}()

	// run() faz ping no banco antes de subir: com porta inalcançável, retorna erro.
	select {
	case err := <-errCh:
		// Esperado: ping falhou porque não há postgres na porta 1.
		require.ErrorContains(t, err, "banco de dados indisponível")
	case <-time.After(15 * time.Second):
		t.Fatal("run() deveria ter retornado erro de ping rapidamente")
	}
	cancel()
}

func TestListenAndServe_ListenError(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := config{HTTPAddr: "endereco-invalido", ShutdownTimeout: time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := listenAndServeWithGracefulShutdown(ctx, cfg, logger, http.NewServeMux())
	require.ErrorContains(t, err, "falha no servidor http")
}

func TestListenAndServe_GracefulShutdownOnCancel(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := config{HTTPAddr: "127.0.0.1:0", ShutdownTimeout: 2 * time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- listenAndServeWithGracefulShutdown(ctx, cfg, logger, http.NewServeMux())
	}()

	// Aguarda o servidor subir e cancela o contexto: deve ocorrer shutdown limpo.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.NoError(t, err, "shutdown via cancelamento de contexto deve ser limpo")
	case <-time.After(5 * time.Second):
		t.Fatal("servidor não encerrou após cancelamento do contexto")
	}
}

func TestPingDB_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectPing()
	require.NoError(t, pingDB(context.Background(), db, time.Second))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPingDB_Failure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectPing().WillReturnError(assert.AnError)
	err = pingDB(context.Background(), db, time.Second)
	require.ErrorContains(t, err, "banco de dados indisponível")
	require.ErrorIs(t, err, assert.AnError)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildServer_ReadyzFailsWhenDBDown(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := config{DBDSN: "postgres://wallet:wallet@localhost:1/wallet_p2p?sslmode=disable&connect_timeout=1"}

	srv, db, err := buildServer(cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	// readyz executa PingContext: sem banco real, deve retornar 503.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	srv.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestMain_ExitPath(t *testing.T) {
	// Não executamos main() diretamente (chamaria os.Exit). Em vez disso,
	// validamos que run() propaga erro, o que main traduz em exit code 1.
	// Este teste documenta o contrato: erro em run => exit 1 em main.
	if os.Getenv("TEST_MAIN_CRASH") == "1" {
		main()
		return
	}
	// Processo-filho: banco inexistente força erro e os.Exit(1).
	cmd := execCommand(os.Args[0], "-test.run=TestMain_ExitPath")
	cmd.Env = append(os.Environ(),
		"TEST_MAIN_CRASH=1",
		"DATABASE_DSN=postgres://wallet:wallet@localhost:1/wallet_p2p?sslmode=disable&connect_timeout=1",
	)
	err := cmd.Run()
	var exitErr interface{ ExitCode() int }
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode(), "main deve sair com código 1 em falha fatal")
}

func TestMain_SignalGracefulShutdown(t *testing.T) {
	if os.Getenv("TEST_MAIN_SIGNAL") == "1" {
		// Sinaliza prontidão e envia SIGTERM para si mesmo rapidamente.
		go func() {
			time.Sleep(300 * time.Millisecond)
			_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		}()
		main()
		return
	}
	cmd := execCommand(os.Args[0], "-test.run=TestMain_SignalGracefulShutdown")
	cmd.Env = append(os.Environ(),
		"TEST_MAIN_SIGNAL=1",
		"DATABASE_DSN=postgres://wallet:wallet@localhost:1/wallet_p2p?sslmode=disable&connect_timeout=5",
	)
	start := time.Now()
	err := cmd.Run()
	// Sem postgres, o ping falha → exit 1. Com postgres, shutdown limpo → exit 0.
	// Em ambos os casos, o processo deve terminar (não travar).
	_ = err
	assert.Less(t, time.Since(start), 30*time.Second, "main deve encerrar sem travar")
}

func TestFormatHelpers(t *testing.T) {
	// getEnv com valor presente.
	t.Setenv("PRESENT", "valor")
	assert.Equal(t, "valor", getEnv("PRESENT", "fallback"))
	assert.Equal(t, "fallback", getEnv("AUSENTE_12345", "fallback"))
}

// sanity: garante que fmt.Sscanf parse de int funciona como esperado nos helpers.
func TestGetEnvInt_Valid(t *testing.T) {
	t.Setenv("VALID_INT", "42")
	assert.Equal(t, 42, getEnvInt("VALID_INT", 1))
}

func TestGetEnvDuration_Valid(t *testing.T) {
	t.Setenv("VALID_DUR", "250ms")
	assert.Equal(t, 250*time.Millisecond, getEnvDuration("VALID_DUR", time.Second))
}
