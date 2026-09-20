package domain

import (
	"testing"
	"time"
)

func TestNewTransfer(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()

	ok, err := NewTransfer("t1", "a", "b", 10, now)
	if err != nil {
		t.Fatal(err)
	}
	if ok.FromWalletID != "a" || ok.ToWalletID != "b" || ok.Amount != 10 {
		t.Fatalf("unexpected transfer %+v", ok)
	}

	if _, err := NewTransfer("", "a", "b", 10, now); err != ErrInvalidWalletID {
		t.Fatalf("got %v", err)
	}
	if _, err := NewTransfer("t1", "", "b", 10, now); err != ErrInvalidWalletID {
		t.Fatalf("got %v", err)
	}
	if _, err := NewTransfer("t1", "a", "a", 10, now); err != ErrSameWalletTransfer {
		t.Fatalf("got %v", err)
	}
	if _, err := NewTransfer("t1", "a", "b", 0, now); err != ErrInvalidAmount {
		t.Fatalf("got %v", err)
	}
}
