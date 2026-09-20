import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))

import argparse
import datetime as dt
import json
import os
import shutil
import signal
import socket
import sqlite3
import subprocess
import time
import urllib.error
import urllib.request

import run_benchmark as base


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONFIG = ROOT / "benchmark" / "campaigns" / "functional-smoke.json"


def phase(status, started, error=None, **details):
    result = {"status": status, "duration_seconds": round(time.monotonic() - started, 3)}
    if error:
        result["error"] = str(error)
    result.update(details)
    return result


def expand(value, variables):
    result = str(value)
    for key, replacement in variables.items():
        result = result.replace("{" + key + "}", str(replacement))
    if "{" in result or "}" in result:
        raise ValueError(f"placeholder nao resolvido: {result}")
    return result


def free_port():
    with socket.socket() as server:
        server.bind(("127.0.0.1", 0))
        return server.getsockname()[1]


def docker(command, *, input_text=None, check=True):
    return subprocess.run(
        ["docker", *command], input=input_text, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=check
    )


def start_postgres(config, run_id, execution_id):
    name = f"lqcpe-fn-{execution_id.lower()}-{run_id}"[:63]
    user = config["postgres_user"]
    password = config["postgres_password"]
    database = run_id.replace("-", "_")
    result = docker([
        "run", "-d", "--rm", "--name", name,
        "--label", f"lqcpe.execution={execution_id}",
        "-e", f"POSTGRES_USER={user}", "-e", f"POSTGRES_PASSWORD={password}",
        "-e", f"POSTGRES_DB={database}", "-p", "127.0.0.1::5432", config["postgres_image"]
    ])
    container_id = result.stdout.strip()
    deadline = time.monotonic() + config["timeouts_seconds"]["database"]
    while time.monotonic() < deadline:
        ready = docker(["exec", name, "pg_isready", "-U", user, "-d", database], check=False)
        if ready.returncode == 0:
            port_output = docker(["port", name, "5432/tcp"]).stdout.strip()
            port = int(port_output.rsplit(":", 1)[1])
            return {
                "name": name, "container_id": container_id, "host": "127.0.0.1", "port": port,
                "database": database, "user": user, "password": password,
                "dsn": f"postgres://{user}:{password}@127.0.0.1:{port}/{database}?sslmode=disable"
            }
        time.sleep(1)
    raise TimeoutError("PostgreSQL nao ficou pronto")


def psql(database, sql, *, tuples=False):
    command = [
        "exec", "-i", database["name"], "psql", "--no-psqlrc", "-v", "ON_ERROR_STOP=1",
        "-U", database["user"], "-d", database["database"]
    ]
    if tuples:
        command.extend(["--tuples-only", "--no-align"])
    return docker(command, input_text=sql).stdout.strip()


def sql_content(run, work_source, key):
    if key == "migration":
        return (work_source / run[key]).read_text(encoding="utf-8")
    path = ROOT / run[key]
    return path.read_text(encoding="utf-8")


def query_postgres(database, sql):
    output = psql(database, sql, tuples=True)
    return [int(value.strip()) for value in output.splitlines() if value.strip()]


def query_sqlite(path, sql):
    with sqlite3.connect(path) as connection:
        return [int(row[0]) for row in connection.execute(sql).fetchall()]


def http_request(port, spec, timeout):
    url = f"http://127.0.0.1:{port}{spec['path']}"
    data = json.dumps(spec.get("body", {})).encode("utf-8") if "body" in spec else None
    headers = {"Content-Type": "application/json"} if data is not None else {}
    request = urllib.request.Request(url, data=data, headers=headers, method=spec["method"])
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8", errors="replace")
            status = response.status
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        status = error.code
    try:
        payload = json.loads(body) if body else None
    except json.JSONDecodeError:
        payload = None
    return {"status_code": status, "expected": status in spec["expected_status"], "json": payload, "body": body[:4000]}


def wait_ready(process, port, readiness, timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError(f"aplicacao encerrou antes do readiness com exit {process.returncode}")
        try:
            if readiness["type"] == "tcp":
                with socket.create_connection(("127.0.0.1", port), timeout=1):
                    return
            else:
                spec = {"method": "GET", "path": readiness["path"], "expected_status": [200]}
                if http_request(port, spec, 2)["expected"]:
                    return
        except (OSError, urllib.error.URLError):
            pass
        time.sleep(0.5)
    raise TimeoutError("aplicacao nao ficou pronta")


def stop_process(process, timeout):
    if process is None or process.poll() is not None:
        return "already_stopped"
    try:
        os.killpg(process.pid, signal.SIGTERM)
        process.wait(timeout=timeout)
        return "graceful"
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=5)
        return "killed"


def check_state(before, after, ledger, expected):
    checks = {
        "initial_state_matches": before == expected["before"],
        "source_debited_once": after[0] == expected["after"][0] if len(after) == 2 else False,
        "destination_credited_once": after[1] == expected["after"][1] if len(after) == 2 else False,
        "total_conserved": len(before) == 2 and len(after) == 2 and sum(before) == sum(after),
        "ledger_recorded_once": ledger == expected["ledger"],
        "non_negative_balances": len(after) == 2 and all(value >= 0 for value in after),
    }
    return checks, all(checks.values())


def evaluate(config, run, execution_root):
    print(f"\n[funcional] {run['id']}", flush=True)
    source = ROOT / run["source"]
    artifact = execution_root / run["id"]
    work = ROOT / "work" / execution_root.name / run["id"]
    work_source = work / "source"
    runtime = work / "runtime"
    binary = work / "bin" / "application"
    logs = artifact / "logs"
    for path in (runtime, binary.parent, logs):
        path.mkdir(parents=True, exist_ok=True)
    phases = {}
    database = None
    process = None
    stdout_file = None
    stderr_file = None
    response = None
    before = []
    after = []
    ledger = 0
    fallbacks = []
    error = None

    try:
        started = time.monotonic()
        base.materialize(source, work_source)
        phases["materialize"] = phase("passed", started)

        http_port = run.get("fixed_http_port", free_port())
        sqlite_path = runtime / Path(run.get("sqlite_file", "runtime.db")).name
        variables = {
            "binary": binary, "http_port": http_port, "sqlite_path": sqlite_path,
            "postgres_port": "", "postgres_database": "", "postgres_dsn": ""
        }

        if run["database"] == "postgres":
            started = time.monotonic()
            database = start_postgres(config, run["id"], execution_root.name)
            variables.update({"postgres_port": database["port"], "postgres_database": database["database"], "postgres_dsn": database["dsn"]})
            phases["database"] = phase("passed", started, image=config["postgres_image"])

            started = time.monotonic()
            migration_key = "migration" if "migration" in run else "migration_external"
            psql(database, sql_content(run, work_source, migration_key))
            phases["migration"] = phase("passed", started, source=run[migration_key])
            started = time.monotonic()
            psql(database, sql_content(run, work_source, "seed"))
            phases["seed"] = phase("passed", started, source=run["seed"])
            before = query_postgres(database, run["state"]["balances"])

        environment = os.environ.copy()
        environment.update(run.get("environment", {}))
        environment.update({
            "GOCACHE": str(ROOT / "cache" / "go-build" / run["id"]),
            "GOMODCACHE": str(ROOT / "cache" / "go-mod"), "GOTMPDIR": str(work / "tmp"), "HOME": str(work / "home")
        })
        environment.update({key: expand(value, variables) for key, value in run.get("environment", {}).items()})
        for key in ("GOCACHE", "GOMODCACHE", "GOTMPDIR", "HOME"):
            Path(environment[key]).mkdir(parents=True, exist_ok=True)

        started = time.monotonic()
        build = [expand(item, variables) for item in run["build"]]
        build_result = base.run_command("build", build, work_source, environment, config["timeouts_seconds"]["build"], logs, log_prefix=f"[{run['id']}] ")
        phases["build"] = phase(build_result["status"], started, exit_code=build_result["exit_code"])
        if build_result["status"] != "passed":
            raise RuntimeError("build falhou")

        command = [expand(item, variables) for item in run["start"]]
        started = time.monotonic()
        stdout_file = (logs / "application.stdout.log").open("wb")
        stderr_file = (logs / "application.stderr.log").open("wb")

        if run["interface"] in {"cli", "oneshot"}:
            completed = subprocess.run(command, cwd=runtime, env=environment, stdout=stdout_file, stderr=stderr_file, timeout=config["timeouts_seconds"]["startup"])
            phases["action"] = phase("passed" if completed.returncode == 0 else "failed", started, exit_code=completed.returncode)
            if completed.returncode != 0:
                raise RuntimeError(f"CLI encerrou com exit {completed.returncode}")
            before = run["expected"]["before"]
        else:
            process = subprocess.Popen(command, cwd=runtime, env=environment, stdout=stdout_file, stderr=stderr_file, start_new_session=True)
            phases["startup"] = phase("passed", started, pid=process.pid)
            started = time.monotonic()
            wait_ready(process, http_port, run["readiness"], config["timeouts_seconds"]["startup"])
            phases["readiness"] = phase("passed", started, type=run["readiness"]["type"])

            for index, preparation in enumerate(run.get("prepare_http", []), 1):
                result = http_request(http_port, preparation, config["timeouts_seconds"]["request"])
                if not result["expected"]:
                    raise RuntimeError(f"preparacao HTTP {index} retornou {result['status_code']}")
            if run["database"] == "sqlite":
                before = query_sqlite(sqlite_path, run["state"]["balances"])
            started = time.monotonic()
            response = http_request(http_port, run["request"], config["timeouts_seconds"]["request"])
            phases["action"] = phase("passed" if response["expected"] else "failed", started, status_code=response["status_code"])

        started = time.monotonic()
        if run["database"] == "postgres":
            after = query_postgres(database, run["state"]["balances"])
            ledger_values = query_postgres(database, run["state"]["ledger"])
        else:
            after = query_sqlite(sqlite_path, run["state"]["balances"])
            ledger_values = query_sqlite(sqlite_path, run["state"]["ledger"])
        ledger = ledger_values[0] if ledger_values else 0
        checks, passed = check_state(before, after, ledger, run["expected"])
        phases["reconciliation"] = phase("passed" if passed else "failed", started)
    except Exception as exc:
        error = str(exc)
        checks = {}
        passed = False
        print(f"  [erro] {error}", flush=True)
    finally:
        started = time.monotonic()
        shutdown = stop_process(process, config["timeouts_seconds"]["shutdown"])
        if stdout_file:
            stdout_file.close()
        if stderr_file:
            stderr_file.close()
        phases["shutdown"] = phase("passed", started, method=shutdown)
        if database:
            docker(["rm", "-f", database["name"]], check=False)

    result = {
        "schema_version": "1.0", "run_id": run["id"],
        "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run["id"] / "source-manifest.json")["root_hash"],
        "status": "passed" if passed else "failed", "error": error,
        "functional": {
            "interface": run["interface"], "response": response,
            "state": {"source_before_cents": before[0] if len(before) == 2 else None, "destination_before_cents": before[1] if len(before) == 2 else None,
                      "source_after_cents": after[0] if len(after) == 2 else None, "destination_after_cents": after[1] if len(after) == 2 else None,
                      "total_before_cents": sum(before) if len(before) == 2 else None, "total_after_cents": sum(after) if len(after) == 2 else None,
                      "matching_ledger_rows": ledger},
            "checks": checks
        },
        "operational_effort": {"assistance_count": len(run["assistance"]), "assistance": run["assistance"], "fallback_count": len(fallbacks), "fallbacks": fallbacks},
        "phases": phases,
        "diagnostics": {"application_stdout": "logs/application.stdout.log", "application_stderr": "logs/application.stderr.log"}
    }
    base.write_json(artifact / "summary.json", result)
    print(f"  [resultado] {result['status']}", flush=True)
    return result


def render_report(config, execution_id, results, output):
    lines = [
        f"# Benchmark {config['campaign_id']}", "", f"Execucao: `{execution_id}`", "",
        "| Run | Status | HTTP/CLI | Debito | Credito | Conservacao | Ledger | Nao negativo | Assistencias |",
        "|---|---|---|---|---|---|---|---|---:|"
    ]
    for item in results:
        checks = item["functional"]["checks"]
        response = item["functional"].get("response")
        observed = str(response["status_code"]) if response else item["functional"]["interface"]
        mark = lambda key: "PASS" if checks.get(key) else "FAIL"
        lines.append(
            f'| {item["run_id"]} | {item["status"].upper()} | {observed} | {mark("source_debited_once")} | '
            f'{mark("destination_credited_once")} | {mark("total_conserved")} | {mark("ledger_recorded_once")} | '
            f'{mark("non_negative_balances")} | {item["operational_effort"]["assistance_count"]} |'
        )
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="Executa smoke tests funcionais externos e reconciliacao de saldos.")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--run")
    parser.add_argument("--keep-work", action="store_true")
    args = parser.parse_args()
    config = base.read_json(args.config)
    selected = [run for run in config["runs"] if not args.run or run["id"] == args.run]
    if not selected:
        raise ValueError(f"run inexistente: {args.run}")
    subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
    execution_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    execution_root = ROOT / "artifacts" / config["campaign_id"] / execution_id
    results = []
    try:
        for run in selected:
            results.append(evaluate(config, run, execution_root))
        base.write_json(execution_root / "summary.json", {"campaign": config["campaign_id"], "execution_id": execution_id, "runs": results})
        render_report(config, execution_id, results, execution_root / "report.md")
    finally:
        subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
        if not args.keep_work:
            shutil.rmtree(ROOT / "work" / execution_id, ignore_errors=True)
    print(f"\n[campanha] {execution_root.relative_to(ROOT)}", flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro: {error}", file=sys.stderr)
        sys.exit(1)
