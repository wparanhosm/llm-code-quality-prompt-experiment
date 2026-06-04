# Engenharia de Prompts Estruturada vs. “Vibecoding”: Impacto de Padrões de Projeto na Consistência de Códigos Gerados por LLMs

Este repositório cumpre o papel de laboratório experimental para a coleta e análise de dados do Trabalho de Conclusão de Curso (TCC) do **MBA em Engenharia de Software da USP/ESALQ**.

O objetivo fundamental desta pesquisa é avaliar empiricamente se a engenharia de prompts estruturada consegue atuar como um "cinto de segurança" arquitetural na geração automatizada de código, mitigando os riscos de endividamento técnico, acoplamento nocivo e vulnerabilidades transacionais associados ao fenômeno do “vibecoding” (programação orientada estritamente à intenção funcional).

---

## 🧪 Desenho do Experimento

O experimento consiste na submissão de um mesmo requisito de negócio de alta criticidade a uma Large Language Model (LLM) sob dois cenários operacionais distintos:

### Cenário 1: “Vibecoding” Puro
*   **Abordagem:** Instruções puramente funcionais, simulando um desenvolvedor focado apenas em velocidade de entrega, sem imposição de restrições de arquitetura ou segurança.
*   **Diretório dos Artefatos:** `/cenario-1-vibecoding`

### Cenário 2: Agente Engenheiro
*   **Abordagem:** Uso de um *System Prompt* altamente estruturado com aplicação de padrões de projeto (*Design Patterns*), princípios *SOLID*, *Clean Architecture* e controle rígido de concorrência e transações distribuídas.
*   **Diretório dos Artefatos:** `/cenario-2-agente-engenheiro`

> **Caso de Uso Avaliado:** Desenvolvimento de uma funcionalidade de transferência financeira *Peer-to-Peer* (P2P) entre duas carteiras digitais utilizando a linguagem **Golang**. O cenário exige validação de saldo, integridade relacional (*ACID*) e prevenção ativa contra condições de corrida (*Race Conditions* / *Double Spending*).

---

## 🛠️ Pilha Tecnológica e Esteira de Auditoria

Para garantir o determinismo científico, a reprodutibilidade da pesquisa e a eliminação de vieses subjetivos, o ecossistema de testes foi blindado com as seguintes ferramentas:

1.  **Rastreabilidade de IA (Langfuse):** Plataforma de observabilidade de LLM utilizada para capturar o rastro (*trace*) exato da execução da API, registrando *inputs*, *outputs*, consumo de *tokens* e hashes de auditoria imutáveis.
2.  **Análise Estática de Código (golangci-lint):** Agregador local de *linters* configurado rigidamente para disparar análises computacionais via `gocyclo` (complexidade), `errcheck` (erros omitidos) e `staticcheck` (concorrência e performance).
3.  **Métricas de Qualidade Industriais (SonarCloud):** Centralizador de qualidade conectado a esteira que extrai os indicadores consolidados para a construção das tabelas comparativas.

---

## 📊 Matriz de Métricas Monitoradas

A avaliação comparativa entre os códigos gerados nos dois cenários baseia-se nos seguintes indicadores quantitativos e qualitativos:


| Dimensão | Indicador Técnico | Ferramenta de Extração | Meta de Engenharia |
| :--- | :--- | :--- | :--- |
| **Manutenibilidade** | Complexidade Ciclomática | `gocyclo` / SonarCloud | Minimizar caminhos independentes por função. |
| **Acoplamento** | Segregação de Camadas | Análise Arquitetural | Isolar regras de negócio da camada de infraestrutura. |
| **Conformidade** | Densidade de “Code Smells” | SonarCloud | Reduzir o endividamento técnico gerado pela LLM. |
| **Segurança** | Resistência a *Race Conditions* | `govet` / `staticcheck` | Bloquear concorrência desprotegida (*Double Spending*). |
| **Integridade** | Atomicidade Transacional | Auditoria de Código | Garantir transações relacionais operando sob propriedades *ACID*. |

---

## ⚙️ Parâmetros de Reprodutibilidade

As chamadas de API são registradas no Langfuse com os seguintes metadados para garantir rastreabilidade e reprodutibilidade:

| Parâmetro | Cenário 1 — Vibecoding | Cenário 2 — Agente Engenheiro |
| :--- | :--- | :--- |
| **Modelo** | `claude-opus-4-5-20260415` | `claude-opus-4-5-20260415` |
| **Temperatura** | `1.0` | `default` |
| **Trace ID** | `trace-pix-p2p-20260604-002` | `p2p-wallet-clean-arch-001` |

---

## 📊 Resultados Apurados (Rodada 1 — 2026-06-04)

| Métrica | Cenário 1 — Vibecoding | Cenário 2 — Agente Engenheiro |
| :--- | :---: | :---: |
| **Testes automatizados** | 5 | 47 |
| **Resultado dos testes** | 100% PASS | 100% PASS |
| **Cobertura de código** | — | 97.6% |
| **Race Detector (`-race`)** | — | Limpo |
| **`go vet` / `go build`** | Limpo | Limpo |
| **Banco de dados** | SQLite (`modernc.org/sqlite`) | PostgreSQL (`lib/pq`) |
| **Arquitetura** | DDD simplificado | Clean Architecture (4 camadas) |
| **Controle de concorrência** | Não aplicado | `SELECT FOR UPDATE` + lock ordering |
| **Integridade transacional** | `sql.Tx` básico | `TxManager` via `context.Value` + ACID |
| **Observabilidade** | Não aplicada | Structured JSON logs (`log/slog`) + Correlation ID |

---

## 📁 Estrutura do Repositório

```text
├── .agents/
│   ├── decisions.md              # Template de instrução para registro de ADRs pelo agente
│   └── system_instruction.md     # Instrução para geração de trace_langfuse.json (spans + tokens)
├── .github/
│   ├── CODEOWNERS                # Proteção de branch e revisão obrigatória
│   └── workflows/
│       └── auto-pr.yml           # PR automático: feature/* → develop, develop → main
├── cenario-1-vibecoding/
│   ├── domain/wallet.go          # Entidade Wallet com Debit/Credit e erros de domínio
│   ├── infra/database/db.go      # Interface DBTX + conexão SQLite + DDL
│   ├── infra/database/wallet_repo.go  # Repositório de carteiras
│   ├── service/transfer.go       # TransferService com transação atômica
│   ├── main.go                   # Entry point com seed data
│   ├── main_test.go              # 5 testes de integração (SQLite :memory:)
│   ├── prompt.txt                # Prompt original enviado à LLM (vibecoding)
│   ├── decisions.md              # ADR-001: decisões arquiteturais registradas pela IA
│   └── trace_langfuse.json       # Log de auditoria: spans, tokens e timestamps
├── cenario-2-agente-engenheiro/
│   ├── cmd/api/main.go           # Entry point com DI manual de todas as dependências
│   ├── internal/
│   │   ├── domain/               # Entidades puras, erros e interfaces de repositório
│   │   ├── usecase/              # Orquestração transacional P2P (TxManager + lock ordering)
│   │   ├── infra/
│   │   │   ├── postgres/         # Repositórios SQL com SELECT FOR UPDATE e go-sqlmock
│   │   │   └── logger/           # Structured JSON logger (log/slog)
│   │   └── delivery/http/        # Handler REST + middleware chain (CorrelationID, Recover)
│   ├── migrations/001_init.sql   # DDL de criação das tabelas wallets e transactions
│   ├── prompt.txt                # Prompt estruturado de engenharia enviado à LLM
│   ├── decisions.md              # ADR-001: Clean Architecture + decisões de concorrência
│   └── trace_langfuse.json       # Log de auditoria: spans, tokens e timestamps
├── sync_traces.py                # Script Python para sincronização de traces com o Langfuse
├── Changelog.md                  # Histórico de alterações do projeto
└── README.md                     # Documentação científica do laboratório
```