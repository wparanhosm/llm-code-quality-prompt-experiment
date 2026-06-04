package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"p2p-wallet/internal/domain"
	"p2p-wallet/internal/infra/postgres"
)

func setupWalletRepo(t *testing.T) (*postgres.WalletRepository, sqlmock.Sqlmock, *sql.DB, *postgres.TxManager) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	return postgres.NewWalletRepository(db), mock, db, postgres.NewTxManager(db)
}

func TestWalletRepo_FindByIDForUpdate_Success(t *testing.T) {
	repo, mock, db, txMgr := setupWalletRepo(t)
	defer db.Close()

	now := time.Now().UTC()
	mock.ExpectBegin()
	rows := sqlmock.NewRows([]string{"id", "balance", "updated_at"}).
		AddRow("w1", int64(1000), now)
	mock.ExpectQuery("SELECT id, balance, updated_at FROM wallets WHERE id = \\$1 FOR UPDATE").
		WithArgs("w1").
		WillReturnRows(rows)
	mock.ExpectCommit()

	var wallet *domain.Wallet
	err := txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		var e error
		wallet, e = repo.FindByIDForUpdate(txCtx, "w1")
		return e
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wallet.ID != "w1" || wallet.Balance != 1000 {
		t.Fatalf("unexpected wallet: %+v", wallet)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWalletRepo_FindByIDForUpdate_NotFound(t *testing.T) {
	repo, mock, db, txMgr := setupWalletRepo(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, balance, updated_at FROM wallets WHERE id = \\$1 FOR UPDATE").
		WithArgs("w999").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	err := txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		_, e := repo.FindByIDForUpdate(txCtx, "w999")
		return e
	})
	if err != domain.ErrWalletNotFound {
		t.Fatalf("expected ErrWalletNotFound, got %v", err)
	}
}

func TestWalletRepo_FindByIDForUpdate_NoTxInContext(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := postgres.NewWalletRepository(db)
	_, e := repo.FindByIDForUpdate(context.Background(), "w1")
	if e == nil {
		t.Fatal("expected error when no tx in context")
	}
}

func TestWalletRepo_UpdateBalance_Success(t *testing.T) {
	repo, mock, db, txMgr := setupWalletRepo(t)
	defer db.Close()

	now := time.Now().UTC()
	w := &domain.Wallet{ID: "w1", Balance: 700, UpdatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets SET balance = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(int64(700), now, "w1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		return repo.UpdateBalance(txCtx, w)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWalletRepo_UpdateBalance_NotFound(t *testing.T) {
	repo, mock, db, txMgr := setupWalletRepo(t)
	defer db.Close()

	now := time.Now().UTC()
	w := &domain.Wallet{ID: "w999", Balance: 700, UpdatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets SET balance = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(int64(700), now, "w999").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		return repo.UpdateBalance(txCtx, w)
	})
	if err != domain.ErrWalletNotFound {
		t.Fatalf("expected ErrWalletNotFound, got %v", err)
	}
}

func TestWalletRepo_UpdateBalance_NoTxInContext(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := postgres.NewWalletRepository(db)
	e := repo.UpdateBalance(context.Background(), &domain.Wallet{ID: "w1", Balance: 100})
	if e == nil {
		t.Fatal("expected error when no tx in context")
	}
}

func TestWalletRepo_FindByIDForUpdate_QueryError(t *testing.T) {
	repo, mock, db, txMgr := setupWalletRepo(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, balance, updated_at FROM wallets WHERE id = \\$1 FOR UPDATE").
		WithArgs("w1").
		WillReturnError(errors.New("connection reset"))
	mock.ExpectRollback()

	err := txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		_, e := repo.FindByIDForUpdate(txCtx, "w1")
		return e
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWalletRepo_UpdateBalance_ExecError(t *testing.T) {
	repo, mock, db, txMgr := setupWalletRepo(t)
	defer db.Close()

	now := time.Now().UTC()
	w := &domain.Wallet{ID: "w1", Balance: 700, UpdatedAt: now}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE wallets SET balance = \\$1, updated_at = \\$2 WHERE id = \\$3").
		WithArgs(int64(700), now, "w1").
		WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()

	err := txMgr.RunInTx(context.Background(), func(txCtx context.Context) error {
		return repo.UpdateBalance(txCtx, w)
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
