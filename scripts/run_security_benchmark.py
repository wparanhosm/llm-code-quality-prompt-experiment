import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))

import argparse
import base64
import datetime as dt
import json
import os
import re
import shutil
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
import concurrent.futures
import threading

import run_benchmark as base



ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONFIG = ROOT / "benchmark" / "campaigns" / "security-sonar.json"
SONAR_SEMAPHORE = threading.Semaphore(1)


def count_json_lines(path, key):
    count = 0
    if not path.is_file():
        return count
    with path.open(encoding="utf-8", errors="replace") as stream:
        for line in stream:
            try:
                payload = json.loads(line)
            except json.JSONDecodeError:
                continue
            if key in payload:
                count += 1
    return count


def parse_gosec(path):
    if not path.is_file():
        return {"issues": None, "severity": {}}
    try:
        payload = base.read_json(path)
    except json.JSONDecodeError:
        return {"issues": None, "severity": {}}
    issues = payload.get("Issues") or []
    severity = {}
    for issue in issues:
        level = str(issue.get("severity", "unknown")).lower()
        severity[level] = severity.get(level, 0) + 1
    return {"issues": len(issues), "severity": severity}


def parse_lint(path):
    if not path.is_file():
        return {"issues": None, "linters": {}}
    try:
        payload = base.read_json(path)
    except json.JSONDecodeError:
        return {"issues": None, "linters": {}}
    issues = payload.get("Issues") or []
    linters = {}
    for issue in issues:
        name = issue.get("FromLinter", "unknown")
        linters[name] = linters.get(name, 0) + 1
    return {"issues": len(issues), "linters": dict(sorted(linters.items()))}


def request_json(host, path, token=None, username=None, password=None, data=None):
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    elif username and password:
        credentials = base64.b64encode(f"{username}:{password}".encode("utf-8")).decode("ascii")
        headers["Authorization"] = f"Basic {credentials}"
    if data is not None:
        data = urllib.parse.urlencode(data).encode("ascii")
        headers["Content-Type"] = "application/x-www-form-urlencoded"
    
    # Adicionado log de debug de requisição
    # print(f"  [http] requisitando {host}{path}", flush=True)
    
    request = urllib.request.Request(f"{host}{path}", headers=headers, data=data)
    with urllib.request.urlopen(request, timeout=30) as response:
        content = response.read()
        return json.loads(content) if content else {}


def create_temporary_token(host, username, password, execution_id):
    name = f"lqcpe-{execution_id}"
    print(f"  [sonar-api] criando token '{name}'...", flush=True)
    payload = request_json(
        host,
        "/api/user_tokens/generate",
        username=username,
        password=password,
        data={"name": name, "type": "GLOBAL_ANALYSIS_TOKEN"},
    )
    return name, payload["token"]


def revoke_temporary_token(host, username, password, name):
    print(f"  [sonar-api] revogando token '{name}'...", flush=True)
    request_json(
        host,
        "/api/user_tokens/revoke",
        username=username,
        password=password,
        data={"name": name},
    )


def sonar_ready(config):
    try:
        payload = request_json(config["host_url"], "/api/system/status")
        return payload.get("status") == "UP"
    except (OSError, urllib.error.URLError, json.JSONDecodeError):
        return False


def wait_sonar(config, ce_task_id, token, log_prefix=""):
    deadline = time.monotonic() + config["timeout_seconds"]
    last_status = None
    while time.monotonic() < deadline:
        query = urllib.parse.urlencode({"id": ce_task_id})
        current = request_json(config["host_url"], f"/api/ce/task?{query}", token).get("task")
        if not current:
            if last_status != "waiting_for_task":
                print(f"  {log_prefix}[sonar] aguardando a tarefa aparecer na fila...", flush=True)
                last_status = "waiting_for_task"
            time.sleep(2)
            continue
        if current["status"] != last_status:
            print(f"  {log_prefix}[sonar] processamento no servidor: {current['status']}", flush=True)
            last_status = current["status"]
        if current["status"] == "SUCCESS":
            return current
        if current["status"] in {"FAILED", "CANCELED"}:
            raise RuntimeError(f"Sonar finalizou com {current['status']}")
        time.sleep(3)
    raise TimeoutError("timeout aguardando SonarQube")


def run_sonar(run, work_source, artifact, coverage, config, execution_id, environment, log_prefix=""):
    token = environment.get("SONAR_TOKEN")
    print(f"  {log_prefix}[sonar] verificando disponibilidade do servidor...", flush=True)
    if not sonar_ready(config):
        print(f"  {log_prefix}[sonar] indisponivel; etapa ignorada", flush=True)
        return {"status": "unavailable", "reason": "SonarQube is not ready"}
    if not token:
        print(f"  {log_prefix}[sonar] SONAR_TOKEN ausente; etapa ignorada", flush=True)
        return {"status": "skipped", "reason": "SONAR_TOKEN is not set"}

    output = artifact / "sonar"
    output.mkdir(parents=True, exist_ok=True)
    project_key = re.sub(r"[^a-zA-Z0-9_.:-]", "-", f"lqcpe-{run['id']}-{execution_id}")
    
    print(f"  {log_prefix}[sonar] iniciando analise no container docker...", flush=True)
    
    docker_user_args = []
    if os.name != "nt":
        docker_user_args = ["--user", f"{os.getuid()}:{os.getgid()}"]

    command = [
        "docker", "run", "--rm"
    ] + docker_user_args + [
        "--network", config["network"],
        "-e", f"SONAR_HOST_URL={config['container_host_url']}",
        "-e", "SONAR_TOKEN",
        "-v", f"{work_source.resolve()}:/workspace/source:ro",
        "-v", f"{output.resolve()}:/workspace/artifacts",
        config["scanner_image"],
        "-D", f"sonar.projectKey={project_key}",
        "-D", f"sonar.projectName={run['id']}",
        "-D", "sonar.projectBaseDir=/workspace/source",
        "-D", "sonar.sources=.",
        "-D", "sonar.exclusions=**/*_test.go,**/.opencode/**,**/node_modules/**,**/coverage.out,**/*.db,**/bin/**",
        "-D", "sonar.tests=.",
        "-D", "sonar.test.inclusions=**/*_test.go",
        "-D", "sonar.go.coverage.reportPaths=/workspace/coverage/generated-tests.out",
        "-D", "sonar.working.directory=/workspace/artifacts/scannerwork"
    ]
    # The coverage mount stays separate because the source snapshot is read-only.
    command[command.index(config["scanner_image"]):command.index(config["scanner_image"])] = [
        "-v", f"{coverage.parent.resolve()}:/workspace/coverage:ro"
    ]
    with SONAR_SEMAPHORE:
        result = base.run_command("sonar", command, work_source, environment, config["timeout_seconds"], artifact / "logs", log_prefix=log_prefix)
    if result["status"] != "passed":
        print(f"  {log_prefix}[sonar] falha na execucao local: status {result['status']}", flush=True)
        return {"status": result["status"], "project_key": project_key, "command": result}
    print(f"  {log_prefix}[sonar] upload concluido; aguardando analise no servidor...", flush=True)
    report_task = {}
    for line in (output / "scannerwork" / "report-task.txt").read_text(encoding="utf-8").splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            report_task[key] = value
    wait_sonar(config, report_task["ceTaskId"], token, log_prefix=log_prefix)
    print(f"  {log_prefix}[sonar] coletando metricas da API...", flush=True)
    query = urllib.parse.urlencode({"component": project_key, "metricKeys": config["metrics"]})
    measures = request_json(config["host_url"], f"/api/measures/component?{query}", token)
    base.write_json(output / "measures.json", measures)
    values = {item["metric"]: item.get("value") for item in measures.get("component", {}).get("measures", [])}
    print(f"  {log_prefix}[sonar] concluido: {len(values)} metricas coletadas", flush=True)
    return {
        "status": "passed",
        "project_key": project_key,
        "ce_task_id": report_task["ceTaskId"],
        "server_version": report_task.get("serverVersion"),
        "measures": values,
        "command": result
    }


def evaluate(config, campaign, run, execution_root):
    log_prefix = f"[{run['id']}] "
    source = ROOT / run["source"]
    work_source = ROOT / "work" / execution_root.name / run["id"] / "source"
    artifact = execution_root / run["id"]
    logs = artifact / "logs"
    security = artifact / "security"
    lint = artifact / "lint"
    coverage = artifact / "coverage" / "generated-tests.out"
    
    print(f"  {log_prefix}[workspace] preparando diretorios e copiando snapshot...", flush=True)
    for directory in (logs, security, lint, coverage.parent):
        directory.mkdir(parents=True, exist_ok=True)
    base.materialize(source, work_source)
    print(f"  {log_prefix}[workspace] preparacao concluida", flush=True)

    print(f"  {log_prefix}[env] configurando variaveis de ambiente locais...", flush=True)
    environment = os.environ.copy()
    environment.update(run.get("environment", {}))
    environment.update({
        "GOCACHE": str(ROOT / "cache" / "go-build" / run["id"]),
        "GOMODCACHE": str(ROOT / "cache" / "go-mod"),
        "GOLANGCI_LINT_CACHE": str(ROOT / "cache" / "golangci-lint" / run["id"]),
        "GOTMPDIR": str(ROOT / "work" / execution_root.name / run["id"] / "tmp"),
        "HOME": str(ROOT / "work" / execution_root.name / run["id"] / "home"),
    })
    for name in ("GOCACHE", "GOMODCACHE", "GOLANGCI_LINT_CACHE", "GOTMPDIR", "HOME"):
        Path(environment[name]).mkdir(parents=True, exist_ok=True)

    commands = {}
    outputs = {
        "govulncheck": security / "govulncheck.jsonl",
        "gosec": security / "gosec.json",
        "golangci_lint": lint / "golangci-lint.json"
    }

    def run_coverage():
        print(f"  {log_prefix}[exec] rodando testes de cobertura (coverage)...", flush=True)
        coverage_command = [part.format(coverage_output=str(coverage)) for part in campaign["commands"]["coverage"]]
        coverage_result = base.run_command("coverage", coverage_command, work_source, environment, campaign["timeouts_seconds"]["coverage"], logs, log_prefix=log_prefix)
        print(f"  {log_prefix}[exec] coverage concluido com status: {coverage_result['status']}", flush=True)
        return "coverage", coverage_result

    def run_local_tool(name):
        print(f"  {log_prefix}[tools] executando {name}...", flush=True)
        spec = config["tools"][name]
        tool_path = (ROOT / spec["path"]).resolve()
        command = [part.format(tool=str(tool_path), output=str(outputs[name])) for part in spec["command"]]
        result = base.run_command(name, command, work_source, environment, config["timeouts_seconds"][name], logs, log_prefix=log_prefix)
        if name == "govulncheck":
            shutil.copyfile(logs / "govulncheck.stdout.log", outputs[name])
        print(f"  {log_prefix}[tools] {name} concluido com status: {result['status']}", flush=True)
        return name, result

    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
        tool_futures = {executor.submit(run_local_tool, name): name for name in ("govulncheck", "gosec", "golangci_lint")}
        tool_futures[executor.submit(run_coverage)] = "coverage"
        for future in concurrent.futures.as_completed(tool_futures):
            name, result = future.result()
            commands[name] = result

    print(f"  {log_prefix}[sonar] iniciando avaliacao do sonar...", flush=True)
    sonar_config = dict(config["sonar"])
    sonar_config["timeout_seconds"] = config["timeouts_seconds"]["sonar"]
    sonar = run_sonar(run, work_source, artifact, coverage, sonar_config, execution_root.name, environment, log_prefix=log_prefix)
    
    print(f"  {log_prefix}[resultado] empacotando metricas...", flush=True)
    result = {
        "schema_version": "1.0",
        "run_id": run["id"],
        "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run["id"] / "source-manifest.json")["root_hash"],
        "commands": commands,
        "tools": {name: {"version": spec["version"], "path": spec["path"]} for name, spec in config["tools"].items()},
        "security": {
            "govulncheck": {
                "status": commands["govulncheck"]["status"],
                "vulnerability_events": count_json_lines(outputs["govulncheck"], "finding")
            },
            "gosec": {"status": commands["gosec"]["status"], **parse_gosec(outputs["gosec"])},
        },
        "lint": {"status": commands["golangci_lint"]["status"], **parse_lint(outputs["golangci_lint"])},
        "sonar": sonar
    }
    base.write_json(artifact / "summary.json", result)
    print(f"  {log_prefix}[resultado] salvo com sucesso em {artifact.relative_to(ROOT)}\n", flush=True)
    return result


def report(config, execution_id, results, output):
    print(f"[report] gerando relatorio consolidado em {output.name}...", flush=True)
    lines = [
        f"# Benchmark {config['campaign_id']}", "", f"Execucao: `{execution_id}`", "",
        "| Run | govulncheck | gosec | Lint | Sonar |",
        "|---|---:|---:|---:|---|"
    ]
    for item in results:
        if item.get("status") == "error":
            lines.append(f'| {item["run_id"]} | ERROR | ERROR | ERROR | ERROR |')
            continue
        vuln = item["security"]["govulncheck"]
        gosec = item["security"]["gosec"]
        lint = item["lint"]
        lines.append(
            f'| {item["run_id"]} | {vuln["vulnerability_events"]} | '
            f'{gosec.get("issues", "")} | {lint.get("issues", "")} | {item["sonar"]["status"]} |'
        )
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("[report] relatorio gerado com sucesso.", flush=True)


def main():
    parser = argparse.ArgumentParser(description="Executa o segundo estagio de seguranca, lint e Sonar.")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--run")
    parser.add_argument("--keep-work", action="store_true")
    args = parser.parse_args()
    
    print("[init] lendo arquivo de configuracao...", flush=True)
    config = base.read_json(args.config)
    campaign = base.read_json(ROOT / config["base_campaign"])
    selected = [item for item in campaign["runs"] if not args.run or item["id"] == args.run]
    if not selected:
        raise ValueError(f"run inexistente: {args.run}")

    print("[init] verificando repositorios/snapshots...", flush=True)
    subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
    
    execution_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    execution_root = ROOT / "artifacts" / config["campaign_id"] / execution_id
    results = []
    temporary_token_name = None
    sonar_user = os.environ.get("SONAR_USER")
    sonar_password = os.environ.get("SONAR_PASSWORD")
    
    if not os.environ.get("SONAR_TOKEN") and sonar_user and sonar_password:
        print("[sonar] criando token temporario para a campanha...", flush=True)
        temporary_token_name, temporary_token = create_temporary_token(
            config["sonar"]["host_url"], sonar_user, sonar_password, execution_id
        )
        os.environ["SONAR_TOKEN"] = temporary_token
        print("[sonar] token temporario criado e adicionado ao ambiente", flush=True)
        
    print(f"[campanha] {config['campaign_id']} iniciada ({len(selected)} runs)", flush=True)
    print(f"[campanha] artefatos serao salvos em: {execution_root.relative_to(ROOT)}", flush=True)
    
    try:
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            future_to_run = {executor.submit(evaluate, config, campaign, run, execution_root): run for run in selected}
            for future in concurrent.futures.as_completed(future_to_run):
                run = future_to_run[future]
                try:
                    results.append(future.result())
                except Exception as exc:
                    print(f"\n[run] run {run['id']} gerou exceção: {exc}", flush=True)
                    failure = {
                        "schema_version": "1.0",
                        "run_id": run["id"],
                        "status": "error",
                        "error": str(exc)
                    }
                    base.write_json(execution_root / run["id"] / "failure.json", failure)
                    results.append(failure)
            
        results.sort(key=lambda item: item["run_id"])
            
        print("\n[campanha] processamento das runs concluido. consolidando sumario global...", flush=True)
        base.write_json(execution_root / "summary.json", {"campaign": config["campaign_id"], "execution_id": execution_id, "runs": results})
        report(config, execution_id, results, execution_root / "report.md")
        
    finally:
        print("\n[cleanup] verificando consistencia final...", flush=True)
        subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
        if temporary_token_name:
            try:
                print("[sonar] revogando token temporario...", flush=True)
                revoke_temporary_token(
                    config["sonar"]["host_url"], sonar_user, sonar_password, temporary_token_name
                )
                print("[sonar] token temporario revogado com sucesso", flush=True)
            finally:
                os.environ.pop("SONAR_TOKEN", None)
        if not args.keep_work:
            print("[workspace] limpando arquivos temporarios...", flush=True)
            shutil.rmtree(ROOT / "work" / execution_id, ignore_errors=True)
            
    print(f"\n[campanha] finalizada com sucesso! Resultados em: {execution_root.relative_to(ROOT)}", flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro critico: {error}", file=sys.stderr)
        sys.exit(1)
