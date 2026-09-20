package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

// TransferInput holds the payload parameters for a P2P transfer.
type TransferInput struct {
	FromAccountID int64 `json:"from_account_id"`
	ToAccountID   int64 `json:"to_account_id"`
	AmountCents   int64 `json:"amount_cents"`
}

// TransferOutput holds the result of a successful transfer execution.
type TransferOutput struct {
	TransferID             int64  `json:"transfer_id"`
	FromAccountID          int64  `json:"from_account_id"`
	ToAccountID            int64  `json:"to_account_id"`
	AmountCents            int64  `json:"amount_cents"`
	FromAccountBalanceLeft int64  `json:"from_account_balance_left"`
	ToAccountBalanceLeft   int64  `json:"to_account_balance_left"`
	Status                 string `json:"status"`
}

// TransferUseCase defines the boundary for transfer operations.
type TransferUseCase interface {
	Execute(ctx context.Context, input TransferInput) (*TransferOutput, error)
	GetAccount(ctx context.Context, id int64) (*domain.Account, error)
}

type transferUseCase struct {
	uow         domain.UnitOfWork
	accountRepo domain.AccountRepository
	logger      *slog.Logger
}

// NewTransferUseCase creates an instance of TransferUseCase with required dependencies.
func NewTransferUseCase(uow domain.UnitOfWork, accountRepo domain.AccountRepository, logger *slog.Logger) TransferUseCase {
	return &transferUseCase{
		uow:         uow,
		accountRepo: accountRepo,
		logger:      logger,
	}
}

// Execute orchestrates atomic P2P transfers with deadlock-free pessimistic locking.
func (uc *transferUseCase) Execute(ctx context.Context, input TransferInput) (*TransferOutput, error) {
	uc.logger.InfoContext(ctx, "initiating P2P transfer",
		slog.Int64("from_account_id", input.FromAccountID),
		slog.Int64("to_account_id", input.ToAccountID),
		slog.Int64("amount_cents", input.AmountCents),
	)

	// Validate basic inputs and build domain value objects
	amount, err := money.New(input.AmountCents)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidAmount, err)
	}

	transfer, err := domain.NewTransfer(input.FromAccountID, input.ToAccountID, amount)
	if err != nil {
		return nil, fmt.Errorf("transfer validation error: %w", err)
	}

	var fromAcc, toAcc *domain.Account

	// Execute within UnitOfWork (single DB transaction)
	err = uc.uow.Execute(ctx, func(txRepo domain.UnitOfWorkRepository) error {
		// Enforce lock ordering to prevent PostgreSQL deadlocks (Deadlock Prevention: lock smaller ID first)
		firstID, secondID := input.FromAccountID, input.ToAccountID
		if firstID > secondID {
			firstID, secondID = input.ToAccountID, input.FromAccountID
		}

		acc1, err := txRepo.Accounts().GetForUpdate(ctx, firstID)
		if err != nil {
			return fmt.Errorf("failed locking account %d: %w", firstID, err)
		}
		if acc1 == nil {
			return fmt.Errorf("%w: account ID %d", domain.ErrAccountNotFound, firstID)
		}

		acc2, err := txRepo.Accounts().GetForUpdate(ctx, secondID)
		if err != nil {
			return fmt.Errorf("failed locking account %d: %w", secondID, err)
		}
		if acc2 == nil {
			return fmt.Errorf("%w: account ID %d", domain.ErrAccountNotFound, secondID)
		}

		// Identify which one is from and to
		if acc1.ID == input.FromAccountID {
			fromAcc = acc1
			toAcc = acc2
		} else {
			fromAcc = acc2
			toAcc = acc1
		}

		// Debit sender wallet
		if err := fromAcc.Debit(amount); err != nil {
			return fmt.Errorf("debit failed: %w", err)
		}

		// Credit recipient wallet
		if err := toAcc.Credit(amount); err != nil {
			return fmt.Errorf("credit failed: %w", err)
		}

		// Persist updated balances
		if err := txRepo.Accounts().UpdateBalance(ctx, fromAcc); err != nil {
			return fmt.Errorf("failed updating sender balance: %w", err)
		}

		if err := txRepo.Accounts().UpdateBalance(ctx, toAcc); err != nil {
			return fmt.Errorf("failed updating recipient balance: %w", err)
		}

		// Record transfer ledger entry
		if err := txRepo.Transfers().Create(ctx, transfer); err != nil {
			return fmt.Errorf("failed recording transfer ledger: %w", err)
		}

		return nil
	})

	if err != nil {
		uc.logger.ErrorContext(ctx, "P2P transfer transaction failed",
			slog.Int64("from_account_id", input.FromAccountID),
			slog.Int64("to_account_id", input.ToAccountID),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("%w: %w", domain.ErrTransactionFailed, err)
	}

	uc.logger.InfoContext(ctx, "P2P transfer completed successfully",
		slog.Int64("transfer_id", transfer.ID),
		slog.Int64("from_account_id", fromAcc.ID),
		slog.Int64("to_account_id", toAcc.ID),
		slog.Int64("amount_cents", transfer.Amount.Cents()),
	)

	return &TransferOutput{
		TransferID:             transfer.ID,
		FromAccountID:          fromAcc.ID,
		ToAccountID:            toAcc.ID,
		AmountCents:            transfer.Amount.Cents(),
		FromAccountBalanceLeft: fromAcc.Balance.Cents(),
		ToAccountBalanceLeft:   toAcc.Balance.Cents(),
		Status:                 string(transfer.Status),
	}, nil
}

// GetAccount queries account details by ID.
func (uc *transferUseCase) GetAccount(ctx context.Context, id int64) (*domain.Account, error) {
	if id <= 0 {
		return nil, domain.ErrInvalidAccountID
	}
	acc, err := uc.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed getting account %d: %w", id, err)
	}
	if acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	return acc, nil
}
