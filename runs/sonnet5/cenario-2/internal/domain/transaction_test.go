package domain_test

import (
	"errors"
	"testing"

	"p2p-transfer/internal/domain"
)

func TestNewTransaction(t *testing.T) {
	t.Parallel()

	t.Run("transação válida", func(t *testing.T) {
		t.Parallel()
		tx, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-b", 500)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if tx.Status != domain.TransactionStatusCompleted {
			t.Fatalf("status = %s, want COMPLETED", tx.Status)
		}
		if tx.CreatedAt.IsZero() {
			t.Fatal("CreatedAt não deveria ser zero")
		}
	})

	t.Run("id vazio", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("", "wallet-a", "wallet-b", 500)
		if err == nil {
			t.Fatal("esperava erro para id vazio")
		}
	})

	t.Run("carteira origem vazia", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("tx-1", "", "wallet-b", 500)
		if err == nil {
			t.Fatal("esperava erro para carteira origem vazia")
		}
	})

	t.Run("carteira destino vazia", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("tx-1", "wallet-a", "", 500)
		if err == nil {
			t.Fatal("esperava erro para carteira destino vazia")
		}
	})

	t.Run("mesma carteira origem e destino", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-a", 500)
		if !errors.Is(err, domain.ErrSameWallet) {
			t.Fatalf("esperava ErrSameWallet, obteve %v", err)
		}
	})

	t.Run("valor inválido", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-b", 0)
		if !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obteve %v", err)
		}
	})
}

func TestTransaction_MarkFailed(t *testing.T) {
	t.Parallel()
	tx, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-b", 500)
	if err != nil {
		t.Fatalf("setup falhou: %v", err)
	}
	tx.MarkFailed("saldo insuficiente")
	if tx.Status != domain.TransactionStatusFailed {
		t.Fatalf("status = %s, want FAILED", tx.Status)
	}
	if tx.FailureReason != "saldo insuficiente" {
		t.Fatalf("FailureReason = %q, want %q", tx.FailureReason, "saldo insuficiente")
	}
}
