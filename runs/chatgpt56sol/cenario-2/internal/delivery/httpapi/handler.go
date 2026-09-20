package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/p2p-transfer/internal/application"
	"github.com/example/p2p-transfer/internal/domain"
)

type TransferExecutor interface {
	Execute(ctx context.Context, input application.TransferInput) (application.TransferOutput, error)
}

type Handler struct {
	executor TransferExecutor
	logger   *slog.Logger
}

func NewHandler(executor TransferExecutor, logger *slog.Logger) *Handler {
	return &Handler{executor: executor, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/transfers", h.transfer)
	return traceMiddleware(h.logger, mux)
}

type transferRequest struct {
	SourceWalletID      string `json:"source_wallet_id"`
	DestinationWalletID string `json:"destination_wallet_id"`
	Amount              string `json:"amount"`
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request transferRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	output, err := h.executor.Execute(r.Context(), application.TransferInput{SourceWalletID: strings.TrimSpace(request.SourceWalletID), DestinationWalletID: strings.TrimSpace(request.DestinationWalletID), Amount: request.Amount})
	if err != nil {
		status := http.StatusInternalServerError
		message := "internal error"
		switch {
		case errors.Is(err, domain.ErrInvalidAmount), errors.Is(err, application.ErrInvalidWallet), errors.Is(err, application.ErrSameWallet):
			status, message = http.StatusBadRequest, "invalid transfer"
		case errors.Is(err, application.ErrWalletNotFound):
			status, message = http.StatusNotFound, "wallet not found"
		case errors.Is(err, domain.ErrInsufficientFunds):
			status, message = http.StatusConflict, "insufficient funds"
		}
		h.logger.WarnContext(r.Context(), "transfer failed", "error", err, "status", status)
		writeError(w, status, message)
		return
	}
	writeJSON(w, http.StatusCreated, output)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Default().Error("encode HTTP response", "error", fmt.Errorf("encode response: %w", err))
	}
}
