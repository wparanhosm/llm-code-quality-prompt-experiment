package transfer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidTransfer   = errors.New("source and destination must be different and amount must be positive")
	ErrWalletNotFound    = errors.New("wallet not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type DB interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

type Service struct {
	db DB
}

type Transfer struct {
	ID            string
	SourceID      string
	DestinationID string
	AmountCents   int64
	CreatedAt     time.Time
}

func NewService(db DB) *Service {
	return &Service{db: db}
}

// Execute atomically locks both wallets, moves the funds and records the transfer.
func (s *Service) Execute(ctx context.Context, sourceID, destinationID string, amountCents int64) (result Transfer, err error) {
	if sourceID == "" || destinationID == "" || sourceID == destinationID || amountCents <= 0 {
		return Transfer{}, ErrInvalidTransfer
	}

	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Transfer{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
	}()

	rows, err := tx.Query(ctx, `
		SELECT id, balance_cents
		FROM wallets
		WHERE id = ANY($1)
		ORDER BY id
		FOR UPDATE`, []string{sourceID, destinationID})
	if err != nil {
		return Transfer{}, fmt.Errorf("lock wallets: %w", err)
	}

	balances := make(map[string]int64, 2)
	for rows.Next() {
		var id string
		var balance int64
		if err := rows.Scan(&id, &balance); err != nil {
			rows.Close()
			return Transfer{}, fmt.Errorf("scan wallet: %w", err)
		}
		balances[id] = balance
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Transfer{}, fmt.Errorf("read wallets: %w", err)
	}
	if len(balances) != 2 {
		return Transfer{}, ErrWalletNotFound
	}
	if balances[sourceID] < amountCents {
		return Transfer{}, ErrInsufficientFunds
	}

	if _, err := tx.Exec(ctx, `UPDATE wallets SET balance_cents = balance_cents - $1, updated_at = now() WHERE id = $2`, amountCents, sourceID); err != nil {
		return Transfer{}, fmt.Errorf("debit source wallet: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE wallets SET balance_cents = balance_cents + $1, updated_at = now() WHERE id = $2`, amountCents, destinationID); err != nil {
		return Transfer{}, fmt.Errorf("credit destination wallet: %w", err)
	}

	result.SourceID = sourceID
	result.DestinationID = destinationID
	result.AmountCents = amountCents
	if err := tx.QueryRow(ctx, `
		INSERT INTO transfers (source_wallet_id, destination_wallet_id, amount_cents)
		VALUES ($1, $2, $3)
		RETURNING id::text, created_at`, sourceID, destinationID, amountCents).Scan(&result.ID, &result.CreatedAt); err != nil {
		return Transfer{}, fmt.Errorf("record transfer: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Transfer{}, fmt.Errorf("commit transaction: %w", err)
	}
	return result, nil
}
