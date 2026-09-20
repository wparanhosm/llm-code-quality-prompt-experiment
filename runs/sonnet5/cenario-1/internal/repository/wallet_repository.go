package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/pix-transfer/internal/domain"
)

// TransferRepository encapsula todo o acesso a dados relacionado a
// carteiras e transferências, garantindo atomicidade via transações SQL.
type TransferRepository struct {
	pool *pgxpool.Pool
}

func NewTransferRepository(pool *pgxpool.Pool) *TransferRepository {
	return &TransferRepository{pool: pool}
}

// Execute realiza a transferência Pix de forma atômica:
//  1. Abre uma transação com o banco.
//  2. Bloqueia (SELECT ... FOR UPDATE) as duas linhas de carteira
//     SEMPRE na mesma ordem (por user_id) para evitar deadlock quando
//     duas transferências acontecem em sentidos opostos ao mesmo tempo.
//  3. Verifica existência das carteiras e saldo suficiente.
//  4. Debita da origem e credita no destino.
//  5. Grava o registro da transferência (ledger) para idempotência/auditoria.
//  6. Faz commit. Qualquer erro no meio do caminho causa rollback total.
func (r *TransferRepository) Execute(ctx context.Context, in domain.TransferInput) (*domain.Transfer, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("iniciar transação: %w", err)
	}
	// Rollback é seguro de chamar mesmo após um Commit bem-sucedido
	// (torna-se um no-op), então garantimos limpeza em qualquer saída.
	defer tx.Rollback(ctx)

	// Bloqueia as linhas sempre na mesma ordem determinística (por UUID)
	// para prevenir deadlocks entre transferências concorrentes A->B e B->A.
	firstID, secondID := in.FromUserID, in.ToUserID
	if firstID.String() > secondID.String() {
		firstID, secondID = secondID, firstID
	}

	if err := lockWallet(ctx, tx, firstID); err != nil {
		return nil, err
	}
	if err := lockWallet(ctx, tx, secondID); err != nil {
		return nil, err
	}

	fromWallet, err := getWalletForUpdate(ctx, tx, in.FromUserID)
	if err != nil {
		return nil, err
	}
	toWallet, err := getWalletForUpdate(ctx, tx, in.ToUserID)
	if err != nil {
		return nil, err
	}

	if fromWallet.BalanceCents < in.AmountCents {
		return nil, domain.ErrInsufficientFunds
	}

	// Debita da origem
	if _, err := tx.Exec(ctx,
		`UPDATE wallets SET balance_cents = balance_cents - $1, updated_at = now() WHERE user_id = $2`,
		in.AmountCents, in.FromUserID,
	); err != nil {
		return nil, fmt.Errorf("debitar origem: %w", err)
	}

	// Credita no destino
	if _, err := tx.Exec(ctx,
		`UPDATE wallets SET balance_cents = balance_cents + $1, updated_at = now() WHERE user_id = $2`,
		in.AmountCents, in.ToUserID,
	); err != nil {
		return nil, fmt.Errorf("creditar destino: %w", err)
	}

	transfer := &domain.Transfer{
		IdempotencyKey: in.IdempotencyKey,
		FromUserID:     in.FromUserID,
		ToUserID:       in.ToUserID,
		AmountCents:    in.AmountCents,
		Status:         domain.TransferCompleted,
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO transfers (idempotency_key, from_user_id, to_user_id, amount_cents, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`, transfer.IdempotencyKey, transfer.FromUserID, transfer.ToUserID, transfer.AmountCents, transfer.Status)

	if err := row.Scan(&transfer.ID, &transfer.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Violação de unique constraint em idempotency_key:
			// esta transferência já foi processada anteriormente.
			return nil, domain.ErrDuplicateTransfer
		}
		return nil, fmt.Errorf("registrar transferência: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit da transação: %w", err)
	}

	toWallet.BalanceCents += in.AmountCents // apenas para eventual uso futuro/log
	_ = toWallet

	return transfer, nil
}

// lockWallet adquire um lock de escrita (FOR UPDATE) na linha da carteira
// sem retornar os dados, usado apenas para garantir a ordem de bloqueio.
func lockWallet(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	var exists bool
	err := tx.QueryRow(ctx,
		`SELECT true FROM wallets WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: user_id=%s", domain.ErrWalletNotFound, userID)
	}
	if err != nil {
		return fmt.Errorf("bloquear carteira %s: %w", userID, err)
	}
	return nil
}

// getWalletForUpdate busca os dados completos da carteira já bloqueada
// (deve ser chamada após lockWallet, dentro da mesma transação).
func getWalletForUpdate(ctx context.Context, tx pgx.Tx, userID uuid.UUID) (*domain.Wallet, error) {
	w := &domain.Wallet{}
	err := tx.QueryRow(ctx,
		`SELECT id, user_id, balance_cents, updated_at FROM wallets WHERE user_id = $1`,
		userID,
	).Scan(&w.ID, &w.UserID, &w.BalanceCents, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: user_id=%s", domain.ErrWalletNotFound, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("buscar carteira %s: %w", userID, err)
	}
	return w, nil
}

// GetBalance retorna o saldo atual (em centavos) da carteira de um usuário.
// Útil para endpoints de consulta e para os testes.
func (r *TransferRepository) GetBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	var balance int64
	err := r.pool.QueryRow(ctx,
		`SELECT balance_cents FROM wallets WHERE user_id = $1`,
		userID,
	).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrWalletNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("consultar saldo: %w", err)
	}
	return balance, nil
}
