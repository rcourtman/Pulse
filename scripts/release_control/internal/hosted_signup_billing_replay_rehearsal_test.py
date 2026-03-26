from __future__ import annotations

import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import hosted_signup_billing_replay_rehearsal as mod


def run_main(argv: list[str]) -> int:
    with contextlib.redirect_stdout(io.StringIO()):
        return mod.main(argv)


class HostedSignupBillingReplayRehearsalTest(unittest.TestCase):
    def test_make_stripe_signature_is_stable_for_fixed_timestamp(self) -> None:
        sig = mod.make_stripe_signature(b'{"id":"evt_1"}', "whsec_test", timestamp=123)
        self.assertEqual(
            sig,
            "t=123,v1=94d2453cc7960463874f8b9ebad526244005352e0a5320ad624d93ecc2df970d",
        )

    def test_run_rehearsal_full_flow(self) -> None:
        prelink_payload = json.dumps({"id": "evt_pre", "type": "checkout.session.completed"}).encode("utf-8")
        with tempfile.TemporaryDirectory() as tmp:
            prelink_path = Path(tmp) / "pre.json"
            prelink_path.write_bytes(prelink_payload)

            args = mod.parse_args(
                [
                    "--base-url",
                    "https://pulse.example.com",
                    "--fail-closed-base-url",
                    "https://pulse-bad.example.com",
                    "--signup-email",
                    "owner@example.com",
                    "--org-name",
                    "Pulse Labs",
                    "--api-token",
                    "token",
                    "--expected-checkout-base",
                    "https://billing.example.com/start-pro-trial",
                    "--prelink-webhook-payload-file",
                    str(prelink_path),
                    "--prelink-webhook-secret",
                    "whsec_test",
                    "--prelink-webhook-expected-status",
                    "500",
                ]
            )

            fetch_json_calls: list[tuple[str, str, dict[str, str] | None]] = []
            fetch_calls: list[tuple[str, str, dict[str, str] | None, bytes | None]] = []

            def fake_fetch_json(method: str, url: str, **kwargs):
                fetch_json_calls.append((method, url, kwargs.get("headers")))
                if url == "https://pulse-bad.example.com/api/public/signup":
                    return 503, {"code": "public_url_missing"}
                if url == "https://pulse.example.com/api/license/trial/start":
                    return 409, {
                        "code": "trial_signup_required",
                        "details": {"action_url": "https://billing.example.com/start-pro-trial?org_id=default"},
                    }
                if url == "https://pulse.example.com/api/public/signup":
                    return 201, {
                        "org_id": "org-123",
                        "message": "Check your email for a magic link to finish signing in.",
                    }
                if url == "https://pulse.example.com/api/public/magic-link/request":
                    return 200, {"success": True}
                if url == "https://pulse.example.com/api/hosted/organizations":
                    return 200, [{"org_id": "default"}, {"org_id": "org-123"}]
                if url == "https://pulse.example.com/api/admin/orgs/org-123/billing-state":
                    return 200, {"subscription_state": "trial", "plan_version": "cloud_trial"}
                raise AssertionError(f"unexpected fetch_json call: {method} {url}")

            def fake_fetch(method: str, url: str, **kwargs):
                fetch_calls.append((method, url, kwargs.get("headers"), kwargs.get("body")))
                if url == "https://pulse.example.com/api/stripe/webhook":
                    return 500, b'{"code":"stripe_processing_failed"}', {"content-type": "application/json"}
                raise AssertionError(f"unexpected fetch call: {method} {url}")

            with (
                mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json),
                mock.patch.object(mod, "fetch", side_effect=fake_fetch),
            ):
                results = mod.run_rehearsal(args)

            self.assertTrue(all(result.ok for result in results), [r.detail for r in results])
            trial_headers = next(headers for _method, url, headers in fetch_json_calls if url.endswith("/api/license/trial/start"))
            self.assertEqual(trial_headers["X-API-Token"], "token")
            webhook_headers = next(headers for _method, url, headers, _body in fetch_calls if url.endswith("/api/stripe/webhook"))
            self.assertIn("Stripe-Signature", webhook_headers)

    def test_run_rehearsal_checks_postlink_billing_state(self) -> None:
        post_payload = json.dumps({"id": "evt_post", "type": "checkout.session.completed"}).encode("utf-8")
        with tempfile.TemporaryDirectory() as tmp:
            post_path = Path(tmp) / "post.json"
            post_path.write_bytes(post_payload)
            billing_state_calls = 0

            def fake_fetch_json(method: str, url: str, **kwargs):
                nonlocal billing_state_calls
                if url.endswith("/api/license/trial/start"):
                    return 409, {
                        "code": "trial_signup_required",
                        "details": {"action_url": "https://billing.example.com/start-pro-trial?org_id=default"},
                    }
                if url.endswith("/api/public/signup"):
                    return 201, {
                        "org_id": "org-123",
                        "message": "Check your email for a magic link to finish signing in.",
                    }
                if url.endswith("/api/public/magic-link/request"):
                    return 200, {"success": True}
                if url.endswith("/api/hosted/organizations"):
                    return 200, [{"org_id": "org-123"}]
                if url.endswith("/api/admin/orgs/org-123/billing-state"):
                    billing_state_calls += 1
                    if billing_state_calls == 1:
                        return 200, {"subscription_state": "trial", "plan_version": "cloud_trial"}
                    return 200, {"subscription_state": "active", "plan_version": "cloud_starter"}
                raise AssertionError(f"unexpected fetch_json call: {method} {url}")

            def fake_fetch(method: str, url: str, **kwargs):
                if url.endswith("/api/stripe/webhook"):
                    return 200, b'{"received":true,"status":"processed"}', {}
                raise AssertionError(f"unexpected fetch call: {method} {url}")

            with (
                mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json),
                mock.patch.object(mod, "fetch", side_effect=fake_fetch),
            ):
                exit_code = run_main(
                    [
                        "--base-url",
                        "https://pulse.example.com",
                        "--signup-email",
                        "owner@example.com",
                        "--org-name",
                        "Pulse Labs",
                        "--api-token",
                        "token",
                        "--postlink-webhook-payload-file",
                        str(post_path),
                        "--postlink-webhook-secret",
                        "whsec_test",
                        "--postlink-webhook-expected-status",
                        "200",
                        "--expected-postlink-subscription-state",
                        "active",
                        "--expected-postlink-plan-version",
                        "cloud_starter",
                    ]
                )
            self.assertEqual(exit_code, 0)

    def test_render_markdown_report(self) -> None:
        report = mod.render_markdown_report(
            title="Hosted Signup Billing Replay Rehearsal",
            base_url="https://pulse.example.com",
            signup_email="owner@example.com",
            org_name="Pulse Labs",
            results=[
                mod.CheckResult(name="public-hosted-signup", ok=True, detail="created org_id=org-123"),
                mod.CheckResult(name="postlink-webhook-delivery", ok=False, detail="status=500"),
            ],
        )
        self.assertIn("# Hosted Signup Billing Replay Rehearsal", report)
        self.assertIn("`PASS` `public-hosted-signup`", report)
        self.assertIn("`FAIL` `postlink-webhook-delivery`", report)

    def test_main_writes_report(self) -> None:
        def fake_fetch_json(method: str, url: str, **kwargs):
            if url.endswith("/api/public/signup"):
                return 201, {
                    "org_id": "org-123",
                    "message": "Check your email for a magic link to finish signing in.",
                }
            if url.endswith("/api/public/magic-link/request"):
                return 200, {"success": True}
            raise AssertionError(f"unexpected fetch_json call: {method} {url}")

        with tempfile.TemporaryDirectory() as tmp:
            report_path = Path(tmp) / "report.md"
            with mock.patch.object(mod, "fetch_json", side_effect=fake_fetch_json):
                exit_code = mod.main(
                    [
                        "--base-url",
                        "https://pulse.example.com",
                        "--signup-email",
                        "owner@example.com",
                        "--org-name",
                        "Pulse Labs",
                        "--report-out",
                        str(report_path),
                    ]
                )
            self.assertEqual(exit_code, 0)
            self.assertIn("# Hosted Signup Billing Replay Rehearsal", report_path.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
