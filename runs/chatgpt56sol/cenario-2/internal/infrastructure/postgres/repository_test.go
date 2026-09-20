package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/p2p-transfer/internal/application"
	"github.com/example/p2p-transfer/internal/domain"
	"github.com/lib/pq"
)

func database(t *testing.T) (*Manager, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewManager(db), mock
}
func TestTransactionAndRepositorySuccess(t *testing.T) {
	manager, mock := database(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, balance_cents FROM wallets`).WithArgs("a", "z").WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", 100).AddRow("z", 200))
	mock.ExpectExec(`UPDATE wallets`).WithArgs(int64(90), "a").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO transfers`).WithArgs("t", "a", "z", int64(10), now).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	err := manager.WithinTransaction(context.Background(), func(ctx context.Context, w application.WalletRepository, tr application.TransferRepository) error {
		wallets, err := w.LockByIDs(ctx, "z", "a")
		if err != nil {
			return err
		}
		wallets["a"].Balance = domain.MoneyFromCents(90)
		if err = w.Update(ctx, wallets["a"]); err != nil {
			return err
		}
		return tr.Create(ctx, &domain.Transfer{ID: "t", SourceWalletID: "a", DestinationWalletID: "z", Amount: domain.MoneyFromCents(10), CreatedAt: now})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestTransactionErrors(t *testing.T) {
	boom := errors.New("boom")
	t.Run("begin", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin().WillReturnError(boom)
		if !errors.Is(m.WithinTransaction(context.Background(), nil), boom) {
			t.Fatal("wanted error")
		}
	})
	t.Run("callback rollback", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(context.Context, application.WalletRepository, application.TransferRepository) error { return boom })
		if !errors.Is(err, boom) {
			t.Fatal(err)
		}
	})
	t.Run("rollback", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectRollback().WillReturnError(errors.New("rollback"))
		if err := m.WithinTransaction(context.Background(), func(context.Context, application.WalletRepository, application.TransferRepository) error { return boom }); err == nil {
			t.Fatal("wanted error")
		}
	})
	t.Run("commit", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectCommit().WillReturnError(boom)
		if !errors.Is(m.WithinTransaction(context.Background(), func(context.Context, application.WalletRepository, application.TransferRepository) error { return nil }), boom) {
			t.Fatal("wanted error")
		}
	})
}
func TestRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")
	t.Run("query", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT`).WillReturnError(boom)
		mock.ExpectRollback()
		_ = m.WithinTransaction(context.Background(), func(ctx context.Context, w application.WalletRepository, _ application.TransferRepository) error {
			_, err := w.LockByIDs(ctx, "a", "b")
			return err
		})
	})
	t.Run("scan", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT`).WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents"}).AddRow("a", "bad"))
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(ctx context.Context, w application.WalletRepository, _ application.TransferRepository) error {
			_, err := w.LockByIDs(ctx, "a", "b")
			return err
		})
		if err == nil {
			t.Fatal("wanted error")
		}
	})
	t.Run("update exec", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE`).WillReturnError(boom)
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(ctx context.Context, w application.WalletRepository, _ application.TransferRepository) error {
			return w.Update(ctx, &domain.Wallet{ID: "a"})
		})
		if !errors.Is(err, boom) {
			t.Fatal(err)
		}
	})
	t.Run("update missing", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE`).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(ctx context.Context, w application.WalletRepository, _ application.TransferRepository) error {
			return w.Update(ctx, &domain.Wallet{ID: "a"})
		})
		if !errors.Is(err, application.ErrWalletNotFound) {
			t.Fatal(err)
		}
	})
	t.Run("rows affected", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectExec(`UPDATE`).WillReturnResult(sqlmock.NewErrorResult(boom))
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(ctx context.Context, w application.WalletRepository, _ application.TransferRepository) error {
			return w.Update(ctx, &domain.Wallet{ID: "a"})
		})
		if !errors.Is(err, boom) {
			t.Fatal(err)
		}
	})
	t.Run("insert", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT`).WillReturnError(boom)
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(ctx context.Context, _ application.WalletRepository, tr application.TransferRepository) error {
			return tr.Create(ctx, &domain.Transfer{ID: "x"})
		})
		if !errors.Is(err, boom) {
			t.Fatal(err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		m, mock := database(t)
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT`).WillReturnError(&pq.Error{Code: "23505"})
		mock.ExpectRollback()
		err := m.WithinTransaction(context.Background(), func(ctx context.Context, _ application.WalletRepository, tr application.TransferRepository) error {
			return tr.Create(ctx, &domain.Transfer{ID: "x"})
		})
		if !errors.Is(err, application.ErrTransferIDCollision) {
			t.Fatal(err)
		}
	})
}
