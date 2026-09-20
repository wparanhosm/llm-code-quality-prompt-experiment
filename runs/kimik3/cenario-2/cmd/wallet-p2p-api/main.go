// Command wallet-p2p-api é o binário do serviço de transferências P2P.
// Toda a injeção de dependência é manual e concentrada aqui (composition root).
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

	"wallet-p2p/internal/adapter/postgres"
	"wallet-p2p/internal/delivery/httpapi"
	"wallet-p2p/internal/usecase"
)

// config carrega as configurações da aplicação a partir de variáveis de ambiente.
type config struct {
	HTTPAddr string
	DBDSN    string

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration

	ShutdownTimeout time.Duration
}

// loadConfig lê a configuração do ambiente com defaults seguros para dev local.
func loadConfig() config {
	return config{
		HTTPAddr:          getEnv("HTTP_ADDR", ":8080"),
		DBDSN:             getEnv("DATABASE_DSN", "postgres://wallet:wallet@localhost:5432/wallet_p2p?sslmode=disable"),
		DBMaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		DBConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

// openDB abre o pool de conexões PostgreSQL e aplica os limites configurados.
func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("main: falha ao abrir conexão com postgres: %w", err)
	}
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)
	return db, nil
}

// buildServer monta o grafo de dependências e retorna o handler HTTP pronto.
// Extraída de main para permitir testes do wiring (exceto o listen real).
func buildServer(cfg config, logger *slog.Logger) (*httpapi.Server, *sql.DB, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, nil, err
	}

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()
	clock := usecase.NewSystemClock()
	uc := usecase.NewTransferUseCase(uow, repo, clock, logger)

	srv := httpapi.NewServer(uc, logger, func(r *http.Request) error {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("main: readiness ping falhou: %w", err)
		}
		return nil
	})
	return srv, db, nil
}

// pingDB verifica a disponibilidade do banco com timeout.
// Separado de run() para permitir testes do caminho de sucesso/erro.
func pingDB(ctx context.Context, db *sql.DB, timeout time.Duration) error {
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("main: banco de dados indisponível na inicialização: %w", err)
	}
	return nil
}

// run inicia o servidor HTTP e bloqueia até receber SIGINT/SIGTERM,
// executando graceful shutdown. Retorna erro somente em falha fatal.
func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	srv, db, err := buildServer(cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("falha ao fechar pool do banco", slog.String("error", err.Error()))
		}
	}()

	if err := pingDB(ctx, db, 5*time.Second); err != nil {
		return err
	}

	return listenAndServeWithGracefulShutdown(ctx, cfg, logger, srv.Handler())
}

// listenAndServeWithGracefulShutdown sobe o http.Server e bloqueia até o
// contexto ser cancelado ou o servidor falhar, executando graceful shutdown.
func listenAndServeWithGracefulShutdown(ctx context.Context, cfg config, logger *slog.Logger, handler http.Handler) error {
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("servidor http iniciado", slog.String("addr", cfg.HTTPAddr))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("main: falha no servidor http: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("main: graceful shutdown falhou: %w", err)
	}
	logger.Info("servidor http encerrado com sucesso")
	return nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, loadConfig(), logger); err != nil {
		logger.Error("aplicação encerrada com erro", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
