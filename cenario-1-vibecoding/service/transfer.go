package service

import (
	"database/sql"
	"time"

	"github.com/wparanhosm/pix-p2p/domain"
	"github.com/wparanhosm/pix-p2p/infra/database"
)

type TransferService struct {
	db *sql.DB
}

func NewTransferService(db *sql.DB) *TransferService {
	return &TransferService{db: db}
}

func (s *TransferService) Transfer(fromUserID, toUserID string, amount float64) error {
	if fromUserID == toUserID {
		return domain.ErrSameUser
	}
	if amount <= 0 {
		return domain.ErrInvalidAmount
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	repo := database.NewWalletRepository(tx)

	fromWallet, err := repo.FindByUserID(fromUserID)
	if err != nil {
		return err
	}

	toWallet, err := repo.FindByUserID(toUserID)
	if err != nil {
		return err
	}

	if err := fromWallet.Debit(amount); err != nil {
		return err
	}

	if err := toWallet.Credit(amount); err != nil {
		return err
	}

	if err := repo.Update(fromWallet); err != nil {
		return err
	}

	if err := repo.Update(toWallet); err != nil {
		return err
	}

	if err := repo.CreateTransaction(&domain.Transaction{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Amount:     amount,
		Status:     "COMPLETED",
		CreatedAt:  time.Now(),
	}); err != nil {
		return err
	}

	return tx.Commit()
}
