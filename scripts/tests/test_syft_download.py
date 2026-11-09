#!/usr/bin/env python3
"""Run all three Syft install blocks with real offline curl/tar/install.

The transport shim maps only the pinned upstream URL and HTTPS protocol to
loopback HTTP. Fixture digests replace the source pin, never production. Only
the explicit exhaustion/stall cases shorten deadlines, recorded separately.
These tests cannot establish upstream recovery or real Syft/SBOM acceptance.
"""

import hashlib
import http.server
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import textwrap
import threading
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ("build-release-candidate.yml", "create-release.yml", "backfill-release-assets.yml")
PIN = "590650c2743b83f327d1bf9bec64f6f83b7fec504187bb84f500c862bf8f2a0f"
URL = "https://github.com/anchore/syft/releases/download/v1.42.4/syft_1.42.4_linux_amd64.tar.gz"
PROGRAM = b"#!/bin/sh\nprintf 'syft 1.42.4 offline fixture\\n'\n"


def install_commands(name):
    content = (ROOT / ".github/workflows" / name).read_text()
    match = re.search(r"^      - name: Install Syft\n(?:        timeout-minutes: \d+\n)?"
                      r"        run: \|\n((?:          .*\n|\n)+)", content, re.M)
    if not match:
        raise AssertionError(f"missing exact Syft consumer in {name}")
    return textwrap.dedent(match[1])


class SyftBootstrapTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.tmp = self.root / "tmp"
        self.tmp.mkdir()
        self.record = self.root / "commands.jsonl"
        self.requests = []
        self.request_times = []
        self.statuses = (200,)
        self.body_delay = 0
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode="w:gz") as stream:
            member = tarfile.TarInfo("syft")
            member.size = len(PROGRAM)
            member.mode = 0o755
            stream.addfile(member, io.BytesIO(PROGRAM))
        self.payload = archive.getvalue()
        self.digest = hashlib.sha256(self.payload).hexdigest()
        fixture = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                fixture.requests.append(self.path)
                fixture.request_times.append(time.monotonic())
                status = fixture.statuses[min(len(fixture.requests) - 1, len(fixture.statuses) - 1)]
                body = fixture.payload if status == 200 else b"unavailable fixture\n"
                self.send_response(status)
                self.send_header("Content-Length", str(len(body)))
                if status == 302:
                    self.send_header("Location", "/redirected")
                self.end_headers()
                time.sleep(fixture.body_delay)
                try:
                    self.wfile.write(body)
                except (BrokenPipeError, ConnectionResetError):
                    pass  # Expected when the transfer-deadline fixture stops curl.

            def log_message(self, *args):
                pass

        self.server = http.server.HTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_server)
        self.env = {**os.environ, "PATH": f"{self.bin}:{os.environ['PATH']}",
                    "HOME": str(self.root), "TMPDIR": str(self.tmp), "FIXTURE": str(self.root),
                    "FIXTURE_URL": f"http://127.0.0.1:{self.server.server_port}/archive"}
        for name in ("curl", "tar", "install"):
            real = shutil.which(name)
            if not real:
                raise RuntimeError(f"{name} is required for offline Syft bootstrap tests")
            self.env["REAL_" + name.upper()] = real
        # An ambient config must not turn a refusal into a retried success.
        (self.root / ".curlrc").write_text("retry-all-errors\n")
        self.tool("curl", r'''
import json, os, pathlib, subprocess, sys
root = pathlib.Path(os.environ['FIXTURE'])
args = sys.argv[1:]
expected = 'https://github.com/anchore/syft/releases/download/v1.42.4/syft_1.42.4_linux_amd64.tar.gz'
assert [arg for arg in args if arg.startswith(('http://', 'https://'))] == [expected]
effective = [os.environ['FIXTURE_URL'] if arg == expected else
             '=http' if arg == '=https' else arg for arg in args]
for flag, variable in (('--retry-max-time', 'FIXTURE_RETRY_WINDOW'),
                       ('--max-time', 'FIXTURE_TRANSFER_TIME')):
    if variable in os.environ:
        effective[effective.index(flag) + 1] = os.environ[variable]
with (root / 'commands.jsonl').open('a') as stream:
    stream.write(json.dumps({'command': 'curl', 'args': args, 'effective_args': effective}) + '\n')
# Proxy bypass applies only to the loopback test server, not production.
sys.exit(subprocess.run([os.environ['REAL_CURL'], *effective, '--noproxy', '*']).returncode)
''')
        for name in ("tar", "install"):
            self.tool(name, f'''
import json, os, pathlib, subprocess, sys
root = pathlib.Path(os.environ['FIXTURE'])
args = sys.argv[1:]
with (root / 'commands.jsonl').open('a') as stream:
    stream.write(json.dumps({{'command': '{name}', 'args': args}}) + '\\n')
if '{name.upper()}_EXIT' in os.environ:
    sys.exit(int(os.environ['{name.upper()}_EXIT']))
args = [str(root / 'bin/syft') if arg == '/usr/local/bin/syft' else arg for arg in args]
sys.exit(subprocess.run([os.environ['REAL_{name.upper()}'], *args]).returncode)
''')

    def tool(self, name, contents):
        path = self.bin / name
        path.write_text(f"#!{sys.executable}\n" + textwrap.dedent(contents).lstrip())
        path.chmod(0o755)

    def stop_server(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        self.assertFalse(self.thread.is_alive())

    def run_install(self, statuses=(200,), *, workflow=WORKFLOWS[0], env=None, bad_digest=False):
        self.statuses = statuses
        self.requests.clear()
        self.request_times.clear()
        self.record.unlink(missing_ok=True)
        (self.bin / "syft").unlink(missing_ok=True)
        command = install_commands(workflow).replace(PIN, "0" * 64 if bad_digest else self.digest)
        result = subprocess.run(["bash", "-e", "-c", command], cwd=self.root,
                                env={**self.env, **(env or {})}, capture_output=True,
                                text=True, timeout=20)
        self.assertEqual(list(self.tmp.iterdir()), [], "workflow must clean up partial downloads")
        return result

    def events(self):
        return [json.loads(line) for line in self.record.read_text().splitlines()]

    def installed(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        installed = self.bin / "syft"
        self.assertEqual(installed.read_bytes(), PROGRAM)
        self.assertEqual(installed.stat().st_mode & 0o777, 0o755)
        version = subprocess.check_output([str(installed), "version"], text=True)
        self.assertIn("syft 1.42.4 offline fixture", version)
        self.assertEqual([event["command"] for event in self.events()], ["curl", "tar", "install"])

    def stopped(self, result, code=22):
        self.assertEqual(result.returncode, code, result.stdout + result.stderr)
        self.assertFalse((self.bin / "syft").exists())
        self.assertEqual([event["command"] for event in self.events()], ["curl"])

    def test_every_consumer_installs_verified_direct_success_once(self):
        for workflow in WORKFLOWS:
            with self.subTest(workflow=workflow):
                result = self.run_install(workflow=workflow)
                self.installed(result)
                self.assertEqual(self.requests, ["/archive"])
                self.assertIn("OK", result.stdout)

    def test_every_consumer_recovers_504_before_verified_extraction(self):
        for workflow in WORKFLOWS:
            with self.subTest(workflow=workflow):
                result = self.run_install((504, 200), workflow=workflow)
                self.installed(result)
                self.assertEqual(self.requests, ["/archive"] * 2)
                self.assertIn("504", result.stderr)

    def test_repeated_504s_retain_exponential_backoff_not_the_fixed_five_seconds(self):
        result = self.run_install((504, 504, 200))
        self.installed(result)
        self.assertEqual(self.requests, ["/archive"] * 3)
        delays = [later - earlier for earlier, later in zip(self.request_times, self.request_times[1:])]
        self.assertGreaterEqual(delays[0], 0.9)
        self.assertGreaterEqual(delays[1], 1.9)
        print(f"Offline curl backoff observations: {delays}")

    def test_all_access_refusals_stop_once_in_every_consumer_despite_ambient_config(self):
        for workflow in WORKFLOWS:
            for status in (401, 403, 404):
                with self.subTest(workflow=workflow, status=status):
                    self.stopped(self.run_install((status, 200), workflow=workflow))
                    self.assertEqual(self.requests, ["/archive"])

    def test_a_refusal_after_transient_failure_does_not_retry_into_success(self):
        self.stopped(self.run_install((504, 403, 200)))
        self.assertEqual(self.requests, ["/archive"] * 2)

    def test_exhausted_short_fixture_window_is_fatal_in_every_consumer(self):
        # Retain the production retry count, but use a one-second test window
        # rather than spend ten minutes deliberately failing each CI consumer.
        for workflow in WORKFLOWS:
            with self.subTest(workflow=workflow):
                result = self.run_install((504,), workflow=workflow,
                                          env={"FIXTURE_RETRY_WINDOW": "1"})
                self.stopped(result)
                self.assertGreaterEqual(len(self.requests), 1)
                self.assertLessEqual(len(self.requests), 2)
                event = self.events()[0]
                self.assertIn("600", event["args"])
                self.assertEqual(event["effective_args"][event["effective_args"].index(
                    "--retry-max-time") + 1], "1")

    def test_short_fixture_transfer_deadline_is_fatal_before_extraction(self):
        self.body_delay = 0.3
        result = self.run_install(env={"FIXTURE_RETRY_WINDOW": "1", "FIXTURE_TRANSFER_TIME": "0.1"})
        self.stopped(result, 28)
        self.assertIn("timed out", result.stderr.lower())

    def test_wrong_digest_never_retries_or_extracts_in_every_consumer(self):
        for workflow in WORKFLOWS:
            with self.subTest(workflow=workflow):
                result = self.run_install(workflow=workflow, bad_digest=True)
                self.stopped(result, 1)
                self.assertEqual(self.requests, ["/archive"])
                self.assertIn("FAILED", result.stdout)

    def test_redirected_refusal_cannot_extract_or_retry(self):
        self.stopped(self.run_install((302, 403, 200)))
        self.assertEqual(self.requests, ["/archive", "/redirected"])

    def test_tar_and_install_errors_remain_fatal_in_every_consumer(self):
        for workflow in WORKFLOWS:
            for variable, code, commands in (("TAR_EXIT", 41, ["curl", "tar"]),
                                             ("INSTALL_EXIT", 42, ["curl", "tar", "install"])):
                with self.subTest(workflow=workflow, variable=variable):
                    result = self.run_install(workflow=workflow, env={variable: str(code)})
                    self.assertEqual(result.returncode, code, result.stdout + result.stderr)
                    self.assertEqual([event["command"] for event in self.events()], commands)
                    self.assertFalse((self.bin / "syft").exists())

    def test_production_bounds_pins_and_verification_order_are_unchanged_in_every_consumer(self):
        for workflow in WORKFLOWS:
            with self.subTest(workflow=workflow):
                result = self.run_install(workflow=workflow)
                self.installed(result)
                args = self.events()[0]["args"]
                self.assertEqual(args[:2], ["-q", "-fsSL"])
                self.assertEqual([arg for arg in args if arg.startswith("https://")], [URL])
                for option, value in (("--connect-timeout", "10"), ("--max-time", "60"),
                                      ("--retry", "8"), ("--retry-max-time", "600"),
                                      ("--proto", "=https"), ("--proto-redir", "=https")):
                    self.assertEqual(args[args.index(option) + 1], value)
                for forbidden in ("-k", "--insecure", "--retry-all-errors", "--retry-connrefused",
                                  "--retry-delay", "--user", "--netrc", "--location-trusted"):
                    self.assertNotIn(forbidden, args)
                content = (ROOT / ".github/workflows" / workflow).read_text()
                step = content.split("      - name: Install Syft\n", 1)[1].split("        run:", 1)[0]
                self.assertIn("        timeout-minutes: 12\n", step)
                command = install_commands(workflow)
                self.assertIn('SYFT_VERSION="1.42.4"', command)
                self.assertIn(f'SYFT_SHA256="{PIN}"', command)
                self.assertLess(command.index("sha256sum --check --"), command.index("tar -xzf"))
                self.assertLess(command.index("tar -xzf"), command.index("install -m 0755"))


if __name__ == "__main__":
    unittest.main(verbosity=2)
