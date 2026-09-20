package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Erros de domínio conhecidos. O handler HTTP mapeia estes erros para
// códigos de status apropriados (400, 404, 409, etc).
var (
	ErrInvalidAmount     = errors.New("valor da transferência deve ser positivo")
	ErrSameAccount       = errors.New("conta de origem e destino não podem ser iguais")
	ErrWalletNotFound    = errors.New("carteira não encontrada")
	ErrInsufficientFunds = errors.New("saldo insuficiente")
	ErrDuplicateTransfer = errors.New("transferência já processada (idempotency key duplicada)")
)

// Wallet representa a carteira digital de um usuário.
// O saldo é mantido em centavos (int64) para evitar erros de
// arredondamento de ponto flutuante ao lidar com dinheiro.
type Wallet struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	BalanceCents int64
	UpdatedAt    time.Time
}

// TransferStatus representa o resultado final de uma transferência.
type TransferStatus string

const (
	TransferCompleted TransferStatus = "COMPLETED"
	TransferFailed    TransferStatus = "FAILED"
)

// Transfer é o registro de uma transferência Pix P2P, usado como
// ledger/auditoria e para garantir idempotência.
type Transfer struct {
	ID             uuid.UUID
	IdempotencyKey uuid.UUID
	FromUserID     uuid.UUID
	ToUserID       uuid.UUID
	AmountCents    int64
	Status         TransferStatus
	FailureReason  string
	CreatedAt      time.Time
}

// TransferInput agrupa os parâmetros necessários para realizar uma
// transferência entre duas carteiras.
type TransferInput struct {
	FromUserID     uuid.UUID
	ToUserID       uuid.UUID
	AmountCents    int64
	IdempotencyKey uuid.UUID
}

// Validate executa validações básicas de negócio antes de tocar no banco.
func (in TransferInput) Validate() error {
	if in.AmountCents <= 0 {
		return ErrInvalidAmount
	}
	if in.FromUserID == in.ToUserID {
		return ErrSameAccount
	}
	return nil
}
