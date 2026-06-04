package domain

import (
	"time"
)

type Wallet struct {
	ID        string
	Balance   int64
	UpdatedAt time.Time
}

func (w *Wallet) Debit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if w.Balance < amount {
		return ErrInsufficientFunds
	}
	w.Balance -= amount
	w.UpdatedAt = time.Now().UTC()
	return nil
}

func (w *Wallet) Credit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	w.Balance += amount
	w.UpdatedAt = time.Now().UTC()
	return nil
}

type TransactionStatus string

const (
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
	TransactionStatusFailed    TransactionStatus = "FAILED"
)

type Transaction struct {
	ID            string
	FromWalletID  string
	ToWalletID    string
	Amount        int64
	Status        TransactionStatus
	CreatedAt     time.Time
}

func NewTransaction(id, from, to string, amount int64) (*Transaction, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if from == to {
		return nil, ErrSameWallet
	}
	return &Transaction{
		ID:           id,
		FromWalletID: from,
		ToWalletID:   to,
		Amount:       amount,
		Status:       TransactionStatusCompleted,
		CreatedAt:    time.Now().UTC(),
	}, nil
}
