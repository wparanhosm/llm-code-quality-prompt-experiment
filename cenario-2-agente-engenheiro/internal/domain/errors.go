package domain

import "errors"

var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrInvalidAmount     = errors.New("amount must be greater than zero")
	ErrSameWallet        = errors.New("source and destination wallets must differ")
	ErrWalletNotFound    = errors.New("wallet not found")
)
