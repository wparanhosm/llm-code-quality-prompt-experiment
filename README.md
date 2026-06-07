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

## 🐳 Execução do SonarQube (Local via Docker)

### Pré-requisitos

1. Suba o SonarQube:
```bash
docker run -d --name sonarqube -p 9000:9000 sonarqube:community
```

2. Acesse `http://localhost:9000`, faça login (`admin`/`admin`) e gere um token em **My Account → Security → Generate Token**.

3. Adicione ao arquivo `.env` na raiz do repositório:
```
SONAR_TOKEN="seu_token_aqui"
SONAR_HOST_URL="http://localhost:9000"
```

4. Gere os relatórios de cobertura antes de rodar o scanner:
```bash
# Cenário 1
cd cenario-1-vibecoding
go test -coverprofile=coverage.out -coverpkg=./... -count=1 ./...

# Cenário 2
cd cenario-2-agente-engenheiro
go test -coverprofile=coverage_r2.out -count=1 ./internal/...
```

### Rodar o scanner

**Cenário 1 — Vibecoding:**
```bash
docker run --rm --network host \
  --env-file "C:/Users/walte/workspace/llm-code-quality-prompt-experiment/.env" \
  -v "C:/Users/walte/workspace/llm-code-quality-prompt-experiment/cenario-1-vibecoding:/usr/src" \
  sonarsource/sonar-scanner-cli
```

**Cenário 2 — Agente Engenheiro:**
```bash
docker run --rm --network host \
  --env-file "C:/Users/walte/workspace/llm-code-quality-prompt-experiment/.env" \
  -v "C:/Users/walte/workspace/llm-code-quality-prompt-experiment/cenario-2-agente-engenheiro:/usr/src" \
  sonarsource/sonar-scanner-cli
```

Os resultados ficam disponíveis em `http://localhost:9000` com os indicadores de **Bugs**, **Vulnerabilities**, **Code Smells**, **Coverage**, **Duplications** e **Security Hotspots** para cada cenário.

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

## 📊 Resultados Apurados (Rodada 2 — 2026-06-07)

> Ferramentas executadas: `go test`, `go test -race`, `go vet`, `go build`, `golangci-lint` (errcheck, staticcheck), `gocyclo`. Ambiente: Go 1.26.0 / Windows amd64.

| Métrica | Cenário 1 — Vibecoding | Cenário 2 — Agente Engenheiro |
| :--- | :---: | :---: |
| **Testes automatizados** | 5 | 56  |
| **Resultado dos testes** | 100% PASS | 100% PASS |
| **Cobertura de código** | 64.5% | 97.6% |
| **Race Detector (`-race`)** | Limpo | Limpo |
| **`go vet` / `go build`** | Limpo | Limpo |
| **golangci-lint — `errcheck`** | **6 violações** (retornos ignorados: `tx.Rollback`, `db.Exec` ×2, `.Scan` ×2, `db.Exec` em test) | 4 violações (não-críticas: `json.Encode`, `rand.Read`, `w.Write`, `json.Decode` em test) |
| **Complexidade Ciclomática máxima (`gocyclo`)** | **11** — `TransferService.Transfer` (monolito transacional) | 12 — `TransferUseCase.Execute` (orquestração com lock ordering; distribuído em camadas) |
| **Banco de dados** | SQLite (`modernc.org/sqlite`) | PostgreSQL (`lib/pq`) |
| **Arquitetura** | DDD simplificado | Clean Architecture (4 camadas) |
| **Controle de concorrência** | Não aplicado | `SELECT FOR UPDATE` + lock ordering |
| **Integridade transacional** | `sql.Tx` básico | `TxManager` via `context.Value` + ACID |
| **Observabilidade** | Não aplicada | Structured JSON logs (`log/slog`) + Correlation ID |

### Destaques da Rodada 1

- **Cobertura Cenário 1 apurada:** A cobertura de 64.5% revela que `main.go` não possui testes e que caminhos de erro em `Transfer` e `Debit`/`Credit` ficam descobertos.
- **`errcheck` Cenário 1 — risco transacional:** O retorno de `tx.Rollback()` não verificado em `service/transfer.go:31` representa um risco real: falhas silenciosas de rollback podem causar inconsistência de saldo sem qualquer log ou propagação de erro.
- **`errcheck` Cenário 2 — baixo impacto:** As 4 violações residem em código de infraestrutura de resposta HTTP (`json.Encode`, `w.Write`) e na geração de UUID (`rand.Read`) — padrões aceitáveis conforme convenção idiomática Go para handlers HTTP.

---

## 📊 Resultados SonarQube — Rodada 1 (2026-06-07)

> Análise executada via `sonar-scanner-cli` (Docker) contra instância local do SonarQube Community Edition. Os resultados foram capturados via API REST e persistidos em `sonar_output.json` em cada cenário.

| Métrica SonarQube | Cenário 1 — Vibecoding | Cenário 2 — Agente Engenheiro |
| :--- | :---: | :---: |
| **Cobertura (`coverage`)** | 72.7% | 87.7% |
| **Bugs** | 0 | 0 |
| **Vulnerabilidades** | 0 | 0 |
| **Code Smells** | 2 | 1 |
| **Violações totais** | 2 | 1 |
| **Complexidade Ciclomática** | 29 | 81 |
| **Complexidade Cognitiva** | 18 | 63 |
| **Dívida Técnica (`sqale_index`)** | 12 min | 10 min |

### Destaques SonarQube

- **Cobertura:** O Cenário 2 supera o Cenário 1 em 15 p.p. (87.7% vs. 72.7%), reflexo direto do conjunto de 56 testes com mocks estruturados.
- **Bugs e Vulnerabilidades:** Ambos os cenários atingiram zero bugs e zero vulnerabilidades detectadas pela análise estática do SonarQube.
- **Code Smells:** O Cenário 2 apresenta metade dos code smells do Cenário 1 (1 vs. 2), indicando código mais idiomático mesmo com maior volume de código.
- **Complexidade Ciclomática:** O valor mais alto no Cenário 2 (81 vs. 29) é esperado e saudável: reflete a distribuição da lógica em múltiplas camadas (Domain, Usecase, Infra, Delivery) em vez de um único monolito transacional.
- **Complexidade Cognitiva:** Analogamente, os 63 pontos do Cenário 2 frente aos 18 do Cenário 1 decorrem da riqueza arquitetural — mais funções, mais interfaces, mais contratos explícitos.
- **Dívida Técnica:** O Cenário 2 possui dívida técnica ligeiramente menor (10 min vs. 12 min), reforçando que a engenharia de prompts estruturada produz código de maior qualidade industrialmente mensurável.

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
│   ├── sonar_output.json         # Métricas SonarQube extraídas via API REST
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
│   ├── sonar_output.json         # Métricas SonarQube extraídas via API REST
│   └── trace_langfuse.json       # Log de auditoria: spans, tokens e timestamps
├── sync_traces.py                # Script Python para sincronização de traces com o Langfuse
├── Changelog.md                  # Histórico de alterações do projeto
└── README.md                     # Documentação científica do laboratório
```