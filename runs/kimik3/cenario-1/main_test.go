package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := openDB(path)
	if err != nil {
		t.Fatalf("falha ao abrir banco de teste: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mustCreateWallet(t *testing.T, store *Store, id string, cents int64) {
	t.Helper()
	if err := store.CreateWallet(context.Background(), id, cents); err != nil {
		t.Fatalf("falha ao criar carteira %s: %v", id, err)
	}
}

func mustBalance(t *testing.T, store *Store, id string) int64 {
	t.Helper()
	w, err := store.GetWallet(context.Background(), id)
	if err != nil {
		t.Fatalf("falha ao buscar carteira %s: %v", id, err)
	}
	return w.BalanceCents
}

func TestTransferPix_Success(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	service := NewService(db)
	ctx := context.Background()

	mustCreateWallet(t, store, "alice", 100000) // R$ 1000,00
	mustCreateWallet(t, store, "bob", 5000)     // R$ 50,00

	txn, err := service.TransferPix(ctx, "alice", "bob", 15075) // R$ 150,75
	if err != nil {
		t.Fatalf("transferência deveria ter sucesso: %v", err)
	}
	if txn.ID == 0 {
		t.Error("transação deveria ter ID gerado")
	}

	if got := mustBalance(t, store, "alice"); got != 100000-15075 {
		t.Errorf("saldo alice = %d, esperado %d", got, 100000-15075)
	}
	if got := mustBalance(t, store, "bob"); got != 5000+15075 {
		t.Errorf("saldo bob = %d, esperado %d", got, 5000+15075)
	}

	txs, err := store.ListTransactions(ctx)
	if err != nil || len(txs) != 1 {
		t.Fatalf("histórico deveria ter 1 transação, err=%v len=%d", err, len(txs))
	}
}

func TestTransferPix_InsufficientFunds(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	service := NewService(db)
	ctx := context.Background()

	mustCreateWallet(t, store, "alice", 1000)
	mustCreateWallet(t, store, "bob", 0)

	_, err := service.TransferPix(ctx, "alice", "bob", 1001)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("erro = %v, esperado ErrInsufficientFunds", err)
	}

	// Nenhum saldo pode ter sido alterado (atomicidade).
	if got := mustBalance(t, store, "alice"); got != 1000 {
		t.Errorf("saldo alice = %d, esperado 1000 (rollback não ocorreu)", got)
	}
	if got := mustBalance(t, store, "bob"); got != 0 {
		t.Errorf("saldo bob = %d, esperado 0", got)
	}
}

func TestTransferPix_Errors(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	service := NewService(db)
	ctx := context.Background()

	mustCreateWallet(t, store, "alice", 1000)
	mustCreateWallet(t, store, "bob", 0)

	cases := []struct {
		name    string
		from    string
		to      string
		amount  int64
		wantErr error
	}{
		{"mesma carteira", "alice", "alice", 100, ErrSameWallet},
		{"valor zero", "alice", "bob", 0, ErrInvalidAmount},
		{"valor negativo", "alice", "bob", -50, ErrInvalidAmount},
		{"origem inexistente", "ninguem", "bob", 100, ErrWalletNotFound},
		{"destino inexistente", "alice", "ninguem", 100, ErrWalletNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.TransferPix(ctx, tc.from, tc.to, tc.amount)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("erro = %v, esperado %v", err, tc.wantErr)
			}
		})
	}
}

// TestTransferPix_Concurrency garante que transferências concorrentes são
// atômicas: com saldo para apenas 5 de 10 transferências, exatamente 5 devem
// ter sucesso e o dinheiro total deve se conservar.
func TestTransferPix_Concurrency(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	service := NewService(db)
	ctx := context.Background()

	mustCreateWallet(t, store, "alice", 100) // saldo para 5 transferências de 20
	mustCreateWallet(t, store, "bob", 0)

	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.TransferPix(ctx, "alice", "bob", 20); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			} else if !errors.Is(err, ErrInsufficientFunds) {
				t.Errorf("erro inesperado: %v", err)
			}
		}()
	}
	wg.Wait()

	if succeeded != 5 {
		t.Errorf("sucessos = %d, esperado 5", succeeded)
	}
	alice := mustBalance(t, store, "alice")
	bob := mustBalance(t, store, "bob")
	if alice != 0 {
		t.Errorf("saldo alice = %d, esperado 0", alice)
	}
	if alice+bob != 100 {
		t.Errorf("dinheiro não se conservou: alice=%d bob=%d", alice, bob)
	}
}

func TestHTTP_TransferEndpoint(t *testing.T) {
	db := newTestDB(t)
	store := NewStore(db)
	srv := &Server{store: store, service: NewService(db)}

	mustCreateWallet(t, store, "alice", 100000)
	mustCreateWallet(t, store, "bob", 0)

	server := httptest.NewServer(srv.routes())
	defer server.Close()

	// Transferência com sucesso.
	body, _ := json.Marshal(map[string]any{"origem": "alice", "destino": "bob", "valor": 250.50})
	resp, err := http.Post(server.URL+"/pix/transfer", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", resp.StatusCode)
	}

	// Saldo insuficiente -> 422.
	body, _ = json.Marshal(map[string]any{"origem": "bob", "destino": "alice", "valor": 99999.0})
	resp2, err := http.Post(server.URL+"/pix/transfer", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, esperado 422", resp2.StatusCode)
	}

	// Consulta saldo atualizado.
	resp3, err := http.Get(server.URL + "/wallets/alice")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var w walletResponse
	if err := json.NewDecoder(resp3.Body).Decode(&w); err != nil {
		t.Fatal(err)
	}
	if w.Saldo != 749.50 {
		t.Errorf("saldo alice = %.2f, esperado 749.50", w.Saldo)
	}
}
