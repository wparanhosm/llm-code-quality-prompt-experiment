package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/wallet/p2p-transfer/internal/domain"
)

type stubUoW struct {
	fn func(ctx context.Context, fromID, toID string, amount domain.Money) (*domain.Transfer, error)
}

func (s stubUoW) ExecuteTransfer(ctx context.Context, fromID, toID string, amount domain.Money) (*domain.Transfer, error) {
	return s.fn(ctx, fromID, toID, amount)
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestTransferP2PValidation(t *testing.T) {
	t.Parallel()
	uc := NewTransferP2P(stubUoW{fn: func(context.Context, string, string, domain.Money) (*domain.Transfer, error) {
		t.Fatal("uow should not be called")
		return nil, nil
	}}, silentLogger())

	cases := []struct {
		name string
		in   TransferInput
		want error
	}{
		{"empty from", TransferInput{ToWalletID: "b", Amount: 1}, domain.ErrInvalidWalletID},
		{"empty to", TransferInput{FromWalletID: "a", Amount: 1}, domain.ErrInvalidWalletID},
		{"same wallet", TransferInput{FromWalletID: "a", ToWalletID: "a", Amount: 1}, domain.ErrSameWalletTransfer},
		{"zero amount", TransferInput{FromWalletID: "a", ToWalletID: "b", Amount: 0}, domain.ErrInvalidAmount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := uc.Execute(context.Background(), tc.in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestTransferP2PSuccessAndUoWError(t *testing.T) {
	t.Parallel()
	want := &domain.Transfer{ID: "t1", FromWalletID: "a", ToWalletID: "b", Amount: 10, CreatedAt: time.Now().UTC()}

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		uc := NewTransferP2P(stubUoW{fn: func(context.Context, string, string, domain.Money) (*domain.Transfer, error) {
			return want, nil
		}}, silentLogger())
		got, err := uc.Execute(context.Background(), TransferInput{FromWalletID: "a", ToWalletID: "b", Amount: 10})
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != want.ID {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("uow error", func(t *testing.T) {
		t.Parallel()
		uc := NewTransferP2P(stubUoW{fn: func(context.Context, string, string, domain.Money) (*domain.Transfer, error) {
			return nil, domain.ErrInsufficientFunds
		}}, silentLogger())
		_, err := uc.Execute(context.Background(), TransferInput{FromWalletID: "a", ToWalletID: "b", Amount: 10})
		if !errors.Is(err, domain.ErrInsufficientFunds) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestNewTransferP2PNilLogger(t *testing.T) {
	t.Parallel()
	uc := NewTransferP2P(stubUoW{fn: func(context.Context, string, string, domain.Money) (*domain.Transfer, error) {
		return &domain.Transfer{ID: "x"}, nil
	}}, nil)
	if uc.logger == nil {
		t.Fatal("logger must not be nil")
	}
}
