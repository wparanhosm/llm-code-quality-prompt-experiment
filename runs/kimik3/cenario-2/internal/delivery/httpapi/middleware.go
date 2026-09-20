package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"wallet-p2p/internal/platform/observability"
)

// RequestIDHeader é o header HTTP usado para correlação de requisições.
// Se o cliente enviar um ID, ele é reutilizado; caso contrário, um UUID v4 é gerado.
const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware gera (ou propaga) um correlation ID por requisição,
// injetando-o no context.Context e no header de resposta.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.NewString()
		}
		ctx := observability.ContextWithRequestID(r.Context(), requestID)
		w.Header().Set(RequestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// statusRecorder captura o status code para logging do acesso HTTP.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware registra um log estruturado por requisição (access log),
// incluindo o request_id correlacionado.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.InfoContext(r.Context(), "http request",
				slog.String("request_id", observability.RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
			)
		})
	}
}

// RecoverMiddleware converte pânicos em 500, sem derrubar o servidor.
func RecoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.ErrorContext(r.Context(), "pânico recuperado no handler",
						slog.String("request_id", observability.RequestIDFromContext(r.Context())),
						slog.Any("panic", rec),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal_error","message":"erro interno do servidor"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
