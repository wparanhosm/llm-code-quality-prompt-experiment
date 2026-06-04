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

## ⚙️ Parâmetros de Reprodutibilidade (Fixos)

As chamadas de API registradas no Langfuse seguem estritamente a seguinte configuração para anular a estocasticidade natural dos modelos de linguagem:

*   **Temperatura:** `0.0` (Determinismo Máximo)
*   **Top_P:** `1.0`
*   **Modelos Homologados:** `gpt-4o` / `claude-3-5-sonnet` (Mantido o mesmo modelo entre os cenários em cada rodada de teste).

---

## 📁 Estrutura do Repositório

```text
├── .github/                  # Workflows de automação do SonarCloud
├── cenario-1-vibecoding/      # Artefatos gerados sem guias arquiteturais
│   ├── ...
│   └── trace_langfuse.json   # Log de auditoria da IA
├── cenario-2-agente-engenheiro/ # Artefatos gerados com prompt estruturado
│   ├── ...
│   └── trace_langfuse.json   # Log de auditoria da IA
├── .golangci-lint.yml        # Configuração ultra-rigorosa dos analisadores de Go
└── README.md                 # Documentação científica do laboratório