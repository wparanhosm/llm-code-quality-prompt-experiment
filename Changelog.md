# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Gerado `cenario-1-vibecoding/sonar_output.json`: métricas SonarQube extraídas via API REST após execução do `sonar-scanner-cli` (Docker) — coverage 72.7%, bugs 0, vulnerabilidades 0, code smells 2, complexidade ciclomática 29, complexidade cognitiva 18, dívida técnica 12 min.
- Gerado `cenario-2-agente-engenheiro/sonar_output.json`: métricas SonarQube extraídas via API REST — coverage 87.7%, bugs 0, vulnerabilidades 0, code smells 1, complexidade ciclomática 81, complexidade cognitiva 63, dívida técnica 10 min.
- Adicionados arquivos `sonar-project.properties` em ambos os cenários para configuração do scanner local.

### Changed
- Atualizado `README.md` com seção de Resultados SonarQube (Rodada 1 — 2026-06-07): tabela comparativa completa com as métricas extraídas dos `sonar_output.json` de ambos os cenários e análise dos destaques.
- Atualizado `README.md` com dados reais da branch: modelos e temperaturas efetivamente usados, resultados dos testes de ambos os cenários, estrutura completa de arquivos do repositório e tabela comparativa de métricas apuradas.
- Atualizado `Changelog.md` para refletir o histórico completo de commits.

---

## [develop] - 2026-06-04

> Commit: `be027a1` — `:test_tube: feat: add AI-generated scenarios 1 and 2 for LLM code quality experiment`

### Added
- Adicionado `cenario-1-vibecoding/`: implementação Pix P2P em Go gerada por IA (`claude-opus-4-5-20260415`, temp. 1.0) com abordagem vibe coding — DDD + SQLite puro (`modernc.org/sqlite`); inclui `domain/`, `infra/`, `service/`, `main.go`, `main_test.go` (5 testes, 100% PASS), `prompt.txt`, `trace_langfuse.json` e `decisions.md` (ADR-001).
- Adicionado `cenario-2-agente-engenheiro/`: implementação P2P Transfer em Go gerada por IA (`claude-opus-4-5-20260415`) com prompt estruturado de engenharia — Clean Architecture (Domain, Usecase, Infra, Delivery), PostgreSQL com bloqueio pessimista (`SELECT FOR UPDATE`), `TxManager` via `context.Value`, structured JSON logs (`log/slog`) e `net/http` puro; 47 testes, 97.6% de cobertura global, race detector limpo.
- Adicionado `.agents/decisions.md`: template de instrução para o agente registrar ADRs com metadados de LLM, trace ID e resultado de testes.
- Adicionado `.agents/system_instruction.md`: instrução para o agente gerar/atualizar `trace_langfuse.json` com spans por tentativa (Self-Correction Loop), tokens e timestamps ISO-8601.
- Adicionado `sync_traces.py`: script Python para sincronização de traces locais com a plataforma Langfuse.

---

## [main] - 2026-06-04

> Commits: `b1fedbd` → `b10fe9d` → `0d09e45` → `f841da4`

### Added
- Criado `README.md` com documentação científica do laboratório experimental (TCC MBA USP/ESALQ).
- Criado `Changelog.md` para rastreamento de alterações do projeto.
- Adicionado `.gitignore` para ignorar variáveis de ambiente e arquivos temporários.
- Configurado Git Flow localmente (branches `main` e `develop`).
- Criado `.github/CODEOWNERS` para definir ownership padrão e exigir revisão de código nas branches protegidas.
- Criado `.github/workflows/auto-pr.yml` com criação automática de Pull Request de `develop` → `main`.

### Changed
- Expandido `.github/workflows/auto-pr.yml` para suportar também branches `feature/*` → `develop`, com título de PR dinâmico baseado no nome da feature.
