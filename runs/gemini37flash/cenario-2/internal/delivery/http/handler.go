package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/internal/usecase"
	"github.com/walter/p2p-wallet-transfer/pkg/logger"
)

// TransferRequest represents the incoming JSON body for transfer endpoint.
type TransferRequest struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	AmountCents   int64 `json:"amount_cents"`
}

// ErrorResponse represents standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Handler handles HTTP requests for financial transfers and accounts.
type Handler struct {
	useCase usecase.TransferUseCase
	logger  *slog.Logger
}

// NewHandler creates a new HTTP Handler instance.
func NewHandler(useCase usecase.TransferUseCase, l *slog.Logger) *Handler {
	return &Handler{
		useCase: useCase,
		logger:  l,
	}
}

// RegisterRoutes registers endpoints strictly using Go net/http.ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /transfers", h.HandleTransfer)
	mux.HandleFunc("GET /accounts/", h.HandleGetAccount)
	mux.HandleFunc("GET /health", h.HandleHealthCheck)
}

// HandleTransfer processes P2P transfer requests.
func (h *Handler) HandleTransfer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Max 1MB payload to protect against payload flooding attacks
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var req TransferRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		h.logger.WarnContext(ctx, "failed decoding transfer payload", slog.String("error", err.Error()))
		h.respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request payload: " + err.Error()})
		return
	}

	if req.FromAccountID <= 0 || req.ToAccountID <= 0 {
		h.respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "account IDs must be positive integers"})
		return
	}

	if req.AmountCents <= 0 {
		h.respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "amount_cents must be strictly greater than zero"})
		return
	}

	output, err := h.useCase.Execute(ctx, usecase.TransferInput{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		AmountCents:   req.AmountCents,
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrAccountNotFound):
			h.respondJSON(w, http.StatusNotFound, ErrorResponse{Error: err.Error()})
		case errors.Is(err, domain.ErrInsufficientBalance),
			errors.Is(err, domain.ErrSameAccountTransfer),
			errors.Is(err, domain.ErrInvalidAmount),
			errors.Is(err, domain.ErrAccountInactive):
			h.respondJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error()})
		default:
			h.logger.ErrorContext(ctx, "unhandled transfer error", slog.String("error", err.Error()))
			h.respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		}
		return
	}

	h.respondJSON(w, http.StatusCreated, output)
}

// HandleGetAccount retrieves account details by ID.
func (h *Handler) HandleGetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/accounts/")
	if path == "" {
		h.respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "account ID required"})
		return
	}

	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil || id <= 0 {
		h.respondJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid account ID"})
		return
	}

	acc, err := h.useCase.GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			h.respondJSON(w, http.StatusNotFound, ErrorResponse{Error: "account not found"})
			return
		}
		h.logger.ErrorContext(ctx, "failed retrieving account", slog.String("error", err.Error()))
		h.respondJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	resp := map[string]any{
		"id":            acc.ID,
		"holder":        acc.Holder,
		"balance_cents": acc.Balance.Cents(),
		"status":        acc.Status,
		"created_at":    acc.CreatedAt,
		"updated_at":    acc.UpdatedAt,
	}
	h.respondJSON(w, http.StatusOK, resp)
}

// HandleHealthCheck responds with 200 OK.
func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed writing json response", slog.String("error", err.Error()))
	}
}

// LoggingAndTracingMiddleware adds a unique Trace/Request ID to context and logs execution metrics.
func LoggingAndTracingMiddleware(next http.Handler, l *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = r.Header.Get("X-Trace-ID")
		}
		if reqID == "" {
			b := make([]byte, 16)
			if _, err := io.ReadFull(rand.Reader, b); err != nil {
				reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
			} else {
				reqID = hex.EncodeToString(b)
			}
		}

		ctx := logger.WithCorrelationID(r.Context(), reqID)
		w.Header().Set("X-Request-ID", reqID)

		start := time.Now()
		l.InfoContext(ctx, "incoming HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
		)

		next.ServeHTTP(w, r.WithContext(ctx))

		l.InfoContext(ctx, "completed HTTP request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Duration("duration_ms", time.Since(start)),
		)
	})
}
