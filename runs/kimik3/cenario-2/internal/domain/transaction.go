package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransactionStatus representa o estado final de uma transação P2P.
type TransactionStatus string

const (
	// TransactionStatusCompleted indica transferência concluída com sucesso.
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
)

// Transaction é o registro imutável de uma transferência P2P entre carteiras.
// Funciona como audit log financeiro da operação.
type Transaction struct {
	id            string
	fromWalletID  string
	toWalletID    string
	amount        Money
	status        TransactionStatus
	idempotencyKey string
	createdAt     time.Time
}

// NewTransaction cria o registro de uma transferência concluída.
// Valida as invariantes do domínio antes de construir a entidade.
func NewTransaction(fromWalletID, toWalletID string, amount Money, idempotencyKey string, now time.Time) (Transaction, error) {
	if fromWalletID == "" || toWalletID == "" {
		return Transaction{}, ErrInvalidWalletID
	}
	if fromWalletID == toWalletID {
		return Transaction{}, ErrSameWallet
	}
	if amount.IsZero() {
		return Transaction{}, ErrInvalidAmount
	}
	return Transaction{
		id:             uuid.NewString(),
		fromWalletID:   fromWalletID,
		toWalletID:     toWalletID,
		amount:         amount,
		status:         TransactionStatusCompleted,
		idempotencyKey: idempotencyKey,
		createdAt:      now,
	}, nil
}

// ReconstructTransaction recria uma transação a partir de dados persistidos.
func ReconstructTransaction(id, fromWalletID, toWalletID string, amountCents int64, status TransactionStatus, idempotencyKey string, createdAt time.Time) Transaction {
	return Transaction{
		id:             id,
		fromWalletID:   fromWalletID,
		toWalletID:     toWalletID,
		amount:         MustMoney(amountCents),
		status:         status,
		idempotencyKey: idempotencyKey,
		createdAt:      createdAt,
	}
}

// ID retorna o identificador da transação.
func (t Transaction) ID() string { return t.id }

// FromWalletID retorna a carteira de origem.
func (t Transaction) FromWalletID() string { return t.fromWalletID }

// ToWalletID retorna a carteira de destino.
func (t Transaction) ToWalletID() string { return t.toWalletID }

// Amount retorna o valor transferido.
func (t Transaction) Amount() Money { return t.amount }

// Status retorna o estado da transação.
func (t Transaction) Status() TransactionStatus { return t.status }

// IdempotencyKey retorna a chave de idempotência associada (pode ser vazia).
func (t Transaction) IdempotencyKey() string { return t.idempotencyKey }

// CreatedAt retorna o instante de criação.
func (t Transaction) CreatedAt() time.Time { return t.createdAt }
