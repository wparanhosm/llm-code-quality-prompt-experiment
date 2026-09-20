# Pix P2P em Go

API HTTP para transferir saldo entre duas carteiras digitais. O débito, o crédito e o registro da transferência são executados na mesma transação PostgreSQL.

## Executar

Requer Docker com Compose:

```bash
docker compose up --build
```

O banco é inicializado com estas carteiras:

| ID | Saldo |
|---|---:|
| `user-1` | R$ 1.000,00 |
| `user-2` | R$ 250,00 |

## Fazer uma transferência

O valor deve ser enviado como string decimal positiva, com no máximo duas casas. Isso evita perda de precisão no JSON.

```bash
curl -i -X POST http://localhost:8080/transfers \
  -H 'Content-Type: application/json' \
  -d '{
    "source_user_id": "user-1",
    "destination_user_id": "user-2",
    "amount": "125.90"
  }'
```

Resposta `201 Created`:

```json
{
  "amount": "125.90",
  "created_at": "2026-09-06T12:00:00Z",
  "destination_user_id": "user-2",
  "id": "307f3da8-1574-465b-b0f4-e46812bbb309",
  "source_user_id": "user-1"
}
```

Possíveis erros:

| HTTP | Motivo |
|---:|---|
| `400` | JSON, valor ou IDs inválidos; origem igual ao destino |
| `404` | Uma das carteiras não existe |
| `409` | Saldo insuficiente |
| `500` | Falha interna ou no banco |

## Consistência

- Dinheiro é armazenado como `BIGINT` em centavos, nunca como ponto flutuante.
- As duas carteiras são bloqueadas com `SELECT ... FOR UPDATE` em ordem de ID.
- Saldo suficiente é validado após adquirir os bloqueios.
- Qualquer falha desfaz débito, crédito e histórico juntos.
- Restrições no banco impedem saldo negativo, transferência não positiva e origem igual ao destino.

## Desenvolvimento local

Com PostgreSQL disponível e o schema de `migrations/001_init.sql` aplicado:

```bash
export DATABASE_URL='postgres://pix:pix@localhost:5432/pix?sslmode=disable'
go run ./cmd/api
```

Para executar os testes:

```bash
go test ./...
```
