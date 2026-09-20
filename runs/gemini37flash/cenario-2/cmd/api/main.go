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
	"syscall"
	"time"

	_ "github.com/lib/pq"
	adapter "github.com/walter/p2p-wallet-transfer/internal/adapter/postgres"
	deliveryhttp "github.com/walter/p2p-wallet-transfer/internal/delivery/http"
	"github.com/walter/p2p-wallet-transfer/internal/usecase"
	"github.com/walter/p2p-wallet-transfer/pkg/logger"
)

func main() {
	l := logger.New()

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "wallet_p2p")
	serverPort := getEnv("PORT", "8080")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		l.Error("failed opening database connection", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		l.Warn("database ping failed, proceeding (ensure database is available)", slog.String("error", err.Error()))
	} else {
		l.Info("connected to PostgreSQL database successfully")
	}

	// Manual Dependency Injection
	accountRepo := adapter.NewAccountRepository(db)
	uow := adapter.NewUnitOfWork(db)
	transferUC := usecase.NewTransferUseCase(uow, accountRepo, l)
	httpHandler := deliveryhttp.NewHandler(transferUC, l)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	loggedMux := deliveryhttp.LoggingAndTracingMiddleware(mux, l)

	server := &http.Server{
		Addr:         ":" + serverPort,
		Handler:      loggedMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		l.Info("starting HTTP server", slog.String("port", serverPort))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			l.Error("HTTP server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	l.Info("shutting down HTTP server gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		l.Error("server forced to shutdown", slog.String("error", err.Error()))
	}

	l.Info("server exited safely")
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
