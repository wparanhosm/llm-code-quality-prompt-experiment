package domain_test

import (
	"testing"

	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

func TestNewAccount(t *testing.T) {
	t.Parallel()

	initBal := money.MustNew(1000)
	acc, err := domain.NewAccount(1, "Alice", initBal, domain.AccountStatusActive)
	if err != nil {
		t.Fatalf("unexpected error creating account: %v", err)
	}

	if acc.ID != 1 || acc.Holder != "Alice" || acc.Balance.Cents() != 1000 || acc.Status != domain.AccountStatusActive {
		t.Errorf("account fields mismatch: %+v", acc)
	}

	// Invalid ID
	_, err = domain.NewAccount(0, "Alice", initBal, domain.AccountStatusActive)
	if err != domain.ErrInvalidAccountID {
		t.Errorf("expected ErrInvalidAccountID, got %v", err)
	}

	// Empty holder
	_, err = domain.NewAccount(1, "", initBal, domain.AccountStatusActive)
	if err == nil {
		t.Errorf("expected error for empty holder, got nil")
	}

	// Default status
	acc2, err := domain.NewAccount(2, "Bob", initBal, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc2.Status != domain.AccountStatusActive {
		t.Errorf("expected default ACTIVE status, got %v", acc2.Status)
	}
}

func TestAccountDebit(t *testing.T) {
	t.Parallel()

	acc, _ := domain.NewAccount(1, "Alice", money.MustNew(5000), domain.AccountStatusActive)

	// Success debit
	err := acc.Debit(money.MustNew(2000))
	if err != nil {
		t.Fatalf("unexpected debit error: %v", err)
	}
	if acc.Balance.Cents() != 3000 {
		t.Errorf("expected balance 3000, got %d", acc.Balance.Cents())
	}

	// Insufficient balance
	err = acc.Debit(money.MustNew(4000))
	if err == nil {
		t.Errorf("expected error for insufficient balance, got nil")
	}

	// Inactive account debit
	accInactive, _ := domain.NewAccount(2, "Bob", money.MustNew(5000), domain.AccountStatusInactive)
	err = accInactive.Debit(money.MustNew(1000))
	if err == nil {
		t.Errorf("expected error for inactive account debit, got nil")
	}

	// Debit invalid zero amount
	zeroMoney, _ := money.FromCentsUnsafe(0)
	err = acc.Debit(zeroMoney)
	if err != domain.ErrInvalidAmount {
		t.Errorf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestAccountCredit(t *testing.T) {
	t.Parallel()

	acc, _ := domain.NewAccount(1, "Alice", money.MustNew(1000), domain.AccountStatusActive)

	// Success credit
	err := acc.Credit(money.MustNew(500))
	if err != nil {
		t.Fatalf("unexpected credit error: %v", err)
	}
	if acc.Balance.Cents() != 1500 {
		t.Errorf("expected balance 1500, got %d", acc.Balance.Cents())
	}

	// Inactive account credit
	accBlocked, _ := domain.NewAccount(2, "Bob", money.MustNew(1000), domain.AccountStatusBlocked)
	err = accBlocked.Credit(money.MustNew(500))
	if err == nil {
		t.Errorf("expected error for inactive account credit, got nil")
	}

	// Credit invalid zero amount
	zeroMoney, _ := money.FromCentsUnsafe(0)
	err = acc.Credit(zeroMoney)
	if err != domain.ErrInvalidAmount {
		t.Errorf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestNewTransfer(t *testing.T) {
	t.Parallel()

	amt := money.MustNew(1000)

	// Success transfer creation
	tr, err := domain.NewTransfer(1, 2, amt)
	if err != nil {
		t.Fatalf("unexpected error creating transfer: %v", err)
	}
	if tr.FromAccountID != 1 || tr.ToAccountID != 2 || tr.Amount.Cents() != 1000 {
		t.Errorf("transfer fields mismatch: %+v", tr)
	}

	// Same account transfer
	_, err = domain.NewTransfer(1, 1, amt)
	if err != domain.ErrSameAccountTransfer {
		t.Errorf("expected ErrSameAccountTransfer, got %v", err)
	}

	// Invalid ID
	_, err = domain.NewTransfer(0, 2, amt)
	if err != domain.ErrInvalidAccountID {
		t.Errorf("expected ErrInvalidAccountID, got %v", err)
	}

	_, err = domain.NewTransfer(1, -1, amt)
	if err != domain.ErrInvalidAccountID {
		t.Errorf("expected ErrInvalidAccountID, got %v", err)
	}

	// Zero amount
	zeroMoney, _ := money.FromCentsUnsafe(0)
	_, err = domain.NewTransfer(1, 2, zeroMoney)
	if err != domain.ErrInvalidAmount {
		t.Errorf("expected ErrInvalidAmount, got %v", err)
	}
}
