// Package observability reúne utilitários de infraestrutura transversais
// (correlation IDs, atributos de log) reutilizáveis pelas camadas de borda.
package observability

import (
	"context"
	"log/slog"
)

// ctxKey é um tipo opaco para chaves de contexto, evitando colisões.
type ctxKey string

// requestIDKey é a chave do correlation/request ID no context.Context.
const requestIDKey ctxKey = "request_id"

// ContextWithRequestID injeta o request ID no contexto.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext extrai o request ID do contexto.
// Retorna string vazia se ausente.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// LogAttrs retorna os atributos estruturados padrão a partir do contexto
// (atualmente o request_id). Uso: logger.InfoContext(ctx, "msg", platform.LogAttrs(ctx)...)
func LogAttrs(ctx context.Context) []any {
	if id := RequestIDFromContext(ctx); id != "" {
		return []any{slog.String("request_id", id)}
	}
	return nil
}
