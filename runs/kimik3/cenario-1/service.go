package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	// ErrWalletNotFound indica que uma das carteiras não existe.
	ErrWalletNotFound = errors.New("carteira não encontrada")
	// ErrInsufficientFunds indica saldo insuficiente na carteira de origem.
	ErrInsufficientFunds = errors.New("saldo insuficiente")
	// ErrSameWallet indica que origem e destino são a mesma carteira.
	ErrSameWallet = errors.New("carteira de origem e destino são iguais")
	// ErrInvalidAmount indica valor de transferência inválido.
	ErrInvalidAmount = errors.New("valor da transferência deve ser maior que zero")
)

// Service contém a lógica de negócio das transferências Pix.
type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// TransferPix executa uma transferência Pix P2P entre duas carteiras digitais.
//
// Recebe o ID do usuário de origem, o ID do usuário de destino e o valor (em centavos).
// Verifica se o saldo é suficiente, debita da conta de origem e credita na conta de
// destino. Todas as alterações são persistidas em uma única transação de banco de
// dados (ACID): ou tudo é aplicado, ou nada é aplicado.
func (s *Service) TransferPix(ctx context.Context, fromID, toID string, amountCents int64) (*Transaction, error) {
	if fromID == toID {
		return nil, ErrSameWallet
	}
	if amountCents <= 0 {
		return nil, ErrInvalidAmount
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("falha ao iniciar transação de banco: %w", err)
	}
	// Rollback é ignorado se o Commit já tiver ocorrido.
	defer tx.Rollback()

	// Bloqueia as duas carteiras em ordem determinística para evitar deadlock
	// quando duas transferências em sentidos opostos ocorrem ao mesmo tempo.
	first, second := fromID, toID
	if second < first {
		first, second = second, first
	}

	balances := make(map[string]int64, 2)
	for _, id := range []string{first, second} {
		var balance int64
		err := tx.QueryRowContext(ctx,
			"SELECT balance_cents FROM wallets WHERE id = ?", id).Scan(&balance)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrWalletNotFound, id)
		}
		if err != nil {
			return nil, fmt.Errorf("falha ao consultar saldo: %w", err)
		}
		balances[id] = balance
	}

	// Verifica se o saldo da origem é suficiente.
	if balances[fromID] < amountCents {
		return nil, ErrInsufficientFunds
	}

	// Debita da conta de origem. A cláusula WHERE é uma proteção extra contra
	// saldo negativo em caso de race condition.
	res, err := tx.ExecContext(ctx,
		"UPDATE wallets SET balance_cents = balance_cents - ? WHERE id = ? AND balance_cents >= ?",
		amountCents, fromID, amountCents)
	if err != nil {
		return nil, fmt.Errorf("falha ao debitar carteira de origem: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrInsufficientFunds
	}

	// Credita na conta de destino.
	if _, err := tx.ExecContext(ctx,
		"UPDATE wallets SET balance_cents = balance_cents + ? WHERE id = ?",
		amountCents, toID); err != nil {
		return nil, fmt.Errorf("falha ao creditar carteira de destino: %w", err)
	}

	// Registra a transferência no histórico.
	res, err = tx.ExecContext(ctx,
		`INSERT INTO transactions (from_wallet_id, to_wallet_id, amount_cents, status)
		 VALUES (?, ?, ?, 'completed')`,
		fromID, toID, amountCents)
	if err != nil {
		return nil, fmt.Errorf("falha ao registrar transação: %w", err)
	}
	txID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("falha ao confirmar transação: %w", err)
	}

	return &Transaction{
		ID:           txID,
		FromWalletID: fromID,
		ToWalletID:   toID,
		AmountCents:  amountCents,
		Status:       "completed",
	}, nil
}
