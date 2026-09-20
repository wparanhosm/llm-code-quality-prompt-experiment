//go:build integration

// Testes de integração contra PostgreSQL real (via docker-compose).
// Execução: go test -tags=integration ./internal/adapter/postgres/... \
//
//	-with env DATABASE_DSN_TEST apontando para o banco do compose.
package postgres_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/adapter/postgres"
	"wallet-p2p/internal/domain"
	"wallet-p2p/internal/usecase"
)

func integrationDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("DATABASE_DSN_TEST")
	if dsn == "" {
		dsn = "postgres://wallet:wallet@localhost:5432/wallet_p2p?sslmode=disable"
	}
	return dsn
}

func openIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", integrationDSN(t))
	require.NoError(t, err)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Minute)
	require.NoError(t, db.Ping())
	t.Cleanup(func() { db.Close() })
	return db
}

func createWallet(t *testing.T, db *sql.DB, balanceCents int64) string {
	t.Helper()
	id := uuid.NewString()
	_, err := db.Exec(`INSERT INTO wallets (id, balance_cents) VALUES ($1, $2)`, id, balanceCents)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM transactions WHERE from_wallet_id = $1 OR to_wallet_id = $1`, id)
		_, _ = db.Exec(`DELETE FROM wallets WHERE id = $1`, id)
	})
	return id
}

func walletBalance(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	var cents int64
	require.NoError(t, db.QueryRow(`SELECT balance_cents FROM wallets WHERE id = $1`, id).Scan(&cents))
	return cents
}

func TestIntegration_Transfer_EndToEnd(t *testing.T) {
	db := openIntegrationDB(t)

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()
	uc := usecase.NewTransferUseCase(uow, repo, usecase.NewSystemClock(), testLogger())

	fromID := createWallet(t, db, 1000)
	toID := createWallet(t, db, 500)

	out, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: fromID,
		ToWalletID:   toID,
		AmountCents:  250,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, out.TransactionID)
	assert.Equal(t, int64(750), out.FromNewBalanceCents)
	assert.Equal(t, int64(750), out.ToNewBalanceCents)

	assert.Equal(t, int64(750), walletBalance(t, db, fromID))
	assert.Equal(t, int64(750), walletBalance(t, db, toID))

	var txCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE id = $1`, out.TransactionID).Scan(&txCount))
	assert.Equal(t, 1, txCount, "histórico da transação deve estar persistido")
}

func TestIntegration_Transfer_InsufficientFunds_RollsBack(t *testing.T) {
	db := openIntegrationDB(t)

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()
	uc := usecase.NewTransferUseCase(uow, repo, usecase.NewSystemClock(), testLogger())

	fromID := createWallet(t, db, 100)
	toID := createWallet(t, db, 500)

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: fromID,
		ToWalletID:   toID,
		AmountCents:  101,
	})
	require.ErrorIs(t, err, domain.ErrInsufficientFunds)

	assert.Equal(t, int64(100), walletBalance(t, db, fromID), "rollback: saldo da origem intacto")
	assert.Equal(t, int64(500), walletBalance(t, db, toID), "rollback: saldo do destino intacto")
}

func TestIntegration_Transfer_WalletNotFound(t *testing.T) {
	db := openIntegrationDB(t)

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()
	uc := usecase.NewTransferUseCase(uow, repo, usecase.NewSystemClock(), testLogger())

	fromID := createWallet(t, db, 1000)

	_, err := uc.Execute(context.Background(), usecase.TransferInput{
		FromWalletID: fromID,
		ToWalletID:   uuid.NewString(), // inexistente
		AmountCents:  100,
	})
	require.ErrorIs(t, err, domain.ErrWalletNotFound)
}

// TestIntegration_ConcurrentTransfers_NoDoubleSpend dispara transferências
// concorrentes (incluindo cruzadas A→B e B→A, cenário clássico de deadlock)
// e verifica a conservação do valor total: nenhum centavo é criado/destruído.
func TestIntegration_ConcurrentTransfers_NoDoubleSpend(t *testing.T) {
	db := openIntegrationDB(t)

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()
	uc := usecase.NewTransferUseCase(uow, repo, usecase.NewSystemClock(), testLogger())

	fromID := createWallet(t, db, 10000)
	toID := createWallet(t, db, 10000)
	totalBefore := walletBalance(t, db, fromID) + walletBalance(t, db, toID)

	const workers = 40
	var wg sync.WaitGroup
	errs := make(chan error, workers*2)

	for i := 0; i < workers; i++ {
		wg.Add(2)
		// A→B
		go func() {
			defer wg.Done()
			_, err := uc.Execute(context.Background(), usecase.TransferInput{
				FromWalletID: fromID, ToWalletID: toID, AmountCents: 1,
			})
			errs <- err
		}()
		// B→A (cruzada: valida a ordenação determinística de locks)
		go func() {
			defer wg.Done()
			_, err := uc.Execute(context.Background(), usecase.TransferInput{
				FromWalletID: toID, ToWalletID: fromID, AmountCents: 1,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err, "nenhuma transferência deve falhar (locks ordenados evitam deadlock)")
	}

	totalAfter := walletBalance(t, db, fromID) + walletBalance(t, db, toID)
	assert.Equal(t, totalBefore, totalAfter,
		"conservação de valor: o total deve ser idêntico após %d transferências concorrentes cruzadas", workers*2)
}

// TestIntegration_TransactionInsertFailure_RollsBack valida atomicidade total:
// forçamos uma falha no INSERT do histórico (violação de FK) e conferimos
// que os saldos NÃO foram alterados.
func TestIntegration_TransactionInsertFailure_RollsBack(t *testing.T) {
	db := openIntegrationDB(t)

	uow := postgres.NewUnitOfWork(db)
	repo := postgres.NewWalletRepository()

	fromID := createWallet(t, db, 1000)
	toID := createWallet(t, db, 500)

	err := uow.Do(context.Background(), func(ctx context.Context, tx usecase.Tx) error {
		from, err := repo.GetByIDForUpdate(ctx, tx, fromID)
		require.NoError(t, err)
		to, err := repo.GetByIDForUpdate(ctx, tx, toID)
		require.NoError(t, err)

		now := time.Now().UTC()
		from, err = from.Debit(domain.MustMoney(300), now)
		require.NoError(t, err)
		to = to.Credit(domain.MustMoney(300), now)

		require.NoError(t, repo.UpdateBalance(ctx, tx, from))
		require.NoError(t, repo.UpdateBalance(ctx, tx, to))

		// Transação com FK inválida força erro no INSERT → rollback total.
		badTx := domain.ReconstructTransaction(uuid.NewString(), uuid.NewString(), uuid.NewString(), 300, domain.TransactionStatusCompleted, "", now)
		return repo.InsertTransaction(ctx, tx, badTx)
	})
	require.Error(t, err, "INSERT com FK inválida deve falhar")

	assert.Equal(t, int64(1000), walletBalance(t, db, fromID), "rollback total: débito desfeito")
	assert.Equal(t, int64(500), walletBalance(t, db, toID), "rollback total: crédito desfeito")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
