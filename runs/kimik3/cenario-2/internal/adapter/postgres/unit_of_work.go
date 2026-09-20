// Package postgres implementa as portas de persistência definidas no usecase
// usando database/sql com driver PostgreSQL (lib/pq em modo stdlib).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"wallet-p2p/internal/usecase"
)

// UnitOfWork implementa usecase.UnitOfWork sobre *sql.DB.
// Cada chamada a Do abre uma *sql.Tx no nível de isolamento Read Committed
// (padrão do PostgreSQL e suficiente aqui, pois o lock pessimista
// FOR UPDATE já serializa o acesso às linhas das carteiras).
type UnitOfWork struct {
	db *sql.DB
}

// NewUnitOfWork constrói a UnitOfWork sobre o pool de conexões.
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

// sqlTx adapta *sql.Tx à interface opaca usecase.Tx.
type sqlTx struct {
	inner *sql.Tx
}

func (sqlTx) Sealed() {}

// Do executa fn dentro de uma transação SQL. Garante:
//   - commit somente se fn retornar nil
//   - rollback em qualquer erro retornado por fn
//   - rollback seguido de repanic se fn entrar em pânico
func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx usecase.Tx) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("postgres: falha ao iniciar transação: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(ctx, sqlTx{inner: tx}); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("postgres: falha no rollback após erro %w: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("postgres: falha no commit da transação: %w", err)
	}
	return nil
}

// asSQLTx faz o type assertion seguro de usecase.Tx para o adaptador concreto.
func asSQLTx(tx usecase.Tx) (*sql.Tx, error) {
	stx, ok := tx.(sqlTx)
	if !ok || stx.inner == nil {
		return nil, errors.New("postgres: transação de tipo inesperado (não é *sql.Tx)")
	}
	return stx.inner, nil
}
