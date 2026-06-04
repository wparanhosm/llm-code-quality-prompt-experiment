package main

import (
	"fmt"
	"log"

	"github.com/wparanhosm/pix-p2p/infra/database"
	"github.com/wparanhosm/pix-p2p/service"
)

func main() {
	db, err := database.NewConnection("pix.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Exec("INSERT OR IGNORE INTO wallets (user_id, balance) VALUES (?, ?)", "user-001", 1000.00)
	db.Exec("INSERT OR IGNORE INTO wallets (user_id, balance) VALUES (?, ?)", "user-002", 500.00)

	svc := service.NewTransferService(db)

	err = svc.Transfer("user-001", "user-002", 250.00)
	if err != nil {
		log.Fatalf("Erro na transferência: %v", err)
	}

	fmt.Println("Transferência Pix realizada com sucesso!")

	var b1, b2 float64
	db.QueryRow("SELECT balance FROM wallets WHERE user_id = ?", "user-001").Scan(&b1)
	db.QueryRow("SELECT balance FROM wallets WHERE user_id = ?", "user-002").Scan(&b2)
	fmt.Printf("Saldo user-001: R$ %.2f\n", b1)
	fmt.Printf("Saldo user-002: R$ %.2f\n", b2)
}
