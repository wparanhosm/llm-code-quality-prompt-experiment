package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"p2p-wallet/internal/domain"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, t *domain.Transaction) error {
	tx, err := txFromCtx(ctx)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO transactions (id, from_wallet_id, to_wallet_id, amount, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.FromWalletID, t.ToWalletID, t.Amount, t.Status, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}
