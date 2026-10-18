#!/usr/bin/env python3
"""Exercise the production scan command against real isolated Git histories.

The detector double inspects the exact Git patch stream, not secret heuristics.
Normal CI still owns the pinned gitleaks detector's actual verdict.
"""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/scan-ci-secrets.sh"
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
CANARY = "DELIVERED_HISTORY_FIXTURE_MARKER"

DETECTOR = r'''#!/usr/bin/env python3
import json, os, pathlib, shlex, subprocess, sys
args = sys.argv[1:]
opts = next(arg.split("=", 1)[1] for arg in args if arg.startswith("--log-opts="))
result = subprocess.run(["git", "log", "-p", "-U0", *shlex.split(opts)],
                        capture_output=True, text=True)
pathlib.Path(os.environ["DETECTOR_RECORD"]).write_text(json.dumps({
    "args": args, "log_opts": opts, "git_exit": result.returncode,
    "marker_in_patch": "DELIVERED_HISTORY_FIXTURE_MARKER" in result.stdout,
}))
sys.exit(result.returncode or int(os.environ.get("DETECTOR_EXIT", "0")))
'''


class DeliveredHistoryScanTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.bin = self.root / "bin"
        self.bin.mkdir()
        detector = self.bin / "gitleaks"
        detector.write_text(DETECTOR)
        detector.chmod(0o755)
        self.record = self.root / "detector.json"
        self.env = {**os.environ, "PATH": f"{self.bin}:{os.environ['PATH']}",
                    "DETECTOR_RECORD": str(self.record)}
        self.git("init", "-q", "--initial-branch=main")
        self.write("fixture.txt", "original\n")
        self.base = self.commit("Base fixture")

    def git(self, *args):
        return subprocess.check_output([
            "git", "-c", "user.name=Secret scan fixture",
            "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=/dev/null",
            *args,
        ], cwd=self.repo, stderr=subprocess.PIPE, text=True).strip()

    def write(self, name, value):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(value)

    def commit(self, message):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", message)
        return self.git("rev-parse", "HEAD")

    def scan(self, base=None, head=None, env=None):
        if self.record.exists():
            self.record.unlink()
        return subprocess.run(["bash", str(SCRIPT), head or self.git("rev-parse", "HEAD"),
                               self.base if base is None else base],
                              cwd=self.repo, env={**self.env, **(env or {})},
                              capture_output=True, text=True, timeout=10)

    def admitted(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        record = json.loads(self.record.read_text())
        self.assertEqual(record["git_exit"], 0)
        return record

    def test_entire_multicommit_push_includes_value_removed_before_head(self):
        self.write("fixture.txt", CANARY + "\n")
        self.commit("Introduced marker")
        self.write("fixture.txt", "removed before publication\n")
        head = self.commit("Removed marker")
        self.write("receipt.json", "{}\n")
        final = self.commit("Later evidence commit")
        self.assertNotIn(CANARY, self.git("log", "-p", "-U0", "-1"))
        record = self.admitted(self.scan())
        self.assertTrue(record["marker_in_patch"])
        self.assertIn(f"{self.base}..{final}", record["log_opts"])
        self.assertNotIn(head + "..", record["log_opts"])

    def test_merged_side_commits_are_scanned_even_when_merge_patch_is_empty(self):
        self.git("checkout", "-q", "-b", "proposal")
        self.write("fixture.txt", CANARY + "\n")
        self.commit("Introduced on proposal")
        self.write("fixture.txt", "original\n")
        self.commit("Removed on proposal")
        self.git("checkout", "-q", "main")
        self.git("merge", "-q", "--no-ff", "-m", "Integrated proposal", "proposal")
        old_stream = self.git("log", "-p", "-U0", "-1")
        self.assertNotIn("diff --git", old_stream)
        record = self.admitted(self.scan())
        self.assertTrue(record["marker_in_patch"])
        self.assertNotIn("--first-parent", record["log_opts"])

    def test_merge_only_resolution_is_part_of_the_actual_patch_stream(self):
        self.git("checkout", "-q", "-b", "proposal")
        self.write("feature.txt", "proposal\n")
        self.commit("Proposal")
        self.git("checkout", "-q", "main")
        self.write("main.txt", "main\n")
        base = self.commit("Advanced main")
        self.git("merge", "-q", "--no-ff", "--no-commit", "proposal")
        self.write("resolution.txt", CANARY + "\n")
        head = self.commit("Integrated with a merge-only change")
        self.assertNotIn(CANARY, self.git("log", "-p", "-U0", "-1"))
        record = self.admitted(self.scan(base=base))
        self.assertTrue(record["marker_in_patch"])
        self.assertIn("--diff-merges=first-parent", record["log_opts"])
        self.assertIn(f"{base}..{head}", record["log_opts"])

    def test_full_history_fallback_for_absent_zero_missing_malformed_or_equal_base(self):
        self.write("fixture.txt", CANARY + "\n")
        marker = self.commit("Historic marker")
        self.write("fixture.txt", "clean final tree\n")
        head = self.commit("Removed marker")
        for base in ("", "0" * 40, "f" * 40, "HEAD^", head, "--all; unexpected"):
            with self.subTest(base=base):
                result = self.scan(base=base)
                record = self.admitted(result)
                self.assertTrue(record["marker_in_patch"])
                self.assertIn("complete checkout history", result.stdout)
                self.assertIn(f" {head} --", record["log_opts"])
                self.assertNotIn("..", record["log_opts"])
                self.assertNotIn(marker + "..", record["log_opts"])

    def test_nonancestor_base_falls_back_to_complete_checkout_history(self):
        self.git("checkout", "-q", "-b", "unrelated")
        self.write("other.txt", "other\n")
        other = self.commit("Other branch")
        self.git("checkout", "-q", "main")
        self.write("fixture.txt", CANARY + "\n")
        self.commit("Main marker")
        result = self.scan(base=other)
        record = self.admitted(result)
        self.assertTrue(record["marker_in_patch"])
        self.assertNotIn("..", record["log_opts"])
        self.assertIn("complete checkout history", result.stdout)

    def test_shallow_history_is_a_failure_not_a_narrow_scan(self):
        self.write("new.txt", "new\n")
        self.commit("Second commit")
        shallow = self.root / "shallow"
        self.git("clone", "-q", "--depth", "1", self.repo.as_uri(), str(shallow))
        self.repo = shallow
        result = self.scan()
        self.assertEqual(result.returncode, 1)
        self.assertIn("complete checkout history", result.stderr)
        self.assertFalse(self.record.exists())

    def test_invalid_or_mismatched_expected_head_never_invokes_detector(self):
        for head in ("f" * 40, "HEAD", "0" * 40, "--all; unexpected"):
            with self.subTest(head=head):
                result = self.scan(head=head)
                self.assertEqual(result.returncode, 1)
                self.assertFalse(self.record.exists())

    def test_detector_finding_and_operational_failure_exits_are_preserved(self):
        self.write("new.txt", "new\n")
        self.commit("Second commit")
        for code in (2, 42):
            with self.subTest(exit=code):
                result = self.scan(env={"DETECTOR_EXIT": str(code)})
                self.assertEqual(result.returncode, code)
                self.assertTrue(self.record.exists())

    def test_git_ancestry_error_cannot_be_treated_as_a_divergent_base(self):
        self.write("new.txt", "new\n")
        self.commit("Second commit")
        real_git = shutil.which("git")
        wrapper = self.bin / "git"
        wrapper.write_text(f'#!/bin/bash\nif [[ "$1" == merge-base ]]; then exit 3; fi\nexec "{real_git}" "$@"\n')
        wrapper.chmod(0o755)
        result = self.scan()
        self.assertEqual(result.returncode, 3)
        self.assertIn("checkout ancestry", result.stderr)
        self.assertFalse(self.record.exists())

    def test_actual_detector_command_preserves_source_redaction_and_fatal_verdict(self):
        self.write("new.txt", "new\n")
        self.commit("Second commit")
        record = self.admitted(self.scan())
        args = record["args"]
        self.assertEqual(args[:3], ["detect", "--source", "."])
        self.assertEqual(args[4:], ["--no-banner", "--redact", "--exit-code", "2",
                                    "--report-format", "sarif", "--report-path", "results.sarif"])
        for flag in ("--full-history", "--no-ext-diff", "--no-textconv", "--no-renames"):
            self.assertIn(flag, record["log_opts"])
        for forbidden in ("-1", "--all", "--first-parent", "--since", "--author"):
            self.assertNotIn(forbidden, record["log_opts"].split())


class SecretScanWorkflowTest(unittest.TestCase):
    def test_unconditional_pinned_job_uses_complete_history_and_exact_event_sources(self):
        workflow = WORKFLOW.read_text()
        match = re.search(r"^  secret-scan:\n(.*?)(?=^  [\w-]+:)", workflow, re.M | re.S)
        self.assertIsNotNone(match)
        job = match[1]
        self.assertNotIn("if:", job)
        self.assertNotIn("continue-on-error:", job)
        self.assertIn('GITLEAKS_VERSION: "8.30.1"', job)
        self.assertIn('GITLEAKS_LINUX_X64_SHA256: "551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb"', job)
        self.assertIn('"${GITLEAKS_LINUX_X64_SHA256}  /tmp/${archive}" | sha256sum -c -', job)
        self.assertIn("persist-credentials: false", job)
        self.assertIn("fetch-depth: 0", job)
        self.assertIn("EXPECTED_HEAD: ${{ github.sha }}", job)
        self.assertIn("DIFF_BASE: ${{ github.event_name == 'pull_request' && github.event.pull_request.base.sha || github.event.before }}", job)
        self.assertIn('bash scripts/scan-ci-secrets.sh "$EXPECTED_HEAD" "$DIFF_BASE"', job)
        self.assertIn("python3 scripts/tests/test_ci_secret_scan.py", job)
        self.assertNotIn("--log-opts=-1", job)


if __name__ == "__main__":
    unittest.main()
