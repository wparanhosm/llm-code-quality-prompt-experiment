import json
from collections import defaultdict
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
RUBRIC_PATH = ROOT / "benchmark" / "architecture" / "rubric.json"
REVIEWS_PATH = ROOT / "benchmark" / "architecture" / "reviews.json"
OUTPUT = ROOT / "artifacts" / "architecture-review-v1"


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def band(score):
    if score < 40:
        return "inadequado"
    if score < 60:
        return "fragil"
    if score < 75:
        return "parcialmente adequado"
    if score < 90:
        return "adequado"
    return "robusto"


def main():
    rubric = read_json(RUBRIC_PATH)
    reviews = read_json(REVIEWS_PATH)
    dimensions = rubric["dimensions"]
    if sum(item["weight"] for item in dimensions.values()) != 100:
        raise RuntimeError("os pesos da rubrica devem totalizar 100")

    provenance = {
        item["run_id"]: item["root_hash"]
        for item in read_json(ROOT / "provenance" / "classification-summary.json")["runs"]
    }
    rows = []
    model_scores = defaultdict(dict)
    for review in reviews["runs"]:
        run_id = review["run_id"]
        if provenance.get(run_id) != review["source_root_hash"]:
            raise RuntimeError(f"hash divergente na revisao: {run_id}")
        if set(review["dimensions"]) != set(dimensions):
            raise RuntimeError(f"dimensoes divergentes na revisao: {run_id}")
        score = round(sum(dimensions[name]["weight"] * value["score"] / 4 for name, value in review["dimensions"].items()), 2)
        scenario = run_id.rsplit("-", 1)[1]
        model = run_id.rsplit("-cenario-", 1)[0]
        model_scores[model][scenario] = score
        rows.append({
            "run_id": run_id,
            "model": model,
            "scenario": scenario,
            "score": score,
            "classification": band(score),
            "dimension_scores": {name: value["score"] for name, value in review["dimensions"].items()},
            "tdd": review["tdd"],
            "critical_risks": review["critical_risks"],
            "confidence": review["confidence"],
        })

    deltas = []
    for model, scores in sorted(model_scores.items()):
        if set(scores) == {"1", "2"}:
            deltas.append({"model": model, "cenario_1": scores["1"], "cenario_2": scores["2"], "delta": round(scores["2"] - scores["1"], 2)})

    OUTPUT.mkdir(parents=True, exist_ok=True)
    payload = {
        "schema_version": "1.0",
        "rubric": str(RUBRIC_PATH.relative_to(ROOT)),
        "reviews": str(REVIEWS_PATH.relative_to(ROOT)),
        "runs": rows,
        "paired_deltas": deltas,
    }
    (OUTPUT / "summary.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    abbreviations = {
        "clean_architecture": "Clean", "ddd": "DDD", "solid": "SOLID", "testability": "Testab.",
        "test_quality": "Testes", "transactional_integrity": "Transacao", "concurrency_design": "Concorr.",
        "observability": "Observ.", "operational_readiness": "Operacao", "simplicity_fit": "Simplic."
    }
    headers = ["Run", *abbreviations.values(), "Score", "Faixa"]
    lines = [
        "# Avaliacao arquitetural", "",
        "Escala por dimensao: 0 a 4. Score ponderado: 0 a 100. TDD e reportado separadamente.", "",
        "| " + " | ".join(headers) + " |", "|" + "|".join(["---"] * len(headers)) + "|"
    ]
    for row in sorted(rows, key=lambda item: item["run_id"]):
        values = [row["run_id"], *[str(row["dimension_scores"][name]) for name in abbreviations], f'{row["score"]:.2f}', row["classification"]]
        lines.append("| " + " | ".join(values) + " |")
    lines.extend(["", "## Diferencas Pareadas", "", "| Modelo | Cenario 1 | Cenario 2 | Delta C2-C1 |", "|---|---:|---:|---:|"])
    for item in deltas:
        lines.append(f'| {item["model"]} | {item["cenario_1"]:.2f} | {item["cenario_2"]:.2f} | {item["delta"]:+.2f} |')
    lines.extend(["", "## TDD", "", "Nenhum snapshot fornece historico red-green-refactor verificavel. A presenca de testes e cobertura nao foi tratada como prova de TDD.", "", "## Riscos Criticos", ""])
    for row in sorted(rows, key=lambda item: item["run_id"]):
        for risk in row["critical_risks"]:
            lines.append(f'- `{row["run_id"]}` [{risk["severity"]}]: {risk["risk"]}')
    lines.extend(["", "## Metodo", "", "As notas usam inspecao de codigo ligada a arquivo e linha, mais evidencias independentes de build, testes, seguranca, smoke funcional e concorrencia. Nomes de camadas e alegacoes em README nao geram credito isoladamente. O JSON de revisoes preserva justificativa e evidencias de cada nota."])
    (OUTPUT / "report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(OUTPUT / "report.md")


if __name__ == "__main__":
    main()
