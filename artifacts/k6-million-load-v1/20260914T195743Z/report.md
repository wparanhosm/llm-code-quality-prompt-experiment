# Benchmark k6-million-load-v1

Execucao: `20260914T195743Z`

| Run | Status | Requisicoes | Sucessos HTTP | Falhas HTTP | Duracao | RPS | Media | p95 | p99 | Ledger | Reconciliacao exata | Conservacao |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| chatgpt56sol-cenario-1 | PASSED | 1000000 | 1000000 | 0 | 9m 35s | 1740.17 | 28.47 ms | 50.65 ms | 76.59 ms | 1000000 | PASS | PASS |
| chatgpt56sol-cenario-2 | PASSED | 1000000 | 1000000 | 0 | 11m 06s | 1503.70 | 32.71 ms | 71.30 ms | 119.73 ms | 1000000 | PASS | PASS |
| gemini37flash-cenario-2 | PASSED | 1000000 | 1000000 | 0 | 10m 58s | 1521.97 | 32.47 ms | 63.74 ms | 84.12 ms | 1000000 | PASS | PASS |
| grok46-cenario-2 | FAILED | 1000000 | 999937 | 63 | 10m 45s | 1553.38 | 31.80 ms | 62.74 ms | 83.46 ms | 999937 | FAIL | PASS |
| kimik3-cenario-1 | PASSED | 1000000 | 1000000 | 0 | 3h 09m 35s | 87.92 | 568.40 ms | 1686.97 ms | 2624.73 ms | 1000000 | PASS | PASS |
| kimik3-cenario-2 | PASSED | 1000000 | 1000000 | 0 | 10m 15s | 1627.65 | 30.16 ms | 59.00 ms | 105.42 ms | 1000000 | PASS | PASS |
| sonnet5-cenario-2 | PASSED | 1000000 | 1000000 | 0 | 9m 23s | 1779.22 | 27.53 ms | 73.28 ms | 127.83 ms | 1000000 | PASS | PASS |

Resultado: 6 backends aprovados e 1 reprovado.
- `grok46-cenario-2`: 63 respostas inesperadas; 999937 sucessos HTTP e 999937 registros no ledger.
