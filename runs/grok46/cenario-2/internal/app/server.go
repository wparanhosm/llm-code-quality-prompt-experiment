package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/wallet/p2p-transfer/internal/adapter/postgres"
	httpdelivery "github.com/wallet/p2p-transfer/internal/delivery/http"
	"github.com/wallet/p2p-transfer/internal/usecase"
)

type Config struct {
	HTTPAddr string
	DB       *sql.DB
	Logger   *slog.Logger
}

func NewHTTPServer(cfg Config) (*http.Server, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("db is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}

	repo := postgres.NewTransferRepository(cfg.DB)
	uc := usecase.NewTransferP2P(repo, cfg.Logger)
	h := httpdelivery.NewHandler(uc, cfg.Logger)

	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("db is required")
	}
	if _, err := db.ExecContext(ctx, postgres.Schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}
