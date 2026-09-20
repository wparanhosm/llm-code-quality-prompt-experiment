package postgres_test

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"p2p-transfer/internal/adapter/repository/postgres"
	"p2p-transfer/internal/domain"
)

func TestTransactionRepository_Save_Success(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewTransactionRepository(db)

	tx, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-b", 1000)
	if err != nil {
		t.Fatalf("setup falhou: %v", err)
	}

	mock.ExpectExec(`INSERT INTO transactions`).
		WithArgs("tx-1", "wallet-a", "wallet-b", int64(1000), "COMPLETED", nil, tx.CreatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Save(context.Background(), tx); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas do mock não atendidas: %v", err)
	}
}

func TestTransactionRepository_Save_WithFailureReason(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewTransactionRepository(db)

	tx, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-b", 1000)
	if err != nil {
		t.Fatalf("setup falhou: %v", err)
	}
	tx.MarkFailed("saldo insuficiente")

	mock.ExpectExec(`INSERT INTO transactions`).
		WithArgs("tx-1", "wallet-a", "wallet-b", int64(1000), "FAILED", "saldo insuficiente", tx.CreatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Save(context.Background(), tx); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func TestTransactionRepository_Save_ExecError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewTransactionRepository(db)

	tx, err := domain.NewTransaction("tx-1", "wallet-a", "wallet-b", 1000)
	if err != nil {
		t.Fatalf("setup falhou: %v", err)
	}

	mock.ExpectExec(`INSERT INTO transactions`).
		WillReturnError(errors.New("violação de constraint"))

	err = repo.Save(context.Background(), tx)
	if err == nil {
		t.Fatal("esperava erro propagado")
	}
}
