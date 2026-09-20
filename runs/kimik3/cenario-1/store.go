package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
)

// ErrWalletExists é retornado ao tentar criar uma carteira com ID já cadastrado.
var ErrWalletExists = errors.New("carteira já existe")

// Wallet representa uma carteira digital persistida no banco.
type Wallet struct {
	ID           string
	BalanceCents int64
	CreatedAt    string
}

// Transaction representa uma transferência Pix registrada no banco.
type Transaction struct {
	ID           int64
	FromWalletID string
	ToWalletID   string
	AmountCents  int64
	Status       string
	CreatedAt    string
}

// toCents converte um valor em reais para centavos (evita erros de ponto flutuante).
func toCents(reais float64) int64 {
	return int64(math.Round(reais * 100))
}

// toReais converte centavos para reais (apenas para exibição).
func toReais(cents int64) float64 {
	return float64(cents) / 100
}

// Store é a camada de acesso a dados (consultas fora da transação de transferência).
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateWallet cria uma carteira com saldo inicial.
func (s *Store) CreateWallet(ctx context.Context, id string, initialCents int64) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO wallets (id, balance_cents) VALUES (?, ?)", id, initialCents)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s", ErrWalletExists, id)
		}
		return fmt.Errorf("falha ao criar carteira: %w", err)
	}
	return nil
}

// GetWallet busca uma carteira pelo ID.
func (s *Store) GetWallet(ctx context.Context, id string) (*Wallet, error) {
	var w Wallet
	err := s.db.QueryRowContext(ctx,
		"SELECT id, balance_cents, created_at FROM wallets WHERE id = ?", id).
		Scan(&w.ID, &w.BalanceCents, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrWalletNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar carteira: %w", err)
	}
	return &w, nil
}

// ListTransactions retorna o histórico de transferências, da mais recente para a mais antiga.
func (s *Store) ListTransactions(ctx context.Context) ([]Transaction, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, from_wallet_id, to_wallet_id, amount_cents, status, created_at
		 FROM transactions ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar transações: %w", err)
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.FromWalletID, &t.ToWalletID, &t.AmountCents, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}
