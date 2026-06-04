package database

import (
	"github.com/wparanhosm/pix-p2p/domain"
)

type WalletRepository struct {
	db DBTX
}

func NewWalletRepository(db DBTX) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) FindByUserID(userID string) (*domain.Wallet, error) {
	row := r.db.QueryRow("SELECT id, user_id, balance, updated_at FROM wallets WHERE user_id = ?", userID)

	var w domain.Wallet
	if err := row.Scan(&w.ID, &w.UserID, &w.Balance, &w.UpdatedAt); err != nil {
		return nil, domain.ErrWalletNotFound
	}
	return &w, nil
}

func (r *WalletRepository) Update(w *domain.Wallet) error {
	_, err := r.db.Exec("UPDATE wallets SET balance = ?, updated_at = ? WHERE id = ?", w.Balance, w.UpdatedAt, w.ID)
	return err
}

func (r *WalletRepository) CreateTransaction(t *domain.Transaction) error {
	_, err := r.db.Exec(
		"INSERT INTO transactions (from_user_id, to_user_id, amount, status, created_at) VALUES (?, ?, ?, ?, ?)",
		t.FromUserID, t.ToUserID, t.Amount, t.Status, t.CreatedAt,
	)
	return err
}
