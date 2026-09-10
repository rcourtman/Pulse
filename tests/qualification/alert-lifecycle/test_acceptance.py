import argparse
import ast
import inspect
import textwrap
import json
import io
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import urllib.error
import urllib.request

import acceptance as a


class AcceptanceContractTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.evidence = a.Evidence(Path(self.temp.name) / "proof")

    def test_refuses_remote_or_credential_bearing_targets(self):
        for url in ("https://127.0.0.1", "http://example.com", "http://10.0.0.1",
                    "http://user:secret@127.0.0.1", "http://127.0.0.1?token=secret"):
            with self.subTest(url=url), self.assertRaises((ValueError, AssertionError)):
                a.loopback_url(url)
        self.assertEqual(a.loopback_url("http://127.0.0.1:7655/"), "http://127.0.0.1:7655")

    def test_real_http_recipient_transient_then_success_retains_both(self):
        recipient = a.Recipient(self.evidence)
        server = a.serve(recipient, 0)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        recipient.mode(1)
        payload = {"resource": "synthetic", "event": "alert", "start": "old"}
        request = urllib.request.Request(
            "http://127.0.0.1:%d/receipt" % server.server_port,
            data=json.dumps(payload).encode(), headers={"Content-Type": "application/json"})
        with self.assertRaises(urllib.error.HTTPError) as failed:
            urllib.request.urlopen(request)
        self.assertEqual(failed.exception.code, 503)
        self.assertEqual(failed.exception.headers["Retry-After"], "0")
        failed.exception.close()
        with urllib.request.urlopen(request) as response:
            self.assertEqual(response.status, 200)
        self.assertEqual(len(recipient.matching("synthetic", 503)), 1)
        self.assertEqual(len(recipient.matching("synthetic")), 1)
        self.assertEqual(len(list(self.evidence.directory.glob("*recipient.json"))), 2)

    def test_persistent_failure_does_not_silently_recover(self):
        recipient = a.Recipient(self.evidence)
        recipient.mode(-1)
        for _ in range(25):
            self.assertEqual(recipient.accept({"resource": "x", "event": "alert"}), 503)
        self.assertFalse(recipient.matching("x"))
        recipient.mode(0)
        self.assertEqual(recipient.accept({"resource": "x", "event": "alert"}), 200)

    def test_resolved_delivery_cannot_satisfy_firing(self):
        recipient = a.Recipient(self.evidence)
        recipient.accept({"resource": "x", "event": "resolved"})
        self.assertFalse(recipient.matching("x"))

    def test_evidence_cannot_overwrite_prior_attempt(self):
        with self.assertRaises(FileExistsError):
            a.Evidence(self.evidence.directory)

    def test_report_is_monitoring_only_and_has_real_host_wire_shape(self):
        payload = a.report("acceptance-test", 95)
        self.assertEqual(payload["agent"]["type"], "unified")
        self.assertFalse(payload["agent"]["commandsEnabled"])
        self.assertEqual(payload["metrics"]["cpuUsagePercent"], 95)
        self.assertEqual(payload["host"]["hostname"], "acceptance-test")
        self.assertIn("timestamp", payload)

    def driver(self):
        args = argparse.Namespace(base="http://127.0.0.1:12345", run_id="test")
        with patch.dict(os.environ, {"PULSE_ACCEPTANCE_TOKEN": "synthetic-secret"}):
            return a.Driver(args, self.evidence, a.Recipient(self.evidence))

    def test_recipient_matches_fractional_api_start_at_webhook_precision(self):
        driver = self.driver()
        alert = {"id": "cpu", "startTime": "2026-09-10T15:30:02.123456789Z"}
        driver.recipient.accept({"id": "cpu", "resource": "x", "event": "alert",
                                 "start": "2026-09-10T15:30:02Z"})
        self.assertTrue(driver.received_occurrence("x", alert))

    def test_recipient_rejects_old_occurrence_and_wrong_alert(self):
        driver = self.driver()
        alert = {"id": "cpu", "startTime": "2026-09-10T15:30:02.123456789Z"}
        for identity, start in (("cpu", "2026-09-10T15:30:01Z"),
                                ("other", "2026-09-10T15:30:02Z")):
            driver.recipient.accept({"id": identity, "resource": "x", "event": "alert",
                                     "start": start})
        self.assertFalse(driver.received_occurrence("x", alert))

    def test_same_second_occurrences_fail_closed(self):
        with self.assertRaisesRegex(AssertionError, "indistinguishable"):
            a.distinct_webhook_occurrences(
                {"startTime": "2026-09-10T15:30:02.123456789Z"},
                {"startTime": "2026-09-10T15:30:02.987654321Z"})
        a.distinct_webhook_occurrences(
            {"startTime": "2026-09-10T15:30:02.123456789Z"},
            {"startTime": "2026-09-10T15:30:03.123456789Z"})

    def test_webhook_projection_preserves_offset_and_requires_timezone(self):
        self.assertEqual(a.webhook_start({"startTime": "2026-09-10T16:30:02.123+01:00"}),
                         "2026-09-10T16:30:02+01:00")
        with self.assertRaisesRegex(AssertionError, "timezone"):
            a.webhook_start({"startTime": "2026-09-10T15:30:02"})

    def test_retry_precedes_resolution_in_scenario(self):
        # Guard scenario ordering separately from the recipient/identity proof.
        tree = ast.parse(textwrap.dedent(inspect.getsource(a.Driver.run)))
        retry_lines = [n.lineno for n in ast.walk(tree) if isinstance(n, ast.Call)
                       and isinstance(n.func, ast.Attribute)
                       and n.func.attr == "retry_terminal"]
        resolved_lines = [n.lineno for n in ast.walk(tree) if isinstance(n, ast.Call)
                          and isinstance(n.func, ast.Attribute) and n.func.attr == "wait"
                          and n.args and isinstance(n.args[0], ast.Constant)
                          and n.args[0].value == "resolved"]
        self.assertEqual(len(retry_lines), 1)
        self.assertEqual(len(resolved_lines), 1)
        self.assertLess(retry_lines[0], resolved_lines[0])

    def test_retry_retains_zero_response_before_failure(self):
        driver = self.driver()
        with patch.object(driver, "api", return_value={"success": True, "affected": 0}):
            with self.assertRaisesRegex(AssertionError, "no terminal retry admitted"):
                driver.retry_terminal({"id": "alert"}, {"original"})
        files = list(self.evidence.directory.glob("*retry-response.json"))
        self.assertEqual(len(files), 1)
        self.assertEqual(json.loads(files[0].read_text())["affected"], 0)

    def test_retry_requires_original_notification_identity(self):
        driver = self.driver()
        with patch.object(driver, "api", return_value={"success": True, "affected": 1}), \
             patch.object(driver, "delivery", return_value=[{"notificationId": "other"}]), \
             patch.object(a.time, "monotonic", side_effect=[0, 100]):
            with self.assertRaises(TimeoutError):
                driver.retry_terminal({"id": "alert"}, {"original"})

    def test_retry_original_notification_succeeds(self):
        driver = self.driver()
        with patch.object(driver, "api", return_value={"success": True, "affected": 1}), \
             patch.object(driver, "delivery", return_value=[{"notificationId": "original"}]):
            driver.retry_terminal({"id": "alert"}, {"original"})
        self.assertIn("terminal-retry-sent", driver.results)

    def test_deadline_fails_instead_of_passing_empty_observation(self):
        driver = self.driver()
        with patch.object(a.time, "monotonic", side_effect=[0, 2]):
            with self.assertRaises(TimeoutError):
                driver.wait("never-seen", lambda: False, timeout=1)
        self.assertFalse(driver.results)

    def test_observer_error_not_treated_as_transient_success(self):
        driver = self.driver()
        def broken():
            raise RuntimeError("observer failed")
        with self.assertRaises(RuntimeError):
            driver.wait("broken", broken)
        self.assertFalse(driver.results)

    def test_no_redirect_token_forwarding(self):
        handler = a.NoRedirect()
        self.assertIsNone(handler.redirect_request(None, None, 302, "", {}, "http://remote"))

    def test_retained_observation_excludes_authentication_header(self):
        driver = self.driver()
        driver.record_observations = True
        with patch.object(driver.opener, "open", return_value=io.StringIO('[]')):
            self.assertEqual(driver.api("/api/alerts/active"), [])
        saved = next(self.evidence.directory.glob("*observation.json")).read_text()
        self.assertIn("/api/alerts/active", saved)
        self.assertNotIn("synthetic-secret", saved)
        self.assertNotIn("X-API-Token", saved)

    def test_wait_retains_named_success(self):
        driver = self.driver()
        self.assertEqual(driver.wait("observed", lambda: {"id": 42}), {"id": 42})
        self.assertEqual(driver.results, ["observed"])


if __name__ == "__main__":
    unittest.main()
