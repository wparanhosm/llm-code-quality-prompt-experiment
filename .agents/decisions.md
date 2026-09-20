# Instrução de Sistema: Decisoes sobre o projeto

Todas as mudanças que você fizer no projeto que considere ser significativas, registre no arquivo decisions.md na raiz do projeto seguindo a estrutura:

# Decision Record
## [ADR-0XX] Título da Decisão

**Data:** [Timestamp]  
**Status:** [Proposta | Aceita | Rejeitada | Substituída por ADR-XXX]  
**Modelo de LLM:** [Ex: Gemini-1.5-Pro-002 | Temperatura: 0.2]  
**Execução OpenCode:** [Modelo, cenário e custo retornado pelo OpenCode]

### 1. Contexto e Problema
[Descreva o problema de consistência, bug de concorrência em Go ou desvio arquitetural que a LLM encontrou no código]

### 2. Direcionadores Tecnológicos (Drivers)
- [Ex: Garantir o princípio de Responsabilidade Única (SRP)]
- [Ex: Reduzir a latência / Evitar Race Conditions no Go runtime]

### 3. Alternativas Consideradas
- **Alternativa 1:** [Descrição da abordagem e por que a LLM gerou ou falhou nela]
- **Alternativa 2:** [Segunda abordagem gerada no Self-Correction Loop]

### 4. Decisão e Padrão Aplicado
[Decisão tomada pela IA/Usuário. Especifique o padrão de projeto ou padrão de prompt estruturado que resolveu o problema]

### 5. Consequências e Validação
- **Positivas:** [O que melhorou na qualidade do código?]
- **Negativas/Compromissos (Trade-offs):** [Houve aumento na complexidade do código? Maior consumo de tokens?]
- **Resultado dos Testes:** [Ex: go test ./... passou com 100% de sucesso]

## [ADR-002] Matriz configurável de modelos e cenários com análise local automatizada

**Data:** 2026-09-06
**Status:** Aceita
**Modelo de LLM:** OpenCode configurável por execução
**Execução OpenCode:** Modelo, cenário e custo registrados em `experiments.json`

### 1. Contexto e Problema
O experimento comparava somente uma execução por cenário e dependia do Langfuse para observabilidade e cálculo de custo. Essa dependência não é necessária quando o OpenCode fornece o custo nativamente e limita a repetição da análise para vários modelos.

### 2. Direcionadores Tecnológicos (Drivers)
- Eliminar a dependência operacional do Langfuse.
- Permitir a combinação de múltiplos modelos e cenários sem alterar o código de análise.
- Produzir métricas SonarQube e cobertura de forma reprodutível em ambiente local.
- Manter o custo por execução junto aos demais dados comparativos.

### 3. Alternativas Consideradas
- **Alternativa 1:** Manter diretórios e comandos manuais por cenário, com consolidação manual dos valores.
- **Alternativa 2:** Usar uma configuração única de experimentos e um script Python sem dependências externas para executar, extrair e consolidar as métricas.

### 4. Decisão e Padrão Aplicado
Adotar `experiments.json` como matriz declarativa de modelos, cenários e execuções. O repositório define quatro modelos iniciais e dois cenários, enquanto novas execuções são cadastradas com seu diretório e custo nativo do OpenCode. O `docker-compose.yaml` provisiona PostgreSQL e SonarQube; `scripts/analyze_experiments.py` executa os testes, aciona o scanner, consulta a API e gera o relatório consolidado.

### 5. Consequências e Validação
- **Positivas:** Sem Langfuse, análise local reproduzível, suporte extensível à matriz de experimentos e consolidação automática das métricas.
- **Negativas/Compromissos (Trade-offs):** O token do SonarQube deve estar disponível na sessão e cada nova execução precisa ser cadastrada antes da análise.
- **Resultado dos Testes:** `docker compose config`, sintaxe do script e JSON validados; testes Go dos dois cenários passaram. PostgreSQL ficou saudável e o endpoint do SonarQube respondeu com estado `UP`.
