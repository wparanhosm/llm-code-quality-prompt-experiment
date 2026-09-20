package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	deliveryhttp "github.com/walter/p2p-wallet-transfer/internal/delivery/http"
	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/internal/usecase"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

type mockTransferUseCase struct {
	executeFunc    func(ctx context.Context, input usecase.TransferInput) (*usecase.TransferOutput, error)
	getAccountFunc func(ctx context.Context, id int64) (*domain.Account, error)
}

func (m *mockTransferUseCase) Execute(ctx context.Context, input usecase.TransferInput) (*usecase.TransferOutput, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, input)
	}
	return nil, nil
}

func (m *mockTransferUseCase) GetAccount(ctx context.Context, id int64) (*domain.Account, error) {
	if m.getAccountFunc != nil {
		return m.getAccountFunc(ctx, id)
	}
	return nil, nil
}

func setupTestServer(mockUC usecase.TransferUseCase) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := deliveryhttp.NewHandler(mockUC, logger)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return deliveryhttp.LoggingAndTracingMiddleware(mux, logger)
}

func TestHandleTransfer_Success(t *testing.T) {
	t.Parallel()

	mockUC := &mockTransferUseCase{
		executeFunc: func(ctx context.Context, input usecase.TransferInput) (*usecase.TransferOutput, error) {
			return &usecase.TransferOutput{
				TransferID:             10,
				FromAccountID:          input.FromAccountID,
				ToAccountID:            input.ToAccountID,
				AmountCents:            input.AmountCents,
				FromAccountBalanceLeft: 2000,
				ToAccountBalanceLeft:   8000,
				Status:                 "COMPLETED",
			}, nil
		},
	}

	server := setupTestServer(mockUC)

	reqBody := `{"from_account_id": 1, "to_account_id": 2, "amount_cents": 5000}`
	req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp usecase.TransferOutput
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed decoding json response: %v", err)
	}

	if resp.TransferID != 10 || resp.AmountCents != 5000 {
		t.Errorf("response data mismatch: %+v", resp)
	}
}

func TestHandleTransfer_ValidationErrors(t *testing.T) {
	t.Parallel()

	mockUC := &mockTransferUseCase{}
	server := setupTestServer(mockUC)

	tests := []struct {
		name       string
		body       string
		headerID   string
		wantStatus int
	}{
		{"Invalid JSON", "{invalid-json}", "custom-trace-1", http.StatusBadRequest},
		{"Negative Account ID", `{"from_account_id": -1, "to_account_id": 2, "amount_cents": 500}`, "", http.StatusBadRequest},
		{"Zero Amount", `{"from_account_id": 1, "to_account_id": 2, "amount_cents": 0}`, "", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.headerID != "" {
				req.Header.Set("X-Trace-ID", tt.headerID)
			}
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
			if w.Header().Get("X-Request-ID") == "" {
				t.Errorf("expected X-Request-ID in response header")
			}
		})
	}
}

func TestHandleTransfer_DomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mockErr    error
		wantStatus int
	}{
		{"Account not found", domain.ErrAccountNotFound, http.StatusNotFound},
		{"Insufficient balance", domain.ErrInsufficientBalance, http.StatusUnprocessableEntity},
		{"Same account transfer", domain.ErrSameAccountTransfer, http.StatusUnprocessableEntity},
		{"Account inactive", domain.ErrAccountInactive, http.StatusUnprocessableEntity},
		{"Internal server error", errors.New("unexpected database error"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUC := &mockTransferUseCase{
				executeFunc: func(ctx context.Context, input usecase.TransferInput) (*usecase.TransferOutput, error) {
					return nil, tt.mockErr
				},
			}
			server := setupTestServer(mockUC)

			reqBody := `{"from_account_id": 1, "to_account_id": 2, "amount_cents": 5000}`
			req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(reqBody))
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandleGetAccount(t *testing.T) {
	t.Parallel()

	mockUC := &mockTransferUseCase{
		getAccountFunc: func(ctx context.Context, id int64) (*domain.Account, error) {
			if id == 1 {
				return domain.NewAccount(1, "Alice", money.MustNew(5000), domain.AccountStatusActive)
			}
			if id == 99 {
				return nil, domain.ErrAccountNotFound
			}
			return nil, errors.New("db error")
		},
	}

	server := setupTestServer(mockUC)

	// Success
	req := httptest.NewRequest(http.MethodGet, "/accounts/1", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	// Not found
	req = httptest.NewRequest(http.MethodGet, "/accounts/99", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", w.Code)
	}

	// Invalid ID
	req = httptest.NewRequest(http.MethodGet, "/accounts/abc", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", w.Code)
	}

	// Internal Error
	req = httptest.NewRequest(http.MethodGet, "/accounts/500", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error, got %d", w.Code)
	}
}

func TestHandleHealthCheck(t *testing.T) {
	t.Parallel()
	server := setupTestServer(&mockTransferUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}
}
