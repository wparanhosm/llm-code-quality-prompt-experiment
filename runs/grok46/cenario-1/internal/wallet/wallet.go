package wallet

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInsufficientFunds = errors.New("saldo insuficiente")
	ErrSameWallet        = errors.New("origem e destino nao podem ser iguais")
	ErrInvalidAmount     = errors.New("valor deve ser maior que zero")
	ErrWalletNotFound    = errors.New("carteira nao encontrada")
)

type TransferRequest struct {
	FromUserID int64
	ToUserID   int64
	Amount     int64 // centavos
}

type TransferResult struct {
	ID         int64
	FromUserID int64
	ToUserID   int64
	Amount     int64
	CreatedAt  time.Time
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) TransferPixP2P(fromUserID, toUserID, amount int64) (*TransferResult, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if fromUserID == toUserID {
		return nil, ErrSameWallet
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("iniciar transacao: %w", err)
	}
	defer tx.Rollback()

	var fromBalance int64
	err = tx.QueryRow(
		`SELECT balance FROM wallets WHERE user_id = ?`,
		fromUserID,
	).Scan(&fromBalance)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("origem: %w", ErrWalletNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consultar origem: %w", err)
	}

	var toExists int
	err = tx.QueryRow(
		`SELECT 1 FROM wallets WHERE user_id = ?`,
		toUserID,
	).Scan(&toExists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("destino: %w", ErrWalletNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("consultar destino: %w", err)
	}

	if fromBalance < amount {
		return nil, ErrInsufficientFunds
	}

	res, err := tx.Exec(
		`UPDATE wallets SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ? AND balance >= ?`,
		amount, fromUserID, amount,
	)
	if err != nil {
		return nil, fmt.Errorf("debitar origem: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, ErrInsufficientFunds
	}

	res, err = tx.Exec(
		`UPDATE wallets SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`,
		amount, toUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("creditar destino: %w", err)
	}
	affected, err = res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, fmt.Errorf("destino: %w", ErrWalletNotFound)
	}

	now := time.Now().UTC()
	ins, err := tx.Exec(
		`INSERT INTO transfers (from_user_id, to_user_id, amount, created_at) VALUES (?, ?, ?, ?)`,
		fromUserID, toUserID, amount, now,
	)
	if err != nil {
		return nil, fmt.Errorf("registrar transferencia: %w", err)
	}
	id, err := ins.LastInsertId()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &TransferResult{
		ID:         id,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Amount:     amount,
		CreatedAt:  now,
	}, nil
}
