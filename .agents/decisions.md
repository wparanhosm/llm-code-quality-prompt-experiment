# Instrução de Sistema: Decisoes sobre o projeto

Todas as mudanças que você fizer no projeto que considere ser significativas, registre no arquivo decisions.md na raiz do projeto seguindo a estrutura:

# Decision Record
## [ADR-0XX] Título da Decisão

**Data:** [Timestamp]  
**Status:** [Proposta | Aceita | Rejeitada | Substituída por ADR-XXX]  
**Modelo de LLM:** [Ex: Gemini-1.5-Pro-002 | Temperatura: 0.2]  
**ID do Trace (Langfuse):** [Cole o traceId correspondente aqui]

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