package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	_ "modernc.org/sqlite"
)

// schema cria as tabelas de carteiras e transações (idempotente).
const schema = `
CREATE TABLE IF NOT EXISTS wallets (
    id            TEXT PRIMARY KEY,
    balance_cents INTEGER NOT NULL CHECK (balance_cents >= 0),
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS transactions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    from_wallet_id TEXT NOT NULL REFERENCES wallets(id),
    to_wallet_id   TEXT NOT NULL REFERENCES wallets(id),
    amount_cents   INTEGER NOT NULL CHECK (amount_cents > 0),
    status         TEXT NOT NULL DEFAULT 'completed',
    created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// openDB abre o banco SQLite e aplica o schema.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// SQLite aceita apenas um escritor por vez: uma única conexão serializa
	// o acesso e evita erros de SQLITE_BUSY sob concorrência.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// Server agrupa as dependências dos handlers HTTP.
type Server struct {
	store   *Store
	service *Service
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /wallets", s.handleCreateWallet)
	mux.HandleFunc("GET /wallets/{id}", s.handleGetWallet)
	mux.HandleFunc("POST /pix/transfer", s.handleTransfer)
	mux.HandleFunc("GET /transactions", s.handleListTransactions)
	return mux
}

// ---- DTOs ----

type createWalletRequest struct {
	ID           string  `json:"id"`
	SaldoInicial float64 `json:"saldo_inicial"`
}

type walletResponse struct {
	ID       string  `json:"id"`
	Saldo    float64 `json:"saldo"`
	CriadoEm string  `json:"criado_em"`
}

type transferRequest struct {
	Origem  string  `json:"origem"`
	Destino string  `json:"destino"`
	Valor   float64 `json:"valor"`
}

type transactionResponse struct {
	ID       int64   `json:"id"`
	Origem   string  `json:"origem"`
	Destino  string  `json:"destino"`
	Valor    float64 `json:"valor"`
	Status   string  `json:"status"`
	CriadoEm string  `json:"criado_em,omitempty"`
}

// ---- Handlers ----

func (s *Server) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	var req createWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "campo 'id' é obrigatório")
		return
	}
	if req.SaldoInicial < 0 {
		writeError(w, http.StatusBadRequest, "saldo inicial não pode ser negativo")
		return
	}

	if err := s.store.CreateWallet(r.Context(), req.ID, toCents(req.SaldoInicial)); err != nil {
		if errors.Is(err, ErrWalletExists) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}

	wallet, err := s.store.GetWallet(r.Context(), req.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}
	writeJSON(w, http.StatusCreated, walletResponse{
		ID:       wallet.ID,
		Saldo:    toReais(wallet.BalanceCents),
		CriadoEm: wallet.CreatedAt,
	})
}

func (s *Server) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wallet, err := s.store.GetWallet(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}
	writeJSON(w, http.StatusOK, walletResponse{
		ID:       wallet.ID,
		Saldo:    toReais(wallet.BalanceCents),
		CriadoEm: wallet.CreatedAt,
	})
}

// handleTransfer é o endpoint da transferência Pix P2P.
// Recebe {"origem": "...", "destino": "...", "valor": 150.75}.
func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	var req transferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if strings.TrimSpace(req.Origem) == "" || strings.TrimSpace(req.Destino) == "" {
		writeError(w, http.StatusBadRequest, "campos 'origem' e 'destino' são obrigatórios")
		return
	}

	txn, err := s.service.TransferPix(r.Context(), req.Origem, req.Destino, toCents(req.Valor))
	if err != nil {
		switch {
		case errors.Is(err, ErrInsufficientFunds):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, ErrWalletNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, ErrSameWallet), errors.Is(err, ErrInvalidAmount):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			log.Printf("erro na transferência: %v", err)
			writeError(w, http.StatusInternalServerError, "erro interno")
		}
		return
	}

	writeJSON(w, http.StatusOK, transactionResponse{
		ID:      txn.ID,
		Origem:  txn.FromWalletID,
		Destino: txn.ToWalletID,
		Valor:   toReais(txn.AmountCents),
		Status:  txn.Status,
	})
}

func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	txs, err := s.store.ListTransactions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro interno")
		return
	}
	resp := make([]transactionResponse, 0, len(txs))
	for _, t := range txs {
		resp = append(resp, transactionResponse{
			ID:       t.ID,
			Origem:   t.FromWalletID,
			Destino:  t.ToWalletID,
			Valor:    toReais(t.AmountCents),
			Status:   t.Status,
			CriadoEm: t.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// ---- Helpers HTTP ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"erro": msg})
}

func main() {
	db, err := openDB("pix.db")
	if err != nil {
		log.Fatalf("falha ao abrir banco de dados: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	service := NewService(db)
	srv := &Server{store: store, service: service}

	// Carteiras de exemplo para facilitar o teste manual (ignora se já existirem).
	ctx := context.Background()
	if err := store.CreateWallet(ctx, "alice", toCents(1000)); err == nil {
		log.Println("carteira 'alice' criada com saldo R$ 1000,00")
	}
	if err := store.CreateWallet(ctx, "bob", toCents(50)); err == nil {
		log.Println("carteira 'bob' criada com saldo R$ 50,00")
	}

	addr := ":8080"
	log.Printf("Servidor Pix P2P ouvindo em %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv.routes()))
}
