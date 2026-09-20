package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/example/p2p-transfer/internal/application"
	"github.com/example/p2p-transfer/internal/delivery/httpapi"
	"github.com/example/p2p-transfer/internal/infrastructure/postgres"
	runtimepkg "github.com/example/p2p-transfer/pkg/runtime"
	_ "github.com/lib/pq"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("configuration: DATABASE_URL is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	maxOpen, err := envInt("DB_MAX_OPEN_CONNS", 50)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen / 2)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err = db.PingContext(ctx); err != nil {
		cancel()
		return fmt.Errorf("ping database: %w", err)
	}
	cancel()
	manager := postgres.NewManager(db)
	usecase := application.NewTransferUseCase(manager, runtimepkg.CryptoIDGenerator{}, runtimepkg.UTCClock{}, logger)
	handler := httpapi.NewHandler(usecase, logger)
	server := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: handler.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	errCh := make(chan error, 1)
	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve HTTP: %w", serveErr)
		} else {
			errCh <- nil
		}
	}()
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err = server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return <-errCh
	case err = <-errCh:
		return err
	}
}
func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func envInt(name string, fallback int) (int, error) {
	value := env(name, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 2 {
		return 0, fmt.Errorf("configuration %s must be an integer >= 2: %w", name, errors.New("invalid value"))
	}
	return parsed, nil
}
