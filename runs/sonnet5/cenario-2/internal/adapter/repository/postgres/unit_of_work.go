// Package postgres implementa os adapters de persistência (camada de
// infraestrutura da Clean Architecture) usando Postgres através do
// database/sql padrão do Go. Nenhuma regra de negócio reside aqui: este
// pacote apenas traduz as portas definidas em internal/usecase para
// operações SQL concretas.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"p2p-transfer/internal/usecase"
)

// UnitOfWork implementa usecase.UnitOfWork usando uma transação nativa do
// database/sql (*sql.Tx), garantindo atomicidade (propriedade "A" do ACID)
// entre o débito, o crédito e a gravação do histórico da transferência.
type UnitOfWork struct {
	db *sql.DB
}

// NewUnitOfWork constrói um UnitOfWork a partir de um pool de conexões já
// configurado e validado (ver internal/platform/database).
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	if db == nil {
		panic("postgres: *sql.DB não pode ser nil")
	}
	return &UnitOfWork{db: db}
}

// Execute abre uma transação SQL com nível de isolamento
// ReadCommitted (suficiente aqui pois a consistência forte é garantida
// explicitamente via SELECT ... FOR UPDATE nas linhas das carteiras
// envolvidas, não pelo nível de isolamento da transação). Se fn retornar
// erro, a transação é revertida (rollback); caso contrário, é commitada.
//
// Qualquer pânico dentro de fn é recuperado, a transação é revertida, e o
// pânico é relançado após o rollback, garantindo que a transação nunca
// fique pendurada (leaked) em caso de erro inesperado.
func (u *UnitOfWork) Execute(ctx context.Context, fn func(ctx context.Context, repos usecase.TransactionalRepositories) error) (err error) {
	tx, err := u.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	repos := &transactionalRepositories{
		walletRepo:      NewWalletRepository(tx),
		transactionRepo: NewTransactionRepository(tx),
	}

	if err = fn(ctx, repos); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("erro na operação (%v) e falha ao reverter transação: %w", err, rbErr)
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("erro ao confirmar transação: %w", err)
	}

	return nil
}

// transactionalRepositories agrupa os repositórios concretos vinculados à
// mesma *sql.Tx corrente, implementando usecase.TransactionalRepositories.
type transactionalRepositories struct {
	walletRepo      *WalletRepository
	transactionRepo *TransactionRepository
}

func (r *transactionalRepositories) WalletRepository() usecase.WalletRepository {
	return r.walletRepo
}

func (r *transactionalRepositories) TransactionRepository() usecase.TransactionRepository {
	return r.transactionRepo
}
