package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/example/pix-transfer/internal/domain"
	"github.com/example/pix-transfer/internal/service"
)

// PixHandler expõe a funcionalidade de transferência Pix via HTTP.
type PixHandler struct {
	service *service.PixService
}

func NewPixHandler(svc *service.PixService) *PixHandler {
	return &PixHandler{service: svc}
}

// transferRequest é o payload esperado no corpo da requisição HTTP.
// AmountCents evita problemas de precisão de ponto flutuante; caso
// prefira receber em reais (float), converta com cuidado antes de
// chamar o serviço (ex: math.Round(amount*100)).
type transferRequest struct {
	FromUserID     string `json:"from_user_id"`
	ToUserID       string `json:"to_user_id"`
	AmountCents    int64  `json:"amount_cents"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type transferResponse struct {
	TransferID     string `json:"transfer_id"`
	Status         string `json:"status"`
	FromUserID     string `json:"from_user_id"`
	ToUserID       string `json:"to_user_id"`
	AmountCents    int64  `json:"amount_cents"`
	IdempotencyKey string `json:"idempotency_key"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// HandleTransfer trata POST /transfers
func (h *PixHandler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "método não permitido")
		return
	}

	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo da requisição inválido: "+err.Error())
		return
	}

	fromID, err := uuid.Parse(req.FromUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "from_user_id inválido")
		return
	}
	toID, err := uuid.Parse(req.ToUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "to_user_id inválido")
		return
	}

	var idempotencyKey uuid.UUID
	if req.IdempotencyKey != "" {
		idempotencyKey, err = uuid.Parse(req.IdempotencyKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "idempotency_key inválida")
			return
		}
	}

	params := service.TransferParams{
		FromUserID:     fromID,
		ToUserID:       toID,
		AmountCents:    req.AmountCents,
		IdempotencyKey: idempotencyKey,
	}

	transfer, err := h.service.Transfer(r.Context(), params)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := transferResponse{
		TransferID:     transfer.ID.String(),
		Status:         string(transfer.Status),
		FromUserID:     transfer.FromUserID.String(),
		ToUserID:       transfer.ToUserID.String(),
		AmountCents:    transfer.AmountCents,
		IdempotencyKey: transfer.IdempotencyKey.String(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleGetBalance trata GET /wallets/{user_id}/balance
func (h *PixHandler) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "método não permitido")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "user_id inválido")
		return
	}

	balance, err := h.service.GetBalance(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":       userID.String(),
		"balance_cents": balance,
	})
}

// handleServiceError mapeia erros de domínio para códigos HTTP apropriados.
func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameAccount):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrWalletNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrDuplicateTransfer):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("erro interno: %v", err)
		writeError(w, http.StatusInternalServerError, "erro interno do servidor")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("erro ao codificar resposta JSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
