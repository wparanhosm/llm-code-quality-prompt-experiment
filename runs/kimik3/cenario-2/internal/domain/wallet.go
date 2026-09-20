package domain

import (
	"time"

	"github.com/google/uuid"
)

// Wallet representa uma carteira digital. Entidade de domínio pura:
// não conhece SQL, HTTP ou qualquer detalhe de infraestrutura.
//
// Invariantes:
//   - ID nunca vazio
//   - Balance nunca negativo (garantido por Money)
type Wallet struct {
	id        string
	balance   Money
	createdAt time.Time
	updatedAt time.Time
}

// NewWallet cria uma carteira com saldo inicial.
// O ID é gerado automaticamente (UUID v4) quando id estiver vazio.
func NewWallet(id string, initialBalance Money, now time.Time) (Wallet, error) {
	if id == "" {
		id = uuid.NewString()
	}
	return Wallet{
		id:        id,
		balance:   initialBalance,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ReconstructWallet recria uma carteira a partir de dados persistidos,
// sem revalidar invariants já garantidos pela camada de origem.
func ReconstructWallet(id string, balanceCents int64, createdAt, updatedAt time.Time) Wallet {
	return Wallet{
		id:        id,
		balance:   MustMoney(balanceCents),
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

// ID retorna o identificador da carteira.
func (w Wallet) ID() string { return w.id }

// Balance retorna o saldo atual da carteira.
func (w Wallet) Balance() Money { return w.balance }

// CreatedAt retorna o instante de criação.
func (w Wallet) CreatedAt() time.Time { return w.createdAt }

// UpdatedAt retorna o instante da última atualização.
func (w Wallet) UpdatedAt() time.Time { return w.updatedAt }

// CanDebit verifica se a carteira possui saldo suficiente para o débito.
func (w Wallet) CanDebit(amount Money) bool {
	return w.balance.GreaterThanOrEqual(amount)
}

// Debit retorna uma NOVA Wallet com o valor debitado.
// Retorna ErrInsufficientFunds se o saldo for insuficiente.
func (w Wallet) Debit(amount Money, now time.Time) (Wallet, error) {
	newBalance, err := w.balance.Sub(amount)
	if err != nil {
		return Wallet{}, err
	}
	w.balance = newBalance
	w.updatedAt = now
	return w, nil
}

// Credit retorna uma NOVA Wallet com o valor creditado.
func (w Wallet) Credit(amount Money, now time.Time) Wallet {
	w.balance = w.balance.Add(amount)
	w.updatedAt = now
	return w
}
