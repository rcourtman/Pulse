#!/usr/bin/env python3
"""Check the release memory summary's RSS, CPU and disk observations."""

import importlib.util
import io
import json
import os
from contextlib import redirect_stdout
from pathlib import Path
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location(
    "summarize_release_memory", ROOT / "scripts/memory/summarize_release_memory.py"
)
summary = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(summary)

HEADER = "elapsed_s\trss_kib\tgo_heap_kib\tgo_other_kib\tanon_other_kib\tfile_kib\tcpu_ticks\twrite_bytes\twchar_bytes\n"


def samples(rss_kib, ticks_per_s, write_bytes_per_s, io=True, rss_hwm=None, end=900, interval=15):
    rows = [HEADER if rss_hwm is None else HEADER.rstrip("\n") + "\trss_hwm_kib\n"]
    for elapsed in range(0, end + 1, interval):
        write = str(write_bytes_per_s * elapsed) if io else ""
        wchar = str(2 * write_bytes_per_s * elapsed) if io else ""
        row = f"{elapsed}\t{rss_kib}\t50000\t10000\t20000\t30000\t{ticks_per_s * elapsed}\t{write}\t{wchar}"
        if rss_hwm is not None:
            row += f"\t{rss_hwm(elapsed)}"
        rows.append(row + "\n")
    return "".join(rows)


def select_samples(raw, keep):
    header, *rows = raw.splitlines(keepends=True)
    return header + "".join(row for row in rows if keep(int(row.split("\t", 1)[0])))


class ReleaseMemorySummaryTests(unittest.TestCase):
    def run_summary(self, baseline, candidate, estate=None, agents=None, env=None, duration=900, interval=15):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            (out / "samples-baseline.tsv").write_text(baseline)
            (out / "samples-candidate.tsv").write_text(candidate)
            (out / "clk_tck").write_text("100\n")
            if estate:
                (out / "estate").write_text(estate + "\n")
            for label, stats in (agents or {}).items():
                (out / f"agents-{label}.json").write_text(json.dumps(stats))
            saved = {key: os.environ.get(key) for key in (env or {})}
            os.environ.update(env or {})
            try:
                with redirect_stdout(io.StringIO()):
                    code = summary.main(["summarize", str(out), str(duration), str(interval)])
            finally:
                for key, value in saved.items():
                    if value is None:
                        os.environ.pop(key, None)
                    else:
                        os.environ[key] = value
            return code, json.loads((out / "result.json").read_text()), (out / "summary.md").read_text()

    def test_agents_estate_reports_traffic_and_can_skip_the_gate(self):
        stats = {
            "agents": 30, "docker_agents": 6, "containers_per_docker_agent": 40, "interval_s": 30,
            "accepted": {"host": 900, "docker": 180}, "failed": {"host": 1, "docker": 0},
            "dashboard": {"connects": 1, "messages": 452, "bytes": 3 * 1024 * 1024,
                          "state_messages": 450, "state_updates": 449},
        }
        code, result, text = self.run_summary(
            samples(130000, 25, 0), samples(170000, 25, 0), estate="agents",
            agents={"baseline": stats, "candidate": stats}, env={"PULSE_MEMORY_GATE": "0"},
        )
        self.assertEqual(code, 0)
        self.assertTrue(result["exceeded"])
        self.assertFalse(result["regressed"])
        self.assertIn("(agents estate)", text)
        self.assertIn("Reported only: this estate does not gate.", text)
        self.assertIn("| Host reports accepted | 900 | 900 |", text)
        self.assertIn("| Dashboard MiB received | 3.0 | 3.0 |", text)
        self.assertIn("| Dashboard application messages | 452 | 452 |", text)
        self.assertIn("| Dashboard state messages | 450 | 450 |", text)
        self.assertIn("| Dashboard state updates | 449 | 449 |", text)
        self.assertNotIn("Mock mode keeps", text)

    def test_legacy_frame_counts_are_not_reported_as_verified_state(self):
        stats = {
            "agents": 30, "docker_agents": 6, "containers_per_docker_agent": 40, "interval_s": 30,
            "accepted": {"host": 900, "docker": 180}, "failed": {"host": 0, "docker": 0},
            "dashboard": {"connects": 1, "messages": 450, "bytes": 3 * 1024 * 1024},
        }
        lines = "\n".join(summary.agent_lines(stats, stats))
        self.assertIn("| Dashboard application messages | 450 | 450 |", lines)
        self.assertIn("| Dashboard state messages | n/a | n/a |", lines)
        self.assertIn("| Dashboard state updates | n/a | n/a |", lines)

    def test_proxmox_reports_served_pbs_groups_without_inventing_legacy_coverage(self):
        legacy = {
            "pve_nodes": 3, "guests": 35, "pbs_instances": 2, "total_pbs_snapshots": 5200,
            "snapshots_per_group": 50, "pve_requests": 914, "pbs_requests": 210,
            "dashboard": {"connects": 1, "messages": 60, "bytes": 1024},
        }
        candidate = dict(legacy, pve_guests_observed=35,
                         pve_guests_recent=34, pve_guest_freshness_window_s=60, pbs_snapshot_group_coverage={
            "pbs1": {"store-fast": {"served_groups": 35, "expected_groups": 35},
                     "store-archive": {"served_groups": 17, "expected_groups": 17}},
            "pbs2": {"store-fast": {"served_groups": 35, "expected_groups": 35},
                     "store-archive": {"served_groups": 5, "expected_groups": 17}},
        })
        lines = "\n".join(summary.proxmox_lines(legacy, candidate))
        self.assertIn("| PVE guests observed in dashboard state | n/a | 35 |", lines)
        self.assertIn("| PVE guests with recent poll sightings at measurement end | n/a | 34/35 (last 60 s) |", lines)
        self.assertIn("| PBS API requests | 210 | 210 |", lines)
        self.assertIn("| PBS snapshot groups served/configured | n/a | 92/104 |", lines)
        self.assertIn("not snapshots Pulse retained or displayed", lines)

    def test_mock_estate_is_the_default(self):
        _, result, text = self.run_summary(samples(130000, 25, 0), samples(130000, 25, 0))
        self.assertEqual(result["estate"], "mock")
        self.assertIn("(mock estate)", text)
        self.assertNotIn("Agent traffic", text)

    def test_refuses_rss_verdict_without_a_covered_window_on_both_builds(self):
        full = samples(131072, 25, 0)
        cases = (
            ("candidate", lambda t: t <= 30, "only 0 RSS samples"),
            ("candidate", lambda t: t >= 870, "no RSS sample across 600–870 s"),
            ("candidate", lambda t: not 660 <= t <= 750, "no RSS sample across 645–765 s"),
            ("baseline", lambda t: t <= 855, "no RSS sample across 855–900 s"),
        )
        for label, keep, diagnostic in cases:
            with self.subTest(label=label, diagnostic=diagnostic):
                partial = select_samples(full, keep)
                code, result, text = self.run_summary(
                    partial if label == "baseline" else full,
                    partial if label == "candidate" else full,
                    env={"PULSE_MEMORY_GATE": "0"},
                )
                self.assertEqual(code, 1)
                self.assertEqual(result["status"], "incomplete")
                self.assertEqual(result["sampling"]["window_start_s"], 600)
                self.assertIn(diagnostic, result["sampling"][label]["error"])
                self.assertNotIn("regressed", result)
                self.assertNotIn("rss_growth_kib", result)
                self.assertIn("**Incomplete measurement:**", text)
                self.assertIn(f"{label.capitalize()}: {diagnostic}", text)
                self.assertNotIn("No steady-state RSS regression", text)

    def test_uses_one_planned_window_and_allows_one_missed_nominal_sample(self):
        baseline = samples(131072, 25, 0)
        candidate = select_samples(baseline, lambda t: t not in (600, 750, 900))
        code, result, text = self.run_summary(baseline, candidate)
        self.assertEqual(code, 0)
        self.assertEqual(result["status"], "measured")
        self.assertEqual(result["sampling"], {
            "duration_s": 900, "interval_s": 15, "window_start_s": 600, "max_gap_s": 30,
            "baseline": {"first_s": 600, "last_s": 900, "count": 21},
            "candidate": {"first_s": 615, "last_s": 885, "count": 18},
        })
        self.assertEqual(result["candidate_activity"]["cpu_pct"], 25.0)
        self.assertIn("planned 600–900 s window", text)

    def test_short_run_uses_its_declared_duration(self):
        short = samples(131072, 25, 0, end=38, interval=2)
        code, result, _ = self.run_summary(short, short, duration=40, interval=2)
        self.assertEqual(code, 0)
        self.assertEqual(result["sampling"]["window_start_s"], 26)
        self.assertEqual(result["sampling"]["candidate"], {"first_s": 26, "last_s": 38, "count": 7})

    def test_duplicate_timestamp_cannot_fill_a_sampling_gap(self):
        baseline = samples(131072, 25, 0)
        head, duplicate, rest = baseline.partition("\n675\t")
        edited = head + duplicate.replace("675", "660") + rest
        code, result, _ = self.run_summary(baseline, edited)
        self.assertEqual(code, 1)
        self.assertEqual(result["status"], "incomplete")
        self.assertIn("timestamps must increase", result["sampling"]["candidate"]["error"])

    def test_reports_cpu_and_write_rates_over_the_final_third(self):
        # 25 ticks/s at 100 Hz is a quarter of a core; 1 MiB per 64 s is 1350 MiB/day.
        code, result, text = self.run_summary(
            samples(130000, 25, 16384), samples(130000, 50, 32768)
        )
        self.assertEqual(code, 0)
        self.assertAlmostEqual(result["baseline_activity"]["cpu_pct"], 25.0)
        self.assertAlmostEqual(result["candidate_activity"]["cpu_pct"], 50.0)
        self.assertAlmostEqual(result["baseline_activity"]["write_mib_per_day"], 1350.0)
        self.assertAlmostEqual(result["candidate_activity"]["wchar_mib_per_day"], 5400.0)
        self.assertIn("| CPU | % of one core | 25.0 | 50.0 | +25.0 |", text)
        self.assertIn("Reported, not gated.", text)

    def test_cpu_and_write_growth_do_not_fail_the_run(self):
        code, result, _ = self.run_summary(samples(130000, 10, 100), samples(130000, 90, 10**7))
        self.assertEqual(code, 0)
        self.assertFalse(result["regressed"])

    def test_tiny_negative_change_prints_as_zero(self):
        self.assertEqual(summary.format_change(0.62, 0.6), "+0.0")

    def test_missing_io_accounting_reports_not_available(self):
        code, result, text = self.run_summary(
            samples(130000, 25, 0, io=False), samples(130000, 25, 0, io=False)
        )
        self.assertEqual(code, 0)
        self.assertIsNone(result["candidate_activity"]["write_mib_per_day"])
        self.assertIn("| Disk writes | MiB/day | n/a | n/a | n/a |", text)

    def test_process_peak_separate_from_unchanged_sampled_rss_and_gate(self):
        code, result, text = self.run_summary(
            samples(131072, 25, 0, rss_hwm=lambda _: 163840),
            samples(131072, 25, 0, rss_hwm=lambda t: 131072 if t == 0 else 262144),
        )
        self.assertEqual(code, 0)
        self.assertEqual(result["rss_growth_kib"], 0)
        self.assertFalse(result["regressed"])
        self.assertEqual(result["baseline_rss_high_water_kib"], 163840)
        self.assertEqual(result["candidate_rss_high_water_kib"], 262144)
        self.assertIn("No steady-state RSS regression", text)
        self.assertIn("| Process RSS high-water mark | 160.0 | 256.0 | +96.0 |", text)
        self.assertIn("does not measure cgroup usage", text)

    def test_legacy_samples_do_not_substitute_current_rss_for_high_water(self):
        code, result, text = self.run_summary(
            samples(131072, 25, 0), samples(131072, 25, 0, rss_hwm=lambda _: 163840)
        )
        self.assertEqual(code, 0)
        self.assertIsNone(result["baseline_rss_high_water_kib"])
        self.assertIn("| Process RSS high-water mark | n/a | 160.0 | n/a |", text)

    def test_missing_or_invalid_last_high_water_keeps_comparison_unknown(self):
        for invalid in ("", "0", "-1", "unavailable"):
            with self.subTest(invalid=invalid):
                code, result, text = self.run_summary(
                    samples(131072, 25, 0, rss_hwm=lambda t: 163840 if t < 900 else invalid),
                    samples(131072, 25, 0, rss_hwm=lambda _: 163840),
                )
                self.assertEqual(code, 0)
                self.assertIsNone(result["baseline_rss_high_water_kib"])
                self.assertIn("| Process RSS high-water mark | n/a | 160.0 | n/a |", text)

    def test_memory_regression_still_fails(self):
        code, result, _ = self.run_summary(samples(130000, 25, 0), samples(170000, 25, 0))
        self.assertEqual(code, 1)
        self.assertTrue(result["regressed"])


if __name__ == "__main__":
    unittest.main()
