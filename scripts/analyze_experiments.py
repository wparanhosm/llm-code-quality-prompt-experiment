import argparse
import json
import os
import subprocess
import sys
import time
import urllib.parse
import urllib.request
import concurrent.futures
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
METRICS = "coverage,bugs,vulnerabilities,code_smells,violations,complexity,cognitive_complexity,sqale_index"


def request(host, path):
    token = os.environ.get("SONAR_TOKEN")
    headers = {"Authorization": f"Bearer {token}"} if token else {}
    with urllib.request.urlopen(urllib.request.Request(f"{host}{path}", headers=headers), timeout=30) as response:
        return json.load(response)


def sonar_metrics(host, project_key):
    query = urllib.parse.urlencode({"component": project_key, "metricKeys": METRICS})
    return request(host, f"/api/measures/component?{query}")


def wait_for_analysis(host, project_key, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        query = urllib.parse.urlencode({"component": project_key})
        task = request(host, f"/api/ce/component?{query}").get("current")
        if not task or task["status"] == "SUCCESS":
            return
        if task["status"] in {"FAILED", "CANCELED"}:
            raise RuntimeError(f"análise SonarQube terminou com {task['status']}")
        time.sleep(3)
    raise RuntimeError("tempo esgotado aguardando a análise SonarQube")


def execute_run(run, scenario, path):
    subprocess.run(scenario["test_command"], cwd=path, check=True)
    command = ["docker", "run", "--rm", "--network", "llm-code-quality-prompt-experiment_default", "-e", "SONAR_HOST_URL=http://sonarqube:9000"]
    if os.environ.get("SONAR_TOKEN"):
        command.extend(["-e", "SONAR_TOKEN"])
    command.extend(["-v", f"{path.resolve()}:/usr/src", "sonarsource/sonar-scanner-cli"])
    subprocess.run(command, check=True)


def main():
    parser = argparse.ArgumentParser(description="Executa e consolida métricas de todos os experimentos configurados.")
    parser.add_argument("--execute", action="store_true", help="executa testes e SonarScanner antes de extrair métricas")
    parser.add_argument("--timeout", type=int, default=180, help="tempo máximo de espera pela análise SonarQube")
    args = parser.parse_args()
    config = json.loads((ROOT / "experiments.json").read_text(encoding="utf-8"))
    scenarios = {item["id"]: item for item in config["scenarios"]}
    models = {item["id"]: item for item in config["models"]}
    host = os.environ.get("SONAR_HOST_URL", "http://localhost:9000").rstrip("/")
    def process_run(index, run):
        path = ROOT / run["path"]
        properties = path / "sonar-project.properties"
        if not path.is_dir() or not properties.is_file():
            raise FileNotFoundError(f"execução inválida: {run['path']}")
        project_key = next(line.split("=", 1)[1] for line in properties.read_text(encoding="utf-8").splitlines() if line.startswith("sonar.projectKey="))
        if args.execute:
            execute_run(run, scenarios[run["scenario"]], path)
            wait_for_analysis(host, project_key, args.timeout)
        payload = sonar_metrics(host, project_key)
        (path / "sonar_output.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        values = {item["metric"]: item["value"] for item in payload["component"].get("measures", [])}
        return index, (models[run["model"]]["opencode_model"], run["scenario"], run.get("opencode_cost_usd"), values)

    rows_with_index = []
    with concurrent.futures.ThreadPoolExecutor() as executor:
        futures = {executor.submit(process_run, i, run): run for i, run in enumerate(config["runs"])}
        for future in concurrent.futures.as_completed(futures):
            try:
                rows_with_index.append(future.result())
            except Exception as exc:
                print(f"run falhou: {exc}", file=sys.stderr)
    
    rows_with_index.sort(key=lambda x: x[0])
    rows = [r[1] for r in rows_with_index]
    headers = ["Modelo", "Cenário", "Custo OpenCode (USD)", "Cobertura", "Bugs", "Vulnerabilidades", "Code Smells", "Violações", "Complexidade", "Complexidade Cognitiva", "Dívida Técnica (min)"]
    lines = ["| " + " | ".join(headers) + " |", "|" + "|".join(["---"] * len(headers)) + "|"]
    for model, scenario, cost, values in rows:
        data = [model, scenario, "" if cost is None else str(cost), values.get("coverage", ""), values.get("bugs", ""), values.get("vulnerabilities", ""), values.get("code_smells", ""), values.get("violations", ""), values.get("complexity", ""), values.get("cognitive_complexity", ""), values.get("sqale_index", "")]
        lines.append("| " + " | ".join(data) + " |")
    (ROOT / "analysis_report.md").write_text("\n".join(lines) + "\n", encoding="utf-8")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro: {error}", file=sys.stderr)
        sys.exit(1)
