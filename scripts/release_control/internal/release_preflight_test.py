#!/usr/bin/env python3
from __future__ import annotations

import os
import pathlib
import subprocess
import sys
import tempfile
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "scripts"))

from shard_go_tests import build_plan, write_plan


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
        test_names = [f"TestReleaseCase{index:04d}" for index in range(17)]
        plan = build_plan(test_names, shard_count=2, batch_size=4)
        reordered_plan = build_plan(list(reversed(test_names)), shard_count=2, batch_size=4)
        self.assertEqual(plan, reordered_plan)

        assigned = [
            name
            for shard in plan["shards"]
            for batch in shard["batches"]
            for name in batch
        ]
        self.assertCountEqual(assigned, test_names)
        self.assertEqual(len(assigned), len(set(assigned)))
        self.assertEqual(
            [name for batch in plan["shards"][0]["batches"] for name in batch],
            sorted(test_names)[:9],
        )
        self.assertEqual(
            [name for batch in plan["shards"][1]["batches"] for name in batch],
            sorted(test_names)[9:],
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

    def test_release_workflow_uses_independent_pve_bundle_and_backend_lanes(self) -> None:
        workflow = (ROOT / ".github/workflows/create-release.yml").read_text()
        backend = (ROOT / "scripts/run-release-backend-tests.sh").read_text()
        self.assertIn("frontend_bundle:", workflow)
        self.assertIn("runs-on: [self-hosted, Linux, X64, pulse-pve-build]", workflow)
        self.assertIn("runs-on: [self-hosted, Linux, X64, pulse-pve-tests]", workflow)
        self.assertIn("./scripts/run-release-backend-tests.sh", workflow)
        self.assertNotIn("      - frontend_checks\n    if:", workflow[workflow.index("  backend_tests:"):workflow.index("  docker_build:")])
        self.assertIn("go test -c -race", backend)
        self.assertIn("python3 scripts/shard_go_tests.py", backend)
        self.assertIn("export GITHUB_ACTIONS=true", backend)
        self.assertIn("export CI=true", backend)


if __name__ == "__main__":
    unittest.main()
