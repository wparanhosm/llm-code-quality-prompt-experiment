# AGENTS.md

Módulo de transferência financeira P2P (Peer-to-Peer) entre carteiras digitais construído com Clean Architecture, DDD, PostgreSQL relacional nativo com pessimistic lock anti-deadlock, `net/http` e observabilidade com `log/slog`.

## Agente customizado: @agente-engenheiro

Definido em `.opencode/agent/agente-engenheiro.md`. É o agente principal para qualquer trabalho de backend em Go neste repositório.

Regras que ele sempre aplica:
- Clean Architecture + DDD: domínio isolado de frameworks/DB/infra.
- Layout padrão Go: `/cmd`, `/internal`, `/pkg`.
- Testes unitários obrigatórios com mocks manuais/estruturados e cobertura >= 95%.
- Erros com `fmt.Errorf("...: %w", err)` preservando rastreabilidade.
- Logs estruturados (`log/slog`) e `context.Context` com correlation ID propagado.
- Concorrência segura: `SELECT ... FOR UPDATE` com ordenação por Account ID (menor ID primeiro) para prevenir deadlocks.
- Sem frameworks pesados ou mágicos (Go nativo `net/http`, `database/sql`).

## Comandos do Projeto (Toolchain Real)

### Build & Execução
```bash
# Compilar binário da aplicação
make build

# Iniciar o servidor HTTP na porta padrão :8080 (ou configurada via PORT)
make run
```

### Testes & Qualidade
```bash
# Executar todos os testes com detector de race conditions
make test

# Executar testes com relatório detalhado de cobertura (atinge > 97%)
make test-coverage

# Executar análise estática e validação de formatação
make lint
```
