package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/example/p2p-transfer/internal/domain"
)

type fakeIDs struct {
	id  string
	err error
}

func (f fakeIDs) New() (string, error) { return f.id, f.err }

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeWallets struct {
	values             map[string]*domain.Wallet
	lockErr, updateErr error
	updates            int
	failUpdate         int
}

func (f *fakeWallets) LockByIDs(context.Context, string, string) (map[string]*domain.Wallet, error) {
	return f.values, f.lockErr
}
func (f *fakeWallets) Update(context.Context, *domain.Wallet) error {
	f.updates++
	if f.failUpdate > 0 && f.updates != f.failUpdate {
		return nil
	}
	return f.updateErr
}

type fakeTransfers struct {
	err     error
	created *domain.Transfer
}

func (f *fakeTransfers) Create(_ context.Context, transfer *domain.Transfer) error {
	f.created = transfer
	return f.err
}

type fakeManager struct {
	wallets   *fakeWallets
	transfers *fakeTransfers
	err       error
}

func (f fakeManager) WithinTransaction(ctx context.Context, fn func(context.Context, WalletRepository, TransferRepository) error) error {
	if f.err != nil {
		return f.err
	}
	return fn(ctx, f.wallets, f.transfers)
}

func TestTransferExecuteSuccess(t *testing.T) {
	w := &fakeWallets{values: map[string]*domain.Wallet{"a": {ID: "a", Balance: domain.MoneyFromCents(200)}, "b": {ID: "b", Balance: domain.MoneyFromCents(50)}}}
	tr := &fakeTransfers{}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600))
	u := NewTransferUseCase(fakeManager{wallets: w, transfers: tr}, fakeIDs{id: "id"}, fakeClock{now}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	out, err := u.Execute(context.Background(), TransferInput{"a", "b", "1.25"})
	if err != nil || out.ID != "id" || out.Amount != "1.25" || out.CreatedAt.Location() != time.UTC {
		t.Fatalf("output=%+v err=%v", out, err)
	}
	if w.values["a"].Balance.Cents() != 75 || w.values["b"].Balance.Cents() != 175 || w.updates != 2 || tr.created == nil {
		t.Fatal("transaction effects incorrect")
	}
}

func TestTransferExecuteErrors(t *testing.T) {
	boom := errors.New("boom")
	baseWallets := func() *fakeWallets {
		return &fakeWallets{values: map[string]*domain.Wallet{"a": {ID: "a", Balance: domain.MoneyFromCents(100)}, "b": {ID: "b", Balance: domain.MoneyFromCents(0)}}}
	}
	tests := []struct {
		name   string
		input  TransferInput
		ids    fakeIDs
		setup  func(*fakeWallets, *fakeTransfers) fakeManager
		target error
	}{
		{"empty", TransferInput{"", "b", "1"}, fakeIDs{id: "id"}, nil, ErrInvalidWallet}, {"same", TransferInput{"a", "a", "1"}, fakeIDs{id: "id"}, nil, ErrSameWallet}, {"amount", TransferInput{"a", "b", "0"}, fakeIDs{id: "id"}, nil, domain.ErrInvalidAmount}, {"id", TransferInput{"a", "b", "1"}, fakeIDs{err: boom}, nil, boom},
		{"manager", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			return fakeManager{wallets: w, transfers: tr, err: boom}
		}, boom},
		{"lock", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			w.lockErr = boom
			return fakeManager{wallets: w, transfers: tr}
		}, boom},
		{"missing", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			delete(w.values, "b")
			return fakeManager{wallets: w, transfers: tr}
		}, ErrWalletNotFound},
		{"funds", TransferInput{"a", "b", "2"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager { return fakeManager{wallets: w, transfers: tr} }, domain.ErrInsufficientFunds},
		{"source update", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			w.updateErr = boom
			return fakeManager{wallets: w, transfers: tr}
		}, boom},
		{"destination update", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			w.updateErr, w.failUpdate = boom, 2
			return fakeManager{wallets: w, transfers: tr}
		}, boom},
		{"destination overflow", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			w.values["b"].Balance = domain.MoneyFromCents(int64(^uint64(0) >> 1))
			return fakeManager{wallets: w, transfers: tr}
		}, domain.ErrInvalidAmount},
		{"transfer create", TransferInput{"a", "b", "1"}, fakeIDs{id: "id"}, func(w *fakeWallets, tr *fakeTransfers) fakeManager {
			tr.err = boom
			return fakeManager{wallets: w, transfers: tr}
		}, boom},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w, tr := baseWallets(), &fakeTransfers{}
			manager := fakeManager{wallets: w, transfers: tr}
			if test.setup != nil {
				manager = test.setup(w, tr)
			}
			u := NewTransferUseCase(manager, test.ids, fakeClock{}, slog.Default())
			_, err := u.Execute(context.Background(), test.input)
			if !errors.Is(err, test.target) {
				t.Fatalf("got %v want %v", err, test.target)
			}
		})
	}
}
