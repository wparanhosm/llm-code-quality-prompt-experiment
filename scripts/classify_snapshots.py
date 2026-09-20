import argparse
import hashlib
import json
import os
import stat
import sys
import concurrent.futures
from collections import Counter
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[1]
RUNS = ROOT / "runs"
PROVENANCE = ROOT / "provenance" / "snapshots"
OVERRIDES_PATH = ROOT / "benchmark" / "classification-overrides.json"


def hash_bytes(data):
    return hashlib.sha256(data).hexdigest()


def hash_file(path):
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def snapshot_entries(source):
    entries = []
    for directory, dirnames, filenames in os.walk(source, topdown=True, followlinks=False):
        directory_path = Path(directory)
        dirnames.sort()
        filenames.sort()

        symlink_dirs = []
        for name in dirnames:
            path = directory_path / name
            if path.is_symlink():
                symlink_dirs.append(name)
                entries.append(symlink_entry(source, path))
        dirnames[:] = [name for name in dirnames if name not in symlink_dirs]

        for name in filenames:
            path = directory_path / name
            entries.append(symlink_entry(source, path) if path.is_symlink() else file_entry(source, path))

    return sorted(entries, key=lambda item: item["path"])


def relative_path(source, path):
    return path.relative_to(source).as_posix()


def file_entry(source, path):
    before = path.stat(follow_symlinks=False)
    digest = hash_file(path)
    after = path.stat(follow_symlinks=False)
    identity_before = (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
    identity_after = (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns)
    if identity_before != identity_after:
        raise RuntimeError(f"arquivo mudou durante a leitura: {path}")
    return {
        "path": relative_path(source, path),
        "type": "file",
        "mode": f"{stat.S_IMODE(after.st_mode):04o}",
        "size": after.st_size,
        "sha256": digest,
    }


def symlink_entry(source, path):
    target = os.readlink(path)
    metadata = path.lstat()
    return {
        "path": relative_path(source, path),
        "type": "symlink",
        "mode": f"{stat.S_IMODE(metadata.st_mode):04o}",
        "size": metadata.st_size,
        "sha256": hash_bytes(target.encode("utf-8", errors="surrogateescape")),
        "target": target,
    }


def root_hash(entries):
    canonical = json.dumps(entries, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
    return hash_bytes(canonical.encode("utf-8"))


def load_overrides():
    payload = json.loads(OVERRIDES_PATH.read_text(encoding="utf-8"))
    return {(item["run_id"], item["path"]): item for item in payload["overrides"]}


def classify(run_id, path, entry, overrides):
    override = overrides.get((run_id, path))
    if override:
        return {key: value for key, value in override.items() if key not in {"run_id"}}

    posix = PurePosixPath(path)
    name = posix.name
    suffix = posix.suffix.lower()
    parts = posix.parts

    if "node_modules" in parts:
        return result(path, "generation_artifact", "installed-dependency", "high")
    if entry["type"] == "symlink":
        return result(path, "unknown", "symbolic-link", "low", "Requer revisao manual do destino do link.")
    if name in {"billing_result.txt", "billing_results.txt"}:
        return result(path, "generation_metadata", "billing-metadata", "high")
    if name == "coverage.out" or name.startswith("coverage_") or suffix in {".db", ".db-wal", ".db-shm"}:
        return result(path, "generation_artifact", "preexisting-generated-output", "high")
    if name in {"api", "wallet-p2p-api"} or parts[:1] == ("bin",):
        return result(path, "generation_artifact", "installed-dependency-or-binary", "high")
    if name.endswith("_test.go"):
        return result(path, "generated_test", "go-test-source", "high")
    if suffix == ".go":
        return result(path, "application_source", "go-production-source", "high")
    if suffix == ".sql" or "migrations" in parts:
        return result(path, "database_definition", "database-schema-or-seed", "high")
    if name in {"go.mod", "go.sum", "package.json", "package-lock.json"}:
        return result(path, "dependency_manifest", "dependency-manifest", "high")
    if ".opencode" in parts or name == "AGENTS.md":
        return result(path, "generation_context", "generation-agent-context", "high")
    if name in {"Dockerfile", "Makefile", "compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml", ".dockerignore", ".gitignore"}:
        return result(path, "model_operational_config", "model-provided-operational-config", "high")
    if parts[:1] == ("scripts",):
        return result(path, "model_operational_config", "model-provided-script", "medium")
    if suffix in {".md", ".rst"} or name.lower().startswith("readme"):
        return result(path, "documentation", "project-documentation", "high")
    return result(path, "unknown", "no-matching-rule", "low")


def result(path, category, rule, confidence, notes=None):
    item = {"path": path, "category": category, "rule": rule, "confidence": confidence}
    if notes:
        item["notes"] = notes
    return item


def discover_runs():
    runs = []
    for model in sorted(path for path in RUNS.iterdir() if path.is_dir()):
        for scenario in sorted(path for path in model.iterdir() if path.is_dir()):
            runs.append((f"{model.name}-{scenario.name}", scenario))
    return runs


def build_run(run_id, source, overrides):
    entries = snapshot_entries(source)
    snapshot_hash = root_hash(entries)

    manifest = {
        "schema_version": "1.0",
        "run_id": run_id,
        "source_path": source.relative_to(ROOT).as_posix(),
        "hash_algorithm": "sha256",
        "root_hash": snapshot_hash,
        "entries": entries,
    }
    classified = [classify(run_id, item["path"], item, overrides) for item in entries]
    counts = Counter(item["category"] for item in classified)
    classification = {
        "schema_version": "1.0",
        "run_id": run_id,
        "source_root_hash": snapshot_hash,
        "entries": classified,
        "summary": dict(sorted(counts.items())),
    }
    return manifest, classification


def write_json(path, payload):
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=False) + "\n", encoding="utf-8")


def checksum_line(manifest):
    return f'{manifest["root_hash"]}  {manifest["source_path"]}\n'


def main():
    parser = argparse.ArgumentParser(description="Classifica e gera manifestos dos snapshots imutaveis em runs/.")
    parser.add_argument("--verify", action="store_true", help="verifica os manifestos existentes sem grava-los")
    args = parser.parse_args()

    overrides = load_overrides()
    def process_snapshot(run_id, source):
        manifest, classification = build_run(run_id, source, overrides)
        output = PROVENANCE / run_id

        if args.verify:
            expected_manifest = json.loads((output / "source-manifest.json").read_text(encoding="utf-8"))
            expected_classification = json.loads((output / "classification.json").read_text(encoding="utf-8"))
            if manifest != expected_manifest or classification != expected_classification:
                raise RuntimeError(f"proveniencia divergente para {run_id}")
            if (output / "snapshot.sha256").read_text(encoding="ascii") != checksum_line(manifest):
                raise RuntimeError(f"checksum selado divergente para {run_id}")
        else:
            existing_manifest_path = output / "source-manifest.json"
            if existing_manifest_path.is_file():
                existing = json.loads(existing_manifest_path.read_text(encoding="utf-8"))
                if existing.get("root_hash") != manifest["root_hash"]:
                    raise RuntimeError(
                        f"snapshot selado foi alterado: {run_id}; use --verify para diagnosticar"
                    )
            output.mkdir(parents=True, exist_ok=True)
            write_json(output / "source-manifest.json", manifest)
            write_json(output / "classification.json", classification)
            (output / "snapshot.sha256").write_text(checksum_line(manifest), encoding="ascii")

        return {
            "run_id": run_id,
            "source_path": manifest["source_path"],
            "root_hash": manifest["root_hash"],
            "file_count": len(manifest["entries"]),
            "total_bytes": sum(item["size"] for item in manifest["entries"]),
            "classification": classification["summary"],
            "unknown_count": classification["summary"].get("unknown", 0),
        }

    consolidated = []
    with concurrent.futures.ThreadPoolExecutor() as executor:
        futures = {executor.submit(process_snapshot, run_id, source): run_id for run_id, source in discover_runs()}
        for future in concurrent.futures.as_completed(futures):
            run_id = futures[future]
            try:
                consolidated.append(future.result())
            except Exception as exc:
                print(f"snapshot {run_id} falhou: {exc}", file=sys.stderr)
                sys.exit(1)
    
    consolidated.sort(key=lambda item: item["run_id"])

    summary = {"schema_version": "1.0", "runs": consolidated}
    summary_path = ROOT / "provenance" / "classification-summary.json"
    if args.verify:
        if summary != json.loads(summary_path.read_text(encoding="utf-8")):
            raise RuntimeError("resumo consolidado divergente")
        print(f"verificados {len(consolidated)} snapshots sem divergencias")
    else:
        summary_path.parent.mkdir(parents=True, exist_ok=True)
        write_json(summary_path, summary)
        print(f"classificados {len(consolidated)} snapshots")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"erro: {error}", file=sys.stderr)
        sys.exit(1)
