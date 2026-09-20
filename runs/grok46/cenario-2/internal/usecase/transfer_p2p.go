package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wallet/p2p-transfer/internal/domain"
)

// TransferInput is the command to move money between wallets.
type TransferInput struct {
	FromWalletID string
	ToWalletID   string
	Amount       domain.Money
}

// TransferP2P orchestrates validation and atomic persistence.
type TransferP2P struct {
	uow    domain.UnitOfWork
	logger *slog.Logger
}

func NewTransferP2P(uow domain.UnitOfWork, logger *slog.Logger) *TransferP2P {
	if logger == nil {
		logger = slog.Default()
	}
	return &TransferP2P{uow: uow, logger: logger}
}

func (uc *TransferP2P) Execute(ctx context.Context, in TransferInput) (*domain.Transfer, error) {
	if in.FromWalletID == "" || in.ToWalletID == "" {
		return nil, domain.ErrInvalidWalletID
	}
	if in.FromWalletID == in.ToWalletID {
		return nil, domain.ErrSameWalletTransfer
	}
	if err := in.Amount.ValidatePositive(); err != nil {
		return nil, err
	}

	uc.logger.InfoContext(ctx, "p2p transfer started",
		slog.String("from", in.FromWalletID),
		slog.String("to", in.ToWalletID),
		slog.Int64("amount_cents", int64(in.Amount)),
	)

	tr, err := uc.uow.ExecuteTransfer(ctx, in.FromWalletID, in.ToWalletID, in.Amount)
	if err != nil {
		uc.logger.ErrorContext(ctx, "p2p transfer failed", slog.String("error", err.Error()))
		return nil, fmt.Errorf("execute transfer: %w", err)
	}

	uc.logger.InfoContext(ctx, "p2p transfer completed", slog.String("transfer_id", tr.ID))
	return tr, nil
}
