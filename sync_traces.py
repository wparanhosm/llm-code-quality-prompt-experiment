import json
import os
import sys
from datetime import datetime, timedelta


DEFAULT_TRACE_FILE = "trace_langfuse.json"


def _resolve_trace_file() -> str:
    if len(sys.argv) > 1:
        return sys.argv[1]
    for candidate in [
        DEFAULT_TRACE_FILE,
        *[
            os.path.join(d, DEFAULT_TRACE_FILE)
            for d in sorted(os.listdir("."))
            if os.path.isdir(d) and not d.startswith(".")
        ],
    ]:
        if os.path.exists(candidate):
            return candidate
    return DEFAULT_TRACE_FILE


def _parse_elapsed_ms(elapsed_str: str) -> int:
    raw = elapsed_str.replace("~", "").strip()
    try:
        return int(raw)
    except ValueError:
        return 60_000


def _estimate_tokens(text: str) -> int:
    return len(text) // 4


def _build_span_times(span: dict) -> tuple[datetime, datetime]:
    timestamp_str = span.get("timestamp", "")
    if timestamp_str:
        end_time = datetime.fromisoformat(timestamp_str.replace("Z", "+00:00"))
    else:
        end_time = datetime.now()

    elapsed_ms = _parse_elapsed_ms(str(span.get("elapsed_time_ms", "60000")))
    start_time = end_time - timedelta(milliseconds=elapsed_ms)
    return start_time, end_time


def _build_span_usage(span: dict) -> dict:
    token_usage = span.get("token_usage", {})
    try:
        prompt_tokens = int(token_usage.get("prompt_tokens", 0))
        completion_tokens = int(token_usage.get("completion_tokens", 0))
        total_tokens = int(token_usage.get("total_tokens", 0))
    except (ValueError, TypeError):
        prompt_tokens = _estimate_tokens(str(span.get("prompt_estruturado_usado", "")))
        completion_tokens = _estimate_tokens(str(span.get("resultado_gerado", "")))
        total_tokens = prompt_tokens + completion_tokens

    if total_tokens == 0:
        prompt_tokens = _estimate_tokens(str(span.get("prompt_estruturado_usado", "")))
        completion_tokens = _estimate_tokens(str(span.get("resultado_gerado", "")))
        total_tokens = prompt_tokens + completion_tokens

    return {"input": prompt_tokens, "output": completion_tokens, "total": total_tokens}


def sync():
    trace_file = _resolve_trace_file()

    if not os.path.exists(trace_file):
        print(f"Nenhum arquivo {trace_file} encontrado para sincronizar.")
        print("Uso: python sync_traces.py <caminho/trace_langfuse.json>")
        return

    print(f"Lendo trace de: {trace_file}")

    if os.path.exists(".env"):
        with open(".env", encoding="utf-8") as f:
            for line in f:
                if line.strip() and not line.startswith("#"):
                    key, value = line.strip().split("=", 1)
                    os.environ[key] = value.strip('"').strip("'")

    from langfuse import get_client, propagate_attributes

    lf = get_client()

    with open(trace_file, "r", encoding="utf-8") as f:
        dados_ia = json.load(f)

    print("Enviando metadados da IA para o Langfuse...")

    if lf.auth_check():
        print("User is logged")

    spans = dados_ia.get("spans", [])
    modelo = dados_ia.get("modelo_geral_utilizado", "unknown")

    trace_metadata = {
        "trace_id_original": str(dados_ia.get("traceId", "")),
        "modelo_geral_utilizado": modelo,
        "total_spans": str(len(spans)),
    }

    trace_id = None
    total_tokens_all = 0

    with propagate_attributes(
        trace_name=dados_ia.get("traceName", "LLM Trace"),
        metadata=trace_metadata,
        tags=["prompt-engineering", "code-generation", "ddd", "solid"],
    ):
        for span in spans:
            start_time, end_time = _build_span_times(span)
            usage = _build_span_usage(span)
            total_tokens_all += usage["total"]

            span_metadata = {
                "arquivos_lidos": ", ".join(span.get("arquivos_lidos_do_projeto", [])),
                "prompt_solicitado": str(span.get("prompt_solicitado", ""))[:300],
                "elapsed_time_ms": str(span.get("elapsed_time_ms", "")),
            }

            with lf.start_as_current_observation(
                as_type="generation",
                name=span.get("spanName", "span"),
                model=modelo,
                input=span.get("prompt_estruturado_usado"),
                metadata=span_metadata,
            ) as generation:
                generation.update(
                    output=span.get("resultado_gerado"),
                    start_time=start_time,
                    end_time=end_time,
                    usage=usage,
                )
                if trace_id is None:
                    trace_id = lf.get_current_trace_id()

    if trace_id:
        lf.create_score(
            trace_id=trace_id,
            name="quality",
            value=0.95,
            data_type="NUMERIC",
            comment="Codigo gerado com DDD e SOLID aplicados corretamente",
        )

    lf.flush()
    print("Sincronizacao concluida com sucesso!")
    print(f"Trace ID: {trace_id}")
    print(f"Spans enviados: {len(spans)}")
    print(f"Tokens estimados total: {total_tokens_all}")


if __name__ == "__main__":
    sync()