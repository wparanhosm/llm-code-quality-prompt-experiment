package domain

import (
	"errors"
	"time"
)

var (
	ErrInsufficientBalance = errors.New("saldo insuficiente para realizar a transferência")
	ErrInvalidAmount       = errors.New("valor da transferência deve ser maior que zero")
	ErrSameUser            = errors.New("usuário de origem e destino não podem ser o mesmo")
	ErrWalletNotFound      = errors.New("carteira não encontrada")
)

type Wallet struct {
	ID        int64
	UserID    string
	Balance   float64
	UpdatedAt time.Time
}

func (w *Wallet) Debit(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if w.Balance < amount {
		return ErrInsufficientBalance
	}
	w.Balance -= amount
	w.UpdatedAt = time.Now()
	return nil
}

func (w *Wallet) Credit(amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	w.Balance += amount
	w.UpdatedAt = time.Now()
	return nil
}

type Transaction struct {
	ID           int64
	FromUserID   string
	ToUserID     string
	Amount       float64
	Status       string
	CreatedAt    time.Time
}
