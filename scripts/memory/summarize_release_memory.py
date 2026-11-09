#!/usr/bin/env python3
"""Compare steady-state memory from compare_release_memory.sh samples (#1803).

Reads samples-baseline.tsv and samples-candidate.tsv from the output
directory, takes the median of each column over the final third of each run,
writes summary.md and result.json, and exits 1 when the candidate's resident
memory exceeds the baseline by more than both the percentage tolerance and the
absolute slack. Requiring both keeps small installs' noise from failing it.

Over the same final third it also reports CPU (percent of one core) and disk
writes extrapolated to a day (#2199, #1966). Those rates are reported, not
gated, until measured budgets exist for them.

The estate file names what was measured. For the agents estate the summary
also shows each driver's accepted reports and dashboard traffic. Setting
PULSE_MEMORY_GATE=0 reports memory without failing, for an estate whose noise
is not yet known.
"""

from __future__ import annotations

import csv
import json
import os
import statistics
import sys
from pathlib import Path

COLUMNS = ("rss_kib", "go_heap_kib", "go_other_kib", "anon_other_kib", "file_kib")
LABELS = {
    "rss_kib": "Resident",
    "go_heap_kib": "Go heap",
    "go_other_kib": "Go runtime",
    "anon_other_kib": "Outside Go (SQLite and C allocations)",
    "file_kib": "File pages",
}
ACTIVITY_LABELS = {
    "cpu_pct": ("CPU", "% of one core"),
    "write_mib_per_day": ("Disk writes", "MiB/day"),
    "wchar_mib_per_day": ("Write calls", "MiB/day"),
}
ESTATES = {
    "mock": "the fixed small mock estate (2 nodes, 24 guests, a Docker host with 10 containers)",
    "agents": "a real Pulse fed by synthetic Unified Agents and one connected dashboard",
}
SECONDS_PER_DAY = 86400
MIB = 1024 * 1024


def read_rows(path: Path) -> list[dict[str, str]]:
    with path.open(newline="") as handle:
        rows = [row for row in csv.DictReader(handle, delimiter="\t") if row.get("rss_kib")]
    if len(rows) < 3:
        raise SystemExit(f"{path.name}: only {len(rows)} samples; the run ended early")
    return rows


def final_third(rows: list[dict[str, str]]) -> list[dict[str, str]]:
    last = int(rows[-1]["elapsed_s"])
    return [row for row in rows if int(row["elapsed_s"]) >= last * 2 // 3] or rows[-3:]


def steady_state(rows: list[dict[str, str]]) -> dict[str, float]:
    tail = final_third(rows)
    return {column: statistics.median(float(row[column]) for row in tail) for column in COLUMNS}


def counter_rate(tail: list[dict[str, str]], column: str) -> float | None:
    """Per-second increase of a cumulative counter across the window, or None."""
    points = [(int(row["elapsed_s"]), float(row[column])) for row in tail if (row.get(column) or "").strip()]
    if len(points) < 2 or points[-1][0] <= points[0][0]:
        return None
    (start, first), (end, last) = points[0], points[-1]
    return max(last - first, 0.0) / (end - start)


def activity(rows: list[dict[str, str]], clk_tck: float) -> dict[str, float | None]:
    tail = final_third(rows)
    ticks = counter_rate(tail, "cpu_ticks")
    writes = counter_rate(tail, "write_bytes")
    wchar = counter_rate(tail, "wchar_bytes")
    return {
        "cpu_pct": None if ticks is None else 100 * ticks / clk_tck,
        "write_mib_per_day": None if writes is None else writes * SECONDS_PER_DAY / MIB,
        "wchar_mib_per_day": None if wchar is None else wchar * SECONDS_PER_DAY / MIB,
    }


def read_clk_tck(out: Path) -> float:
    try:
        return float((out / "clk_tck").read_text().strip())
    except (OSError, ValueError):
        return 100.0


def read_estate(out: Path) -> str:
    try:
        return (out / "estate").read_text().strip() or "mock"
    except OSError:
        return "mock"


def read_agent_stats(out: Path, label: str) -> dict | None:
    try:
        return json.loads((out / f"agents-{label}.json").read_text())
    except (OSError, ValueError):
        return None


def agent_lines(baseline: dict | None, candidate: dict | None) -> list[str]:
    shape = candidate or baseline
    if shape is None:
        return ["", "No agent load statistics were written."]
    lines = [
        "",
        "### Agent traffic",
        "",
        f"{shape['agents']} agents report every {shape['interval_s']}s; {shape['docker_agents']} also send "
        f"Docker reports with {shape['containers_per_docker_agent']} containers each.",
        "",
        "| | Baseline | Candidate |",
        "|---|---:|---:|",
    ]
    rows = (
        ("Host reports accepted", lambda d: d["accepted"]["host"]),
        ("Docker reports accepted", lambda d: d["accepted"]["docker"]),
        ("Reports failed", lambda d: sum(d["failed"].values())),
        ("Host report p95 ms", lambda d: (d.get("latency") or {}).get("host", {}).get("p95_ms", "n/a")),
        ("Host report max ms", lambda d: (d.get("latency") or {}).get("host", {}).get("max_ms", "n/a")),
        ("Dashboard state messages", lambda d: d["dashboard"]["messages"]),
        ("Dashboard MiB received", lambda d: f"{d['dashboard']['bytes'] / MIB:.1f}"),
    )
    for label, value in rows:
        cells = [str(value(d)) if d else "n/a" for d in (baseline, candidate)]
        lines.append(f"| {label} | {cells[0]} | {cells[1]} |")
    return lines


def format_rate(value: float | None) -> str:
    return "n/a" if value is None else f"{value:.1f}"


def format_change(base: float | None, cand: float | None) -> str:
    if base is None or cand is None:
        return "n/a"
    change = cand - base
    return f"{0.0 if abs(change) < 0.05 else change:+.1f}"


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    out = Path(argv[1])
    tolerance = float(os.environ.get("PULSE_MEMORY_TOLERANCE_PCT", "15"))
    slack_kib = float(os.environ.get("PULSE_MEMORY_SLACK_MIB", "16")) * 1024
    gated = os.environ.get("PULSE_MEMORY_GATE", "1") != "0"
    estate = read_estate(out)
    clk_tck = read_clk_tck(out)
    baseline_rows = read_rows(out / "samples-baseline.tsv")
    candidate_rows = read_rows(out / "samples-candidate.tsv")
    baseline = steady_state(baseline_rows)
    candidate = steady_state(candidate_rows)
    baseline_activity = activity(baseline_rows, clk_tck)
    candidate_activity = activity(candidate_rows, clk_tck)

    growth = candidate["rss_kib"] - baseline["rss_kib"]
    growth_pct = 100 * growth / baseline["rss_kib"] if baseline["rss_kib"] else 0.0
    exceeded = growth > slack_kib and growth_pct > tolerance
    regressed = exceeded and gated

    lines = [
        f"## Pulse memory: candidate against baseline ({estate} estate)",
        "",
        f"Steady state is the median over the final third of each run, on {ESTATES.get(estate, estate)}.",
        "",
        "| | Baseline MiB | Candidate MiB | Change |",
        "|---|---:|---:|---:|",
    ]
    for column in COLUMNS:
        base, cand = baseline[column] / 1024, candidate[column] / 1024
        lines.append(f"| {LABELS[column]} | {base:.1f} | {cand:.1f} | {cand - base:+.1f} |")
    if exceeded:
        verdict = (
            f"**Regression:** resident memory grew {growth / 1024:+.1f} MiB ({growth_pct:+.1f}%), "
            f"past both {tolerance:g}% and {slack_kib / 1024:g} MiB."
        )
        if not gated:
            verdict += " Reported only: this estate does not gate."
    else:
        verdict = f"No regression: resident memory changed {growth / 1024:+.1f} MiB ({growth_pct:+.1f}%)."
    lines += ["", verdict]
    lines += [
        "",
        "### CPU and disk writes",
        "",
        "Rates over the same final third; writes are extrapolated to a day. Reported, not gated.",
        "",
        "| | Unit | Baseline | Candidate | Change |",
        "|---|---|---:|---:|---:|",
    ]
    for key, (label, unit) in ACTIVITY_LABELS.items():
        base, cand = baseline_activity[key], candidate_activity[key]
        lines.append(f"| {label} | {unit} | {format_rate(base)} | {format_rate(cand)} | {format_change(base, cand)} |")
    baseline_agents = read_agent_stats(out, "baseline")
    candidate_agents = read_agent_stats(out, "candidate")
    if estate == "agents":
        lines += agent_lines(baseline_agents, candidate_agents)
    growth_file = out / "heap-growth.txt"
    if growth_file.exists() and growth_file.stat().st_size:
        top = growth_file.read_text().splitlines()[:30]
        lines += ["", "Heap growth by function (candidate minus baseline, in-use space):", "", "```", *top, "```"]
    if estate == "mock":
        lines += [
            "",
            "Mock mode keeps its own fixtures in the heap, so check the growth list for `internal/mock` before",
            "attributing a change to production code.",
        ]
    summary = "\n".join(lines) + "\n"
    (out / "summary.md").write_text(summary)
    (out / "result.json").write_text(json.dumps({
        "baseline": baseline,
        "candidate": candidate,
        "baseline_activity": baseline_activity,
        "candidate_activity": candidate_activity,
        "clk_tck": clk_tck,
        "estate": estate,
        "baseline_agents": baseline_agents,
        "candidate_agents": candidate_agents,
        "gated": gated,
        "exceeded": exceeded,
        "rss_growth_kib": growth,
        "rss_growth_pct": growth_pct,
        "tolerance_pct": tolerance,
        "slack_kib": slack_kib,
        "regressed": regressed,
    }, indent=2) + "\n")
    print(summary)
    return 1 if regressed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
