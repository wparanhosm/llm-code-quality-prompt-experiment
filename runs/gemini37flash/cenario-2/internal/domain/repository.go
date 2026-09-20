package domain

import (
	"context"
)

// AccountRepository defines the boundary for account persistence operations.
type AccountRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	// GetForUpdate retrieves an account with pessimistic locking (SELECT FOR UPDATE)
	GetForUpdate(ctx context.Context, id int64) (*Account, error)
	UpdateBalance(ctx context.Context, account *Account) error
	Create(ctx context.Context, account *Account) error
}

// TransferRepository defines the boundary for transfer persistence operations.
type TransferRepository interface {
	Create(ctx context.Context, transfer *Transfer) error
	GetByID(ctx context.Context, id int64) (*Transfer, error)
}

// UnitOfWork defines the atomic transaction boundary orchestrating repositories.
type UnitOfWork interface {
	// Execute runs operations within a single database transaction.
	Execute(ctx context.Context, fn func(txRepo UnitOfWorkRepository) error) error
}

// UnitOfWorkRepository provides repository access scoped within an active transaction.
type UnitOfWorkRepository interface {
	Accounts() AccountRepository
	Transfers() TransferRepository
}
