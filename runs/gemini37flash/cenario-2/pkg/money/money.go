package money

import (
	"errors"
	"fmt"
)

var (
	ErrNegativeAmount = errors.New("amount cannot be negative")
	ErrZeroAmount     = errors.New("amount must be greater than zero")
)

// Money represents a monetary value in the lowest currency unit (cents).
// Storing monetary values as integers (cents) avoids floating-point precision issues.
type Money struct {
	cents int64
}

// New creates a new Money instance given the amount in cents.
func New(cents int64) (Money, error) {
	if cents <= 0 {
		return Money{}, ErrZeroAmount
	}
	return Money{cents: cents}, nil
}

// FromCentsUnsafe creates a Money instance without strict positive validation (e.g. for zero balance).
func FromCentsUnsafe(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, ErrNegativeAmount
	}
	return Money{cents: cents}, nil
}

// MustNew creates a new Money instance or panics.
func MustNew(cents int64) Money {
	m, err := New(cents)
	if err != nil {
		panic(err)
	}
	return m
}

// Cents returns the raw integer value in cents.
func (m Money) Cents() int64 {
	return m.cents
}

// Add adds another Money amount to the current one.
func (m Money) Add(other Money) Money {
	return Money{cents: m.cents + other.cents}
}

// Sub subtracts another Money amount from the current one.
func (m Money) Sub(other Money) (Money, error) {
	if m.cents < other.cents {
		return Money{}, fmt.Errorf("insufficient amount: current %d is less than required %d", m.cents, other.cents)
	}
	return Money{cents: m.cents - other.cents}, nil
}

// IsZero checks if the amount is 0.
func (m Money) IsZero() bool {
	return m.cents == 0
}

// String provides a formatted representation (e.g., $10.50).
func (m Money) String() string {
	return fmt.Sprintf("%.2f", float64(m.cents)/100.0)
}
