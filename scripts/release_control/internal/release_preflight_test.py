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

from shard_go_tests import _test_regex, allocate_cpu_plan, build_plan, write_plan


class ReleasePreflightTest(unittest.TestCase):
    def test_backend_cpu_plan_reserves_non_api_capacity_without_oversubscription(
        self,
    ) -> None:
        widths, non_api = allocate_cpu_plan([3624, 15, 16], 8)
        self.assertEqual(widths, [4, 1, 1])
        self.assertEqual(non_api, 2)
        self.assertEqual(sum(widths) + non_api, 8)

        widths, non_api = allocate_cpu_plan([10, 10], 4)
        self.assertEqual(widths, [1, 1])
        self.assertEqual(non_api, 2)
        self.assertEqual(sum(widths) + non_api, 4)

        widths, non_api = allocate_cpu_plan([10], 1)
        self.assertEqual(widths, [1])
        self.assertEqual(non_api, 0)

        for counts, vcpus in [([], 8), ([1], 0), ([1, 1], 1), ([0], 1)]:
            with self.subTest(counts=counts, vcpus=vcpus):
                with self.assertRaises(ValueError):
                    allocate_cpu_plan(counts, vcpus)

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

    def assert_release_dry_run_diagnostic_contract(
        self, workflow: str, diagnostic: str
    ) -> None:
        installed_browser = re.search(
            r"npx playwright install --with-deps ([a-z-]+)", workflow
        )
        diagnostic_step = re.search(
            r"      - name: Run integration diagnostics\n(?P<body>.*?)(?=\n      - name:)",
            workflow,
            flags=re.DOTALL,
        )
        self.assertIsNotNone(diagnostic_step)
        step_body = diagnostic_step.group("body")
        diagnostic_command = re.search(
            r"npx playwright test tests/00-diagnostic\.spec\.ts (?P<args>[^\n]+)",
            step_body,
        )
        self.assertIsNotNone(installed_browser)
        self.assertIsNotNone(diagnostic_command)
        self.assertIn('PULSE_E2E_DIAGNOSTIC: "1"', step_body)
        self.assertLess(
            step_body.index("set -o pipefail"),
            step_body.index("npx playwright test tests/00-diagnostic.spec.ts"),
        )
        self.assertIn("2>&1 | tee diagnostic-evidence/playwright-diagnostic.log", step_body)
        selected_browser = re.search(
            r"--project(?:=|\s+)([a-z-]+)", diagnostic_command.group("args")
        )
        self.assertIsNotNone(
            selected_browser,
            "release diagnostic must select the one browser installed by the job",
        )
        self.assertEqual(selected_browser.group(1), installed_browser.group(1))
        self.assertIn("--retries=0", diagnostic_command.group("args"))

        self.assertRegex(
            diagnostic,
            r"test\.describe\.configure\(\{\s*retries:\s*0\s*\}\);",
        )
        self.assertIn("test.setTimeout(120_000)", diagnostic)

        for required_diagnostic in (
            "API readiness failed:",
            "dependencies.monitor",
            "dependencies.scheduler",
            "dependencies.websocket",
            "/api/security/status",
            "hasAuthentication",
            "Rendered UI readiness failed:",
            "Pulse app shell did not render within 20s",
            "effectiveRenderedOpacity",
            "stayed transparent",
            "Pulse Setup Wizard",
            "Paste your bootstrap token",
            "diagnostic-evidence",
            "release-dry-run-rendered-readiness-chromium.png",
            "testInfo.attach(",
            "DIAGNOSTIC EVIDENCE ERROR:",
            "if (!readinessFailed)",
        ):
            self.assertIn(required_diagnostic, diagnostic)

        self.assertIn("Collect integration diagnostic runtime evidence", workflow)
        self.assertIn("Upload integration diagnostic evidence", workflow)
        self.assertIn("if-no-files-found: error", workflow)
        self.assertIn("tests/integration/test-results/", workflow)
        self.assertNotIn("expect(true).toBe(true)", diagnostic)
        self.assertNotIn("await page.waitForTimeout(3000)", diagnostic)

    def test_release_dry_run_diagnostic_contract(self) -> None:
        workflow = (ROOT / ".github/workflows/release-dry-run.yml").read_text()
        diagnostic = (
            ROOT / "tests/integration/tests/00-diagnostic.spec.ts"
        ).read_text()

        self.assert_release_dry_run_diagnostic_contract(workflow, diagnostic)

        legacy_workflow = workflow.replace(
            " --project=chromium --retries=0", "", 1
        )
        with self.assertRaises(AssertionError):
            self.assert_release_dry_run_diagnostic_contract(
                legacy_workflow, diagnostic
            )

        retrying_diagnostic = diagnostic.replace(
            "test.describe.configure({ retries: 0 });", "", 1
        )
        with self.assertRaises(AssertionError):
            self.assert_release_dry_run_diagnostic_contract(
                workflow, retrying_diagnostic
            )

        unconditional_pass = f"{diagnostic}\nexpect(true).toBe(true);\n"
        with self.assertRaises(AssertionError):
            self.assert_release_dry_run_diagnostic_contract(
                workflow, unconditional_pass
            )

        evidence_failure_ignored = diagnostic.replace(
            "if (!readinessFailed) {", "if (false) {", 1
        )
        with self.assertRaises(AssertionError):
            self.assert_release_dry_run_diagnostic_contract(
                workflow, evidence_failure_ignored
            )

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

    def test_worker_serializes_resource_intensive_test_suites(self) -> None:
        worker = (ROOT / "scripts/release-preflight-worker.sh").read_text()
        scheduling_block = re.search(
            r"# Static frontend checks.*?run_backend\n",
            worker,
            flags=re.DOTALL,
        )
        self.assertIsNotNone(scheduling_block)
        block = scheduling_block.group(0)

        self.assertIn("run_frontend_static_quality &", block)
        self.assertIn("run_integration_prep &", block)
        self.assertNotIn("run_frontend_tests &", block)
        self.assertNotIn("run_backend &", block)
        self.assertLess(block.index("done\n"), block.index("run_frontend_tests\n"))
        self.assertLess(block.index("run_frontend_tests\n"), block.index("run_backend\n"))

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
        with self.assertRaisesRegex(ValueError, "one value per shard"):
            build_plan(["TestOne", "TestTwo"], 2, 10, shard_weights=[1])
        with self.assertRaisesRegex(ValueError, "positive integers"):
            build_plan(["TestOne", "TestTwo"], 2, 10, shard_weights=[1, 0])
        with self.assertRaisesRegex(ValueError, "one fewer value"):
            build_plan(
                ["TestOne", "TestTwo"],
                2,
                10,
                shard_boundaries=["TestOne", "TestTwo"],
            )
        with self.assertRaisesRegex(ValueError, "unknown shard boundary"):
            build_plan(
                ["TestOne", "TestTwo"],
                2,
                10,
                shard_boundaries=["TestMissing"],
            )
        with self.assertRaisesRegex(ValueError, "mutually exclusive"):
            build_plan(
                ["TestOne", "TestTwo"],
                2,
                10,
                shard_weights=[1, 1],
                shard_boundaries=["TestOne"],
            )

    def test_api_shard_plan_supports_measured_contiguous_weights(self) -> None:
        test_names = [f"TestReleaseCase{index:04d}" for index in range(20)]
        plan = build_plan(
            test_names,
            shard_count=3,
            batch_size=100,
            shard_weights=[6, 1, 1],
        )

        shards = [
            [name for batch in shard["batches"] for name in batch]
            for shard in plan["shards"]
        ]
        self.assertEqual([len(shard) for shard in shards], [15, 3, 2])
        self.assertEqual([name for shard in shards for name in shard], test_names)
        self.assertEqual(plan["shard_weights"], [6, 1, 1])

    def test_api_shard_plan_supports_named_contiguous_boundaries(self) -> None:
        test_names = [f"TestReleaseCase{index:04d}" for index in range(8)]
        plan = build_plan(
            test_names,
            shard_count=3,
            batch_size=100,
            shard_boundaries=[test_names[3], test_names[5]],
        )

        shards = [
            [name for batch in shard["batches"] for name in batch]
            for shard in plan["shards"]
        ]
        self.assertEqual([len(shard) for shard in shards], [4, 2, 2])
        self.assertEqual([name for shard in shards for name in shard], test_names)
        self.assertEqual(
            plan["shard_boundaries"], [test_names[3], test_names[5]]
        )

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
        bundle_job = workflow[
            workflow.index("  frontend_bundle:") : workflow.index("  frontend_checks:")
        ]
        self.assertIn('"pulse-pve-build"', bundle_job)
        self.assertIn("./scripts/run-release-backend-tests.sh", workflow)
        backend_job = workflow[
            workflow.index("  backend_tests:") : workflow.index("  integration_tests:")
        ]
        self.assertIn('"pulse-pve-tests"', backend_job)
        self.assertNotIn("      - frontend_checks\n    if:", backend_job)
        self.assertIn("go test -c -race", backend)
        self.assertIn("python3 scripts/shard_go_tests.py", backend)
        self.assertIn('--max-regex-bytes "$MAX_REGEX_BYTES"', backend)
        self.assertIn(
            'MEMORY_WAIT_SECONDS="${PULSE_BACKEND_TEST_MEMORY_WAIT_SECONDS:-120}"',
            backend,
        )
        self.assertIn("shard_admission_required_kib", backend)
        self.assertIn("Degrading to $cpu_shards API shard(s)", backend)
        self.assertIn(
            'API_SHARD_TIMEOUT="${PULSE_BACKEND_API_SHARD_TIMEOUT:-45m}"',
            backend,
        )
        self.assertIn("RESERVED_OTHER_PACKAGE_PROCS", backend)
        self.assertIn(
            'GOMAXPROCS=1 PULSE_DATA_DIR="$RUN_ROOT/data/other"', backend
        )
        self.assertIn(
            'go test -race -p "$OTHER_PACKAGE_PROCS" -timeout 30m', backend
        )
        self.assertIn('-test.timeout "$API_SHARD_TIMEOUT"', backend)
        self.assertIn(
            "TestWebSocketOriginAllowsTrustedForwardedHostedOriginIPv6Loopback,"
            "TestServerInfoEndpointMethodNotAllowed",
            backend,
        )
        self.assertIn("Completed internal/api shard", backend)
        self.assertIn("export GITHUB_ACTIONS=true", backend)
        self.assertIn("export CI=true", backend)


if __name__ == "__main__":
    unittest.main()
