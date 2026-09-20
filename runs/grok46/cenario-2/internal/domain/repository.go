package domain

import "context"

// UnitOfWork coordinates an atomic P2P transfer (debit, credit, history).
type UnitOfWork interface {
	ExecuteTransfer(ctx context.Context, fromID, toID string, amount Money) (*Transfer, error)
}
