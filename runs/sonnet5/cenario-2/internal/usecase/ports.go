// Package usecase contém a lógica de aplicação (regras de negócio de mais
// alto nível) do sistema de transferências P2P. Esta camada depende apenas
// do domínio e de interfaces (portas) que serão implementadas pela camada
// de infraestrutura (adapters), respeitando a Regra de Dependência da
// Clean Architecture: o núcleo nunca conhece detalhes externos.
package usecase

import (
	"context"
	"time"

	"p2p-transfer/internal/domain"
)

// IDGenerator abstrai a geração de identificadores únicos (ex: UUID) para
// que o usecase permaneça independente de qualquer biblioteca concreta.
type IDGenerator interface {
	NewID() string
}

// Clock abstrai a obtenção do instante atual, permitindo testes
// determinísticos de código sensível ao tempo.
type Clock interface {
	Now() time.Time
}

// UnitOfWork representa uma transação atômica de persistência que abrange
// múltiplas operações de repositório (débito, crédito e histórico). A
// implementação concreta (ex: baseada em *sql.Tx) garante propriedades
// ACID: todas as operações executadas dentro de Execute são commitadas
// juntas, ou nenhuma delas é (rollback automático em caso de erro).
//
// A ordenação de aquisição de locks pelas camadas de infraestrutura
// (SELECT FOR UPDATE) é responsabilidade da implementação de
// WalletRepository fornecida dentro do contexto transacional.
type UnitOfWork interface {
	// Execute roda fn dentro de uma transação de banco de dados. O
	// TransactionalRepositories fornecido a fn deve ser usado para todas as
	// operações de leitura/escrita que precisam fazer parte da mesma
	// transação atômica. Se fn retornar erro, a transação é revertida
	// (rollback); caso contrário, é commitada.
	Execute(ctx context.Context, fn func(ctx context.Context, repos TransactionalRepositories) error) error
}

// TransactionalRepositories agrupa os repositórios que devem operar dentro
// do escopo de uma única transação de banco de dados (*sql.Tx).
type TransactionalRepositories interface {
	WalletRepository() WalletRepository
	TransactionRepository() TransactionRepository
}

// WalletRepository é a porta de acesso a dados de carteiras digitais.
// Implementações concretas residem na camada de infraestrutura (ex:
// adapter/repository/postgres).
type WalletRepository interface {
	// LockForUpdate busca as carteiras identificadas por ids aplicando
	// bloqueio pessimista (ex: SELECT ... FOR UPDATE) para impedir leituras
	// e escritas concorrentes sobre as mesmas linhas até o fim da
	// transação corrente. A implementação é responsável por ordenar a
	// aquisição dos locks (ex: por ID crescente) para prevenir deadlocks
	// quando múltiplas transações concorrentes bloqueiam as mesmas
	// carteiras em ordens diferentes.
	//
	// Retorna um mapa indexado por WalletID. Caso alguma carteira não
	// exista, domain.ErrWalletNotFound deve ser retornado.
	LockForUpdate(ctx context.Context, ids []string) (map[string]*domain.Wallet, error)

	// Save persiste o estado atualizado da carteira (tipicamente um
	// UPDATE de saldo) dentro da transação corrente.
	Save(ctx context.Context, wallet *domain.Wallet) error
}

// TransactionRepository é a porta de acesso a dados do histórico de
// transações de transferência.
type TransactionRepository interface {
	// Save persiste um registro de transação (completada ou falha) dentro
	// da transação corrente, garantindo o histórico auditável da operação.
	Save(ctx context.Context, tx *domain.Transaction) error
}
