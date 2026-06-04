package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"p2p-wallet/internal/domain"
)

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) FindByIDForUpdate(ctx context.Context, id string) (*domain.Wallet, error) {
	tx, err := txFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	w := &domain.Wallet{}
	err = tx.QueryRowContext(ctx,
		"SELECT id, balance, updated_at FROM wallets WHERE id = $1 FOR UPDATE",
		id,
	).Scan(&w.ID, &w.Balance, &w.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query wallet %s: %w", id, err)
	}
	return w, nil
}

func (r *WalletRepository) UpdateBalance(ctx context.Context, wallet *domain.Wallet) error {
	tx, err := txFromCtx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx,
		"UPDATE wallets SET balance = $1, updated_at = $2 WHERE id = $3",
		wallet.Balance, wallet.UpdatedAt, wallet.ID,
	)
	if err != nil {
		return fmt.Errorf("update wallet %s: %w", wallet.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrWalletNotFound
	}
	return nil
}
