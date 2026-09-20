// Package usecase contém a lógica de negócio transacional do P2P.
// Define as portas (interfaces) de saída — repositório e unit of work —
// invertendo a dependência para que a infraestrutura dependa do usecase,
// e nunca o contrário (Clean Architecture / Ports & Adapters).
package usecase

import (
	"context"
	"time"

	"wallet-p2p/internal/domain"
)

// WalletRepository define as operações de persistência de carteiras
// executadas DENTRO de uma transação de banco gerenciada pela UnitOfWork.
type WalletRepository interface {
	// GetByIDForUpdate busca uma carteira adquirindo lock pessimista
	// (SELECT ... FOR UPDATE), bloqueando a linha até o commit/rollback.
	// Retorna domain.ErrWalletNotFound se a carteira não existir.
	GetByIDForUpdate(ctx context.Context, tx Tx, id string) (domain.Wallet, error)

	// UpdateBalance persiste o novo saldo de uma carteira previamente
	// travada por GetByIDForUpdate.
	UpdateBalance(ctx context.Context, tx Tx, wallet domain.Wallet) error

	// InsertTransaction registra o histórico imutável da transferência.
	InsertTransaction(ctx context.Context, tx Tx, transaction domain.Transaction) error
}

// Tx representa uma transação de banco de dados em andamento.
// É um marcador opaco: a implementação concreta (ex: *sql.Tx) fica
// encapsulada na camada de infraestrutura.
type Tx interface {
	// Sealed marca o tipo como uma transação gerenciada pela infraestrutura.
	Sealed()
}

// UnitOfWork coordena uma unidade de trabalho transacional: todas as
// operações executadas dentro de fn compartilham a mesma transação.
// Se fn retornar erro, ocorre rollback total; caso contrário, commit.
type UnitOfWork interface {
	// Do executa fn dentro de uma transação. A propagação de pânico é
	// garantida: rollback é feito antes do repanic.
	Do(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

// Clock abstrai a obtenção do tempo atual, permitindo testes determinísticos.
type Clock interface {
	Now() time.Time
}
