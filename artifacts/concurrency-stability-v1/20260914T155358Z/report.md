# Benchmark concurrency-stability-v1

Execucao: `20260914T155358Z`

| Run | Rodadas | Falhas | 5xx | Timeouts | Erros cliente | Latencia media/rodada | p95 | Elegivel K6 |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| chatgpt56sol-cenario-1 | 30/30 | 0 | 0 | 0 | 0 | 15.224 ms | 17.535 ms | SIM |
| chatgpt56sol-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 19.561 ms | 22.226 ms | SIM |
| gemini37flash-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 23.21 ms | 29.001 ms | SIM |
| grok46-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 21.432 ms | 24.922 ms | SIM |
| kimik3-cenario-1 | 30/30 | 60 | 0 | 0 | 0 | 27.183 ms | 57.469 ms | NAO |
| kimik3-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 29.089 ms | 38.626 ms | SIM |
| sonnet5-cenario-1 | 30/30 | 60 | 540 | 0 | 0 | 10.738 ms | 12.355 ms | NAO |
| sonnet5-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 16.8 ms | 20.331 ms | SIM |
| gemini37flash-cenario-1 | 0/30 | - | - | - | - | - | - | NAO (sem HTTP) |
| grok46-cenario-1 | 0/30 | - | - | - | - | - | - | NAO (sem HTTP) |
