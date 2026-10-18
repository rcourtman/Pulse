#!/usr/bin/env python3
"""Exercise actual log-file retention, bounded output and CI wiring offline."""

import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/test-e2e.yml"
SPEC = importlib.util.spec_from_file_location(
    "e2e_container_logs", ROOT / "scripts/collect_e2e_container_logs.py")
LOGS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(LOGS)
IDENTITY = {"source_sha": "a" * 40, "run_id": "123", "run_attempt": "2"}


def job_block(workflow, job):
    return re.split(r"(?m)^  \S", workflow.split(f"\n  {job}:\n", 1)[1], maxsplit=1)[0]


def assert_workflow_policy(test, workflow):
    for event in ("pull_request", "push"):
        block = re.split(r"(?m)^  \S", workflow.split(f"  {event}:\n", 1)[1], maxsplit=1)[0]
        test.assertIn("'scripts/collect_e2e_container_logs.py'", block)
        test.assertIn("'scripts/tests/test_e2e_container_logs.py'", block)
    selection = job_block(workflow, "tier-selection")
    test.assertLess(selection.index("python3 scripts/tests/test_e2e_container_logs.py"),
                    selection.index("npm ci"))
    for job, suffix, selector in (("e2e", "shard-${{ matrix.shard }}", ""),
                                  ("agent-registration", "agent-registration", " --registration")):
        steps = re.split(r"(?m)^      - name: ", job_block(workflow, job))[1:]
        collector = next(step for step in steps if step.startswith("Collect container logs\n"))
        upload = next(step for step in steps if step.startswith("Upload complete container diagnostics\n"))
        test.assertIn("if: always()", collector)
        test.assertIn("timeout-minutes: 3", collector)
        test.assertIn('python3 scripts/collect_e2e_container_logs.py "$RUNNER_TEMP/pulse-e2e-container-logs"' + selector,
                      collector)
        test.assertNotIn("continue-on-error", collector)
        test.assertNotIn("docker logs", collector)
        test.assertNotIn("||", collector)
        test.assertIn("if: always()", upload)
        test.assertIn("actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a", upload)
        test.assertIn("e2e-container-logs-" + suffix + "-${{ github.sha }}-${{ github.run_attempt }}", upload)
        test.assertIn("path: ${{ runner.temp }}/pulse-e2e-container-logs/", upload)
        test.assertIn("retention-days: 3", upload)
        test.assertIn("if-no-files-found: error", upload)
        test.assertLess(steps.index(collector), steps.index(upload))


class ContainerLogTest(unittest.TestCase):
    def test_large_real_subprocess_stdout_and_stderr_are_retained_not_printed(self):
        # Real child/pipe/file I/O, but no Docker or localhost listener. This
        # exceeds the observed gateway ceiling without sending it to stdout.
        size = 8_001_774
        calls = []

        def run(command, **options):
            calls.append(command)
            return subprocess.run(
                [sys.executable, "-c",
                 "import sys; sys.stdout.buffer.write(b'x'*" + str(size) + "); "
                 "sys.stderr.buffer.write(b'\\n::error::untrusted diagnostic\\n')"],
                **options,
            )

        payload = b"x" * size + b"\n::error::untrusted diagnostic\n"
        with tempfile.TemporaryDirectory() as temp:
            output = Path(temp) / "logs"
            console = io.StringIO()
            self.assertEqual(0, LOGS.collect(output, IDENTITY, registration=True,
                                            run=run, console=console))
            self.assertEqual(calls, [command for _, command in LOGS.COMMANDS[:2]])
            manifest = json.loads((output / "manifest.json").read_text())
            self.assertTrue(manifest["collection_complete"])
            self.assertEqual(manifest["source_sha"], IDENTITY["source_sha"])
            for record in manifest["records"]:
                self.assertEqual((output / record["file"]).read_bytes(), payload)
                self.assertEqual(record["bytes"], len(payload))
                self.assertEqual(record["sha256"], hashlib.sha256(payload).hexdigest())
            summaries = [json.loads(line) for line in console.getvalue().splitlines()]
            self.assertEqual(len(summaries), 2)
            for summary in summaries:
                self.assertTrue(summary["console_tail_truncated"])
                self.assertEqual(summary["console_tail_bytes"], LOGS.TAIL_BYTES)
            self.assertLess(len(console.getvalue().encode()), 12_000)
            self.assertFalse(any(line.startswith("::") for line in console.getvalue().splitlines()))

    def test_nonzero_timeout_and_spawn_failures_keep_partial_bytes_and_other_records(self):
        for failure in ("exit", "signal", "timeout", "spawn"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temp:
                calls = []

                def run(command, **options):
                    calls.append(command)
                    options["stdout"].write(b"partial diagnostic\n")
                    self.assertEqual(options["stderr"], subprocess.STDOUT)
                    self.assertEqual(options["timeout"], 30)
                    self.assertFalse(options["check"])
                    if len(calls) != 1:
                        return subprocess.CompletedProcess(command, 0)
                    if failure == "timeout":
                        raise subprocess.TimeoutExpired(command, 30)
                    if failure == "spawn":
                        raise OSError("private path must not be printed")
                    return subprocess.CompletedProcess(command, -15 if failure == "signal" else 7)

                output = Path(temp) / "logs"
                console = io.StringIO()
                self.assertEqual(1, LOGS.collect(output, IDENTITY, run=run, console=console))
                manifest = json.loads((output / "manifest.json").read_text())
                self.assertEqual(len(calls), 3)
                self.assertFalse(manifest["collection_complete"])
                first = manifest["records"][0]
                expected = {"exit": 7, "signal": -15}.get(failure)
                self.assertEqual(first["exit_code"], expected)
                self.assertFalse(first["collection_complete"])
                if failure in ("timeout", "spawn"):
                    self.assertEqual(first["failure"], failure)
                self.assertTrue(all(record["collection_complete"] for record in manifest["records"][1:]))
                self.assertNotIn("private path", console.getvalue())
                self.assertEqual((output / first["file"]).read_bytes(), b"partial diagnostic\n")

    def test_real_child_deadline_retains_partial_output_and_reaps_owned_cli_double(self):
        observed = []
        real_popen = subprocess.Popen

        def observe_popen(*args, **options):
            child = real_popen(*args, **options)
            observed.append(child)
            return child

        def run(command, **options):
            if command != LOGS.COMMANDS[0][1]:
                return subprocess.CompletedProcess(command, 0)
            # Only this owned CLI double has a short deadline. The production
            # helper's fixed 30-second deadline is checked separately above.
            return subprocess.run(
                [sys.executable, "-c", "import time; print('partial', flush=True); time.sleep(10)"],
                **{**options, "timeout": 0.2},
            )

        with tempfile.TemporaryDirectory() as temp, patch.object(subprocess, "Popen", observe_popen):
            output = Path(temp) / "logs"
            self.assertEqual(1, LOGS.collect(output, IDENTITY, registration=True,
                                            run=run, console=io.StringIO()))
            manifest = json.loads((output / "manifest.json").read_text())
            first = manifest["records"][0]
            self.assertEqual(first["failure"], "timeout")
            self.assertIsNone(first["exit_code"])
            self.assertEqual((output / first["file"]).read_bytes(), b"partial\n")
            self.assertEqual(len(observed), 1)
            self.assertIsNotNone(observed[0].returncode)
            self.assertIsNotNone(observed[0].poll())

    def test_empty_binary_and_short_logs_have_exact_byte_identity_and_bounded_safe_console(self):
        for payload in (b"", b"short\n", b"\xff\x00\r\n::warning::\x1b[31m" * 1000):
            with self.subTest(size=len(payload)), tempfile.TemporaryDirectory() as temp:
                def run(command, **options):
                    options["stdout"].write(payload)
                    return subprocess.CompletedProcess(command, 0)
                output = Path(temp) / "logs"
                console = io.StringIO()
                self.assertEqual(0, LOGS.collect(output, IDENTITY, run=run, console=console))
                for summary in map(json.loads, console.getvalue().splitlines()):
                    self.assertEqual(summary["bytes"], len(payload))
                    self.assertEqual(summary["sha256"], hashlib.sha256(payload).hexdigest())
                    self.assertEqual(summary["console_tail_truncated"], len(payload) > LOGS.TAIL_BYTES)
                self.assertLess(len(console.getvalue()), 80_000)
                self.assertEqual(len(console.getvalue().splitlines()), 3)

    def test_existing_directory_or_symlink_is_never_overwritten(self):
        with tempfile.TemporaryDirectory() as temp:
            existing = Path(temp) / "existing"
            existing.mkdir()
            (existing / "keep").write_bytes(b"unchanged")
            link = Path(temp) / "link"
            link.symlink_to(existing, target_is_directory=True)
            for output in (existing, link):
                with self.assertRaises(FileExistsError):
                    LOGS.collect(output, IDENTITY, run=lambda *args, **kwargs: self.fail("unexpected command"))
            self.assertEqual((existing / "keep").read_bytes(), b"unchanged")

    def test_source_and_run_identity_are_checked_before_collection(self):
        environment = {"GITHUB_SHA": "a" * 40, "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "2"}
        with patch.dict(os.environ, environment), patch.object(LOGS.subprocess, "run") as run:
            run.return_value.stdout = "a" * 40 + "\n"
            self.assertEqual(LOGS.checkout_identity(), IDENTITY)
            run.assert_called_once_with(["git", "rev-parse", "HEAD"], check=True,
                                        capture_output=True, text=True, timeout=30)
            run.return_value.stdout = "b" * 40 + "\n"
            with self.assertRaisesRegex(ValueError, "checkout differs"):
                LOGS.checkout_identity()
        for key, value in (("GITHUB_SHA", "not-a-commit"), ("GITHUB_RUN_ID", "0"),
                           ("GITHUB_RUN_ATTEMPT", "1\n2")):
            with patch.dict(os.environ, {**environment, key: value}), patch.object(LOGS.subprocess, "run") as run:
                with self.assertRaises(ValueError):
                    LOGS.checkout_identity()
                run.assert_not_called()

    def test_workflow_retains_full_diagnostics_on_success_and_failure(self):
        assert_workflow_policy(self, WORKFLOW.read_text())

    def test_old_dump_missing_upload_and_failure_suppression_are_rejected(self):
        workflow = WORKFLOW.read_text()
        mutants = (
            workflow.replace('python3 scripts/collect_e2e_container_logs.py "$RUNNER_TEMP/pulse-e2e-container-logs"',
                             'docker logs pulse-test-server'),
            workflow.replace("if-no-files-found: error", "if-no-files-found: ignore"),
            workflow.replace("if: always()\n        timeout-minutes: 3", "if: failure()\n        timeout-minutes: 3"),
            workflow.replace("Collect container logs\n", "Collect container logs\n        continue-on-error: true\n"),
        )
        for workflow in mutants:
            with self.subTest(mutant=workflow[:30]), self.assertRaises(AssertionError):
                assert_workflow_policy(self, workflow)


if __name__ == "__main__":
    unittest.main()
