package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wallet-p2p/internal/domain"
	"wallet-p2p/internal/usecase"
)

// --- Fakes/Mocks -------------------------------------------------------------

type mockTx struct{ usecase.Tx }

type mockUoW struct {
	fnResult error
	innerFn  func(ctx context.Context, tx usecase.Tx) error
}

func (m *mockUoW) Do(ctx context.Context, fn func(ctx context.Context, tx usecase.Tx) error) error {
	if m.fnResult != nil {
		return m.fnResult
	}
	return fn(ctx, &mockTx{})
}

type mockRepo struct {
	mock.Mock
}

func (m *mockRepo) GetByIDForUpdate(ctx context.Context, tx usecase.Tx, id string) (domain.Wallet, error) {
	args := m.Called(ctx, tx, id)
	w, _ := args.Get(0).(domain.Wallet)
	return w, args.Error(1)
}

func (m *mockRepo) UpdateBalance(ctx context.Context, tx usecase.Tx, w domain.Wallet) error {
	return m.Called(ctx, tx, w).Error(0)
}

func (m *mockRepo) InsertTransaction(ctx context.Context, tx usecase.Tx, t domain.Transaction) error {
	return m.Called(ctx, tx, t).Error(0)
}

type fixedClock struct{ now time.Time }

func (f fixedClock) Now() time.Time { return f.now }

// --- Helpers -----------------------------------------------------------------

var testNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func walletWithBalance(id string, cents int64) domain.Wallet {
	return domain.ReconstructWallet(id, cents, testNow, testNow)
}

func validInput() usecase.TransferInput {
	return usecase.TransferInput{
		FromWalletID: "11111111-1111-1111-1111-111111111111",
		ToWalletID:   "22222222-2222-2222-2222-222222222222",
		AmountCents:  250,
	}
}

// --- Testes ------------------------------------------------------------------

func TestTransferUseCase_Execute_Success(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uow := &mockUoW{}
	uc := usecase.NewTransferUseCase(uow, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	from := walletWithBalance(input.FromWalletID, 1000)
	to := walletWithBalance(input.ToWalletID, 500)

	// Locks adquiridos em ordem determinística (menor ID primeiro).
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).Return(from, nil).Once()
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.ToWalletID).Return(to, nil).Once()
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.MatchedBy(func(w domain.Wallet) bool {
		return w.ID() == input.FromWalletID && w.Balance().Cents() == 750
	})).Return(nil).Once()
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.MatchedBy(func(w domain.Wallet) bool {
		return w.ID() == input.ToWalletID && w.Balance().Cents() == 750
	})).Return(nil).Once()
	repo.On("InsertTransaction", mock.Anything, mock.Anything, mock.MatchedBy(func(tx domain.Transaction) bool {
		return tx.FromWalletID() == input.FromWalletID &&
			tx.ToWalletID() == input.ToWalletID &&
			tx.Amount().Cents() == 250 &&
			tx.Status() == domain.TransactionStatusCompleted
	})).Return(nil).Once()

	out, err := uc.Execute(context.Background(), input)
	require.NoError(t, err)
	assert.NotEmpty(t, out.TransactionID)
	assert.Equal(t, input.FromWalletID, out.FromWalletID)
	assert.Equal(t, input.ToWalletID, out.ToWalletID)
	assert.Equal(t, int64(250), out.AmountCents)
	assert.Equal(t, int64(750), out.FromNewBalanceCents)
	assert.Equal(t, int64(750), out.ToNewBalanceCents)
	repo.AssertExpectations(t)
}

func TestTransferUseCase_Execute_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(in *usecase.TransferInput)
		wantErr error
	}{
		{
			name:    "origem vazia",
			mutate:  func(in *usecase.TransferInput) { in.FromWalletID = "" },
			wantErr: domain.ErrInvalidWalletID,
		},
		{
			name:    "destino vazio",
			mutate:  func(in *usecase.TransferInput) { in.ToWalletID = "" },
			wantErr: domain.ErrInvalidWalletID,
		},
		{
			name:    "origem igual destino",
			mutate:  func(in *usecase.TransferInput) { in.ToWalletID = in.FromWalletID },
			wantErr: domain.ErrSameWallet,
		},
		{
			name:    "valor zero",
			mutate:  func(in *usecase.TransferInput) { in.AmountCents = 0 },
			wantErr: domain.ErrInvalidAmount,
		},
		{
			name:    "valor negativo",
			mutate:  func(in *usecase.TransferInput) { in.AmountCents = -10 },
			wantErr: domain.ErrInvalidAmount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := new(mockRepo)
			uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())

			input := validInput()
			tt.mutate(&input)

			_, err := uc.Execute(context.Background(), input)
			require.ErrorIs(t, err, tt.wantErr)
			repo.AssertNotCalled(t, "GetByIDForUpdate", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestTransferUseCase_Execute_LockOrdering(t *testing.T) {
	t.Parallel()

	// from > to lexicograficamente: o lock deve ocorrer primeiro em `to`.
	repo := new(mockRepo)
	uow := &mockUoW{}
	uc := usecase.NewTransferUseCase(uow, repo, fixedClock{now: testNow}, newTestLogger())

	input := usecase.TransferInput{
		FromWalletID: "ffffffff-ffff-ffff-ffff-ffffffffffff",
		ToWalletID:   "00000000-0000-0000-0000-000000000000",
		AmountCents:  100,
	}
	wallets := map[string]domain.Wallet{
		input.FromWalletID: walletWithBalance(input.FromWalletID, 1000),
		input.ToWalletID:   walletWithBalance(input.ToWalletID, 0),
	}

	var lockOrder []string
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.ToWalletID).
		Run(func(args mock.Arguments) { lockOrder = append(lockOrder, args.String(2)) }).
		Return(wallets[input.ToWalletID], nil).Once()
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).
		Run(func(args mock.Arguments) { lockOrder = append(lockOrder, args.String(2)) }).
		Return(wallets[input.FromWalletID], nil).Once()
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repo.On("InsertTransaction", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Execute(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, []string{input.ToWalletID, input.FromWalletID}, lockOrder,
		"locks devem ser adquiridos em ordem determinística (menor ID primeiro)")
}

func TestTransferUseCase_Execute_WalletNotFound(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).
		Return(domain.Wallet{}, domain.ErrWalletNotFound).Once()

	_, err := uc.Execute(context.Background(), input)
	require.ErrorIs(t, err, domain.ErrWalletNotFound)
	repo.AssertExpectations(t)
}

func TestTransferUseCase_Execute_LockQueryFailure(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	dbErr := errors.New("conexão perdida")
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, mock.Anything).Return(domain.Wallet{}, dbErr).Once()

	_, err := uc.Execute(context.Background(), input)
	require.ErrorIs(t, err, dbErr)
}

func TestTransferUseCase_Execute_InsufficientFunds(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).
		Return(walletWithBalance(input.FromWalletID, 100), nil).Once()
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.ToWalletID).
		Return(walletWithBalance(input.ToWalletID, 500), nil).Once()

	_, err := uc.Execute(context.Background(), input)
	require.ErrorIs(t, err, domain.ErrInsufficientFunds)
	repo.AssertNotCalled(t, "UpdateBalance", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "InsertTransaction", mock.Anything, mock.Anything, mock.Anything)
}

func TestTransferUseCase_Execute_RollbackOnUpdateBalanceFailure(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).
		Return(walletWithBalance(input.FromWalletID, 1000), nil).Once()
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.ToWalletID).
		Return(walletWithBalance(input.ToWalletID, 500), nil).Once()
	dbErr := errors.New("falha de escrita")
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.MatchedBy(func(w domain.Wallet) bool {
		return w.ID() == input.FromWalletID
	})).Return(dbErr).Once()

	_, err := uc.Execute(context.Background(), input)
	require.ErrorIs(t, err, dbErr)
	// Crédito da destino nunca deve ser persistido após falha no débito.
	repo.AssertNotCalled(t, "InsertTransaction", mock.Anything, mock.Anything, mock.Anything)
}

func TestTransferUseCase_Execute_RollbackOnCreditUpdateFailure(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).
		Return(walletWithBalance(input.FromWalletID, 1000), nil).Once()
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.ToWalletID).
		Return(walletWithBalance(input.ToWalletID, 500), nil).Once()
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.MatchedBy(func(w domain.Wallet) bool {
		return w.ID() == input.FromWalletID
	})).Return(nil).Once()
	dbErr := errors.New("falha de escrita no crédito")
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.MatchedBy(func(w domain.Wallet) bool {
		return w.ID() == input.ToWalletID
	})).Return(dbErr).Once()

	_, err := uc.Execute(context.Background(), input)
	require.ErrorIs(t, err, dbErr)
	repo.AssertNotCalled(t, "InsertTransaction", mock.Anything, mock.Anything, mock.Anything)
}

func TestTransferUseCase_Execute_RollbackOnInsertTransactionFailure(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uc := usecase.NewTransferUseCase(&mockUoW{}, repo, fixedClock{now: testNow}, newTestLogger())
	input := validInput()

	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.FromWalletID).
		Return(walletWithBalance(input.FromWalletID, 1000), nil).Once()
	repo.On("GetByIDForUpdate", mock.Anything, mock.Anything, input.ToWalletID).
		Return(walletWithBalance(input.ToWalletID, 500), nil).Once()
	repo.On("UpdateBalance", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	dbErr := errors.New("violação de chave de idempotência")
	repo.On("InsertTransaction", mock.Anything, mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := uc.Execute(context.Background(), input)
	require.ErrorIs(t, err, dbErr)
}

func TestTransferUseCase_Execute_UoWFailure(t *testing.T) {
	t.Parallel()

	repo := new(mockRepo)
	uowErr := errors.New("falha ao iniciar transação")
	uow := &mockUoW{fnResult: uowErr}
	uc := usecase.NewTransferUseCase(uow, repo, fixedClock{now: testNow}, newTestLogger())

	_, err := uc.Execute(context.Background(), validInput())
	require.ErrorIs(t, err, uowErr)
}

func TestSystemClock_Now(t *testing.T) {
	t.Parallel()

	clock := usecase.NewSystemClock()
	before := time.Now().UTC()
	got := clock.Now()
	after := time.Now().UTC()

	assert.False(t, got.Before(before))
	assert.False(t, got.After(after))
	assert.Equal(t, time.UTC, got.Location())
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	assert.True(t, usecase.IsNotFound(domain.ErrWalletNotFound))
	assert.False(t, usecase.IsNotFound(domain.ErrInsufficientFunds))
	assert.False(t, usecase.IsNotFound(nil))
}
