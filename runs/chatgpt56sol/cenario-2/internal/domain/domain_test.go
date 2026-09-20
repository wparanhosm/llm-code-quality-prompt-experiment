package domain

import (
	"errors"
	"math"
	"testing"
)

func TestMoney(t *testing.T) {
	tests := []struct {
		input, want string
		invalid     bool
	}{{"1", "1.00", false}, {" 12.3 ", "12.30", false}, {"0.01", "0.01", false}, {"", "", true}, {"0", "", true}, {"-1", "", true}, {"+1", "", true}, {".1", "", true}, {"1.", "", true}, {"1.234", "", true}, {"1.2.3", "", true}, {"abc", "", true}, {"1.xx", "", true}, {"999999999999999999999", "", true}}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			money, err := ParseMoney(test.input)
			if test.invalid {
				if !errors.Is(err, ErrInvalidAmount) {
					t.Fatalf("wanted invalid amount, got %v", err)
				}
				return
			}
			if err != nil || money.String() != test.want {
				t.Fatalf("got %v %q", err, money.String())
			}
		})
	}
	if MoneyFromCents(123).Cents() != 123 {
		t.Fatal("unexpected cents")
	}
}

func TestWallet(t *testing.T) {
	wallet := &Wallet{ID: "a", Balance: MoneyFromCents(100)}
	if err := wallet.Debit(MoneyFromCents(40)); err != nil || wallet.Balance.Cents() != 60 {
		t.Fatalf("debit: %v", err)
	}
	if err := wallet.Credit(MoneyFromCents(10)); err != nil || wallet.Balance.Cents() != 70 {
		t.Fatalf("credit: %v", err)
	}
	if !errors.Is(wallet.Debit(MoneyFromCents(80)), ErrInsufficientFunds) {
		t.Fatal("expected funds error")
	}
	if !errors.Is(wallet.Debit(MoneyFromCents(0)), ErrInvalidAmount) {
		t.Fatal("expected debit amount error")
	}
	if !errors.Is(wallet.Credit(MoneyFromCents(0)), ErrInvalidAmount) {
		t.Fatal("expected credit amount error")
	}
	wallet.Balance = MoneyFromCents(math.MaxInt64)
	if !errors.Is(wallet.Credit(MoneyFromCents(1)), ErrInvalidAmount) {
		t.Fatal("expected overflow")
	}
}
