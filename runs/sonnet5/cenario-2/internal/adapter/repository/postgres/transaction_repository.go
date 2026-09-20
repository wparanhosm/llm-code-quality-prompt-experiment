package postgres

import (
	"context"
	"fmt"

	"p2p-transfer/internal/domain"
)

// TransactionRepository implementa usecase.TransactionRepository usando
// Postgres, persistindo o histórico imutável de transferências P2P para
// fins de auditoria e rastreabilidade.
type TransactionRepository struct {
	exec sqlExecutor
}

// NewTransactionRepository constrói o repositório de transações vinculado
// à transação corrente (tx implementa sqlExecutor).
func NewTransactionRepository(exec sqlExecutor) *TransactionRepository {
	return &TransactionRepository{exec: exec}
}

// Save insere o registro de transação (completada ou falha) na tabela de
// histórico. A tabela deve possuir uma constraint UNIQUE em (id) para
// garantir idempotência caso o mesmo ID de transação seja submetido
// novamente (ex: retry de cliente).
func (r *TransactionRepository) Save(ctx context.Context, tx *domain.Transaction) error {
	const query = `
		INSERT INTO transactions (id, from_wallet_id, to_wallet_id, amount, status, failure_reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.exec.ExecContext(
		ctx,
		query,
		tx.ID,
		tx.FromWalletID,
		tx.ToWalletID,
		tx.Amount.Int64(),
		string(tx.Status),
		nullableString(tx.FailureReason),
		tx.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("erro ao persistir transação %s: %w", tx.ID, err)
	}

	return nil
}

// nullableString converte uma string vazia em nil para que o driver SQL a
// grave como NULL em vez de string vazia, mantendo a semântica correta da
// coluna opcional failure_reason.
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
