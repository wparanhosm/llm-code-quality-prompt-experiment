// Package httpapi é a camada de delivery: handlers HTTP REST construídos
// exclusivamente sobre net/http nativo (sem frameworks web).
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"

	"wallet-p2p/internal/domain"
	"wallet-p2p/internal/platform/observability"
	"wallet-p2p/internal/usecase"
)

// TransferHandler expõe o endpoint POST /v1/transfers.
type TransferHandler struct {
	uc     *usecase.TransferUseCase
	logger *slog.Logger
}

// NewTransferHandler constrói o handler com as dependências injetadas.
func NewTransferHandler(uc *usecase.TransferUseCase, logger *slog.Logger) *TransferHandler {
	return &TransferHandler{uc: uc, logger: logger}
}

// transferRequest é o payload JSON aceito pelo endpoint.
type transferRequest struct {
	FromWalletID   string `json:"from_wallet_id"`
	ToWalletID     string `json:"to_wallet_id"`
	AmountCents    int64  `json:"amount_cents"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// transferResponse é o payload JSON retornado em sucesso (201 Created).
type transferResponse struct {
	TransactionID       string `json:"transaction_id"`
	FromWalletID        string `json:"from_wallet_id"`
	ToWalletID          string `json:"to_wallet_id"`
	AmountCents         int64  `json:"amount_cents"`
	FromNewBalanceCents int64  `json:"from_new_balance_cents"`
	ToNewBalanceCents   int64  `json:"to_new_balance_cents"`
}

// errorResponse é o envelope de erro padronizado da API.
type errorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// ServeHTTP implementa http.Handler.
func (h *TransferHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "método não permitido", nil)
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "" {
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil || mt != "application/json" {
			h.writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type deve ser application/json", nil)
			return
		}
	}

	// Limite defensivo de payload: 1 MiB é muito mais que suficiente para
	// uma transferência e bloqueia abusos de corpos gigantes (OWASP).
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req transferRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid_body", "corpo da requisição JSON inválido", err)
		return
	}
	if dec.More() {
		h.writeError(w, r, http.StatusBadRequest, "invalid_body", "corpo da requisição deve conter um único objeto JSON", nil)
		return
	}

	output, err := h.uc.Execute(r.Context(), usecase.TransferInput{
		FromWalletID:   req.FromWalletID,
		ToWalletID:     req.ToWalletID,
		AmountCents:    req.AmountCents,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		h.handleUsecaseError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, transferResponse{
		TransactionID:       output.TransactionID,
		FromWalletID:        output.FromWalletID,
		ToWalletID:          output.ToWalletID,
		AmountCents:         output.AmountCents,
		FromNewBalanceCents: output.FromNewBalanceCents,
		ToNewBalanceCents:   output.ToNewBalanceCents,
	})
}

// handleUsecaseError traduz erros de domínio/usecase para status HTTP:
//
//	400 Bad Request           -> entrada inválida (IDs vazios, valor <= 0, origem == destino)
//	404 Not Found             -> carteira inexistente
//	422 Unprocessable Entity  -> saldo insuficiente (requisição bem formada, regra de negócio violada)
//	500 Internal Server Error -> falha interna (detalhe apenas em log, nunca exposto ao cliente)
func (h *TransferHandler) handleUsecaseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidWalletID),
		errors.Is(err, domain.ErrSameWallet),
		errors.Is(err, domain.ErrInvalidAmount):
		h.writeError(w, r, http.StatusBadRequest, "invalid_input", err.Error(), nil)
	case errors.Is(err, domain.ErrWalletNotFound):
		h.writeError(w, r, http.StatusNotFound, "wallet_not_found", "carteira de origem ou destino não encontrada", nil)
	case errors.Is(err, domain.ErrInsufficientFunds):
		h.writeError(w, r, http.StatusUnprocessableEntity, "insufficient_funds", "saldo insuficiente na carteira de origem", nil)
	default:
		h.logger.ErrorContext(r.Context(), "erro interno ao processar transferência",
			append(observability.LogAttrs(r.Context()), slog.String("error", err.Error()))...,
		)
		h.writeError(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor", nil)
	}
}

// writeJSON serializa v como JSON com o status informado.
func (h *TransferHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.logger.Error("falha ao serializar resposta JSON", slog.String("error", err.Error()))
	}
}

// writeJSONWithContext é a variante que propaga o contexto (usada por erros).
func (h *TransferHandler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, cause error) {
	if cause != nil {
		h.logger.WarnContext(r.Context(), "requisição rejeitada",
			append(observability.LogAttrs(r.Context()),
				slog.String("code", code),
				slog.String("error", cause.Error()),
			)...,
		)
	}
	h.writeJSON(w, status, errorResponse{
		Error:     code,
		Message:   message,
		RequestID: observability.RequestIDFromContext(r.Context()),
	})
}
