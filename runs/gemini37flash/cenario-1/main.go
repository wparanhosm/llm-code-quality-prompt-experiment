package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrSenderNotFound    = errors.New("conta de origem não encontrada")
	ErrReceiverNotFound  = errors.New("conta de destino não encontrada")
	ErrInsufficientFunds = errors.New("saldo insuficiente")
	ErrInvalidAmount     = errors.New("o valor da transferência deve ser maior que zero")
	ErrSameAccount       = errors.New("a conta de origem e destino não podem ser iguais")
)

type WalletService struct {
	db *sql.DB
}

func NewWalletService(db *sql.DB) *WalletService {
	return &WalletService{db: db}
}

func (s *WalletService) TransferPix(ctx context.Context, senderID, receiverID string, amountCents int64) error {
	if amountCents <= 0 {
		return ErrInvalidAmount
	}
	if senderID == receiverID {
		return ErrSameAccount
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("falha ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	// 1. Obter e travar saldo da conta de origem
	var senderBalance int64
	err = tx.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE user_id = ?", senderID).Scan(&senderBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSenderNotFound
	}
	if err != nil {
		return fmt.Errorf("erro ao consultar saldo do remetente: %w", err)
	}

	if senderBalance < amountCents {
		return ErrInsufficientFunds
	}

	// 2. Verificar existência da conta de destino
	var receiverBalance int64
	err = tx.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE user_id = ?", receiverID).Scan(&receiverBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrReceiverNotFound
	}
	if err != nil {
		return fmt.Errorf("erro ao consultar saldo do destinatário: %w", err)
	}

	// 3. Debitar conta de origem
	_, err = tx.ExecContext(ctx, "UPDATE wallets SET balance = balance - ? WHERE user_id = ?", amountCents, senderID)
	if err != nil {
		return fmt.Errorf("erro ao debitar remetente: %w", err)
	}

	// 4. Creditar conta de destino
	_, err = tx.ExecContext(ctx, "UPDATE wallets SET balance = balance + ? WHERE user_id = ?", amountCents, receiverID)
	if err != nil {
		return fmt.Errorf("erro ao creditar destinatário: %w", err)
	}

	// 5. Registrar histórico de transação Pix
	_, err = tx.ExecContext(ctx,
		`INSERT INTO pix_transactions (sender_id, receiver_id, amount, created_at) VALUES (?, ?, ?, ?)`,
		senderID, receiverID, amountCents, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("erro ao registrar histórico pix: %w", err)
	}

	return tx.Commit()
}

func (s *WalletService) GetBalance(ctx context.Context, userID string) (int64, error) {
	var balance int64
	err := s.db.QueryRowContext(ctx, "SELECT balance FROM wallets WHERE user_id = ?", userID).Scan(&balance)
	return balance, err
}

func initSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS wallets (
		user_id TEXT PRIMARY KEY,
		balance INTEGER NOT NULL CHECK (balance >= 0)
	);

	CREATE TABLE IF NOT EXISTS pix_transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender_id TEXT NOT NULL,
		receiver_id TEXT NOT NULL,
		amount INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (sender_id) REFERENCES wallets(user_id),
		FOREIGN KEY (receiver_id) REFERENCES wallets(user_id)
	);`
	_, err := db.Exec(query)
	return err
}

func formatCurrency(cents int64) string {
	return fmt.Sprintf("R$ %.2f", float64(cents)/100.0)
}

func main() {
	db, err := sql.Open("sqlite", "pix_wallets.db")
	if err != nil {
		log.Fatalf("Erro ao abrir banco de dados: %v", err)
	}
	defer db.Close()

	if err := initSchema(db); err != nil {
		log.Fatalf("Erro ao criar tabelas: %v", err)
	}

	service := NewWalletService(db)
	ctx := context.Background()

	// Seed inicial de carteiras
	_, _ = db.Exec(`INSERT OR REPLACE INTO wallets (user_id, balance) VALUES 
		('user_alice', 15000), -- R$ 150.00
		('user_bob', 5000);    -- R$ 50.00
	`)

	fmt.Println("--- Estado Inicial ---")
	balAlice, _ := service.GetBalance(ctx, "user_alice")
	balBob, _ := service.GetBalance(ctx, "user_bob")
	fmt.Printf("Alice: %s | Bob: %s\n", formatCurrency(balAlice), formatCurrency(balBob))

	transferValue := int64(4500) // R$ 45.00
	fmt.Printf("\nExecutando Pix de Alice para Bob no valor de %s...\n", formatCurrency(transferValue))

	err = service.TransferPix(ctx, "user_alice", "user_bob", transferValue)
	if err != nil {
		log.Fatalf("Falha na transferência Pix: %v", err)
	}
	fmt.Println("Transferência realizada com sucesso!")

	fmt.Println("\n--- Estado Final ---")
	balAlice, _ = service.GetBalance(ctx, "user_alice")
	balBob, _ = service.GetBalance(ctx, "user_bob")
	fmt.Printf("Alice: %s | Bob: %s\n", formatCurrency(balAlice), formatCurrency(balBob))
}
