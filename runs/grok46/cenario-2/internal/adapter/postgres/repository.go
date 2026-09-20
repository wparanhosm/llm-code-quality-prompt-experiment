package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wallet/p2p-transfer/internal/domain"
)

// IDGenerator produces unique IDs (injected to keep domain free of crypto in tests).
type IDGenerator interface {
	NewID() string
}

type uuidGen struct{}

func (uuidGen) NewID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixMilli())
}

// TransferRepository implements domain.UnitOfWork with pessimistic locking.
type TransferRepository struct {
	db   *sql.DB
	ids  IDGenerator
	now  func() time.Time
}

func NewTransferRepository(db *sql.DB) *TransferRepository {
	return &TransferRepository{
		db:  db,
		ids: uuidGen{},
		now: func() time.Time { return time.Now().UTC() },
	}
}

func NewTransferRepositoryWithDeps(db *sql.DB, ids IDGenerator, now func() time.Time) *TransferRepository {
	return &TransferRepository{db: db, ids: ids, now: now}
}

func (r *TransferRepository) ExecuteTransfer(ctx context.Context, fromID, toID string, amount domain.Money) (*domain.Transfer, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	firstID, secondID := fromID, toID
	if toID < fromID {
		firstID, secondID = toID, fromID
	}

	wallets := make(map[string]*domain.Wallet, 2)
	for _, id := range []string{firstID, secondID} {
		w, err := r.lockWallet(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		wallets[id] = w
	}

	from := wallets[fromID]
	to := wallets[toID]
	if from == nil {
		return nil, fmt.Errorf("from wallet: %w", domain.ErrWalletNotFound)
	}
	if to == nil {
		return nil, fmt.Errorf("to wallet: %w", domain.ErrWalletNotFound)
	}

	if err := from.Debit(amount); err != nil {
		return nil, err
	}
	if err := to.Credit(amount); err != nil {
		return nil, err
	}

	if err := r.updateBalance(ctx, tx, from); err != nil {
		return nil, err
	}
	if err := r.updateBalance(ctx, tx, to); err != nil {
		return nil, err
	}

	tr, err := domain.NewTransfer(r.ids.NewID(), fromID, toID, amount, r.now())
	if err != nil {
		return nil, fmt.Errorf("new transfer: %w", err)
	}
	if err := r.insertTransfer(ctx, tx, tr); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return tr, nil
}

func (r *TransferRepository) lockWallet(ctx context.Context, tx *sql.Tx, id string) (*domain.Wallet, error) {
	const q = `SELECT id, balance_cents FROM wallets WHERE id = $1 FOR UPDATE`
	var w domain.Wallet
	var cents int64
	err := tx.QueryRowContext(ctx, q, id).Scan(&w.ID, &cents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("wallet %s: %w", id, domain.ErrWalletNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lock wallet %s: %w", id, err)
	}
	w.Balance = domain.Money(cents)
	return &w, nil
}

func (r *TransferRepository) updateBalance(ctx context.Context, tx *sql.Tx, w *domain.Wallet) error {
	const q = `UPDATE wallets SET balance_cents = $1 WHERE id = $2`
	res, err := tx.ExecContext(ctx, q, int64(w.Balance), w.ID)
	if err != nil {
		return fmt.Errorf("update wallet %s: %w", w.ID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected wallet %s: %w", w.ID, err)
	}
	if n != 1 {
		return fmt.Errorf("update wallet %s: expected 1 row, got %d: %w", w.ID, n, domain.ErrWalletNotFound)
	}
	return nil
}

func (r *TransferRepository) insertTransfer(ctx context.Context, tx *sql.Tx, tr *domain.Transfer) error {
	const q = `INSERT INTO transfers (id, from_wallet_id, to_wallet_id, amount_cents, created_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := tx.ExecContext(ctx, q, tr.ID, tr.FromWalletID, tr.ToWalletID, int64(tr.Amount), tr.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert transfer: %w", err)
	}
	return nil
}

// Schema is the PostgreSQL DDL for wallets and transfer history.
const Schema = `
CREATE TABLE IF NOT EXISTS wallets (
    id TEXT PRIMARY KEY,
    balance_cents BIGINT NOT NULL CHECK (balance_cents >= 0)
);
CREATE TABLE IF NOT EXISTS transfers (
    id TEXT PRIMARY KEY,
    from_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    to_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    created_at TIMESTAMPTZ NOT NULL
);
`
