"""Offline controls for the actual browser workflow's narrow registration route."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "scripts/release_browser_tailnet.py"
WORKFLOW = ROOT / ".github/workflows/qualify-browser-update-release.yml"
SPEC = importlib.util.spec_from_file_location("release_browser_tailnet", HELPER)
tailnet = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(tailnet)


def status():
    return {"BackendState": "Running", "Self": {"Online": True,
            "Tags": ["tag:ci-deploy"], "DNSName": "ci-browser.tawny-powan.ts.net."},
            "Peer": {"private": {"DNSName": "private-peer", "private_data": "not-for-receipts"}}}


def workflow_step(name):
    step = WORKFLOW.read_text().split("      - name: " + name + "\n", 1)[1].split("      - name:", 1)[0]
    return textwrap.dedent(step.split("        run: |\n", 1)[1])


def assert_registration_binding(case, source):
    # Closed, literal action inputs: no demo fallback or caller-selected tag/key.
    action = source.split("uses: tailscale/github-action@", 1)[1].split("      - name:", 1)[0]
    expected = {
        "oauth-client-id": "${{ secrets.TS_CI_DEPLOY_OAUTH_CLIENT_ID }}",
        "oauth-secret": "${{ secrets.TS_CI_DEPLOY_OAUTH_SECRET }}",
        "tags": "tag:ci-deploy",
        "version": "'1.94.2'",
    }
    actual = dict(line.strip().split(": ", 1) for line in action.split("        with:\n", 1)[1].splitlines() if line.strip())
    case.assertEqual(expected, actual)
    for broad in ("secrets.TS_OAUTH_CLIENT_ID", "secrets.TS_OAUTH_SECRET", "tags: tag:infra"):
        case.assertNotIn(broad, source)


class SelfIdentityTest(unittest.TestCase):
    def test_reads_only_the_running_narrow_self_identity(self):
        self.assertEqual("https://ci-browser.tawny-powan.ts.net", tailnet.self_origin(status()))
        observed = status()
        observed["Peer"] = "untrusted and deliberately not a peer inventory"
        self.assertEqual("https://ci-browser.tawny-powan.ts.net", tailnet.self_origin(observed))
        observed["Self"]["DNSName"] = "a.tawny-powan.ts.net"
        self.assertEqual("https://a.tawny-powan.ts.net", tailnet.self_origin(observed))

    def test_rejects_broad_extra_missing_and_malformed_tags(self):
        for tags in (["tag:infra"], ["tag:ci-deploy", "tag:infra"], [], None,
                     "tag:ci-deploy", ["tag:ci-deploy", "tag:ci-deploy"], ["tag:production"]):
            with self.subTest(tags=tags), self.assertRaisesRegex(ValueError, "only tag:ci-deploy"):
                observed = status()
                observed["Self"]["Tags"] = tags
                tailnet.self_origin(observed)

    def test_rejects_non_self_wrong_tailnet_and_unsafe_dns(self):
        for name in (None, 1, "private-peer", "ci.other.ts.net", "https://ci.tawny-powan.ts.net",
                     "ci.tawny-powan.ts.net\nsecret", "ci.tawny-powan.ts.net..",
                     "-ci.tawny-powan.ts.net", "ci-.tawny-powan.ts.net",
                     "a" * 64 + ".tawny-powan.ts.net"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                observed = status()
                observed["Self"]["DNSName"] = name
                tailnet.self_origin(observed)
        for observed in (None, [], {}, status() | {"Self": []}, status() | {"BackendState": "NeedsLogin"}):
            with self.subTest(observed=observed), self.assertRaises(ValueError):
                tailnet.self_origin(observed)
        for online in (False, None, 1, "true"):
            with self.subTest(online=online), self.assertRaises(ValueError):
                observed = status()
                observed["Self"]["Online"] = online
                tailnet.self_origin(observed)

    def test_cli_errors_never_return_raw_status_or_peers(self):
        invalid = status()
        invalid["Self"]["Tags"] = ["tag:infra"]
        for raw in (json.dumps(invalid).encode(), b"private diagnostic is not JSON", b"\xff",
                    b" " * (tailnet.MAX_STATUS_BYTES + 1)):
            with self.subTest(bytes=len(raw)):
                result = subprocess.run(["python3", HELPER], input=raw, capture_output=True)
                self.assertEqual(2, result.returncode)
                self.assertEqual(b"", result.stdout)
                self.assertEqual((tailnet.ERROR + "\n").encode(), result.stderr)


class WorkflowBindingTest(unittest.TestCase):
    def test_only_dedicated_tag_scoped_oauth_binding_is_declared(self):
        source = WORKFLOW.read_text()
        assert_registration_binding(self, source)
        self.assertIn("id: tailnet\n", source)
        self.assertIn("if: always() && steps.tailnet.outcome == 'success'", source)
        self.assertNotIn("steps.serve.outcome == 'success'", source)

    def test_rejects_demo_credentials_and_broad_or_caller_selected_registration(self):
        source = WORKFLOW.read_text()
        for before, after in (
            ("secrets.TS_CI_DEPLOY_OAUTH_CLIENT_ID", "secrets.TS_OAUTH_CLIENT_ID"),
            ("secrets.TS_CI_DEPLOY_OAUTH_SECRET", "secrets.TS_OAUTH_SECRET"),
            ("tags: tag:ci-deploy", "tags: tag:infra"),
            ("tags: tag:ci-deploy", "tags: tag:ci-deploy,tag:infra"),
            ("tags: tag:ci-deploy", "tags: ${{ inputs.tags }}"),
            ("tags: tag:ci-deploy", "authkey: ${{ secrets.TS_OAUTH_SECRET }}"),
        ):
            with self.subTest(after=after), self.assertRaises(AssertionError):
                assert_registration_binding(self, source.replace(before, after))

    def test_missing_narrow_binding_stops_even_when_demo_credentials_exist(self):
        script = workflow_step("Require the dedicated narrow CI registration binding")
        for client_id, secret in (("", ""), ("present", ""), ("", "present"), ("present", "present")):
            env = {"PATH": os.environ["PATH"], "CI_CLIENT_ID": client_id, "CI_CLIENT_SECRET": secret,
                   "TS_OAUTH_CLIENT_ID": "broad-id-not-to-be-used", "TS_OAUTH_SECRET": "broad-secret-not-to-be-used"}
            result = subprocess.run(["bash", "-c", script], env=env, capture_output=True, text=True)
            self.assertEqual(0 if client_id and secret else 2, result.returncode)
            self.assertEqual("", result.stdout)
            self.assertNotIn("broad-id-not-to-be-used", result.stderr)
            self.assertNotIn("broad-secret-not-to-be-used", result.stderr)

    def test_actual_serve_step_stops_before_serve_and_output_on_wrong_identity(self):
        script = workflow_step("Serve only this disposable install")
        fixtures = [status()]
        for tags in (["tag:infra"], ["tag:ci-deploy", "tag:infra"], []):
            observed = status()
            observed["Self"]["Tags"] = tags
            fixtures.append(observed)
        observed = status()
        observed["Self"]["DNSName"] = "private-peer.other.ts.net"
        fixtures.append(observed)
        fixtures.append({})
        for i, observed in enumerate(fixtures):
            with self.subTest(fixture=i), tempfile.TemporaryDirectory() as tmp:
                work = Path(tmp)
                (work / "status.json").write_text(json.dumps(observed))
                fake = work / "tailscale"
                fake.write_text('#!/bin/bash\nset -eu\n'
                                'if [[ "$*" == "status --json" ]]; then cat "$FIXTURE/status.json"; '
                                'else printf "%s\\n" "$*" >> "$FIXTURE/serve-calls"; fi\n')
                fake.chmod(0o700)
                env = {"PATH": str(work) + ":" + os.environ["PATH"], "FIXTURE": tmp,
                       "GITHUB_OUTPUT": str(work / "output")}
                result = subprocess.run(["bash", "-c", script], cwd=ROOT, env=env, capture_output=True, text=True)
                self.assertEqual(0 if i == 0 else 2, result.returncode, result.stderr)
                self.assertEqual("", result.stdout)
                self.assertNotIn("private-peer", result.stderr)
                self.assertNotIn("not-for-receipts", result.stderr)
                if i == 0:
                    self.assertEqual("serve --bg --https=443 http://127.0.0.1:7655\n", (work / "serve-calls").read_text())
                    header, value, delimiter = (work / "output").read_text().splitlines()
                    self.assertEqual("origin<<" + delimiter, header)
                    self.assertTrue(delimiter.startswith("pulse_output_"))
                    self.assertEqual("https://ci-browser.tawny-powan.ts.net", value)
                else:
                    self.assertFalse((work / "serve-calls").exists())
                    self.assertFalse((work / "output").exists())


if __name__ == "__main__":
    unittest.main()
