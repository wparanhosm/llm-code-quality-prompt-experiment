# wallet-p2p — Transferências Financeiras P2P entre Carteiras Digitais

Serviço HTTP em **Go 1.26** para transferências Peer-to-Peer (P2P) entre carteiras
digitais, com garantias **ACID**, prevenção a **double-spending** via lock pessimista
e observabilidade estruturada. Construído com **Clean Architecture + DDD**, injeção
de dependência manual e **exclusivamente `net/http` nativo** (sem frameworks web).

---

## 1. Arquitetura

```
┌─────────────────────────────────────────────────────────────┐
│ Delivery (net/http nativo)                                  │
│   internal/delivery/httpapi — handlers, middlewares, rotas  │
└──────────────────────┬──────────────────────────────────────┘
                       │ depende de ▼ (interfaces puras)
┌──────────────────────┴──────────────────────────────────────┐
│ Usecase / Application                                       │
│   internal/usecase — TransferUseCase, portas (UnitOfWork,   │
│   WalletRepository, Tx, Clock)                              │
└──────────────────────┬──────────────────────────────────────┘
                       │ depende de ▼ (entidades puras)
┌──────────────────────┴──────────────────────────────────────┐
│ Domain (isolado, zero dependência de infra)                 │
│   internal/domain — Wallet, Transaction, Money, erros       │
└─────────────────────────────────────────────────────────────┘
                       ▲ implementa portas
┌──────────────────────┴──────────────────────────────────────┐
│ Infrastructure / Adapter                                    │
│   internal/adapter/postgres — database/sql + lib/pq,        │
│   SELECT ... FOR UPDATE, UnitOfWork sobre *sql.Tx           │
└─────────────────────────────────────────────────────────────┘
```

- **Domínio puro**: `Wallet`/`Transaction`/`Money` são imutáveis em operações de
  negócio (`Debit`/`Credit` retornam novas instâncias). `Money` é inteiro em
  centavos — sem ponto flutuante. Invariantes: saldo nunca negativo, valor > 0,
  origem ≠ destino.
- **DI manual**: todo o wiring está em `cmd/wallet-p2p-api/main.go`
  (composition root). Nenhum framework de DI (fx/dig/wire).
- **Atomicidade**: débito + crédito + histórico em **uma única `sql.Tx`**
  (Unit of Work). Qualquer falha ⇒ `ROLLBACK` total. Pânico ⇒ rollback + repanic.
- **Anti-double-spending**: `SELECT ... FOR UPDATE` com **ordenção
  determinística de locks** (menor `wallet_id` primeiro), eliminando deadlocks
  em transferências cruzadas A→B / B→A. Isolamento: `READ COMMITTED`
  (suficiente, pois o lock de linha serializa o acesso às carteiras).
- **Idempotência (suporte de schema)**: coluna `idempotency_key` com índice
  único parcial — requisições repetidas com a mesma chave falham por violação
  de unicidade e sofrem rollback completo.

### Mapeamento de erros HTTP

| Condição                                   | Status | `error` code          |
|--------------------------------------------|--------|-----------------------|
| JSON malformado / campos desconhecidos     | 400    | `invalid_body`        |
| ID vazio, valor ≤ 0, origem == destino     | 400    | `invalid_input`       |
| Carteira inexistente                       | 404    | `wallet_not_found`    |
| Saldo insuficiente                         | 422    | `insufficient_funds`  |
| Falha interna (detalhe só em log)          | 500    | `internal_error`      |
| Método ≠ POST em `/v1/transfers`           | 405    | `method_not_allowed`  |
| Content-Type ≠ application/json            | 415    | `unsupported_media_type` |

> Saldo insuficiente usa **422**: a requisição é sintaticamente válida e a
> carteira existe — a regra de negócio é quem impede o processamento.

## 2. Estrutura de diretórios

```
.
├── cmd/
│   └── wallet-p2p-api/
│       ├── main.go            # composition root: config, wiring, graceful shutdown
│       └── main_test.go       # testes de config, ping, listen/shutdown e wiring
├── internal/
│   ├── domain/
│   │   ├── money.go / money_test.go
│   │   ├── wallet.go / wallet_test.go
│   │   ├── transaction.go / transaction_test.go
│   │   └── errors.go
│   ├── usecase/
│   │   ├── ports.go           # UnitOfWork, WalletRepository, Tx, Clock
│   │   ├── transfer.go        # TransferUseCase (orquestração ACID)
│   │   ├── clock.go
│   │   └── transfer_test.go   # mocks testify, ordem de locks, rollbacks
│   ├── adapter/postgres/
│   │   ├── unit_of_work.go    # BeginTx/Commit/Rollback + repanic
│   │   ├── wallet_repository.go
│   │   ├── wallet_repository_test.go  # go-sqlmock
│   │   └── integration_test.go        # build tag: integration (postgres real)
│   ├── delivery/httpapi/
│   │   ├── transfer_handler.go
│   │   ├── middleware.go      # RequestID, Logging, Recover
│   │   ├── server.go          # mux nativo: rotas e cadeia de middlewares
│   │   └── transfer_handler_test.go   # httptest end-to-end do handler
│   └── platform/observability/
│       ├── context.go         # request_id no context + atributos slog
│       └── context_test.go
├── migrations/
│   ├── 000001_create_wallets_and_transactions.up.sql
│   ├── 000001_create_wallets_and_transactions.down.sql
│   └── 000002_seed_dev.up.sql # 2 carteiras de exemplo
├── docker-compose.yml
├── Makefile
├── go.mod / go.sum
└── README.md
```

## 3. API

### `POST /v1/transfers`

```json
{
  "from_wallet_id": "11111111-1111-1111-1111-111111111111",
  "to_wallet_id":   "22222222-2222-2222-2222-222222222222",
  "amount_cents":   1500,
  "idempotency_key": "opcional-uuid-do-cliente"
}
```

**201 Created**:
```json
{
  "transaction_id": "5fc5688f-05c9-488c-afc7-d09d92ec81c7",
  "from_wallet_id": "11111111-1111-1111-1111-111111111111",
  "to_wallet_id":   "22222222-2222-2222-2222-222222222222",
  "amount_cents":   1500,
  "from_new_balance_cents": 98500,
  "to_new_balance_cents":   6500
}
```

### `GET /healthz` e `GET /readyz`

Liveness simples e readiness com `PingContext` no PostgreSQL (503 se indisponível).

## 4. Observabilidade

- **Logs JSON** via `log/slog` (handler JSON) com `context.Context` propagado
  em todas as camadas (`InfoContext`/`WarnContext`/`ErrorContext`).
- **Correlation ID**: middleware gera UUID v4 (ou reutiliza `X-Request-ID` do
  cliente), injeta no `context.Context`, no log de acesso e no header de resposta.
- **Tracing**: optou-se por **não** adicionar OpenTelemetry neste módulo para
  manter o grafo de dependências mínimo e o wiring 100% manual. O `context.Context`
  já flui por todas as camadas; para adotar OTel basta instalar um
  `otelhttp.NewHandler` envolvendo o handler raiz em `Server.Handler()` e um
  exporter no `main.go` — nenhuma camada interna precisa mudar.

## 5. Como executar

```bash
# 1. Sobe PostgreSQL (migrations + seed aplicados automaticamente)
make docker-up          # ou: docker compose up -d --wait

# 2. Testes unitários (race detector)
make test               # ou: go test ./... -race -count=1

# 3. Cobertura (perfil + resumo por função)
make coverage

# 4. Testes de integração (postgres real do compose)
make test-integration   # go test -tags=integration ./internal/adapter/postgres/...

# 5. Build + execução do serviço
make build run          # binário em bin/wallet-p2p-api, porta :8080 (default)

# 6. Lint (golangci-lint se instalado; fallback go vet)
make lint
```

### Configuração (variáveis de ambiente)

| Variável              | Default                                                        |
|-----------------------|----------------------------------------------------------------|
| `HTTP_ADDR`           | `:8080`                                                        |
| `DATABASE_DSN`        | `postgres://wallet:wallet@localhost:5432/wallet_p2p?sslmode=disable` |
| `DB_MAX_OPEN_CONNS`   | `25`                                                           |
| `DB_MAX_IDLE_CONNS`   | `5`                                                            |
| `DB_CONN_MAX_LIFETIME`| `5m`                                                           |
| `DB_CONN_MAX_IDLE_TIME`| `1m`                                                          |
| `SHUTDOWN_TIMEOUT`    | `10s`                                                          |

### Exemplo fumaca (com seed do compose)

```bash
curl -i -X POST http://localhost:8080/v1/transfers \
  -H 'Content-Type: application/json' \
  -H 'X-Request-ID: demo-001' \
  -d '{"from_wallet_id":"11111111-1111-1111-1111-111111111111",
       "to_wallet_id":"22222222-2222-2222-2222-222222222222",
       "amount_cents":1500}'
```

## 6. Qualidade verificada

- `go vet ./...` — limpo.
- `go test ./... -race -coverprofile=coverage.out` — **cobertura total: 96.7%**
  (domain 100%, adapter/postgres 100%, observability 100%, httpapi 98.7%,
  usecase 95.9%, cmd 90.7%).
- Testes de integração com PostgreSQL real (docker-compose), incluindo
  **40 transferências concorrentes cruzadas** com verificação de conservação
  de valor (anti-double-spending) e rollback total em falha de INSERT.
