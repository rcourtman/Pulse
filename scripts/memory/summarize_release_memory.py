#!/usr/bin/env python3
"""Compare steady-state memory from compare_release_memory.sh samples (#1803).

Usage: summarize_release_memory.py OUTPUT_DIR DURATION_SECONDS INTERVAL_SECONDS
       [BASELINE_WAIT_STATUS CANDIDATE_WAIT_STATUS [STABLE_TAG BASELINE_SHA CANDIDATE_SHA]]

Reads both sample files from the output directory and uses the final third of
the planned duration for both builds. Writes an incomplete observation instead
of a memory verdict if either sample file does not cover that window. Otherwise
exits 1 when the candidate's resident memory exceeds both the percentage
tolerance and the absolute slack. Requiring both keeps small installs' noise
from failing it.

The watch passes its two worker wait statuses. A failed worker makes the
comparison incomplete even if its sample timestamps cover the window.

The verdict uses resident memory independently of the optional mapping
breakdown. Class medians are not estimated from a partially observed window.

It also reports Linux's process RSS high-water mark through the last sample,
when available. The high-water mark is not cgroup usage and does not gate.

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
import re
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
    "proxmox": "a real Pulse polling a synthetic 3-node Proxmox VE cluster (35 guests), 2 PBS servers, and one connected dashboard",
}
SECONDS_PER_DAY = 86400
MIB = 1024 * 1024
CHILD_WAIT_LABELS = {
    "pulse_wait_status_before_stop": "Pulse wait status before the watch's stop",
    "pulse_wait_status_after_stop_request": "Pulse wait status after the watch's TERM request",
    "driver_wait_status": "load driver wait status",
}


def sample_kib(value: str | None) -> int | None:
    try:
        kib = int(value or "")
    except ValueError:
        return None
    return kib if kib >= 0 else None


def read_steady_window(path: Path, start: int, end: int, max_gap: int) -> tuple[list[dict[str, str]], dict]:
    """Require observations across the fixed window, not just anywhere in the run."""
    try:
        with path.open(newline="") as handle:
            rows = [row for row in csv.DictReader(handle, delimiter="\t") if row.get("rss_kib")]
    except OSError as exc:
        raise ValueError("RSS sample file unavailable") from exc
    try:
        elapsed = [int(row["elapsed_s"]) for row in rows]
    except (KeyError, TypeError, ValueError) as exc:
        raise ValueError("invalid RSS sample timestamps") from exc
    if elapsed and (elapsed[0] < 0 or elapsed[-1] > end
                    or any(b <= a for a, b in zip(elapsed, elapsed[1:]))):
        raise ValueError("RSS sample timestamps must increase within the planned duration")

    points = [(when, row) for when, row in zip(elapsed, rows) if when >= start]
    if any(None in row or None in row.values() for _, row in points):
        raise ValueError("misaligned RSS sample fields in the planned final third")
    if any(sample_kib(row["rss_kib"]) is None for _, row in points):
        raise ValueError("invalid resident memory sample in the planned final third")
    if len(points) < 3:
        raise ValueError(f"only {len(points)} RSS samples in the planned final third (need at least 3)")
    times = [when for when, _ in points]
    boundaries = [start, *times, end]
    left, right = max(zip(boundaries, boundaries[1:]), key=lambda pair: pair[1] - pair[0])
    if right - left > max_gap:
        raise ValueError(f"no RSS sample across {left}–{right} s (maximum gap {max_gap} s)")
    return [row for _, row in points], {"first_s": times[0], "last_s": times[-1], "count": len(times)}


def steady_state(rows: list[dict[str, str]]) -> dict[str, float | None]:
    readings = {}
    for column in COLUMNS:
        values = [sample_kib(row.get(column)) for row in rows]
        observed = [value for value in values if value is not None]
        readings[column] = statistics.median(observed) if len(observed) == len(rows) else None
    return readings


def rss_high_water_kib(rows: list[dict[str, str]]) -> int | None:
    """Process VmHWM at the last sample, not the maximum sampled VmRSS."""
    try:
        peak = int(rows[-1].get("rss_hwm_kib") or "")
    except ValueError:
        return None
    return peak if peak > 0 else None


def counter_rate(tail: list[dict[str, str]], column: str) -> float | None:
    """Per-second increase of a cumulative counter across the window, or None."""
    points = [(int(row["elapsed_s"]), float(row[column])) for row in tail if (row.get(column) or "").strip()]
    if len(points) < 2 or points[-1][0] <= points[0][0]:
        return None
    (start, first), (end, last) = points[0], points[-1]
    return max(last - first, 0.0) / (end - start)


def activity(rows: list[dict[str, str]], clk_tck: float) -> dict[str, float | None]:
    ticks = counter_rate(rows, "cpu_ticks")
    writes = counter_rate(rows, "write_bytes")
    wchar = counter_rate(rows, "wchar_bytes")
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


def read_proxmox_stats(out: Path, label: str) -> dict | None:
    try:
        return json.loads((out / f"proxmox-{label}.json").read_text())
    except (OSError, ValueError):
        return None


def pbs_snapshot_coverage(stats: dict) -> str:
    coverage = stats.get("pbs_snapshot_group_coverage")
    if coverage is None:
        return "n/a"
    stores = [store for instance in coverage.values() for store in instance.values()]
    served = sum(store["served_groups"] for store in stores)
    expected = sum(store["expected_groups"] for store in stores)
    return f"{served}/{expected}"


def pve_steady_poll_sightings(stats: dict) -> str:
    coverage = stats.get("pve_steady_poll_sightings")
    if coverage is None:
        return "n/a"
    return (
        f"{coverage['guests_with_sightings']}/{stats['guests']} sighted; "
        f"{len(coverage['guests_over_limit'])} with a gap over {coverage['limit_s']} s; "
        f"longest {coverage['longest_gap_ms'] / 1000:g} s in {coverage['window_s']} s"
    )


def pbs_datastore_rows(coverage: dict) -> list[dict]:
    return [store for instance in coverage["datastores"].values() for store in instance.values()]


def pbs_steady_datastore_status(stats: dict) -> str:
    coverage = stats.get("pbs_steady_datastore_status")
    if coverage is None:
        return "n/a"
    stores = pbs_datastore_rows(coverage)
    if not stores:
        return "n/a"
    seen = sum(store["responses"] > 0 for store in stores)
    longest = max(store["longest_gap_ms"] for store in stores) / 1000
    return f"{seen}/{len(stores)} with a response; longest gap {longest:g} s in {coverage['window_s']} s"


def proxmox_polling_limits(label: str, stats: dict | None, window_s: int) -> list[str]:
    if not stats:
        return [f"- {label}: PVE/PBS polling evidence is unavailable."]

    notes = []
    pve = stats.get("pve_steady_poll_sightings")
    if pve is None or pve["window_s"] != window_s:
        notes.append(f"- {label}: PVE guest sightings are unavailable for the RSS window.")
    elif pve["guests_with_sightings"] != stats["guests"] or pve["guests_over_limit"]:
        notes.append(f"- {label}: PVE guest sightings were missing or exceeded the {pve['limit_s']} s gap limit.")

    if stats["pbs_instances"] > 0:
        pbs = stats.get("pbs_steady_datastore_status")
        stores = pbs_datastore_rows(pbs) if pbs is not None and pbs["window_s"] == window_s else []
        if not stores:
            notes.append(f"- {label}: PBS datastore status evidence is unavailable for the RSS window.")
        elif silent := sum(store["responses"] == 0 for store in stores):
            notes.append(f"- {label}: {silent} of {len(stores)} PBS datastores had no status response in the RSS window.")
    return notes


def proxmox_lines(baseline: dict | None, candidate: dict | None) -> list[str]:
    shape = candidate or baseline
    if shape is None:
        return ["", "No Proxmox load statistics were written."]
    lines = [
        "",
        "### Proxmox traffic",
        "",
        f"{shape['pve_nodes']} PVE nodes ({shape['guests']} guests) and {shape['pbs_instances']} PBS instances "
        f"({shape['total_pbs_snapshots']} total PBS snapshots across {shape['snapshots_per_group']} snapshots/group).",
        "",
        "PBS coverage counts distinct groups for which the fixture finished serving snapshots, "
        "not snapshots Pulse retained or displayed.",
        "PBS status gaps are measured per configured datastore, including the window edges. "
        "They are diagnostic only, not proof of backup reads or of Pulse accepting the response.",
        "PVE sighting gaps describe what the dashboard observed during steady RSS sampling. "
        "They are reported only; a gap does not identify where observation stopped.",
        "",
        "| | Baseline | Candidate |",
        "|---|---:|---:|",
    ]
    rows = (
        ("PVE API requests", lambda d: d["pve_requests"]),
        ("PVE guests observed in dashboard state", lambda d: d.get("pve_guests_observed", "n/a")),
        ("PVE guests with recent poll sightings at measurement end",
         lambda d: (f"{d['pve_guests_recent']}/{d['guests']} (last {d['pve_guest_freshness_window_s']} s)"
                    if "pve_guests_recent" in d else "n/a")),
        ("PVE guest poll sightings during steady RSS sampling", pve_steady_poll_sightings),
        ("PBS API requests", lambda d: d["pbs_requests"]),
        ("PBS snapshot groups served/configured", pbs_snapshot_coverage),
        ("PBS datastore status responses during steady RSS sampling", pbs_steady_datastore_status),
        ("Dashboard connections", lambda d: d["dashboard"]["connects"]),
        ("Dashboard application messages", lambda d: d["dashboard"]["messages"]),
        ("Dashboard state messages", lambda d: d["dashboard"].get("state_messages", "n/a")),
        ("Dashboard state updates", lambda d: d["dashboard"].get("state_updates", "n/a")),
        ("Dashboard MiB received", lambda d: f"{d['dashboard']['bytes'] / MIB:.1f}"),
    )
    for label, value in rows:
        cells = [str(value(d)) if d else "n/a" for d in (baseline, candidate)]
        lines.append(f"| {label} | {cells[0]} | {cells[1]} |")
    return lines


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
        ("Dashboard connections", lambda d: d["dashboard"]["connects"]),
        ("Dashboard application messages", lambda d: d["dashboard"]["messages"]),
        # Older driver output counted arbitrary frames; it proves no state traffic.
        ("Dashboard state messages", lambda d: d["dashboard"].get("state_messages", "n/a")),
        ("Dashboard state updates", lambda d: d["dashboard"].get("state_updates", "n/a")),
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


def write_observation(out: Path, summary: str, result: dict) -> None:
    (out / "summary.md").write_text(summary, encoding="utf-8")
    (out / "result.json").write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    print(summary)


def read_child_wait_statuses(out: Path, label: str) -> dict[str, int]:
    try:
        data = json.loads((out / f"run-{label}-wait.json").read_text())
    except (OSError, ValueError):
        return {}
    if not isinstance(data, dict):
        return {}
    observed = {}
    for key in CHILD_WAIT_LABELS:
        status = data.get(key)
        if type(status) is int and 0 <= status <= 255:
            observed[key] = status
    return observed


def main(argv: list[str]) -> int:
    source = {}
    try:
        out_dir, duration_arg, interval_arg, *wait_args = argv[1:]
        if len(wait_args) == 5:
            tag, baseline_sha, candidate_sha = wait_args[2:]
            if not (re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag)
                    and all(re.fullmatch(r"[0-9a-f]{40}", sha) for sha in (baseline_sha, candidate_sha))):
                raise ValueError
            source = {"sources": {
                "baseline": {"tag": tag, "sha": baseline_sha},
                "candidate": {"sha": candidate_sha},
            }}
            wait_args = wait_args[:2]
        duration, interval = int(duration_arg), int(interval_arg)
        wait_statuses = [int(arg) for arg in wait_args]
        if (min(duration, interval) <= 0 or len(wait_statuses) not in (0, 2)
                or any(not 0 <= status <= 255 for status in wait_statuses)):
            raise ValueError
    except ValueError:
        print(__doc__, file=sys.stderr)
        return 2
    out = Path(out_dir)
    tolerance = float(os.environ.get("PULSE_MEMORY_TOLERANCE_PCT", "15"))
    slack_kib = float(os.environ.get("PULSE_MEMORY_SLACK_MIB", "16")) * 1024
    gated = os.environ.get("PULSE_MEMORY_GATE", "1") != "0"
    estate = read_estate(out)
    clk_tck = read_clk_tck(out)
    start = duration * 2 // 3
    max_gap = 2 * interval
    sampling = {"duration_s": duration, "interval_s": interval, "window_start_s": start, "max_gap_s": max_gap}
    windows = {}
    incomplete = []
    for label in ("baseline", "candidate"):
        try:
            windows[label], sampling[label] = read_steady_window(out / f"samples-{label}.tsv", start, duration, max_gap)
        except ValueError as exc:
            sampling[label] = {"error": str(exc)}
            incomplete.append(f"- {label.capitalize()}: {exc}.")
    failed_runs = {}
    for label, status in zip(("baseline", "candidate"), wait_statuses):
        if status == 0:
            continue
        observed = read_child_wait_statuses(out, label)
        failed_runs[label] = {"worker_wait_status": status, **observed}
        details = ", ".join(f"{CHILD_WAIT_LABELS[key]} {value}" for key, value in observed.items())
        incomplete.append(
            f"- {label.capitalize()} run did not complete successfully "
            f"(worker wait status {status}; {details or 'no child wait status observed'})."
        )
    heading = [f"## Pulse memory: candidate against baseline ({estate} estate)"]
    if source:
        heading += ["", f"Baseline: `{tag}` at `{baseline_sha}`. Candidate: `{candidate_sha}`."]
    if incomplete:
        result = {"status": "incomplete", "estate": estate, "sampling": sampling, **source}
        if failed_runs:
            result["failed_runs"] = failed_runs
        write_observation(out, "\n".join([
            *heading, "", f"**Incomplete measurement:** no comparison for the planned {start}–{duration} s window.",
            "", *incomplete, "",
        ]), result)
        return 1
    baseline_rows, candidate_rows = windows["baseline"], windows["candidate"]
    baseline = steady_state(baseline_rows)
    candidate = steady_state(candidate_rows)
    baseline_high_water = rss_high_water_kib(baseline_rows)
    candidate_high_water = rss_high_water_kib(candidate_rows)
    baseline_activity = activity(baseline_rows, clk_tck)
    candidate_activity = activity(candidate_rows, clk_tck)

    # The selected window requires valid VmRSS independently of smaps.
    growth = candidate["rss_kib"] - baseline["rss_kib"]
    growth_pct = 100 * growth / baseline["rss_kib"] if baseline["rss_kib"] else 0.0
    exceeded = growth > slack_kib and growth_pct > tolerance
    regressed = exceeded and gated

    lines = [
        *heading,
        "",
        f"Steady state is the median over the planned {start}–{duration} s window, on {ESTATES.get(estate, estate)}.",
        f"RSS samples in that window: {len(baseline_rows)} baseline, {len(candidate_rows)} candidate.",
        "",
        "| | Baseline MiB | Candidate MiB | Change |",
        "|---|---:|---:|---:|",
    ]
    for column in COLUMNS:
        base_kib, cand_kib = baseline[column], candidate[column]
        base = None if base_kib is None else base_kib / 1024
        cand = None if cand_kib is None else cand_kib / 1024
        lines.append(
            f"| {LABELS[column]} | {format_rate(base)} | {format_rate(cand)} | {format_change(base, cand)} |"
        )
    if any(baseline[column] is None or candidate[column] is None for column in COLUMNS[1:]):
        lines += ["", "Mapping breakdown is unavailable for at least one sample in that run's window. "
                  "The verdict uses the separately measured resident memory."]
    baseline_proxmox = read_proxmox_stats(out, "baseline")
    candidate_proxmox = read_proxmox_stats(out, "candidate")
    if estate == "proxmox":
        threshold = "past" if exceeded else "not past"
        verdict = (
            f"**Steady-state process RSS observation:** resident memory changed "
            f"{growth / 1024:+.1f} MiB ({growth_pct:+.1f}%), "
            f"{threshold} both {tolerance:g}% and {slack_kib / 1024:g} MiB of growth."
        )
        if not gated:
            verdict += " Reported only: this estate does not gate."
    elif exceeded:
        verdict = (
            f"**Steady-state RSS regression:** resident memory grew "
            f"{growth / 1024:+.1f} MiB ({growth_pct:+.1f}%), "
            f"past both {tolerance:g}% and {slack_kib / 1024:g} MiB."
        )
        if not gated:
            verdict += " Reported only: this estate does not gate."
    else:
        verdict = f"No steady-state RSS regression: resident memory changed {growth / 1024:+.1f} MiB ({growth_pct:+.1f}%)."
    lines += ["", verdict]
    if estate == "proxmox":
        limits = (proxmox_polling_limits("Baseline", baseline_proxmox, duration - start)
                  + proxmox_polling_limits("Candidate", candidate_proxmox, duration - start))
        if limits:
            lines += ["", "**Polling evidence limits the comparison.** "
                      "This RSS change does not establish memory use under equivalent polling.", "", *limits]

    baseline_peak_mib = baseline_high_water / 1024 if baseline_high_water is not None else None
    candidate_peak_mib = candidate_high_water / 1024 if candidate_high_water is not None else None
    lines += [
        "",
        "### Peak process memory",
        "",
        "Linux `VmHWM` through the last sample can include peaks between samples. "
        "It does not measure cgroup usage or affect the steady-state verdict.",
        "",
        "| | Baseline MiB | Candidate MiB | Change |",
        "|---|---:|---:|---:|",
        f"| Process RSS high-water mark | {format_rate(baseline_peak_mib)} | "
        f"{format_rate(candidate_peak_mib)} | {format_change(baseline_peak_mib, candidate_peak_mib)} |",
    ]
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
    elif estate == "proxmox":
        lines += proxmox_lines(baseline_proxmox, candidate_proxmox)
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
    write_observation(out, "\n".join(lines) + "\n", {
        "status": "measured",
        "sampling": sampling,
        "baseline": baseline,
        "candidate": candidate,
        "baseline_rss_high_water_kib": baseline_high_water,
        "candidate_rss_high_water_kib": candidate_high_water,
        "baseline_activity": baseline_activity,
        "candidate_activity": candidate_activity,
        "clk_tck": clk_tck,
        "estate": estate,
        "baseline_agents": baseline_agents,
        "candidate_agents": candidate_agents,
        "baseline_proxmox": baseline_proxmox,
        "candidate_proxmox": candidate_proxmox,
        "gated": gated,
        "exceeded": exceeded,
        "rss_growth_kib": growth,
        "rss_growth_pct": growth_pct,
        "tolerance_pct": tolerance,
        "slack_kib": slack_kib,
        "regressed": regressed,
        **source,
    })
    return 1 if regressed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
