package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	delivery "p2p-wallet/internal/delivery/http"
	"p2p-wallet/internal/infra/logger"
	"p2p-wallet/internal/infra/postgres"
	"p2p-wallet/internal/usecase"

	_ "github.com/lib/pq"
)

type uuidGen struct{}

func (uuidGen) NewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func main() {
	log := logger.New(slog.LevelInfo)

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/p2p_wallet?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)

	walletRepo := postgres.NewWalletRepository(db)
	txRepo := postgres.NewTransactionRepository(db)
	txManager := postgres.NewTxManager(db)
	idGen := uuidGen{}

	transferUC := usecase.NewTransferUseCase(walletRepo, txRepo, txManager, idGen, log)
	transferHandler := delivery.NewTransferHandler(transferUC, log)

	correlationGen := func() string { return uuidGen{}.NewID() }

	mux := http.NewServeMux()
	mux.Handle("/transfer", delivery.Chain(
		transferHandler,
		delivery.RecoverMiddleware(log),
		delivery.CorrelationMiddleware(correlationGen),
		delivery.CORSMiddleware,
	))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Info("server starting", slog.String("port", port))
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Error("server failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
