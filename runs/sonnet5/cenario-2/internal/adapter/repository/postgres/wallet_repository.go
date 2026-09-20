package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"p2p-transfer/internal/domain"
)

// sqlExecutor abstrai o subconjunto de métodos de *sql.Tx utilizados pelos
// repositórios, permitindo testá-los com um double leve (sqlmock ou
// implementação manual) sem depender de um banco de dados real.
type sqlExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// WalletRepository implementa usecase.WalletRepository usando Postgres.
type WalletRepository struct {
	exec sqlExecutor
}

// NewWalletRepository constrói o repositório de carteiras vinculado à
// transação corrente (tx implementa sqlExecutor).
func NewWalletRepository(exec sqlExecutor) *WalletRepository {
	return &WalletRepository{exec: exec}
}

// LockForUpdate busca as carteiras identificadas por ids aplicando
// bloqueio pessimista via "SELECT ... FOR UPDATE".
//
// Prevenção de deadlock: quando duas transações concorrentes precisam
// bloquear o mesmo par de carteiras (ex: transferência A->B concorrente
// com uma transferência B->A), adquirir os locks em ordens diferentes
// causaria deadlock no Postgres. Para eliminar essa condição, os IDs são
// ordenados lexicograficamente antes da consulta, garantindo que todas as
// transações concorrentes adquiram os locks sempre na mesma ordem
// determinística.
func (r *WalletRepository) LockForUpdate(ctx context.Context, ids []string) (map[string]*domain.Wallet, error) {
	if len(ids) == 0 {
		return map[string]*domain.Wallet{}, nil
	}

	// Remove duplicatas e ordena para garantir ordem determinística de
	// aquisição de locks entre transações concorrentes.
	uniqueSorted := uniqueSortedIDs(ids)

	query, args := buildLockForUpdateQuery(uniqueSorted)

	rows, err := r.exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar SELECT FOR UPDATE em carteiras: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*domain.Wallet, len(uniqueSorted))
	for rows.Next() {
		var (
			id      string
			ownerID string
			balance int64
			status  string
		)
		if err := rows.Scan(&id, &ownerID, &balance, &status); err != nil {
			return nil, fmt.Errorf("erro ao ler linha de carteira bloqueada: %w", err)
		}
		wallet, err := domain.NewWallet(id, ownerID, domain.Money(balance), domain.WalletStatus(status))
		if err != nil {
			return nil, fmt.Errorf("dados inconsistentes para carteira %s: %w", id, err)
		}
		result[id] = wallet
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("erro ao iterar resultado de carteiras bloqueadas: %w", err)
	}

	for _, id := range ids {
		if _, ok := result[id]; !ok {
			return nil, fmt.Errorf("carteira %s: %w", id, domain.ErrWalletNotFound)
		}
	}

	return result, nil
}

// Save persiste o saldo atualizado da carteira via UPDATE. Deve ser
// chamado dentro da mesma transação em que a carteira foi obtida via
// LockForUpdate, garantindo que o UPDATE ocorra sobre a linha já
// bloqueada.
func (r *WalletRepository) Save(ctx context.Context, wallet *domain.Wallet) error {
	const query = `UPDATE wallets SET balance = $1, status = $2, updated_at = now() WHERE id = $3`

	result, err := r.exec.ExecContext(ctx, query, wallet.Balance.Int64(), string(wallet.Status), wallet.ID)
	if err != nil {
		return fmt.Errorf("erro ao atualizar carteira %s: %w", wallet.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("erro ao verificar linhas afetadas ao salvar carteira %s: %w", wallet.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("carteira %s: %w", wallet.ID, domain.ErrWalletNotFound)
	}

	return nil
}

// uniqueSortedIDs remove duplicatas e ordena a lista de IDs
// lexicograficamente, garantindo ordem determinística de bloqueio.
func uniqueSortedIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// buildLockForUpdateQuery monta a query parametrizada de SELECT FOR UPDATE
// para o conjunto de IDs fornecido, evitando concatenação de strings de
// entrada do usuário (proteção contra SQL Injection).
func buildLockForUpdateQuery(ids []string) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := "SELECT id, owner_id, balance, status FROM wallets WHERE id IN (" +
		joinPlaceholders(placeholders) + ") ORDER BY id FOR UPDATE"

	return query, args
}

func joinPlaceholders(placeholders []string) string {
	out := ""
	for i, p := range placeholders {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// ErrRepository é um erro sentinela genérico usado para encapsular falhas
// de infraestrutura não mapeadas para erros de domínio específicos.
var ErrRepository = errors.New("erro de repositório")
