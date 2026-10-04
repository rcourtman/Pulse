#!/usr/bin/env python3
"""Execute the receiver verifier copied from the shipped webhook guide.

Synthetic signatures exercise freshness, malformed inputs and byte integrity.
This is not a running receiver, durable deduplication or notification delivery.
"""

from __future__ import annotations

import hashlib
import hmac
from pathlib import Path
import re
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/WEBHOOKS.md"
NOW = 1_791_030_000
SECRET = "synthetic-receiver-test-secret"
BODY = b'{"alertId":"alert-42","event":"alert","message":"test"}\r\n'


def copied_verifier():
    blocks = re.findall(r"```python\n(.*?)```", DOC.read_text(), re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one copyable webhook verifier")
    namespace = {}
    exec(compile(blocks[0], str(DOC), "exec"), namespace)
    return namespace["verify"]


def sign(timestamp: str, body: bytes = BODY, secret: str = SECRET) -> str:
    # Independent wire-format oracle; do not use the copied implementation.
    message = timestamp.encode("utf-8") + b"." + body
    return "v1=" + hmac.new(secret.encode("utf-8"), message, hashlib.sha256).hexdigest()


class WebhookVerificationDocsTest(unittest.TestCase):
    def check(self, timestamp=str(NOW), *, body=BODY, signature=None, secret=SECRET, now=NOW):
        if signature is None and isinstance(timestamp, str):
            signature = sign(timestamp, secret=secret.strip())
        with patch("time.time", return_value=now):
            return copied_verifier()(secret, timestamp, body, signature)

    def test_shipped_copy_matches_the_executed_guide(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/WEBHOOKS.md").read_bytes())

    def test_current_signed_payload_is_accepted(self):
        self.assertIs(self.check(), True)
        self.assertIs(self.check(secret="  " + SECRET + "\n"), True)

    def test_five_minute_boundary_and_nearby_delivery_are_accepted(self):
        for delta in (-300, -1, 0, 1, 300):
            with self.subTest(delta=delta):
                self.assertIs(self.check(str(NOW + delta)), True)

    def test_old_valid_signature_cannot_be_replayed_after_the_window(self):
        timestamp = str(NOW)
        self.assertIs(self.check(timestamp), True)
        for delta in (301, 3_600, 86_400):
            with self.subTest(delta=delta):
                self.assertIs(self.check(timestamp, now=NOW + delta), False)

    def test_future_valid_signature_is_not_accepted_beyond_the_window(self):
        for delta in (301, 3_600, 86_400):
            with self.subTest(delta=delta):
                self.assertIs(self.check(str(NOW + delta)), False)

    def test_signed_malformed_timestamps_are_rejected_without_an_exception(self):
        # These are valid MACs for invalid headers, not ordinary bad signatures.
        # Bound integer parsing and accept only the sender's ASCII seconds.
        values = ("", " " + str(NOW), str(NOW) + "\n", "+" + str(NOW),
                  str(NOW) + ".0", "1e9", "١٧٩١٠٣٠٠٠٠", str(NOW) + "," + str(NOW),
                  "9" * 5_000)
        for timestamp in values:
            with self.subTest(timestamp=timestamp[:30]):
                self.assertIs(self.check(timestamp), False)
        self.assertIs(self.check(None, signature=sign(str(NOW))), False)

    def test_malformed_or_missing_signatures_fail_closed(self):
        good = sign(str(NOW))
        for signature in ("", "v2=" + good[3:], good.upper(), good + "\n",
                          good + "," + good, "v1=" + "é" * 64, b"v1=" + b"a" * 64):
            with self.subTest(signature=str(signature)[:30]):
                self.assertIs(self.check(signature=signature), False)
        with patch("time.time", return_value=NOW):
            self.assertIs(copied_verifier()(SECRET, str(NOW), BODY, None), False)

    def test_empty_or_whitespace_key_cannot_authenticate(self):
        for secret in ("", " \n\t"):
            with self.subTest(secret=repr(secret)):
                self.assertIs(self.check(secret=secret), False)

    def test_body_timestamp_key_and_signature_are_all_authenticated(self):
        good = sign(str(NOW))
        cases = (
            {"body": BODY.replace(b"alert-42", b"alert-43")},
            {"timestamp": str(NOW + 1)},
            {"secret": SECRET + "-other"},
            {"signature": "v1=" + ("0" if good[3] != "0" else "1") + good[4:]},
        )
        for changed in cases:
            with self.subTest(changed=list(changed)):
                self.assertIs(self.check(**{"signature": good, **changed}), False)

    def test_raw_bytes_not_reserialised_json_are_signed(self):
        body = '{"message":"héllo", "alertId":"alert-42", "event":"alert"}\r\n'.encode()
        signature = sign(str(NOW), body=body)
        self.assertIs(self.check(body=body, signature=signature), True)
        self.assertIs(self.check(body=body.rstrip(), signature=signature), False)
        self.assertIs(self.check(body=body.replace(b", ", b","), signature=signature), False)
        self.assertIs(self.check(body=body.decode(), signature=signature), False)

    def test_go_sender_deterministic_vector_uses_the_same_wire_format(self):
        # The production sender's TestSignWebhookPayloadDeterministic uses
        # these exact key/timestamp/body bytes and an independent Go MAC.
        sender_test = (ROOT / "internal/notifications/webhook_signing_test.go").read_text()
        self.assertIn('signWebhookPayload("secret", "1700000000", []byte(`{"a":1}`))', sender_test)
        signature = "v1=" + hmac.new(b"secret", b'1700000000.{"a":1}', hashlib.sha256).hexdigest()
        self.assertIs(self.check("1700000000", body=b'{"a":1}', signature=signature,
                                 secret="secret", now=1_700_000_000), True)

    def test_digest_comparison_remains_constant_time(self):
        verifier = copied_verifier()
        compare = hmac.compare_digest
        with patch("time.time", return_value=NOW), patch("hmac.compare_digest", wraps=compare) as spy:
            self.assertIs(verifier(SECRET, str(NOW), BODY, sign(str(NOW))), True)
            spy.assert_called_once_with(sign(str(NOW)), sign(str(NOW)))

    def test_signed_retry_is_valid_but_not_a_new_logical_event(self):
        # Freshness alone is not at-most-once processing. A receiver needs
        # durable/atomic payload-derived deduplication, not a new MAC per retry.
        first, retry = sign(str(NOW)), sign(str(NOW + 60))
        self.assertNotEqual(first, retry)
        self.assertIs(self.check(signature=first), True)
        self.assertIs(self.check(str(NOW + 60), signature=retry, now=NOW + 60), True)
        # Changing the correlation header cannot change a timestamp/body MAC.
        # The docs must not claim that X-Pulse-Event-ID is authenticated.
        text = " ".join(DOC.read_text().split())
        for phrase in ("not every replay", "deduplicate atomically", "authenticated payload's",
                       "receiver restarts", "different events", "not covered by this HMAC",
                       "missing or duplicate signing headers", "before parsing the JSON",
                       "not from the request", "Never log the secret"):
            self.assertIn(phrase, text)

    def test_receiver_guidance_preserves_recurrences_and_all_group_members(self):
        # The Go sender tests render the copied PSA templates over actual
        # loopback HTTP. Keep the safety explanation beside those examples.
        text = " ".join(DOC.read_text().split())
        for phrase in ("not a unique incident or delivery ID", "later occurrences",
                       "Do not deduplicate permanently", "every member",
                       "old delayed recovery must not close a newer incident",
                       "event and severity", "whole-second precision",
                       "incomplete or ambiguous events", "atomically",
                       "Do not mark an event processed before its action succeeds"):
            self.assertIn(phrase, text.replace("**", ""))
        self.assertNotIn("deduplicate on it", text)
        self.assertNotIn("two-priority PSA mapping covers the full range", text)
        self.assertIn('"info"', text)
        self.assertIn("successful firing-delivery receipt", text)


if __name__ == "__main__":
    unittest.main()
