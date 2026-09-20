package database_test

import (
	"context"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"p2p-transfer/internal/platform/database"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := database.DefaultConfig("postgres://user:pass@localhost:5432/db")

	if cfg.DSN != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("DSN = %q, inesperado", cfg.DSN)
	}
	if cfg.MaxOpenConns <= 0 {
		t.Error("MaxOpenConns deveria ser positivo")
	}
	if cfg.MaxIdleConns <= 0 {
		t.Error("MaxIdleConns deveria ser positivo")
	}
	if cfg.ConnMaxLifetime <= 0 {
		t.Error("ConnMaxLifetime deveria ser positivo")
	}
	if cfg.ConnMaxIdleTime <= 0 {
		t.Error("ConnMaxIdleTime deveria ser positivo")
	}
}

func TestOpen_PingFailsForUnreachableHost(t *testing.T) {
	t.Parallel()

	// Usa um host inexistente/porta inválida para forçar falha de conexão
	// sem depender de um banco de dados real disponível no ambiente de
	// testes (teste unitário, não de integração).
	cfg := database.DefaultConfig("postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := database.Open(ctx, cfg)
	if err == nil {
		t.Fatal("esperava erro ao conectar a um host inalcançável")
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	t.Parallel()

	cfg := database.DefaultConfig("://dsn-invalido")

	_, err := database.Open(context.Background(), cfg)
	if err == nil {
		t.Fatal("esperava erro para DSN malformado")
	}
}
