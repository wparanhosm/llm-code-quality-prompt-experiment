package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/example/p2p-transfer/internal/application"
	"github.com/example/p2p-transfer/internal/domain"
	"github.com/lib/pq"
)

type Manager struct{ db *sql.DB }

func NewManager(db *sql.DB) *Manager { return &Manager{db: db} }

func (m *Manager) WithinTransaction(ctx context.Context, fn func(context.Context, application.WalletRepository, application.TransferRepository) error) error {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	repo := &Repository{tx: tx}
	if err = fn(ctx, repo, repo); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return fmt.Errorf("rollback transaction after %v: %w", err, rollbackErr)
		}
		return fmt.Errorf("transaction callback: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type Repository struct{ tx *sql.Tx }

func (r *Repository) LockByIDs(ctx context.Context, first, second string) (map[string]*domain.Wallet, error) {
	ids := []string{first, second}
	sort.Strings(ids)
	rows, err := r.tx.QueryContext(ctx, `SELECT id, balance_cents FROM wallets WHERE id IN ($1, $2) ORDER BY id FOR UPDATE`, ids[0], ids[1])
	if err != nil {
		return nil, fmt.Errorf("query wallets for update: %w", err)
	}
	defer rows.Close()
	wallets := make(map[string]*domain.Wallet, 2)
	for rows.Next() {
		var id string
		var cents int64
		if err = rows.Scan(&id, &cents); err != nil {
			return nil, fmt.Errorf("scan locked wallet: %w", err)
		}
		wallets[id] = &domain.Wallet{ID: id, Balance: domain.MoneyFromCents(cents)}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked wallets: %w", err)
	}
	return wallets, nil
}

func (r *Repository) Update(ctx context.Context, wallet *domain.Wallet) error {
	result, err := r.tx.ExecContext(ctx, `UPDATE wallets SET balance_cents = $1, updated_at = NOW() WHERE id = $2`, wallet.Balance.Cents(), wallet.ID)
	if err != nil {
		return fmt.Errorf("update wallet %s: %w", wallet.ID, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated wallet count: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("update wallet %s: %w", wallet.ID, application.ErrWalletNotFound)
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, transfer *domain.Transfer) error {
	_, err := r.tx.ExecContext(ctx, `INSERT INTO transfers (id, source_wallet_id, destination_wallet_id, amount_cents, created_at) VALUES ($1, $2, $3, $4, $5)`, transfer.ID, transfer.SourceWalletID, transfer.DestinationWalletID, transfer.Amount.Cents(), transfer.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return fmt.Errorf("insert transfer %s: %w", transfer.ID, application.ErrTransferIDCollision)
		}
		return fmt.Errorf("insert transfer %s: %w", transfer.ID, err)
	}
	return nil
}
