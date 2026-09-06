#!/usr/bin/env python3
"""Guard the paired, adequately sampled benchmark workflow contract."""

from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "build-and-test.yml"


class BenchmarkWorkflowContractTest(unittest.TestCase):
    def test_prs_compare_base_and_candidate_on_one_runner(self) -> None:
        workflow = WORKFLOW.read_text(encoding="utf-8")
        benchmark_job = workflow[workflow.index("  benchmarks:") :]

        self.assertIn("ref: ${{ github.event.pull_request.base.sha }}", benchmark_job)
        self.assertIn("path: benchmark-base", benchmark_job)
        self.assertIn('PULSE_BENCH_SAMPLE_COUNT: "10"', benchmark_job)
        self.assertIn("bash scripts/run-ci-benchmarks.sh", benchmark_job)
        self.assertNotIn("go-bench-baseline-", benchmark_job)
        self.assertNotIn("actions/cache/restore@", benchmark_job)
        self.assertIn("bench-baseline.txt", benchmark_job)
        self.assertIn("bench-comparison.txt", benchmark_job)


if __name__ == "__main__":
    unittest.main()
