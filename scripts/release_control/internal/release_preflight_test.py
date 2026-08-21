#!/usr/bin/env python3
from __future__ import annotations

import os
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts"))

from shard_go_tests import _test_regex, build_plan, write_plan


class ReleasePreflightTest(unittest.TestCase):
    def run_script(
        self, *args: str, env: dict[str, str] | None = None
    ) -> subprocess.CompletedProcess[str]:
        command_env = os.environ.copy()
        if env:
            command_env.update(env)
        return subprocess.run(
            [str(ROOT / "scripts/run-release-preflight.sh"), *args],
            cwd=ROOT,
            env=command_env,
            check=False,
            capture_output=True,
            text=True,
        )

    def test_plan_resolves_exact_sha_and_wsl_transport(self) -> None:
        sha = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=ROOT, text=True
        ).strip()
        result = self.run_script(
            "--profile",
            "rehearsal",
            "--host",
            "test-worker",
            "--wsl-distro",
            "Ubuntu",
            "--plan",
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(f"SHA:     {sha}", result.stdout)
        self.assertIn("Runtime: WSL Ubuntu", result.stdout)

    def test_missing_optional_worker_is_a_non_gate(self) -> None:
        configured_host = subprocess.run(
            ["git", "config", "--get", "pulse.releasePreflightHost"],
            cwd=ROOT,
            check=False,
            capture_output=True,
            text=True,
        )
        if configured_host.returncode == 0:
            self.skipTest("repository has a local release-preflight host configured")
        result = self.run_script(
            "--profile",
            "release",
            "--if-configured",
            env={
                "PULSE_RELEASE_PREFLIGHT_HOST": "",
                "PULSE_RELEASE_PREFLIGHT_WSL_DISTRO": "",
                "GIT_CONFIG_NOSYSTEM": "1",
            },
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("canonical hosted checks", result.stdout)

    def test_worker_rejects_non_exact_sha_before_touching_worker_state(self) -> None:
        result = subprocess.run(
            [str(ROOT / "scripts/release-preflight-worker.sh"), "HEAD", "rehearsal"],
            cwd=ROOT,
            check=False,
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("40-character Git commit id", result.stderr)

    def test_dispatch_helpers_select_the_matching_profiles(self) -> None:
        dry_run = (ROOT / "scripts/trigger-release-dry-run.sh").read_text()
        release = (ROOT / "scripts/trigger-release.sh").read_text()
        workflow = (ROOT / ".github/workflows/release-dry-run.yml").read_text()
        self.assertIn("--profile rehearsal", dry_run)
        self.assertIn("--profile release", release)
        self.assertIn("--if-configured", dry_run)
        self.assertIn("--if-configured", release)
        self.assertIn('PULSE_E2E_DIAGNOSTIC: "1"', workflow)

    def test_worker_has_no_publication_or_signing_authority(self) -> None:
        worker = (ROOT / "scripts/release-preflight-worker.sh").read_text()
        runner = (ROOT / "scripts/run-release-preflight.sh").read_text()
        self.assertIn("unset GH_TOKEN GITHUB_TOKEN", worker)
        self.assertIn("export GITHUB_ACTIONS=true", worker)
        self.assertIn("export CI=true", worker)
        self.assertIn('export GOTMPDIR="$GO_TMP_DIR"', worker)
        self.assertIn('rm -rf "$GO_TMP_DIR"', worker)
        self.assertIn('sub(/^toolchain /, "")', worker)
        self.assertNotIn("docker push", worker)
        self.assertNotIn("gh release", worker)
        self.assertIn("mcr.microsoft.com/playwright:v${PLAYWRIGHT_VERSION}-noble", worker)
        self.assertIn(
            'git show "${SOURCE_SHA}:scripts/release-preflight-worker.sh"', runner
        )
        self.assertIn("is not reachable from a fetched origin branch", runner)

    def test_api_shard_plan_is_deterministic_complete_and_disjoint(self) -> None:
        test_names = [
            f"TestReleaseCase{index:04d}"
            for index in [8, 1, 14, 3, 16, 0, 7, 11, 2, 15, 5, 13, 4, 10, 6, 12, 9]
        ]
        plan = build_plan(test_names, shard_count=2, batch_size=4)
        reordered_plan = build_plan(list(reversed(test_names)), shard_count=2, batch_size=4)
        self.assertNotEqual(plan, reordered_plan)

        assigned = [
            name
            for shard in plan["shards"]
            for batch in shard["batches"]
            for name in batch
        ]
        self.assertEqual(assigned, test_names)
        self.assertEqual(len(assigned), len(set(assigned)))
        self.assertEqual(
            [name for batch in plan["shards"][0]["batches"] for name in batch],
            test_names[:9],
        )
        self.assertEqual(
            [name for batch in plan["shards"][1]["batches"] for name in batch],
            test_names[9:],
        )

        with tempfile.TemporaryDirectory() as directory:
            manifest_path = write_plan(plan, pathlib.Path(directory))
            manifest = manifest_path.read_text()
            self.assertIn('"test_count": 17', manifest)
            self.assertTrue((pathlib.Path(directory) / "shard-01-batch-01.regex").is_file())
            self.assertTrue((pathlib.Path(directory) / "shard-02-batch-01.regex").is_file())

    def test_api_shard_plan_rejects_invalid_or_duplicate_names(self) -> None:
        with self.assertRaisesRegex(ValueError, "invalid top-level"):
            build_plan(["TestGood", "BenchmarkNotATest"], 2, 10)
        with self.assertRaisesRegex(ValueError, "duplicate top-level"):
            build_plan(["TestDuplicate", "TestDuplicate"], 2, 10)
        with self.assertRaisesRegex(ValueError, "cannot exceed"):
            build_plan(["TestOnly"], 2, 10)
        with self.assertRaisesRegex(ValueError, "safe per-argument ceiling"):
            build_plan(["TestOnly"], 1, 10, max_regex_bytes=120_001)

    def test_api_shard_plan_bounds_one_shard_regex_argument_bytes(self) -> None:
        test_names = [
            f"Test{index:04d}{index * 7919:08d}{index * 104729:010d}{'X' * 64}"
            for index in range(3736)
        ]
        max_regex_bytes = 64 * 1024
        plan = build_plan(
            test_names,
            shard_count=1,
            batch_size=10000,
            max_regex_bytes=max_regex_bytes,
        )
        batches = plan["shards"][0]["batches"]
        self.assertGreater(len(batches), 1)
        self.assertEqual(
            [name for batch in batches for name in batch],
            test_names,
        )

        with tempfile.TemporaryDirectory() as directory:
            write_plan(plan, pathlib.Path(directory))
            regex_files = sorted(pathlib.Path(directory).glob("*.regex"))
            self.assertEqual(len(regex_files), len(batches))
            for regex_file in regex_files:
                self.assertLessEqual(
                    len(regex_file.read_text().strip().encode()),
                    max_regex_bytes,
                )

        with self.assertRaisesRegex(ValueError, "exceeds max regex bytes"):
            build_plan(
                ["TestNameThatCannotFit"],
                shard_count=1,
                batch_size=1,
                max_regex_bytes=8,
            )

    def test_api_shard_regex_compresses_shared_names_without_changing_membership(
        self,
    ) -> None:
        test_names = [
            "TestHandleCharts_Success",
            "TestHandleCharts_MethodNotAllowed",
            "TestHandleStorageCharts_Success",
            "TestHandleStorageCharts_MethodNotAllowed",
            "TestRouterPublicPathsInventory",
        ]
        regex = _test_regex(test_names)
        naive = "^(?:" + "|".join(test_names) + ")$"
        self.assertLess(len(regex.encode()), len(naive.encode()))

        compiled = re.compile(regex)
        for test_name in test_names:
            self.assertIsNotNone(compiled.fullmatch(test_name))
        for excluded in [
            "TestHandleCharts",
            "TestHandleCharts_Failure",
            "TestRouterPublicPathsInventoryExtra",
        ]:
            self.assertIsNone(compiled.fullmatch(excluded))

    def test_release_workflow_uses_independent_pve_bundle_and_backend_lanes(self) -> None:
        workflow = (ROOT / ".github/workflows/create-release.yml").read_text()
        backend = (ROOT / "scripts/run-release-backend-tests.sh").read_text()
        self.assertIn("frontend_bundle:", workflow)
        self.assertIn("runs-on: [self-hosted, Linux, X64, pulse-pve-build]", workflow)
        self.assertIn("runs-on: [self-hosted, Linux, X64, pulse-pve-tests]", workflow)
        self.assertIn("./scripts/run-release-backend-tests.sh", workflow)
        backend_job = workflow[
            workflow.index("  backend_tests:") : workflow.index("  integration_tests:")
        ]
        self.assertNotIn("      - frontend_checks\n    if:", backend_job)
        self.assertIn("go test -c -race", backend)
        self.assertIn("python3 scripts/shard_go_tests.py", backend)
        self.assertIn('--max-regex-bytes "$MAX_REGEX_BYTES"', backend)
        self.assertIn(
            'MEMORY_WAIT_SECONDS="${PULSE_BACKEND_TEST_MEMORY_WAIT_SECONDS:-120}"',
            backend,
        )
        self.assertIn("two-shard backend admission requires", backend)
        self.assertIn("export GITHUB_ACTIONS=true", backend)
        self.assertIn("export CI=true", backend)


if __name__ == "__main__":
    unittest.main()
