# P2P Transfer Service

Serviço Go para transferências P2P com consistência forte, Clean Architecture e PostgreSQL.

## Arquitetura e fluxo

- **Domain** contém `Money`, `Wallet` e `Transfer`, sem dependências de infraestrutura.
- **Application** valida o comando e orquestra uma unidade transacional por interfaces.
- **Infrastructure** implementa os ports com `database/sql` e PostgreSQL.
- **Delivery** expõe somente `net/http`; `cmd/api` faz o wiring manual.

Em uma única `sql.Tx`, o repositório ordena os dois IDs, executa `SELECT ... ORDER BY id FOR UPDATE`, verifica ambas as carteiras, debita, credita, insere o histórico e faz commit. Qualquer falha causa rollback. A ordem global de locks evita deadlocks entre transferências inversas; o lock pessimista impede double spending sob requisições concorrentes. Valores são inteiros em centavos, nunca ponto flutuante.

PostgreSQL foi escolhido por locks de linha maduros, integridade referencial, constraints e transações ACID. O pool padrão é 50 conexões abertas e 25 ociosas, configurável por `DB_MAX_OPEN_CONNS`; deve ser ajustado ao limite do banco e número de réplicas da aplicação.

## Observabilidade e segurança

Logs JSON estruturados usam `slog`. O middleware aceita ou gera `X-Trace-ID`, devolve-o na resposta e o propaga via `context.Context` até SQL. Isso fornece correlação distribuída sem framework; um collector OpenTelemetry pode ser acrescentado no composition root sem contaminar domínio/use case. O servidor limita corpo e headers, rejeita campos JSON desconhecidos, parametriza todas as queries e define timeouts HTTP. Erros internos não são expostos ao cliente.

## Banco e execução

Requer Go 1.23 e PostgreSQL 16. A migration em `migrations/001_init.sql` cria schema, constraints e índices.

```sh
docker compose up -d
export DATABASE_URL='postgres://transfers:transfers@localhost:5432/transfers?sslmode=disable'
make run
```

Crie dados de desenvolvimento, por exemplo:

```sql
INSERT INTO wallets(id, balance_cents) VALUES ('wallet-a', 100000), ('wallet-b', 5000);
```

Requisição:

```sh
curl -i -X POST localhost:8080/v1/transfers -H 'Content-Type: application/json' \
  -d '{"source_wallet_id":"wallet-a","destination_wallet_id":"wallet-b","amount":"12.50"}'
```

## Qualidade

```sh
make fmt
make lint
make test
make test-race
make coverage
make build
```

O gate mede os pacotes produtivos reutilizáveis (`internal/...` e `pkg/...`) e exige 95%. O entrypoint `cmd/api`, que contém apenas composição, configuração, lifecycle e integração real com SO/PostgreSQL, é validado por build/vet; integração operacional deve também usar PostgreSQL em ambiente de CI. Os testes SQL com `sqlmock` verificam begin, lock ordenado, updates, insert, commit e rollback.

## Pressupostos e riscos

- IDs de transferência são aleatórios criptográficos; clientes não fornecem chave de idempotência. Retries após timeout podem produzir nova transferência. Em produção, recomenda-se adicionar uma chave de idempotência única por cliente.
- `READ COMMITTED` é suficiente porque todas as mutações de saldo passam pelos mesmos locks. Escritores externos que ignorem este protocolo quebram a garantia.
- A migration é automática apenas no Compose; deployments devem executá-la como etapa controlada antes do rollout.
