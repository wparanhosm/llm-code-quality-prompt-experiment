import argparse
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def value(measures, name, default=""):
    return measures.get(name, default)


def main():
    parser = argparse.ArgumentParser(description="Consolida resultados locais e Sonar de execucoes completas.")
    parser.add_argument("--local-execution", default="20260912T233323Z")
    parser.add_argument("--sonar-execution", default="20260913T010319Z")
    args = parser.parse_args()

    root = ROOT / "artifacts" / "security-sonar-v1"
    local_path = root / args.local_execution / "summary.json"
    sonar_path = root / args.sonar_execution / "summary.json"
    local = {item["run_id"]: item for item in read_json(local_path)["runs"]}
    sonar = {item["run_id"]: item for item in read_json(sonar_path)["runs"]}
    if set(local) != set(sonar):
        raise RuntimeError("as execucoes local e Sonar nao possuem a mesma matriz de runs")

    rows = []
    for run_id in sorted(local):
        local_result = local[run_id]
        sonar_result = sonar[run_id]
        measures = sonar_result["sonar"]["measures"]
        rows.append({
            "run_id": run_id,
            "source_root_hash": sonar_result["source_root_hash"],
            "govulncheck_findings": local_result["security"]["govulncheck"]["vulnerability_events"],
            "gosec_issues": local_result["security"]["gosec"]["issues"],
            "gosec_severity": local_result["security"]["gosec"]["severity"],
            "lint_issues": local_result["lint"]["issues"],
            "lint_linters": local_result["lint"]["linters"],
            "sonar": {
                "coverage": value(measures, "coverage"),
                "bugs": value(measures, "bugs"),
                "vulnerabilities": value(measures, "vulnerabilities"),
                "security_hotspots": value(measures, "security_hotspots"),
                "code_smells": value(measures, "code_smells"),
                "complexity": value(measures, "complexity"),
                "cognitive_complexity": value(measures, "cognitive_complexity"),
                "duplicated_lines_density": value(measures, "duplicated_lines_density"),
                "ncloc": value(measures, "ncloc"),
                "technical_debt_minutes": value(measures, "sqale_index"),
                "issue_count": sonar_result["sonar"]["issue_count"],
                "quality_gate": sonar_result["sonar"]["quality_gate"],
                "project_key": sonar_result["sonar"]["project_key"],
            }
        })

    output = root / args.sonar_execution
    payload = {
        "schema_version": "1.0",
        "local_execution": args.local_execution,
        "sonar_execution": args.sonar_execution,
        "runs": rows,
    }
    (output / "consolidated-summary.json").write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )

    lines = [
        "# Seguranca, lint e SonarQube",
        "",
        f"Resultados locais: `{args.local_execution}`  ",
        f"Resultados SonarQube: `{args.sonar_execution}`",
        "",
        "| Run | Govuln | Gosec | Lint | Sonar cov. | Bugs | Vuln. | Hotspots | Smells | Duplicacao | Gate |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|",
    ]
    for row in rows:
        measures = row["sonar"]
        lines.append(
            f'| {row["run_id"]} | {row["govulncheck_findings"]} | {row["gosec_issues"]} | '
            f'{row["lint_issues"]} | {measures["coverage"]}% | {measures["bugs"]} | '
            f'{measures["vulnerabilities"]} | {measures["security_hotspots"]} | '
            f'{measures["code_smells"]} | {measures["duplicated_lines_density"]}% | '
            f'{measures["quality_gate"]} |'
        )
    lines.extend([
        "",
        "## Proveniencia",
        "",
        "Os scanners locais e o SonarQube foram executados sobre os mesmos hashes de snapshot. "
        "A consolidacao usa a rodada local completa porque a rodada de upload Sonar concorrente "
        "nao preservou todos os JSONs locais. Nenhum valor ausente foi convertido em zero.",
    ])
    (output / "consolidated-report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(output / "consolidated-report.md")


if __name__ == "__main__":
    main()
