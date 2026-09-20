# p2p-transfer

Serviço HTTP para processar transferências financeiras Peer-to-Peer (P2P) entre carteiras digitais, escrito em Go seguindo Clean Architecture, DDD e TDD.

## 1. Visão Geral Arquitetural

O sistema é dividido em camadas concêntricas, onde as dependências sempre apontam para dentro (Regra de Dependência da Clean Architecture):

```
delivery/http  --->  usecase  --->  domain
                         ^
                         |
adapter/repository/postgres (implementa as portas definidas em usecase)
```

- **`internal/domain`**: entidades puras (`Wallet`, `Transaction`, `Money`). Zero dependências externas — nem sequer de `database/sql` ou `net/http`. Concentra os invariantes de negócio (saldo não pode ficar negativo, valores devem ser positivos, carteira inativa não opera).
- **`internal/usecase`**: orquestra a regra de negócio da transferência P2P (`TransferUseCase`). Depende apenas de **interfaces** (`WalletRepository`, `TransactionRepository`, `UnitOfWork`, `IDGenerator`), nunca de implementações concretas — inversão de dependência pura.
- **`internal/adapter/repository/postgres`**: implementação concreta das portas do usecase usando `database/sql` + driver `pgx`. É aqui que vive o `SELECT ... FOR UPDATE` (bloqueio pessimista) e o controle transacional via `*sql.Tx`.
- **`internal/delivery/http`**: controller HTTP REST usando **exclusivamente** o pacote nativo `net/http` (sem frameworks). Traduz JSON <-> `usecase.TransferInput`/`TransferOutput` e mapeia erros de domínio para códigos HTTP.
- **`cmd/p2ptransfer/main.go`**: único ponto de wiring manual (injeção de dependência via construtores). Nenhum container de DI ou framework é usado — todas as dependências concretas (conexão Postgres, gerador de UUID, logger) são instanciadas aqui e passadas explicitamente.
- **`pkg/idgen`**: capacidade genérica e reutilizável (geração de UUID), sem conhecimento do domínio.

### Decisões de design relevantes

| Decisão | Justificativa |
|---|---|
| Dinheiro como `int64` (centavos) no domínio | Evita erros de arredondamento de ponto flutuante em operações financeiras. |
| `SELECT ... FOR UPDATE` com IDs ordenados | Garante consistência forte (anti-double-spending) e previne deadlocks entre transferências concorrentes que envolvem o mesmo par de carteiras em ordens opostas (A→B concorrente com B→A). |
| `UnitOfWork` como abstração de transação | O usecase nunca importa `database/sql`; toda atomicidade (ACID) é garantida pela implementação concreta em `adapter/repository/postgres`, mantendo o núcleo de negócio 100% testável com mocks em memória. |
| `net/http` puro na camada delivery | Atende ao requisito de não introduzir frameworks HTTP externos, usando `http.ServeMux` com roteamento por método (Go 1.22+). |
| Erros sempre com `%w` | Preserva a cadeia de causas (`errors.Is`/`errors.As`), permitindo que a camada HTTP mapeie erros de domínio (`ErrInsufficientFunds`, `ErrWalletNotFound`, etc.) para códigos HTTP corretos sem acoplamento direto. |

## 2. Estrutura de Diretórios

```
p2p-transfer/
├── cmd/
│   └── p2ptransfer/
│       ├── main.go              # wiring manual + ciclo de vida do servidor
│       └── main_test.go
├── internal/
│   ├── domain/                  # entidades puras (Wallet, Transaction, Money)
│   │   ├── wallet.go
│   │   ├── wallet_test.go
│   │   ├── transaction.go
│   │   └── transaction_test.go
│   ├── usecase/                 # regra de negócio + portas (interfaces)
│   │   ├── ports.go
│   │   ├── transfer.go
│   │   ├── transfer_test.go
│   │   └── mocks/                # test doubles em memória das portas
│   │       └── mocks.go
│   ├── adapter/
│   │   └── repository/
│   │       └── postgres/         # implementação concreta das portas (SQL)
│   │           ├── unit_of_work.go
│   │           ├── unit_of_work_test.go
│   │           ├── wallet_repository.go
│   │           ├── wallet_repository_test.go
│   │           ├── transaction_repository.go
│   │           └── transaction_repository_test.go
│   ├── delivery/
│   │   └── http/                 # controller REST (net/http puro)
│   │       ├── transfer_handler.go
│   │       └── transfer_handler_test.go
│   └── platform/
│       ├── database/              # conexão/pool Postgres
│       │   ├── db.go
│       │   └── db_test.go
│       └── logger/                 # setup de slog estruturado
│           ├── logger.go
│           └── logger_test.go
├── pkg/
│   └── idgen/                     # gerador de UUID reutilizável
│       ├── uuid.go
│       └── uuid_test.go
├── migrations/                    # scripts SQL versionados (up/down)
│   ├── 001_create_wallets_and_transactions.up.sql
│   ├── 001_create_wallets_and_transactions.down.sql
│   ├── 002_seed_sample_wallets.up.sql
│   ├── 002_seed_sample_wallets.down.sql
│   └── init/                      # cópias aplicadas automaticamente pelo container Postgres
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod / go.sum
└── README.md
```

## 3. Fluxo da Aplicação

### Endpoint: `POST /transfers`

**Requisição:**
```json
{
  "from_wallet_id": "wallet-alice",
  "to_wallet_id": "wallet-bob",
  "amount_cents": 1500
}
```

**Fluxo interno (`TransferUseCase.Execute`):**
1. Validação de entrada: valor positivo (`domain.NewMoney`), IDs não vazios e distintos.
2. Abre uma transação SQL (`UnitOfWork.Execute` → `*sql.Tx` com `ReadCommitted`).
3. `WalletRepository.LockForUpdate` busca as duas carteiras com `SELECT ... FOR UPDATE`, com os IDs ordenados lexicograficamente antes da query — isso garante que, mesmo sob alta concorrência com múltiplas transferências simultâneas envolvendo as mesmas carteiras (em qualquer direção), os locks são sempre adquiridos na mesma ordem, eliminando deadlocks.
4. Regras de negócio aplicadas em memória sobre as entidades `domain.Wallet`: carteiras devem existir, estar `ACTIVE`, e a origem deve ter saldo suficiente (`Wallet.Debit` / `Wallet.Credit`).
5. Débito e crédito são persistidos (`WalletRepository.Save`) e o histórico da transação é gravado (`TransactionRepository.Save`) — tudo dentro da mesma transação.
6. Se qualquer etapa falhar, a transação inteira é revertida (`tx.Rollback`) e nenhum saldo é alterado (atomicidade ACID). Se tudo suceder, `tx.Commit()` confirma as mudanças.
7. A camada HTTP mapeia o resultado: `200 OK` em sucesso; `400`, `404` ou `500` em caso de erro, conforme o tipo de erro de domínio retornado.

**Resposta de sucesso (200):**
```json
{
  "transaction_id": "b3f1...",
  "from_wallet_id": "wallet-alice",
  "to_wallet_id": "wallet-bob",
  "amount_cents": 1500,
  "from_new_balance": 98500,
  "to_new_balance": 51500
}
```

**Respostas de erro:** `{"error": "mensagem"}` com status `400` (valor inválido, saldo insuficiente, carteira inativa, carteiras iguais), `404` (carteira não encontrada), `405` (método diferente de POST) ou `500` (falha interna).

### Endpoint: `GET /healthz`
Verificação simples de liveness, retorna `{"status":"ok"}`.

## 4. Como rodar os testes

Pré-requisitos: Go 1.23+ (o módulo declara `go 1.26` em `go.mod`, ajuste a toolchain conforme disponível no seu ambiente).

```bash
# Todos os testes (domain, usecase, adapters via sqlmock, delivery via httptest)
make test
# equivalente a: go test ./...

# Com detector de race conditions (recomendado, valida a concorrência do usecase)
make test-race
# equivalente a: go test ./... -race

# Relatório de cobertura (a suíte atual mantém >90% nas camadas de negócio)
make cover
# equivalente a: go test ./... -coverpkg=./... -coverprofile=coverage.out

# Relatório HTML navegável
make cover-html
```

Nenhum teste depende de um Postgres real rodando: os testes de `internal/adapter/repository/postgres` usam [`go-sqlmock`](https://github.com/DATA-DOG/go-sqlmock) para simular o driver SQL, e os testes de `internal/usecase` usam test doubles em memória (`internal/usecase/mocks`).

## 5. Como iniciar o serviço

### Opção A — Docker Compose (recomendado, sobe Postgres + aplicação)

```bash
make docker-up
# equivalente a: docker compose up --build
```

O Postgres inicia com o schema e dados de exemplo (`migrations/init/*.sql`) já aplicados automaticamente. O serviço fica disponível em `http://localhost:8080`.

```bash
curl -X POST http://localhost:8080/transfers \
  -H "Content-Type: application/json" \
  -d '{"from_wallet_id":"wallet-alice","to_wallet_id":"wallet-bob","amount_cents":1500}'
```

### Opção B — Local (Postgres externo)

```bash
export DATABASE_DSN="postgres://postgres:postgres@localhost:5432/p2p_transfer?sslmode=disable"
export HTTP_ADDR=":8080"
export LOG_LEVEL="info"

make build
./bin/p2ptransfer
```

Aplique as migrations manualmente antes de iniciar (`migrations/001_*.up.sql`, `migrations/002_*.up.sql`) usando a ferramenta de sua preferência (`psql -f`, `migrate`, etc.).

### Variáveis de ambiente suportadas

| Variável | Padrão | Descrição |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Endereço/porta do servidor HTTP |
| `DATABASE_DSN` | `postgres://postgres:postgres@localhost:5432/p2p_transfer?sslmode=disable` | DSN de conexão com o Postgres |
| `LOG_LEVEL` | `info` | Nível mínimo de log (`debug`, `info`, `warn`, `error`) |

O servidor realiza *graceful shutdown* ao receber `SIGINT`/`SIGTERM`, aguardando até 15s para finalizar requisições em andamento antes de encerrar.
