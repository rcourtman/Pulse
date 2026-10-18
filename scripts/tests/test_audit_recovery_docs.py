#!/usr/bin/env python3
"""Keep audit help from treating a failed check as a diagnosis or a reset task.

Signer/key behaviour is exercised separately by TestSignerRecoveryGuidance;
these checks bind those distinctions to the actual canonical and shipped help.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDES = ("AUDIT_LOGGING", "TROUBLESHOOTING", "CONFIGURATION", "DEPLOYMENT_MODELS", "WEBHOOKS")


def read_guide(name: str) -> str:
    return " ".join((ROOT / "docs" / f"{name}.md").read_text(encoding="utf-8").split())


class AuditRecoveryDocsTest(unittest.TestCase):
    def test_shipped_guides_match_the_canonical_evidence(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                self.assertEqual((ROOT / "docs" / f"{name}.md").read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / f"{name}.md").read_bytes())

    def test_verification_results_are_not_conflated(self):
        guide = read_guide("AUDIT_LOGGING")
        for result in ("**Not checked**", "**Unsigned**", "**Failed**", "**Unavailable**", "**Error**"):
            with self.subTest(result=result):
                self.assertIn(result, guide)
        self.assertIn("not that Pulse has established why it failed", guide)
        self.assertIn("not proof that the event failed verification", guide)
        self.assertIn("does not prove that the history is complete", guide)
        self.assertNotIn("the event data has been tampered with since it was recorded", guide)

    def test_recovery_preserves_history_and_both_keys(self):
        guide = read_guide("AUDIT_LOGGING")
        for boundary in ("consistent private backup", "copying a live `.db` file alone",
                         "including any SQLite sidecar files", "matching `.encryption.key`",
                         "isolated test instance", "without replacing the live data",
                         "or contacting monitored systems and notification destinations",
                         "Do not delete or regenerate a key", "edit audit rows or re-sign old events"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, guide)
        self.assertIn("newly generated signing key does not verify events signed with the previous key", guide)
        self.assertIn("Later enabling signing cannot authenticate it retroactively", guide)

    def test_troubleshooting_does_not_offer_a_live_key_swap(self):
        guide = read_guide("TROUBLESHOOTING")
        self.assertNotIn("restart Pulse to regenerate", guide)
        self.assertNotIn("Restore the previous `.audit-signing.key` from backup to verify", guide)
        self.assertIn("Do not swap an old key into the live instance", guide)
        self.assertIn("cannot authenticate an old unsigned event", guide)
        self.assertIn("AUDIT_LOGGING.md#verification-failures-and-safe-recovery", guide)

    def test_empty_and_gated_are_not_missing_history(self):
        for name in ("TROUBLESHOOTING", "AUDIT_LOGGING"):
            guide = read_guide(name)
            with self.subTest(guide=name):
                self.assertIn("Pulse Pro runtime required", guide)
                self.assertIn("Download Pulse Pro", guide)
                self.assertIn("filters", guide)
                self.assertIn("organisation", guide)
        short = read_guide("TROUBLESHOOTING")
        self.assertIn("A query error is not an empty history", short)
        self.assertIn("Do not change passwords or create tokens merely to populate the panel", short)
        self.assertNotIn("Community plan uses console logging only", short)

    def test_storage_paths_match_the_default_store_not_the_old_root_path(self):
        for name in ("CONFIGURATION", "DEPLOYMENT_MODELS"):
            guide = read_guide(name)
            with self.subTest(guide=name):
                self.assertIn("`audit/audit.db`", guide)
                self.assertIn("`audit/.audit-signing.key`", guide)
                self.assertIn("AUDIT_LOGGING.md#storage", guide)
                self.assertNotRegex(guide, r"(?:\| |\*\*: )`\.audit-signing\.key`")
        guide = read_guide("AUDIT_LOGGING")
        self.assertIn("**inside the audit store directory**", guide)
        self.assertIn("organisation's data directory", guide)
        self.assertIn("runtime may supply a different audit directory or managed signing key", guide)


class AuditWebhookDocsTest(unittest.TestCase):
    """Static guidance/source binding, not delivery or signature execution.

    Override text for literal-parent/adverse controls without altering source
    or reducing a substantive failure to a shipped-mirror mismatch.
    """

    text = None

    def setUp(self):
        text = self.text if self.text is not None else (ROOT / "docs/WEBHOOKS.md").read_text()
        self.guide = " ".join(text.split("## 🧾 Audit Webhooks", 1)[1].split(
            "## 🏢 Provider-hosted MSP webhooks", 1)[0].replace("**", "").split())
        self.sender = (ROOT / "pkg/audit/webhook.go").read_text()

    def test_audit_access_is_not_alert_configuration_or_licence_only(self):
        for phrase in ("separate from", "retry policies and delivery activity do not configure",
                       "`audit_logging` capability and a supporting runtime",
                       "public Community runtime does not enable", "Pulse Pro runtime required",
                       "Download Pulse Pro", "Do not buy another licence or reset data",
                       "Configuration is organisation-scoped", "administrator access",
                       "`settings:read` for GET and `settings:write` for updates",
                       "complete URL list", "not an append operation"):
            self.assertIn(phrase, self.guide)
        route = (ROOT / "internal/api/router_routes_licensing.go").read_text().split(
            "// Audit Webhook routes", 1)[1].split("type auditAdminEndpointAdapter", 1)[0]
        for boundary in ("ensureAdminSession", "featureAuditLoggingValue", "ScopeSettingsRead",
                         "ScopeSettingsWrite", "auth.ResourceAuditLogs"):
            self.assertIn(boundary, route)
        handler = (ROOT / "internal/api/activity_audit_handlers.go").read_text().split(
            "func (h *AuditHandlers) HandleUpdateWebhooks", 1)[1].split(
                "// validateWebhookURL", 1)[0]
        self.assertIn('URLs []string `json:"urls"`', handler)
        self.assertIn("getLoggerForOrg(orgID)", handler)
        self.assertIn("logger.UpdateWebhookURLs(validatedURLs)", handler)

    def test_audit_endpoint_restrictions_are_not_an_alert_setting(self):
        for phrase in ("using HTTPS with valid certificate trust", "placeholder",
                       "private/reserved IPs", "public names resolving to those addresses",
                       "does not relax this audit restriction", "Do not bypass it",
                       "separately secured, authorised ingress", "not a delivery test",
                       "Do not create failed logins"):
            self.assertIn(phrase, self.guide)
        self.assertNotIn("https://siem.corp.local", self.guide)
        validator = self.sender.split("func validateWebhookURL(", 1)[1]
        for boundary in ('".local"', "isPrivateOrReservedIP(addr.IP)", "isPrivateOrReservedIP(ip)"):
            self.assertIn(boundary, validator)
        self.assertNotIn("WebhookAllowedCIDRs", self.sender)

    def test_queue_attempts_and_success_are_not_complete_delivery(self):
        constants = dict(re.findall(r"webhook(QueueSize|MaxRetries|WorkerCount)\s*=\s*(\d+)", self.sender))
        self.assertIn(f"at most {int(constants['QueueSize']):,} events", self.guide)
        self.assertEqual(int(constants["MaxRetries"]), 3)
        self.assertEqual(int(constants["WorkerCount"]), 3)
        self.assertIn("webhookTimeout     = 30 * time.Second", self.sender)
        self.assertIn("[]time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second}", self.sender)
        for phrase in ("best effort", "drops new events when full", "in-memory queue",
                       "restart does not replay", "up to four attempts", "1, 5 and 30 seconds",
                       "30-second request timeout", "not a total delivery deadline",
                       "Only an HTTP 2xx", "does not prove the receiver verified or durably stored"):
            self.assertIn(phrase, self.guide)
        self.assertNotIn("payload of every security-relevant action", self.guide)
        self.assertIn("make(chan Event, webhookQueueSize)", self.sender)
        self.assertIn("attempt <= webhookMaxRetries", self.sender)
        self.assertIn("resp.StatusCode < 200 || resp.StatusCode >= 300", self.sender)

    def test_receiver_and_missing_event_guidance_preserve_uncertainty(self):
        for phrase in ("duplicates and out-of-order arrival", "no audit dead-letter queue",
                       "automatic history replay", "do not recover audit deliveries",
                       "not a guaranteed cancellation", "bounded local sender error",
                       "not proof that the action was never recorded",
                       "history query error is not an empty history", "not a broad export",
                       "Preserve keys and history", "do not restart or clear data"):
            self.assertIn(phrase, self.guide)
        for anchor in ("#viewing-audit-events", "#tamper-detection", "#storage"):
            self.assertIn(f"AUDIT_LOGGING.md{anchor}", self.guide)
        self.assertIn("urls := make([]string, len(w.urls))", self.sender)
        self.assertIn("copy(urls, w.urls)", self.sender)

    def test_signature_is_on_the_event_not_the_raw_post_body(self):
        for phrase in ("`data.id`", "`X-Pulse-Event-ID`", "instance/organisation",
                       "headers alone do not authenticate", "`data.signature`",
                       "not a top-level payload field", "not the raw POST body",
                       "not an audit verifier", "without signing can omit this field",
                       "unsigned, failed or unknown", "freedom from replay"):
            self.assertIn(phrase, self.guide)
        payload = self.sender.split("type WebhookPayload struct {", 1)[1].split("\n}", 1)[0]
        for field in ('`json:"event"`', '`json:"timestamp"`', 'Data      Event     `json:"data"`'):
            self.assertIn(field, payload)
        event = (ROOT / "pkg/audit/audit.go").read_text()
        self.assertIn('Signature string    `json:"signature,omitempty"`', event)
        self.assertIn('req.Header.Set("X-Pulse-Event-ID", event.ID)', self.sender)
        signer = (ROOT / "pkg/audit/signer.go").read_text()
        self.assertIn("s.mac(s.canonicalV2Form(event))", signer)

    def test_key_paths_and_private_evidence_do_not_offer_key_disclosure_or_reset(self):
        for phrase in ("`audit/.audit-signing.key`", "instance or organisation data directory",
                       "matching `.encryption.key`", "different audit directory",
                       "managed signing key", "not a guessed root path",
                       "Do not copy an encrypted key", "disclose keys or reset them",
                       "Keep raw payloads, full URLs, headers and logs private",
                       "locally reviewed, redacted excerpt"):
            self.assertIn(phrase, self.guide)
        store = (ROOT / "pkg/audit/sqlite_logger.go").read_text()
        self.assertIn('auditDir = filepath.Join(cfg.DataDir, "audit")', store)
        self.assertIn("NewSigner(auditDir, cfg.CryptoMgr)", store)
        self.assertIn("NewSignerWithKey(cfg.SigningKey)", store)
        signer = (ROOT / "pkg/audit/signer.go").read_text()
        self.assertIn('keyPath := filepath.Join(dataDir, ".audit-signing.key")', signer)
        self.assertIn("cryptoMgr.Decrypt(data)", signer)


if __name__ == "__main__":
    unittest.main()
