package usecase_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/internal/usecase"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

type failingAccRepo struct {
	*mockAccountRepo
	failGetByID      bool
	failGetForUpdate bool
	failUpdate       bool
}

func (f *failingAccRepo) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	if f.failGetByID {
		return nil, errors.New("db error")
	}
	return f.mockAccountRepo.GetByID(ctx, id)
}

func (f *failingAccRepo) GetForUpdate(ctx context.Context, id int64) (*domain.Account, error) {
	if f.failGetForUpdate {
		return nil, errors.New("lock error")
	}
	return f.mockAccountRepo.GetForUpdate(ctx, id)
}

func (f *failingAccRepo) UpdateBalance(ctx context.Context, account *domain.Account) error {
	if f.failUpdate {
		return errors.New("update error")
	}
	return f.mockAccountRepo.UpdateBalance(ctx, account)
}

type failingTxRepo struct {
	*mockTransferRepo
	failCreate bool
}

func (f *failingTxRepo) Create(ctx context.Context, transfer *domain.Transfer) error {
	if f.failCreate {
		return errors.New("create transfer error")
	}
	return f.mockTransferRepo.Create(ctx, transfer)
}

func TestTransferUseCase_ErrorBranches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. GetAccount repository error
	uc, accRepo, _, _ := setupTestUseCase()
	failingRepo := &failingAccRepo{mockAccountRepo: accRepo, failGetByID: true}
	uow := newMockUnitOfWork(failingRepo, newMockTransferRepo())
	ucFailing := setupCustomUseCase(uow, failingRepo)

	_, err := ucFailing.GetAccount(ctx, 1)
	if err == nil {
		t.Errorf("expected error when GetByID fails")
	}

	// 2. Lock error on first account
	failingLock1 := &failingAccRepo{mockAccountRepo: accRepo, failGetForUpdate: true}
	uowLock1 := newMockUnitOfWork(failingLock1, newMockTransferRepo())
	ucLock1 := setupCustomUseCase(uowLock1, failingLock1)
	_, err = ucLock1.Execute(ctx, defaultInput())
	if err == nil {
		t.Errorf("expected error on lock 1 failure")
	}

	// 3. Lock error on second account
	baseRepo := newMockAccountRepo()
	acc1, _ := domain.NewAccount(1, "Alice", money.MustNew(5000), domain.AccountStatusActive)
	_ = baseRepo.Create(ctx, acc1)
	selectiveRepo := &selectiveFailRepo{
		mockAccountRepo: baseRepo,
		failOnID:        2,
	}
	uowLock2 := newMockUnitOfWork(selectiveRepo, newMockTransferRepo())
	ucLock2 := setupCustomUseCase(uowLock2, selectiveRepo)
	_, err = ucLock2.Execute(ctx, defaultInput())
	if err == nil {
		t.Errorf("expected error on lock 2 failure")
	}

	// 4. Update sender balance failure
	acc2, _ := domain.NewAccount(2, "Bob", money.MustNew(5000), domain.AccountStatusActive)
	_ = baseRepo.Create(ctx, acc2)
	failUpdateRepo := &failingAccRepo{mockAccountRepo: baseRepo, failUpdate: true}
	uowUpdate := newMockUnitOfWork(failUpdateRepo, newMockTransferRepo())
	ucUpdate := setupCustomUseCase(uowUpdate, failUpdateRepo)
	_, err = ucUpdate.Execute(ctx, defaultInput())
	if err == nil {
		t.Errorf("expected error on update balance failure")
	}

	// 5. Update recipient balance failure
	selectiveUpdateRepo := &selectiveFailRepo{
		mockAccountRepo: baseRepo,
		failUpdateOnID:  2,
	}
	uowUpdate2 := newMockUnitOfWork(selectiveUpdateRepo, newMockTransferRepo())
	ucUpdate2 := setupCustomUseCase(uowUpdate2, selectiveUpdateRepo)
	_, err = ucUpdate2.Execute(ctx, defaultInput())
	if err == nil {
		t.Errorf("expected error on recipient update balance failure")
	}

	// 6. Transfer ledger insert failure
	failTxRepo := &failingTxRepo{mockTransferRepo: newMockTransferRepo(), failCreate: true}
	uowLedger := newMockUnitOfWork(baseRepo, failTxRepo)
	ucLedger := setupCustomUseCase(uowLedger, baseRepo)
	_, err = ucLedger.Execute(ctx, defaultInput())
	if err == nil {
		t.Errorf("expected error on ledger insert failure")
	}

	// 7. Debit failed on inactive sender
	inactiveAcc1, _ := domain.NewAccount(10, "Inactive", money.MustNew(5000), domain.AccountStatusInactive)
	activeAcc2, _ := domain.NewAccount(11, "Active", money.MustNew(5000), domain.AccountStatusActive)
	_ = baseRepo.Create(ctx, inactiveAcc1)
	_ = baseRepo.Create(ctx, activeAcc2)
	uowInactive := newMockUnitOfWork(baseRepo, newMockTransferRepo())
	ucInactive := setupCustomUseCase(uowInactive, baseRepo)
	_, err = ucInactive.Execute(ctx, usecase.TransferInput{
		FromAccountID: 10,
		ToAccountID:   11,
		AmountCents:   500,
	})
	if err == nil {
		t.Errorf("expected error on inactive sender")
	}

	// 8. Credit failed on inactive recipient
	_, err = ucInactive.Execute(ctx, usecase.TransferInput{
		FromAccountID: 11,
		ToAccountID:   10,
		AmountCents:   500,
	})
	if err == nil {
		t.Errorf("expected error on inactive recipient")
	}

	_ = uc
}

type selectiveFailRepo struct {
	*mockAccountRepo
	failOnID       int64
	failUpdateOnID int64
}

func (s *selectiveFailRepo) GetForUpdate(ctx context.Context, id int64) (*domain.Account, error) {
	if id == s.failOnID {
		return nil, errors.New("selective lock error")
	}
	return s.mockAccountRepo.GetForUpdate(ctx, id)
}

func (s *selectiveFailRepo) UpdateBalance(ctx context.Context, account *domain.Account) error {
	if account.ID == s.failUpdateOnID {
		return errors.New("selective update error")
	}
	return s.mockAccountRepo.UpdateBalance(ctx, account)
}

func defaultInput() usecase.TransferInput {
	return usecase.TransferInput{
		FromAccountID: 1,
		ToAccountID:   2,
		AmountCents:   100,
	}
}

func setupCustomUseCase(uow domain.UnitOfWork, accRepo domain.AccountRepository) usecase.TransferUseCase {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return usecase.NewTransferUseCase(uow, accRepo, logger)
}
