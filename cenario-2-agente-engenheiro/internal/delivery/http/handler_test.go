package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"p2p-wallet/internal/domain"
	delivery "p2p-wallet/internal/delivery/http"
	"p2p-wallet/internal/usecase"
)

type stubWalletRepo struct {
	wallets map[string]*domain.Wallet
}

func (m *stubWalletRepo) FindByIDForUpdate(_ context.Context, id string) (*domain.Wallet, error) {
	w, ok := m.wallets[id]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	cp := *w
	return &cp, nil
}

func (m *stubWalletRepo) UpdateBalance(_ context.Context, wallet *domain.Wallet) error {
	m.wallets[wallet.ID] = wallet
	return nil
}

type stubTxRepo struct{ created []*domain.Transaction }

func (m *stubTxRepo) Create(_ context.Context, tx *domain.Transaction) error {
	m.created = append(m.created, tx)
	return nil
}

type stubTxManager struct{}

func (m *stubTxManager) RunInTx(_ context.Context, fn func(ctx context.Context) error) error {
	return fn(context.Background())
}

type stubIDGen struct{ id string }

func (m *stubIDGen) NewID() string { return m.id }

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestHandler(wallets map[string]*domain.Wallet) *delivery.TransferHandler {
	wr := &stubWalletRepo{wallets: wallets}
	tr := &stubTxRepo{}
	uc := usecase.NewTransferUseCase(wr, tr, &stubTxManager{}, &stubIDGen{id: "tx-test"}, testLogger())
	return delivery.NewTransferHandler(uc, testLogger())
}

func TestTransferHandler_Success(t *testing.T) {
	h := newTestHandler(map[string]*domain.Wallet{
		"w1": {ID: "w1", Balance: 1000},
		"w2": {ID: "w2", Balance: 500},
	})

	body, _ := json.Marshal(delivery.TransferRequest{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 300,
	})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp delivery.TransferResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.TransactionID != "tx-test" {
		t.Fatalf("expected tx-test, got %s", resp.TransactionID)
	}
}

func TestTransferHandler_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/transfer", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}

func TestTransferHandler_InvalidBody(t *testing.T) {
	h := newTestHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/transfer", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransferHandler_MissingFields(t *testing.T) {
	h := newTestHandler(nil)
	body, _ := json.Marshal(delivery.TransferRequest{Amount: 100})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransferHandler_InvalidAmount(t *testing.T) {
	h := newTestHandler(map[string]*domain.Wallet{
		"w1": {ID: "w1", Balance: 1000},
		"w2": {ID: "w2", Balance: 500},
	})
	body, _ := json.Marshal(delivery.TransferRequest{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransferHandler_SameWallet(t *testing.T) {
	h := newTestHandler(nil)
	body, _ := json.Marshal(delivery.TransferRequest{
		FromWalletID: "w1", ToWalletID: "w1", Amount: 100,
	})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTransferHandler_InsufficientFunds(t *testing.T) {
	h := newTestHandler(map[string]*domain.Wallet{
		"w1": {ID: "w1", Balance: 10},
		"w2": {ID: "w2", Balance: 500},
	})
	body, _ := json.Marshal(delivery.TransferRequest{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestTransferHandler_WalletNotFound(t *testing.T) {
	h := newTestHandler(map[string]*domain.Wallet{
		"w1": {ID: "w1", Balance: 1000},
	})
	body, _ := json.Marshal(delivery.TransferRequest{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestTransferHandler_InternalError(t *testing.T) {
	wr := &failWalletRepo{err: errors.New("db exploded")}
	tr := &stubTxRepo{}
	uc := usecase.NewTransferUseCase(wr, tr, &stubTxManager{}, &stubIDGen{id: "x"}, testLogger())
	h := delivery.NewTransferHandler(uc, testLogger())

	body, _ := json.Marshal(delivery.TransferRequest{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	req := httptest.NewRequest(http.MethodPost, "/transfer", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

type failWalletRepo struct{ err error }

func (m *failWalletRepo) FindByIDForUpdate(_ context.Context, _ string) (*domain.Wallet, error) {
	return nil, m.err
}
func (m *failWalletRepo) UpdateBalance(_ context.Context, _ *domain.Wallet) error {
	return m.err
}
