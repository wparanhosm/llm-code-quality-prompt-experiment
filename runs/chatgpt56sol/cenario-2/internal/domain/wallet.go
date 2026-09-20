package domain

import (
	"errors"
	"fmt"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

type Wallet struct {
	ID      string
	Balance Money
}

func (w *Wallet) Debit(amount Money) error {
	if amount.Cents() <= 0 {
		return fmt.Errorf("debit wallet %s: %w", w.ID, ErrInvalidAmount)
	}
	if w.Balance.Cents() < amount.Cents() {
		return fmt.Errorf("debit wallet %s: %w", w.ID, ErrInsufficientFunds)
	}
	w.Balance = MoneyFromCents(w.Balance.Cents() - amount.Cents())
	return nil
}

func (w *Wallet) Credit(amount Money) error {
	if amount.Cents() <= 0 {
		return fmt.Errorf("credit wallet %s: %w", w.ID, ErrInvalidAmount)
	}
	if w.Balance.Cents() > int64(^uint64(0)>>1)-amount.Cents() {
		return fmt.Errorf("credit wallet %s: balance overflow: %w", w.ID, ErrInvalidAmount)
	}
	w.Balance = MoneyFromCents(w.Balance.Cents() + amount.Cents())
	return nil
}
