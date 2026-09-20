# Benchmark concurrency-stability-v1

Execucao: `20260914T161458Z`

| Run | Rodadas | Falhas | 5xx | Timeouts | Erros cliente | Latencia media/rodada | p95 | Elegivel K6 |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| chatgpt56sol-cenario-1 | 30/30 | 0 | 0 | 0 | 0 | 15.418 ms | 19.39 ms | SIM |
| chatgpt56sol-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 19.978 ms | 27.064 ms | SIM |
| gemini37flash-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 24.099 ms | 33.507 ms | SIM |
| grok46-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 21.594 ms | 25.954 ms | SIM |
| kimik3-cenario-1 | 30/30 | 0 | 0 | 0 | 0 | 58.073 ms | 71.768 ms | SIM |
| kimik3-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 29.813 ms | 37.783 ms | SIM |
| sonnet5-cenario-1 | 30/30 | 60 | 540 | 0 | 0 | 11.635 ms | 15.749 ms | NAO |
| sonnet5-cenario-2 | 30/30 | 0 | 0 | 0 | 0 | 17.2 ms | 23.562 ms | SIM |
| gemini37flash-cenario-1 | 0/30 | - | - | - | - | - | - | NAO (sem HTTP) |
| grok46-cenario-1 | 0/30 | - | - | - | - | - | - | NAO (sem HTTP) |
