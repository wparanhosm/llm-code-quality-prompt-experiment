package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/wallet/p2p-transfer/internal/app"
	"github.com/wallet/p2p-transfer/internal/platform"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("fatal", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	addr := platform.Getenv("HTTP_ADDR", ":8080")
	dsn := platform.Getenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/p2p?sslmode=disable")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return err
	}
	if err := app.Migrate(context.Background(), db); err != nil {
		return err
	}

	srv, err := app.NewHTTPServer(app.Config{HTTPAddr: addr, DB: db, Logger: logger})
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http listening", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-stop:
		logger.Info("signal received", slog.String("signal", sig.String()))
	case err := <-errCh:
		return err
	}

	shctx, shcancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shcancel()
	if err := srv.Shutdown(shctx); err != nil {
		return err
	}
	logger.Info("server stopped")
	return nil
}
