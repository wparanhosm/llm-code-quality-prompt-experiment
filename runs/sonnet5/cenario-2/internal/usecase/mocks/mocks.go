// Package mocks fornece implementações de teste (test doubles) manuais para
// as portas definidas em internal/usecase, permitindo testes unitários
// isolados do TransferUseCase sem qualquer dependência de infraestrutura
// real (banco de dados, geração de UUID, etc).
package mocks

import (
	"context"
	"sync"

	"p2p-transfer/internal/domain"
	"p2p-transfer/internal/usecase"
)

// StaticIDGenerator é um IDGenerator determinístico para testes: sempre
// retorna o mesmo ID configurado.
type StaticIDGenerator struct {
	ID string
}

// NewID implementa usecase.IDGenerator.
func (g *StaticIDGenerator) NewID() string {
	return g.ID
}

// FakeWalletRepository é uma implementação em memória de
// usecase.WalletRepository, útil para simular carteiras existentes e
// inspecionar o estado salvo após a execução do usecase.
type FakeWalletRepository struct {
	mu      sync.Mutex
	wallets map[string]*domain.Wallet
	saved   []*domain.Wallet

	// LockForUpdateErr, se definido, é retornado por LockForUpdate,
	// simulando falhas de infraestrutura (ex: timeout de lock, erro de
	// conexão).
	LockForUpdateErr error
	// SaveErr, se definido, é retornado por Save.
	SaveErr error
	// SaveErrForID, se definido, faz Save retornar SaveErr apenas quando o
	// ID da carteira salva corresponder, permitindo simular falha em uma
	// operação específica (ex: falha ao salvar apenas a carteira de
	// destino, após a origem já ter sido salva com sucesso).
	SaveErrForID string
}

// NewFakeWalletRepository cria o repositório fake pré-populado com as
// carteiras fornecidas.
func NewFakeWalletRepository(wallets ...*domain.Wallet) *FakeWalletRepository {
	m := make(map[string]*domain.Wallet, len(wallets))
	for _, w := range wallets {
		cp := *w
		m[w.ID] = &cp
	}
	return &FakeWalletRepository{wallets: m}
}

// LockForUpdate implementa usecase.WalletRepository.
func (r *FakeWalletRepository) LockForUpdate(_ context.Context, ids []string) (map[string]*domain.Wallet, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.LockForUpdateErr != nil {
		return nil, r.LockForUpdateErr
	}

	result := make(map[string]*domain.Wallet, len(ids))
	for _, id := range ids {
		w, ok := r.wallets[id]
		if !ok {
			return nil, domain.ErrWalletNotFound
		}
		cp := *w
		result[id] = &cp
	}
	return result, nil
}

// Save implementa usecase.WalletRepository.
func (r *FakeWalletRepository) Save(_ context.Context, wallet *domain.Wallet) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.SaveErr != nil && (r.SaveErrForID == "" || r.SaveErrForID == wallet.ID) {
		return r.SaveErr
	}

	cp := *wallet
	r.wallets[wallet.ID] = &cp
	r.saved = append(r.saved, &cp)
	return nil
}

// Get retorna o estado atual (pós-save) de uma carteira, para asserções em
// testes.
func (r *FakeWalletRepository) Get(id string) (*domain.Wallet, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	w, ok := r.wallets[id]
	return w, ok
}

// SavedCount retorna quantas vezes Save foi chamado, útil para verificar
// que exatamente as carteiras esperadas foram persistidas.
func (r *FakeWalletRepository) SavedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.saved)
}

// FakeTransactionRepository é uma implementação em memória de
// usecase.TransactionRepository.
type FakeTransactionRepository struct {
	mu    sync.Mutex
	saved []*domain.Transaction

	// SaveErr, se definido, é retornado por Save.
	SaveErr error
}

// NewFakeTransactionRepository cria um repositório fake vazio.
func NewFakeTransactionRepository() *FakeTransactionRepository {
	return &FakeTransactionRepository{}
}

// Save implementa usecase.TransactionRepository.
func (r *FakeTransactionRepository) Save(_ context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.SaveErr != nil {
		return r.SaveErr
	}

	cp := *tx
	r.saved = append(r.saved, &cp)
	return nil
}

// Saved retorna todas as transações persistidas, na ordem em que foram
// salvas.
func (r *FakeTransactionRepository) Saved() []*domain.Transaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Transaction, len(r.saved))
	copy(out, r.saved)
	return out
}

// fakeTransactionalRepositories agrupa os repositórios fake dentro do
// escopo "transacional" simulado pelo FakeUnitOfWork.
type fakeTransactionalRepositories struct {
	wallets      *FakeWalletRepository
	transactions *FakeTransactionRepository
}

func (f *fakeTransactionalRepositories) WalletRepository() usecase.WalletRepository {
	return f.wallets
}

func (f *fakeTransactionalRepositories) TransactionRepository() usecase.TransactionRepository {
	return f.transactions
}

// FakeUnitOfWork é uma implementação em memória de usecase.UnitOfWork que
// executa a função fornecida contra os repositórios fake configurados. Não
// implementa rollback real (pois os fakes são em memória e mutados
// diretamente), mas permite testar o fluxo de orquestração do usecase e
// simular falhas via ExecuteErr.
type FakeUnitOfWork struct {
	Wallets      *FakeWalletRepository
	Transactions *FakeTransactionRepository

	// ExecuteErr, se definido, é retornado imediatamente por Execute sem
	// invocar fn, simulando falha ao abrir a transação de banco de dados
	// (ex: conexão indisponível).
	ExecuteErr error

	// txMu serializa a execução de fn, simulando o isolamento fornecido em
	// produção por uma transação real de banco de dados com SELECT FOR
	// UPDATE: enquanto uma transação segura o lock pessimista sobre as
	// carteiras, nenhuma outra transação concorrente pode lê-las ou
	// escrevê-las. Sem essa serialização, o fake em memória permitiria
	// interleaving entre o Lock e o Save de transações concorrentes, o que
	// não reflete o comportamento real do Postgres.
	txMu sync.Mutex
}

// NewFakeUnitOfWork cria um FakeUnitOfWork com repositórios fake prontos
// para uso.
func NewFakeUnitOfWork(wallets *FakeWalletRepository, transactions *FakeTransactionRepository) *FakeUnitOfWork {
	return &FakeUnitOfWork{Wallets: wallets, Transactions: transactions}
}

// Execute implementa usecase.UnitOfWork.
func (u *FakeUnitOfWork) Execute(ctx context.Context, fn func(ctx context.Context, repos usecase.TransactionalRepositories) error) error {
	if u.ExecuteErr != nil {
		return u.ExecuteErr
	}
	u.txMu.Lock()
	defer u.txMu.Unlock()
	repos := &fakeTransactionalRepositories{wallets: u.Wallets, transactions: u.Transactions}
	return fn(ctx, repos)
}
