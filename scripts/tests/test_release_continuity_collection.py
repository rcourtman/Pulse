#!/usr/bin/env python3
"""Exercise the scheduled check's complete-read boundary without provider access."""

import argparse
from contextlib import redirect_stderr
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import importlib.util
import io
from itertools import product
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from threading import Thread
import unittest
from unittest.mock import patch

import yaml


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/release_control/release_continuity.py"
SPEC = importlib.util.spec_from_file_location("continuity_collection", SCRIPT)
collector = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = collector
SPEC.loader.exec_module(collector)

ENDPOINT = "repos/rcourtman/Pulse/releases?per_page=100"
REFS_ENDPOINT = "repos/rcourtman/Pulse/git/matching-refs/tags/v?per_page=100"
CANCEL = b"stream error: stream ID 1; CANCEL; received from peer\n"
BAD_GATEWAY = b"gh: service error (HTTP 502)\n"
GATEWAY_TIMEOUT = b"gh: upstream timeout (HTTP 504)\n"
RELEASE_LIST_GATEWAY_ERRORS = {
    BAD_GATEWAY: (502, "http_bad_gateway"),
    GATEWAY_TIMEOUT: (504, "http_gateway_timeout"),
}
COMPLETE = b'[[{"id":1}],[{"id":2}]]\n'


class InventoryCollectionTest(unittest.TestCase):
    def collect(self, responses, existing=None, endpoint=ENDPOINT):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            output = root / "inventory.json"
            diagnostic = root / "collection.json"
            if existing is not None:
                output.write_bytes(existing)
            calls = []

            def read(command, *, stdout, stderr, check):
                self.assertFalse(output.exists(), "partial output must not be admitted")
                calls.append(list(command))
                code, body, error = responses[len(calls) - 1]
                stdout.write(body)
                return subprocess.CompletedProcess(command, code, stderr=error)

            args = argparse.Namespace(endpoint=endpoint, output=output, diagnostic=diagnostic)
            with patch.object(collector.subprocess, "run", side_effect=read), \
                    patch.object(collector.time, "sleep") as sleep, \
                    redirect_stderr(io.StringIO()) as console:
                result = collector.collect_inventory(args)
            return (
                result, json.loads(diagnostic.read_text()),
                output.read_bytes() if output.exists() else None,
                calls, sleep.call_args_list, console.getvalue(),
            )

    def test_collects_all_pages_without_reencoding_or_a_second_read(self):
        result, receipt, output, calls, sleeps, _ = self.collect([(0, COMPLETE, b"")])
        self.assertEqual(result, 0)
        self.assertEqual(output, COMPLETE)
        self.assertEqual(receipt["status"], "collected")
        self.assertEqual(receipt["attempts"][0]["payload_sha256"], hashlib.sha256(COMPLETE).hexdigest())
        self.assertEqual(calls, [[
            "gh", "api", "--method", "GET", "--paginate", "--slurp",
            "-H", "Accept: application/vnd.github+json",
            "-H", "X-GitHub-Api-Version: 2026-03-10", ENDPOINT,
        ]])
        self.assertEqual(sleeps, [])

    def test_repeats_only_the_identical_cancelled_read_and_discards_its_partial_body(self):
        partial = b'[[{"id":999,"unfinished":'
        result, receipt, output, calls, sleeps, _ = self.collect([
            (1, partial, CANCEL), (0, COMPLETE, b""),
        ])
        self.assertEqual(result, 0)
        self.assertEqual(output, COMPLETE)
        self.assertEqual(len(calls), 2)
        self.assertEqual(calls[0], calls[1])
        self.assertEqual([call.args for call in sleeps], [(2,)])
        self.assertEqual([a["result"] for a in receipt["attempts"]], ["http2_peer_cancel", "collected"])
        first = receipt["attempts"][0]
        self.assertEqual(first["exit_code"], 1)
        self.assertEqual(first["payload_bytes"], len(partial))
        self.assertEqual(first["payload_sha256"], hashlib.sha256(partial).hexdigest())
        self.assertEqual(first["stderr_sha256"], hashlib.sha256(CANCEL).hexdigest())
        self.assertNotIn("unfinished", json.dumps(receipt))

    def test_second_cancellation_stops_without_publishing_either_partial_body(self):
        result, receipt, output, calls, sleeps, _ = self.collect([(1, b"[", CANCEL)] * 2)
        self.assertEqual(result, 1)
        self.assertIsNone(output)
        self.assertEqual(receipt["status"], "unavailable")
        self.assertEqual(len(calls), 2)
        self.assertEqual(len(sleeps), 1)

    def test_release_list_gateway_error_repeats_the_same_complete_read(self):
        partial = b'[[{"id":999}],[{"message":"incomplete page"}]]'
        for error, (status, reason) in RELEASE_LIST_GATEWAY_ERRORS.items():
            with self.subTest(status=status):
                result, receipt, output, calls, sleeps, console = self.collect([
                    (1, partial, error), (0, COMPLETE, b""),
                ])
                self.assertEqual(result, 0)
                self.assertEqual(output, COMPLETE, "never reuse pages from the failed read")
                self.assertEqual(receipt["status"], "collected")
                self.assertEqual(len(calls), 2)
                self.assertEqual(calls[0], calls[1])
                self.assertEqual([call.args for call in sleeps], [(2,)])
                first, second = receipt["attempts"]
                self.assertEqual(first["result"], reason)
                self.assertEqual(first["reported_http_status"], status)
                self.assertEqual(first["payload_sha256"], hashlib.sha256(partial).hexdigest())
                self.assertEqual(first["stderr_sha256"], hashlib.sha256(error).hexdigest())
                self.assertEqual(second["result"], "collected")
                self.assertIn(f"HTTP {status} reported by gh; repeating the identical read once", console)
                self.assertNotIn(error.decode().strip(), console + json.dumps(receipt))

    def test_transient_errors_share_one_repeat_budget(self):
        reasons = {
            CANCEL: "http2_peer_cancel",
            **{error: reason for error, (_, reason) in RELEASE_LIST_GATEWAY_ERRORS.items()},
        }
        for first, second in product(reasons, repeat=2):
            with self.subTest(first=first, second=second):
                result, receipt, output, calls, sleeps, _ = self.collect([
                    (1, b"[[", first), (1, b"{", second),
                ])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(receipt["status"], "unavailable")
                self.assertEqual(len(calls), 2)
                self.assertEqual(calls[0], calls[1])
                self.assertEqual(len(sleeps), 1)
                self.assertEqual(
                    [a["result"] for a in receipt["attempts"]], [reasons[first], reasons[second]]
                )

    def test_collected_inventory_still_rejects_a_newer_unadvertised_release(self):
        latest = {"tag_name": "v6.7.0"}
        newer = {"tag_name": "v6.8.0", "draft": False, "published_at": "2026-11-07T01:00:00Z"}
        payload = json.dumps([[latest], [newer]]).encode()
        for error in RELEASE_LIST_GATEWAY_ERRORS:
            with self.subTest(error=error):
                result, receipt, output, _, _, _ = self.collect([(1, b"[[", error), (0, payload, b"")])
                self.assertEqual(result, 0)
                self.assertEqual(receipt["status"], "collected", "collection is not trust admission")
                failures, _ = collector.frontier_violations(latest, [[]], json.loads(output), [])
                self.assertIn("newer_stable_release_not_advertised", [failure.code for failure in failures])

    def test_refusal_on_second_attempt_stops_and_remains_unavailable(self):
        for first in (CANCEL, *RELEASE_LIST_GATEWAY_ERRORS):
            with self.subTest(first=first):
                result, receipt, output, calls, _, _ = self.collect([
                    (1, b"[", first), (1, b"", b"gh: Forbidden (HTTP 403)\n"),
                ])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(receipt["attempts"][-1]["result"], "gh_failed")
                self.assertEqual(receipt["attempts"][-1]["reported_http_status"], 403)
                self.assertEqual(len(calls), 2)

    def test_reports_only_a_fixed_inventory_name_and_cli_http_status_without_retrying(self):
        for endpoint, label in [(ENDPOINT, "release"), (REFS_ENDPOINT, "tag reference")]:
            statuses = [401, 403, 404, 429, 500, 503]
            if endpoint == REFS_ENDPOINT:
                statuses.extend([502, 504])
            for status in statuses:
                with self.subTest(endpoint=endpoint, status=status):
                    error = f"gh: PRIVATE_FIXTURE (HTTP {status})\n".encode()
                    result, receipt, output, calls, sleeps, console = self.collect([
                        (1, b"{discarded", error),
                    ], endpoint=endpoint)
                    self.assertEqual(result, 1)
                    self.assertIsNone(output)
                    self.assertEqual(len(calls), 1)
                    self.assertEqual(sleeps, [])
                    attempt, = receipt["attempts"]
                    self.assertEqual(attempt["result"], "gh_failed")
                    self.assertEqual(attempt["reported_http_status"], status)
                    self.assertIn(f"GitHub {label} inventory unavailable", console)
                    self.assertIn(f"HTTP {status} reported by gh", console)
                    self.assertNotIn("PRIVATE_FIXTURE", console + json.dumps(receipt))
                    self.assertNotIn("discarded", console + json.dumps(receipt))

    def test_mixed_or_non_http_exit_reports_no_http_status(self):
        denied = b"gh: PRIVATE_FIXTURE (HTTP 403)\n"
        for code, error in [
            (1, denied + CANCEL),
            (1, b"warning: earlier failure\n" + denied),
            (1, b"private diagnostic contains (HTTP 403)"),
            (1, b"gh: line\x1b[31m (HTTP 403)"),
            (0, denied),
            (-15, denied),
        ]:
            with self.subTest(code=code, error=error):
                result, receipt, output, calls, sleeps, console = self.collect([(code, COMPLETE, error)])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(len(calls), 1)
                self.assertEqual(sleeps, [])
                self.assertIsNone(receipt["attempts"][0]["reported_http_status"])
                self.assertNotIn("HTTP 403", console)

    def test_refusals_unknown_errors_signals_and_mixed_diagnostics_never_repeat(self):
        cases = [
            (1, b"HTTP 401: Bad credentials\n"),
            (1, b"HTTP 403: Forbidden\n"),
            (1, b"HTTP 404: Not Found\n"),
            (1, b"HTTP 429: Too Many Requests\n"),
            (1, b"connection reset by peer\n"),
            (1, b"stream error: stream ID 1; INTERNAL_ERROR; received from peer\n"),
            (1, CANCEL + b"HTTP 403: Forbidden\n"),
            (1, b"private provider diagnostic\n"),
            (-15, CANCEL),
            (2, CANCEL),
        ]
        for error in RELEASE_LIST_GATEWAY_ERRORS:
            cases.extend([
                (1, b"gh: Forbidden (HTTP 403)\n" + error),
                (1, error + b"\x1b[0m"),
                (-15, error),
                (2, error),
            ])
        for code, error in cases:
            with self.subTest(code=code, error=error):
                result, receipt, output, calls, sleeps, console = self.collect([(code, b"[", error)])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(len(calls), 1)
                self.assertEqual(sleeps, [])
                self.assertEqual(receipt["attempts"][0]["exit_code"], code)
                self.assertNotIn(error.decode().strip(), console + json.dumps(receipt))

    def test_competing_status_on_the_same_line_never_authorizes_a_repeat(self):
        for terminal in (502, 504):
            for earlier in (401, 403, 429, 503):
                with self.subTest(earlier=earlier, terminal=terminal):
                    error = f"gh: upstream reported HTTP {earlier} (HTTP {terminal})\n".encode()
                    result, receipt, output, calls, sleeps, console = self.collect([
                        (1, COMPLETE, error),
                        (0, COMPLETE, b""),
                    ])
                    self.assertEqual(result, 1)
                    self.assertEqual(receipt["status"], "unavailable")
                    self.assertIsNone(output)
                    self.assertEqual(len(calls), 1)
                    self.assertEqual(sleeps, [])
                    attempt, = receipt["attempts"]
                    self.assertEqual(attempt["stderr_sha256"], hashlib.sha256(error).hexdigest())
                    self.assertIsNone(attempt["reported_http_status"])
                    self.assertNotIn("upstream reported", console + json.dumps(receipt))

    def test_successful_exit_with_incomplete_or_wrong_shape_json_never_repeats(self):
        for payload in [b"", b"[[", b"{}", b"[]", b"[{}]", b"[[1]]", b"[[true]]", b"\xff"]:
            with self.subTest(payload=payload):
                result, receipt, output, calls, sleeps, _ = self.collect([(0, payload, b"")])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(receipt["attempts"][0]["result"], "invalid_inventory")
                self.assertEqual(len(calls), 1)
                self.assertEqual(sleeps, [])

    def test_zero_exit_cannot_hide_a_refusal_or_unknown_diagnostic(self):
        for error in [b"HTTP 403: Forbidden\n", CANCEL, b"unknown warning\n"]:
            with self.subTest(error=error):
                result, receipt, output, calls, sleeps, _ = self.collect([(0, COMPLETE, error)])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(receipt["attempts"][0]["result"], "unexpected_diagnostic")
                self.assertEqual(len(calls), 1)
                self.assertEqual(sleeps, [])

    def test_preexisting_output_is_not_reused_or_overwritten(self):
        result, receipt, output, calls, sleeps, _ = self.collect([], existing=b"old inventory")
        self.assertEqual(result, 1)
        self.assertEqual(output, b"old inventory")
        self.assertEqual(receipt["reason"], "output_already_exists")
        self.assertEqual(calls, [])
        self.assertEqual(sleeps, [])

    def test_missing_gh_is_unavailable_not_a_retry(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            args = argparse.Namespace(endpoint=ENDPOINT, output=root / "out", diagnostic=root / "receipt")
            with patch.object(collector.subprocess, "run", side_effect=FileNotFoundError) as read, \
                    patch.object(collector.time, "sleep") as sleep:
                self.assertEqual(collector.collect_inventory(args), 1)
            self.assertEqual(json.loads(args.diagnostic.read_text())["reason"], "gh_unavailable")
            self.assertFalse(args.output.exists())
            self.assertEqual(read.call_count, 1)
            sleep.assert_not_called()

    def test_only_the_two_get_inventory_endpoints_are_admitted(self):
        for suffix in ["releases", "git/matching-refs/tags/v"]:
            endpoint = f"repos/rcourtman/Pulse/{suffix}?per_page=100"
            self.assertEqual(collector.inventory_endpoint(endpoint), endpoint)
        for endpoint in [
            "https://api.github.com/" + ENDPOINT, "--method=POST",
            ENDPOINT + "&page=1", ENDPOINT + "\n", "repos/rcourtman/Pulse/actions/runs",
            "repos/../Pulse/releases?per_page=100",
        ]:
            with self.subTest(endpoint=endpoint), self.assertRaises(argparse.ArgumentTypeError):
                collector.inventory_endpoint(endpoint)

    def test_real_cli_uses_the_validated_collect_route(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            gh = root / "gh"
            gh.write_text(f"#!{sys.executable}\nimport sys\nsys.stdout.buffer.write({COMPLETE!r})\n")
            gh.chmod(0o700)
            output = root / "output.json"
            receipt = root / "receipt.json"
            result = subprocess.run([
                sys.executable, str(SCRIPT), "collect", "--endpoint", ENDPOINT,
                "--output", str(output), "--diagnostic", str(receipt),
            ], env={**os.environ, "PATH": str(root) + os.pathsep + os.environ["PATH"]}, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(output.read_bytes(), COMPLETE)
            self.assertEqual(json.loads(receipt.read_text())["status"], "collected")
            self.assertEqual(sorted(p.name for p in root.iterdir()), ["gh", "output.json", "receipt.json"])

    def test_scheduled_workflow_collects_both_inventories_before_trust_checks(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/security-scan.yml").read_text())
        job = workflow["jobs"]["release-continuity"]
        step = next(s for s in job["steps"] if s.get("id") == "frontier")
        body = step["run"]
        self.assertTrue(body.startswith("set -euo pipefail\n"))
        self.assertEqual(body.count("release_continuity.py collect"), 2)
        for endpoint, name in [("git/matching-refs/tags/v", "stable-refs"), ("releases", "releases")]:
            self.assertIn(f'--endpoint "repos/${{REPOSITORY}}/{endpoint}?per_page=100"', body)
            self.assertIn(f'--output "${{evidence}}/{name}.json"', body)
            self.assertIn(f'--diagnostic "${{evidence}}/{name}-collection.json"', body)
        self.assertNotIn("gh api", body)
        self.assertLess(body.rindex("release_continuity.py collect"), body.index("fetch_registry_tags()"))
        self.assertIn("release_continuity.py frontier", body)
        for name in ["packet", "assets", "containers", "aliases", "helm"]:
            check = next(s for s in job["steps"] if s.get("id") == name)
            self.assertIn("steps.frontier.outcome == 'success'", check["if"])

    def record_workflow_evidence(self, collections):
        workflow = yaml.safe_load((ROOT / ".github/workflows/security-scan.yml").read_text())
        step = next(
            step for step in workflow["jobs"]["release-continuity"]["steps"]
            if step["name"] == "Record continuity evidence"
        )
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            evidence = root / "release-continuity-evidence"
            evidence.mkdir()
            activation = {"check": "release_activation_binding", "status": "success"}
            (evidence / "activation-diagnostic.json").write_text(json.dumps(activation))
            for name, data in collections.items():
                (evidence / f"{name}-collection.json").write_text(data)
            summary = root / "summary"
            result = subprocess.run(
                ["bash", "-c", step["run"]], cwd=root, text=True, capture_output=True,
                env={
                    **os.environ,
                    **dict.fromkeys(step["env"], ""),
                    "RUN_ID": "42",
                    "RUN_ATTEMPT": "1",
                    "TRIGGER_SCHEDULE": "17 */6 * * *",
                    "FRONTIER_RESULT": "failure",
                    "ACTIVATION_RESULT": "success",
                    "GITHUB_STEP_SUMMARY": str(summary),
                },
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            report = (evidence / "continuity-evidence.json").read_text()
            self.assertEqual(summary.read_text(), report)
            return json.loads(report)

    def test_reports_collector_failure_separately_from_policy_and_activation(self):
        _, refs, _, _, _, _ = self.collect([(0, COMPLETE, b"")], endpoint=REFS_ENDPOINT)
        _, releases, _, _, _, _ = self.collect([(1, b"[", GATEWAY_TIMEOUT)] * 2)
        report = self.record_workflow_evidence({
            "stable-refs": json.dumps(refs),
            "releases": json.dumps(releases),
        })
        self.assertEqual(report["checks"]["stable_publication_frontier"], "failure")
        self.assertEqual(report["diagnostics"]["stable_publication_frontier"]["status"], "not_run")
        self.assertEqual(report["diagnostics"]["activation_binding"]["status"], "success")
        self.assertEqual(report["diagnostics"]["github_inventory_collection"], {
            "stable_refs": refs,
            "releases": releases,
        })

    def test_missing_or_truncated_receipts_do_not_hide_the_independent_activation_result(self):
        report = self.record_workflow_evidence({"stable-refs": '{"status": "collected"'})
        missing = {"status": "not_observed", "attempts": []}
        self.assertEqual(report["diagnostics"]["github_inventory_collection"], {
            "stable_refs": missing,
            "releases": missing,
        })
        self.assertEqual(report["diagnostics"]["activation_binding"]["status"], "success")


class RegistryInventoryPolicyTest(unittest.TestCase):
    workflow = ROOT / ".github/workflows/security-scan.yml"
    token_path = "/token"
    tags_path = "/v2/fixture/pulse/tags/list?n=10000"
    token = b'{"token":"fixture-read-token"}'
    tags = b'{"name":"fixture/pulse","tags":["v6.8.0"]}'

    def request_inventory(self, token_responses, tag_responses):
        workflow = yaml.safe_load(self.workflow.read_text())
        step = next(item for item in workflow["jobs"]["release-continuity"]["steps"]
                    if item.get("id") == "frontier")
        body = step["run"]
        start = body.index("fetch_registry_tags() {")
        helper = body[start:body.index("\n}\n", start) + 2]
        token_path, tags_path = self.token_path, self.tags_path
        responses = {token_path: iter(token_responses), tags_path: iter(tag_responses)}
        calls = []

        class Registry(BaseHTTPRequestHandler):
            def do_GET(self):
                calls.append((self.path, self.headers.get("Authorization")))
                status, reply = next(responses.get(self.path, iter(())), (404, b""))
                self.send_response(status)
                self.send_header("Content-Length", str(len(reply)))
                self.end_headers()
                self.wfile.write(reply)

            def log_message(self, *_):
                pass

        # Execute the actual workflow helper and real curl with no inherited
        # credentials, proxy configuration or provider endpoints.
        with tempfile.TemporaryDirectory() as directory, HTTPServer(("127.0.0.1", 0), Registry) as server:
            base = f"http://127.0.0.1:{server.server_port}"
            output = Path(directory) / "tags.json"
            serving = Thread(target=server.serve_forever, kwargs={"poll_interval": 0.05})
            serving.start()
            try:
                result = subprocess.run(
                    ["bash", "-c", f'set -euo pipefail\n{helper}\nfetch_registry_tags "$@"',
                     "registry-fixture", "registry.example", base + token_path, base, "fixture/pulse", str(output)],
                    cwd=directory, env={"PATH": os.environ["PATH"], "HOME": directory, "LC_ALL": "C"},
                    capture_output=True, text=True, timeout=20,
                )
            finally:
                server.shutdown()
                serving.join()
            return result, calls, json.loads(output.read_text()) if output.exists() else None

    def test_access_refusal_is_never_replayed(self):
        token_call = (self.token_path, None)
        tags_call = (self.tags_path, "Bearer fixture-read-token")
        for status, phase in product((401, 403), ("token", "tags")):
            with self.subTest(status=status, phase=phase):
                refusal = (status, b'{"error":"denied"}')
                tokens = [refusal] if phase == "token" else []
                tags = [refusal] if phase == "tags" else []
                # The next reply would pass if the refusal were replayed.
                result, calls, inventory = self.request_inventory(
                    tokens + [(200, self.token)], tags + [(200, self.tags)],
                )
                self.assertEqual(calls, [token_call] if phase == "token" else [token_call, tags_call])
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(str(status), result.stderr)
                self.assertNotIn("fixture-read-token", result.stdout + result.stderr)
                self.assertIsNone(inventory)

    def test_transient_failures_retry_the_same_get_and_keep_only_the_complete_inventory(self):
        result, calls, inventory = self.request_inventory(
            [(502, b'{"error":"transient"}'), (200, self.token)],
            [(503, b'{"error":"transient"}'), (200, self.tags)],
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [(self.token_path, None)] * 2 +
                         [(self.tags_path, "Bearer fixture-read-token")] * 2)
        self.assertEqual(inventory, {"name": "registry.example/fixture/pulse", "tags": ["v6.8.0"]})
        self.assertNotIn("fixture-read-token", result.stdout + result.stderr)

    def test_partial_or_duplicate_payloads_are_not_inventory(self):
        for response in (b'{"tags":', self.tags + b"\n" + self.tags):
            with self.subTest(response=response):
                result, calls, inventory = self.request_inventory([(200, self.token)], [(200, response)])
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, [(self.token_path, None), (self.tags_path, "Bearer fixture-read-token")])
                self.assertIsNone(inventory)


if __name__ == "__main__":
    unittest.main()
