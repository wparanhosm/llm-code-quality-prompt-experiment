package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"p2p-wallet/internal/domain"
)

type TransferInput struct {
	FromWalletID string
	ToWalletID   string
	Amount       int64
}

type TransferOutput struct {
	TransactionID string
}

type TransferUseCase struct {
	walletRepo domain.WalletRepository
	txRepo     domain.TransactionRepository
	txManager  domain.TxManager
	idGen      domain.IDGenerator
	logger     *slog.Logger
}

func NewTransferUseCase(
	walletRepo domain.WalletRepository,
	txRepo domain.TransactionRepository,
	txManager domain.TxManager,
	idGen domain.IDGenerator,
	logger *slog.Logger,
) *TransferUseCase {
	return &TransferUseCase{
		walletRepo: walletRepo,
		txRepo:     txRepo,
		txManager:  txManager,
		idGen:      idGen,
		logger:     logger,
	}
}

func (uc *TransferUseCase) Execute(ctx context.Context, input TransferInput) (*TransferOutput, error) {
	if input.Amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}
	if input.FromWalletID == input.ToWalletID {
		return nil, domain.ErrSameWallet
	}

	ids := []string{input.FromWalletID, input.ToWalletID}
	sort.Strings(ids)

	txID := uc.idGen.NewID()

	uc.logger.InfoContext(ctx, "starting p2p transfer",
		slog.String("transaction_id", txID),
		slog.String("from", input.FromWalletID),
		slog.String("to", input.ToWalletID),
		slog.Int64("amount", input.Amount),
	)

	err := uc.txManager.RunInTx(ctx, func(txCtx context.Context) error {
		wallets := make(map[string]*domain.Wallet, 2)
		for _, id := range ids {
			w, err := uc.walletRepo.FindByIDForUpdate(txCtx, id)
			if err != nil {
				return fmt.Errorf("lock wallet %s: %w", id, err)
			}
			wallets[id] = w
		}

		source := wallets[input.FromWalletID]
		dest := wallets[input.ToWalletID]

		if err := source.Debit(input.Amount); err != nil {
			return err
		}
		if err := dest.Credit(input.Amount); err != nil {
			return err
		}

		if err := uc.walletRepo.UpdateBalance(txCtx, source); err != nil {
			return fmt.Errorf("update source balance: %w", err)
		}
		if err := uc.walletRepo.UpdateBalance(txCtx, dest); err != nil {
			return fmt.Errorf("update dest balance: %w", err)
		}

		record, err := domain.NewTransaction(txID, input.FromWalletID, input.ToWalletID, input.Amount)
		if err != nil {
			return err
		}

		if err := uc.txRepo.Create(txCtx, record); err != nil {
			return fmt.Errorf("create transaction record: %w", err)
		}

		return nil
	})

	if err != nil {
		uc.logger.ErrorContext(ctx, "transfer failed",
			slog.String("transaction_id", txID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	uc.logger.InfoContext(ctx, "transfer completed",
		slog.String("transaction_id", txID),
	)

	return &TransferOutput{TransactionID: txID}, nil
}
