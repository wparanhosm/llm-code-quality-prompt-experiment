package domain_test

import (
	"testing"

	"p2p-wallet/internal/domain"
)

func TestWallet_Debit_Success(t *testing.T) {
	w := &domain.Wallet{ID: "w1", Balance: 1000}
	if err := w.Debit(300); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Balance != 700 {
		t.Fatalf("expected balance 700, got %d", w.Balance)
	}
	if w.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
}

func TestWallet_Debit_InsufficientFunds(t *testing.T) {
	w := &domain.Wallet{ID: "w1", Balance: 100}
	err := w.Debit(200)
	if err != domain.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if w.Balance != 100 {
		t.Fatalf("balance should not change on error, got %d", w.Balance)
	}
}

func TestWallet_Debit_InvalidAmount(t *testing.T) {
	w := &domain.Wallet{ID: "w1", Balance: 1000}
	if err := w.Debit(0); err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for zero, got %v", err)
	}
	if err := w.Debit(-5); err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for negative, got %v", err)
	}
}

func TestWallet_Credit_Success(t *testing.T) {
	w := &domain.Wallet{ID: "w1", Balance: 500}
	if err := w.Credit(200); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Balance != 700 {
		t.Fatalf("expected balance 700, got %d", w.Balance)
	}
	if w.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
}

func TestWallet_Credit_InvalidAmount(t *testing.T) {
	w := &domain.Wallet{ID: "w1", Balance: 500}
	if err := w.Credit(0); err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for zero, got %v", err)
	}
	if err := w.Credit(-10); err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount for negative, got %v", err)
	}
}

func TestNewTransaction_Success(t *testing.T) {
	tx, err := domain.NewTransaction("tx1", "w1", "w2", 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.ID != "tx1" {
		t.Fatalf("expected id tx1, got %s", tx.ID)
	}
	if tx.FromWalletID != "w1" || tx.ToWalletID != "w2" {
		t.Fatal("wallet IDs mismatch")
	}
	if tx.Amount != 500 {
		t.Fatalf("expected amount 500, got %d", tx.Amount)
	}
	if tx.Status != domain.TransactionStatusCompleted {
		t.Fatalf("expected COMPLETED status, got %s", tx.Status)
	}
	if tx.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestNewTransaction_InvalidAmount(t *testing.T) {
	_, err := domain.NewTransaction("tx1", "w1", "w2", 0)
	if err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
	_, err = domain.NewTransaction("tx1", "w1", "w2", -100)
	if err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestNewTransaction_SameWallet(t *testing.T) {
	_, err := domain.NewTransaction("tx1", "w1", "w1", 100)
	if err != domain.ErrSameWallet {
		t.Fatalf("expected ErrSameWallet, got %v", err)
	}
}
