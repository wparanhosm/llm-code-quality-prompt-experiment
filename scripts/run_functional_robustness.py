import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))

import argparse
import concurrent.futures
import copy
import datetime as dt
import json
import os
import shutil
import subprocess
import time

import run_benchmark as base
import run_functional_benchmark as functional


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONFIG = ROOT / "benchmark" / "campaigns" / "functional-robustness.json"
NEGATIVE_CASES = ("same_account", "zero", "negative", "missing_wallet", "insufficient_funds")


def reset_state(run, work_source, database, sqlite_path):
    if run["database"] == "postgres":
        functional.psql(database, functional.sql_content(run, work_source, "seed"))
        return
    if run["id"] == "grok46-cenario-1":
        with functional.sqlite3.connect(sqlite_path) as connection:
            connection.execute("DELETE FROM transfers")
            connection.execute("UPDATE wallets SET balance=CASE user_id WHEN 1 THEN 100000 WHEN 2 THEN 25000 ELSE balance END")
        return
    with functional.sqlite3.connect(sqlite_path) as connection:
        connection.execute("DELETE FROM transactions")
        connection.execute("DELETE FROM wallets WHERE id IN ('bench-a','bench-b')")
        connection.executemany(
            "INSERT INTO wallets(id,balance_cents) VALUES (?,?)",
            [("bench-a", 10000), ("bench-b", 2000)],
        )


def query_state(run, database, sqlite_path):
    query = functional.query_postgres if run["database"] == "postgres" else functional.query_sqlite
    target = database if run["database"] == "postgres" else sqlite_path
    balances = query(target, run["state"]["balances"])
    ledger_values = query(target, run["state"]["ledger"])
    return balances, ledger_values[0] if ledger_values else 0


def request_for_case(run, robust, case):
    request = copy.deepcopy(run["request"])
    body = request["body"]
    fields = robust["fields"]
    if case == "same_account":
        body[fields["destination"]] = body[fields["source"]]
    elif case in {"zero", "negative", "insufficient_funds"}:
        body[fields["amount"]] = robust["values"][case.split("_funds")[0]]
    elif case == "missing_wallet":
        body[fields["destination"]] = robust["values"]["missing"]
    request["expected_status"] = robust["statuses"][case]
    return request


def cli_command(run, robust, variables, case):
    source = 1
    destination = 2
    amount = 1250
    if case == "same_account":
        destination = source
    elif case == "missing_wallet":
        destination = robust["values"]["missing"]
    elif case in {"zero", "negative", "insufficient_funds"}:
        amount = robust["values"][case.split("_funds")[0]]
    return [
        str(variables["binary"]), f'-db={variables["sqlite_path"]}',
        f"-from={source}", f"-to={destination}", f"-amount={amount}",
    ]


def run_negative_cases(run, robust, work_source, runtime, variables, environment, database, sqlite_path, port, config, logs):
    results = []
    for case in NEGATIVE_CASES:
        print(f"  [caso] {case}", flush=True)
        reset_state(run, work_source, database, sqlite_path)
        if robust.get("cli"):
            command = cli_command(run, robust, variables, case)
            completed = subprocess.run(command, cwd=runtime, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=config["timeouts_seconds"]["request"])
            observed = completed.returncode
            response_ok = observed in robust["exit_codes"][case]
            response = {"exit_code": observed, "expected": response_ok, "stderr": completed.stderr.decode(errors="replace")[:2000]}
        else:
            response = functional.http_request(port, request_for_case(run, robust, case), config["timeouts_seconds"]["request"])
            response_ok = response["expected"]
        balances, ledger = query_state(run, database, sqlite_path)
        state_unchanged = balances == run["expected"]["before"] and ledger == 0
        results.append({
            "id": case,
            "status": "passed" if response_ok and state_unchanged else "failed",
            "response": response,
            "checks": {"expected_rejection": response_ok, "balances_unchanged": balances == run["expected"]["before"], "ledger_unchanged": ledger == 0},
            "observed": {"balances": balances, "ledger_rows": ledger},
        })
    return results


def run_concurrency(run, robust, work_source, runtime, variables, environment, database, sqlite_path, port, campaign, config):
    print("  [caso] concurrency", flush=True)
    reset_state(run, work_source, database, sqlite_path)
    requests = config["concurrency"]["requests"]
    amount = config["concurrency"]["amount_cents"]
    expected_successes = min(requests, run["expected"]["before"][0] // amount)

    def invoke(index):
        if robust.get("cli"):
            command = [str(variables["binary"]), f'-db={variables["sqlite_path"]}', "-from=1", "-to=2", f'-amount={robust["values"]["concurrent"]}']
            completed = subprocess.run(command, cwd=runtime, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=campaign["timeouts_seconds"]["request"])
            return {"success": completed.returncode == 0, "observed": completed.returncode}
        request = copy.deepcopy(run["request"])
        request["body"][robust["fields"]["amount"]] = robust["values"]["concurrent"]
        if "idempotency_key" in request["body"]:
            request["body"]["idempotency_key"] = f"aaaaaaaa-aaaa-aaaa-aaaa-{index:012d}"
        request["expected_status"] = run["request"]["expected_status"]
        response = functional.http_request(port, request, campaign["timeouts_seconds"]["request"])
        return {"success": response["expected"], "observed": response["status_code"]}

    with concurrent.futures.ThreadPoolExecutor(max_workers=requests) as executor:
        responses = list(executor.map(invoke, range(requests)))
    successes = sum(item["success"] for item in responses)
    query = functional.query_postgres if run["database"] == "postgres" else functional.query_sqlite
    target = database if run["database"] == "postgres" else sqlite_path
    balances = query(target, run["state"]["balances"])
    concurrent_ledger_sql = run["state"]["ledger"].replace("1250", str(amount))
    ledger_values = query(target, concurrent_ledger_sql)
    ledger = ledger_values[0] if ledger_values else 0
    expected_after = [run["expected"]["before"][0] - successes * amount, run["expected"]["before"][1] + successes * amount]
    checks = {
        "bounded_successes": successes == expected_successes,
        "balances_match_successes": balances == expected_after,
        "total_conserved": sum(balances) == sum(run["expected"]["before"]) if len(balances) == 2 else False,
        "ledger_matches_successes": ledger == successes,
        "non_negative_balances": len(balances) == 2 and all(value >= 0 for value in balances),
        "all_requests_accounted": len(responses) == requests,
    }
    return {
        "id": "concurrency", "status": "passed" if all(checks.values()) else "failed", "checks": checks,
        "observed": {"requests": requests, "successes": successes, "responses": [item["observed"] for item in responses], "balances": balances, "ledger_rows": ledger},
    }


def evaluate(campaign, robust_config, run, execution_root):
    run_id = run["id"]
    robust = robust_config["runs"][run_id]
    artifact = execution_root / run_id
    artifact.mkdir(parents=True, exist_ok=True)
    if robust.get("unsupported"):
        result = {
            "schema_version": "1.0", "run_id": run_id, "status": "unsupported",
            "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run_id / "source-manifest.json")["root_hash"],
            "reason": robust["reason"], "cases": [{"id": case, "status": "unsupported"} for case in robust["unsupported"]],
        }
        base.write_json(artifact / "summary.json", result)
        return result

    print(f"\n[robustez] {run_id}", flush=True)
    source = ROOT / run["source"]
    work = ROOT / "work" / execution_root.name / run_id
    work_source = work / "source"
    runtime = work / "runtime"
    binary = work / "bin" / "application"
    logs = artifact / "logs"
    for path in (runtime, binary.parent, logs):
        path.mkdir(parents=True, exist_ok=True)
    base.materialize(source, work_source)
    database = None
    process = None
    stdout_file = None
    stderr_file = None
    cases = []
    error = None
    try:
        port = run.get("fixed_http_port", functional.free_port())
        sqlite_path = runtime / Path(run.get("sqlite_file", "runtime.db")).name
        variables = {"binary": binary, "http_port": port, "sqlite_path": sqlite_path, "postgres_port": "", "postgres_database": "", "postgres_dsn": ""}
        if run["database"] == "postgres":
            database = functional.start_postgres(campaign, run_id, execution_root.name)
            variables.update({"postgres_port": database["port"], "postgres_database": database["database"], "postgres_dsn": database["dsn"]})
            migration_key = "migration" if "migration" in run else "migration_external"
            functional.psql(database, functional.sql_content(run, work_source, migration_key))
        environment = os.environ.copy()
        environment.update({"GOCACHE": str(ROOT / "cache" / "go-build" / run_id), "GOMODCACHE": str(ROOT / "cache" / "go-mod"), "GOTMPDIR": str(work / "tmp"), "HOME": str(work / "home")})
        environment.update({key: functional.expand(value, variables) for key, value in run.get("environment", {}).items()})
        for key in ("GOCACHE", "GOMODCACHE", "GOTMPDIR", "HOME"):
            Path(environment[key]).mkdir(parents=True, exist_ok=True)
        build = [functional.expand(item, variables) for item in run["build"]]
        build_result = base.run_command("build", build, work_source, environment, campaign["timeouts_seconds"]["build"], logs, log_prefix=f"[{run_id}] ")
        if build_result["status"] != "passed":
            raise RuntimeError("build falhou")

        if robust.get("cli"):
            setup = subprocess.run(
                [str(binary), f"-db={sqlite_path}", "-from=1", "-to=2", "-amount=1"],
                cwd=runtime, env=environment, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                timeout=campaign["timeouts_seconds"]["request"]
            )
            if setup.returncode != 0:
                raise RuntimeError(f"inicializacao SQLite da CLI falhou com exit {setup.returncode}")
            reset_state(run, work_source, database, sqlite_path)

        if run["interface"] == "http":
            command = [functional.expand(item, variables) for item in run["start"]]
            stdout_file = (logs / "application.stdout.log").open("wb")
            stderr_file = (logs / "application.stderr.log").open("wb")
            if run["database"] == "sqlite":
                # Start once so the embedded schema is created before external resets.
                process = subprocess.Popen(command, cwd=runtime, env=environment, stdout=stdout_file, stderr=stderr_file, start_new_session=True)
                functional.wait_ready(process, port, run["readiness"], campaign["timeouts_seconds"]["startup"])
            else:
                functional.psql(database, functional.sql_content(run, work_source, "seed"))
                process = subprocess.Popen(command, cwd=runtime, env=environment, stdout=stdout_file, stderr=stderr_file, start_new_session=True)
                functional.wait_ready(process, port, run["readiness"], campaign["timeouts_seconds"]["startup"])

        cases.extend(run_negative_cases(run, robust, work_source, runtime, variables, environment, database, sqlite_path, port, campaign, logs))
        cases.append(run_concurrency(run, robust, work_source, runtime, variables, environment, database, sqlite_path, port, campaign, robust_config))
    except Exception as exc:
        error = str(exc)
        print(f"  [erro] {error}", flush=True)
    finally:
        functional.stop_process(process, campaign["timeouts_seconds"]["shutdown"])
        if stdout_file:
            stdout_file.close()
        if stderr_file:
            stderr_file.close()
        if database:
            functional.docker(["rm", "-f", database["name"]], check=False)
    status = "passed" if len(cases) == 6 and all(item["status"] == "passed" for item in cases) else "failed"
    result = {
        "schema_version": "1.0", "run_id": run_id, "status": status, "error": error,
        "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run_id / "source-manifest.json")["root_hash"],
        "cases": cases,
    }
    base.write_json(artifact / "summary.json", result)
    return result


def report(config, execution_id, results, output):
    case_ids = [*NEGATIVE_CASES, "concurrency"]
    lines = [f"# Benchmark {config['campaign_id']}", "", f"Execucao: `{execution_id}`", "", "| Run | " + " | ".join(case_ids) + " | Geral |", "|---|" + "|".join(["---"] * (len(case_ids) + 1)) + "|"]
    for result in results:
        statuses = {item["id"]: item["status"] for item in result["cases"]}
        lines.append("| " + " | ".join([result["run_id"], *[statuses.get(case, "not_run").upper() for case in case_ids], result["status"].upper()]) + " |")
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="Executa casos negativos e concorrencia sobre os adapters funcionais.")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--run")
    parser.add_argument("--keep-work", action="store_true")
    args = parser.parse_args()
    robust = base.read_json(args.config)
    campaign = base.read_json(ROOT / robust["base_campaign"])
    selected = [run for run in campaign["runs"] if not args.run or run["id"] == args.run]
    if not selected:
        raise ValueError(f"run inexistente: {args.run}")
    subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
    execution_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    execution_root = ROOT / "artifacts" / robust["campaign_id"] / execution_id
    results = []
    try:
        for run in selected:
            results.append(evaluate(campaign, robust, run, execution_root))
        base.write_json(execution_root / "summary.json", {"campaign": robust["campaign_id"], "execution_id": execution_id, "runs": results})
        report(robust, execution_id, results, execution_root / "report.md")
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
