package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestUpdateBalanceRowsAffectedZero(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "x"}, time.Now)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", 100))
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("b").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("b", 0))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(90), "a").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInsertTransferError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "tr"}, func() time.Time { return now })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", 100))
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("b").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("b", 0))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(90), "a").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(10), "b").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO transfers`).
		WillReturnError(errors.New("insert fail"))
	mock.ExpectRollback()

	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "tr"}, func() time.Time { return now })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", 100))
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("b").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("b", 0))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(90), "a").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(10), "b").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO transfers`).
		WithArgs("tr", "a", "b", int64(10), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit fail"))

	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateBalanceExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "x"}, time.Now)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", 100))
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("b").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("b", 0))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(90), "a").
		WillReturnError(errors.New("upd fail"))
	mock.ExpectRollback()

	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}
