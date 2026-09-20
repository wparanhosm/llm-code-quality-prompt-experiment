# AGENTS.md

Repositório vazio (sem código, sem manifests, sem CI ainda). Não assuma stack, testes ou build steps até que existam arquivos que os definam — verifique antes de inventar comandos.

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

Bootstrap concluído (2026-09-06): módulo `wallet-p2p` — transferências P2P entre carteiras digitais (Go 1.26, PostgreSQL, net/http nativo, Clean Architecture).

Comandos reais do toolchain (verificados):
- Build: `make build` (binário em `bin/wallet-p2p-api`) ou `go build ./...`
- Testes unitários: `make test` (= `go test ./... -race -count=1`)
- Cobertura: `make coverage` (= test + `go tool cover -func=coverage.out`) — total atual: **96.7%**
- Lint: `make lint` (golangci-lint se instalado; fallback `go vet ./...`)
- Postgres local: `make docker-up` / `make docker-down` (migrations + seed via docker-entrypoint-initdb.d)
- Testes de integração (postgres real): `make test-integration` (= `go test -tags=integration ./internal/adapter/postgres/... -race -count=1 -v`)
- Executar serviço: `make run` (porta `:8080`; config via env — ver README.md)

Detalhes de arquitetura, API e configuração: `README.md`.
