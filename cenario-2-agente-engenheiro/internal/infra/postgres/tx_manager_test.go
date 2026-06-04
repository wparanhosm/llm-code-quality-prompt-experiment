package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"p2p-wallet/internal/domain"
	"p2p-wallet/internal/infra/postgres"
)

func TestTxManager_RunInTx_CommitSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit()

	mgr := postgres.NewTxManager(db)
	called := false
	err = mgr.RunInTx(context.Background(), func(_ context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTxManager_RunInTx_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()

	mgr := postgres.NewTxManager(db)
	err = mgr.RunInTx(context.Background(), func(_ context.Context) error {
		return domain.ErrInsufficientFunds
	})
	if err != domain.ErrInsufficientFunds {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTxManager_RunInTx_RollbackError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("rollback failed"))

	mgr := postgres.NewTxManager(db)
	err = mgr.RunInTx(context.Background(), func(_ context.Context) error {
		return domain.ErrInsufficientFunds
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err2 := mock.ExpectationsWereMet(); err2 != nil {
		t.Fatal(err2)
	}
}

func TestTxManager_RunInTx_BeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin().WillReturnError(domain.ErrWalletNotFound)

	mgr := postgres.NewTxManager(db)
	err = mgr.RunInTx(context.Background(), func(_ context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err2 := mock.ExpectationsWereMet(); err2 != nil {
		t.Fatal(err2)
	}
}

func TestTxManager_RunInTx_CommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(domain.ErrWalletNotFound)

	mgr := postgres.NewTxManager(db)
	err = mgr.RunInTx(context.Background(), func(_ context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if err2 := mock.ExpectationsWereMet(); err2 != nil {
		t.Fatal(err2)
	}
}

func TestTxManager_ContextPassesTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id", "balance", "updated_at"}).
		AddRow("w1", int64(1000), time.Now())
	mock.ExpectQuery("SELECT id, balance, updated_at FROM wallets WHERE id = \\$1 FOR UPDATE").
		WithArgs("w1").
		WillReturnRows(rows)
	mock.ExpectCommit()

	mgr := postgres.NewTxManager(db)
	walletRepo := postgres.NewWalletRepository(db)

	err = mgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		_, err := walletRepo.FindByIDForUpdate(txCtx, "w1")
		return err
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
