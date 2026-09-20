# Seguranca, lint e SonarQube

Resultados locais: `20260912T233323Z`  
Resultados SonarQube: `20260913T010319Z`

| Run | Govuln | Gosec | Lint | Sonar cov. | Bugs | Vuln. | Hotspots | Smells | Duplicacao | Gate |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| chatgpt56sol-cenario-1 | 0 | 0 | 1 | 39.1% | 0 | 0 | 2 | 1 | 0.0% | OK |
| chatgpt56sol-cenario-2 | 0 | 0 | 2 | 74.5% | 0 | 0 | 0 | 2 | 0.0% | OK |
| gemini37flash-cenario-1 | 0 | 0 | 2 | 0.0% | 0 | 0 | 0 | 1 | 0.0% | OK |
| gemini37flash-cenario-2 | 0 | 0 | 4 | 86.9% | 0 | 0 | 0 | 2 | 5.4% | OK |
| grok46-cenario-1 | 0 | 1 | 4 | 67.5% | 0 | 0 | 0 | 1 | 0.0% | OK |
| grok46-cenario-2 | 0 | 0 | 9 | 81.5% | 0 | 0 | 0 | 1 | 0.0% | OK |
| kimik3-cenario-1 | 0 | 3 | 9 | 55.7% | 0 | 0 | 0 | 2 | 0.0% | OK |
| kimik3-cenario-2 | 0 | 0 | 6 | 97.6% | 0 | 1 | 0 | 2 | 0.0% | OK |
| sonnet5-cenario-1 | 0 | 2 | 1 | 0.0% | 0 | 0 | 0 | 1 | 0.0% | OK |
| sonnet5-cenario-2 | 0 | 0 | 7 | 97.4% | 0 | 0 | 2 | 3 | 0.0% | OK |

## Proveniencia

Os scanners locais e o SonarQube foram executados sobre os mesmos hashes de snapshot. A consolidacao usa a rodada local completa porque a rodada de upload Sonar concorrente nao preservou todos os JSONs locais. Nenhum valor ausente foi convertido em zero.
