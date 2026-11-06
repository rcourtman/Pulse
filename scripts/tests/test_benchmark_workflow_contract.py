#!/usr/bin/env python3
"""Guard complete, adequately sampled same-host benchmark comparisons."""

from copy import deepcopy
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "build-and-test.yml"


class BenchmarkWorkflowContractTest(unittest.TestCase):
    def setUp(self) -> None:
        self.workflow = yaml.safe_load(WORKFLOW.read_text(encoding="utf-8"))

    def assert_contract(self, workflow: dict) -> None:
        jobs = workflow["jobs"]
        collector = jobs["benchmarks"]
        self.assertEqual(collector["name"], "Benchmarks (shard ${{ matrix.shard }})")
        self.assertEqual(collector["needs"], "changes")
        self.assertEqual(collector["if"], "needs.changes.outputs.code == 'true'")
        self.assertEqual(collector["strategy"]["matrix"], {"shard": [0, 1, 2]})
        self.assertIs(collector["strategy"]["fail-fast"], False)
        self.assertFalse(collector.get("continue-on-error", False))
        self.assertEqual(collector["env"]["PULSE_BENCH_SHARD"], "${{ matrix.shard }}")
        steps = {step["name"]: step for step in collector["steps"]}
        base = steps["Checkout pull request base for paired comparison"]
        self.assertEqual(base["if"], "github.event_name == 'pull_request'")
        self.assertEqual(base["with"]["ref"], "${{ github.event.pull_request.base.sha }}")
        self.assertEqual(base["with"]["path"], "benchmark-base")
        pair = steps["Paired base and candidate benchmarks (pull requests)"]
        single = steps["Go benchmarks (non-PR)"]
        self.assertEqual(pair["if"], "github.event_name == 'pull_request'")
        self.assertEqual(single["if"], "github.event_name != 'pull_request'")
        for step in [pair, single]:
            self.assertEqual(step["env"]["PULSE_BENCH_SAMPLE_COUNT"], "10")
            self.assertEqual(step["run"], "bash scripts/run-ci-benchmarks.sh")
            self.assertFalse(step.get("continue-on-error", False))
        self.assertEqual(pair["env"]["PULSE_BENCH_BUDGET_SECONDS"], "1680")
        compare = steps["Compare benchmarks against baseline"]["run"]
        self.assertEqual(steps["Compare benchmarks against baseline"]["if"], "github.event_name == 'pull_request'")
        self.assertIn("benchstat bench-baseline.txt bench-results.txt", compare)
        self.assertIn("bash scripts/check-bench-regression.sh bench-comparison.txt", compare)
        self.assertFalse(steps["Compare benchmarks against baseline"].get("continue-on-error", False))
        upload = steps["Upload benchmark evidence"]
        self.assertEqual(upload["if"], "always()")
        self.assertEqual(upload["with"]["name"], "bench-results-${{ matrix.shard }}")
        self.assertEqual(set(upload["with"]["path"].split()), {
            "bench-baseline.txt", "bench-results.txt", "bench-comparison.txt", "bench-metadata.txt",
        })
        verdict = jobs["benchmarks-verdict"]
        self.assertEqual(verdict["name"], "Benchmarks")
        self.assertEqual(verdict["needs"], ["changes", "benchmarks"])
        self.assertEqual(verdict["if"], "always() && needs.changes.outputs.code == 'true'")
        self.assertFalse(verdict.get("continue-on-error", False))
        self.assertEqual(verdict["steps"][0]["env"]["SHARD_RESULT"], "${{ needs.benchmarks.result }}")
        for step in verdict["steps"]:
            self.assertNotIn("if", step)
            self.assertFalse(step.get("continue-on-error", False))

    def test_prs_keep_each_complete_pair_on_one_runner(self) -> None:
        self.assert_contract(self.workflow)
        text = WORKFLOW.read_text(encoding="utf-8")
        benchmark_jobs = text[text.index("  benchmarks:"):]
        self.assertNotIn("go-bench-baseline-", benchmark_jobs)
        self.assertNotIn("actions/cache/restore@", benchmark_jobs)

    def test_incomplete_matrix_or_weakened_result_is_rejected(self) -> None:
        mutations = [
            lambda w: w["jobs"]["benchmarks"]["strategy"]["matrix"].update(shard=[0, 1]),
            lambda w: w["jobs"]["benchmarks"]["strategy"].update({"fail-fast": True}),
            lambda w: w["jobs"]["benchmarks"].update({"continue-on-error": True}),
            lambda w: w["jobs"]["benchmarks"]["env"].update(PULSE_BENCH_SHARD="all"),
            lambda w: w["jobs"]["benchmarks-verdict"].update(needs=["changes"]),
            lambda w: w["jobs"]["benchmarks-verdict"].update({"if": "success()"}),
            lambda w: w["jobs"]["benchmarks-verdict"]["steps"][0].update({"continue-on-error": True}),
        ]
        for index, mutate in enumerate(mutations):
            with self.subTest(index=index):
                workflow = deepcopy(self.workflow)
                mutate(workflow)
                with self.assertRaises(AssertionError):
                    self.assert_contract(workflow)

    def test_actual_verdict_command_fails_for_every_incomplete_result(self) -> None:
        command = self.workflow["jobs"]["benchmarks-verdict"]["steps"][0]["run"]
        for result in ["success", "failure", "cancelled", "skipped", "", "unknown"]:
            with self.subTest(result=result):
                process = subprocess.run(
                    ["bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", command],
                    env={**os.environ, "SHARD_RESULT": result},
                    capture_output=True, text=True, check=False,
                )
                self.assertEqual(process.returncode, 0 if result == "success" else 1)
                if result != "success":
                    self.assertIn("incomplete:", process.stdout)

    def run_compare(self, metadata, results="", table=""):
        """Run the step's real command with a stand-in for benchstat's table."""
        steps = {step["name"]: step for step in self.workflow["jobs"]["benchmarks"]["steps"]}
        command = steps["Compare benchmarks against baseline"]["run"]
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            (work / "scripts").mkdir()
            shutil.copy(ROOT / "scripts" / "check-bench-regression.sh", work / "scripts")
            (work / "bench-baseline.txt").write_text(results, encoding="utf-8")
            (work / "bench-results.txt").write_text(results, encoding="utf-8")
            if metadata is not None:
                (work / "bench-metadata.txt").write_text(metadata, encoding="utf-8")
            (work / "table.txt").write_text(table, encoding="utf-8")
            benchstat = work / "benchstat"
            benchstat.write_text("#!/bin/sh\ncat table.txt\n", encoding="utf-8")
            benchstat.chmod(0o755)
            process = subprocess.run(
                ["bash", "--noprofile", "--norc", "-e", "-o", "pipefail", "-c", command],
                cwd=work,
                env={**os.environ, "PATH": f"{work}{os.pathsep}{os.environ['PATH']}"},
                capture_output=True, text=True, check=False,
            )
            comparison = (work / "bench-comparison.txt").read_text(encoding="utf-8")
        return process, comparison

    def test_a_shard_declared_empty_has_nothing_to_compare(self) -> None:
        process, comparison = self.run_compare("selection=affected\nshard=2\npackages=\n")
        self.assertEqual(process.returncode, 0, process.stdout + process.stderr)
        self.assertIn("nothing to compare", process.stdout)
        self.assertIn("nothing to compare", comparison)
        self.assertNotIn("No significant", process.stdout)

    def test_every_other_empty_comparison_is_refused(self) -> None:
        cases = {
            "packages were selected": ("selection=affected\npackages=./internal/api/\n", ""),
            "not an affected selection": ("selection=all (module files changed)\npackages=\n", ""),
            "no metadata": (None, ""),
            "samples beside the declaration": (
                "selection=affected\npackages=\n",
                "BenchmarkExample-4 1 100 ns/op\n",
            ),
        }
        for name, (metadata, results) in cases.items():
            with self.subTest(name):
                process, _ = self.run_compare(metadata, results=results)
                self.assertEqual(process.returncode, 1, process.stdout + process.stderr)
                self.assertIn("BENCHMARK EVIDENCE INSUFFICIENT", process.stdout)
                self.assertNotIn("nothing to compare", process.stdout)

    def test_a_measured_shard_is_still_judged(self) -> None:
        metadata = "selection=affected\npackages=./internal/api/\n"
        samples = "BenchmarkExample-4 1 100 ns/op\n"
        process, _ = self.run_compare(
            metadata, results=samples,
            table="Example-4  100.0n ± 1%  105.0n ± 1%  +5.00% (p=0.001 n=10)\n",
        )
        self.assertEqual(process.returncode, 0, process.stdout + process.stderr)
        self.assertIn("No significant benchmark regressions", process.stdout)
        process, _ = self.run_compare(
            metadata, results=samples,
            table="Example-4  100.0n ± 1%  111.0n ± 1%  +11.00% (p=0.001 n=10)\n",
        )
        self.assertEqual(process.returncode, 1, process.stdout + process.stderr)
        self.assertIn("BENCHMARK REGRESSION DETECTED", process.stdout)


if __name__ == "__main__":
    unittest.main()
