package domain_test

import (
	"errors"
	"testing"

	"p2p-transfer/internal/domain"
)

func TestNewMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cents   int64
		wantErr error
	}{
		{name: "valor positivo válido", cents: 1000, wantErr: nil},
		{name: "valor zero é inválido", cents: 0, wantErr: domain.ErrInvalidAmount},
		{name: "valor negativo é inválido", cents: -50, wantErr: domain.ErrInvalidAmount},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.NewMoney(tt.cents)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewMoney(%d) err = %v, want %v", tt.cents, err, tt.wantErr)
			}
			if tt.wantErr == nil && got.Int64() != tt.cents {
				t.Fatalf("NewMoney(%d) = %d, want %d", tt.cents, got.Int64(), tt.cents)
			}
		})
	}
}

func TestMoney_String(t *testing.T) {
	t.Parallel()
	m := domain.Money(150055)
	want := "R$ 1500.55"
	if got := m.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestNewWallet(t *testing.T) {
	t.Parallel()

	t.Run("carteira válida", func(t *testing.T) {
		t.Parallel()
		w, err := domain.NewWallet("wallet-1", "owner-1", 1000, domain.WalletStatusActive)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if w.ID != "wallet-1" || w.OwnerID != "owner-1" || w.Balance != 1000 {
			t.Fatalf("wallet criada incorretamente: %+v", w)
		}
	})

	t.Run("id vazio retorna erro", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewWallet("", "owner-1", 1000, domain.WalletStatusActive)
		if err == nil {
			t.Fatal("esperava erro para id vazio")
		}
	})

	t.Run("owner id vazio retorna erro", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewWallet("wallet-1", "", 1000, domain.WalletStatusActive)
		if err == nil {
			t.Fatal("esperava erro para owner id vazio")
		}
	})

	t.Run("status inválido retorna erro", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewWallet("wallet-1", "owner-1", 1000, "BOGUS")
		if err == nil {
			t.Fatal("esperava erro para status inválido")
		}
	})
}

func TestWallet_EnsureActive(t *testing.T) {
	t.Parallel()

	active, _ := domain.NewWallet("w1", "o1", 100, domain.WalletStatusActive)
	if err := active.EnsureActive(); err != nil {
		t.Fatalf("carteira ativa não deveria retornar erro: %v", err)
	}

	inactive, _ := domain.NewWallet("w2", "o2", 100, domain.WalletStatusInactive)
	if err := inactive.EnsureActive(); !errors.Is(err, domain.ErrWalletInactive) {
		t.Fatalf("esperava ErrWalletInactive, obteve %v", err)
	}
}

func TestWallet_Debit(t *testing.T) {
	t.Parallel()

	t.Run("débito com saldo suficiente", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 1000, domain.WalletStatusActive)
		if err := w.Debit(400); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if w.Balance != 600 {
			t.Fatalf("saldo = %d, want 600", w.Balance)
		}
	})

	t.Run("débito com saldo insuficiente", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 100, domain.WalletStatusActive)
		err := w.Debit(200)
		if !errors.Is(err, domain.ErrInsufficientFunds) {
			t.Fatalf("esperava ErrInsufficientFunds, obteve %v", err)
		}
		if w.Balance != 100 {
			t.Fatalf("saldo não deveria ter mudado, got %d", w.Balance)
		}
	})

	t.Run("débito com valor inválido", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 100, domain.WalletStatusActive)
		if err := w.Debit(0); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obteve %v", err)
		}
		if err := w.Debit(-10); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obteve %v", err)
		}
	})

	t.Run("débito em carteira inativa", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 1000, domain.WalletStatusInactive)
		if err := w.Debit(100); !errors.Is(err, domain.ErrWalletInactive) {
			t.Fatalf("esperava ErrWalletInactive, obteve %v", err)
		}
	})
}

func TestWallet_Credit(t *testing.T) {
	t.Parallel()

	t.Run("crédito válido", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 1000, domain.WalletStatusActive)
		if err := w.Credit(500); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if w.Balance != 1500 {
			t.Fatalf("saldo = %d, want 1500", w.Balance)
		}
	})

	t.Run("crédito com valor inválido", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 1000, domain.WalletStatusActive)
		if err := w.Credit(0); !errors.Is(err, domain.ErrInvalidAmount) {
			t.Fatalf("esperava ErrInvalidAmount, obteve %v", err)
		}
	})

	t.Run("crédito em carteira inativa", func(t *testing.T) {
		t.Parallel()
		w, _ := domain.NewWallet("w1", "o1", 1000, domain.WalletStatusInactive)
		if err := w.Credit(100); !errors.Is(err, domain.ErrWalletInactive) {
			t.Fatalf("esperava ErrWalletInactive, obteve %v", err)
		}
	})
}
