#!/usr/bin/env python3
"""Exercise copied setup/metrics recipes with synthetic credentials.

Run with pulse-worker-source-proof --workspace scripts. The kubectl recorder
checks local argv, pipe payloads and stop behaviour, not a real cluster. Curl
uses guest-local listeners; no real Pulse instance or token is involved.
"""

from __future__ import annotations

import base64
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest
from urllib.parse import parse_qs, quote, urlsplit

from test_api_auth_docs import (
    ROOT, TEST_TOKEN, exercise_curl, private_recording_server, request_helper,
)


GUIDES = ("KUBERNETES", "METRICS_HISTORY", "CLOUD")


def guide(name: str) -> str:
    return (ROOT / "docs" / f"{name}.md").read_text(encoding="utf-8")


def blocks(name: str, language: str = "bash") -> list[str]:
    return re.findall(r"```" + language + r"\n(.*?)```", guide(name), re.DOTALL)


def recipe(name: str, needle: str) -> str:
    matches = [block for block in blocks(name) if needle in block]
    if len(matches) != 1:
        raise AssertionError(f"expected one {name} recipe containing {needle!r}")
    return matches[0]


KUBECTL_RECORDER = '''#!/usr/bin/env python3
import base64, json, os, sys
from pathlib import Path
args = sys.argv[1:]
assert os.environ['FIXTURE_TOKEN'] not in ' '.join(args)
assert not any(k in os.environ for k in ('PULSE_TOKEN', 'API_TOKENS'))
if args[:2] == ['create', 'namespace']:
    phase = 'namespace-create'
    assert args == ['create', 'namespace', 'pulse', '--dry-run=client', '-o', 'yaml']
    document = {'kind': 'Namespace', 'metadata': {'name': 'pulse'}}
elif args[:5] == ['-n', 'pulse', 'create', 'secret', 'generic']:
    phase = 'secret-create'
    assert args[5] == 'pulse-agent-env'
    assert args[7:] == ['--dry-run=client', '-o', 'yaml']
    assert args[6].startswith('--from-file=PULSE_TOKEN=')
    path = Path(args[6].split('=', 2)[2])
    assert path.read_text().strip() == os.environ['FIXTURE_TOKEN']
    assert path.stat().st_mode & 0o777 == 0o600
    document = {'apiVersion': 'v1', 'kind': 'Secret',
                'metadata': {'name': 'pulse-agent-env', 'namespace': 'pulse'},
                'data': {'PULSE_TOKEN': base64.b64encode(path.read_bytes()).decode()}}
else:
    assert args == ['apply', '--server-side', '--field-manager=pulse-agent-setup', '-f', '-']
    payload = sys.stdin.read()
    document = json.loads(payload) if payload else None
    phase = 'secret-apply' if document and document['kind'] == 'Secret' else 'namespace-apply'
with Path(os.environ['KUBECTL_RECEIPT']).open('a') as output:
    output.write(json.dumps({'phase': phase, 'argv': args, 'document': document}) + '\\n')
if phase == os.environ.get('FAIL_PHASE'):
    sys.exit(23)
if args[0] == 'apply':
    if document is None:
        sys.exit(24)
    print(document['kind'].lower() + '/' + document['metadata']['name'] + ' serverside-applied')
else:
    print(json.dumps(document))
'''


class SetupAccessDocsTest(unittest.TestCase):
    def test_mirrors_and_secret_free_shell_recipes(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                self.assertEqual((ROOT / "docs" / f"{name}.md").read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / f"{name}.md").read_bytes())
                shell = "\n".join(blocks(name))
                self.assertNotRegex(shell, r"(?:export\s+PULSE_TOKEN|--from-literal=\w*TOKEN|--token(?:\s|=)|X-API-Token:|Bearer\s)")
                self.assertNotRegex(shell, r"(?:--insecure|--verbose|--trace\S*|\s-k\b|curl[^`]*\|\s*bash)")
                for block in blocks(name):
                    subprocess.run(["bash", "-n", "-c", block], check=True, capture_output=True)

    def test_private_file_preparation_preserves_existing_token(self):
        command = recipe("KUBERNETES", 'vi "$HOME/.config/pulse/kubernetes-agent-token"')
        with tempfile.TemporaryDirectory(prefix="setup docs ") as temporary:
            home = Path(temporary)
            tools = home / "tools"
            tools.mkdir()
            editor = tools / "vi"
            editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ -f "$1" ]\n')
            editor.chmod(0o700)
            env = {**os.environ, "HOME": str(home), "PATH": f"{tools}:{os.environ['PATH']}"}
            token = home / ".config/pulse/kubernetes-agent-token"
            for existing in (False, True):
                if existing:
                    token.write_text(TEST_TOKEN + "\n")
                    token.chmod(0o644)
                    token.parent.chmod(0o755)
                result = subprocess.run(["bash", "-eu", "-c", command], env=env, capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual(stat.S_IMODE(token.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(token.parent.stat().st_mode), 0o700)
                self.assertEqual(token.read_text(), TEST_TOKEN + "\n" if existing else "")
                self.assertNotIn(TEST_TOKEN.encode(), result.stdout + result.stderr)

    def run_secret(self, home: Path, fail: str = "", empty: bool = False):
        private = home / ".config/pulse"
        private.mkdir(parents=True, mode=0o700)
        token = private / "kubernetes-agent-token"
        token.write_text("" if empty else TEST_TOKEN + "\n")
        token.chmod(0o600)
        tools = home / "tools"
        tools.mkdir()
        recorder = tools / "kubectl"
        recorder.write_text(KUBECTL_RECORDER)
        recorder.chmod(0o700)
        receipt = home / "kubectl.jsonl"
        env = {**os.environ, "HOME": str(home), "PATH": f"{tools}:{os.environ['PATH']}",
               "FIXTURE_TOKEN": TEST_TOKEN, "KUBECTL_RECEIPT": str(receipt), "FAIL_PHASE": fail}
        for key in ("PULSE_TOKEN", "API_TOKENS"):
            env.pop(key, None)
        result = subprocess.run(["bash", "-c", recipe("KUBERNETES", "--from-file=PULSE_TOKEN=")],
                                env=env, capture_output=True, timeout=10)
        records = [json.loads(line) for line in receipt.read_text().splitlines()] if receipt.exists() else []
        self.assertNotIn(TEST_TOKEN.encode(), result.stdout + result.stderr)
        self.assertNotIn(base64.b64encode((TEST_TOKEN + "\n").encode()), result.stdout + result.stderr)
        return result, records

    def test_secret_passes_only_file_path_in_argv_and_data_over_stdin(self):
        with tempfile.TemporaryDirectory(prefix="setup docs ") as temporary:
            result, records = self.run_secret(Path(temporary))
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertEqual(sorted(r["phase"] for r in records),
                             sorted(("namespace-create", "namespace-apply", "secret-create", "secret-apply")))
            created = next(r["document"] for r in records if r["phase"] == "secret-create")
            applied = next(r["document"] for r in records if r["phase"] == "secret-apply")
            self.assertEqual(created, applied)
            self.assertEqual(base64.b64decode(applied["data"]["PULSE_TOKEN"]), (TEST_TOKEN + "\n").encode())
            self.assertNotIn("annotations", applied["metadata"])
            self.assertNotIn(TEST_TOKEN, json.dumps([r["argv"] for r in records]))

    def test_pipeline_failure_and_empty_file_stop_provisioning(self):
        for phase in ("namespace-create", "namespace-apply", "secret-create", "secret-apply", ""):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as temporary:
                result, records = self.run_secret(Path(temporary), fail=phase, empty=not phase)
                self.assertNotEqual(result.returncode, 0)
                if phase.startswith("namespace"):
                    self.assertFalse(any(r["phase"].startswith("secret") for r in records))
                if not phase:
                    self.assertEqual(records, [])

    def test_daemonset_and_helm_reference_the_same_scoped_secret(self):
        manifest = next(block for block in blocks("KUBERNETES", "yaml") if "kind: DaemonSet" in block)
        self.assertRegex(manifest, r"name: PULSE_TOKEN\n\s+valueFrom:\n\s+secretKeyRef:\n\s+name: pulse-agent-env\n\s+key: PULSE_TOKEN")
        self.assertNotIn("YOUR_API_TOKEN", manifest)
        collector = recipe("KUBERNETES", "openShift.kubernetesAgent.enabled=true")
        self.assertIn("agent.secretEnv.name=pulse-agent-env", collector)
        self.assertIn("agent.secretEnv.keys[0]=PULSE_TOKEN", collector)
        self.assertNotIn("server.secretEnv", collector)
        chart = (ROOT / "deploy/helm/pulse/templates/agent.yaml").read_text()
        for key in (".Values.agent.secretEnv", '"valueFrom"', '"secretKeyRef"', '"pulse.agentSecretName"'):
            self.assertIn(key, chart)
        content = " ".join(guide("KUBERNETES").split())
        for boundary in ("kubernetes:report", "agent:report", "Do not reuse an administrator token",
                         "does not provision API tokens", "not inherently encrypted", "RBAC",
                         "restarting the agent pods", "forcing the conflict"):
            self.assertIn(boundary, content)

    def test_cloud_uses_existing_private_file_installation_not_server_installer(self):
        content = " ".join(guide("CLOUD").split())
        for boundary in ("UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas", "--token-file",
                         "host being monitored", "Windows", "HTTPS", "stop if the download fails",
                         "top-level GitHub server installer", "Keep TLS verification enabled"):
            self.assertIn(boundary, content)


class MetricsAccessDocsHTTPTest(unittest.TestCase):
    def history_request(self) -> str:
        # Use the same helper readers are told to define, not a fixture-only
        # implementation that could conceal a direct-curl response leak.
        return request_helper() + "\n" + recipe("METRICS_HISTORY", "pulse_api GET ")

    def test_history_query_with_both_headers_and_hostile_curl_defaults(self):
        request = self.history_request()
        expected_id = "pve 1:node/1:100&other=1"
        request = request.replace("pve1%3Anode1%3A100", quote(expected_id, safe=""))
        with private_recording_server() as (port, records):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                with self.subTest(header=key), tempfile.TemporaryDirectory() as temporary:
                    result = exercise_curl(self, Path(temporary), f"{key}: {value}", port,
                                           request, private_response=True)
                    self.assertEqual(result.returncode, 0, result.stderr.decode())
                    path, headers, method, body = records[-1]
                    self.assertEqual(urlsplit(path).path, "/api/metrics-store/history")
                    self.assertEqual(parse_qs(urlsplit(path).query), {
                        "resourceType": ["vm"], "resourceId": [expected_id], "range": ["7d"], "metric": ["cpu"]})
                    self.assertEqual((method, body), ("GET", b""))
                    self.assertEqual(headers[key], value)
                    self.assertNotIn("X-Curlrc-Injected", headers)

    def test_http_auth_license_and_server_errors_fail(self):
        for status in (401, 402, 403, 500):
            with self.subTest(status=status), private_recording_server(status) as (port, records), tempfile.TemporaryDirectory() as temporary:
                result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                       self.history_request(), private_response=True)
                self.assertEqual(result.returncode, 22, result.stderr.decode())
                self.assertEqual(len(records), 1)
                self.assertIn(f"HTTP {status}".encode(), result.stdout)

    def test_redirect_and_incomplete_transport_are_not_empty_success(self):
        for status, partial, expected_exit in ((302, False, 1), (200, True, 18)):
            with self.subTest(status=status, partial=partial), private_recording_server(status, partial) as (port, records), tempfile.TemporaryDirectory() as temporary:
                result = exercise_curl(self, Path(temporary), f"X-API-Token: {TEST_TOKEN}", port,
                                       self.history_request(), private_response=True)
                self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                self.assertEqual(len(records), 1)

    def test_repeated_reads_preserve_earlier_private_responses(self):
        with private_recording_server() as (port, records), tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            for _ in range(2):
                result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}", port,
                                       self.history_request(), private_response=True)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertEqual(len(records), 2)
            self.assertEqual(len(list((home / ".config/pulse").glob("api-response.*"))), 3)

    def test_query_types_match_current_handler_and_both_references(self):
        expected = {"node", "storage", "agent", "disk", "k8s", "vm", "system-container",
                    "oci-container", "app-container", "docker-host"}
        handler = (ROOT / "internal/api/router.go").read_text().split(
            "func normalizeMetricsHistoryResourceType(", 1)[1].split("\n}\n", 1)[0]
        self.assertEqual(set(re.findall(r'case "([^"]+)":', handler)), expected)
        for name in ("API", "METRICS_HISTORY"):
            with self.subTest(guide=name):
                parameters = guide(name).split("### History" if name == "API" else
                                               "### History Query Parameters", 1)[1]
                types = parameters.split("- `resourceType` (required):", 1)[1].split(
                    "- `resourceId`", 1)[0]
                self.assertEqual(set(re.findall(r"`([^`]+)`", types)), expected)

    def test_guidance_distinguishes_coverage_access_and_safe_collection(self):
        content = " ".join(guide("METRICS_HISTORY").split())
        for boundary in ("same Bash session", "not an installed Pulse command", "owner-only file",
                         "not print or post it", "URL-encoding each query value", "no automatic retry",
                         "a `points` array", "a `metrics` object", "timestamps and `source`",
                         "does not establish durable historical coverage", "no points proves neither zero",
                         "existence of `metrics.db` does not establish", "selected organisation",
                         "not a repair for missing history", "does not migrate the existing history",
                         "Do not delete history", "backup freeze/thaw", "manually redacted error"):
            self.assertIn(boundary, content)
        self.assertFalse(any("curl " in block for block in blocks("METRICS_HISTORY")),
                         "history examples must use the existing private-response helper")


if __name__ == "__main__":
    unittest.main()
