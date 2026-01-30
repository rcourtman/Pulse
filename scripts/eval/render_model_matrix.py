#!/usr/bin/env python3
import argparse
import datetime as dt
import json
from pathlib import Path


SCENARIO_COLUMNS = {
    "Quick Smoke Test": "Smoke",
    "Read-Only Infrastructure": "Read-only",
}

EXCLUDED_MODEL_IDS = {
    "openai:gpt-5.2-pro",
}

EXCLUDED_MODEL_KEYWORDS = (
    "codex",
    "embedding",
    "image",
    "vision",
    "video",
    "audio",
    "speech",
    "tts",
    "transcribe",
    "rerank",
    "moderation",
    "realtime",
)

TRANSIENT_ERROR_MARKERS = (
    "rate limit",
    "resource has been exhausted",
    "quota",
    "429",
    "not a chat model",
    "v1/chat/completions endpoint",
)


def parse_time(value):
    if not value:
        return None
    if isinstance(value, (int, float)):
        return dt.datetime.utcfromtimestamp(value)
    if isinstance(value, str):
        text = value.strip()
        if text.endswith("Z"):
            text = text[:-1] + "+00:00"
        try:
            return dt.datetime.fromisoformat(text)
        except ValueError:
            return None
    return None


def normalize_scenario(name):
    if not name:
        return None
    for key in SCENARIO_COLUMNS:
        if name == key:
            return key
    for key in SCENARIO_COLUMNS:
        if key.lower() in name.lower():
            return key
    return None


def load_reports(report_dir):
    report_dir = Path(report_dir)
    if not report_dir.exists():
        return []
    return sorted(report_dir.glob("*.json"))


def build_matrix(report_paths):
    records = {}
    for path in report_paths:
        try:
            payload = json.loads(path.read_text())
        except Exception:
            continue
        model = (payload.get("model") or "").strip()
        if not model:
            continue
        if should_exclude_model(model):
            continue
        generated_at = parse_time(payload.get("generated_at"))
        result = payload.get("result") or {}
        scenario = normalize_scenario(result.get("ScenarioName"))
        if not scenario:
            continue
        if has_transient_error(result):
            continue
        passed = bool(result.get("Passed"))
        duration_ns = int(result.get("Duration") or 0)
        tokens = 0
        for step in result.get("Steps") or []:
            tokens += int(step.get("InputTokens") or 0)
            tokens += int(step.get("OutputTokens") or 0)

        model_entry = records.setdefault(model, {"scenarios": {}, "last_run": None})
        existing = model_entry["scenarios"].get(scenario)
        if existing is None or (generated_at and existing["generated_at"] and generated_at > existing["generated_at"]) or (generated_at and existing["generated_at"] is None):
            model_entry["scenarios"][scenario] = {
                "passed": passed,
                "generated_at": generated_at,
                "duration_ns": duration_ns,
                "tokens": tokens,
            }
        if generated_at:
            last_run = model_entry["last_run"]
            if last_run is None or generated_at > last_run:
                model_entry["last_run"] = generated_at
    return records


def should_exclude_model(model_id):
    if model_id in EXCLUDED_MODEL_IDS:
        return True
    lowered = model_id.lower()
    for keyword in EXCLUDED_MODEL_KEYWORDS:
        if keyword and keyword in lowered:
            return True
    return False


def has_transient_error(result):
    steps = result.get("Steps") or []
    for step in steps:
        error_text = str(step.get("Error") or "").lower()
        if error_text and contains_any(error_text, TRANSIENT_ERROR_MARKERS):
            return True
        for event in step.get("RawEvents") or []:
            if event.get("Type") != "error":
                continue
            data = event.get("Data")
            if isinstance(data, (dict, list)):
                text = json.dumps(data)
            else:
                text = str(data or "")
            if contains_any(text.lower(), TRANSIENT_ERROR_MARKERS):
                return True
    return False


def format_status(passed):
    if passed is True:
        return "✅"
    if passed is False:
        return "❌"
    return "—"


def render_table(records):
    header = ["Model", "Smoke", "Read-only", "Time (matrix)", "Tokens (matrix)", "Last run (UTC)"]
    rows = [header, ["---"] * len(header)]

    def sort_key(model_id):
        if ":" in model_id:
            provider, name = model_id.split(":", 1)
        else:
            provider, name = "", model_id
        return (provider, name)

    for model_id in sorted(records.keys(), key=sort_key):
        entry = records[model_id]
        scenarios = entry["scenarios"]
        last_run = entry["last_run"]
        last_run_text = last_run.strftime("%Y-%m-%d") if last_run else "—"
        smoke = scenarios.get("Quick Smoke Test")
        readonly = scenarios.get("Read-Only Infrastructure")
        total_duration = 0
        total_tokens = 0
        for scenario in (smoke, readonly):
            if scenario:
                total_duration += int(scenario.get("duration_ns") or 0)
                total_tokens += int(scenario.get("tokens") or 0)
        duration_text = format_duration(total_duration) if total_duration else "—"
        tokens_text = f"{total_tokens:,}" if total_tokens else "—"
        rows.append([
            model_id,
            format_status(smoke["passed"] if smoke else None),
            format_status(readonly["passed"] if readonly else None),
            duration_text,
            tokens_text,
            last_run_text,
        ])

    if len(rows) == 2:
        rows.append(["_No results yet_", "—", "—", "—", "—", "—"])

    return "\n".join("| " + " | ".join(row) + " |" for row in rows)


def format_duration(ns):
    if not ns:
        return "—"
    seconds = int(ns / 1_000_000_000)
    if seconds < 60:
        return f"{seconds}s"
    minutes = seconds // 60
    rem = seconds % 60
    if minutes < 60:
        return f"{minutes}m {rem}s"
    hours = minutes // 60
    rem_m = minutes % 60
    return f"{hours}h {rem_m}m"


def contains_any(text, markers):
    for marker in markers:
        if marker and marker in text:
            return True
    return False


def update_doc(doc_path, table):
    doc_path = Path(doc_path)
    content = doc_path.read_text()
    start = "<!-- MODEL_MATRIX_START -->"
    end = "<!-- MODEL_MATRIX_END -->"
    if start not in content or end not in content:
        raise RuntimeError("MODEL_MATRIX markers not found in doc")
    prefix, rest = content.split(start, 1)
    _, suffix = rest.split(end, 1)
    next_content = prefix + start + "\n" + table + "\n" + end + suffix
    doc_path.write_text(next_content)


def main():
    parser = argparse.ArgumentParser(description="Render the Pulse Assistant model matrix table.")
    parser.add_argument("report_dir", nargs="?", default="tmp/eval-reports", help="Directory with eval report JSON files.")
    parser.add_argument("--write-doc", default="", help="Path to doc file to update in-place.")
    args = parser.parse_args()

    reports = load_reports(args.report_dir)
    records = build_matrix(reports)
    table = render_table(records)

    if args.write_doc:
        update_doc(args.write_doc, table)
    else:
        print(table)


if __name__ == "__main__":
    main()
