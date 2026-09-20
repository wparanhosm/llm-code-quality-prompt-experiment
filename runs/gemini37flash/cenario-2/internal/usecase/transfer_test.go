package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/internal/usecase"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

// In-memory thread-safe mock repository and UnitOfWork implementation for testing
type mockAccountRepo struct {
	mu       sync.Mutex
	accounts map[int64]*domain.Account
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{
		accounts: make(map[int64]*domain.Account),
	}
}

func (m *mockAccountRepo) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	acc, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	cp := *acc
	return &cp, nil
}

func (m *mockAccountRepo) GetForUpdate(ctx context.Context, id int64) (*domain.Account, error) {
	return m.GetByID(ctx, id)
}

func (m *mockAccountRepo) UpdateBalance(ctx context.Context, account *domain.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	acc, ok := m.accounts[account.ID]
	if !ok {
		return domain.ErrAccountNotFound
	}
	acc.Balance = account.Balance
	acc.UpdatedAt = account.UpdatedAt
	return nil
}

func (m *mockAccountRepo) Create(ctx context.Context, account *domain.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[account.ID] = account
	return nil
}

type mockTransferRepo struct {
	mu        sync.Mutex
	transfers map[int64]*domain.Transfer
	nextID    int64
}

func newMockTransferRepo() *mockTransferRepo {
	return &mockTransferRepo{
		transfers: make(map[int64]*domain.Transfer),
		nextID:    1,
	}
}

func (m *mockTransferRepo) Create(ctx context.Context, transfer *domain.Transfer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	transfer.ID = m.nextID
	m.nextID++
	m.transfers[transfer.ID] = transfer
	return nil
}

func (m *mockTransferRepo) GetByID(ctx context.Context, id int64) (*domain.Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transfers[id]
	if !ok {
		return nil, domain.ErrTransferNotFound
	}
	return t, nil
}

type mockUoWRepo struct {
	accRepo domain.AccountRepository
	txRepo  domain.TransferRepository
}

func (m *mockUoWRepo) Accounts() domain.AccountRepository {
	return m.accRepo
}

func (m *mockUoWRepo) Transfers() domain.TransferRepository {
	return m.txRepo
}

type mockUnitOfWork struct {
	globalMu sync.Mutex
	uowRepo  domain.UnitOfWorkRepository
	failTx   bool
}

func newMockUnitOfWork(accRepo domain.AccountRepository, txRepo domain.TransferRepository) *mockUnitOfWork {
	return &mockUnitOfWork{
		uowRepo: &mockUoWRepo{
			accRepo: accRepo,
			txRepo:  txRepo,
		},
	}
}

func (m *mockUnitOfWork) Execute(ctx context.Context, fn func(txRepo domain.UnitOfWorkRepository) error) error {
	m.globalMu.Lock()
	defer m.globalMu.Unlock()

	if m.failTx {
		return errors.New("simulated tx failure")
	}

	return fn(m.uowRepo)
}

func setupTestUseCase() (usecase.TransferUseCase, *mockAccountRepo, *mockTransferRepo, *mockUnitOfWork) {
	accRepo := newMockAccountRepo()
	txRepo := newMockTransferRepo()
	uow := newMockUnitOfWork(accRepo, txRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	uc := usecase.NewTransferUseCase(uow, accRepo, logger)
	return uc, accRepo, txRepo, uow
}

func TestTransferUseCase_Success(t *testing.T) {
	t.Parallel()
	uc, accRepo, _, _ := setupTestUseCase()
	ctx := context.Background()

	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(10000), domain.AccountStatusActive)
	acc2, _ := domain.NewAccount(2, "Bob", money.MustNew(5000), domain.AccountStatusActive)
	_ = accRepo.Create(ctx, acc1)
	_ = accRepo.Create(ctx, acc2)

	out, err := uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		AmountCents:   3000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.FromAccountBalanceLeft != 7000 {
		t.Errorf("expected sender balance 7000, got %d", out.FromAccountBalanceLeft)
	}
	if out.ToAccountBalanceLeft != 8000 {
		t.Errorf("expected recipient balance 8000, got %d", out.ToAccountBalanceLeft)
	}
}

func TestTransferUseCase_InverseLockOrdering(t *testing.T) {
	t.Parallel()
	uc, accRepo, _, _ := setupTestUseCase()
	ctx := context.Background()

	// Account 2 transfers to Account 1 (tests ordering where From > To)
	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(10000), domain.AccountStatusActive)
	acc2, _ := domain.NewAccount(2, "Bob", money.MustNew(5000), domain.AccountStatusActive)
	_ = accRepo.Create(ctx, acc1)
	_ = accRepo.Create(ctx, acc2)

	out, err := uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 2,
		ToAccountID:   1,
		AmountCents:   2000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.FromAccountBalanceLeft != 3000 {
		t.Errorf("expected sender balance 3000, got %d", out.FromAccountBalanceLeft)
	}
	if out.ToAccountBalanceLeft != 12000 {
		t.Errorf("expected recipient balance 12000, got %d", out.ToAccountBalanceLeft)
	}
}

func TestTransferUseCase_ValidationErrors(t *testing.T) {
	t.Parallel()
	uc, _, _, _ := setupTestUseCase()
	ctx := context.Background()

	// Zero / negative amount
	_, err := uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		AmountCents:   0,
	})
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount, got %v", err)
	}

	// Same account
	_, err = uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 1,
		ToAccountID:   1,
		AmountCents:   100,
	})
	if !errors.Is(err, domain.ErrSameAccountTransfer) {
		t.Errorf("expected ErrSameAccountTransfer, got %v", err)
	}
}

func TestTransferUseCase_AccountNotFound(t *testing.T) {
	t.Parallel()
	uc, accRepo, _, _ := setupTestUseCase()
	ctx := context.Background()

	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(10000), domain.AccountStatusActive)
	_ = accRepo.Create(ctx, acc1)

	// Recipient account 2 does not exist
	_, err := uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		AmountCents:   1000,
	})
	if !errors.Is(err, domain.ErrTransactionFailed) || !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound inside transaction failed, got %v", err)
	}

	// Sender account 3 does not exist
	_, err = uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 3,
		ToAccountID:   1,
		AmountCents:   1000,
	})
	if !errors.Is(err, domain.ErrTransactionFailed) || !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestTransferUseCase_InsufficientBalance(t *testing.T) {
	t.Parallel()
	uc, accRepo, _, _ := setupTestUseCase()
	ctx := context.Background()

	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(500), domain.AccountStatusActive)
	acc2, _ := domain.NewAccount(2, "Bob", money.MustNew(500), domain.AccountStatusActive)
	_ = accRepo.Create(ctx, acc1)
	_ = accRepo.Create(ctx, acc2)

	_, err := uc.Execute(ctx, usecase.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		AmountCents:   1000,
	})
	if !errors.Is(err, domain.ErrTransactionFailed) || !errors.Is(err, domain.ErrInsufficientBalance) {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestTransferUseCase_GetAccount(t *testing.T) {
	t.Parallel()
	uc, accRepo, _, _ := setupTestUseCase()
	ctx := context.Background()

	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(500), domain.AccountStatusActive)
	_ = accRepo.Create(ctx, acc1)

	acc, err := uc.GetAccount(ctx, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acc.ID != 1 || acc.Holder != "Alice" {
		t.Errorf("account mismatch: %+v", acc)
	}

	// Invalid ID
	_, err = uc.GetAccount(ctx, 0)
	if !errors.Is(err, domain.ErrInvalidAccountID) {
		t.Errorf("expected ErrInvalidAccountID, got %v", err)
	}

	// Not found
	_, err = uc.GetAccount(ctx, 999)
	if !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestTransferUseCase_ConcurrentTransfers(t *testing.T) {
	t.Parallel()
	uc, accRepo, _, _ := setupTestUseCase()
	ctx := context.Background()

	// Alice starts with 100,000 cents ($1000.00)
	// Bob starts with 100,000 cents ($1000.00)
	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(100000), domain.AccountStatusActive)
	acc2, _ := domain.NewAccount(2, "Bob", money.MustNew(100000), domain.AccountStatusActive)
	_ = accRepo.Create(ctx, acc1)
	_ = accRepo.Create(ctx, acc2)

	var wg sync.WaitGroup
	iterations := 100
	amount := int64(10) // 10 cents each

	// 100 transfers from 1 -> 2
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			_, err := uc.Execute(ctx, usecase.TransferInput{
				FromAccountID: 1,
				ToAccountID:   2,
				AmountCents:   amount,
			})
			if err != nil {
				t.Errorf("concurrent transfer 1->2 failed: %v", err)
			}
		}()
	}

	// 100 transfers from 2 -> 1
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			_, err := uc.Execute(ctx, usecase.TransferInput{
				FromAccountID: 2,
				ToAccountID:   1,
				AmountCents:   amount,
			})
			if err != nil {
				t.Errorf("concurrent transfer 2->1 failed: %v", err)
			}
		}()
	}

	wg.Wait()

	finalAcc1, _ := uc.GetAccount(ctx, 1)
	finalAcc2, _ := uc.GetAccount(ctx, 2)

	// Since net exchange is equal, balances should remain identical
	if finalAcc1.Balance.Cents() != 100000 {
		t.Errorf("expected final Alice balance 100000, got %d", finalAcc1.Balance.Cents())
	}
	if finalAcc2.Balance.Cents() != 100000 {
		t.Errorf("expected final Bob balance 100000, got %d", finalAcc2.Balance.Cents())
	}
}
