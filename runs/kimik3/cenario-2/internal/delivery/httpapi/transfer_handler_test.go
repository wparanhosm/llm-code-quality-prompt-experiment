package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/delivery/httpapi"
	"wallet-p2p/internal/domain"
	"wallet-p2p/internal/usecase"
)

// --- Mocks -------------------------------------------------------------------

type stubTx struct{ usecase.Tx }

func (stubTx) Sealed() {}

type mockUoW struct {
	executeErr error
}

func (m *mockUoW) Do(ctx context.Context, fn func(ctx context.Context, tx usecase.Tx) error) error {
	if m.executeErr != nil {
		return m.executeErr
	}
	return fn(ctx, stubTx{})
}

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) GetByIDForUpdate(ctx context.Context, tx usecase.Tx, id string) (domain.Wallet, error) {
	args := m.Called(ctx, tx, id)
	w, _ := args.Get(0).(domain.Wallet)
	return w, args.Error(1)
}

func (m *mockRepo) UpdateBalance(ctx context.Context, tx usecase.Tx, w domain.Wallet) error {
	return m.Called(ctx, tx, w).Error(0)
}

func (m *mockRepo) InsertTransaction(ctx context.Context, tx usecase.Tx, t domain.Transaction) error {
	return m.Called(ctx, tx, t).Error(0)
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }

// --- Helpers -----------------------------------------------------------------

var testNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

const (
	walletA = "11111111-1111-1111-1111-111111111111"
	walletB = "22222222-2222-2222-2222-222222222222"
)

func walletWithBalance(id string, cents int64) domain.Wallet {
	return domain.ReconstructWallet(id, cents, testNow, testNow)
}

// happyPathRepo retorna um mock de repositório configurado para uma
// transferência de 250 cents de walletA (saldo 1000) para walletB (saldo 500).
func happyPathRepo() *mockRepo {
	repo := new(mockRepo)
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, walletA).
		Return(walletWithBalance(walletA, 1000), nil)
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, walletB).
		Return(walletWithBalance(walletB, 500), nil)
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertTransaction", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return repo
}

// newServerWithRepo monta o servidor HTTP completo (com middlewares) sobre
// um repositório mockado.
func newServerWithRepo(repo *mockRepo) http.Handler {
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newDiscardLogger())
	srv := httpapi.NewServer(uc, newDiscardLogger(), func(_ *http.Request) error { return nil })
	return srv.Handler()
}

func doRequest(t *testing.T, handler http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func validTransferBody() string {
	return `{"from_wallet_id":"` + walletA + `","to_wallet_id":"` + walletB + `","amount_cents":250}`
}

// --- Testes ------------------------------------------------------------------

func TestTransferHandler_Success(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(happyPathRepo())
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type": "application/json",
	})

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get(httpapi.RequestIDHeader), "response deve conter o request id")

	var resp struct {
		TransactionID       string `json:"transaction_id"`
		FromWalletID        string `json:"from_wallet_id"`
		ToWalletID          string `json:"to_wallet_id"`
		AmountCents         int64  `json:"amount_cents"`
		FromNewBalanceCents int64  `json:"from_new_balance_cents"`
		ToNewBalanceCents   int64  `json:"to_new_balance_cents"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.TransactionID)
	assert.Equal(t, walletA, resp.FromWalletID)
	assert.Equal(t, walletB, resp.ToWalletID)
	assert.Equal(t, int64(250), resp.AmountCents)
	assert.Equal(t, int64(750), resp.FromNewBalanceCents)
	assert.Equal(t, int64(750), resp.ToNewBalanceCents)
}

func TestTransferHandler_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := doRequest(t, handler, method, "/v1/transfers", "", nil)
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "método %s", method)
	}
}

func TestTransferHandler_UnsupportedMediaType(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type": "text/plain",
	})
	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)

	var errResp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "unsupported_media_type", errResp.Error)
}

func TestTransferHandler_InvalidJSON(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", `{invalid json`, map[string]string{
		"Content-Type": "application/json",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "invalid_body", errResp.Error)
}

func TestTransferHandler_MultipleJSONObjects(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	body := validTransferBody() + validTransferBody()
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", body, map[string]string{
		"Content-Type": "application/json",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTransferHandler_UnknownFields(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	body := `{"from_wallet_id":"` + walletA + `","to_wallet_id":"` + walletB + `","amount_cents":250,"extra":true}`
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", body, map[string]string{
		"Content-Type": "application/json",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "campos desconhecidos devem ser rejeitados (DisallowUnknownFields)")
}

func TestTransferHandler_PayloadTooLarge(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	big := `{"from_wallet_id":"` + walletA + `","to_wallet_id":"` + walletB + `","amount_cents":250,"pad":"` +
		strings.Repeat("x", 2<<20) + `"}`
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", big, map[string]string{
		"Content-Type": "application/json",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "payload acima de 1 MiB deve ser rejeitado")
}

func TestTransferHandler_DomainValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "origem vazia",
			body:       `{"from_wallet_id":"","to_wallet_id":"` + walletB + `","amount_cents":100}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "destino vazio",
			body:       `{"from_wallet_id":"` + walletA + `","to_wallet_id":"","amount_cents":100}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "origem igual ao destino",
			body:       `{"from_wallet_id":"` + walletA + `","to_wallet_id":"` + walletA + `","amount_cents":100}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "valor zero",
			body:       `{"from_wallet_id":"` + walletA + `","to_wallet_id":"` + walletB + `","amount_cents":0}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
		{
			name:       "valor negativo",
			body:       `{"from_wallet_id":"` + walletA + `","to_wallet_id":"` + walletB + `","amount_cents":-5}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := newServerWithRepo(new(mockRepo))
			rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", tt.body, map[string]string{
				"Content-Type": "application/json",
			})
			require.Equal(t, tt.wantStatus, rec.Code)
			var errResp struct {
				Error string `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
			assert.Equal(t, tt.wantCode, errResp.Error)
		})
	}
}

func TestTransferHandler_WalletNotFound(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, walletA).
		Return(domain.Wallet{}, domain.ErrWalletNotFound)

	handler := newServerWithRepo(repo)
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type": "application/json",
	})
	require.Equal(t, http.StatusNotFound, rec.Code)

	var errResp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "wallet_not_found", errResp.Error)
}

func TestTransferHandler_InsufficientFunds(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, walletA).
		Return(walletWithBalance(walletA, 100), nil) // saldo menor que 250
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, walletB).
		Return(walletWithBalance(walletB, 500), nil)

	handler := newServerWithRepo(repo)
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type": "application/json",
	})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var errResp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "insufficient_funds", errResp.Error)
}

func TestTransferHandler_InternalError(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, mock.Anything).
		Return(domain.Wallet{}, errors.New("banco fora do ar"))

	handler := newServerWithRepo(repo)
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type": "application/json",
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "internal_error", errResp.Error)
	assert.NotContains(t, errResp.Message, "banco fora do ar", "detalhes internos nunca devem vazar para o cliente")
}

// --- Middlewares e endpoints de operação -------------------------------------

func TestRequestIDMiddleware_GeneratesAndPropagates(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(happyPathRepo())
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type": "application/json",
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	id := rec.Header().Get(httpapi.RequestIDHeader)
	require.NotEmpty(t, id)
	assert.Len(t, id, 36, "request id gerado deve ser um UUID v4")
}

func TestRequestIDMiddleware_ReusesClientProvidedID(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(happyPathRepo())
	rec := doRequest(t, handler, http.MethodPost, "/v1/transfers", validTransferBody(), map[string]string{
		"Content-Type":             "application/json",
		httpapi.RequestIDHeader: "my-correlation-id-123",
	})
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "my-correlation-id-123", rec.Header().Get(httpapi.RequestIDHeader))

	var resp struct {
		TransactionID string `json:"transaction_id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
}

func TestHealthz(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	rec := doRequest(t, handler, http.MethodGet, "/healthz", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp.Status)
}

func TestReadyz(t *testing.T) {
	t.Parallel()

	t.Run("pronto quando dependência saudável", func(t *testing.T) {
		t.Parallel()
		uc := usecase.NewTransferUseCase(&mockUoW{}, new(mockRepo), fixedClock{now: testNow}, newDiscardLogger())
		srv := httpapi.NewServer(uc, newDiscardLogger(), func(_ *http.Request) error { return nil })
		rec := doRequest(t, srv.Handler(), http.MethodGet, "/readyz", "", nil)
		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("indisponível quando dependência falha", func(t *testing.T) {
		t.Parallel()
		uc := usecase.NewTransferUseCase(&mockUoW{}, new(mockRepo), fixedClock{now: testNow}, newDiscardLogger())
		srv := httpapi.NewServer(uc, newDiscardLogger(), func(_ *http.Request) error {
			return errors.New("banco inacessível")
		})
		rec := doRequest(t, srv.Handler(), http.MethodGet, "/readyz", "", nil)
		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

func TestNotFoundRoute(t *testing.T) {
	t.Parallel()

	handler := newServerWithRepo(new(mockRepo))
	rec := doRequest(t, handler, http.MethodGet, "/rota-inexistente", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRecoverMiddleware_PanicBecomes500(t *testing.T) {
	t.Parallel()

	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("falha catastrófica simulada")
	})
	handler := httpapi.RecoverMiddleware(newDiscardLogger())(
		httpapi.RequestIDMiddleware(panicHandler),
	)

	rec := doRequest(t, handler, http.MethodGet, "/qualquer", "", nil)
	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var errResp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Equal(t, "internal_error", errResp.Error)
}

func TestLoggingMiddleware_CapturesStatus(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := httpapi.RequestIDMiddleware(httpapi.LoggingMiddleware(logger)(inner))

	rec := doRequest(t, handler, http.MethodGet, "/status-test", "", nil)
	require.Equal(t, http.StatusTeapot, rec.Code)

	var logEntry struct {
		Message   string `json:"msg"`
		Status    int    `json:"status"`
		Method    string `json:"method"`
		Path      string `json:"path"`
		RequestID string `json:"request_id"`
	}
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &logEntry))
	assert.Equal(t, "http request", logEntry.Message)
	assert.Equal(t, http.StatusTeapot, logEntry.Status)
	assert.Equal(t, http.MethodGet, logEntry.Method)
	assert.Equal(t, "/status-test", logEntry.Path)
	assert.NotEmpty(t, logEntry.RequestID)
}
