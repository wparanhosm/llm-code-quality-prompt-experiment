// Package database contém utilitários de infraestrutura para conexão com
// o banco de dados relacional (Postgres), sem qualquer regra de negócio.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Config agrupa os parâmetros de conexão e do pool de conexões com o
// Postgres. Valores de pool são dimensionados para cenários de alta
// volumetria/concorrência, evitando esgotamento de conexões sob carga.
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig retorna uma configuração de pool razoável para produção em
// cenários de alta concorrência. O DSN deve ser sempre fornecido pelo
// chamador (nunca hardcoded).
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxOpenConns:    50,
		MaxIdleConns:    25,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// Open estabelece a conexão com o Postgres usando o driver configurado via
// import _ do pacote main (database/sql usa registro de drivers), aplica
// os parâmetros de pool e valida a conectividade com um Ping.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão com o banco de dados: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("erro ao verificar conectividade com o banco de dados: %w", err)
	}

	return db, nil
}
