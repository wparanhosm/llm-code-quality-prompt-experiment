package money_test

import (
	"testing"

	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

func TestNewMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cents     int64
		expectErr bool
	}{
		{"Valid positive amount", 1000, false},
		{"Zero amount", 0, true},
		{"Negative amount", -500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := money.New(tt.cents)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if m.Cents() != tt.cents {
					t.Errorf("expected %d, got %d", tt.cents, m.Cents())
				}
			}
		})
	}
}

func TestFromCentsUnsafe(t *testing.T) {
	t.Parallel()

	m, err := money.FromCentsUnsafe(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.IsZero() {
		t.Errorf("expected zero, got %d", m.Cents())
	}

	_, err = money.FromCentsUnsafe(-1)
	if err == nil {
		t.Errorf("expected error for negative amount, got nil")
	}
}

func TestMustNew(t *testing.T) {
	t.Parallel()

	m := money.MustNew(500)
	if m.Cents() != 500 {
		t.Errorf("expected 500, got %d", m.Cents())
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for invalid amount")
		}
	}()
	_ = money.MustNew(-10)
}

func TestMoneyOperations(t *testing.T) {
	t.Parallel()

	m1 := money.MustNew(1050)
	m2 := money.MustNew(450)

	// Add
	sum := m1.Add(m2)
	if sum.Cents() != 1500 {
		t.Errorf("expected 1500, got %d", sum.Cents())
	}

	// Sub success
	sub, err := m1.Sub(m2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Cents() != 600 {
		t.Errorf("expected 600, got %d", sub.Cents())
	}

	// Sub failure (negative balance prevention)
	_, err = m2.Sub(m1)
	if err == nil {
		t.Errorf("expected error when subtracting larger amount, got nil")
	}

	// String format
	if m1.String() != "10.50" {
		t.Errorf("expected '10.50', got %q", m1.String())
	}
}
