# Documentacao do Benchmark de Qualidade de Codigo Gerado por LLMs

## 1. Visao Geral

Este repositorio implementa um laboratorio experimental para comparar codigo Go gerado por modelos de linguagem em dois cenarios de prompt aplicados ao mesmo dominio: transferencia financeira P2P entre carteiras digitais.

O objetivo nao e avaliar apenas se o codigo compila ou possui cobertura. O protocolo combina evidencias de:

- qualidade basica do codigo;
- testes produzidos pelo modelo;
- cobertura;
- seguranca e vulnerabilidades;
- analise SonarQube;
- funcionamento real da transferencia;
- integridade contabil e transacional;
- comportamento sob concorrencia;
- esforco para executar a solucao;
- Clean Architecture, DDD, SOLID e testabilidade;
- custo, tokens e tempo de geracao;
- estabilidade antes de uma carga de um milhao de requisicoes.

Os cinco modelos avaliados sao:

- `chatgpt56sol`;
- `gemini37flash`;
- `grok46`;
- `kimik3`;
- `sonnet5`.

Cada modelo possui uma execucao em cada cenario, totalizando dez snapshots.

### 1.1 Matriz selada

| Run | Modelo | Cenario | Arquivos | Hash raiz SHA-256 |
|---|---|---:|---:|---|
| `chatgpt56sol-cenario-1` | chatgpt56sol | 1 | 14 | `856ecf6d3afb9a97b52c05a47c5e148a5f97d57f42924b26edafc1608d90255b` |
| `chatgpt56sol-cenario-2` | chatgpt56sol | 2 | 3.685 | `06ac94ee765dd44f022c6d0e1b0a6e81f717333252a70a47598380a0ea464a2c` |
| `gemini37flash-cenario-1` | gemini37flash | 1 | 5 | `7907395bdc1bb47cb4ac22bfd643b8960802dd641fbaf439abd4e2db62045c78` |
| `gemini37flash-cenario-2` | gemini37flash | 2 | 3.681 | `d898fe9810beabadd3871fdd34db981c58dc5bace4e38fcd5d43b7d18efca706` |
| `grok46-cenario-1` | grok46 | 1 | 7 | `c72eb796ffc762bed3da4759343223778079bd7b8aab99ea75cb41543ab95e6b` |
| `grok46-cenario-2` | grok46 | 2 | 3.688 | `a883949e52665e8f1788fc0b8b042682d0c93ecf28fc1348fbbf3c3dc225e6cd` |
| `kimik3-cenario-1` | kimik3 | 1 | 7 | `d9ace4c75bb83c4ba8f3fbcead1c3f554cb7a1bde706b695fb1c799cfde54eb9` |
| `kimik3-cenario-2` | kimik3 | 2 | 3.466 | `fec80cafbf1a2f828a1c4995b00299ec59abc94f19b11491976286b8068a41e8` |
| `sonnet5-cenario-1` | sonnet5 | 1 | 9 | `b8af98fc5ee5ea507276315a2d925dd6b314326fdd6298af2066d059ec295aa5` |
| `sonnet5-cenario-2` | sonnet5 | 2 | 40 | `83dbd6d1ad23d5635f697d844bb09acdc1480cd2fdab2c068ef1657f1bb33337` |

Os hashes e as contagens acima sao derivados de `provenance/classification-summary.json`; os manifestos por arquivo continuam sendo a fonte detalhada.

## 2. Cenarios Experimentais

### 2.1 Cenario 1: prompt funcional

O primeiro cenario descreve principalmente o resultado funcional esperado. Ele solicita uma transferencia entre carteiras com validacao de saldo, debito, credito e persistencia, sem prescrever detalhadamente arquitetura, observabilidade, estrategia de concorrencia ou processo de testes.

Esse tratamento procura representar uma solicitacao orientada a entrega rapida, frequentemente associada ao termo "vibecoding".

### 2.2 Cenario 2: prompt estruturado de engenharia

O segundo cenario adiciona restricoes explicitas de engenharia:

- Clean Architecture;
- principios SOLID;
- separacao entre dominio, aplicacao, infraestrutura e delivery;
- inversao de dependencia;
- injecao manual de dependencias;
- API HTTP com `net/http`;
- transacao ACID;
- locks pessimistas com `SELECT FOR UPDATE`;
- ordenacao de locks;
- observabilidade;
- testes e meta de cobertura.

### 2.3 Interpretacao correta da comparacao

Os cenarios nao diferem apenas na forma textual. O cenario 2 tambem explicita requisitos tecnicos ausentes no cenario 1. Portanto, os resultados medem o efeito do conjunto de instrucoes estruturadas, e nao apenas o efeito da formatacao do prompt.

A principal comparacao e pareada por modelo:

```text
resultado do modelo no cenario 2 - resultado do mesmo modelo no cenario 1
```

Isso reduz o efeito da capacidade intrinseca de cada modelo.

## 3. Organizacao do Repositorio

```text
runs/                           snapshots originais e imutaveis
provenance/                     hashes, manifestos e classificacao dos snapshots
benchmark/
  campaigns/                    configuracoes versionadas das campanhas
  architecture/                 rubrica e revisoes arquiteturais
  functional/sql/               schemas e seeds externos do avaliador
  schemas/                      JSON Schemas
scripts/                        executores e consolidadores
work/                           copias temporarias e descartaveis
cache/                          caches de ferramentas, sem valor probatorio
artifacts/                      resultados produzidos pelo benchmark
```

### 3.1 Separacao entre geracao e avaliacao

O diretorio `runs/` contem exatamente o material recebido dos modelos. Ele pode incluir, alem do codigo:

- testes;
- Dockerfiles e arquivos Compose;
- Makefiles;
- migrations;
- documentacao;
- billing;
- binarios preexistentes;
- bancos SQLite preexistentes;
- `coverage.out` preexistente;
- dependencias instaladas em `node_modules`.

Esses itens nao sao removidos. Eles sao classificados conforme sua origem para impedir que um artefato preexistente seja confundido com um resultado produzido pelo benchmark.

As categorias de classificacao incluem:

| Categoria | Significado |
|---|---|
| `application_source` | Codigo de producao produzido pelo modelo |
| `generated_test` | Testes produzidos pelo modelo |
| `dependency_manifest` | Manifestos e locks de dependencias |
| `database_definition` | Schema, migration ou seed fornecido |
| `model_operational_config` | Dockerfile, Compose, Makefile e scripts operacionais |
| `generation_context` | Contexto e configuracao do agente gerador |
| `documentation` | Documentacao produzida pelo modelo |
| `generation_metadata` | Billing, tokens, tempo e custo |
| `generation_artifact` | Binarios, coverage, bancos e dependencias instaladas preexistentes |
| `benchmark_artifact` | Reservado aos resultados produzidos pelo avaliador |

## 4. Integridade e Proveniencia

### 4.1 Snapshot imutavel

Nenhuma ferramenta de avaliacao deve executar diretamente em `runs/`. Antes de uma campanha, o snapshot e copiado para:

```text
work/<execution-id>/<run-id>/source
```

Builds, testes, bancos SQLite e scanners operam nessa copia. Resultados sao gravados em `artifacts/`.

### 4.2 Manifestos SHA-256

Cada run possui em `provenance/snapshots/<run-id>/`:

- `source-manifest.json`: path, tipo, tamanho, modo e SHA-256 de cada entrada;
- `classification.json`: classificacao de cada arquivo;
- `snapshot.sha256`: hash raiz do snapshot.

O hash raiz e calculado sobre uma representacao canonica e ordenada do manifesto. Datas de modificacao nao participam do hash, evitando divergencias causadas apenas por copia de arquivos.

### 4.3 Verificacao antes e depois

Os executores de qualidade, seguranca, smoke, robustez e estabilidade executam a verificacao de integridade antes e depois da avaliacao:

```bash
python3 scripts/classify_snapshots.py --verify
```

A campanha falha se qualquer byte protegido divergir do manifesto selado.

O gerador do relatorio arquitetural nao executa codigo dos snapshots: ele compara os hashes registrados nas revisoes com `provenance/classification-summary.json` antes de calcular as notas. Para uma reproducao completa, o comando de verificacao tambem deve ser executado antes e depois dessa geracao.

### 4.4 Estado atual

Os dez snapshots foram classificados e verificados. Nenhum arquivo permaneceu na categoria `unknown`, e nenhuma campanha alterou os hashes registrados.

## 5. Ambientes e Isolamento

### 5.1 Go

A configuracao da campanha registra, por run, versoes Go declaradas entre 1.22 e 1.26. A rodada canonica nao gravou no resumo a saida de `go version`, o sistema operacional ou a arquitetura efetivamente usados; por isso, esses dados nao devem ser inferidos retroativamente.

Os caches sao externos aos snapshots:

```text
cache/go-build/
cache/go-mod/
```

### 5.2 PostgreSQL

Nos testes funcionais, cada run PostgreSQL recebe um container efemero exclusivo, publicado apenas em loopback e sem volume persistente. O ciclo de vida e:

1. criar container;
2. aguardar `pg_isready`;
3. aplicar migration;
4. aplicar seed controlado;
5. iniciar aplicacao;
6. executar casos;
7. reconciliar dados diretamente no banco;
8. encerrar aplicacao;
9. remover container.

Isso evita contaminacao entre modelos e entre casos.

### 5.3 SQLite

Aplicacoes SQLite sao executadas em uma copia temporaria ou recebem um path externo para o banco. Arquivos como `pix.db` nunca sao criados dentro de `runs/`.

### 5.4 SonarQube

O SonarQube local utiliza PostgreSQL proprio. Cada analise recebe uma chave exclusiva:

```text
lqcpe-<modelo>-<cenario>-<execution-id>
```

O scanner analisa a copia temporaria em modo somente leitura e grava `.scannerwork` em `artifacts/`.

Tokens e senhas do SonarQube nao fazem parte deste protocolo nem dos relatorios. Devem ser fornecidos apenas por variavel de ambiente, nunca versionados; credenciais usadas durante campanhas anteriores devem ser rotacionadas.

## 6. Campanhas Executadas

### 6.1 Qualidade inicial

Configuracao:

```text
benchmark/campaigns/initial-quality.json
```

Executor:

```text
scripts/run_benchmark.py
```

Comandos uniformes:

```bash
gofmt -l .
go build ./...
go vet ./...
go test -json -count=1 ./...
go test -race -count=1 ./...
go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=<artifact> ./...
```

O race detector so possui valor para os caminhos efetivamente exercitados. Um resultado limpo com zero testes nao demonstra ausencia de problemas concorrentes.

### Resultado

| Run | Build | Vet | Testes produzidos | Race | Cobertura Go | Tokens | Custo USD |
|---|---|---|---:|---|---:|---:|---:|
| ChatGPT C1 | PASS | PASS | 18/18 | PASS | 40,8% | 22.535 | 0,45 |
| ChatGPT C2 | PASS | PASS | 55/55 | PASS | 74,9% | 16.930 | 0,23 |
| Gemini C1 | PASS | PASS | 0 | PASS | 0,0% | 9.225 | 0,03 |
| Gemini C2 | PASS | PASS | 41/41 | PASS | 83,3% | 66.500 | 0,32 |
| Grok C1 | PASS | PASS | 1/1 | PASS | 53,1% | 10.637 | 0,06 |
| Grok C2 | PASS | PASS | 45/45 | PASS | 79,1% | 30.812 | 0,37 |
| Kimi C1 | PASS | PASS | 10/10 | PASS | 51,3% | 18.057 | 0,26 |
| Kimi C2 | PASS | PASS | 108/108 | PASS | 96,7% | 67.640 | 1,90 |
| Sonnet C1 | PASS | PASS | 0 | PASS | 0,0% | 23.337 | 0,08 |
| Sonnet C2 | PASS | PASS | 118/118 | PASS | 96,7% | 138.064 | 2,49 |

Resultado canonico:

```text
artifacts/initial-quality-v1/20260907T002744Z/
```

### 6.2 Seguranca, lint e SonarQube

Configuracao:

```text
benchmark/campaigns/security-sonar.json
```

Ferramentas:

- `govulncheck` v1.1.4;
- `gosec` v2.29.0;
- `golangci-lint` v2.13.2;
- SonarScanner executado pela imagem imutavel identificada por digest em `benchmark/campaigns/security-sonar.json`;
- SonarQube Community Build 26.6.0.123539.

### Interpretacao das ferramentas

`govulncheck` procura vulnerabilidades conhecidas alcancaveis no grafo Go. `gosec` encontra padroes potencialmente inseguros. SonarQube adiciona bugs, vulnerabilidades, hotspots, smells, duplicacao, cobertura e divida tecnica. Nenhuma ferramenta isolada prova seguranca.

### Resultado consolidado

| Run | Govuln | Gosec | Lint | Sonar cov. | Bugs | Vulnerabilidades | Hotspots | Smells | Duplicacao | Gate |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| ChatGPT C1 | 0 | 0 | 1 | 39,1% | 0 | 0 | 2 | 1 | 0,0% | OK |
| ChatGPT C2 | 0 | 0 | 2 | 74,5% | 0 | 0 | 0 | 2 | 0,0% | OK |
| Gemini C1 | 0 | 0 | 2 | 0,0% | 0 | 0 | 0 | 1 | 0,0% | OK |
| Gemini C2 | 0 | 0 | 4 | 86,9% | 0 | 0 | 0 | 2 | 5,4% | OK |
| Grok C1 | 0 | 1 | 4 | 67,5% | 0 | 0 | 0 | 1 | 0,0% | OK |
| Grok C2 | 0 | 0 | 9 | 81,5% | 0 | 0 | 0 | 1 | 0,0% | OK |
| Kimi C1 | 0 | 3 | 9 | 55,7% | 0 | 0 | 0 | 2 | 0,0% | OK |
| Kimi C2 | 0 | 0 | 6 | 97,6% | 0 | 1 | 0 | 2 | 0,0% | OK |
| Sonnet C1 | 0 | 2 | 1 | 0,0% | 0 | 0 | 0 | 1 | 0,0% | OK |
| Sonnet C2 | 0 | 0 | 7 | 97,4% | 0 | 0 | 2 | 3 | 0,0% | OK |

Achados relevantes:

- Kimi C2 possui senha PostgreSQL padrao embutida, classificada como vulnerabilidade blocker pelo Sonar;
- Sonnet C1 possui credencial em URL detectada pelo `gosec` como `G101` de severidade alta;
- Kimi C1 usa servidor HTTP sem timeouts, detectado como `G114`;
- a maioria dos achados de lint envolve erros ignorados em `Close`, `Rollback` e escrita HTTP;
- o Quality Gate padrao ficou `OK` mesmo com vulnerabilidade blocker em Kimi C2, portanto nao deve ser usado sozinho como criterio de aprovacao.

O consolidado combina a rodada local completa `20260912T233323Z` e a rodada Sonar `20260913T010319Z`. Os hashes dos snapshots sao iguais entre as duas rodadas. Valores ausentes nao foram convertidos em zero.

Resultado canonico:

```text
artifacts/security-sonar-v1/20260913T010319Z/consolidated-report.md
```

### 6.3 Smoke test funcional

Configuracao:

```text
benchmark/campaigns/functional-smoke.json
```

Executor:

```text
scripts/run_functional_benchmark.py
```

O smoke test nao confia apenas na resposta HTTP. Apos uma transferencia de sucesso, o avaliador consulta diretamente o banco e valida:

```text
saldo_origem_final = saldo_origem_inicial - valor
saldo_destino_final = saldo_destino_inicial + valor
soma_final = soma_inicial
quantidade_de_registros_correspondentes = 1
nenhum_saldo_negativo = verdadeiro
```

### Resultado

Todos os dez snapshots passaram no caminho de sucesso. Isso inclui as duas CLIs e os oito backends HTTP.

| Run | Interface | Status observado | Debito | Credito | Conservacao | Ledger |
|---|---|---:|---|---|---|---|
| ChatGPT C1 | HTTP | 201 | PASS | PASS | PASS | PASS |
| ChatGPT C2 | HTTP | 201 | PASS | PASS | PASS | PASS |
| Gemini C1 | One-shot | exit 0 | PASS | PASS | PASS | PASS |
| Gemini C2 | HTTP | 201 | PASS | PASS | PASS | PASS |
| Grok C1 | CLI | exit 0 | PASS | PASS | PASS | PASS |
| Grok C2 | HTTP | 201 | PASS | PASS | PASS | PASS |
| Kimi C1 | HTTP | 200 | PASS | PASS | PASS | PASS |
| Kimi C2 | HTTP | 201 | PASS | PASS | PASS | PASS |
| Sonnet C1 | HTTP | 200 | PASS | PASS | PASS | PASS |
| Sonnet C2 | HTTP | 200 | PASS | PASS | PASS | PASS |

Resultado canonico:

```text
artifacts/functional-smoke-v1/20260913T015119Z/
```

### 6.4 Casos negativos e robustez

Configuracao:

```text
benchmark/campaigns/functional-robustness.json
```

Executor:

```text
scripts/run_functional_robustness.py
```

Cada caso com erro exige simultaneamente:

- resposta ou exit code de rejeicao esperado;
- saldos inalterados;
- soma total inalterada;
- nenhum novo registro no ledger.

Casos:

- mesma origem e destino;
- valor zero;
- valor negativo;
- carteira inexistente;
- saldo insuficiente;
- dez operacoes concorrentes.

### Resultado

| Run | Mesma conta | Zero | Negativo | Inexistente | Sem saldo | Concorrencia |
|---|---|---|---|---|---|---|
| ChatGPT C1 | PASS | PASS | PASS | PASS | PASS | PASS |
| ChatGPT C2 | PASS | PASS | PASS | PASS | PASS | PASS |
| Gemini C1 | Nao suportado | Nao suportado | Nao suportado | Nao suportado | Nao suportado | Nao suportado |
| Gemini C2 | PASS | PASS | PASS | PASS | PASS | PASS |
| Grok C1 | PASS | PASS | PASS | PASS | PASS | PASS |
| Grok C2 | PASS | PASS | PASS | PASS | PASS | PASS |
| Kimi C1 | PASS | PASS | PASS | PASS | PASS | PASS |
| Kimi C2 | PASS | PASS | PASS | PASS | PASS | PASS |
| Sonnet C1 | PASS | PASS | PASS | PASS | PASS | FAIL |
| Sonnet C2 | PASS | PASS | PASS | PASS | PASS | PASS |

Gemini C1 e marcado como nao suportado porque sua CLI executa uma unica transferencia fixa e nao aceita origem, destino ou valor.

Sonnet C1 preservou dinheiro e ledger, mas rejeitou operacoes ainda validas com HTTP 500 devido a `SQLSTATE 40001`. A implementacao usa isolamento serializavel sem retry.

Resultado canonico:

```text
artifacts/functional-robustness-v1/20260913T022017Z/
```

### 6.5 Estabilidade concorrente repetida

Configuracao:

```text
benchmark/campaigns/concurrency-stability.json
```

Executor:

```text
scripts/run_concurrency_stability.py
```

O protocolo executa 30 repeticoes de dois testes por backend HTTP:

1. dez transferencias concorrentes sobre a mesma origem;
2. dez transferencias inversas simultaneas entre A e B.

Uma barreira sincroniza o inicio das requisicoes. Apos cada rodada, o banco e reconciliado independentemente.

Cada backend recebe 600 requisicoes:

```text
30 repeticoes x 2 grupos x 10 requisicoes = 600
```

### Criterios de aprovacao por rodada

- quantidade de sucessos limitada pelo saldo;
- nenhum HTTP 5xx;
- nenhum timeout;
- nenhuma falha de transporte;
- saldo final correspondente ao numero de sucessos;
- soma total preservada;
- ledger correspondente aos sucessos;
- nenhum saldo negativo;
- transferencias inversas sem deadlock e com saldo final esperado.

### Resultado

| Run | Rodadas | Falhas | HTTP 5xx | Timeouts | p95 | Elegivel para K6 |
|---|---:|---:|---:|---:|---:|---|
| ChatGPT C1 | 30/30 | 0 | 0 | 0 | 19,39 ms | Sim |
| ChatGPT C2 | 30/30 | 0 | 0 | 0 | 27,06 ms | Sim |
| Gemini C2 | 30/30 | 0 | 0 | 0 | 33,51 ms | Sim |
| Grok C2 | 30/30 | 0 | 0 | 0 | 25,95 ms | Sim |
| Kimi C1 | 30/30 | 0 | 0 | 0 | 71,77 ms | Sim |
| Kimi C2 | 30/30 | 0 | 0 | 0 | 37,78 ms | Sim |
| Sonnet C1 | 30/30 | 60 | 540 | 0 | 15,75 ms | Nao |
| Sonnet C2 | 30/30 | 0 | 0 | 0 | 23,56 ms | Sim |
| Gemini C1 | 0/30 | - | - | - | - | Nao, sem HTTP parametrizavel |
| Grok C1 | 0/30 | - | - | - | - | Nao, somente CLI |

A latencia baixa do Sonnet C1 nao representa bom desempenho. Muitas requisicoes falharam rapidamente.

Resultado canonico:

```text
artifacts/concurrency-stability-v1/20260914T161458Z/
```

## 7. Integridade Financeira e Transacional

### 7.1 Invariantes comuns

O benchmark adota as seguintes invariantes independentes da arquitetura:

1. O valor deve ser positivo.
2. Origem e destino devem ser diferentes.
3. As duas partes devem existir.
4. A origem deve possuir saldo suficiente.
5. Nenhum saldo pode ficar negativo.
6. O valor debitado deve ser igual ao valor creditado.
7. A soma dos saldos envolvidos deve permanecer constante.
8. Uma transferencia confirmada deve produzir exatamente um registro no ledger.
9. Uma transferencia rejeitada nao pode alterar saldos ou ledger.
10. Debito, credito e ledger devem ocorrer atomicamente.
11. Concorrencia nao pode produzir double spending ou lost update.

### 7.2 Oraculo independente

Os testes produzidos pelo modelo nao sao tratados como unico oraculo. O avaliador realiza consultas diretas em PostgreSQL ou SQLite depois de cada acao.

Essa separacao evita que uma solucao com testes superficiais pareca correta apenas porque seus proprios testes passam.

### 7.3 Falhas criticas

As seguintes condicoes reprovam a integridade, independentemente do score arquitetural:

- dinheiro criado ou perdido;
- saldo negativo;
- debito sem credito;
- credito sem debito;
- ledger divergente das transferencias confirmadas;
- double spending;
- lost update reproduzido.

Erros HTTP 500 sob contencao, sem corrupcao contabil, sao classificados como falha de disponibilidade e desenho concorrente.

## 8. Esforco Operacional

As solucoes sao heterogeneas. Algumas fornecem Dockerfile e Compose; outras fornecem apenas um arquivo Go. Para executar todas sem alterar os snapshots, o harness pode fornecer assistencias registradas.

Categorias usadas:

- `database_provisioned`;
- `migration_applied_external`;
- `migration_reconstructed_external`;
- `schema_created_external`;
- `seed_applied_external`;
- `seed_via_public_api`;
- `environment_injected`;
- `build_command_supplied`;
- `readiness_probe_supplied`;
- `working_directory_isolated`;
- `cli_arguments_supplied`;
- `fixed_port_required`.

O numero bruto de assistencias nao deve ser interpretado isoladamente. Criar um schema que o modelo omitiu e mais grave do que injetar uma variavel de ambiente normal.

Exemplos relevantes:

- Gemini C2 exigiu schema completo criado externamente;
- Grok C2 exigiu reconstrucao externa do schema embutido para preparacao controlada;
- Kimi C1 exige porta fixa 8080;
- Gemini C1 exige diretorio isolado e nao aceita parametros;
- Sonnet C2 possui Dockerfile incompatível com a versao Go declarada.

## 9. Avaliacao Arquitetural

### 9.1 Escala

Cada dimensao recebe uma nota inteira de 0 a 4:

| Nota | Interpretacao |
|---:|---|
| 0 | Propriedade ausente, inadequada ou contradita por falha critica |
| 1 | Implementacao fraca; riscos fundamentais permanecem |
| 2 | Implementacao parcial, com lacunas relevantes |
| 3 | Implementacao boa e adequada ao escopo |
| 4 | Implementacao robusta, demonstrada por evidencias independentes |

Em caso de duvida entre dois niveis, aplica-se o menor.

### 9.2 Pesos

| Dimensao | Peso | Pergunta central |
|---|---:|---|
| Clean Architecture | 12% | As regras centrais independem dos mecanismos e as dependencias apontam para dentro? |
| DDD | 8% | Tipos e operacoes de dominio protegem invariantes financeiras? |
| SOLID | 10% | Responsabilidades e contratos reduzem acoplamento real sem abstracoes artificiais? |
| Testabilidade | 10% | Dependencias e falhas podem ser controladas deterministicamente? |
| Qualidade dos testes | 12% | Os testes detectam falhas em regras e estado, em vez de apenas executar linhas? |
| Integridade transacional | 15% | Debito, credito e ledger sao atomicos? |
| Concorrencia | 15% | O desenho evita double spending, lost update e erros indevidos sob contencao? |
| Observabilidade | 6% | Sucessos e falhas podem ser diagnosticados e correlacionados? |
| Prontidao operacional | 6% | Build, configuracao, migration, readiness e shutdown sao reproduziveis? |
| Simplicidade proporcional | 6% | A complexidade adicionada resolve riscos demonstraveis? |

Integridade transacional e concorrencia recebem os maiores pesos porque o dominio e financeiro.

### 9.3 Formula

```text
score = soma(peso_da_dimensao * nota_da_dimensao / 4)
```

O resultado varia de 0 a 100.

### 9.4 Faixas

| Score | Classificacao |
|---:|---|
| 0 a 39,99 | Inadequado |
| 40 a 59,99 | Fragil |
| 60 a 74,99 | Parcialmente adequado |
| 75 a 89,99 | Adequado |
| 90 a 100 | Robusto |

### 9.5 Regras contra vies

- A mesma rubrica e aplicada aos dois cenarios.
- Nomes como `domain`, `usecase` e `adapter` nao geram pontos sozinhos.
- Quantidade de arquivos, camadas ou interfaces nao gera credito automatico.
- Complexidade adicional precisa resolver um risco observavel.
- Uma solucao pequena pode receber nota maxima.
- Falhas comportamentais prevalecem sobre comentarios, README ou mocks.
- `go test` com zero testes recebe zero em qualidade dos testes.
- Cobertura alta nao garante testes fortes.
- Complexidade total do Sonar nao e tratada automaticamente como boa ou ruim.

### 9.6 TDD

TDD e uma variavel de processo, nao uma propriedade comprovavel apenas pelo snapshot final.

Para considerar TDD comprovado seria necessario preservar ciclos verificaveis:

```text
teste falha -> implementacao minima -> teste passa -> refatoracao
```

Nenhuma das dez execucoes possui esse historico. Todas foram marcadas como `not_provable`. Gemini C1 e Sonnet C1 tambem nao possuem testes no snapshot, mas isso nao autoriza afirmar retroativamente como o codigo foi produzido.

### 9.7 Resultado arquitetural

| Run | Clean | DDD | SOLID | Testabilidade | Testes | Transacao | Concorrencia | Observabilidade | Operacao | Simplicidade | Score | Faixa |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| ChatGPT C1 | 2 | 1 | 2 | 2 | 2 | 3 | 3 | 1 | 3 | 4 | 58,50 | Fragil |
| ChatGPT C2 | 4 | 3 | 3 | 4 | 3 | 4 | 3 | 3 | 2 | 3 | 82,75 | Adequado |
| Gemini C1 | 0 | 1 | 1 | 1 | 0 | 3 | 1 | 1 | 1 | 3 | 29,50 | Inadequado |
| Gemini C2 | 3 | 3 | 3 | 3 | 3 | 3 | 3 | 3 | 1 | 2 | 70,50 | Parcialmente adequado |
| Grok C1 | 1 | 1 | 1 | 2 | 1 | 3 | 2 | 1 | 2 | 4 | 44,75 | Fragil |
| Grok C2 | 3 | 3 | 3 | 4 | 3 | 4 | 3 | 2 | 2 | 3 | 78,25 | Adequado |
| Kimi C1 | 1 | 1 | 1 | 2 | 3 | 4 | 2 | 1 | 1 | 4 | 53,00 | Fragil |
| Kimi C2 | 4 | 3 | 4 | 4 | 4 | 4 | 4 | 3 | 3 | 3 | 93,50 | Robusto |
| Sonnet C1 | 2 | 2 | 2 | 1 | 0 | 4 | 2 | 1 | 2 | 3 | 49,00 | Fragil |
| Sonnet C2 | 4 | 3 | 4 | 4 | 3 | 4 | 3 | 2 | 2 | 2 | 82,25 | Adequado |

Revisoes completas, incluindo justificativas e referencias de arquivo e linha:

```text
benchmark/architecture/reviews.json
```

### 9.8 Efeito pareado

| Modelo | Cenario 1 | Cenario 2 | Delta C2-C1 |
|---|---:|---:|---:|
| ChatGPT | 58,50 | 82,75 | +24,25 |
| Gemini | 29,50 | 70,50 | +41,00 |
| Grok | 44,75 | 78,25 | +33,50 |
| Kimi | 53,00 | 93,50 | +40,50 |
| Sonnet | 49,00 | 82,25 | +33,25 |

O ganho medio pareado e 34,50 pontos. Como ha apenas uma geracao por combinacao, esse resultado nao mede variabilidade estocastica.

## 10. Riscos Identificados

### ChatGPT C1

- ausencia de idempotencia para retries ambiguos;
- nucleo transacional sem teste proprio gerado;
- observabilidade basica.

### ChatGPT C2

- ausencia de idempotencia;
- concorrencia critica testada pelo modelo principalmente com mocks;
- Compose sobe apenas o banco.

### Gemini C1

- `INSERT OR REPLACE` redefine saldos em cada execucao;
- interface fixa impede variar entradas;
- nenhum teste produzido;
- nao e backend HTTP reutilizavel.

### Gemini C2

- nenhum schema ou migration fornecido;
- `Money.Add` sem verificacao de overflow;
- servidor continua ativo se o ping inicial ao banco falhar;
- 5,4% de duplicacao no Sonar.

### Grok C1

- uma conexao SQLite serializa o processo;
- ausencia de idempotencia;
- credito sem protecao explicita contra overflow;
- requer CGO.

### Grok C2

- gerador chamado `uuidGen` nao produz UUID e concatena valores de relogio;
- migration embutida no startup;
- ausencia de retry para conflitos residuais.

### Kimi C1

- API usa `float64` para valores monetarios;
- `SetMaxOpenConns(1)` limita throughput;
- porta 8080 e banco fixos;
- servidor sem timeouts e sem graceful shutdown.

### Kimi C2

- senha PostgreSQL padrao embutida;
- idempotencia funciona como restricao de unicidade, sem replay do resultado original;
- rotas de reconstrucao de dominio sao permissivas.

### Sonnet C1

- nenhum teste produzido;
- isolamento serializavel sem retry;
- 540 respostas HTTP 500 no protocolo repetido;
- migrations sem foreign keys do ledger para wallets.

### Sonnet C2

- Dockerfile usa Go 1.23 enquanto `go.mod` exige Go 1.26;
- teste concorrente produzido usa fake e ignora erros;
- healthcheck nao verifica PostgreSQL;
- ausencia de idempotencia.

## 11. Custo, Tokens e Tempo

Os dados de billing sao extraidos dos arquivos originais `billing_result.txt` ou `billing_results.txt`, sem renomear ou alterar os snapshots.

Campos normalizados:

- modelo;
- tempo em segundos;
- tokens;
- custo em USD;
- nome do arquivo de origem.

| Run | Tempo registrado | Tokens | Custo USD |
|---|---:|---:|---:|
| ChatGPT C1 | 2m 35s | 22.535 | 0,45 |
| ChatGPT C2 | 5m 5s | 16.930 | 0,23 |
| Gemini C1 | 46s | 9.225 | 0,03 |
| Gemini C2 | 4m 19s | 66.500 | 0,32 |
| Grok C1 | 1m 22s | 10.637 | 0,06 |
| Grok C2 | 3m 44s | 30.812 | 0,37 |
| Kimi C1 | 6m 42s | 18.057 | 0,26 |
| Kimi C2 | 31m 45s | 67.640 | 1,90 |
| Sonnet C1 | 2m 16s | 23.337 | 0,08 |
| Sonnet C2 | 19m 10s | 138.064 | 2,49 |

Tempo, tokens e custo sao metadados reportados na geracao, nao medicoes refeitas pelo harness. Eles devem ser tratados conforme a confiabilidade da fonte original de billing.

O custo nao integra automaticamente o score arquitetural. Ele deve ser analisado como dimensao economica separada, por exemplo:

- custo por solucao funcional;
- custo por ponto arquitetural;
- custo por ponto percentual de cobertura;
- custo por backend elegivel para carga;
- tokens por teste produzido.

Uma solucao barata, mas nao parametrizavel, nao deve vencer apenas pelo custo. Da mesma forma, uma solucao cara nao deve receber credito arquitetural por volume de codigo.

## 12. Fontes Canonicas dos Resultados

| Dimensao | Fonte canonica |
|---|---|
| Proveniencia | `provenance/classification-summary.json` |
| Qualidade inicial | `artifacts/initial-quality-v1/20260907T002744Z/summary.json` |
| Seguranca e Sonar | `artifacts/security-sonar-v1/20260913T010319Z/consolidated-summary.json` |
| Smoke funcional | `artifacts/functional-smoke-v1/20260913T015119Z/summary.json` |
| Robustez | `artifacts/functional-robustness-v1/20260913T022017Z/summary.json` |
| Arquitetura | `artifacts/architecture-review-v1/summary.json` |
| Estabilidade | `artifacts/concurrency-stability-v1/20260914T161458Z/summary.json` |
| Carga K6 de um milhao | `artifacts/k6-million-load-v1/20260914T195743Z/summary.json` |

Arquivos Markdown sao relatorios derivados. Os JSONs sao a fonte primaria para futuras tabelas e analises estatisticas.

## 13. Comandos

### 13.1 Integridade

```bash
python3 scripts/classify_snapshots.py --verify
```

### 13.2 Qualidade inicial

```bash
python3 scripts/run_benchmark.py
```

### 13.3 Seguranca e SonarQube

```bash
python3 scripts/run_security_benchmark.py
```

### 13.4 Smoke funcional

```bash
python3 scripts/run_functional_benchmark.py
```

### 13.5 Robustez funcional

```bash
python3 scripts/run_functional_robustness.py
```

### 13.6 Relatorio arquitetural

```bash
python3 scripts/generate_architecture_report.py
```

### 13.7 Estabilidade concorrente

```bash
python3 scripts/run_concurrency_stability.py
```

Cada executor funcional aceita `--run <run-id>` para uma execucao individual e `--keep-work` para preservar a copia temporaria para diagnostico.

### 13.8 Atalhos Windows

Os wrappers numerados executam as mesmas etapas principais no Windows:

| Arquivo | Etapa |
|---|---|
| `01_classificar_snapshots.bat` | Classificacao e verificacao dos snapshots |
| `02_executar_benchmark.bat` | Qualidade inicial |
| `03_executar_benchmark_seguranca.bat` | Seguranca, lint e SonarQube |
| `05_executar_benchmark_funcional.bat` | Smoke funcional |
| `06_executar_robustez_funcional.bat` | Robustez funcional |
| `07_gerar_avaliacao_arquitetural.bat` | Relatorio arquitetural |
| `08_executar_estabilidade_concorrente.bat` | Estabilidade concorrente |
| `09_executar_carga_k6.bat` | Carga K6 final e reconciliacao |

`00_executar_todos.bat` orquestra a sequencia e `04_analisar_experimentos.bat` pertence ao fluxo legado de analise. Os JSONs de campanha e os scripts Python permanecem as definicoes auditaveis do protocolo.

## 14. Elegibilidade para a Carga Final

Um backend avanca para a carga K6 somente se:

- possui interface HTTP parametrizavel;
- passa no smoke funcional;
- passa nos casos negativos;
- preserva integridade e ledger sob concorrencia;
- conclui as 30 repeticoes sem HTTP 5xx inesperado;
- nao apresenta timeout ou erro de transporte;
- nao deixa saldo negativo;
- nao apresenta double spending ou lost update.

### Elegiveis

```text
chatgpt56sol-cenario-1
chatgpt56sol-cenario-2
gemini37flash-cenario-2
grok46-cenario-2
kimik3-cenario-1
kimik3-cenario-2
sonnet5-cenario-2
```

### Nao elegiveis

| Run | Motivo |
|---|---|
| Gemini C1 | Programa one-shot sem API ou entradas parametrizaveis |
| Grok C1 | Somente CLI; nao representa backend HTTP para o K6 |
| Sonnet C1 | Falha consistente sob contencao, com 540 respostas HTTP 500 |

## 15. Carga K6 de Um Milhao de Requisicoes

A carga final foi executada em `20260914T195743Z` com K6 via Docker nos sete backends elegiveis. Cada backend recebeu exatamente 1.000.000 de iteracoes, com uma requisicao HTTP de transferencia por iteracao.

Implementacao versionada da campanha:

```text
benchmark/campaigns/k6-million-load.json
benchmark/k6/million-transfers.js
scripts/run_k6_benchmark.py
```

Execucao completa:

```bash
python3 scripts/run_k6_benchmark.py
```

Uma execucao existente pode ter seu Markdown regenerado sem repetir a carga:

```bash
python3 scripts/run_k6_benchmark.py --report-only <execution-id>
```

### 15.1 Configuracao efetiva

| Parametro | Valor |
|---|---:|
| Executor K6 | `shared-iterations` |
| Iteracoes por backend | 1.000.000 |
| Requisicoes por iteracao | 1 |
| VUs | 50 |
| Pares de carteiras | 100 |
| Transferencias planejadas por par | 10.000 |
| Valor por transferencia | 1 centavo |
| Timeout por requisicao | 30 segundos |
| Duracao maxima | 24 horas |
| Intervalo do progresso no terminal | 30 segundos |
| Retries do cliente | nenhum |

A imagem K6 foi fixada pelo digest `sha256:1f40432b1cbe7234e977f96c362c9bc550a2d2b583d014dd8669fe40d3e9e755`. O executor fechado `shared-iterations` foi escolhido para garantir uma quantidade finita e exata de tentativas; testes por duracao ou arrival rate poderiam encerrar com iteracoes descartadas.

As operacoes foram distribuidas deterministicamente entre 100 pares. Cada origem recebeu 10.000 centavos e cada destino recebeu saldo inicial zero. A iteracao global foi atribuida por modulo do numero de pares, de modo que cada origem pudesse financiar exatamente suas 10.000 transferencias de um centavo. O estado esperado por backend era:

```text
200 carteiras
saldo total inicial = 1.000.000 centavos
saldo final das origens = 0
saldo final dos destinos = 1.000.000 centavos
ledger esperado = 1.000.000 linhas
valor total no ledger = 1.000.000 centavos
nenhum saldo negativo
```

Os contratos HTTP permaneceram especificos por run:

| Run | Endpoint | Status esperado |
|---|---|---:|
| ChatGPT C1 | `POST /transfers` | 201 |
| ChatGPT C2 | `POST /v1/transfers` | 201 |
| Gemini C2 | `POST /transfers` | 201 |
| Grok C2 | `POST /transfers` | 201 |
| Kimi C1 | `POST /pix/transfer` | 200 |
| Kimi C2 | `POST /v1/transfers` | 201 |
| Sonnet C2 | `POST /transfers` | 200 |

### 15.2 Evidencias coletadas

O protocolo registrou:

- exatamente 1.000.000 de iteracoes planejadas;
- requisicoes concluidas;
- throughput em requisicoes por segundo;
- duracao total;
- latencia media, mediana, p90, p95 e p99;
- taxa de erro;
- timeouts;
- respostas por status HTTP;
- exit code do K6;
- contagem final do ledger;
- saldos finais;
- conservacao do saldo total;
- saldos negativos;
- progresso, throughput medio e ETA no terminal a cada 30 segundos.

CPU, memoria e deadlocks nao foram persistidos na rodada canonica e nao devem ser inferidos retroativamente. No momento da preparacao, o Docker informava 12 CPUs e aproximadamente 16,7 GB de memoria disponivel ao ambiente, embora a maquina hospedeira possua 32 GB. O container K6 nao recebeu limite individual de memoria.

Os binarios foram compilados em paralelo, mas as sete cargas foram executadas sequencialmente para reduzir interferencia entre medicoes. Aplicacoes, bancos e K6 operaram fora de `runs/`; PostgreSQL foi efemero por run, enquanto Kimi C1 utilizou SQLite em `work/`. Logs por requisicao das aplicacoes foram descartados para evitar que escrita de milhoes de linhas alterasse o teste; logs e resumos do K6 foram preservados em `artifacts/`.

### 15.3 Criterios de aprovacao

Um run foi aprovado apenas quando todas as condicoes abaixo foram satisfeitas:

- 1.000.000 de iteracoes concluidas;
- 1.000.000 de requisicoes HTTP emitidas;
- 1.000.000 de respostas com o status de sucesso esperado;
- zero respostas inesperadas ou falhas de transporte;
- 1.000.000 de registros correspondentes no ledger;
- valor acumulado no ledger igual a 1.000.000 de centavos;
- 200 carteiras de teste encontradas;
- saldos finais de origem e destino exatamente iguais ao esperado;
- saldo total conservado;
- nenhum saldo negativo.

Nao houve retry automatico. Como a maioria das APIs nao oferece idempotencia completa, repetir uma requisicao apos resposta ambigua poderia duplicar uma transferencia ja confirmada.

### 15.4 Resultado canonico

| Run | Status | Requisicoes | Sucessos HTTP | Falhas HTTP | Duracao | RPS | Media | Mediana | p90 | p95 | p99 | Ledger |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| ChatGPT C1 | PASS | 1.000.000 | 1.000.000 | 0 | 9m 35s | 1.740,17 | 28,47 ms | 22,42 ms | 43,75 ms | 50,65 ms | 76,59 ms | 1.000.000 |
| ChatGPT C2 | PASS | 1.000.000 | 1.000.000 | 0 | 11m 06s | 1.503,70 | 32,71 ms | 26,86 ms | 53,20 ms | 71,30 ms | 119,73 ms | 1.000.000 |
| Gemini C2 | PASS | 1.000.000 | 1.000.000 | 0 | 10m 58s | 1.521,97 | 32,47 ms | 27,56 ms | 54,80 ms | 63,74 ms | 84,12 ms | 1.000.000 |
| Grok C2 | FAIL | 1.000.000 | 999.937 | 63 HTTP 500 | 10m 45s | 1.553,38 | 31,80 ms | 26,85 ms | 53,89 ms | 62,74 ms | 83,46 ms | 999.937 |
| Kimi C1 | PASS | 1.000.000 | 1.000.000 | 0 | 3h 09m 35s | 87,92 | 568,40 ms | 394,09 ms | 1.290,88 ms | 1.686,97 ms | 2.624,73 ms | 1.000.000 |
| Kimi C2 | PASS | 1.000.000 | 1.000.000 | 0 | 10m 15s | 1.627,65 | 30,16 ms | 25,77 ms | 45,89 ms | 59,00 ms | 105,42 ms | 1.000.000 |
| Sonnet C2 | PASS | 1.000.000 | 1.000.000 | 0 | 9m 23s | 1.779,22 | 27,53 ms | 20,77 ms | 47,87 ms | 73,28 ms | 127,83 ms | 1.000.000 |

Seis backends foram aprovados e um foi reprovado. Entre os aprovados, Sonnet C2 apresentou o maior throughput, com 1.779,22 requisicoes por segundo, seguido por ChatGPT C1, com 1.740,17. Kimi C1 foi o mais lento: seus 87,92 RPS e 3h09m35s refletem principalmente SQLite com `SetMaxOpenConns(1)`, que serializa a escrita. Ele foi aproximadamente vinte vezes mais lento que o maior throughput observado, mas concluiu todas as operacoes com reconciliacao exata.

Os valores de throughput nao devem ser lidos isoladamente da taxa de sucesso e da reconciliacao. Grok C2, por exemplo, apresentou 1.553,38 tentativas por segundo, mas nao cumpriu o contrato de um milhao de transferencias confirmadas.

### 15.5 Falha do Grok C2

Grok C2 concluiu as 1.000.000 de iteracoes e requisicoes, mas retornou 63 respostas HTTP 500. O K6 registrou exit code 99 porque os thresholds de sucesso, checks e taxa de erro foram violados:

```text
HTTP 201 = 999.937
HTTP 500 = 63
ledger = 999.937 linhas
saldo restante nas origens = 63 centavos
saldo recebido pelos destinos = 999.937 centavos
saldo total = 1.000.000 centavos
saldos negativos = 0
```

A reconciliacao, portanto, nao falhou operacionalmente: ela detectou corretamente que as 63 requisicoes rejeitadas nao produziram debito nem ledger. A conservacao financeira permaneceu valida, mas a reconciliacao exata contra um milhao de sucessos falhou, como deveria.

O comportamento e consistente com o risco arquitetural previamente identificado em Grok C2. O gerador denominado `uuidGen` concatena leituras de relogio em vez de produzir um UUID ou usar uma sequencia segura. Sob 50 VUs, colisoes de chave primaria podem fazer o INSERT do ledger falhar e resultar em HTTP 500. A carga independente transformou esse risco estatico em uma falha observada.

### 15.6 Interpretacao dos exit codes

Exit code zero indica que todos os thresholds K6 foram satisfeitos. O exit code 99 de Grok C2 representa quebra de thresholds, nao falha de inicializacao do K6 ou do conciliador. O runner encerra a campanha agregada com codigo diferente de zero quando qualquer backend e reprovado, ainda que preserve resultados validos dos demais.

Resultado canonico:

```text
artifacts/k6-million-load-v1/20260914T195743Z/summary.json
artifacts/k6-million-load-v1/20260914T195743Z/report.md
```

A carga precisa distribuir operacoes entre varias carteiras para evitar que saldo insuficiente transforme o teste em mera medicao de rejeicoes. Apos o K6, uma reconciliacao independente deve validar o banco.

Um backend rapido, mas contabilmente incorreto, e reprovado. Throughput nao compensa corrupcao financeira.

## 16. Limitacoes Metodologicas

### 16.1 Uma amostra por combinacao

Ha uma unica geracao por modelo e cenario. O experimento nao estima adequadamente a variabilidade estocastica de cada modelo.

Para uma extensao futura, recomenda-se tres a cinco repeticoes por combinacao.

### 16.2 Versao unica do ambiente de avaliacao

Os projetos declaram versoes Go diferentes, mas foram avaliados principalmente com Go 1.26. Isso favorece comparabilidade operacional, mas nao reproduz exatamente todos os ambientes declarados pelos modelos.

### 16.3 Adapters especificos

As APIs e schemas diferem. O benchmark usa configuracao por run para traduzir cada contrato para invariantes comuns. Essa traducao deve permanecer versionada e auditavel.

### 16.4 Schema externo

Gemini C2 nao forneceu schema. O benchmark criou um schema minimo baseado nas queries da propria implementacao. Isso permite testar funcionalidade, mas e registrado como assistencia penalizavel.

### 16.5 Cobertura Go e Sonar

Os percentuais Go e Sonar podem divergir devido a inclusoes, exclusoes e interpretacao de arquivos. Ambos sao preservados e nao devem ser substituidos um pelo outro.

### 16.6 Quality Gate padrao

O gate local aceitou um projeto com vulnerabilidade blocker. Seu status e informativo, nao eliminatorio.

### 16.7 TDD nao comprovavel

Sem historico temporal, transcript estruturado ou commits granulares, nao e possivel provar TDD.

### 16.8 Avaliacao arquitetural humana

As notas incluem julgamento tecnico, ainda que cada nota possua justificativa e evidencia. Para maior rigor cientifico, recomenda-se:

- dois avaliadores independentes;
- identidade de modelo e cenario ocultada;
- ordem randomizada;
- calculo de weighted Cohen's kappa;
- adjudicacao registrada para divergencias maiores que um ponto.

### 16.9 Uma rodada de carga por backend

A campanha K6 possui uma unica rodada medida por backend, com 50 VUs. Ela nao estima variacao entre repeticoes nem demonstra o ponto de saturacao de cada implementacao. O resultado combina capacidade do backend, banco, host, Docker/WSL e gerador de carga no ambiente observado.

### 16.10 Recursos nao monitorados

A rodada canonica nao persistiu series de CPU, memoria, I/O, WAL ou deadlocks. O throughput e as latencias sao validos para a execucao, mas nao permitem atribuir quantitativamente o gargalo a CPU, memoria, locks ou armazenamento.

### 16.11 Heterogeneidade de persistencia

Seis backends da carga usam PostgreSQL efemero; Kimi C1 usa SQLite com uma conexao. Essa diferenca pertence aos artefatos avaliados e explica parte importante do desempenho, mas impede interpretar a tabela como uma comparacao isolada apenas da qualidade do codigo HTTP.

### 16.12 Crescimento durante a carga

Cada transferencia gera um commit e uma linha de ledger. O crescimento das tabelas e indices ao longo de um milhao de operacoes pode alterar o throughput durante a propria rodada. O RPS apresentado e a media da execucao completa, nao uma taxa estacionaria garantida.

## 17. Analise Estatistica Recomendada

Com cinco pares e uma amostra por celula, a analise deve ser apresentada como exploratoria.

Recomenda-se publicar:

- resultados individuais;
- mediana e intervalo interquartil por cenario;
- deltas pareados por modelo;
- teste de permutacao pareado ou Wilcoxon como exploratorio;
- tamanho de efeito pareado;
- bootstrap pareado para intervalo de confianca;
- analise de sensibilidade com pesos iguais;
- score apenas estrutural;
- score apenas comportamental;
- custo e tokens separadamente.

Nao se deve tratar os dez snapshots como dez observacoes independentes, pois sao cinco modelos observados sob dois tratamentos.

## 18. Conclusoes Consolidadas

Depois da carga final, as evidencias indicam:

1. Os dez artefatos conseguem realizar ao menos uma transferencia correta no ambiente preparado pelo benchmark.
2. O cenario 2 elevou o score arquitetural nos cinco modelos.
3. O aumento medio pareado foi de 34,50 pontos.
4. Cobertura e quantidade de testes cresceram substancialmente no cenario 2.
5. Mais estrutura nao eliminou riscos operacionais: Gemini C2 omitiu schema e Sonnet C2 forneceu Dockerfile incompatível.
6. Sonnet C1 preserva integridade, mas falha severamente em disponibilidade concorrente.
7. Kimi C2 obteve o maior score arquitetural, mas ainda apresentou credencial padrao embutida.
8. Sete backends demonstraram estabilidade suficiente para entrar no K6, mas apenas seis concluiram um milhao de transferencias com sucesso e reconciliacao exata.
9. Sonnet C2 apresentou o maior throughput da rodada, com 1.779,22 RPS; ChatGPT C1 ficou proximo, com 1.740,17 RPS.
10. Kimi C1 preservou toda a integridade, mas a configuracao SQLite de uma conexao limitou o throughput a 87,92 RPS e elevou a duracao para 3h09m35s.
11. Grok C2 produziu 63 HTTP 500, confirmando sob carga o risco do gerador de IDs baseado em relogio; apesar disso, nao houve perda de dinheiro, saldo negativo ou ledger divergente dos 999.937 sucessos observados.
12. Seis backends processaram e reconciliaram exatamente 1.000.000 de transferencias cada, totalizando 6.000.000 de transferencias confirmadas entre os aprovados.

Esses resultados sustentam a hipotese de que prompts estruturados podem melhorar propriedades arquiteturais e de teste, mas tambem mostram que verificacao funcional, seguranca, operacao e carga continuam indispensaveis. A presenca nominal de Clean Architecture, testes, estabilidade em rodadas pequenas ou Quality Gate aprovado nao substitui evidencia comportamental independente em escala.
