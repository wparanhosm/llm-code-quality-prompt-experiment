package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

type ctxKey string

const txKey ctxKey = "pg_tx"

type TxManager struct {
	db *sql.DB
}

func NewTxManager(db *sql.DB) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func txFromCtx(ctx context.Context) (*sql.Tx, error) {
	tx, ok := ctx.Value(txKey).(*sql.Tx)
	if !ok || tx == nil {
		return nil, fmt.Errorf("no transaction in context")
	}
	return tx, nil
}
