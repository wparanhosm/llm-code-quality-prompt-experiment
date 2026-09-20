package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"wallet-p2p/internal/usecase"
)

// Server agrupa o roteador HTTP e os handlers da API.
// Construído via NewServer (wiring manual em cmd/...).
type Server struct {
	mux    *http.ServeMux
	logger *slog.Logger
}

// NewServer monta o servidor HTTP com rotas e middlewares.
// A cadeia de middlewares é: Recover -> RequestID -> Logging -> mux.
func NewServer(uc *usecase.TransferUseCase, logger *slog.Logger, readinessCheck func(r *http.Request) error) *Server {
	mux := http.NewServeMux()

	transferHandler := NewTransferHandler(uc, logger)
	mux.Handle("/v1/transfers", transferHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := readinessCheck(r); err != nil {
			logger.ErrorContext(r.Context(), "readiness check falhou", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	return &Server{mux: mux, logger: logger}
}

// Handler retorna o http.Handler raiz com todos os middlewares aplicados.
func (s *Server) Handler() http.Handler {
	return RecoverMiddleware(s.logger)(
		RequestIDMiddleware(
			LoggingMiddleware(s.logger)(s.mux),
		),
	)
}
