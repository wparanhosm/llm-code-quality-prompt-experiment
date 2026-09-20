package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
)

type traceKey struct{}

func TraceID(ctx context.Context) string { value, _ := ctx.Value(traceKey{}).(string); return value }
func traceMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" || len(traceID) > 128 {
			var id [16]byte
			if _, err := rand.Read(id[:]); err != nil {
				traceID = "unavailable"
			} else {
				traceID = hex.EncodeToString(id[:])
			}
		}
		w.Header().Set("X-Trace-ID", traceID)
		ctx := context.WithValue(r.Context(), traceKey{}, traceID)
		logger.InfoContext(ctx, "HTTP request", "trace_id", traceID, "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
