package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"p2p-transfer/internal/domain"
)

// TransferInput representa os dados de entrada para uma transferência P2P
// entre carteiras digitais.
type TransferInput struct {
	FromWalletID string
	ToWalletID   string
	AmountCents  int64
}

// TransferOutput representa o resultado de uma transferência P2P bem
// sucedida.
type TransferOutput struct {
	TransactionID  string
	FromWalletID   string
	ToWalletID     string
	AmountCents    int64
	FromNewBalance int64
	ToNewBalance   int64
}

// TransferUseCase implementa o caso de uso de transferência P2P entre
// carteiras digitais, garantindo atomicidade (ACID) e proteção contra
// condições de corrida (double spending) através de bloqueio pessimista
// coordenado pela camada de infraestrutura via UnitOfWork.
//
// Todas as dependências são injetadas via interfaces (inversão de
// dependência), permitindo testes unitários completos com mocks e mantendo
// o núcleo de negócio livre de acoplamento a frameworks ou drivers de
// banco de dados.
type TransferUseCase struct {
	uow    UnitOfWork
	ids    IDGenerator
	logger *slog.Logger
}

// NewTransferUseCase constrói um TransferUseCase válido. Todas as
// dependências são obrigatórias; a construção falha explicitamente (panic)
// se alguma dependência crítica não for fornecida, pois isso representa um
// erro de programação (wiring incorreto), não uma condição de negócio.
func NewTransferUseCase(uow UnitOfWork, ids IDGenerator, logger *slog.Logger) *TransferUseCase {
	if uow == nil {
		panic("usecase: UnitOfWork não pode ser nil")
	}
	if ids == nil {
		panic("usecase: IDGenerator não pode ser nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &TransferUseCase{uow: uow, ids: ids, logger: logger}
}

// Execute realiza a transferência de fundos entre duas carteiras digitais.
//
// Fluxo:
//  1. Validação do valor de entrada (deve ser positivo) e identificadores.
//  2. Abertura de uma transação de banco de dados via UnitOfWork.
//  3. Bloqueio pessimista das duas carteiras envolvidas (SELECT FOR UPDATE),
//     com ordenação determinística dos IDs feita pela implementação de
//     infraestrutura para prevenir deadlocks.
//  4. Validação de regras de negócio: carteiras existem, estão ativas e a
//     origem possui saldo suficiente.
//  5. Débito da origem, crédito do destino e persistência do histórico da
//     transação, tudo dentro da mesma transação atômica.
//  6. Commit (sucesso) ou rollback (qualquer erro) automático via UnitOfWork.
func (uc *TransferUseCase) Execute(ctx context.Context, in TransferInput) (*TransferOutput, error) {
	amount, err := domain.NewMoney(in.AmountCents)
	if err != nil {
		return nil, fmt.Errorf("transferência inválida: %w", err)
	}
	if in.FromWalletID == "" || in.ToWalletID == "" {
		return nil, fmt.Errorf("transferência inválida: %w", errors.New("ids de carteira são obrigatórios"))
	}
	if in.FromWalletID == in.ToWalletID {
		return nil, fmt.Errorf("transferência inválida: %w", domain.ErrSameWallet)
	}

	txID := uc.ids.NewID()

	uc.logger.InfoContext(ctx, "iniciando transferência p2p",
		slog.String("transaction_id", txID),
		slog.String("from_wallet_id", in.FromWalletID),
		slog.String("to_wallet_id", in.ToWalletID),
		slog.Int64("amount_cents", int64(amount)),
	)

	var out *TransferOutput

	err = uc.uow.Execute(ctx, func(ctx context.Context, repos TransactionalRepositories) error {
		wallets, err := repos.WalletRepository().LockForUpdate(ctx, []string{in.FromWalletID, in.ToWalletID})
		if err != nil {
			return fmt.Errorf("erro ao bloquear carteiras para atualização: %w", err)
		}

		from, ok := wallets[in.FromWalletID]
		if !ok {
			return fmt.Errorf("carteira de origem: %w", domain.ErrWalletNotFound)
		}
		to, ok := wallets[in.ToWalletID]
		if !ok {
			return fmt.Errorf("carteira de destino: %w", domain.ErrWalletNotFound)
		}

		if err := from.Debit(amount); err != nil {
			return fmt.Errorf("falha ao debitar carteira de origem %s: %w", from.ID, err)
		}
		if err := to.Credit(amount); err != nil {
			return fmt.Errorf("falha ao creditar carteira de destino %s: %w", to.ID, err)
		}

		if err := repos.WalletRepository().Save(ctx, from); err != nil {
			return fmt.Errorf("falha ao salvar carteira de origem %s: %w", from.ID, err)
		}
		if err := repos.WalletRepository().Save(ctx, to); err != nil {
			return fmt.Errorf("falha ao salvar carteira de destino %s: %w", to.ID, err)
		}

		tx, err := domain.NewTransaction(txID, in.FromWalletID, in.ToWalletID, amount)
		if err != nil {
			return fmt.Errorf("falha ao criar registro de transação: %w", err)
		}
		if err := repos.TransactionRepository().Save(ctx, tx); err != nil {
			return fmt.Errorf("falha ao persistir histórico da transação: %w", err)
		}

		out = &TransferOutput{
			TransactionID:  tx.ID,
			FromWalletID:   from.ID,
			ToWalletID:     to.ID,
			AmountCents:    amount.Int64(),
			FromNewBalance: from.Balance.Int64(),
			ToNewBalance:   to.Balance.Int64(),
		}
		return nil
	})

	if err != nil {
		uc.logger.ErrorContext(ctx, "transferência p2p falhou",
			slog.String("transaction_id", txID),
			slog.String("from_wallet_id", in.FromWalletID),
			slog.String("to_wallet_id", in.ToWalletID),
			slog.Any("error", err),
		)
		return nil, err
	}

	uc.logger.InfoContext(ctx, "transferência p2p concluída com sucesso",
		slog.String("transaction_id", out.TransactionID),
		slog.Int64("from_new_balance", out.FromNewBalance),
		slog.Int64("to_new_balance", out.ToNewBalance),
	)

	return out, nil
}
