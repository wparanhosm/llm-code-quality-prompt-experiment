package domain

import "context"

type WalletRepository interface {
	FindByIDForUpdate(ctx context.Context, id string) (*Wallet, error)
	UpdateBalance(ctx context.Context, wallet *Wallet) error
}

type TransactionRepository interface {
	Create(ctx context.Context, tx *Transaction) error
}

type TxManager interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type IDGenerator interface {
	NewID() string
}
