package domain

import (
	"errors"
	"time"
)

// TransactionStatus representa o estado final de uma transação de
// transferência entre carteiras.
type TransactionStatus string

const (
	// TransactionStatusCompleted indica que a transferência foi concluída
	// com sucesso: débito, crédito e histórico persistidos atomicamente.
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
	// TransactionStatusFailed indica que a transferência foi revertida por
	// completo (rollback) e nenhum saldo foi alterado.
	TransactionStatusFailed TransactionStatus = "FAILED"
)

// Transaction é a entidade que representa o histórico imutável de uma
// operação de transferência P2P entre duas carteiras.
type Transaction struct {
	ID            string
	FromWalletID  string
	ToWalletID    string
	Amount        Money
	Status        TransactionStatus
	FailureReason string
	CreatedAt     time.Time
}

// NewTransaction cria uma nova transação de transferência já validando os
// invariantes de negócio: valor positivo e carteiras distintas.
func NewTransaction(id, fromWalletID, toWalletID string, amount Money) (*Transaction, error) {
	if id == "" {
		return nil, errors.New("id da transação não pode ser vazio")
	}
	if fromWalletID == "" || toWalletID == "" {
		return nil, errors.New("carteiras de origem e destino são obrigatórias")
	}
	if fromWalletID == toWalletID {
		return nil, ErrSameWallet
	}
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	return &Transaction{
		ID:           id,
		FromWalletID: fromWalletID,
		ToWalletID:   toWalletID,
		Amount:       amount,
		Status:       TransactionStatusCompleted,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

// MarkFailed transiciona a transação para o estado de falha, registrando o
// motivo para fins de auditoria e rastreabilidade.
func (t *Transaction) MarkFailed(reason string) {
	t.Status = TransactionStatusFailed
	t.FailureReason = reason
}
