#!/usr/bin/env python3
"""Compare steady-state memory from compare_release_memory.sh samples (#1803).

Reads samples-baseline.tsv and samples-candidate.tsv from the output
directory, takes the median of each column over the final third of each run,
writes summary.md and result.json, and exits 1 when the candidate's resident
memory exceeds the baseline by more than both the percentage tolerance and the
absolute slack. Requiring both keeps small installs' noise from failing it.
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


def steady_state(path: Path) -> dict[str, float]:
    with path.open(newline="") as handle:
        rows = [row for row in csv.DictReader(handle, delimiter="\t") if row.get("rss_kib")]
    if len(rows) < 3:
        raise SystemExit(f"{path.name}: only {len(rows)} samples; the run ended early")
    last = int(rows[-1]["elapsed_s"])
    tail = [row for row in rows if int(row["elapsed_s"]) >= last * 2 // 3] or rows[-3:]
    return {column: statistics.median(float(row[column]) for row in tail) for column in COLUMNS}


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    out = Path(argv[1])
    tolerance = float(os.environ.get("PULSE_MEMORY_TOLERANCE_PCT", "15"))
    slack_kib = float(os.environ.get("PULSE_MEMORY_SLACK_MIB", "16")) * 1024
    baseline = steady_state(out / "samples-baseline.tsv")
    candidate = steady_state(out / "samples-candidate.tsv")

    growth = candidate["rss_kib"] - baseline["rss_kib"]
    growth_pct = 100 * growth / baseline["rss_kib"] if baseline["rss_kib"] else 0.0
    regressed = growth > slack_kib and growth_pct > tolerance

    lines = [
        "## Pulse memory: candidate against baseline",
        "",
        "Steady state is the median over the final third of each run, on the fixed small mock estate.",
        "",
        "| | Baseline MiB | Candidate MiB | Change |",
        "|---|---:|---:|---:|",
    ]
    for column in COLUMNS:
        base, cand = baseline[column] / 1024, candidate[column] / 1024
        lines.append(f"| {LABELS[column]} | {base:.1f} | {cand:.1f} | {cand - base:+.1f} |")
    verdict = (
        f"**Regression:** resident memory grew {growth / 1024:+.1f} MiB ({growth_pct:+.1f}%), "
        f"past both {tolerance:g}% and {slack_kib / 1024:g} MiB."
        if regressed
        else f"No regression: resident memory changed {growth / 1024:+.1f} MiB ({growth_pct:+.1f}%)."
    )
    lines += ["", verdict]
    growth_file = out / "heap-growth.txt"
    if growth_file.exists() and growth_file.stat().st_size:
        top = growth_file.read_text().splitlines()[:30]
        lines += ["", "Heap growth by function (candidate minus baseline, in-use space):", "", "```", *top, "```"]
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
