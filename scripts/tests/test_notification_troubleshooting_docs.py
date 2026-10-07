#!/usr/bin/env python3
"""Exercise copied notification-log recipes without a service or destination.

Synthetic readers prove bounds, stream visibility and exit handling. They do
not prove native installation, notification delivery or automatic redaction.
"""

from __future__ import annotations

from pathlib import Path
import re
import unittest

from test_troubleshooting_logs import exercise_log_recipe


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/TROUBLESHOOTING.md"
SMTP_ERROR = '{"level":"error","message":"SMTP authentication failed"}\n'
WEBHOOK_ERROR = '{"level":"error","message":"webhook returned HTTP 429"}\n'
PARTIAL_EMAIL_LOG = '{"message":"attempting to send email"}\n'
SYNTHETIC_SECRET = "synthetic-provider-echo-secret"


def section(heading: str, level: int = 3) -> str:
    marker = "#" * level + " " + heading + "\n"
    text = DOC.read_text(encoding="utf-8")
    if marker not in text:
        raise AssertionError(f"missing diagnostic section: {heading}")
    # These guide sections have no later H1. Ignore the single-# shell comments
    # inside recipes while stopping at the next same/higher-level section.
    return re.split(r"\n#{2," + str(level) + r"} ", text.split(marker, 1)[1], 1)[0]


def recipes() -> dict[str, str]:
    blocks = re.findall(r"```bash\n(.*?)```", section("Inspect Notification Logs"), re.DOTALL)
    if len(blocks) != 2:
        raise AssertionError("expected one recipe per log reader")
    return {"docker" if "# Docker" in block else "journalctl": block for block in blocks}


class NotificationTroubleshootingDocsTest(unittest.TestCase):
    def exercise(self, reader: str, *, stdout: str = "", stderr: str = "",
                 exit_code: int = 0, recipe: str | None = None, **settings):
        copied = recipe if recipe is not None else recipes()[reader]
        result, argv = exercise_log_recipe(reader, copied, stdout=stdout, stderr=stderr,
                                          exit_code=exit_code, **settings)
        self.assertEqual(argv is None, bool(settings.get("missing_timeout")))
        return result, argv

    def test_shipped_document_matches_the_tested_source(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/TROUBLESHOOTING.md").read_bytes())

    def test_copied_commands_bound_the_read_without_filtering_or_mutation(self):
        expected = {
            "journalctl": ["-u", "pulse", "--since", "15 minutes ago", "--lines", "200", "--no-pager"],
            "docker": ["logs", "--since", "15m", "--tail", "200", "pulse"],
        }
        self.assertEqual(set(recipes()), set(expected))
        for reader, copied in recipes().items():
            with self.subTest(reader=reader):
                result, argv = self.exercise(reader, stdout=SMTP_ERROR + WEBHOOK_ERROR)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(argv, expected[reader])
                self.assertEqual(result.stdout, SMTP_ERROR + WEBHOOK_ERROR)
                self.assertNotRegex(copied, r"\b(?:grep|curl|inspect|restart|printenv)\b|--follow|--token")
                self.assertIn("--signal=TERM --kill-after=1s 8s", copied)

    def test_docker_application_logs_on_both_streams_remain_visible(self):
        result, _ = self.exercise("docker", stdout=WEBHOOK_ERROR, stderr=SMTP_ERROR)
        self.assertEqual(result.returncode, 0, result.stderr)
        # Independently buffered streams have no total ordering. Both records
        # must survive, without inventing chronological order across streams.
        self.assertCountEqual(result.stdout.splitlines(),
                              (WEBHOOK_ERROR + SMTP_ERROR).splitlines())
        self.assertEqual(result.stderr, "")

    def test_reader_failure_keeps_its_exit_even_after_a_partial_email_log(self):
        for reader in recipes():
            for output in ("", PARTIAL_EMAIL_LOG):
                with self.subTest(reader=reader, partial=bool(output)):
                    result, _ = self.exercise(reader, stdout=output,
                                              stderr="synthetic reader access failure\n", exit_code=2)
                    self.assertEqual(result.returncode, 2)
                    self.assertEqual(result.stdout, "")
                    self.assertIn("Log read unavailable (exit 2)", result.stderr)
                    self.assertNotIn("synthetic reader access failure", result.stderr)

    def test_hung_readers_withhold_partial_excerpts_and_stop_by_deadline(self):
        for reader in recipes():
            with self.subTest(reader=reader):
                result, _ = self.exercise(reader, stdout=PARTIAL_EMAIL_LOG, hang="term")
                self.assertEqual(result.returncode, 124, result.stderr)
                self.assertEqual(result.stdout, "")
                self.assertIn("Log read unavailable", result.stderr)

    def test_reader_ignoring_term_is_killed_after_one_second_grace(self):
        result, _ = self.exercise("docker", stdout=PARTIAL_EMAIL_LOG, hang="ignore-term")
        self.assertEqual(result.returncode, 137, result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn("Log read unavailable (exit 137)", result.stderr)

    def test_missing_timeout_does_not_run_a_reader_or_unbounded_fallback(self):
        for reader in recipes():
            with self.subTest(reader=reader):
                result, argv = self.exercise(reader, missing_timeout=True)
                self.assertIsNone(argv)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(result.stdout, "")
                self.assertIn("no unbounded fallback", result.stderr)

    def test_successful_empty_read_is_not_documented_as_healthy_delivery(self):
        for reader in recipes():
            result, _ = self.exercise(reader)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout + result.stderr, "")
        self.assertIn("successful empty read is inconclusive", section("Inspect Notification Logs"))

    def test_raw_provider_echo_is_not_claimed_to_be_sanitised(self):
        # The command does not magically sanitise either stream. The nearby
        # disclosure boundary must survive edits to the copied recipe.
        for reader in recipes():
            result, _ = self.exercise(reader, stderr=SYNTHETIC_SECRET + "\n")
            self.assertIn(SYNTHETIC_SECRET, result.stdout)
        text = " ".join(section("Inspect Notification Logs").split())
        for boundary in ("not sanitised", "Do not post them wholesale", "manually redacted",
                         "anything echoed by the provider", "Never upload full environments",
                         "queue database", "inside the Pulse container", "actual service or container name"):
            self.assertIn(boundary, text)

    def test_legacy_email_pipeline_is_an_adverse_control(self):
        legacy = "docker logs pulse | grep email"
        result, argv = self.exercise("docker", stderr=SMTP_ERROR, recipe=legacy)
        self.assertEqual(argv, ["logs", "pulse"])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, "")
        # A human can see stderr in a terminal, but the old search itself loses
        # the match, encouraging collection of more history or debug output.
        self.assertEqual(result.stderr, SMTP_ERROR)
        result, _ = self.exercise("docker", stdout=PARTIAL_EMAIL_LOG,
                                  stderr="synthetic reader access failure\n", exit_code=2, recipe=legacy)
        self.assertEqual(result.returncode, 0, "legacy pipe must demonstrate the masked reader failure")

    def test_recovery_guidance_uses_existing_ui_without_promising_delivery(self):
        presentation = (ROOT / "frontend-modern/src/utils/alertDestinationsPresentation.ts").read_text()
        notifications = " ".join(section("Notifications").split())
        for label in ("Recent delivery activity", "Retry retained deliveries", "Dismiss retained failures"):
            self.assertIn(label, presentation)
            self.assertIn(label, notifications)
        for boundary in ("skips the persistent delivery queue", "not listed", "Notifications are paused",
                         "minimum severity and tag filters", "held-notification reasons",
                         "unavailable", "may receive a duplicate", "successful test does not itself retry",
                         "do not disable verification", "Do not delete `notification_queue.db`",
                         "do not post it wholesale", "enable debug logging just to collect it"):
            self.assertIn(boundary, notifications)
        self.assertIn("[minimum severity and tag filters](CONFIGURATION.md#destination-severity-and-tag-routing)",
                      section("Test succeeds but real alerts are missing", 4))
        self.assertIn("Both must match: an empty tag filter does not bypass minimum severity", notifications)
        self.assertIn("critical severity does not bypass a nonempty tag filter", notifications)
        for heading in ("Emails not sending", "Webhooks failing"):
            self.assertIn("#recover-retained-delivery-failures", section(heading, 4))
            self.assertIn("#inspect-notification-logs", section(heading, 4))

    def test_recovery_distinguishes_new_settings_from_retained_work(self):
        recovery = " ".join(section("Recover retained delivery failures", 4).split())
        for boundary in ("settings saved when they were queued", "does not replace that saved configuration",
                         "all retained terminal failures", "not just the destination you tested",
                         "old endpoint or credential", "disable it", "original settings remain appropriate",
                         "leave the failures retained", "do not use a batch retry to test a settings edit"):
            self.assertIn(boundary, recovery)
        # The connected queue regression exercises these distinctions with real
        # retained HTTP failures/restart/retry, not a source-text mock.
        guide = (ROOT / "docs/WEBHOOKS.md").read_text()
        self.assertIn("TROUBLESHOOTING.md#test-succeeds-but-real-alerts-are-missing", guide)
        self.assertIn("original URL and credentials", guide)

    def test_telegram_workaround_is_static_passive_and_private(self):
        heading = 'Telegram Test works but real alerts say "message text is empty"'
        telegram = " ".join(section(heading, 4).split())
        for boundary in ("no custom template", "ignore the JSON body", "does not by itself prove",
                         "one static `Content-Type` header to `application/json`", "conflicting duplicate",
                         "leaving the bot URL, `chat_id`, template and grouping settings unchanged",
                         "next normally occurring alert", "do not induce an alert or retry",
                         "#recover-retained-delivery-failures", "custom template", "private"):
            self.assertIn(boundary, telegram)
        self.assertNotRegex(telegram, r"https://api\.telegram\.org|curl|--token|Debug")
        guide = (ROOT / "docs/WEBHOOKS.md").read_text()
        self.assertIn("TROUBLESHOOTING.md#telegram-test-works-but-real-alerts-say-message-text-is-empty", guide)


if __name__ == "__main__":
    unittest.main()
