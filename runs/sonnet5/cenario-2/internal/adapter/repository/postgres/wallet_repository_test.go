package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"p2p-transfer/internal/adapter/repository/postgres"
	"p2p-transfer/internal/domain"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("erro ao criar sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestWalletRepository_LockForUpdate_Success(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	rows := sqlmock.NewRows([]string{"id", "owner_id", "balance", "status"}).
		AddRow("wallet-a", "owner-a", int64(10000), "ACTIVE").
		AddRow("wallet-b", "owner-b", int64(500), "ACTIVE")

	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets WHERE id IN \(\$1, \$2\) ORDER BY id FOR UPDATE`).
		WithArgs("wallet-a", "wallet-b").
		WillReturnRows(rows)

	result, err := repo.LockForUpdate(context.Background(), []string{"wallet-a", "wallet-b"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("esperava 2 carteiras, obteve %d", len(result))
	}
	if result["wallet-a"].Balance != 10000 {
		t.Errorf("saldo wallet-a = %d, want 10000", result["wallet-a"].Balance)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas do mock não atendidas: %v", err)
	}
}

func TestWalletRepository_LockForUpdate_DeduplicatesAndSortsIDs(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	rows := sqlmock.NewRows([]string{"id", "owner_id", "balance", "status"}).
		AddRow("wallet-a", "owner-a", int64(10000), "ACTIVE").
		AddRow("wallet-b", "owner-b", int64(500), "ACTIVE")

	// IDs fornecidos fora de ordem e com duplicata; a query deve ser
	// montada com IDs únicos e ordenados (wallet-a, wallet-b).
	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets WHERE id IN \(\$1, \$2\) ORDER BY id FOR UPDATE`).
		WithArgs("wallet-a", "wallet-b").
		WillReturnRows(rows)

	_, err := repo.LockForUpdate(context.Background(), []string{"wallet-b", "wallet-a", "wallet-b"})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas do mock não atendidas: %v", err)
	}
}

func TestWalletRepository_LockForUpdate_EmptyIDs(t *testing.T) {
	t.Parallel()

	db, _ := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	result, err := repo.LockForUpdate(context.Background(), nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("esperava mapa vazio, obteve %d itens", len(result))
	}
}

func TestWalletRepository_LockForUpdate_WalletNotFound(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	rows := sqlmock.NewRows([]string{"id", "owner_id", "balance", "status"}).
		AddRow("wallet-a", "owner-a", int64(10000), "ACTIVE")

	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets`).
		WithArgs("wallet-a", "wallet-b").
		WillReturnRows(rows)

	_, err := repo.LockForUpdate(context.Background(), []string{"wallet-a", "wallet-b"})
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("esperava ErrWalletNotFound, obteve %v", err)
	}
}

func TestWalletRepository_LockForUpdate_QueryError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets`).
		WithArgs("wallet-a").
		WillReturnError(errors.New("conexão perdida"))

	_, err := repo.LockForUpdate(context.Background(), []string{"wallet-a"})
	if err == nil {
		t.Fatal("esperava erro de query propagado")
	}
}

func TestWalletRepository_LockForUpdate_ScanError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	// Retorna uma linha com tipo incompatível para provocar erro de Scan.
	rows := sqlmock.NewRows([]string{"id", "owner_id", "balance", "status"}).
		AddRow("wallet-a", "owner-a", "não-é-um-número", "ACTIVE")

	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets`).
		WithArgs("wallet-a").
		WillReturnRows(rows)

	_, err := repo.LockForUpdate(context.Background(), []string{"wallet-a"})
	if err == nil {
		t.Fatal("esperava erro de scan propagado")
	}
}

func TestWalletRepository_LockForUpdate_InvalidWalletData(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	rows := sqlmock.NewRows([]string{"id", "owner_id", "balance", "status"}).
		AddRow("wallet-a", "owner-a", int64(1000), "STATUS_INVALIDO")

	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets`).
		WithArgs("wallet-a").
		WillReturnRows(rows)

	_, err := repo.LockForUpdate(context.Background(), []string{"wallet-a"})
	if err == nil {
		t.Fatal("esperava erro para status de carteira inválido vindo do banco")
	}
}

func TestWalletRepository_LockForUpdate_RowsErr(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	rows := sqlmock.NewRows([]string{"id", "owner_id", "balance", "status"}).
		AddRow("wallet-a", "owner-a", int64(1000), "ACTIVE").
		RowError(0, errors.New("erro ao iterar linha"))

	mock.ExpectQuery(`SELECT id, owner_id, balance, status FROM wallets`).
		WithArgs("wallet-a").
		WillReturnRows(rows)

	_, err := repo.LockForUpdate(context.Background(), []string{"wallet-a"})
	if err == nil {
		t.Fatal("esperava erro de iteração de linhas propagado")
	}
}

func TestWalletRepository_Save_Success(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	wallet, _ := domain.NewWallet("wallet-a", "owner-a", 5000, domain.WalletStatusActive)

	mock.ExpectExec(`UPDATE wallets SET balance = \$1, status = \$2, updated_at = now\(\) WHERE id = \$3`).
		WithArgs(int64(5000), "ACTIVE", "wallet-a").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Save(context.Background(), wallet); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectativas do mock não atendidas: %v", err)
	}
}

func TestWalletRepository_Save_ExecError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	wallet, _ := domain.NewWallet("wallet-a", "owner-a", 5000, domain.WalletStatusActive)

	mock.ExpectExec(`UPDATE wallets`).
		WithArgs(int64(5000), "ACTIVE", "wallet-a").
		WillReturnError(errors.New("erro de conexão"))

	err := repo.Save(context.Background(), wallet)
	if err == nil {
		t.Fatal("esperava erro propagado")
	}
}

func TestWalletRepository_Save_NoRowsAffected(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	wallet, _ := domain.NewWallet("wallet-a", "owner-a", 5000, domain.WalletStatusActive)

	mock.ExpectExec(`UPDATE wallets`).
		WithArgs(int64(5000), "ACTIVE", "wallet-a").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Save(context.Background(), wallet)
	if !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("esperava ErrWalletNotFound, obteve %v", err)
	}
}

func TestWalletRepository_Save_RowsAffectedError(t *testing.T) {
	t.Parallel()

	db, mock := newMockDB(t)
	repo := postgres.NewWalletRepository(db)

	wallet, _ := domain.NewWallet("wallet-a", "owner-a", 5000, domain.WalletStatusActive)

	mock.ExpectExec(`UPDATE wallets`).
		WithArgs(int64(5000), "ACTIVE", "wallet-a").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("driver não suporta RowsAffected")))

	err := repo.Save(context.Background(), wallet)
	if err == nil {
		t.Fatal("esperava erro de RowsAffected propagado")
	}
}

func TestNewUnitOfWork_PanicsOnNilDB(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("esperava panic com *sql.DB nil")
		}
	}()
	postgres.NewUnitOfWork(nil)
}
