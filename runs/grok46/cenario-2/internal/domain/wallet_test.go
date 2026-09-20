package domain

import "testing"

func TestNewWallet(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		w, err := NewWallet("w1", 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.ID != "w1" || w.Balance != 100 {
			t.Fatalf("unexpected wallet: %+v", w)
		}
	})

	t.Run("empty id", func(t *testing.T) {
		t.Parallel()
		if _, err := NewWallet("", 10); err != ErrInvalidWalletID {
			t.Fatalf("expected ErrInvalidWalletID, got %v", err)
		}
	})

	t.Run("negative balance", func(t *testing.T) {
		t.Parallel()
		if _, err := NewWallet("w1", -1); err != ErrNegativeBalance {
			t.Fatalf("expected ErrNegativeBalance, got %v", err)
		}
	})
}

func TestWalletDebitCredit(t *testing.T) {
	t.Parallel()

	w, err := NewWallet("w1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Debit(40); err != nil {
		t.Fatal(err)
	}
	if w.Balance != 60 {
		t.Fatalf("balance = %d", w.Balance)
	}
	if err := w.Credit(15); err != nil {
		t.Fatal(err)
	}
	if w.Balance != 75 {
		t.Fatalf("balance = %d", w.Balance)
	}
	if err := w.Debit(100); err == nil {
		t.Fatal("expected insufficient funds")
	}
	if err := w.Debit(0); err != ErrInvalidAmount {
		t.Fatalf("got %v", err)
	}
	if err := w.Credit(0); err != ErrInvalidAmount {
		t.Fatalf("got %v", err)
	}
}

func TestMoneyValidatePositive(t *testing.T) {
	t.Parallel()
	if err := Money(10).ValidatePositive(); err != nil {
		t.Fatal(err)
	}
	if err := Money(0).ValidatePositive(); err != ErrInvalidAmount {
		t.Fatalf("got %v", err)
	}
}
