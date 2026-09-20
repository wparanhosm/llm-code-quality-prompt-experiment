# Avaliacao arquitetural

Escala por dimensao: 0 a 4. Score ponderado: 0 a 100. TDD e reportado separadamente.

| Run | Clean | DDD | SOLID | Testab. | Testes | Transacao | Concorr. | Observ. | Operacao | Simplic. | Score | Faixa |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| chatgpt56sol-cenario-1 | 2 | 1 | 2 | 2 | 2 | 3 | 3 | 1 | 3 | 4 | 58.50 | fragil |
| chatgpt56sol-cenario-2 | 4 | 3 | 3 | 4 | 3 | 4 | 3 | 3 | 2 | 3 | 82.75 | adequado |
| gemini37flash-cenario-1 | 0 | 1 | 1 | 1 | 0 | 3 | 1 | 1 | 1 | 3 | 29.50 | inadequado |
| gemini37flash-cenario-2 | 3 | 3 | 3 | 3 | 3 | 3 | 3 | 3 | 1 | 2 | 70.50 | parcialmente adequado |
| grok46-cenario-1 | 1 | 1 | 1 | 2 | 1 | 3 | 2 | 1 | 2 | 4 | 44.75 | fragil |
| grok46-cenario-2 | 3 | 3 | 3 | 4 | 3 | 4 | 3 | 2 | 2 | 3 | 78.25 | adequado |
| kimik3-cenario-1 | 1 | 1 | 1 | 2 | 3 | 4 | 2 | 1 | 1 | 4 | 53.00 | fragil |
| kimik3-cenario-2 | 4 | 3 | 4 | 4 | 4 | 4 | 4 | 3 | 3 | 3 | 93.50 | robusto |
| sonnet5-cenario-1 | 2 | 2 | 2 | 1 | 0 | 4 | 2 | 1 | 2 | 3 | 49.00 | fragil |
| sonnet5-cenario-2 | 4 | 3 | 4 | 4 | 3 | 4 | 3 | 2 | 2 | 2 | 82.25 | adequado |

## Diferencas Pareadas

| Modelo | Cenario 1 | Cenario 2 | Delta C2-C1 |
|---|---:|---:|---:|
| chatgpt56sol | 58.50 | 82.75 | +24.25 |
| gemini37flash | 29.50 | 70.50 | +41.00 |
| grok46 | 44.75 | 78.25 | +33.50 |
| kimik3 | 53.00 | 93.50 | +40.50 |
| sonnet5 | 49.00 | 82.25 | +33.25 |

## TDD

Nenhum snapshot fornece historico red-green-refactor verificavel. A presenca de testes e cobertura nao foi tratada como prova de TDD.

## Riscos Criticos

- `chatgpt56sol-cenario-1` [high]: Retries podem duplicar transferencias por ausencia de idempotencia.
- `chatgpt56sol-cenario-2` [high]: Ausencia de idempotencia permite duplicacao em retry ambiguo.
- `gemini37flash-cenario-1` [critical]: INSERT OR REPLACE redefine saldos em toda execucao.
- `gemini37flash-cenario-1` [high]: Interface fixa impede validar requisitos negativos e concorrencia.
- `gemini37flash-cenario-2` [critical]: Snapshot nao fornece schema ou migration e exige reconstrucao externa.
- `gemini37flash-cenario-2` [critical]: Money.Add nao verifica overflow de int64.
- `grok46-cenario-1` [high]: Sem idempotencia e com serializacao limitada ao processo.
- `grok46-cenario-2` [high]: Gerador denominado uuidGen concatena relogios e pode colidir sob carga.
- `kimik3-cenario-1` [high]: Pool de uma conexao inviabiliza throughput concorrente.
- `kimik3-cenario-1` [medium]: API monetaria usa float64.
- `kimik3-cenario-2` [high]: Senha PostgreSQL padrao embutida no codigo.
- `sonnet5-cenario-1` [high]: Falhas SQLSTATE 40001 viram HTTP 500 sem retry, rejeitando transferencias ainda cobertas pelo saldo.
- `sonnet5-cenario-1` [high]: Nenhum teste foi produzido.
- `sonnet5-cenario-2` [critical]: Dockerfile declara Go 1.23 para modulo que exige Go 1.26.
- `sonnet5-cenario-2` [medium]: Sem idempotencia para retries ambiguos.

## Metodo

As notas usam inspecao de codigo ligada a arquivo e linha, mais evidencias independentes de build, testes, seguranca, smoke funcional e concorrencia. Nomes de camadas e alegacoes em README nao geram credito isoladamente. O JSON de revisoes preserva justificativa e evidencias de cada nota.
