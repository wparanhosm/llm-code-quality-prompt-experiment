package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"pix-p2p/internal/db"
	"pix-p2p/internal/wallet"
)

func main() {
	from := flag.Int64("from", 0, "ID do usuario de origem")
	to := flag.Int64("to", 0, "ID do usuario de destino")
	amount := flag.Int64("amount", 0, "Valor em centavos")
	dbPath := flag.String("db", "pix.db", "Caminho do SQLite")
	flag.Parse()

	if *from == 0 || *to == 0 || *amount == 0 {
		fmt.Fprintf(os.Stderr, "uso: pix -from=1 -to=2 -amount=1500\n")
		os.Exit(2)
	}

	conn, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := db.Seed(conn); err != nil {
		log.Fatal(err)
	}

	svc := wallet.NewService(conn)
	result, err := svc.TransferPixP2P(*from, *to, *amount)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Pix P2P concluido id=%d de=%d para=%d valor=%d centavos em=%s\n",
		result.ID, result.FromUserID, result.ToUserID, result.Amount, result.CreatedAt.Format("2006-01-02 15:04:05"))
}
