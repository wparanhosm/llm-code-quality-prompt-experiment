package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"wallet-p2p/internal/domain"
	"wallet-p2p/internal/usecase"
)

// WalletRepository implementa usecase.WalletRepository com database/sql.
// Todas as operações recebem a transação (usecase.Tx) aberta pela UnitOfWork,
// garantindo que débito + crédito + histórico sejam atômicos.
type WalletRepository struct{}

// NewWalletRepository constrói o repositório de carteiras.
func NewWalletRepository() *WalletRepository {
	return &WalletRepository{}
}

const getWalletForUpdateQuery = `
SELECT id, balance_cents, created_at, updated_at
FROM wallets
WHERE id = $1
FOR UPDATE`

// GetByIDForUpdate busca a carteira adquirindo lock pessimista de linha
// (SELECT ... FOR UPDATE). A linha permanece bloqueada até o fim da transação.
func (r *WalletRepository) GetByIDForUpdate(ctx context.Context, tx usecase.Tx, id string) (domain.Wallet, error) {
	sqlTx, err := asSQLTx(tx)
	if err != nil {
		return domain.Wallet{}, fmt.Errorf("postgres: GetByIDForUpdate: %w", err)
	}

	var (
		walletID      string
		balanceCents  int64
		createdAt     time.Time
		updatedAt     time.Time
	)
	row := sqlTx.QueryRowContext(ctx, getWalletForUpdateQuery, id)
	if err := row.Scan(&walletID, &balanceCents, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Wallet{}, fmt.Errorf("postgres: carteira %s: %w", id, domain.ErrWalletNotFound)
		}
		return domain.Wallet{}, fmt.Errorf("postgres: falha ao consultar carteira %s: %w", id, err)
	}

	return domain.ReconstructWallet(walletID, balanceCents, createdAt, updatedAt), nil
}

const updateBalanceQuery = `
UPDATE wallets
SET balance_cents = $2, updated_at = $3
WHERE id = $1`

// UpdateBalance persiste o saldo atualizado da carteira. O optimistic check
// de linha afetada protege contra inconsistências inesperadas.
func (r *WalletRepository) UpdateBalance(ctx context.Context, tx usecase.Tx, wallet domain.Wallet) error {
	sqlTx, err := asSQLTx(tx)
	if err != nil {
		return fmt.Errorf("postgres: UpdateBalance: %w", err)
	}

	res, err := sqlTx.ExecContext(ctx, updateBalanceQuery, wallet.ID(), wallet.Balance().Cents(), wallet.UpdatedAt())
	if err != nil {
		return fmt.Errorf("postgres: falha ao atualizar saldo da carteira %s: %w", wallet.ID(), err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("postgres: falha ao verificar linhas afetadas: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("postgres: carteira %s: %w", wallet.ID(), domain.ErrWalletNotFound)
	}
	return nil
}

const insertTransactionQuery = `
INSERT INTO transactions (id, from_wallet_id, to_wallet_id, amount_cents, status, idempotency_key, created_at)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)`

// InsertTransaction registra o histórico imutável da transferência.
func (r *WalletRepository) InsertTransaction(ctx context.Context, tx usecase.Tx, transaction domain.Transaction) error {
	sqlTx, err := asSQLTx(tx)
	if err != nil {
		return fmt.Errorf("postgres: InsertTransaction: %w", err)
	}

	if _, err := sqlTx.ExecContext(ctx, insertTransactionQuery,
		transaction.ID(),
		transaction.FromWalletID(),
		transaction.ToWalletID(),
		transaction.Amount().Cents(),
		string(transaction.Status()),
		transaction.IdempotencyKey(),
		transaction.CreatedAt(),
	); err != nil {
		return fmt.Errorf("postgres: falha ao inserir transação %s: %w", transaction.ID(), err)
	}
	return nil
}
