#!/usr/bin/env python3
"""Check the release memory summary's CPU and disk-write rates."""

import importlib.util
import io
import json
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


def samples(rss_kib, ticks_per_s, write_bytes_per_s, io=True):
    rows = [HEADER]
    for elapsed in range(0, 901, 15):
        write = str(write_bytes_per_s * elapsed) if io else ""
        wchar = str(2 * write_bytes_per_s * elapsed) if io else ""
        rows.append(
            f"{elapsed}\t{rss_kib}\t50000\t10000\t20000\t30000\t{ticks_per_s * elapsed}\t{write}\t{wchar}\n"
        )
    return "".join(rows)


class ReleaseMemorySummaryTests(unittest.TestCase):
    def run_summary(self, baseline, candidate):
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp)
            (out / "samples-baseline.tsv").write_text(baseline)
            (out / "samples-candidate.tsv").write_text(candidate)
            (out / "clk_tck").write_text("100\n")
            with redirect_stdout(io.StringIO()):
                code = summary.main(["summarize", str(out)])
            return code, json.loads((out / "result.json").read_text()), (out / "summary.md").read_text()

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

    def test_memory_regression_still_fails(self):
        code, result, _ = self.run_summary(samples(130000, 25, 0), samples(170000, 25, 0))
        self.assertEqual(code, 1)
        self.assertTrue(result["regressed"])


if __name__ == "__main__":
    unittest.main()
