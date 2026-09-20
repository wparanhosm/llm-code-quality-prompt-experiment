package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"wallet-p2p/internal/domain"
)

// TransferInput carrega os dados necessários para uma transferência P2P.
type TransferInput struct {
	FromWalletID   string
	ToWalletID     string
	AmountCents    int64
	IdempotencyKey string
}

// TransferOutput é o resultado de uma transferência bem-sucedida.
type TransferOutput struct {
	TransactionID       string
	FromWalletID        string
	ToWalletID          string
	AmountCents         int64
	FromNewBalanceCents int64
	ToNewBalanceCents   int64
}

// TransferUseCase orquestra a transferência P2P com garantias ACID:
// débito na origem + crédito no destino + registro do histórico ocorrem
// em UMA única transação de banco. Qualquer falha provoca rollback total.
//
// Anti-double-spending: as linhas das carteiras são travadas com
// SELECT ... FOR UPDATE, sempre em ordem determinística (menor ID primeiro),
// eliminando deadlocks entre transferências concorrentes cruzadas (A→B e B→A).
type TransferUseCase struct {
	uow    UnitOfWork
	repo   WalletRepository
	clock  Clock
	logger *slog.Logger
}

// NewTransferUseCase constrói o usecase com todas as dependências injetadas.
func NewTransferUseCase(uow UnitOfWork, repo WalletRepository, clock Clock, logger *slog.Logger) *TransferUseCase {
	return &TransferUseCase{
		uow:    uow,
		repo:   repo,
		clock:  clock,
		logger: logger,
	}
}

// Execute realiza a transferência P2P de forma atômica.
//
// Erros possíveis (todos comparáveis com errors.Is):
//   - domain.ErrInvalidWalletID: ID vazio
//   - domain.ErrSameWallet: origem == destino
//   - domain.ErrInvalidAmount: valor <= 0
//   - domain.ErrWalletNotFound: carteira inexistente
//   - domain.ErrInsufficientFunds: saldo insuficiente na origem
func (uc *TransferUseCase) Execute(ctx context.Context, input TransferInput) (TransferOutput, error) {
	if err := validateTransferInput(input); err != nil {
		return TransferOutput{}, err
	}

	amount := domain.MustMoney(input.AmountCents)
	var output TransferOutput

	err := uc.uow.Do(ctx, func(ctx context.Context, tx Tx) error {
		// Ordenação determinística de locks: menor ID primeiro.
		// Previne deadlock entre transferências simultâneas A→B e B→A,
		// pois ambas adquirem os locks na mesma ordem global.
		lockOrder := []string{input.FromWalletID, input.ToWalletID}
		sort.Strings(lockOrder)

		locked := make(map[string]domain.Wallet, 2)
		for _, walletID := range lockOrder {
			wallet, err := uc.repo.GetByIDForUpdate(ctx, tx, walletID)
			if err != nil {
				return fmt.Errorf("usecase: falha ao travar carteira %s: %w", walletID, err)
			}
			locked[walletID] = wallet
		}

		from := locked[input.FromWalletID]
		to := locked[input.ToWalletID]

		if !from.CanDebit(amount) {
			return domain.ErrInsufficientFunds
		}

		now := uc.clock.Now()

		from, err := from.Debit(amount, now)
		if err != nil {
			return fmt.Errorf("usecase: falha ao debitar carteira %s: %w", from.ID(), err)
		}
		to = to.Credit(amount, now)

		if err := uc.repo.UpdateBalance(ctx, tx, from); err != nil {
			return fmt.Errorf("usecase: falha ao persistir débito da carteira %s: %w", from.ID(), err)
		}
		if err := uc.repo.UpdateBalance(ctx, tx, to); err != nil {
			return fmt.Errorf("usecase: falha ao persistir crédito da carteira %s: %w", to.ID(), err)
		}

		transaction, err := domain.NewTransaction(input.FromWalletID, input.ToWalletID, amount, input.IdempotencyKey, now)
		if err != nil {
			return fmt.Errorf("usecase: falha ao criar registro da transação: %w", err)
		}
		if err := uc.repo.InsertTransaction(ctx, tx, transaction); err != nil {
			return fmt.Errorf("usecase: falha ao registrar transação: %w", err)
		}

		output = TransferOutput{
			TransactionID:       transaction.ID(),
			FromWalletID:        from.ID(),
			ToWalletID:          to.ID(),
			AmountCents:         amount.Cents(),
			FromNewBalanceCents: from.Balance().Cents(),
			ToNewBalanceCents:   to.Balance().Cents(),
		}
		return nil
	})
	if err != nil {
		uc.logger.WarnContext(ctx, "transferência p2p não concluída",
			slog.String("from_wallet_id", input.FromWalletID),
			slog.String("to_wallet_id", input.ToWalletID),
			slog.Int64("amount_cents", input.AmountCents),
			slog.String("error", err.Error()),
		)
		return TransferOutput{}, err
	}

	uc.logger.InfoContext(ctx, "transferência p2p concluída",
		slog.String("transaction_id", output.TransactionID),
		slog.String("from_wallet_id", output.FromWalletID),
		slog.String("to_wallet_id", output.ToWalletID),
		slog.Int64("amount_cents", output.AmountCents),
	)
	return output, nil
}

// validateTransferInput aplica as regras de validação de entrada do domínio.
func validateTransferInput(input TransferInput) error {
	if input.FromWalletID == "" || input.ToWalletID == "" {
		return domain.ErrInvalidWalletID
	}
	if input.FromWalletID == input.ToWalletID {
		return domain.ErrSameWallet
	}
	if input.AmountCents <= 0 {
		return domain.ErrInvalidAmount
	}
	return nil
}

// IsNotFound reporta se o erro representa carteira inexistente.
func IsNotFound(err error) bool { return errors.Is(err, domain.ErrWalletNotFound) }
