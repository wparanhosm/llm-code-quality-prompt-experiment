package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"p2p-wallet/internal/domain"
	"p2p-wallet/internal/usecase"
)

type TransferRequest struct {
	FromWalletID string `json:"from_wallet_id"`
	ToWalletID   string `json:"to_wallet_id"`
	Amount       int64  `json:"amount"`
}

type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type TransferHandler struct {
	uc     *usecase.TransferUseCase
	logger *slog.Logger
}

func NewTransferHandler(uc *usecase.TransferUseCase, logger *slog.Logger) *TransferHandler {
	return &TransferHandler{uc: uc, logger: logger}
}

func (h *TransferHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.FromWalletID == "" || req.ToWalletID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "from_wallet_id and to_wallet_id are required"})
		return
	}

	out, err := h.uc.Execute(r.Context(), usecase.TransferInput{
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		Amount:       req.Amount,
	})

	if err != nil {
		h.handleError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, TransferResponse{TransactionID: out.TransactionID})
}

func (h *TransferHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidAmount):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrSameWallet):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrInsufficientFunds):
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
	case errors.Is(err, domain.ErrWalletNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
	default:
		h.logger.Error("internal error", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
