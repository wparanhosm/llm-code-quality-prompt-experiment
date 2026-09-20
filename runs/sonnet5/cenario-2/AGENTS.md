# AGENTS.md

## Agente customizado: @agente-engenheiro

Definido em `.opencode/agent/agente-engenheiro.md`. É o agente principal para qualquer trabalho de backend em Go neste repositório.

Regras que ele sempre aplica (não repita/perguntar, apenas siga):
- Clean Architecture + DDD: domínio isolado de frameworks/DB/infra.
- Layout padrão Go: `/cmd`, `/internal`, `/pkg`.
- Testes unitários obrigatórios junto com todo código produtivo (TDD-friendly: interfaces + DI + mocks).
- Erros sempre com `fmt.Errorf("...: %w", err)` — nunca engolir erros.
- Logs estruturados (`slog`/`zap`) e `context.Context` propagado por todas as camadas.
- Concorrência seguindo `sync.WaitGroup`/`errgroup`, sem race conditions.
- Nunca entregar código com placeholders/TODO — só código completo e compilável.
- Formato de saída obrigatório ao entregar algo: (1) visão geral arquitetural, (2) árvore de diretórios, (3) código-fonte + testes, (4) documentação markdown com fluxo/como rodar testes/como iniciar o serviço.

## Estado do projeto

Módulo Go `p2p-transfer` implementado: serviço HTTP de transferência P2P entre carteiras digitais, seguindo Clean Architecture (`internal/domain`, `internal/usecase`, `internal/adapter/repository/postgres`, `internal/delivery/http`), com wiring manual em `cmd/p2ptransfer/main.go`. Ver `README.md` para visão geral arquitetural completa e fluxo da aplicação.

### Comandos reais (Go 1.26, ver `go.mod`)

- Build: `make build` (ou `go build ./...`)
- Rodar todos os testes: `make test` (ou `go test ./...`)
- Rodar testes com detector de race conditions: `make test-race` (ou `go test ./... -race`)
- Cobertura de testes: `make cover` (gera `coverage.out`; cobertura atual >90% nas camadas domain/usecase/adapter/delivery)
- Lint (gofmt + go vet): `make lint`
- Subir aplicação + Postgres via Docker: `make docker-up` (usa `docker-compose.yml`)
- Migrations SQL: `migrations/001_*.sql`, `migrations/002_*.sql` (aplicadas automaticamente pelo container Postgres via `migrations/init/`)

Toolchain: `go.mod` (módulo `p2p-transfer`), `Makefile`, `Dockerfile`, `docker-compose.yml`.
