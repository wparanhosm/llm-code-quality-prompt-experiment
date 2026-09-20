import argparse
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import time
import concurrent.futures
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CAMPAIGN = ROOT / "benchmark" / "campaigns" / "initial-quality.json"


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path, payload):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def parse_billing(source):
    candidates = [source / "billing_result.txt", source / "billing_results.txt"]
    path = next((item for item in candidates if item.is_file()), None)
    if not path:
        return {"status": "missing"}

    fields = {}
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            fields[key.strip().lower()] = value.strip()

    duration = fields.get("time", "")
    minutes = re.search(r"(\d+)\s*m", duration)
    seconds = re.search(r"(\d+)\s*s", duration)
    token_match = re.search(r"\d+", fields.get("tokens", "").replace(",", ""))
    cost_match = re.search(r"\d+(?:[.,]\d+)?", fields.get("spent", ""))
    return {
        "status": "parsed",
        "source_file": path.name,
        "model": fields.get("model"),
        "time_raw": duration or None,
        "time_seconds": (int(minutes.group(1)) * 60 if minutes else 0) + (int(seconds.group(1)) if seconds else 0),
        "tokens": int(token_match.group()) if token_match else None,
        "spent_usd": float(cost_match.group().replace(",", ".")) if cost_match else None,
    }


def tool(command):
    path = shutil.which(command)
    return {"available": bool(path), "path": path}


def run_command(name, command, cwd, environment, timeout, logs, log_prefix=""):
    print(f"  {log_prefix}[{name}] iniciando: {' '.join(command)}", flush=True)
    started_at = dt.datetime.now(dt.timezone.utc)
    started = time.monotonic()
    stdout_path = logs / f"{name}.stdout.log"
    stderr_path = logs / f"{name}.stderr.log"
    status = "completed"
    exit_code = None
    error = None
    try:
        with stdout_path.open("wb") as stdout, stderr_path.open("wb") as stderr:
            process = subprocess.run(
                command,
                cwd=cwd,
                env=environment,
                stdout=stdout,
                stderr=stderr,
                timeout=timeout,
                check=False,
            )
        exit_code = process.returncode
        status = "passed" if exit_code == 0 else "failed"
    except subprocess.TimeoutExpired:
        status = "timeout"
        error = f"timeout after {timeout} seconds"
    except OSError as exc:
        status = "unavailable"
        error = str(exc)
    duration = round(time.monotonic() - started, 3)
    detail = f"exit={exit_code}" if exit_code is not None else (error or status)
    print(f"  {log_prefix}[{name}] {status} em {duration:.1f}s ({detail})", flush=True)
    return {
        "name": name,
        "command": command,
        "started_at": started_at.isoformat(),
        "duration_seconds": duration,
        "status": status,
        "exit_code": exit_code,
        "error": error,
        "stdout": stdout_path.relative_to(logs.parent).as_posix(),
        "stderr": stderr_path.relative_to(logs.parent).as_posix(),
    }


def coverage_total(profile, cwd, environment):
    if not profile.is_file():
        return None
    process = subprocess.run(
        ["go", "tool", "cover", f"-func={profile}"],
        cwd=cwd,
        env=environment,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    match = re.search(r"^total:\s+\(statements\)\s+([0-9.]+)%", process.stdout, re.MULTILINE)
    return float(match.group(1)) if match else None


def test_summary(log_path):
    summary = {"tests_run": 0, "tests_passed": 0, "tests_failed": 0, "tests_skipped": 0}
    if not log_path.is_file():
        return summary
    with log_path.open(encoding="utf-8", errors="replace") as stream:
        for line in stream:
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue
            if not event.get("Test") or event.get("Action") not in {"pass", "fail", "skip"}:
                continue
            summary["tests_run"] += 1
            summary[f'tests_{"passed" if event["Action"] == "pass" else "failed" if event["Action"] == "fail" else "skipped"}'] += 1
    return summary


def materialize(source, work_source):
    if work_source.exists():
        shutil.rmtree(work_source)
    shutil.copytree(source, work_source, symlinks=True)


def evaluate(campaign, run, execution_root):
    source = ROOT / run["source"]
    provenance = read_json(ROOT / "provenance" / "snapshots" / run["id"] / "source-manifest.json")
    work_source = ROOT / "work" / execution_root.name / run["id"] / "source"
    artifact = execution_root / run["id"]
    logs = artifact / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    materialize(source, work_source)

    environment = os.environ.copy()
    environment.update(run.get("environment", {}))
    environment.update({
        "GOCACHE": str(ROOT / "cache" / "go-build" / run["id"]),
        "GOMODCACHE": str(ROOT / "cache" / "go-mod"),
        "GOTMPDIR": str(ROOT / "work" / execution_root.name / run["id"] / "tmp"),
        "HOME": str(ROOT / "work" / execution_root.name / run["id"] / "home"),
    })
    for directory in (Path(environment["GOCACHE"]), Path(environment["GOMODCACHE"]), Path(environment["GOTMPDIR"]), Path(environment["HOME"])):
        directory.mkdir(parents=True, exist_ok=True)

    coverage = artifact / "coverage" / "generated-tests.out"
    coverage.parent.mkdir(parents=True, exist_ok=True)
    commands = []
    for name in ("format", "build", "vet", "test", "race", "coverage"):
        command = [part.format(coverage_output=str(coverage)) for part in campaign["commands"][name]]
        commands.append(run_command(name, command, work_source, environment, campaign["timeouts_seconds"][name], logs, log_prefix=f"[{run['id']}] "))

    test_data = test_summary(logs / "test.stdout.log")
    test_data["coverage_percent"] = coverage_total(coverage, work_source, environment)
    status_by_name = {item["name"]: item["status"] for item in commands}
    summary = {
        "schema_version": "1.0",
        "run_id": run["id"],
        "source_path": run["source"],
        "source_root_hash": provenance["root_hash"],
        "declared_go_version": run["go_version"],
        "billing": parse_billing(source),
        "commands": commands,
        "tests": test_data,
        "quality": {
            "format_clean": status_by_name.get("format") == "passed" and (logs / "format.stdout.log").stat().st_size == 0,
            "build_passed": status_by_name.get("build") == "passed",
            "vet_passed": status_by_name.get("vet") == "passed",
            "tests_passed": status_by_name.get("test") == "passed",
            "race_passed": status_by_name.get("race") == "passed",
            "coverage_generated": status_by_name.get("coverage") == "passed",
        },
        "security": {
            "govulncheck": {"status": "not_run", **tool("govulncheck")},
            "gosec": {"status": "not_run", **tool("gosec")},
            "golangci_lint": {"status": "not_run", **tool("golangci-lint")},
            "sonar": {"status": "not_run", "reason": "requires explicit scanner credentials/configuration"}
        }
    }
    write_json(artifact / "summary.json", summary)
    return summary


def render_report(campaign, execution_id, summaries, output):
    headers = ["Run", "Build", "Vet", "Testes", "Race", "Cobertura", "Tokens", "Custo USD"]
    lines = [f"# Benchmark {campaign['campaign_id']}", "", f"Execucao: `{execution_id}`", "", "| " + " | ".join(headers) + " |", "|" + "|".join(["---"] * len(headers)) + "|"]
    for item in summaries:
        quality = item["quality"]
        tests = item["tests"]
        billing = item["billing"]
        test_result = f'{tests["tests_passed"]}/{tests["tests_run"]}'
        lines.append("| " + " | ".join([
            item["run_id"],
            "PASS" if quality["build_passed"] else "FAIL",
            "PASS" if quality["vet_passed"] else "FAIL",
            test_result,
            "PASS" if quality["race_passed"] else "FAIL",
            "" if tests["coverage_percent"] is None else f'{tests["coverage_percent"]:.1f}%',
            "" if billing.get("tokens") is None else str(billing["tokens"]),
            "" if billing.get("spent_usd") is None else f'{billing["spent_usd"]:.4f}',
        ]) + " |")
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="Executa benchmark isolado sem modificar runs/.")
    parser.add_argument("--campaign", type=Path, default=DEFAULT_CAMPAIGN)
    parser.add_argument("--run", help="executa somente um run_id")
    parser.add_argument("--keep-work", action="store_true")
    args = parser.parse_args()
    campaign = read_json(args.campaign)
    selected = [item for item in campaign["runs"] if not args.run or item["id"] == args.run]
    if not selected:
        raise ValueError(f"run inexistente: {args.run}")

    subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
    execution_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    execution_root = ROOT / "artifacts" / campaign["campaign_id"] / execution_id
    summaries = []
    try:
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future_to_run = {executor.submit(evaluate, campaign, run, execution_root): run for run in selected}
            for future in concurrent.futures.as_completed(future_to_run):
                run = future_to_run[future]
                try:
                    summaries.append(future.result())
                    print(f"avaliado {run['id']} com sucesso", flush=True)
                except Exception as exc:
                    print(f"run {run['id']} gerou exceção: {exc}", flush=True)
        summaries.sort(key=lambda item: item["run_id"])
        write_json(execution_root / "summary.json", {"campaign": campaign["campaign_id"], "execution_id": execution_id, "runs": summaries})
        render_report(campaign, execution_id, summaries, execution_root / "report.md")
    finally:
        subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
        if not args.keep_work:
            shutil.rmtree(ROOT / "work" / execution_id, ignore_errors=True)
    print(execution_root.relative_to(ROOT))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro: {error}", file=sys.stderr)
        sys.exit(1)
