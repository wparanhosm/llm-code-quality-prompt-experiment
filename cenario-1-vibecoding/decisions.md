# Decision Record

## [ADR-001] Implementação da Aplicação Pix P2P — Execução do ADR-001

**Data:** 2026-06-04T21:15:00Z  
**Status:** Aceita  
**Modelo de LLM:** claude-opus-4-5-20260415 | Temperatura: 1.0  
**ID do Trace (Langfuse):** trace-pix-p2p-20260604-002

### 1. Contexto e Problema
Com o ADR-001 já definindo a arquitetura (DDD + DBTX + modernc.org/sqlite), era necessário materializar a decisão em código Go funcional, incluindo testes automatizados e persistência via SQLite. O diretório `app/` estava vazio.

### 2. Direcionadores Tecnológicos (Drivers)
- Fidelidade total ao ADR-001: mesma interface DBTX, mesma estratégia de transação atômica, mesmo driver pure-Go
- Cobertura de testes para os 5 cenários críticos documentados no ADR-001
- Código mínimo e idiomático em Go, sem over-engineering

### 3. Estrutura de Arquivos Criados
- `app/domain/wallet.go` — Entidade `Wallet` com `Debit`/`Credit`, `Transaction`, erros de domínio
- `app/infra/database/db.go` — Interface `DBTX`, `NewConnection`, schema DDL
- `app/infra/database/wallet_repo.go` — `WalletRepository` com `FindByUserID`, `Update`, `CreateTransaction`
- `app/service/transfer.go` — `TransferService` com transação atômica (`Begin`/`Commit`/`Rollback`)
- `app/main.go` — Entry point com seed data e demonstração
- `app/main_test.go` — 5 testes (sucesso, saldo insuficiente, mesmo usuário, valor inválido, carteira não encontrada)

### 4. Decisão e Padrão Aplicado
Implementação direta do ADR-001 sem desvios. Padrão de prompt: **Chain-of-Thought + DDD Layered Architecture** com SOLID aplicado por camada. O `TransferService.Transfer()` coordena: validação de entrada → `db.Begin()` → busca carteiras → `Debit` na origem → `Credit` no destino → `Update` ambas → `CreateTransaction` → `tx.Commit()`. O `defer tx.Rollback()` é no-op após commit bem-sucedido.

### 5. Consequências e Validação
- **Resultado do Build:** `go build ./...` — sucesso, zero erros
- **Resultado dos Testes:** `go test ./... -v` — 5/5 PASS em 0.691s
- **Trade-off:** Testes usam SQLite `:memory:` para isolamento e velocidade, sem necessidade de cleanup de arquivos
