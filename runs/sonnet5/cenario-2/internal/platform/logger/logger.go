// Package logger centraliza a configuração de logging estruturado da
// aplicação usando o pacote nativo log/slog, garantindo formato consistente
// (JSON) e rastreabilidade em todos os componentes.
package logger

import (
	"log/slog"
	"os"
)

// New constrói um *slog.Logger estruturado em JSON, adequado para
// ingestão por ferramentas de observabilidade (ex: ELK, Loki, Datadog). O
// nível mínimo de log é configurável via parâmetro para permitir ajuste
// entre ambientes (debug em desenvolvimento, info em produção).
func New(level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	return slog.New(handler)
}
