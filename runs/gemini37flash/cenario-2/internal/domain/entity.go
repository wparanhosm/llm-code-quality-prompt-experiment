package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/walter/p2p-wallet-transfer/pkg/money"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrSameAccountTransfer = errors.New("cannot transfer money to the same account")
	ErrInvalidAmount       = errors.New("transfer amount must be strictly greater than zero")
	ErrAccountInactive     = errors.New("account is inactive")
	ErrInvalidAccountID    = errors.New("invalid account ID")
	ErrTransactionFailed   = errors.New("transaction failed")
	ErrTransferNotFound    = errors.New("transfer not found")
)

// AccountStatus defines the active state of an account.
type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "ACTIVE"
	AccountStatusInactive AccountStatus = "INACTIVE"
	AccountStatusBlocked  AccountStatus = "BLOCKED"
)

// Account is the core domain entity representing a digital wallet.
type Account struct {
	ID        int64         `json:"id"`
	Holder    string        `json:"holder"`
	Balance   money.Money   `json:"balance"`
	Status    AccountStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// NewAccount instantiates an Account entity validating domain invariants.
func NewAccount(id int64, holder string, initialBalance money.Money, status AccountStatus) (*Account, error) {
	if id <= 0 {
		return nil, ErrInvalidAccountID
	}
	if holder == "" {
		return nil, errors.New("holder name cannot be empty")
	}
	if status == "" {
		status = AccountStatusActive
	}

	now := time.Now().UTC()
	return &Account{
		ID:        id,
		Holder:    holder,
		Balance:   initialBalance,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Debit subtracts an amount from the account balance.
func (a *Account) Debit(amount money.Money) error {
	if a.Status != AccountStatusActive {
		return fmt.Errorf("%w: cannot debit inactive account %d", ErrAccountInactive, a.ID)
	}
	if amount.Cents() <= 0 {
		return ErrInvalidAmount
	}

	newBalance, err := a.Balance.Sub(amount)
	if err != nil {
		return fmt.Errorf("%w: account %d balance %d is less than required %d", ErrInsufficientBalance, a.ID, a.Balance.Cents(), amount.Cents())
	}

	a.Balance = newBalance
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// Credit adds an amount to the account balance.
func (a *Account) Credit(amount money.Money) error {
	if a.Status != AccountStatusActive {
		return fmt.Errorf("%w: cannot credit inactive account %d", ErrAccountInactive, a.ID)
	}
	if amount.Cents() <= 0 {
		return ErrInvalidAmount
	}

	a.Balance = a.Balance.Add(amount)
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// TransferStatus represents the state of a P2P transfer.
type TransferStatus string

const (
	TransferStatusCompleted TransferStatus = "COMPLETED"
	TransferStatusFailed    TransferStatus = "FAILED"
)

// Transfer represents a completed or recorded P2P transfer between two accounts.
type Transfer struct {
	ID            int64          `json:"id"`
	FromAccountID int64          `json:"from_account_id"`
	ToAccountID   int64          `json:"to_account_id"`
	Amount        money.Money    `json:"amount"`
	Status        TransferStatus `json:"status"`
	CreatedAt     time.Time      `json:"created_at"`
}

// NewTransfer creates a new Transfer entity validating invariant constraints.
func NewTransfer(fromAccountID, toAccountID int64, amount money.Money) (*Transfer, error) {
	if fromAccountID <= 0 || toAccountID <= 0 {
		return nil, ErrInvalidAccountID
	}
	if fromAccountID == toAccountID {
		return nil, ErrSameAccountTransfer
	}
	if amount.Cents() <= 0 {
		return nil, ErrInvalidAmount
	}

	return &Transfer{
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        amount,
		Status:        TransferStatusCompleted,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
