package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wallet/p2p-transfer/internal/domain"
)

type fixedID struct{ id string }

func (f fixedID) NewID() string { return f.id }

func TestExecuteTransferSuccessLockOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "tr-1"}, func() time.Time { return now })

	mock.ExpectBegin()
	// from=w2 to=w1 => lock order w1 then w2
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("w1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("w1", 50))
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("w2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("w2", 100))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(90), "w2").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE wallets SET balance_cents = \$1 WHERE id = \$2`).
		WithArgs(int64(60), "w1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO transfers`).
		WithArgs("tr-1", "w2", "w1", int64(10), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tr, err := repo.ExecuteTransfer(context.Background(), "w2", "w1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if tr.ID != "tr-1" || tr.Amount != 10 {
		t.Fatalf("unexpected %+v", tr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteTransferWalletNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "x"}, time.Now)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 10)
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestExecuteTransferInsufficientFunds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "x"}, time.Now)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", 5))
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("b").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("b", 0))
	mock.ExpectRollback()

	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 10)
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("got %v", err)
	}
}

func TestExecuteTransferBeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewTransferRepository(db)
	mock.ExpectBegin().WillReturnError(errors.New("begin fail"))
	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUUIDGenNewID(t *testing.T) {
	g := uuidGen{}
	if g.NewID() == "" {
		t.Fatal("empty id")
	}
}

func TestNewTransferRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewTransferRepository(db)
	if r.db == nil || r.ids == nil || r.now == nil {
		t.Fatal("incomplete repo")
	}
	_ = r.now()
}

func TestLockWalletQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := NewTransferRepositoryWithDeps(db, fixedID{id: "x"}, time.Now)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets WHERE id = \$1 FOR UPDATE`).
		WithArgs("a").
		WillReturnError(errors.New("db down"))
	mock.ExpectRollback()
	_, err = repo.ExecuteTransfer(context.Background(), "a", "b", 1)
	if err == nil {
		t.Fatal("expected error")
	}
}
