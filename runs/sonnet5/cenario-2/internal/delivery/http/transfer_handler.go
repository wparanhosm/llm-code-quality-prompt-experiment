// Package http implementa a camada de entrega (delivery/controller) HTTP
// da aplicação usando exclusivamente o pacote nativo net/http do Go, sem
// nenhum framework externo (ex: gin, echo, chi). Esta camada apenas
// traduz requisições HTTP em chamadas ao usecase e formata as respostas;
// nenhuma regra de negócio reside aqui.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"p2p-transfer/internal/domain"
	"p2p-transfer/internal/usecase"
)

// maxRequestBodyBytes limita o tamanho do payload aceito para mitigar
// ataques de negação de serviço (DoS) via payloads excessivamente
// grandes, seguindo práticas defensivas recomendadas pela OWASP.
const maxRequestBodyBytes = 1 << 16 // 64 KiB

// transferExecutor é a porta que o handler HTTP depende (o usecase real
// implementa esta interface), permitindo testar o handler isoladamente
// com um double simples, sem qualquer dependência de infraestrutura.
type transferExecutor interface {
	Execute(ctx context.Context, in usecase.TransferInput) (*usecase.TransferOutput, error)
}

// transferRequest é o payload JSON aceito pelo endpoint de transferência.
type transferRequest struct {
	FromWalletID string `json:"from_wallet_id"`
	ToWalletID   string `json:"to_wallet_id"`
	AmountCents  int64  `json:"amount_cents"`
}

// transferResponse é o payload JSON de sucesso retornado pelo endpoint de
// transferência.
type transferResponse struct {
	TransactionID  string `json:"transaction_id"`
	FromWalletID   string `json:"from_wallet_id"`
	ToWalletID     string `json:"to_wallet_id"`
	AmountCents    int64  `json:"amount_cents"`
	FromNewBalance int64  `json:"from_new_balance"`
	ToNewBalance   int64  `json:"to_new_balance"`
}

// errorResponse é o payload JSON padronizado retornado em caso de erro,
// evitando vazar detalhes internos de implementação (stack traces, erros
// de driver SQL) para o cliente.
type errorResponse struct {
	Error string `json:"error"`
}

// TransferHandler expõe o endpoint HTTP de transferência P2P entre
// carteiras digitais.
type TransferHandler struct {
	usecase transferExecutor
	logger  *slog.Logger
}

// NewTransferHandler constrói o handler HTTP, recebendo o usecase e o
// logger via injeção de dependência (nenhuma instanciação direta de
// dependências dentro do handler).
func NewTransferHandler(uc transferExecutor, logger *slog.Logger) *TransferHandler {
	if uc == nil {
		panic("http: transferExecutor não pode ser nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &TransferHandler{usecase: uc, logger: logger}
}

// ServeHTTP implementa http.Handler, processando requisições POST para
// realizar uma transferência P2P entre duas carteiras digitais.
//
// Contrato:
//   - Método: POST
//   - Content-Type: application/json
//   - Body: {"from_wallet_id": string, "to_wallet_id": string, "amount_cents": int64}
//
// Respostas:
//   - 200 OK: transferência concluída, corpo contém transferResponse.
//   - 400 Bad Request: payload inválido ou violação de regra de negócio
//     (valor inválido, carteiras iguais, saldo insuficiente, carteira
//     inativa).
//   - 404 Not Found: carteira de origem ou destino não encontrada.
//   - 405 Method Not Allowed: método HTTP diferente de POST.
//   - 500 Internal Server Error: falha inesperada de infraestrutura.
func (h *TransferHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "método não permitido, utilize POST")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	defer r.Body.Close()

	var req transferRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		h.logger.WarnContext(r.Context(), "payload de transferência inválido", slog.Any("error", err))
		h.writeError(w, http.StatusBadRequest, "payload JSON inválido: "+sanitizeDecodeError(err))
		return
	}

	// Garante que não há conteúdo extra após o objeto JSON principal.
	if dec.More() {
		h.writeError(w, http.StatusBadRequest, "payload JSON inválido: conteúdo inesperado após o objeto principal")
		return
	}

	out, err := h.usecase.Execute(r.Context(), usecase.TransferInput{
		FromWalletID: req.FromWalletID,
		ToWalletID:   req.ToWalletID,
		AmountCents:  req.AmountCents,
	})
	if err != nil {
		h.handleUsecaseError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusOK, transferResponse{
		TransactionID:  out.TransactionID,
		FromWalletID:   out.FromWalletID,
		ToWalletID:     out.ToWalletID,
		AmountCents:    out.AmountCents,
		FromNewBalance: out.FromNewBalance,
		ToNewBalance:   out.ToNewBalance,
	})
}

// handleUsecaseError mapeia erros de domínio/negócio para códigos HTTP
// apropriados, sem vazar detalhes internos sensíveis (ex: erros de driver
// SQL) na resposta ao cliente. O erro completo é sempre logado
// internamente para rastreabilidade.
func (h *TransferHandler) handleUsecaseError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "falha ao processar transferência", slog.Any("error", err))

	switch {
	case errors.Is(err, domain.ErrWalletNotFound):
		h.writeError(w, http.StatusNotFound, "carteira não encontrada")
	case errors.Is(err, domain.ErrInsufficientFunds):
		h.writeError(w, http.StatusBadRequest, "saldo insuficiente para realizar a transferência")
	case errors.Is(err, domain.ErrInvalidAmount):
		h.writeError(w, http.StatusBadRequest, "valor da transferência deve ser positivo")
	case errors.Is(err, domain.ErrSameWallet):
		h.writeError(w, http.StatusBadRequest, "carteira de origem e destino não podem ser iguais")
	case errors.Is(err, domain.ErrWalletInactive):
		h.writeError(w, http.StatusBadRequest, "carteira inativa não pode realizar operações")
	default:
		h.writeError(w, http.StatusInternalServerError, "erro interno ao processar a transferência")
	}
}

func (h *TransferHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("falha ao serializar resposta JSON", slog.Any("error", err))
	}
}

func (h *TransferHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, errorResponse{Error: message})
}

// sanitizeDecodeError extrai uma mensagem de erro segura e legível a
// partir de um erro de decodificação JSON, sem vazar detalhes internos de
// implementação para o cliente.
func sanitizeDecodeError(err error) string {
	if errors.Is(err, io.EOF) {
		return "corpo da requisição vazio"
	}
	return "formato inválido"
}
