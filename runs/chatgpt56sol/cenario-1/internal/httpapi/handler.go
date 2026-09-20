package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"pixp2p/internal/money"
	"pixp2p/internal/transfer"
)

type TransferService interface {
	Execute(context.Context, string, string, int64) (transfer.Transfer, error)
}

type handler struct {
	service TransferService
}

type transferRequest struct {
	SourceUserID      string `json:"source_user_id"`
	DestinationUserID string `json:"destination_user_id"`
	Amount            string `json:"amount"`
}

func NewHandler(service TransferService) http.Handler {
	h := &handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /transfers", h.createTransfer)
	return mux
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) createTransfer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	var request transferRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request body must contain one JSON object")
		return
	}

	amountCents, err := money.Parse(request.Amount)
	if err != nil {
		writeError(w, http.StatusBadRequest, "amount must be a positive decimal with at most two fractional digits")
		return
	}

	result, err := h.service.Execute(r.Context(), request.SourceUserID, request.DestinationUserID, amountCents)
	if err != nil {
		switch {
		case errors.Is(err, transfer.ErrInvalidTransfer):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, transfer.ErrWalletNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, transfer.ErrInsufficientFunds):
			writeError(w, http.StatusConflict, err.Error())
		default:
			slog.Error("transfer failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":                  result.ID,
		"source_user_id":      result.SourceID,
		"destination_user_id": result.DestinationID,
		"amount":              money.Format(result.AmountCents),
		"created_at":          result.CreatedAt,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write HTTP response", "error", err)
	}
}
