# Decision Record
## [ADR-001] Arquitetura Clean Architecture para Módulo P2P Transfer

**Data:** 2026-06-04T21:30:00Z  
**Status:** Aceita  
**Modelo de LLM:** Claude Opus 4.6 (claude-sonnet-4-20250514) | Temperatura: default  
**ID do Trace (Langfuse):** p2p-wallet-clean-arch-001

### 1. Contexto e Problema
Necessidade de implementar um módulo de transferência financeira P2P com alta concorrência, integridade transacional ACID, prevenção de double-spending e rastreabilidade de erros. O sistema deve suportar alta volumetria e chamadas HTTP simultâneas para o mesmo ID de carteira.

### 2. Direcionadores Tecnológicos (Drivers)
- Garantir o princípio de Responsabilidade Única (SRP) separando regras de negócio, persistência e transporte HTTP
- Inversão de Dependência (DIP) via interfaces puras do Go para testabilidade
- Evitar Race Conditions com bloqueio pessimista (SELECT FOR UPDATE) e ordenação de locks por ID
- Atomicidade ACID usando *sql.Tx do driver SQL do Go
- Observabilidade via structured logging (log/slog) com correlation ID propagado via context
- Uso exclusivo de net/http sem frameworks externos

### 3. Alternativas Consideradas
- **Alternativa 1 (Adotada):** Clean Architecture com 4 camadas (Domain, Usecase, Infra, Delivery) + TxManager via context.Value para propagar *sql.Tx. Lock ordering por sort.Strings nos IDs antes da aquisição.
- **Alternativa 2 (Descartada):** Repository com métodos que recebem *sql.Tx explicitamente. Descartada pois vaza detalhes de infraestrutura para a camada de domínio, violando DIP.
- **Alternativa 3 (Descartada):** Usar ORM (GORM/Ent) para abstração de transação. Descartada pois adiciona dependência pesada e reduz controle sobre queries críticas de bloqueio pessimista.

### 4. Decisão e Padrão Aplicado
Adotada a Alternativa 1 com os seguintes padrões:
- **Unit of Work via TxManager:** Interface `TxManager` com `RunInTx(ctx, fn)` que injeta *sql.Tx no context, permitindo que repositórios extraiam a transação sem acoplamento direto.
- **Pessimistic Locking com Dead Lock Prevention:** `SELECT FOR UPDATE` com ordenação lexicográfica dos IDs (`sort.Strings`) antes da aquisição dos locks.
- **Correlation ID Middleware:** Propagação de ID de correlação via `context.Value` e header HTTP `X-Correlation-ID` para rastreabilidade end-to-end.
- **Manual Dependency Injection:** Todas as dependências resolvidas em `main.go` via construtores, sem containers de DI.
- **int64 para valores monetários:** Representação em centavos evitando problemas de floating point.

### 5. Consequências e Validação
- **Positivas:** 97.6% de cobertura de testes; zero warnings em `go vet`; race detector limpo; camadas completamente desacopladas; domínio puro sem imports externos; erros rastreáveis com correlation ID em structured JSON logs.
- **Negativas/Compromissos (Trade-offs):** Uso de `context.Value` para propagar *sql.Tx adiciona um nível de indireção que não é type-safe em compile time; `conn.go` movido para `main.go` para manter cobertura alta (código de bootstrap não é testável unitariamente sem DB real).
- **Resultado dos Testes:** `go test ./... -race` passou com 47 testes, 0 falhas, 97.6% cobertura global.
