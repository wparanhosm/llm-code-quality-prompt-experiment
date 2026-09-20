package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/pix-transfer/internal/handler"
	"github.com/example/pix-transfer/internal/repository"
	"github.com/example/pix-transfer/internal/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		// Exemplo: postgres://usuario:senha@localhost:5432/pix_db?sslmode=disable
		dsn = "postgres://postgres:postgres@localhost:5432/pix_db?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("erro ao conectar no banco: %v", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		log.Fatalf("erro ao pingar banco: %v", err)
	}

	repo := repository.NewTransferRepository(pool)
	svc := service.NewPixService(repo)
	h := handler.NewPixHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/transfers", h.HandleTransfer)
	mux.HandleFunc("/wallets/balance", h.HandleGetBalance)

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("servidor Pix escutando em %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("erro ao iniciar servidor: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("encerrando servidor...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro no shutdown: %v", err)
	}
}
