package main

import (
	"testing"

	"github.com/wparanhosm/pix-p2p/domain"
	"github.com/wparanhosm/pix-p2p/infra/database"
	"github.com/wparanhosm/pix-p2p/service"
)

func setupTestDB(t *testing.T) *service.TransferService {
	t.Helper()
	db, err := database.NewConnection(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	db.Exec("INSERT INTO wallets (user_id, balance) VALUES (?, ?)", "alice", 1000.00)
	db.Exec("INSERT INTO wallets (user_id, balance) VALUES (?, ?)", "bob", 500.00)

	return service.NewTransferService(db)
}

func TestTransferSuccess(t *testing.T) {
	svc := setupTestDB(t)

	if err := svc.Transfer("alice", "bob", 200.00); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTransferInsufficientBalance(t *testing.T) {
	svc := setupTestDB(t)

	err := svc.Transfer("alice", "bob", 5000.00)
	if err != domain.ErrInsufficientBalance {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestTransferSameUser(t *testing.T) {
	svc := setupTestDB(t)

	err := svc.Transfer("alice", "alice", 100.00)
	if err != domain.ErrSameUser {
		t.Fatalf("expected ErrSameUser, got %v", err)
	}
}

func TestTransferInvalidAmount(t *testing.T) {
	svc := setupTestDB(t)

	err := svc.Transfer("alice", "bob", -50.00)
	if err != domain.ErrInvalidAmount {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestTransferWalletNotFound(t *testing.T) {
	svc := setupTestDB(t)

	err := svc.Transfer("alice", "unknown", 100.00)
	if err != domain.ErrWalletNotFound {
		t.Fatalf("expected ErrWalletNotFound, got %v", err)
	}
}
