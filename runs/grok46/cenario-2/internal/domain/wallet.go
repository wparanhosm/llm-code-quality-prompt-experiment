package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidWalletID      = errors.New("invalid wallet id")
	ErrNegativeBalance      = errors.New("wallet balance cannot be negative")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrSameWalletTransfer   = errors.New("origin and destination wallets must differ")
	ErrInvalidAmount        = errors.New("transfer amount must be greater than zero")
	ErrWalletNotFound       = errors.New("wallet not found")
	ErrTransferNotPersisted = errors.New("transfer was not persisted")
)

// Money represents an amount in cents to avoid floating-point rounding.
type Money int64

func (m Money) ValidatePositive() error {
	if m <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

// Wallet is a digital wallet aggregate root.
type Wallet struct {
	ID      string
	Balance Money
}

func NewWallet(id string, balance Money) (*Wallet, error) {
	if id == "" {
		return nil, ErrInvalidWalletID
	}
	if balance < 0 {
		return nil, ErrNegativeBalance
	}
	return &Wallet{ID: id, Balance: balance}, nil
}

func (w *Wallet) Debit(amount Money) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if w.Balance < amount {
		return fmt.Errorf("wallet %s: %w", w.ID, ErrInsufficientFunds)
	}
	w.Balance -= amount
	return nil
}

func (w *Wallet) Credit(amount Money) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	w.Balance += amount
	return nil
}
