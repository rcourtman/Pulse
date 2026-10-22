#!/usr/bin/env python3
"""Check safe operator entry points, not external-client or workload recovery.

The copied HTTP recipes run only against synthetic guest-local fixtures.
"""

from pathlib import Path
import json
import os
import re
import subprocess
import tempfile
import unittest

from test_api_auth_docs import (
    ROOT, TEST_TOKEN, exercise_curl, private_recording_server, request_helper,
)


def guide():
    return (ROOT / "docs/AGENT_SUBSTRATE.md").read_text(encoding="utf-8")


def adapter_readme():
    return (ROOT / "cmd/pulse-mcp/README.md").read_text(encoding="utf-8")


def installer_section():
    return adapter_readme().split("## Install\n", 1)[1].split("### 3. Build from source", 1)[0]


def compact(value):
    return " ".join(value.split())


def requests():
    blocks = re.findall(r"```bash\n(.*?)```", guide(), re.DOTALL)
    return [line.strip() for block in blocks for line in block.splitlines() if line.strip()]


class ExternalAgentDocsTest(unittest.TestCase):
    def test_install_guides_distinguish_release_signatures_from_platform_approval(self):
        for source in (guide(), adapter_readme()):
            text = compact(source)
            for boundary in ("pinned release key", "notaris", "Gatekeeper", "Authenticode",
                             "ssh-keygen", "-Y verify", "do not clear quarantine"):
                with self.subTest(boundary=boundary):
                    self.assertIn(boundary, text)
            self.assertNotRegex(text, r"xattr\s+-d|ExecutionPolicy\s+Bypass|same unsigned binary")
            self.assertIn("do not cryptographically authenticate the script itself", text)
        # The guide describes both production installers' existing trust check,
        # not a newly implemented signing or OS-notarisation capability.
        for name in ("install-mcp.sh", "install-mcp.ps1"):
            source = (ROOT / "scripts" / name).read_text()
            self.assertIn("checksums.txt", source)
            self.assertIn("pulse-install", source)
            self.assertRegex(source, r"-Y[\s\S]{0,4}verify")

    def test_adapter_downloads_are_separate_from_execution(self):
        readme = installer_section()
        shell = re.findall(r"```sh\n(.*?)```", readme, re.DOTALL)
        powershell = re.findall(r"```powershell\n(.*?)```", readme, re.DOTALL)
        self.assertEqual(len(shell), 2)
        self.assertEqual(len(powershell), 2)
        for download in (shell[0], powershell[0]):
            self.assertNotRegex(download, r"\|\s*(?:bash|sh|iex|Invoke-Expression)\b")
            self.assertNotIn("PULSE_API_TOKEN", download)
        self.assertIn("mktemp -d", shell[0])
        self.assertIn("&&", shell[0])
        self.assertIn("[Guid]::NewGuid()", powershell[0])
        self.assertIn("-ErrorAction Stop", powershell[0])
        self.assertIn("-TimeoutSec 60", powershell[0])
        self.assertEqual(powershell[1].strip(), "& $pulseMcpInstaller")

    def test_copied_unix_download_never_executes_received_bytes(self):
        shell = re.findall(r"```sh\n(.*?)```", installer_section(), re.DOTALL)
        # Run the exact copied download block. curl is a local argument/payload
        # fixture, not a network client; the received script only writes a marker.
        for exit_code in (0, 22, 28):
            with self.subTest(exit_code=exit_code), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / "bin").mkdir()
                (root / "scratch").mkdir()
                curl = root / "bin/curl"
                curl.write_text("#!/usr/bin/env python3\n"
                    "import json, os, pathlib, sys\n"
                    "args = sys.argv[1:]\n"
                    "pathlib.Path(os.environ['CURL_ARGS']).write_text(json.dumps(args))\n"
                    "pathlib.Path(args[args.index('--output') + 1]).write_text("
                    "'printf executed > \"$INSTALL_EXECUTED\"\\n')\n"
                    "sys.exit(int(os.environ['CURL_EXIT']))\n")
                curl.chmod(0o700)
                env = {**os.environ, "PATH": str(root / "bin") + os.pathsep + os.environ['PATH'],
                       "TMPDIR": str(root / "scratch"), "CURL_ARGS": str(root / "args.json"),
                       "CURL_EXIT": str(exit_code), "INSTALL_EXECUTED": str(root / "executed")}
                result = subprocess.run(["bash", "-c", shell[0]], env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, exit_code, result.stderr)
                self.assertFalse((root / "executed").exists())
                args = json.loads((root / "args.json").read_text())
                self.assertEqual(args[0], "--disable")
                for flag, value in (("--proto", "=https"), ("--proto-redir", "=https"),
                                    ("--connect-timeout", "10"), ("--max-time", "60")):
                    self.assertEqual(args[args.index(flag) + 1], value)
                for flag in ("--fail", "--silent", "--show-error", "--location"):
                    self.assertIn(flag, args)
                self.assertEqual(args[-1], "https://github.com/rcourtman/Pulse/releases/latest/download/install-mcp.sh")
                download = Path(args[args.index("--output") + 1])
                self.assertTrue(download.is_relative_to(root / "scratch"))
                self.assertEqual(download.parent.stat().st_mode & 0o777, 0o700)
                if exit_code == 0:
                    # Only the separate, explicitly selected execution block
                    # runs the benign received fixture, in the same saved path.
                    run = subprocess.run(["bash", "-c", shell[1]],
                        env={**env, "pulse_mcp_setup": str(download.parent)}, capture_output=True, text=True)
                    self.assertEqual(run.returncode, 0, run.stderr)
                    self.assertEqual((root / "executed").read_text(), "executed")

    def test_copied_unix_download_stops_when_private_directory_creation_fails(self):
        download = re.findall(r"```sh\n(.*?)```", installer_section(), re.DOTALL)[0]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "mktemp").write_text("#!/bin/sh\nexit 1\n")
            (root / "mktemp").chmod(0o700)
            (root / "curl").write_text('#!/bin/sh\nprintf called > "$CURL_CALLED"\n')
            (root / "curl").chmod(0o700)
            result = subprocess.run(["bash", "-c", download], capture_output=True, text=True,
                env={**os.environ, "PATH": str(root) + os.pathsep + os.environ['PATH'],
                     "CURL_CALLED": str(root / "called")})
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((root / "called").exists())

    def test_shipped_guide_matches_canonical(self):
        self.assertEqual(guide(), (ROOT / "frontend-modern/public/docs/AGENT_SUBSTRATE.md").read_text())

    def test_read_only_setup_does_not_grant_whole_surface_authority(self):
        text = compact(guide())
        self.assertIn("Start read-only", text)
        self.assertIn("## What the endpoints offer", text)
        self.assertLess(text.index("Start read-only"), text.index("## What the endpoints offer"))
        for boundary in ("not the minimum for a read-only client", "monitoring:read",
                         "client can list tools its token cannot call", "scope_only",
                         "writes do not use the action approval loop", "organisation/resource restrictions",
                         "not a read-only substitute", "PUT replaces the whole saved record",
                         "preserve unrelated fields", "separation of duties", "live readiness"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)

    def test_credentials_data_and_proxy_access_stay_private(self):
        for source in (guide(), adapter_readme()):
            text = compact(source)
            for boundary in ("project-shared", ".mcp.json", "opencode.json", "HTTPS",
                             "client process", "local administrators", "model provider",
                             "deleting", "does not revoke it"):
                with self.subTest(boundary=boundary):
                    self.assertIn(boundary.casefold(), text.casefold())
        readme = compact(adapter_readme())
        self.assertIn("not a recommended default", readme)
        self.assertIn("do not exempt discovery or health from proxy authentication", readme)
        self.assertIn("403 is not by itself proof of a missing scope", readme)
        self.assertIn("leave it denied", readme)
        self.assertNotIn("Make sure `/api/agent/capabilities` is reachable without a credential", readme)

    def test_probe_is_not_a_real_credential_diagnostic(self):
        text = compact(guide())
        for boundary in ("not a production diagnostic tool", "--token", "process arguments",
                         "prints resource context and event bodies", "Do not use it with a real token",
                         "isolated synthetic fixture", "private-response HTTP checks",
                         "do not prove that an event was received"):
            self.assertIn(boundary, text)
        # Bind the warning to the unchanged reference-client behaviour, rather
        # than claiming a safer credential input or redacted output exists.
        source = (ROOT / "cmd/agent-probe/main.go").read_text()
        self.assertIn('flag.String("token",', source)
        self.assertIn('fmt.Println("  " + string(out))', source)

    def test_terminal_events_and_partial_checks_do_not_claim_recovery(self):
        text = compact(guide())
        for boundary in ("including failed or refused actions", "not necessarily a successful dispatch",
                         "unknown", "unverified", "ran: false", "failed", "verified",
                         "ranAt", "particular postcondition", "recorded time",
                         "does not prove workload recovery", "a change may already have happened",
                         "Do not resend execution, invent another request ID", "independently of Pulse"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)
        self.assertNotIn("so an agent can confirm an outcome without polling", text)
        self.assertIn("failed or refused terminal actions", compact(adapter_readme()))

    def test_copied_checks_are_only_two_bounded_private_reads(self):
        self.assertEqual(requests(), ["pulse_api GET /api/agent/capabilities",
                                     "pulse_api GET /api/agent/fleet-context"])
        text = compact(guide())
        for boundary in ("same Bash session", "API.md#-authentication", "owner-only files",
                         "without redirects or retries", "not an installed Pulse command",
                         "only the protected fleet read checks token access", "not an empty fleet",
                         "Do not run a workload action, change operator state, induce a finding"):
            self.assertIn(boundary, text)
        for step in requests():
            subprocess.run(["bash", "-n", "-c", request_helper() + "\n" + step],
                           check=True, capture_output=True)

    def test_exact_copied_reads_keep_synthetic_credentials_and_responses_private(self):
        for step in requests():
            with self.subTest(step=step), tempfile.TemporaryDirectory() as temporary:
                with private_recording_server() as (port, recorded):
                    result = exercise_curl(self, Path(temporary), "X-API-Token: " + TEST_TOKEN,
                                           port, request_helper() + "\n" + step, private_response=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual([(r[0], r[2], r[3]) for r in recorded],
                                 [(step.split()[2], "GET", b"")])

    def test_denied_redirect_and_incomplete_reads_stop_without_mutation_or_retry(self):
        for step in requests():
            for status, partial in ((401, False), (403, False), (302, False), (200, True)):
                with self.subTest(step=step, status=status, partial=partial):
                    with tempfile.TemporaryDirectory() as temporary:
                        with private_recording_server(status, partial) as (port, recorded):
                            result = exercise_curl(self, Path(temporary), "X-API-Token: " + TEST_TOKEN,
                                                   port, request_helper() + "\n" + step,
                                                   private_response=True)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual([(r[0], r[2]) for r in recorded], [(step.split()[2], "GET")])


if __name__ == "__main__":
    unittest.main()
