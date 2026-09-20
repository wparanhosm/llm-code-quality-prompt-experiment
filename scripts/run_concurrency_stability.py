import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))

import argparse
import concurrent.futures
import copy
import datetime as dt
import json
import math
import os
import shutil
import subprocess
import threading
import time
import urllib.error
from collections import Counter

import run_benchmark as base
import run_functional_benchmark as functional
import run_functional_robustness as robustness


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONFIG = ROOT / "benchmark" / "campaigns" / "concurrency-stability.json"


def percentile(values, percent):
    if not values:
        return None
    ordered = sorted(values)
    rank = max(0, math.ceil(percent / 100 * len(ordered)) - 1)
    return round(ordered[rank], 3)


def query(run, database, sqlite_path, sql):
    function = functional.query_postgres if run["database"] == "postgres" else functional.query_sqlite
    target = database if run["database"] == "postgres" else sqlite_path
    values = function(target, sql)
    return values


def unique_idempotency(body, sequence):
    if "idempotency_key" in body:
        body["idempotency_key"] = f"bbbbbbbb-bbbb-bbbb-bbbb-{sequence:012d}"


def transport_amount(robust, amount_cents):
    value = amount_cents / robust.get("amount_scale", 1)
    if isinstance(robust["values"]["concurrent"], str):
        return f"{value:.2f}"
    return int(value) if value.is_integer() else value


def invoke_group(run, robust, port, workers, amount_value, inverse, timeout, sequence_base):
    barrier = threading.Barrier(workers)

    def invoke(index):
        request = copy.deepcopy(run["request"])
        fields = robust["fields"]
        request["body"][fields["amount"]] = amount_value
        if inverse and index % 2:
            request["body"][fields["source"]], request["body"][fields["destination"]] = (
                request["body"][fields["destination"]], request["body"][fields["source"]]
            )
        unique_idempotency(request["body"], sequence_base + index)
        started = time.monotonic()
        try:
            barrier.wait(timeout=5)
            response = functional.http_request(port, request, timeout)
            return {
                "kind": "success" if response["expected"] else "rejected",
                "status_code": response["status_code"],
                "latency_ms": round((time.monotonic() - started) * 1000, 3),
            }
        except Exception as error:
            return {
                "kind": "timeout" if isinstance(error, (TimeoutError, urllib.error.URLError)) else "error",
                "error": str(error),
                "latency_ms": round((time.monotonic() - started) * 1000, 3),
            }

    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as executor:
        return list(executor.map(invoke, range(workers)))


def summarize_responses(responses):
    latencies = [item["latency_ms"] for item in responses]
    statuses = Counter(str(item.get("status_code", item["kind"])) for item in responses)
    return {
        "requests": len(responses),
        "successes": sum(item["kind"] == "success" for item in responses),
        "rejections": sum(item["kind"] == "rejected" for item in responses),
        "timeouts": sum(item["kind"] == "timeout" for item in responses),
        "errors": sum(item["kind"] == "error" for item in responses),
        "server_errors": sum(item.get("status_code", 0) >= 500 for item in responses),
        "status_counts": dict(sorted(statuses.items())),
        "latency_ms": {
            "min": round(min(latencies), 3), "mean": round(sum(latencies) / len(latencies), 3),
            "p50": percentile(latencies, 50), "p95": percentile(latencies, 95),
            "p99": percentile(latencies, 99), "max": round(max(latencies), 3)
        }
    }


def evaluate(config, campaign, robustness_config, run, execution_root):
    run_id = run["id"]
    print(f"\n[estabilidade] {run_id}", flush=True)
    robust = robustness_config["runs"][run_id]
    artifact = execution_root / run_id
    logs = artifact / "logs"
    work = ROOT / "work" / execution_root.name / run_id
    work_source = work / "source"
    runtime = work / "runtime"
    binary = work / "bin" / "application"
    for path in (logs, runtime, binary.parent):
        path.mkdir(parents=True, exist_ok=True)
    base.materialize(ROOT / run["source"], work_source)
    database = None
    process = None
    stdout_file = None
    stderr_file = None
    repetitions = []
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
            functional.psql(database, functional.sql_content(run, work_source, "seed"))

        environment = os.environ.copy()
        environment.update({"GOCACHE": str(ROOT / "cache" / "go-build" / run_id), "GOMODCACHE": str(ROOT / "cache" / "go-mod"), "GOTMPDIR": str(work / "tmp"), "HOME": str(work / "home")})
        environment.update({key: functional.expand(value, variables) for key, value in run.get("environment", {}).items()})
        for key in ("GOCACHE", "GOMODCACHE", "GOTMPDIR", "HOME"):
            Path(environment[key]).mkdir(parents=True, exist_ok=True)
        build = [functional.expand(item, variables) for item in run["build"]]
        build_result = base.run_command("build", build, work_source, environment, campaign["timeouts_seconds"]["build"], logs, log_prefix=f"[{run_id}] ")
        if build_result["status"] != "passed":
            raise RuntimeError("build falhou")

        command = [functional.expand(item, variables) for item in run["start"]]
        stdout_file = (logs / "application.stdout.log").open("wb")
        stderr_file = (logs / "application.stderr.log").open("wb")
        process = subprocess.Popen(command, cwd=runtime, env=environment, stdout=stdout_file, stderr=stderr_file, start_new_session=True)
        functional.wait_ready(process, port, run["readiness"], campaign["timeouts_seconds"]["startup"])

        for repetition in range(config["repetitions"]):
            if repetition % 5 == 0:
                print(f"  [rodada {repetition + 1}/{config['repetitions']}]", flush=True)
            round_result = {"repetition": repetition + 1}
            for inverse in (False, True):
                test_name = "inverse" if inverse else "contention"
                robustness.reset_state(run, work_source, database, sqlite_path)
                amount_cents = config["inverse_amount_cents"] if inverse else config["contention_amount_cents"]
                amount_value = transport_amount(robust, amount_cents)
                responses = invoke_group(
                    run, robust, port, config["workers"], amount_value, inverse,
                    campaign["timeouts_seconds"]["request"], repetition * 100 + (50 if inverse else 0)
                )
                observed = summarize_responses(responses)
                balances = query(run, database, sqlite_path, run["state"]["balances"])
                ledger_values = query(run, database, sqlite_path, config["ledger_total_queries"][run_id])
                ledger = ledger_values[0] if ledger_values else 0
                expected_successes = config["workers"] if inverse else min(config["workers"], run["expected"]["before"][0] // amount_cents)
                expected_balances = run["expected"]["before"] if inverse else [
                    run["expected"]["before"][0] - expected_successes * amount_cents,
                    run["expected"]["before"][1] + expected_successes * amount_cents,
                ]
                checks = {
                    "expected_successes": observed["successes"] == expected_successes,
                    "no_server_errors": observed["server_errors"] == 0,
                    "no_timeouts_or_client_errors": observed["timeouts"] == 0 and observed["errors"] == 0,
                    "balances_match": balances == expected_balances,
                    "total_conserved": len(balances) == 2 and sum(balances) == sum(run["expected"]["before"]),
                    "ledger_matches": ledger == observed["successes"],
                    "non_negative": len(balances) == 2 and all(value >= 0 for value in balances),
                }
                round_result[test_name] = {
                    "status": "passed" if all(checks.values()) else "failed", "checks": checks,
                    "observed": {**observed, "balances": balances, "ledger_rows": ledger}
                }
            repetitions.append(round_result)
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

    tests = [item[name] for item in repetitions for name in ("contention", "inverse") if name in item]
    failed = sum(item["status"] == "failed" for item in tests)
    all_latencies = [item["observed"]["latency_ms"]["mean"] for item in tests]
    result = {
        "schema_version": "1.0", "run_id": run_id,
        "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run_id / "source-manifest.json")["root_hash"],
        "status": "passed" if len(repetitions) == config["repetitions"] and failed == 0 else "failed",
        "error": error, "repetitions_completed": len(repetitions), "tests_failed": failed,
        "aggregate": {
            "server_errors": sum(item["observed"]["server_errors"] for item in tests),
            "timeouts": sum(item["observed"]["timeouts"] for item in tests),
            "request_errors": sum(item["observed"]["errors"] for item in tests),
            "mean_round_latency_ms": round(sum(all_latencies) / len(all_latencies), 3) if all_latencies else None,
            "p95_round_latency_ms": percentile(all_latencies, 95),
        },
        "repetitions": repetitions,
    }
    base.write_json(artifact / "summary.json", result)
    return result


def render_report(config, execution_id, results, unsupported, output):
    lines = [
        f"# Benchmark {config['campaign_id']}", "", f"Execucao: `{execution_id}`", "",
        "| Run | Rodadas | Falhas | 5xx | Timeouts | Erros cliente | Latencia media/rodada | p95 | Elegivel K6 |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---|"
    ]
    for item in results:
        aggregate = item["aggregate"]
        lines.append(
            f'| {item["run_id"]} | {item["repetitions_completed"]}/{config["repetitions"]} | {item["tests_failed"]} | '
            f'{aggregate["server_errors"]} | {aggregate["timeouts"]} | {aggregate["request_errors"]} | '
            f'{aggregate["mean_round_latency_ms"]} ms | {aggregate["p95_round_latency_ms"]} ms | '
            f'{"SIM" if item["status"] == "passed" else "NAO"} |'
        )
    for run_id in unsupported:
        lines.append(f"| {run_id} | 0/{config['repetitions']} | - | - | - | - | - | - | NAO (sem HTTP) |")
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="Executa estabilidade concorrente repetida antes da carga K6.")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--run")
    parser.add_argument("--keep-work", action="store_true")
    args = parser.parse_args()
    config = base.read_json(args.config)
    campaign = base.read_json(ROOT / config["base_campaign"])
    robust = base.read_json(ROOT / config["robustness_campaign"])
    by_id = {run["id"]: run for run in campaign["runs"]}
    eligible = [run_id for run_id in config["eligible_runs"] if not args.run or run_id == args.run]
    if args.run and args.run not in by_id:
        raise ValueError(f"run inexistente: {args.run}")
    if args.run and args.run not in config["eligible_runs"]:
        raise ValueError(f"run nao possui backend HTTP elegivel: {args.run}")

    subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
    execution_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    execution_root = ROOT / "artifacts" / config["campaign_id"] / execution_id
    results = []
    try:
        for run_id in eligible:
            results.append(evaluate(config, campaign, robust, by_id[run_id], execution_root))
        unsupported = [] if args.run else sorted(set(by_id) - set(config["eligible_runs"]))
        payload = {"campaign": config["campaign_id"], "execution_id": execution_id, "runs": results, "unsupported_http": unsupported}
        base.write_json(execution_root / "summary.json", payload)
        render_report(config, execution_id, results, unsupported, execution_root / "report.md")
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
