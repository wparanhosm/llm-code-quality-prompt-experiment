package wallet

import (
	"testing"

	"pix-p2p/internal/db"
)

func TestTransferPixP2P(t *testing.T) {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Seed(conn); err != nil {
		t.Fatal(err)
	}

	svc := NewService(conn)
	res, err := svc.TransferPixP2P(1, 2, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if res.Amount != 1000 {
		t.Fatalf("amount=%d", res.Amount)
	}

	var fromBal, toBal int64
	if err := conn.QueryRow(`SELECT balance FROM wallets WHERE user_id=1`).Scan(&fromBal); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(`SELECT balance FROM wallets WHERE user_id=2`).Scan(&toBal); err != nil {
		t.Fatal(err)
	}
	if fromBal != 99000 || toBal != 26000 {
		t.Fatalf("saldos from=%d to=%d", fromBal, toBal)
	}

	_, err = svc.TransferPixP2P(1, 2, 999999999)
	if err != ErrInsufficientFunds {
		t.Fatalf("esperado saldo insuficiente, got %v", err)
	}
}
