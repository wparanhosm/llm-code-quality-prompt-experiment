import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).resolve().parent))

import concurrent.futures
import argparse
import datetime as dt
import json
import os
import re
import shutil
import socket
import sqlite3
import subprocess
import time

import run_benchmark as base
import run_functional_benchmark as functional


ROOT = Path(__file__).resolve().parents[1]
DEFAULT_CONFIG = ROOT / "benchmark" / "campaigns" / "k6-million-load.json"


def scalar_postgres(database, sql):
    values = functional.query_postgres(database, sql)
    return values[0] if values else None


def scalar_sqlite(path, sql):
    with sqlite3.connect(path) as connection:
        row = connection.execute(sql).fetchone()
        return int(row[0]) if row else None


def seed_postgres(database, adapter, pairs, requests_per_pair):
    if adapter in {"chatgpt-c1", "chatgpt-c2"}:
        sql = f"""
        TRUNCATE TABLE transfers, wallets CASCADE;
        INSERT INTO wallets(id, balance_cents)
        SELECT 'k6-src-' || lpad(g::text, 3, '0'), {requests_per_pair} FROM generate_series(0, {pairs - 1}) g
        UNION ALL
        SELECT 'k6-dst-' || lpad(g::text, 3, '0'), 0 FROM generate_series(0, {pairs - 1}) g;
        """
    elif adapter == "gemini-c2":
        sql = f"""
        TRUNCATE TABLE transfers, accounts RESTART IDENTITY CASCADE;
        INSERT INTO accounts(id, holder, balance_cents, status)
        SELECT 1000001 + g, 'K6 source ' || g, {requests_per_pair}, 'ACTIVE' FROM generate_series(0, {pairs - 1}) g;
        INSERT INTO accounts(id, holder, balance_cents, status)
        SELECT 2000001 + g, 'K6 destination ' || g, 0, 'ACTIVE' FROM generate_series(0, {pairs - 1}) g;
        """
    elif adapter == "grok-c2":
        sql = f"""
        TRUNCATE TABLE transfers, wallets CASCADE;
        INSERT INTO wallets(id, balance_cents)
        SELECT 'k6-src-' || lpad(g::text, 3, '0'), {requests_per_pair} FROM generate_series(0, {pairs - 1}) g;
        INSERT INTO wallets(id, balance_cents)
        SELECT 'k6-dst-' || lpad(g::text, 3, '0'), 0 FROM generate_series(0, {pairs - 1}) g;
        """
    elif adapter == "kimi-c2":
        sql = f"""
        TRUNCATE TABLE transactions, wallets CASCADE;
        INSERT INTO wallets(id, balance_cents)
        SELECT format('10000000-0000-4000-8000-%s', lpad(g::text, 12, '0'))::uuid, {requests_per_pair}
        FROM generate_series(0, {pairs - 1}) g;
        INSERT INTO wallets(id, balance_cents)
        SELECT format('20000000-0000-4000-8000-%s', lpad(g::text, 12, '0'))::uuid, 0
        FROM generate_series(0, {pairs - 1}) g;
        """
    elif adapter == "sonnet-c2":
        sql = f"""
        TRUNCATE TABLE transactions, wallets CASCADE;
        INSERT INTO wallets(id, owner_id, balance, status)
        SELECT 'k6-src-' || lpad(g::text, 3, '0'), 'k6-owner-src-' || g, {requests_per_pair}, 'ACTIVE'
        FROM generate_series(0, {pairs - 1}) g;
        INSERT INTO wallets(id, owner_id, balance, status)
        SELECT 'k6-dst-' || lpad(g::text, 3, '0'), 'k6-owner-dst-' || g, 0, 'ACTIVE'
        FROM generate_series(0, {pairs - 1}) g;
        """
    else:
        raise ValueError(f"adapter PostgreSQL desconhecido: {adapter}")
    functional.psql(database, sql)


def seed_sqlite(path, pairs, requests_per_pair):
    with sqlite3.connect(path) as connection:
        connection.execute("PRAGMA foreign_keys=ON")
        connection.execute("DELETE FROM transactions")
        connection.execute("DELETE FROM wallets WHERE id LIKE 'k6-src-%' OR id LIKE 'k6-dst-%'")
        connection.executemany(
            "INSERT INTO wallets(id, balance_cents) VALUES (?, ?)",
            [(f"k6-src-{pair:03d}", requests_per_pair) for pair in range(pairs)]
            + [(f"k6-dst-{pair:03d}", 0) for pair in range(pairs)],
        )


def reconciliation(run, adapter, database, sqlite_path, pairs, requests_per_pair, iterations):
    query = (lambda sql: scalar_postgres(database, sql)) if database else (lambda sql: scalar_sqlite(sqlite_path, sql))
    if adapter == "gemini-c2":
        wallet_table, balance = "accounts", "balance_cents"
        source_end, destination_end = 1000000 + pairs, 2000000 + pairs
        scope = f"id BETWEEN 1000001 AND {source_end} OR id BETWEEN 2000001 AND {destination_end}"
        source_scope = f"id BETWEEN 1000001 AND {source_end}"
        destination_scope = f"id BETWEEN 2000001 AND {destination_end}"
        ledger_table, amount = "transfers", "amount_cents"
        ledger_scope = "from_account_id BETWEEN 1000001 AND 1000100 AND to_account_id BETWEEN 2000001 AND 2000100"
    elif adapter == "kimi-c2":
        wallet_table, balance = "wallets", "balance_cents"
        source_scope = "id::text LIKE '10000000-0000-4000-8000-%'"
        destination_scope = "id::text LIKE '20000000-0000-4000-8000-%'"
        scope = f"{source_scope} OR {destination_scope}"
        ledger_table, amount = "transactions", "amount_cents"
        ledger_scope = "idempotency_key LIKE 'kimik3-cenario-2-%'"
    elif adapter == "sonnet-c2":
        wallet_table, balance = "wallets", "balance"
        source_scope, destination_scope = "id LIKE 'k6-src-%'", "id LIKE 'k6-dst-%'"
        scope = f"{source_scope} OR {destination_scope}"
        ledger_table, amount = "transactions", "amount"
        ledger_scope = "from_wallet_id LIKE 'k6-src-%' AND to_wallet_id LIKE 'k6-dst-%'"
    else:
        wallet_table, balance = "wallets", "balance_cents"
        source_scope, destination_scope = "id LIKE 'k6-src-%'", "id LIKE 'k6-dst-%'"
        scope = f"{source_scope} OR {destination_scope}"
        ledger_table = "transactions" if adapter == "kimi-c1" else "transfers"
        amount = "amount_cents"
        from_column = "from_wallet_id" if adapter in {"grok-c2", "kimi-c1"} else "source_wallet_id"
        to_column = "to_wallet_id" if adapter in {"grok-c2", "kimi-c1"} else "destination_wallet_id"
        ledger_scope = f"{from_column} LIKE 'k6-src-%' AND {to_column} LIKE 'k6-dst-%'"

    observed = {
        "wallet_count": query(f"SELECT COUNT(*) FROM {wallet_table} WHERE {scope}"),
        "source_total": query(f"SELECT COALESCE(SUM({balance}), 0) FROM {wallet_table} WHERE {source_scope}"),
        "destination_total": query(f"SELECT COALESCE(SUM({balance}), 0) FROM {wallet_table} WHERE {destination_scope}"),
        "total_balance": query(f"SELECT COALESCE(SUM({balance}), 0) FROM {wallet_table} WHERE {scope}"),
        "negative_wallets": query(f"SELECT COUNT(*) FROM {wallet_table} WHERE ({scope}) AND {balance} < 0"),
        "ledger_rows": query(f"SELECT COUNT(*) FROM {ledger_table} WHERE {ledger_scope}"),
        "ledger_amount": query(f"SELECT COALESCE(SUM({amount}), 0) FROM {ledger_table} WHERE {ledger_scope}"),
    }
    expected = {
        "wallet_count": pairs * 2,
        "source_total": 0,
        "destination_total": iterations,
        "total_balance": iterations,
        "negative_wallets": 0,
        "ledger_rows": iterations,
        "ledger_amount": iterations,
    }
    checks = {name: observed[name] == value for name, value in expected.items()}
    return {"observed": observed, "expected": expected, "checks": checks, "passed": all(checks.values())}


def k6_metric(summary, name, field):
    metric = summary.get("metrics", {}).get(name, {})
    value = metric.get("values", {}).get(field, metric.get(field))
    if value is None and field == "rate":
        value = metric.get("value")
    return value


def format_elapsed(seconds):
    elapsed = int(seconds)
    hours, remainder = divmod(elapsed, 3600)
    minutes, seconds = divmod(remainder, 60)
    if hours:
        return f"{hours}h {minutes:02d}m {seconds:02d}s"
    if minutes:
        return f"{minutes}m {seconds:02d}s"
    return f"{seconds}s"


def format_number(value, decimals=2, suffix=""):
    if value is None:
        return "-"
    return f"{value:.{decimals}f}{suffix}"


def completed_iterations(log_path):
    if not log_path.is_file():
        return 0
    text = log_path.read_text(encoding="utf-8", errors="replace")
    matches = re.findall(r"(\d+) complete", text)
    return max((int(value) for value in matches), default=0)


def start_sleep_inhibitor():
    powershell = shutil.which("powershell.exe")
    if not powershell:
        print("[energia] powershell.exe indisponivel; bloqueio de descanso nao ativado", flush=True)
        return None
    script = """
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class PowerState {
    [DllImport("kernel32.dll")]
    public static extern uint SetThreadExecutionState(uint flags);
}
'@
$continuous = 0x80000000
$systemRequired = 0x00000001
[PowerState]::SetThreadExecutionState($continuous -bor $systemRequired) | Out-Null
try { while ($true) { Start-Sleep -Seconds 30 } }
finally { [PowerState]::SetThreadExecutionState($continuous) | Out-Null }
"""
    process = subprocess.Popen(
        [powershell, "-NoProfile", "-NonInteractive", "-Command", script],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    time.sleep(0.5)
    if process.poll() is not None:
        print("[energia] nao foi possivel bloquear o descanso do Windows", flush=True)
        return None
    print("[energia] descanso automatico do Windows bloqueado durante a campanha", flush=True)
    return process


def stop_sleep_inhibitor(process):
    if process is None or process.poll() is not None:
        return
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait(timeout=5)


def run_k6(config, spec, run_id, port, artifact, iterations):
    output = artifact / "k6"
    output.mkdir(parents=True, exist_ok=True)
    name = f"lqcpe-k6-{run_id}-{artifact.parents[0].name.lower()}"[:63]
    command = [
        "docker", "run", "--rm", "--name", name,
        "-e", f"BASE_URL=http://{config['host_address']}:{port}",
        "-e", f"RUN_ID={run_id}", "-e", f"ADAPTER={spec['adapter']}",
        "-e", f"EXPECTED_STATUS={spec['expected_status']}", "-e", f"TOTAL_ITERATIONS={iterations}",
        "-e", f"PAIR_COUNT={config['pairs']}", "-e", f"VUS={config['vus']}",
        "-e", f"MAX_DURATION={config['max_duration']}", "-e", f"REQUEST_TIMEOUT={config['request_timeout']}",
        "-v", f"{(ROOT / config['k6_script']).resolve()}:/scripts/million-transfers.js:ro",
        "-v", f"{output.resolve()}:/output",
        config["k6_image"], "run", "--summary-export=/output/summary.json", "/scripts/million-transfers.js",
    ]
    started = time.monotonic()
    interval = config.get("progress_interval_seconds", 30)
    stdout_path = output / "stdout.log"
    print(f"  [{run_id}] [k6] iniciado; progresso a cada {interval}s", flush=True)
    with stdout_path.open("wb") as stdout, (output / "stderr.log").open("wb") as stderr:
        process = subprocess.Popen(command, stdout=stdout, stderr=stderr)
        while True:
            try:
                exit_code = process.wait(timeout=interval)
                break
            except subprocess.TimeoutExpired:
                elapsed = time.monotonic() - started
                now = dt.datetime.now().astimezone().strftime("%H:%M:%S")
                stdout.flush()
                completed = completed_iterations(stdout_path)
                percent = completed * 100 / iterations
                rate = completed / elapsed if elapsed else 0
                remaining = (iterations - completed) / rate if rate else None
                eta = format_elapsed(remaining) if remaining is not None else "calculando"
                print(
                    f"  [{run_id}] [k6] {completed:,}/{iterations:,} ({percent:.1f}%) | "
                    f"{rate:,.1f} req/s | decorrido {format_elapsed(elapsed)} | ETA {eta} | {now}",
                    flush=True,
                )
        elapsed = time.monotonic() - started
        print(f"  [{run_id}] [k6] finalizado em {format_elapsed(elapsed)} (exit={exit_code})", flush=True)
    summary_path = output / "summary.json"
    summary = json.loads(summary_path.read_text(encoding="utf-8")) if summary_path.is_file() else {}
    return {
        "exit_code": exit_code,
        "duration_seconds": round(elapsed, 3),
        "metrics": {
            "iterations": k6_metric(summary, "iterations", "count"),
            "http_reqs": k6_metric(summary, "http_reqs", "count"),
            "successful_transfers": k6_metric(summary, "successful_transfers", "count"),
            "unexpected_statuses": k6_metric(summary, "unexpected_statuses", "count"),
            "status_counts": {
                status: k6_metric(summary, f"responses_{status}", "count") or 0
                for status in ("200", "201", "400", "404", "409", "422", "500", "other")
            },
            "http_req_failed_rate": k6_metric(summary, "http_req_failed", "rate"),
            "requests_per_second": k6_metric(summary, "http_reqs", "rate"),
            "latency_avg_ms": k6_metric(summary, "http_req_duration", "avg"),
            "latency_med_ms": k6_metric(summary, "http_req_duration", "med"),
            "latency_p90_ms": k6_metric(summary, "http_req_duration", "p(90)"),
            "latency_p95_ms": k6_metric(summary, "http_req_duration", "p(95)"),
            "latency_p99_ms": k6_metric(summary, "http_req_duration", "p(99)"),
        },
    }


def prepare(config, campaign, run, spec, execution_root, iterations):
    """Fase 1 (paralela): cria diretorios, materializa fonte e compila o binario."""
    run_id = run["id"]
    log_prefix = f"[{run_id}] "
    artifact = execution_root / run_id
    work = ROOT / "work" / execution_root.name / run_id
    work_source = work / "source"
    runtime = work / "runtime"
    binary = work / "bin" / "application"
    port = run.get("fixed_http_port", functional.free_port())
    sqlite_path = runtime / Path(run.get("sqlite_file", "runtime.db")).name

    for path in (artifact, artifact / "logs", runtime, binary.parent):
        path.mkdir(parents=True, exist_ok=True)
    base.materialize(ROOT / run["source"], work_source)

    build_env = os.environ.copy()
    build_env.update({
        "GOCACHE": str(ROOT / "cache" / "go-build" / run_id),
        "GOMODCACHE": str(ROOT / "cache" / "go-mod"),
        "GOTMPDIR": str(work / "tmp"),
        "HOME": str(work / "home"),
    })
    for key in ("GOCACHE", "GOMODCACHE", "GOTMPDIR", "HOME"):
        Path(build_env[key]).mkdir(parents=True, exist_ok=True)

    # Postgres nao e necessario na compilacao; passa strings vazias para satisfazer expand()
    build_vars = {"binary": binary, "http_port": port, "sqlite_path": sqlite_path, "postgres_port": "", "postgres_database": "", "postgres_dsn": ""}
    build_cmd = [functional.expand(item, build_vars) for item in run["build"]]

    print(f"[build] {run_id}: compilando...", flush=True)
    build_result = base.run_command("build", build_cmd, work_source, build_env, campaign["timeouts_seconds"]["build"], artifact / "logs", log_prefix=log_prefix)
    print(f"  {log_prefix}[build] {build_result['status']}", flush=True)

    return {
        "run": run, "spec": spec, "port": port, "binary": binary,
        "work_source": work_source, "runtime": runtime, "sqlite_path": sqlite_path,
        "build_env": build_env, "build_result": build_result, "artifact": artifact,
    }


def run_load(config, campaign, prepared, execution_root, iterations):
    """Fase 2 (sequencial): sobe banco, inicia aplicacao, dispara k6 e reconcilia."""
    run = prepared["run"]
    spec = prepared["spec"]
    run_id = run["id"]
    log_prefix = f"[{run_id}] "
    artifact = prepared["artifact"]
    port = prepared["port"]
    binary = prepared["binary"]
    work_source = prepared["work_source"]
    runtime = prepared["runtime"]
    sqlite_path = prepared["sqlite_path"]
    build_env = prepared["build_env"]
    build_result = prepared["build_result"]

    print(f"\n[k6] {run_id}: {iterations} requisicoes", flush=True)
    database = process = None
    error = None
    k6_result = {}
    reconciliation_result = {"passed": False}
    requests_per_pair = iterations // config["pairs"]

    if build_result["status"] != "passed":
        error = "build falhou"
        print(f"  {log_prefix}[erro] {error}", flush=True)
    else:
        try:
            variables = {"binary": binary, "http_port": port, "sqlite_path": sqlite_path, "postgres_port": "", "postgres_database": "", "postgres_dsn": ""}
            if run["database"] == "postgres":
                database = functional.start_postgres(campaign, run_id, execution_root.name)
                variables.update({"postgres_port": database["port"], "postgres_database": database["database"], "postgres_dsn": database["dsn"]})
                migration_key = "migration" if "migration" in run else "migration_external"
                functional.psql(database, functional.sql_content(run, work_source, migration_key))
                seed_postgres(database, spec["adapter"], config["pairs"], requests_per_pair)

            # Monta ambiente completo agora que variaveis do postgres estao disponiveis
            environment = build_env.copy()
            environment.update({key: functional.expand(value, variables) for key, value in run.get("environment", {}).items()})
            environment.update(spec.get("environment", {}))
            if "HTTP_ADDR" in environment and not run.get("fixed_http_port"):
                environment["HTTP_ADDR"] = f"0.0.0.0:{port}"

            command = [functional.expand(item, variables) for item in run["start"]]

            def start_application():
                return subprocess.Popen(command, cwd=runtime, env=environment, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)

            process = start_application()
            functional.wait_ready(process, port, run["readiness"], campaign["timeouts_seconds"]["startup"])
            if run["database"] == "sqlite":
                functional.stop_process(process, campaign["timeouts_seconds"]["shutdown"])
                seed_sqlite(sqlite_path, config["pairs"], requests_per_pair)
                process = start_application()
                functional.wait_ready(process, port, run["readiness"], campaign["timeouts_seconds"]["startup"])

            k6_result = run_k6(config, spec, run_id, port, artifact, iterations)
            if process.poll() is not None:
                raise RuntimeError(f"aplicacao encerrou durante a carga com exit {process.returncode}")
            functional.stop_process(process, campaign["timeouts_seconds"]["shutdown"])
            process = None
            reconciliation_result = reconciliation(run, spec["adapter"], database, sqlite_path, config["pairs"], requests_per_pair, iterations)
        except Exception as exc:
            error = str(exc)
            print(f"  {log_prefix}[erro] {error}", flush=True)
        finally:
            functional.stop_process(process, campaign["timeouts_seconds"]["shutdown"])
            if database:
                functional.docker(["rm", "-f", database["name"]], check=False)

    metrics = k6_result.get("metrics", {})
    k6_passed = (
        k6_result.get("exit_code") == 0
        and metrics.get("iterations") == iterations
        and metrics.get("http_reqs") == iterations
        and metrics.get("successful_transfers") == iterations
        and (metrics.get("unexpected_statuses") or 0) == 0
        and (metrics.get("http_req_failed_rate") or 0) == 0
    )
    result = {
        "schema_version": "1.0", "run_id": run_id,
        "source_root_hash": base.read_json(ROOT / "provenance" / "snapshots" / run_id / "source-manifest.json")["root_hash"],
        "status": "passed" if k6_passed and reconciliation_result.get("passed") and not error else "failed",
        "error": error, "planned_iterations": iterations, "pairs": config["pairs"], "vus": config["vus"],
        "k6": k6_result, "reconciliation": reconciliation_result,
        "operational_assistance": ["snapshot_materialized", "database_provisioned", "seed_applied_external", "environment_injected", "application_logs_discarded_during_load"],
    }
    base.write_json(artifact / "summary.json", result)
    print(f"  {log_prefix}[resultado] {result['status']}", flush=True)
    return result


def render_report(config, execution_id, results, output):
    lines = [
        f"# Benchmark {config['campaign_id']}", "", f"Execucao: `{execution_id}`", "",
        "| Run | Status | Requisicoes | Sucessos HTTP | Falhas HTTP | Duracao | RPS | Media | p95 | p99 | Ledger | Reconciliacao exata | Conservacao |",
        "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|",
    ]
    for item in results:
        metrics = item.get("k6", {}).get("metrics", {})
        reconciliation_result = item.get("reconciliation", {})
        checks = reconciliation_result.get("checks", {})
        duration = item.get("k6", {}).get("duration_seconds")
        duration_text = format_elapsed(duration) if duration is not None else "-"
        lines.append(
            f"| {item['run_id']} | {item['status'].upper()} | {metrics.get('http_reqs', '-')} | "
            f"{metrics.get('successful_transfers', '-')} | {metrics.get('unexpected_statuses', '-')} | {duration_text} | "
            f"{format_number(metrics.get('requests_per_second'))} | {format_number(metrics.get('latency_avg_ms'), suffix=' ms')} | "
            f"{format_number(metrics.get('latency_p95_ms'), suffix=' ms')} | {format_number(metrics.get('latency_p99_ms'), suffix=' ms')} | "
            f"{reconciliation_result.get('observed', {}).get('ledger_rows', '-')} | "
            f"{'PASS' if reconciliation_result.get('passed') else 'FAIL'} | "
            f"{'PASS' if checks.get('total_balance') else 'FAIL'} |"
        )
    passed = sum(item.get("status") == "passed" for item in results)
    failed = len(results) - passed
    lines.extend(["", f"Resultado: {passed} backends aprovados e {failed} {'reprovado' if failed == 1 else 'reprovados'}."])
    for item in results:
        if item.get("status") != "passed":
            metrics = item.get("k6", {}).get("metrics", {})
            observed = item.get("reconciliation", {}).get("observed", {})
            lines.append(
                f"- `{item['run_id']}`: {metrics.get('unexpected_statuses', 0)} respostas inesperadas; "
                f"{metrics.get('successful_transfers', 0)} sucessos HTTP e {observed.get('ledger_rows', 0)} registros no ledger."
            )
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    parser = argparse.ArgumentParser(description="Executa carga K6 isolada e reconcilia um milhao de transferencias por backend.")
    parser.add_argument("--config", type=Path, default=DEFAULT_CONFIG)
    parser.add_argument("--run")
    parser.add_argument("--iterations", type=int, help="sobrescreve a contagem apenas para calibracao")
    parser.add_argument("--keep-work", action="store_true")
    parser.add_argument("--report-only", help="regenera o relatorio de um execution_id sem executar carga")
    args = parser.parse_args()
    config = base.read_json(args.config)
    if args.report_only:
        execution_root = ROOT / "artifacts" / config["campaign_id"] / args.report_only
        payload = base.read_json(execution_root / "summary.json")
        render_report(config, args.report_only, payload["runs"], execution_root / "report.md")
        print(f"[relatorio] {execution_root.relative_to(ROOT) / 'report.md'}", flush=True)
        return
    campaign = base.read_json(ROOT / config["base_campaign"])
    by_id = {run["id"]: run for run in campaign["runs"]}
    selected = [run_id for run_id in config["runs"] if not args.run or args.run == run_id]
    if args.run and not selected:
        raise ValueError(f"run nao elegivel ou inexistente: {args.run}")
    iterations = args.iterations or config["iterations"]
    if iterations <= 0 or iterations % config["pairs"]:
        raise ValueError(f"iterations deve ser positivo e divisivel por {config['pairs']}")

    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as probe:
        probe.connect(("8.8.8.8", 80))
        host_address = probe.getsockname()[0]
    config["host_address"] = host_address

    subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
    subprocess.run(["docker", "image", "inspect", config["k6_image"]], check=True, stdout=subprocess.DEVNULL)
    execution_id = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    execution_root = ROOT / "artifacts" / config["campaign_id"] / execution_id
    results = []
    sleep_inhibitor = start_sleep_inhibitor()
    try:
        # Fase 1: builds em paralelo
        print(f"[builds] compilando {len(selected)} binarios em paralelo...", flush=True)
        prepared = {}
        with concurrent.futures.ThreadPoolExecutor() as executor:
            future_to_id = {
                executor.submit(prepare, config, campaign, by_id[run_id], config["runs"][run_id], execution_root, iterations): run_id
                for run_id in selected
            }
            for future in concurrent.futures.as_completed(future_to_id):
                run_id = future_to_id[future]
                try:
                    prepared[run_id] = future.result()
                except Exception as exc:
                    print(f"[build] {run_id} excecao inesperada: {exc}", flush=True)

        # Fase 2: cargas k6 sequenciais (isola medicoes de latencia)
        print(f"\n[k6] iniciando cargas sequenciais ({len(selected)} runs)...", flush=True)
        for run_id in selected:
            if run_id in prepared:
                results.append(run_load(config, campaign, prepared[run_id], execution_root, iterations))
            else:
                results.append({"run_id": run_id, "status": "failed", "error": "prepare/build falhou", "k6": {}, "reconciliation": {}})

        base.write_json(execution_root / "summary.json", {"campaign": config["campaign_id"], "execution_id": execution_id, "iterations_per_run": iterations, "runs": results})
        render_report(config, execution_id, results, execution_root / "report.md")
    finally:
        stop_sleep_inhibitor(sleep_inhibitor)
        subprocess.run([sys.executable, str(ROOT / "scripts" / "classify_snapshots.py"), "--verify"], check=True)
        if not args.keep_work:
            shutil.rmtree(ROOT / "work" / execution_id, ignore_errors=True)
    print(f"\n[campanha] {execution_root.relative_to(ROOT)}", flush=True)
    if any(item["status"] != "passed" for item in results):
        raise RuntimeError("um ou mais backends falharam na carga ou reconciliacao")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro: {error}", file=sys.stderr)
        sys.exit(1)
