#!/usr/bin/env python3
"""Exercise the production scan command against real isolated Git histories.

The detector double inspects the exact Git patch stream, not secret heuristics.
Normal CI still owns the pinned gitleaks detector's actual verdict.
"""

import hashlib
import http.server
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import textwrap
import threading
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


def install_commands(workflow):
    step = workflow.split("      - name: Install gitleaks\n", 1)[1].split(
        "\n      - name:", 1)[0]
    return textwrap.dedent(step.split("        run: |\n", 1)[1])


class SecretScanBootstrapTest(unittest.TestCase):
    """Real curl/checksum, inert extraction/install, and no external requests.

    Only the fixture transport and /tmp destination are remapped by the curl
    shim. Retry policy is executed by curl itself, not simulated by the shim.
    """

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-secret-bootstrap-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.payload = b"synthetic pinned detector archive\n"
        self.statuses = [200]
        self.requests = []
        self.record = self.root / "commands.jsonl"
        self.download = self.root / "archive.tar.gz"
        fixture = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                fixture.requests.append(self.path)
                status = fixture.statuses[min(len(fixture.requests) - 1,
                                              len(fixture.statuses) - 1)]
                body = fixture.payload if status == 200 else b"unavailable fixture\n"
                self.send_response(status)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        self.server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_server)
        real_curl = shutil.which("curl")
        if not real_curl:
            raise RuntimeError("curl is required for the bootstrap transport tests")
        self.env = {
            **os.environ, "PATH": f"{self.bin}:{os.environ['PATH']}",
            "HOME": str(self.root), "FIXTURE": str(self.root),
            "REAL_CURL": real_curl,
            "FIXTURE_URL": f"http://127.0.0.1:{self.server.server_port}/archive",
            "GITLEAKS_VERSION": "fixture",
            "GITLEAKS_LINUX_X64_SHA256": hashlib.sha256(self.payload).hexdigest(),
        }
        self.tool("curl", r'''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
root = pathlib.Path(os.environ["FIXTURE"])
args = sys.argv[1:]
expected = "https://github.com/gitleaks/gitleaks/releases/download/vfixture/gitleaks_fixture_linux_x64.tar.gz"
assert [arg for arg in args if arg.startswith(("http://", "https://"))] == [expected]
with (root / "commands.jsonl").open("a") as record:
    record.write(json.dumps({"command": "curl", "args": args}) + "\n")
# Test transport only: use loopback HTTP, never the production HTTPS endpoint.
args = [os.environ["FIXTURE_URL"] if arg == expected else
        str(root / "archive.tar.gz") if arg == "/tmp/gitleaks_fixture_linux_x64.tar.gz" else
        "=http" if arg == "=https" else arg for arg in args]
# Ignore host proxy variables, without changing the production retry arguments.
args.extend(["--noproxy", "*"])
sys.exit(subprocess.run([os.environ["REAL_CURL"], *args]).returncode)
''')
        for command in ("tar", "install"):
            self.tool(command, f'''#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ["FIXTURE"])
with (root / "commands.jsonl").open("a") as record:
    record.write(json.dumps({{"command": "{command}", "args": sys.argv[1:]}}) + "\\n")
sys.exit(int(os.environ.get("{command.upper()}_EXIT", "0")))
''')
        # The exact checksum command still runs; only its fixed /tmp filename is
        # mapped to this test's private download (parallel tests cannot collide).
        self.commands = install_commands(WORKFLOW.read_text()).replace(
            '"${GITLEAKS_LINUX_X64_SHA256}  /tmp/${archive}"',
            '"${GITLEAKS_LINUX_X64_SHA256}  ${FIXTURE}/archive.tar.gz"')

    def stop_server(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        self.assertFalse(self.thread.is_alive())

    def tool(self, name, contents):
        path = self.bin / name
        path.write_text(contents)
        path.chmod(0o755)

    def run_install(self, statuses=(200,), env=None, commands=None):
        self.statuses = statuses
        return subprocess.run(["bash", "-c", commands or self.commands], cwd=self.root,
                              env={**self.env, **(env or {})}, capture_output=True,
                              text=True, timeout=20)

    def events(self):
        return [json.loads(line) for line in self.record.read_text().splitlines()]

    def test_transient_504_recovers_before_verified_extraction_and_install(self):
        result = self.run_install((504, 200))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.requests, ["/archive", "/archive"])
        self.assertEqual(self.download.read_bytes(), self.payload)
        self.assertEqual([event["command"] for event in self.events()],
                         ["curl", "tar", "install"])
        self.assertIn("OK", result.stdout)
        self.assertIn("504", result.stderr)

    def test_retry_and_transfer_limits_do_not_weaken_transport_or_checksum(self):
        result = self.run_install()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        args = self.events()[0]["args"]
        self.assertEqual(args[:2], ["-q", "-fsSL"])
        for option, value in (("--connect-timeout", "5"), ("--max-time", "60"),
                              ("--retry", "3"), ("--retry-max-time", "120"),
                              ("--proto", "=https"), ("--proto-redir", "=https")):
            self.assertEqual(args[args.index(option) + 1], value)
        for forbidden in ("-k", "--insecure", "--retry-all-errors", "--retry-connrefused"):
            self.assertNotIn(forbidden, args)
        step = WORKFLOW.read_text().split("      - name: Install gitleaks\n", 1)[1]
        self.assertIn("        timeout-minutes: 4\n", step.split("        run:", 1)[0])

    def test_access_refusal_and_missing_archive_are_not_retried(self):
        for status in (401, 403, 404):
            with self.subTest(status=status):
                self.requests.clear()
                result = self.run_install((status, 200))
                self.assertEqual(result.returncode, 22, result.stdout + result.stderr)
                self.assertEqual(self.requests, ["/archive"])
        self.assertTrue(all(event["command"] == "curl" for event in self.events()))

    def test_exhausted_transient_failure_is_fatal_and_never_extracts(self):
        result = self.run_install((504,))
        self.assertEqual(result.returncode, 22, result.stdout + result.stderr)
        self.assertEqual(len(self.requests), 4)
        self.assertEqual([event["command"] for event in self.events()], ["curl"])

    def test_wrong_archive_digest_is_fatal_without_retry_or_extraction(self):
        result = self.run_install(env={"GITLEAKS_LINUX_X64_SHA256": "0" * 64})
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(len(self.requests), 1)
        self.assertIn("FAILED", result.stdout)
        self.assertEqual([event["command"] for event in self.events()], ["curl"])

    def test_extraction_and_install_failures_remain_fatal(self):
        for variable, code, commands in (("TAR_EXIT", 41, ["curl", "tar"]),
                                         ("INSTALL_EXIT", 42, ["curl", "tar", "install"])):
            with self.subTest(variable=variable):
                if self.record.exists():
                    self.record.unlink()
                result = self.run_install(env={variable: str(code)})
                self.assertEqual(result.returncode, code, result.stdout + result.stderr)
                self.assertEqual([event["command"] for event in self.events()], commands)


if __name__ == "__main__":
    unittest.main()
