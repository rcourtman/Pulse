#!/usr/bin/env python3
"""Exercise the scheduled check's complete-read boundary without provider access."""

import argparse
from contextlib import redirect_stderr
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
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
CANCEL = b"stream error: stream ID 1; CANCEL; received from peer\n"
COMPLETE = b'[[{"id":1}],[{"id":2}]]\n'


class InventoryCollectionTest(unittest.TestCase):
    def collect(self, responses, existing=None):
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

            args = argparse.Namespace(endpoint=ENDPOINT, output=output, diagnostic=diagnostic)
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

    def test_collected_inventory_still_rejects_a_newer_unadvertised_release(self):
        latest = {"tag_name": "v6.7.0"}
        newer = {"tag_name": "v6.8.0", "draft": False, "published_at": "2026-11-07T01:00:00Z"}
        payload = json.dumps([[latest], [newer]]).encode()
        result, receipt, output, _, _, _ = self.collect([(1, b"[[", CANCEL), (0, payload, b"")])
        self.assertEqual(result, 0)
        self.assertEqual(receipt["status"], "collected", "collection is not trust admission")
        failures, _ = collector.frontier_violations(latest, [[]], json.loads(output), [])
        self.assertIn("newer_stable_release_not_advertised", [failure.code for failure in failures])

    def test_refusal_on_second_attempt_stops_and_remains_unavailable(self):
        result, receipt, output, calls, _, _ = self.collect([
            (1, b"[", CANCEL), (1, b"", b"HTTP 403: Forbidden\n"),
        ])
        self.assertEqual(result, 1)
        self.assertIsNone(output)
        self.assertEqual(receipt["attempts"][-1]["result"], "gh_failed")
        self.assertEqual(len(calls), 2)

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
        for code, error in cases:
            with self.subTest(code=code, error=error):
                result, receipt, output, calls, sleeps, console = self.collect([(code, b"[", error)])
                self.assertEqual(result, 1)
                self.assertIsNone(output)
                self.assertEqual(len(calls), 1)
                self.assertEqual(sleeps, [])
                self.assertEqual(receipt["attempts"][0]["exit_code"], code)
                self.assertNotIn(error.decode().strip(), console + json.dumps(receipt))

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


if __name__ == "__main__":
    unittest.main()
