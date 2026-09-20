package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/example/pix-transfer/internal/domain"
	"github.com/example/pix-transfer/internal/repository"
)

// PixService concentra a lógica de negócio de transferências Pix P2P,
// delegando persistência ao repositório.
type PixService struct {
	repo *repository.TransferRepository
}

func NewPixService(repo *repository.TransferRepository) *PixService {
	return &PixService{repo: repo}
}

// TransferParams são os parâmetros de entrada expostos publicamente pelo
// serviço (camada acima do repositório, mais amigável para handlers/CLI).
type TransferParams struct {
	FromUserID     uuid.UUID
	ToUserID       uuid.UUID
	AmountCents    int64
	IdempotencyKey uuid.UUID // opcional; se zero-value, é gerada automaticamente
}

// Transfer executa uma transferência Pix P2P entre duas carteiras digitais:
//  1. Valida os dados de entrada (valor positivo, contas diferentes).
//  2. Delega ao repositório a operação atômica de debitar/creditar,
//     que também verifica saldo suficiente e existência das carteiras.
//
// Retorna o registro da transferência criada ou um erro de domínio
// (ErrInvalidAmount, ErrSameAccount, ErrWalletNotFound,
// ErrInsufficientFunds, ErrDuplicateTransfer).
func (s *PixService) Transfer(ctx context.Context, p TransferParams) (*domain.Transfer, error) {
	if p.IdempotencyKey == uuid.Nil {
		p.IdempotencyKey = uuid.New()
	}

	input := domain.TransferInput{
		FromUserID:     p.FromUserID,
		ToUserID:       p.ToUserID,
		AmountCents:    p.AmountCents,
		IdempotencyKey: p.IdempotencyKey,
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	transfer, err := s.repo.Execute(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("executar transferência: %w", err)
	}

	return transfer, nil
}

// GetBalance retorna o saldo (em centavos) da carteira de um usuário.
func (s *PixService) GetBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	return s.repo.GetBalance(ctx, userID)
}
