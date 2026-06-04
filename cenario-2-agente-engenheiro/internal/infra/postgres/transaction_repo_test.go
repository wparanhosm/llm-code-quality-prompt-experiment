package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"p2p-wallet/internal/domain"
	"p2p-wallet/internal/infra/postgres"
)

func TestTransactionRepo_Create_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	txMgr := postgres.NewTxManager(db)
	repo := postgres.NewTransactionRepository(db)

	now := time.Now().UTC()
	tr := &domain.Transaction{
		ID:           "tx-001",
		FromWalletID: "w1",
		ToWalletID:   "w2",
		Amount:       500,
		Status:       domain.TransactionStatusCompleted,
		CreatedAt:    now,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO transactions").
		WithArgs("tx-001", "w1", "w2", int64(500), domain.TransactionStatusCompleted, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		return repo.Create(txCtx, tr)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionRepo_Create_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	txMgr := postgres.NewTxManager(db)
	repo := postgres.NewTransactionRepository(db)

	now := time.Now().UTC()
	tr := &domain.Transaction{
		ID:           "tx-001",
		FromWalletID: "w1",
		ToWalletID:   "w2",
		Amount:       500,
		Status:       domain.TransactionStatusCompleted,
		CreatedAt:    now,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO transactions").
		WithArgs("tx-001", "w1", "w2", int64(500), domain.TransactionStatusCompleted, now).
		WillReturnError(domain.ErrWalletNotFound)
	mock.ExpectRollback()

	err = txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		return repo.Create(txCtx, tr)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err2 := mock.ExpectationsWereMet(); err2 != nil {
		t.Fatal(err2)
	}
}

func TestTransactionRepo_Create_NoTxInContext(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := postgres.NewTransactionRepository(db)
	e := repo.Create(context.Background(), &domain.Transaction{})
	if e == nil {
		t.Fatal("expected error when no tx in context")
	}
}
