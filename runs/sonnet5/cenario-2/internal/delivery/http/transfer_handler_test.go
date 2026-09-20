package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	deliveryhttp "p2p-transfer/internal/delivery/http"
	"p2p-transfer/internal/domain"
	"p2p-transfer/internal/usecase"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubExecutor é um double controlável do usecase.TransferUseCase para
// testar o handler HTTP em total isolamento.
type stubExecutor struct {
	output   *usecase.TransferOutput
	err      error
	received usecase.TransferInput
	called   bool
}

func (s *stubExecutor) Execute(_ context.Context, in usecase.TransferInput) (*usecase.TransferOutput, error) {
	s.called = true
	s.received = in
	if s.err != nil {
		return nil, s.err
	}
	return s.output, nil
}

func TestNewTransferHandler_PanicsOnNilExecutor(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("esperava panic com executor nil")
		}
	}()
	deliveryhttp.NewTransferHandler(nil, testLogger())
}

func TestNewTransferHandler_DefaultsLoggerWhenNil(t *testing.T) {
	t.Parallel()
	stub := &stubExecutor{output: &usecase.TransferOutput{}}
	handler := deliveryhttp.NewTransferHandler(stub, nil)
	if handler == nil {
		t.Fatal("esperava handler não nil mesmo com logger nil")
	}
}

// failingResponseWriter simula um cliente que fecha a conexão no meio da
// escrita da resposta, forçando json.Encoder.Encode a retornar erro. Usado
// para exercitar o branch defensivo de log de erro em writeJSON.
type failingResponseWriter struct {
	header http.Header
	code   int
}

func (f *failingResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("conexão fechada pelo cliente")
}

func (f *failingResponseWriter) WriteHeader(statusCode int) {
	f.code = statusCode
}

func TestTransferHandler_ServeHTTP_WriteResponseError(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{output: &usecase.TransferOutput{TransactionID: "tx-1"}}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	body := strings.NewReader(`{"from_wallet_id":"wallet-a","to_wallet_id":"wallet-b","amount_cents":100}`)
	req := httptest.NewRequest(http.MethodPost, "/transfers", body)
	rec := &failingResponseWriter{}

	// Não deve entrar em pânico mesmo que a escrita da resposta falhe; o
	// erro deve apenas ser logado internamente.
	handler.ServeHTTP(rec, req)

	if rec.code != http.StatusOK {
		t.Fatalf("status definido = %d, want 200", rec.code)
	}
}

func TestTransferHandler_ServeHTTP_Success(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{
		output: &usecase.TransferOutput{
			TransactionID:  "tx-1",
			FromWalletID:   "wallet-a",
			ToWalletID:     "wallet-b",
			AmountCents:    1000,
			FromNewBalance: 9000,
			ToNewBalance:   1500,
		},
	}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	body := strings.NewReader(`{"from_wallet_id":"wallet-a","to_wallet_id":"wallet-b","amount_cents":1000}`)
	req := httptest.NewRequest(http.MethodPost, "/transfers", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if !stub.called {
		t.Fatal("usecase deveria ter sido chamado")
	}
	if stub.received.FromWalletID != "wallet-a" || stub.received.ToWalletID != "wallet-b" || stub.received.AmountCents != 1000 {
		t.Fatalf("input recebido pelo usecase incorreto: %+v", stub.received)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("erro ao decodificar resposta: %v", err)
	}
	if resp["transaction_id"] != "tx-1" {
		t.Errorf("transaction_id = %v, want tx-1", resp["transaction_id"])
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestTransferHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/transfers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if stub.called {
		t.Fatal("usecase não deveria ter sido chamado")
	}
}

func TestTransferHandler_ServeHTTP_InvalidJSON(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/transfers", strings.NewReader(`{"from_wallet_id":`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if stub.called {
		t.Fatal("usecase não deveria ter sido chamado com JSON inválido")
	}
}

func TestTransferHandler_ServeHTTP_EmptyBody(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/transfers", strings.NewReader(``))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTransferHandler_ServeHTTP_UnknownFields(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	body := strings.NewReader(`{"from_wallet_id":"a","to_wallet_id":"b","amount_cents":100,"campo_desconhecido":true}`)
	req := httptest.NewRequest(http.MethodPost, "/transfers", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if stub.called {
		t.Fatal("usecase não deveria ter sido chamado com campo desconhecido")
	}
}

func TestTransferHandler_ServeHTTP_TrailingContentAfterJSON(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{output: &usecase.TransferOutput{}}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	body := strings.NewReader(`{"from_wallet_id":"a","to_wallet_id":"b","amount_cents":100}{"extra":"lixo"}`)
	req := httptest.NewRequest(http.MethodPost, "/transfers", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestTransferHandler_ServeHTTP_PayloadTooLarge(t *testing.T) {
	t.Parallel()

	stub := &stubExecutor{}
	handler := deliveryhttp.NewTransferHandler(stub, testLogger())

	hugeValue := strings.Repeat("a", 1<<17) // maior que maxRequestBodyBytes
	body := strings.NewReader(`{"from_wallet_id":"` + hugeValue + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/transfers", body)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (payload excessivo)", rec.Code)
	}
}

func TestTransferHandler_ServeHTTP_UsecaseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"carteira não encontrada", domain.ErrWalletNotFound, http.StatusNotFound},
		{"saldo insuficiente", domain.ErrInsufficientFunds, http.StatusBadRequest},
		{"valor inválido", domain.ErrInvalidAmount, http.StatusBadRequest},
		{"mesma carteira", domain.ErrSameWallet, http.StatusBadRequest},
		{"carteira inativa", domain.ErrWalletInactive, http.StatusBadRequest},
		{"erro genérico de infraestrutura", errors.New("falha de conexão com o banco"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubExecutor{err: tt.err}
			handler := deliveryhttp.NewTransferHandler(stub, testLogger())

			body := strings.NewReader(`{"from_wallet_id":"wallet-a","to_wallet_id":"wallet-b","amount_cents":100}`)
			req := httptest.NewRequest(http.MethodPost, "/transfers", body)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}

			var resp map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("erro ao decodificar resposta de erro: %v", err)
			}
			if _, ok := resp["error"]; !ok {
				t.Fatal("resposta de erro deveria conter campo 'error'")
			}
		})
	}
}
