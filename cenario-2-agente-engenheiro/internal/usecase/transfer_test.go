package usecase_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"p2p-wallet/internal/domain"
	"p2p-wallet/internal/usecase"
)

type mockWalletRepo struct {
	wallets    map[string]*domain.Wallet
	findErr    error
	updateErr  error
	updateCall int
}

func (m *mockWalletRepo) FindByIDForUpdate(_ context.Context, id string) (*domain.Wallet, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	w, ok := m.wallets[id]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	cp := *w
	return &cp, nil
}

func (m *mockWalletRepo) UpdateBalance(_ context.Context, wallet *domain.Wallet) error {
	m.updateCall++
	if m.updateErr != nil {
		return m.updateErr
	}
	m.wallets[wallet.ID] = wallet
	return nil
}

type mockTxRepo struct {
	created []*domain.Transaction
	err     error
}

func (m *mockTxRepo) Create(_ context.Context, tx *domain.Transaction) error {
	if m.err != nil {
		return m.err
	}
	m.created = append(m.created, tx)
	return nil
}

type mockTxManager struct {
	err error
}

func (m *mockTxManager) RunInTx(_ context.Context, fn func(ctx context.Context) error) error {
	if m.err != nil {
		return m.err
	}
	return fn(context.Background())
}

type mockIDGen struct {
	id string
}

func (m *mockIDGen) NewID() string { return m.id }

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestTransferUseCase_Execute_Success(t *testing.T) {
	walletRepo := &mockWalletRepo{
		wallets: map[string]*domain.Wallet{
			"w1": {ID: "w1", Balance: 1000},
			"w2": {ID: "w2", Balance: 500},
		},
	}
	txRepo := &mockTxRepo{}
	uc := usecase.NewTransferUseCase(walletRepo, txRepo, &mockTxManager{}, &mockIDGen{id: "tx-001"}, newTestLogger())

	out, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1",
		ToWalletID:   "w2",
		Amount:       300,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TransactionID != "tx-001" {
		t.Fatalf("expected tx-001, got %s", out.TransactionID)
	}
	if walletRepo.wallets["w1"].Balance != 700 {
		t.Fatalf("expected source balance 700, got %d", walletRepo.wallets["w1"].Balance)
	}
	if walletRepo.wallets["w2"].Balance != 800 {
		t.Fatalf("expected dest balance 800, got %d", walletRepo.wallets["w2"].Balance)
	}
	if len(txRepo.created) != 1 {
		t.Fatalf("expected 1 transaction record, got %d", len(txRepo.created))
	}
	rec := txRepo.created[0]
	if rec.Amount != 300 || rec.FromWalletID != "w1" || rec.ToWalletID != "w2" {
		t.Fatal("transaction record mismatch")
	}
}

func TestTransferUseCase_Execute_InvalidAmount(t *testing.T) {
	uc := usecase.NewTransferUseCase(&mockWalletRepo{wallets: map[string]*domain.Wallet{}}, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 0,
	})
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestTransferUseCase_Execute_SameWallet(t *testing.T) {
	uc := usecase.NewTransferUseCase(&mockWalletRepo{wallets: map[string]*domain.Wallet{}}, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w1", Amount: 100,
	})
	if !errors.Is(err, domain.ErrSameWallet) {
		t.Fatalf("expected ErrSameWallet, got %v", err)
	}
}

func TestTransferUseCase_Execute_InsufficientFunds(t *testing.T) {
	walletRepo := &mockWalletRepo{
		wallets: map[string]*domain.Wallet{
			"w1": {ID: "w1", Balance: 50},
			"w2": {ID: "w2", Balance: 500},
		},
	}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
}

func TestTransferUseCase_Execute_WalletNotFound(t *testing.T) {
	walletRepo := &mockWalletRepo{
		wallets: map[string]*domain.Wallet{
			"w1": {ID: "w1", Balance: 1000},
		},
	}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("expected ErrWalletNotFound, got %v", err)
	}
}

func TestTransferUseCase_Execute_FindError(t *testing.T) {
	dbErr := errors.New("db connection lost")
	walletRepo := &mockWalletRepo{wallets: map[string]*domain.Wallet{}, findErr: dbErr}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected db error, got %v", err)
	}
}

func TestTransferUseCase_Execute_UpdateError(t *testing.T) {
	dbErr := errors.New("update failed")
	walletRepo := &mockWalletRepo{
		wallets: map[string]*domain.Wallet{
			"w1": {ID: "w1", Balance: 1000},
			"w2": {ID: "w2", Balance: 500},
		},
		updateErr: dbErr,
	}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected update error, got %v", err)
	}
}

func TestTransferUseCase_Execute_CreateTransactionError(t *testing.T) {
	dbErr := errors.New("insert failed")
	walletRepo := &mockWalletRepo{
		wallets: map[string]*domain.Wallet{
			"w1": {ID: "w1", Balance: 1000},
			"w2": {ID: "w2", Balance: 500},
		},
	}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{err: dbErr}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected insert error, got %v", err)
	}
}

func TestTransferUseCase_Execute_TxManagerError(t *testing.T) {
	txErr := errors.New("tx begin failed")
	walletRepo := &mockWalletRepo{wallets: map[string]*domain.Wallet{}}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{}, &mockTxManager{err: txErr}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w1", ToWalletID: "w2", Amount: 100,
	})
	if !errors.Is(err, txErr) {
		t.Fatalf("expected tx error, got %v", err)
	}
}

func TestTransferUseCase_Execute_LockOrdering(t *testing.T) {
	var lockOrder []string
	walletRepo := &lockOrderWalletRepo{
		wallets: map[string]*domain.Wallet{
			"w2": {ID: "w2", Balance: 1000},
			"w1": {ID: "w1", Balance: 500},
		},
		order: &lockOrder,
	}
	uc := usecase.NewTransferUseCase(walletRepo, &mockTxRepo{}, &mockTxManager{}, &mockIDGen{id: "x"}, newTestLogger())
	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: "w2", ToWalletID: "w1", Amount: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lockOrder) != 2 || lockOrder[0] != "w1" || lockOrder[1] != "w2" {
		t.Fatalf("expected lock order [w1, w2], got %v", lockOrder)
	}
}

type lockOrderWalletRepo struct {
	wallets map[string]*domain.Wallet
	order   *[]string
}

func (m *lockOrderWalletRepo) FindByIDForUpdate(_ context.Context, id string) (*domain.Wallet, error) {
	*m.order = append(*m.order, id)
	w, ok := m.wallets[id]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	cp := *w
	return &cp, nil
}

func (m *lockOrderWalletRepo) UpdateBalance(_ context.Context, wallet *domain.Wallet) error {
	m.wallets[wallet.ID] = wallet
	return nil
}
