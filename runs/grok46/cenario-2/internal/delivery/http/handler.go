package httpdelivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wallet/p2p-transfer/internal/domain"
	"github.com/wallet/p2p-transfer/internal/usecase"
)

const maxBodyBytes = 1 << 20

type transferExecutor interface {
	Execute(ctx context.Context, in usecase.TransferInput) (*domain.Transfer, error)
}

type Handler struct {
	uc     transferExecutor
	logger *slog.Logger
}

func NewHandler(uc transferExecutor, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{uc: uc, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /transfers", h.createTransfer)
	return requestIDMiddleware(mux)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type transferRequest struct {
	FromWalletID string `json:"from_wallet_id"`
	ToWalletID   string `json:"to_wallet_id"`
	Amount       int64  `json:"amount"`
}

type transferResponse struct {
	ID           string `json:"id"`
	FromWalletID string `json:"from_wallet_id"`
	ToWalletID   string `json:"to_wallet_id"`
	Amount       int64  `json:"amount"`
	CreatedAt    string `json:"created_at"`
}

func (h *Handler) createTransfer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()

	var req transferRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "empty body")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	tr, err := h.uc.Execute(r.Context(), usecase.TransferInput{
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		Amount:       domain.Money(req.Amount),
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "transfer handler error", slog.String("error", err.Error()))
		status, msg := mapDomainError(err)
		writeError(w, status, msg)
		return
	}

	writeJSON(w, http.StatusCreated, transferResponse{
		ID:           tr.ID,
		FromWalletID: tr.FromWalletID,
		ToWalletID:   tr.ToWalletID,
		Amount:       int64(tr.Amount),
		CreatedAt:    tr.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

func mapDomainError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalidWalletID),
		errors.Is(err, domain.ErrInvalidAmount),
		errors.Is(err, domain.ErrSameWalletTransfer):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrWalletNotFound):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, domain.ErrInsufficientFunds):
		return http.StatusConflict, err.Error()
	default:
		return http.StatusInternalServerError, "internal error"
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
