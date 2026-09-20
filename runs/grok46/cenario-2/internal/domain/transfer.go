package domain

import (
	"time"
)

// Transfer is the immutable record of a completed P2P movement.
type Transfer struct {
	ID            string
	FromWalletID  string
	ToWalletID    string
	Amount        Money
	CreatedAt     time.Time
}

func NewTransfer(id, fromID, toID string, amount Money, createdAt time.Time) (*Transfer, error) {
	if id == "" {
		return nil, ErrInvalidWalletID
	}
	if fromID == "" || toID == "" {
		return nil, ErrInvalidWalletID
	}
	if fromID == toID {
		return nil, ErrSameWalletTransfer
	}
	if err := amount.ValidatePositive(); err != nil {
		return nil, err
	}
	return &Transfer{
		ID:           id,
		FromWalletID: fromID,
		ToWalletID:   toID,
		Amount:       amount,
		CreatedAt:    createdAt,
	}, nil
}
