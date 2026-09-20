package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/example/p2p-transfer/internal/domain"
)

var (
	ErrInvalidWallet       = errors.New("wallet ID is invalid")
	ErrSameWallet          = errors.New("source and destination wallets must differ")
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrTransferIDCollision = errors.New("transfer ID already exists")
)

type WalletRepository interface {
	LockByIDs(context.Context, string, string) (map[string]*domain.Wallet, error)
	Update(context.Context, *domain.Wallet) error
}
type TransferRepository interface {
	Create(context.Context, *domain.Transfer) error
}
type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context, WalletRepository, TransferRepository) error) error
}
type IDGenerator interface{ New() (string, error) }
type Clock interface{ Now() time.Time }

type TransferInput struct{ SourceWalletID, DestinationWalletID, Amount string }
type TransferOutput struct {
	ID, Amount string
	CreatedAt  time.Time
}

type TransferUseCase struct {
	tx     TransactionManager
	ids    IDGenerator
	clock  Clock
	logger *slog.Logger
}

func NewTransferUseCase(tx TransactionManager, ids IDGenerator, clock Clock, logger *slog.Logger) *TransferUseCase {
	return &TransferUseCase{tx: tx, ids: ids, clock: clock, logger: logger}
}

func (u *TransferUseCase) Execute(ctx context.Context, input TransferInput) (TransferOutput, error) {
	if input.SourceWalletID == "" || input.DestinationWalletID == "" {
		return TransferOutput{}, fmt.Errorf("validate wallet IDs: %w", ErrInvalidWallet)
	}
	if input.SourceWalletID == input.DestinationWalletID {
		return TransferOutput{}, fmt.Errorf("validate wallet IDs: %w", ErrSameWallet)
	}
	amount, err := domain.ParseMoney(input.Amount)
	if err != nil {
		return TransferOutput{}, fmt.Errorf("validate amount: %w", err)
	}
	id, err := u.ids.New()
	if err != nil {
		return TransferOutput{}, fmt.Errorf("generate transfer ID: %w", err)
	}
	transfer := &domain.Transfer{ID: id, SourceWalletID: input.SourceWalletID, DestinationWalletID: input.DestinationWalletID, Amount: amount, CreatedAt: u.clock.Now().UTC()}
	err = u.tx.WithinTransaction(ctx, func(txCtx context.Context, wallets WalletRepository, transfers TransferRepository) error {
		locked, lockErr := wallets.LockByIDs(txCtx, transfer.SourceWalletID, transfer.DestinationWalletID)
		if lockErr != nil {
			return fmt.Errorf("lock wallets: %w", lockErr)
		}
		source, sourceOK := locked[transfer.SourceWalletID]
		destination, destinationOK := locked[transfer.DestinationWalletID]
		if !sourceOK || !destinationOK {
			return fmt.Errorf("load transfer wallets: %w", ErrWalletNotFound)
		}
		if debitErr := source.Debit(amount); debitErr != nil {
			return fmt.Errorf("debit source: %w", debitErr)
		}
		if creditErr := destination.Credit(amount); creditErr != nil {
			return fmt.Errorf("credit destination: %w", creditErr)
		}
		if updateErr := wallets.Update(txCtx, source); updateErr != nil {
			return fmt.Errorf("persist source: %w", updateErr)
		}
		if updateErr := wallets.Update(txCtx, destination); updateErr != nil {
			return fmt.Errorf("persist destination: %w", updateErr)
		}
		if createErr := transfers.Create(txCtx, transfer); createErr != nil {
			return fmt.Errorf("persist transfer: %w", createErr)
		}
		return nil
	})
	if err != nil {
		return TransferOutput{}, fmt.Errorf("execute transfer: %w", err)
	}
	u.logger.InfoContext(ctx, "transfer completed", "transfer_id", id, "source_wallet_id", input.SourceWalletID, "destination_wallet_id", input.DestinationWalletID, "amount", amount.String())
	return TransferOutput{ID: id, Amount: amount.String(), CreatedAt: transfer.CreatedAt}, nil
}
