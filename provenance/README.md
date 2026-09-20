# Proveniencia dos snapshots

Este diretorio registra a integridade e a classificacao dos arquivos recebidos em `runs/`.
Os snapshots sao entradas imutaveis: nenhuma ferramenta do benchmark deve executar ou gravar
diretamente nesses diretorios.

Para cada combinacao de modelo e cenario, sao mantidos:

- `source-manifest.json`: metadados e SHA-256 de cada arquivo ou link simbolico.
- `classification.json`: natureza de cada item do snapshot.
- `snapshot.sha256`: hash raiz calculado sobre a representacao canonica do manifesto.

O resumo dos dez snapshots esta em `classification-summary.json`. Arquivos preexistentes como
binarios, bancos SQLite, perfis de cobertura e `node_modules` permanecem no snapshot, mas sao
classificados como `generation_artifact` e nao podem ser confundidos com resultados futuros do
benchmark.

## Comandos

Gerar a proveniencia inicial ou atualizar apenas a classificacao de snapshots inalterados:

```bash
python3 scripts/classify_snapshots.py
```

Verificar o conteudo atual de `runs/` contra a proveniencia selada:

```bash
python3 scripts/classify_snapshots.py --verify
```

O primeiro comando recusa sobrescrever um manifesto quando o hash do snapshot mudou. Alteracoes
intencionais nas regras ficam em `benchmark/classification-overrides.json`; os schemas ficam em
`benchmark/schemas/`.
