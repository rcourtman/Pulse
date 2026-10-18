#!/usr/bin/env python3
"""Check safe operator entry points, not external-client or workload recovery.

The copied HTTP recipes run only against synthetic guest-local fixtures.
"""

from pathlib import Path
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


def compact(value):
    return " ".join(value.split())


def requests():
    blocks = re.findall(r"```bash\n(.*?)```", guide(), re.DOTALL)
    return [line.strip() for block in blocks for line in block.splitlines() if line.strip()]


class ExternalAgentDocsTest(unittest.TestCase):
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
