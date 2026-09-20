# AGENTS.md

Serviço Go de transferências P2P com PostgreSQL. Os comandos abaixo são os comandos reais do repositório.

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

## Comandos

- Formatação: `make fmt`
- Lint estático: `make lint`
- Testes: `make test`
- Testes com race detector: `make test-race`
- Cobertura com gate de 95%: `make coverage`
- Build: `make build`
- Execução (requer `DATABASE_URL`): `make run`
