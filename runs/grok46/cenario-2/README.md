# P2P Transfer Service

Serviço HTTP em Go para transferências financeiras Peer-to-Peer entre carteiras digitais, com Clean Architecture, consistência ACID e bloqueio pessimista anti double-spending.

## Visão geral arquitetural

- **Domain** (`internal/domain`): entidades `Wallet` e `Transfer`, money em centavos (`int64`), erros de negócio. Sem dependência de HTTP/SQL.
- **Use case** (`internal/usecase`): `TransferP2P` valida origem ≠ destino, valor > 0 e delega atomicidade ao `UnitOfWork`.
- **Adapter** (`internal/adapter/postgres`): `SELECT ... FOR UPDATE` ordenado por ID de carteira (evita deadlock), débito/crédito/histórico no mesmo `*sql.Tx`.
- **Delivery** (`internal/delivery/http`): `net/http` nativo, JSON, request ID, limites de payload.
- **Composition root**: `cmd/p2p-transfer/main.go` faz wiring manual (sem fx).

## Fluxo da aplicação

1. `POST /transfers` com `{ "from_wallet_id", "to_wallet_id", "amount" }` (amount em **centavos**).
2. Handler valida JSON e chama o use case.
3. Repositório inicia transação, trava as duas carteiras em ordem lexicográfica de ID, aplica débito/crédito, grava histórico, commit.
4. Resposta `201` com o registro da transferência.

Erros: `400` validação, `404` carteira inexistente, `409` saldo insuficiente, `500` falha interna.

## Estrutura

```
cmd/p2p-transfer/main.go
internal/domain/
internal/usecase/
internal/adapter/postgres/
internal/delivery/http/
```

## Como executar testes

```bash
make test
make coverage
make lint
```

## Como iniciar o serviço

Requer PostgreSQL.

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/p2p?sslmode=disable"
export HTTP_ADDR=":8080"
make build
./bin/p2p-transfer
```

Health:

```bash
curl -s localhost:8080/health
```

Transferência:

```bash
curl -s -X POST localhost:8080/transfers \
  -H 'Content-Type: application/json' \
  -d '{"from_wallet_id":"w1","to_wallet_id":"w2","amount":100}'
```

Seed de carteiras (exemplo):

```sql
INSERT INTO wallets (id, balance_cents) VALUES ('w1', 10000), ('w2', 0);
```
