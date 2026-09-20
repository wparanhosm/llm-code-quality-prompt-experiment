package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/adapter/postgres"
	"wallet-p2p/internal/domain"
	"wallet-p2p/internal/usecase"
)

var testNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

// runInUoW abre uma transação via UnitOfWork (com expectativa de Begin no
// sqlmock) e entrega o usecase.Tx para a função de teste.
func runInUoW(t *testing.T, uow *postgres.UnitOfWork, fn func(ctx context.Context, tx usecase.Tx) error) error {
	t.Helper()
	return uow.Do(context.Background(), fn)
}

func TestUnitOfWork_Do_CommitOnSuccess(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectCommit()

	err = runInUoW(t, uow, func(_ context.Context, _ usecase.Tx) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_Do_RollbackOnError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectRollback()

	bizErr := errors.New("falha de negócio")
	err = runInUoW(t, uow, func(_ context.Context, _ usecase.Tx) error {
		return bizErr
	})
	require.ErrorIs(t, err, bizErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_Do_RollbackErrorIsWrapped(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectRollback().WillReturnError(errors.New("falha no rollback"))

	bizErr := errors.New("falha de negócio")
	err = runInUoW(t, uow, func(_ context.Context, _ usecase.Tx) error {
		return bizErr
	})
	require.ErrorIs(t, err, bizErr)
	require.ErrorContains(t, err, "falha no rollback")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_Do_RollbackOnPanicAndRepanic(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectRollback()

	require.PanicsWithValue(t, "boom", func() {
		_ = runInUoW(t, uow, func(_ context.Context, _ usecase.Tx) error {
			panic("boom")
		})
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_Do_BeginFailure(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin().WillReturnError(errors.New("banco indisponível"))

	err = runInUoW(t, uow, func(_ context.Context, _ usecase.Tx) error {
		return nil
	})
	require.ErrorContains(t, err, "falha ao iniciar transação")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_Do_CommitFailure(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)

	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(errors.New("deadlock detectado"))

	err = runInUoW(t, uow, func(_ context.Context, _ usecase.Tx) error {
		return nil
	})
	require.ErrorContains(t, err, "falha no commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- WalletRepository ---------------------------------------------------------

const (
	selectForUpdatePattern = `SELECT id, balance_cents, created_at, updated_at FROM wallets WHERE id = \$1 FOR UPDATE`
	updateBalancePattern   = `UPDATE wallets SET balance_cents = \$2, updated_at = \$3 WHERE id = \$1`
	insertTxPattern        = `INSERT INTO transactions \(id, from_wallet_id, to_wallet_id, amount_cents, status, idempotency_key, created_at\) VALUES \(\$1, \$2, \$3, \$4, \$5, NULLIF\(\$6, ''\), \$7\)`
)

func TestWalletRepository_GetByIDForUpdate_Success(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	mock.ExpectBegin()
	mock.ExpectQuery(selectForUpdatePattern).
		WithArgs("w-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents", "created_at", "updated_at"}).
			AddRow("w-1", 1000, testNow, testNow))
	mock.ExpectCommit()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		w, err := repo.GetByIDForUpdate(ctx, tx, "w-1")
		require.NoError(t, err)
		assert.Equal(t, "w-1", w.ID())
		assert.Equal(t, int64(1000), w.Balance().Cents())
		assert.Equal(t, testNow, w.CreatedAt())
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_GetByIDForUpdate_NotFound(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	mock.ExpectBegin()
	mock.ExpectQuery(selectForUpdatePattern).
		WithArgs("missing").
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance_cents", "created_at", "updated_at"}))
	mock.ExpectRollback()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		_, err := repo.GetByIDForUpdate(ctx, tx, "missing")
		require.ErrorIs(t, err, domain.ErrWalletNotFound)
		return err
	})
	require.ErrorIs(t, err, domain.ErrWalletNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_GetByIDForUpdate_QueryError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	mock.ExpectBegin()
	mock.ExpectQuery(selectForUpdatePattern).
		WithArgs("w-1").
		WillReturnError(errors.New("conexão resetada"))
	mock.ExpectRollback()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		_, err := repo.GetByIDForUpdate(ctx, tx, "w-1")
		require.ErrorContains(t, err, "conexão resetada")
		return err
	})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_UpdateBalance_Success(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	wallet := domain.ReconstructWallet("w-1", 750, testNow, testNow)

	mock.ExpectBegin()
	mock.ExpectExec(updateBalancePattern).
		WithArgs("w-1", int64(750), testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		return repo.UpdateBalance(ctx, tx, wallet)
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_UpdateBalance_NotFound(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	wallet := domain.ReconstructWallet("w-gone", 750, testNow, testNow)

	mock.ExpectBegin()
	mock.ExpectExec(updateBalancePattern).
		WithArgs("w-gone", int64(750), testNow).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		return repo.UpdateBalance(ctx, tx, wallet)
	})
	require.ErrorIs(t, err, domain.ErrWalletNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_UpdateBalance_ExecError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	wallet := domain.ReconstructWallet("w-1", 750, testNow, testNow)

	mock.ExpectBegin()
	mock.ExpectExec(updateBalancePattern).
		WithArgs("w-1", int64(750), testNow).
		WillReturnError(errors.New("violação de constraint CHECK"))
	mock.ExpectRollback()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		return repo.UpdateBalance(ctx, tx, wallet)
	})
	require.ErrorContains(t, err, "violação de constraint CHECK")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_InsertTransaction_Success(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	txn := domain.ReconstructTransaction("tx-1", "w-from", "w-to", 250, domain.TransactionStatusCompleted, "idem-1", testNow)

	mock.ExpectBegin()
	mock.ExpectExec(insertTxPattern).
		WithArgs("tx-1", "w-from", "w-to", int64(250), "COMPLETED", "idem-1", testNow).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		return repo.InsertTransaction(ctx, tx, txn)
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_InsertTransaction_Error(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	txn := domain.ReconstructTransaction("tx-1", "w-from", "w-to", 250, domain.TransactionStatusCompleted, "idem-dup", testNow)

	mock.ExpectBegin()
	mock.ExpectExec(insertTxPattern).
		WithArgs("tx-1", "w-from", "w-to", int64(250), "COMPLETED", "idem-dup", testNow).
		WillReturnError(errors.New("duplicate key value violates unique constraint"))
	mock.ExpectRollback()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		return repo.InsertTransaction(ctx, tx, txn)
	})
	require.ErrorContains(t, err, "duplicate key")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWalletRepository_UpdateBalance_RowsAffectedError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	wallet := domain.ReconstructWallet("w-1", 750, testNow, testNow)

	mock.ExpectBegin()
	mock.ExpectExec(updateBalancePattern).
		WithArgs("w-1", int64(750), testNow).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("driver não suporta RowsAffected")))
	mock.ExpectRollback()

	err = runInUoW(t, uow, func(ctx context.Context, tx usecase.Tx) error {
		return repo.UpdateBalance(ctx, tx, wallet)
	})
	require.ErrorContains(t, err, "linhas afetadas")
	require.NoError(t, mock.ExpectationsWereMet())
}

// --- Tx de tipo inesperado (caminho defensivo de asSQLTx) --------------------

type foreignTx struct{}

func (foreignTx) Sealed() {}

func TestRepository_RejectsForeignTxType(t *testing.T) {
	t.Parallel()

	repo := postgres.NewWalletRepository()
	ctx := context.Background()
	wallet := domain.ReconstructWallet("w-1", 100, testNow, testNow)
	txn := domain.ReconstructTransaction("tx-1", "w-1", "w-2", 100, domain.TransactionStatusCompleted, "", testNow)

	_, err := repo.GetByIDForUpdate(ctx, foreignTx{}, "w-1")
	require.ErrorContains(t, err, "tipo inesperado")

	err = repo.UpdateBalance(ctx, foreignTx{}, wallet)
	require.ErrorContains(t, err, "tipo inesperado")

	err = repo.InsertTransaction(ctx, foreignTx{}, txn)
	require.ErrorContains(t, err, "tipo inesperado")
}

func TestSqlTx_SealedMarker(t *testing.T) {
	t.Parallel()
	// Garante que o marcador da interface é exercitável via interface pública.
	var _ usecase.Tx = foreignTx{}
}
