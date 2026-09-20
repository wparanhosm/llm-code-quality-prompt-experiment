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

Serviço P2P em Go (módulo `github.com/wallet/p2p-transfer`). Comandos reais:

- `make test` — `go test ./...`
- `make coverage` — coverage com `-covermode=atomic`
- `make lint` — `go vet ./...`
- `make build` — binário em `bin/p2p-transfer`
- `make tidy` — `go mod tidy`

Entrypoint: `cmd/p2p-transfer`. Variáveis: `DATABASE_URL`, `HTTP_ADDR`.
