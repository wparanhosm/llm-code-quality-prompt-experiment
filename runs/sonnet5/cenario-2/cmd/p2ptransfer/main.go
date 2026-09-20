// Command p2ptransfer inicia o serviço HTTP de transferências P2P entre
// carteiras digitais. Este arquivo é o único ponto do sistema onde as
// dependências concretas (banco de dados, gerador de UUID, logger) são
// instanciadas e conectadas (wiring manual via injeção de dependência por
// construtores), respeitando a Regra de Dependência da Clean Architecture:
// nenhuma camada interna (domain, usecase) conhece este arquivo.
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

	_ "github.com/jackc/pgx/v5/stdlib"

	"p2p-transfer/internal/adapter/repository/postgres"
	deliveryhttp "p2p-transfer/internal/delivery/http"
	"p2p-transfer/internal/platform/database"
	"p2p-transfer/internal/platform/logger"
	"p2p-transfer/internal/usecase"
	"p2p-transfer/pkg/idgen"
)

func main() {
	if err := run(); err != nil {
		slog.Error("aplicação encerrada com erro fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

// run centraliza toda a inicialização e o ciclo de vida da aplicação,
// retornando erro em vez de chamar os.Exit diretamente, o que a torna
// testável isoladamente caso testes de integração de topo sejam
// adicionados no futuro.
func run() error {
	cfg := loadConfig()

	log := logger.New(cfg.logLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, database.DefaultConfig(cfg.databaseDSN))
	if err != nil {
		return fmt.Errorf("erro ao conectar ao banco de dados: %w", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			log.Error("erro ao fechar conexão com o banco de dados", slog.Any("error", cerr))
		}
	}()

	server := newServer(db, cfg, log)

	return serve(ctx, server, log, cfg.httpAddr)
}

// newServer monta o wiring manual de dependências (Injeção de Dependência
// via construtores: HTTP -> Usecase -> Repository) e retorna o
// *http.Server pronto para ser iniciado. Isolada de run() para permitir
// testes unitários do wiring sem depender do ciclo de vida completo do
// processo (sinais de SO, ListenAndServe bloqueante).
func newServer(db *sql.DB, cfg config, log *slog.Logger) *http.Server {
	uow := postgres.NewUnitOfWork(db)
	ids := idgen.NewUUIDGenerator()
	transferUseCase := usecase.NewTransferUseCase(uow, ids, log)
	transferHandler := deliveryhttp.NewTransferHandler(transferUseCase, log)

	mux := http.NewServeMux()
	mux.Handle("POST /transfers", transferHandler)
	mux.HandleFunc("GET /healthz", handleHealthz)

	return &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// serve inicia o servidor HTTP em background e aguarda até que o contexto
// seja cancelado (sinal de encerramento do SO) ou o servidor falhe
// prematuramente, então realiza o graceful shutdown.
func serve(ctx context.Context, server *http.Server, log *slog.Logger, addr string) error {
	serverErrCh := make(chan error, 1)
	go func() {
		log.Info("servidor http iniciado", slog.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- fmt.Errorf("erro ao iniciar servidor http: %w", err)
			return
		}
		serverErrCh <- nil
	}()

	select {
	case <-ctx.Done():
		log.Info("sinal de encerramento recebido, iniciando graceful shutdown")
	case err := <-serverErrCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("erro durante graceful shutdown do servidor http: %w", err)
	}

	log.Info("servidor http encerrado com sucesso")
	return nil
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// config agrupa os parâmetros de configuração da aplicação, lidos
// exclusivamente de variáveis de ambiente (12-factor app), nunca
// hardcoded.
type config struct {
	httpAddr    string
	databaseDSN string
	logLevel    slog.Level
}

// loadConfig lê a configuração de variáveis de ambiente, aplicando valores
// padrão sensatos quando não fornecidas.
func loadConfig() config {
	return config{
		httpAddr:    getEnv("HTTP_ADDR", ":8080"),
		databaseDSN: getEnv("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/p2p_transfer?sslmode=disable"),
		logLevel:    parseLogLevel(getEnv("LOG_LEVEL", "info")),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(raw string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return slog.LevelInfo
	}
	return level
}
