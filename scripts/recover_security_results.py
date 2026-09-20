import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))

import argparse
import json
import os
import urllib.parse

import run_benchmark as base
import run_security_benchmark as security



ROOT = Path(__file__).resolve().parents[1]


def report_task(path):
    values = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            values[key] = value
    return values


def command_status(logs, name):
    stdout = logs / f"{name}.stdout.log"
    stderr = logs / f"{name}.stderr.log"
    return {
        "name": name,
        "status": "completed" if stdout.is_file() and stderr.is_file() else "missing",
        "stdout": stdout.relative_to(logs.parent).as_posix() if stdout.is_file() else None,
        "stderr": stderr.relative_to(logs.parent).as_posix() if stderr.is_file() else None,
    }


def recover_run(config, run, execution_root, token):
    artifact = execution_root / run["id"]
    logs = artifact / "logs"
    task_path = artifact / "sonar" / "scannerwork" / "report-task.txt"
    task = report_task(task_path)
    security.wait_sonar(config, task["ceTaskId"], token, log_prefix=f"[{run['id']}] ")

    query = urllib.parse.urlencode({"component": task["projectKey"], "metricKeys": config["metrics"]})
    measures = security.request_json(config["host_url"], f"/api/measures/component?{query}", token)
    base.write_json(artifact / "sonar" / "measures.json", measures)
    values = {item["metric"]: item.get("value") for item in measures.get("component", {}).get("measures", [])}

    issues_query = urllib.parse.urlencode({"componentKeys": task["projectKey"], "ps": 500})
    issues = security.request_json(config["host_url"], f"/api/issues/search?{issues_query}", token)
    base.write_json(artifact / "sonar" / "issues.json", issues)

    gate_query = urllib.parse.urlencode({"projectKey": task["projectKey"]})
    gate = security.request_json(config["host_url"], f"/api/qualitygates/project_status?{gate_query}", token)
    base.write_json(artifact / "sonar" / "quality-gate.json", gate)

    govuln = artifact / "security" / "govulncheck.jsonl"
    gosec = artifact / "security" / "gosec.json"
    lint = artifact / "lint" / "golangci-lint.json"
    commands = {name: command_status(logs, name) for name in ("coverage", "govulncheck", "gosec", "golangci_lint", "sonar")}
    result = {
        "schema_version": "1.0",
        "run_id": run["id"],
        "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run["id"] / "source-manifest.json")["root_hash"],
        "recovered_from_completed_artifacts": True,
        "commands": commands,
        "security": {
            "govulncheck": {
                "status": commands["govulncheck"]["status"],
                "vulnerability_events": security.count_json_lines(govuln, "finding")
            },
            "gosec": {"status": commands["gosec"]["status"], **security.parse_gosec(gosec)}
        },
        "lint": {"status": commands["golangci_lint"]["status"], **security.parse_lint(lint)},
        "sonar": {
            "status": "passed",
            "project_key": task["projectKey"],
            "ce_task_id": task["ceTaskId"],
            "server_version": task.get("serverVersion"),
            "measures": values,
            "issue_count": issues.get("total", 0),
            "quality_gate": gate.get("projectStatus", {}).get("status")
        }
    }
    base.write_json(artifact / "summary.json", result)
    return result


def main():
    parser = argparse.ArgumentParser(description="Recupera uma campanha cujo upload Sonar terminou sem consolidacao.")
    parser.add_argument("execution_id")
    args = parser.parse_args()
    campaign_config = base.read_json(ROOT / "benchmark" / "campaigns" / "security-sonar.json")
    campaign = base.read_json(ROOT / campaign_config["base_campaign"])
    sonar = dict(campaign_config["sonar"])
    sonar["timeout_seconds"] = campaign_config["timeouts_seconds"]["sonar"]
    execution_root = ROOT / "artifacts" / campaign_config["campaign_id"] / args.execution_id

    token = os.environ.get("SONAR_TOKEN")
    temporary_name = None
    username = os.environ.get("SONAR_USER")
    password = os.environ.get("SONAR_PASSWORD")
    if not token and username and password:
        temporary_name, token = security.create_temporary_token(sonar["host_url"], username, password, f"recover-{args.execution_id}")
    if not token:
        raise RuntimeError("Defina SONAR_TOKEN ou SONAR_USER e SONAR_PASSWORD")

    results = []
    try:
        for index, run in enumerate(campaign["runs"], 1):
            print(f"[recover {index}/{len(campaign['runs'])}] {run['id']}", flush=True)
            results.append(recover_run(sonar, run, execution_root, token))
        base.write_json(execution_root / "summary.json", {"campaign": campaign_config["campaign_id"], "execution_id": args.execution_id, "recovered": True, "runs": results})
        security.report(campaign_config, args.execution_id, results, execution_root / "report.md")
    finally:
        if temporary_name:
            security.revoke_temporary_token(sonar["host_url"], username, password, temporary_name)
    print(f"resultados recuperados em {execution_root.relative_to(ROOT)}", flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro: {error}", file=sys.stderr)
        sys.exit(1)
