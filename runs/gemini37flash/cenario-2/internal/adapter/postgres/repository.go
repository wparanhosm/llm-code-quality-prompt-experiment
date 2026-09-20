package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/walter/p2p-wallet-transfer/internal/domain"
	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

// AccountRepository implements domain.AccountRepository using standard database/sql.
type AccountRepository struct {
	db DBExecutor
}

// DBExecutor represents common database querying interface satisfied by *sql.DB and *sql.Tx.
type DBExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// NewAccountRepository returns a new instance of AccountRepository.
func NewAccountRepository(db DBExecutor) *AccountRepository {
	return &AccountRepository{db: db}
}

// GetByID retrieves an account by its unique ID.
func (r *AccountRepository) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	query := `
		SELECT id, holder, balance_cents, status, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`
	var acc domain.Account
	var balanceCents int64
	var status string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&acc.ID,
		&acc.Holder,
		&balanceCents,
		&status,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed querying account %d: %w", id, err)
	}

	bal, err := money.FromCentsUnsafe(balanceCents)
	if err != nil {
		return nil, fmt.Errorf("invalid balance data for account %d: %w", id, err)
	}

	acc.Balance = bal
	acc.Status = domain.AccountStatus(status)
	return &acc, nil
}

// GetForUpdate retrieves an account locking the row using SELECT FOR UPDATE.
func (r *AccountRepository) GetForUpdate(ctx context.Context, id int64) (*domain.Account, error) {
	query := `
		SELECT id, holder, balance_cents, status, created_at, updated_at
		FROM accounts
		WHERE id = $1
		FOR UPDATE
	`
	var acc domain.Account
	var balanceCents int64
	var status string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&acc.ID,
		&acc.Holder,
		&balanceCents,
		&status,
		&acc.CreatedAt,
		&acc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed querying account %d with lock: %w", id, err)
	}

	bal, err := money.FromCentsUnsafe(balanceCents)
	if err != nil {
		return nil, fmt.Errorf("invalid balance data for account %d: %w", id, err)
	}

	acc.Balance = bal
	acc.Status = domain.AccountStatus(status)
	return &acc, nil
}

// UpdateBalance updates the balance and updated_at timestamp of an account.
func (r *AccountRepository) UpdateBalance(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE accounts
		SET balance_cents = $1, updated_at = $2
		WHERE id = $3
	`
	res, err := r.db.ExecContext(ctx, query, account.Balance.Cents(), account.UpdatedAt, account.ID)
	if err != nil {
		return fmt.Errorf("failed updating balance for account %d: %w", account.ID, err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed getting rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrAccountNotFound
	}

	return nil
}

// Create inserts a new account record into the database.
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (id, holder, balance_cents, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(
		ctx,
		query,
		account.ID,
		account.Holder,
		account.Balance.Cents(),
		string(account.Status),
		account.CreatedAt,
		account.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed inserting account %d: %w", account.ID, err)
	}
	return nil
}

// TransferRepository implements domain.TransferRepository using standard database/sql.
type TransferRepository struct {
	db DBExecutor
}

// NewTransferRepository returns a new instance of TransferRepository.
func NewTransferRepository(db DBExecutor) *TransferRepository {
	return &TransferRepository{db: db}
}

// Create inserts a new transfer record and populates its auto-generated ID.
func (r *TransferRepository) Create(ctx context.Context, transfer *domain.Transfer) error {
	query := `
		INSERT INTO transfers (from_account_id, to_account_id, amount_cents, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := r.db.QueryRowContext(
		ctx,
		query,
		transfer.FromAccountID,
		transfer.ToAccountID,
		transfer.Amount.Cents(),
		string(transfer.Status),
		transfer.CreatedAt,
	).Scan(&transfer.ID)

	if err != nil {
		return fmt.Errorf("failed inserting transfer: %w", err)
	}
	return nil
}

// GetByID retrieves a transfer record by ID.
func (r *TransferRepository) GetByID(ctx context.Context, id int64) (*domain.Transfer, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount_cents, status, created_at
		FROM transfers
		WHERE id = $1
	`
	var tr domain.Transfer
	var amountCents int64
	var status string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tr.ID,
		&tr.FromAccountID,
		&tr.ToAccountID,
		&amountCents,
		&status,
		&tr.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTransferNotFound
		}
		return nil, fmt.Errorf("failed querying transfer %d: %w", id, err)
	}

	amt, err := money.New(amountCents)
	if err != nil {
		return nil, fmt.Errorf("invalid amount data for transfer %d: %w", id, err)
	}

	tr.Amount = amt
	tr.Status = domain.TransferStatus(status)
	return &tr, nil
}

// UnitOfWorkRepository implements domain.UnitOfWorkRepository scoped to a *sql.Tx.
type UnitOfWorkRepository struct {
	accountRepo  *AccountRepository
	transferRepo *TransferRepository
}

// Accounts returns the transaction-scoped AccountRepository.
func (u *UnitOfWorkRepository) Accounts() domain.AccountRepository {
	return u.accountRepo
}

// Transfers returns the transaction-scoped TransferRepository.
func (u *UnitOfWorkRepository) Transfers() domain.TransferRepository {
	return u.transferRepo
}

// UnitOfWork implements domain.UnitOfWork managing transactions via *sql.DB.
type UnitOfWork struct {
	db *sql.DB
}

// NewUnitOfWork creates a new UnitOfWork for PostgreSQL.
func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

// Execute wraps transactional logic with automatic commit or rollback.
func (u *UnitOfWork) Execute(ctx context.Context, fn func(txRepo domain.UnitOfWorkRepository) error) error {
	tx, err := u.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("failed initiating database transaction: %w", err)
	}

	txRepo := &UnitOfWorkRepository{
		accountRepo:  NewAccountRepository(tx),
		transferRepo: NewTransferRepository(tx),
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(txRepo); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			return fmt.Errorf("error during transaction: %w (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed committing database transaction: %w", err)
	}

	return nil
}
