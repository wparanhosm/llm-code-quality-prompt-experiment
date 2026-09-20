package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	adapter "github.com/walter/p2p-wallet-transfer/internal/adapter/postgres"
	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

func TestPostgresAdapter_Interfaces(t *testing.T) {
	t.Parallel()

	var _ domain.AccountRepository = (*adapter.AccountRepository)(nil)
	var _ domain.TransferRepository = (*adapter.TransferRepository)(nil)
	var _ domain.UnitOfWork = (*adapter.UnitOfWork)(nil)
	var _ domain.UnitOfWorkRepository = (*adapter.UnitOfWorkRepository)(nil)
}

func TestAccountRepository_GetByID(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := adapter.NewAccountRepository(db)
	now := time.Now().UTC()

	// 1. Success case
	rows := sqlmock.NewRows([]string{"id", "holder", "balance_cents", "status", "created_at", "updated_at"}).
		AddRow(1, "Alice", 5000, "ACTIVE", now, now)
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1`).
		WithArgs(1).
		WillReturnRows(rows)

	acc, err := repo.GetByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc == nil || acc.ID != 1 || acc.Balance.Cents() != 5000 {
		t.Errorf("unexpected account: %+v", acc)
	}

	// 2. Not found case (ErrNoRows)
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1`).
		WithArgs(99).
		WillReturnError(sql.ErrNoRows)

	acc, err = repo.GetByID(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error for not found: %v", err)
	}
	if acc != nil {
		t.Errorf("expected nil account, got %+v", acc)
	}

	// 3. Database error case
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1`).
		WithArgs(500).
		WillReturnError(errors.New("db timeout"))

	_, err = repo.GetByID(context.Background(), 500)
	if err == nil {
		t.Errorf("expected error on DB failure")
	}

	// 4. Invalid balance data (< 0)
	rowsInvalid := sqlmock.NewRows([]string{"id", "holder", "balance_cents", "status", "created_at", "updated_at"}).
		AddRow(2, "Bob", -100, "ACTIVE", now, now)
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1`).
		WithArgs(2).
		WillReturnRows(rowsInvalid)

	_, err = repo.GetByID(context.Background(), 2)
	if err == nil {
		t.Errorf("expected error for invalid negative balance")
	}
}

func TestAccountRepository_GetForUpdate(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := adapter.NewAccountRepository(db)
	now := time.Now().UTC()

	// 1. Success case
	rows := sqlmock.NewRows([]string{"id", "holder", "balance_cents", "status", "created_at", "updated_at"}).
		AddRow(1, "Alice", 5000, "ACTIVE", now, now)
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1 FOR UPDATE`).
		WithArgs(1).
		WillReturnRows(rows)

	acc, err := repo.GetForUpdate(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc == nil || acc.ID != 1 {
		t.Errorf("unexpected account: %+v", acc)
	}

	// 2. Not found
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1 FOR UPDATE`).
		WithArgs(99).
		WillReturnError(sql.ErrNoRows)

	acc, err = repo.GetForUpdate(context.Background(), 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc != nil {
		t.Errorf("expected nil account, got %+v", acc)
	}

	// 3. Query error
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1 FOR UPDATE`).
		WithArgs(500).
		WillReturnError(errors.New("lock timeout"))

	_, err = repo.GetForUpdate(context.Background(), 500)
	if err == nil {
		t.Errorf("expected error on lock timeout")
	}

	// 4. Invalid balance data
	rowsInvalid := sqlmock.NewRows([]string{"id", "holder", "balance_cents", "status", "created_at", "updated_at"}).
		AddRow(2, "Bob", -100, "ACTIVE", now, now)
	mock.ExpectQuery(`SELECT (.+) FROM accounts WHERE id = \$1 FOR UPDATE`).
		WithArgs(2).
		WillReturnRows(rowsInvalid)

	_, err = repo.GetForUpdate(context.Background(), 2)
	if err == nil {
		t.Errorf("expected error for invalid negative balance")
	}
}

func TestAccountRepository_UpdateBalance(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := adapter.NewAccountRepository(db)
	acc, _ := domain.NewAccount(1, "Alice", money.MustNew(5000), domain.AccountStatusActive)

	// 1. Success case
	mock.ExpectExec(`UPDATE accounts SET balance_cents = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(acc.Balance.Cents(), acc.UpdatedAt, acc.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateBalance(context.Background(), acc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2. Exec failure
	mock.ExpectExec(`UPDATE accounts SET balance_cents = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(acc.Balance.Cents(), acc.UpdatedAt, acc.ID).
		WillReturnError(errors.New("connection reset"))

	if err := repo.UpdateBalance(context.Background(), acc); err == nil {
		t.Errorf("expected error on exec failure")
	}

	// 3. Rows affected failure (0 rows)
	mock.ExpectExec(`UPDATE accounts SET balance_cents = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(acc.Balance.Cents(), acc.UpdatedAt, acc.ID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := repo.UpdateBalance(context.Background(), acc); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}

	// 4. Result error
	mock.ExpectExec(`UPDATE accounts SET balance_cents = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(acc.Balance.Cents(), acc.UpdatedAt, acc.ID).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("driver error")))

	if err := repo.UpdateBalance(context.Background(), acc); err == nil {
		t.Errorf("expected error on result error")
	}
}

func TestAccountRepository_Create(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := adapter.NewAccountRepository(db)
	acc, _ := domain.NewAccount(1, "Alice", money.MustNew(5000), domain.AccountStatusActive)

	// 1. Success case
	mock.ExpectExec(`INSERT INTO accounts`).
		WithArgs(acc.ID, acc.Holder, acc.Balance.Cents(), string(acc.Status), acc.CreatedAt, acc.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2. Failure case
	mock.ExpectExec(`INSERT INTO accounts`).
		WithArgs(acc.ID, acc.Holder, acc.Balance.Cents(), string(acc.Status), acc.CreatedAt, acc.UpdatedAt).
		WillReturnError(errors.New("duplicate key"))

	if err := repo.Create(context.Background(), acc); err == nil {
		t.Errorf("expected error on duplicate key")
	}
}

func TestTransferRepository(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := adapter.NewTransferRepository(db)
	now := time.Now().UTC()

	// 1. Create Success
	tr, _ := domain.NewTransfer(1, 2, money.MustNew(1000))
	rows := sqlmock.NewRows([]string{"id"}).AddRow(42)
	mock.ExpectQuery(`INSERT INTO transfers`).
		WithArgs(tr.FromAccountID, tr.ToAccountID, tr.Amount.Cents(), string(tr.Status), tr.CreatedAt).
		WillReturnRows(rows)

	if err := repo.Create(context.Background(), tr); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.ID != 42 {
		t.Errorf("expected transfer ID 42, got %d", tr.ID)
	}

	// 2. Create failure
	mock.ExpectQuery(`INSERT INTO transfers`).
		WithArgs(tr.FromAccountID, tr.ToAccountID, tr.Amount.Cents(), string(tr.Status), tr.CreatedAt).
		WillReturnError(errors.New("db error"))

	if err := repo.Create(context.Background(), tr); err == nil {
		t.Errorf("expected error on DB insert failure")
	}

	// 3. GetByID Success
	getRows := sqlmock.NewRows([]string{"id", "from_account_id", "to_account_id", "amount_cents", "status", "created_at"}).
		AddRow(42, 1, 2, 1000, "COMPLETED", now)
	mock.ExpectQuery(`SELECT (.+) FROM transfers WHERE id = \$1`).
		WithArgs(42).
		WillReturnRows(getRows)

	res, err := repo.GetByID(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ID != 42 || res.Amount.Cents() != 1000 {
		t.Errorf("unexpected transfer: %+v", res)
	}

	// 4. GetByID Not Found
	mock.ExpectQuery(`SELECT (.+) FROM transfers WHERE id = \$1`).
		WithArgs(99).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByID(context.Background(), 99)
	if !errors.Is(err, domain.ErrTransferNotFound) {
		t.Errorf("expected ErrTransferNotFound, got %v", err)
	}

	// 5. GetByID Query error
	mock.ExpectQuery(`SELECT (.+) FROM transfers WHERE id = \$1`).
		WithArgs(500).
		WillReturnError(errors.New("query error"))

	_, err = repo.GetByID(context.Background(), 500)
	if err == nil {
		t.Errorf("expected query error")
	}

	// 6. GetByID Invalid amount (< 0)
	invalidRows := sqlmock.NewRows([]string{"id", "from_account_id", "to_account_id", "amount_cents", "status", "created_at"}).
		AddRow(43, 1, 2, 0, "COMPLETED", now)
	mock.ExpectQuery(`SELECT (.+) FROM transfers WHERE id = \$1`).
		WithArgs(43).
		WillReturnRows(invalidRows)

	_, err = repo.GetByID(context.Background(), 43)
	if err == nil {
		t.Errorf("expected error for invalid amount")
	}
}

func TestUnitOfWork_Execute(t *testing.T) {
	t.Parallel()

	// 1. Successful commit
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	uow := adapter.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = uow.Execute(context.Background(), func(txRepo domain.UnitOfWorkRepository) error {
		if txRepo.Accounts() == nil || txRepo.Transfers() == nil {
			t.Errorf("expected non-nil repos inside UoW")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error in UoW execute: %v", err)
	}

	// 2. Transaction initiation error
	mock.ExpectBegin().WillReturnError(errors.New("begin error"))
	err = uow.Execute(context.Background(), func(txRepo domain.UnitOfWorkRepository) error {
		return nil
	})
	if err == nil {
		t.Errorf("expected error on begin tx failure")
	}

	// 3. Error during execution with rollback
	mock.ExpectBegin()
	mock.ExpectRollback()
	customErr := errors.New("custom domain failure")
	err = uow.Execute(context.Background(), func(txRepo domain.UnitOfWorkRepository) error {
		return customErr
	})
	if !errors.Is(err, customErr) {
		t.Errorf("expected %v, got %v", customErr, err)
	}

	// 4. Commit failure
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("commit conflict"))
	err = uow.Execute(context.Background(), func(txRepo domain.UnitOfWorkRepository) error {
		return nil
	})
	if err == nil {
		t.Errorf("expected error on commit conflict")
	}

	// 5. Panic recovery and rollback
	mock.ExpectBegin()
	mock.ExpectRollback()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic recovery")
		}
	}()
	_ = uow.Execute(context.Background(), func(txRepo domain.UnitOfWorkRepository) error {
		panic("unexpected runtime panic")
	})
}
