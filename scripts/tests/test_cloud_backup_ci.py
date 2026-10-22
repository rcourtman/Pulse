#!/usr/bin/env python3
"""Native-mode admission and required CI wiring, not real-client proof."""

import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
TEST = ROOT / "scripts/tests/test_cloud_backup.py"
spec = importlib.util.spec_from_file_location("backup_fixture", TEST)
backup = importlib.util.module_from_spec(spec)
spec.loader.exec_module(backup)


class NativeBackupAdmissionTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.paths = {}
        # Identity-only fixtures: subprocess and suite execution are doubled.
        # These do not implement or claim age/rclone semantics.
        for name in ("age", "age-keygen", "rclone"):
            tool = self.root / name
            tool.write_bytes(b"synthetic executable identity fixture\n")
            tool.chmod(0o700)
            self.paths[name] = str(tool)
        self.expected = unittest.TestLoader().loadTestsFromTestCase(backup.CloudBackupTest).countTestCases()
        self.result = SimpleNamespace(testsRun=self.expected, failures=[], errors=[], skipped=[],
                                      wasSuccessful=lambda: True)

    def run_native(self, paths=None, version=None):
        paths = self.paths if paths is None else paths
        stdout, stderr = io.StringIO(), io.StringIO()
        with patch.multiple(backup, REAL_AGE=paths["age"], REAL_AGE_KEYGEN=paths["age-keygen"],
                            REAL_RCLONE=paths["rclone"]), \
                patch.object(backup.subprocess, "run", return_value=version or SimpleNamespace(
                    returncode=0, stdout="synthetic version fixture\n", stderr="")) as command, \
                patch.object(backup.unittest, "TextTestRunner") as runner, \
                contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            runner.return_value.run.return_value = self.result
            status = backup.run_native_suite()
        return status, stdout.getvalue(), stderr.getvalue(), command, runner

    def test_each_missing_client_stops_before_versions_or_suite(self):
        for name in self.paths:
            with self.subTest(tool=name):
                status, stdout, stderr, command, runner = self.run_native({**self.paths, name: None})
                self.assertEqual(status, 2)
                self.assertIn(f"requires executable {name}", stderr)
                self.assertEqual(stdout, "")
                command.assert_not_called()
                runner.assert_not_called()

    def test_absent_nonexecutable_and_directory_inputs_stop(self):
        nonexecutable = self.root / "not-executable"
        nonexecutable.write_text("synthetic\n")
        for path in (str(self.root / "missing"), str(nonexecutable), str(self.root)):
            with self.subTest(path=path):
                status, _, _, command, runner = self.run_native({**self.paths, "rclone": path})
                self.assertEqual(status, 2)
                command.assert_not_called()
                runner.assert_not_called()

    def test_failed_version_stops_without_running_suite(self):
        status, _, stderr, _, runner = self.run_native(version=SimpleNamespace(
            returncode=7, stdout="not identity evidence", stderr=""))
        self.assertEqual(status, 2)
        self.assertIn("cannot identify age", stderr)
        runner.assert_not_called()

    def test_empty_or_overbound_version_is_not_identity(self):
        for output in ("", "x" * 4097):
            with self.subTest(length=len(output)):
                status, _, _, _, runner = self.run_native(version=SimpleNamespace(
                    returncode=0, stdout=output, stderr=""))
                self.assertEqual(status, 2)
                runner.assert_not_called()

    def test_version_on_stderr_is_recorded(self):
        status, stdout, _, _, _ = self.run_native(version=SimpleNamespace(
            returncode=0, stdout="", stderr="synthetic stderr version\n"))
        self.assertEqual(status, 0)
        self.assertEqual(json.loads(stdout.splitlines()[0])["version"], "synthetic stderr version")

    def test_unavailable_or_timed_out_version_stops_without_suite(self):
        for error in (OSError("synthetic"), subprocess.TimeoutExpired("synthetic", 10)):
            with self.subTest(error=type(error).__name__), \
                    patch.object(backup.subprocess, "run", side_effect=error), \
                    patch.multiple(backup, REAL_AGE=self.paths["age"],
                                   REAL_AGE_KEYGEN=self.paths["age-keygen"],
                                   REAL_RCLONE=self.paths["rclone"]), \
                    patch.object(backup.unittest, "TextTestRunner") as runner, \
                    contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(backup.run_native_suite(), 2)
                runner.assert_not_called()

    def test_complete_suite_binds_tool_versions_hashes_and_source(self):
        status, stdout, stderr, command, runner = self.run_native()
        self.assertEqual((status, stderr), (0, ""))
        records = [json.loads(line) for line in stdout.splitlines()]
        self.assertEqual([row["native_tool"] for row in records[:3]], list(self.paths))
        for row in records[:3]:
            tool = Path(self.paths[row["native_tool"]])
            self.assertEqual(row["sha256"], hashlib.sha256(tool.read_bytes()).hexdigest())
            self.assertEqual(row["version"], "synthetic version fixture")
        for row, path in zip(records[3:5], (backup.SCRIPT, TEST), strict=True):
            self.assertEqual(row["source_file"], path.name)
            self.assertEqual(row["sha256"], hashlib.sha256(path.read_bytes()).hexdigest())
        self.assertEqual([call.args[0] for call in command.call_args_list],
                         [[self.paths["age"], "--version"], [self.paths["age-keygen"], "--version"],
                          [self.paths["rclone"], "version"]])
        suite = runner.return_value.run.call_args.args[0]
        self.assertEqual(suite.countTestCases(), self.expected)
        self.assertEqual(records[-1]["native_backup_summary"],
                         {"expected": self.expected, "ran": self.expected, "errors": 0,
                          "failures": 0, "skipped": 0, "complete": True})

    def test_native_cases_cannot_disappear_from_full_suite(self):
        for missing in ("test_real_age_recovers_archives_and_private_metadata",
                        "test_real_rclone_local_backend_preserves_unowned_objects",
                        "test_real_rclone_dry_run_success_is_not_an_upload",
                        "test_real_rclone_skipped_existing_destination_cannot_authorize_retention"):
            names = unittest.TestLoader().getTestCaseNames(backup.CloudBackupTest)
            with self.subTest(case=missing), patch.object(backup.unittest.TestLoader, "getTestCaseNames",
                                                        return_value=[name for name in names if name != missing]):
                status, _, stderr, _, runner = self.run_native()
                self.assertEqual(status, 2)
                self.assertIn("Native backup cases are missing", stderr)
                runner.assert_not_called()

    def test_successful_but_skipped_suite_is_not_native_proof(self):
        self.result.skipped = [("synthetic case", "client absent")]
        status, stdout, _, _, _ = self.run_native()
        self.assertEqual(status, 1)
        self.assertFalse(json.loads(stdout.splitlines()[-1])["native_backup_summary"]["complete"])

    def test_incomplete_execution_is_not_native_proof(self):
        for ran in (0, self.expected - 1):
            with self.subTest(ran=ran):
                self.result.testsRun = ran
                self.assertEqual(self.run_native()[0], 1)

    def test_failure_or_error_is_not_native_proof(self):
        self.result.wasSuccessful = lambda: False
        for attribute in ("failures", "errors"):
            with self.subTest(attribute=attribute):
                setattr(self.result, attribute, [("synthetic case", "synthetic failure")])
                self.assertEqual(self.run_native()[0], 1)
                setattr(self.result, attribute, [])

    def test_actual_cli_missing_clients_is_failure_not_skip(self):
        env = dict(os.environ)
        for name in ("AGE", "AGE_KEYGEN", "RCLONE"):
            env["PULSE_CLOUD_BACKUP_TEST_REAL_" + name] = str(self.root / "missing-client")
        result = subprocess.run([sys.executable, str(TEST), "--require-native"],
                                env=env, capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("no tests started", result.stderr)
        self.assertNotIn("Ran ", result.stderr)

    def test_required_cli_does_not_allow_filtered_selection(self):
        result = subprocess.run([sys.executable, str(TEST), "--require-native", "CloudBackupTest.test_no_sources"],
                                capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unrecognized arguments: --require-native", result.stderr)


class NativeBackupWorkflowTest(unittest.TestCase):
    def setUp(self):
        workflow = (ROOT / ".github/workflows/build-and-test.yml").read_text()
        self.job = workflow.split("\n  scripts-and-build:\n", 1)[1].split("\n  chart-latency-slo:\n", 1)[0]

    def test_required_check_executes_native_proof_after_all_shards(self):
        self.assertIn("name: Script smoke tests & backend build", self.job)
        self.assertIn("needs: [changes, script-smoke]", self.job)
        self.assertIn("if: always() && needs.changes.outputs.code == 'true'", self.job)
        self.assertNotIn("continue-on-error:", self.job)
        self.assertNotIn("|| true", self.job)
        self.assertIn('if [ "$SMOKE_RESULT" != success ]; then', self.job)
        self.assertIn("python3 scripts/tests/test_cloud_backup.py --require-native", self.job)
        self.assertLess(self.job.index("Require every script smoke shard"),
                        self.job.index("Require native backup encryption and retention proof"))

    def test_clients_are_installed_once_before_proof_and_backend_build(self):
        self.assertEqual(self.job.count("apt-get install"), 1)
        self.assertIn("sudo -n apt-get update", self.job)
        self.assertIn("sudo -n apt-get install -y --no-install-recommends age rclone", self.job)
        self.assertIn("dpkg-query -W age rclone", self.job)
        self.assertNotIn("--allow-unauthenticated", self.job)
        self.assertLess(self.job.index("apt-get install"), self.job.index("--require-native"))
        self.assertLess(self.job.index("--require-native"), self.job.index("go build ./cmd/pulse"))

    def test_exact_source_and_fixed_system_clients_are_bound(self):
        self.assertIn("persist-credentials: false", self.job)
        self.assertIn("EXPECTED_SOURCE_SHA: ${{ github.sha }}", self.job)
        self.assertIn('test "$(git rev-parse HEAD)" = "$EXPECTED_SOURCE_SHA"', self.job)
        for name, path in (("AGE", "age"), ("AGE_KEYGEN", "age-keygen"), ("RCLONE", "rclone")):
            self.assertIn(f"PULSE_CLOUD_BACKUP_TEST_REAL_{name}: /usr/bin/{path}", self.job)


if __name__ == "__main__":
    unittest.main()
