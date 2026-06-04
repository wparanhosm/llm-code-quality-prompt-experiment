# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Adicionado `cenario-1-vibecoding/`: implementação Pix P2P em Go gerada por IA (claude-opus-4-5) com vibe coding — arquitetura DDD + SQLite puro (modernc.org/sqlite), sem prompt estruturado de engenharia; inclui `domain/`, `infra/`, `service/`, `main.go`, `main_test.go` (5 testes, 100% pass), `prompt.txt`, `trace_langfuse.json` e `decisions.md` (ADR-001).
- Adicionado `cenario-2-agente-engenheiro/`: implementação P2P Transfer em Go gerada por IA (Claude Opus 4.6) com prompt estruturado de engenharia — Clean Architecture (Domain, Usecase, Infra, Delivery), PostgreSQL com bloqueio pessimista (`SELECT FOR UPDATE`), TxManager via `context.Value`, correlation ID em structured JSON logs (`log/slog`) e `net/http` puro; 47 testes, 97.6% de cobertura global, race detector limpo.
- Adicionado `.agents/decisions.md`: template de instruction para o agente registrar ADRs (Architecture Decision Records) com metadados de LLM, trace ID e resultado de testes.
- Adicionado `.agents/system_instruction.md`: instrução para o agente gerar/atualizar `trace_langfuse.json` com estrutura de spans por tentativa (Self-Correction Loop), incluindo tokens e timestamps ISO-8601.
- Adicionado `sync_traces.py`: script Python para sincronização de traces locais com a plataforma Langfuse.

## [main] - 2026-06-04

### Added
- Created `Changelog.md` to track project updates.
- Added `.gitignore` to prevent environment variables and temporary files from being tracked by git.
- Configured Git Flow structure locally (branch `main` and `develop`).
- Created `.github/CODEOWNERS` to set default ownership and require code reviews for branch protection.
- Created `.github/workflows/auto-pr.yml` to automate Pull Request creation (from `develop` to `main` and from `feature/*` to `develop`).
