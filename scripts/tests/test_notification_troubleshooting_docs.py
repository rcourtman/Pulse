#!/usr/bin/env python3
"""Exercise copied notification-log recipes without a service or destination.

Synthetic readers prove bounds, stream visibility and exit handling. They do
not prove native installation, notification delivery or automatic redaction.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


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


READER = '''#!/usr/bin/env python3
import json, os, pathlib, sys
pathlib.Path(os.environ['READER_ARGV']).write_text(json.dumps(sys.argv[1:]))
sys.stdout.write(os.environ.get('LOG_STDOUT', ''))
sys.stderr.write(os.environ.get('LOG_STDERR', ''))
sys.exit(int(os.environ.get('READER_EXIT', '0')))
'''


class NotificationTroubleshootingDocsTest(unittest.TestCase):
    def exercise(self, reader: str, *, stdout: str = "", stderr: str = "",
                 exit_code: int = 0, recipe: str | None = None):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            command = directory / reader
            command.write_text(READER, encoding="utf-8")
            command.chmod(0o700)
            argv = directory / "argv.json"
            env = dict(os.environ, PATH=f"{directory}:{os.environ['PATH']}",
                       READER_ARGV=str(argv), LOG_STDOUT=stdout, LOG_STDERR=stderr,
                       READER_EXIT=str(exit_code))
            copied = recipe if recipe is not None else recipes()[reader]
            result = subprocess.run(["bash", "-c", copied], env=env,
                                    capture_output=True, text=True, timeout=5)
            self.assertTrue(argv.exists(), "copied command did not invoke its log reader")
            return result, json.loads(argv.read_text())

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
                self.assertNotRegex(copied, r"[|<>]|\b(?:grep|curl|inspect|restart|printenv)\b|--follow|--token")

    def test_docker_application_logs_on_both_streams_remain_visible(self):
        result, _ = self.exercise("docker", stdout=WEBHOOK_ERROR, stderr=SMTP_ERROR)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, WEBHOOK_ERROR)
        self.assertEqual(result.stderr, SMTP_ERROR)

    def test_reader_failure_keeps_its_exit_even_after_a_partial_email_log(self):
        for reader in recipes():
            for output in ("", PARTIAL_EMAIL_LOG):
                with self.subTest(reader=reader, partial=bool(output)):
                    result, _ = self.exercise(reader, stdout=output,
                                              stderr="synthetic reader access failure\n", exit_code=2)
                    self.assertEqual(result.returncode, 2)
                    self.assertEqual(result.stdout, output)
                    self.assertEqual(result.stderr, "synthetic reader access failure\n")

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
            self.assertIn(SYNTHETIC_SECRET, result.stderr)
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
                         "minimum alert severity and tag filters", "held-notification reasons",
                         "unavailable", "may receive a duplicate", "successful test does not itself retry",
                         "do not disable verification", "Do not delete `notification_queue.db`",
                         "do not post it wholesale", "enable debug logging just to collect it"):
            self.assertIn(boundary, notifications)
        for heading in ("Emails not sending", "Webhooks failing"):
            self.assertIn("#recover-retained-delivery-failures", section(heading, 4))
            self.assertIn("#inspect-notification-logs", section(heading, 4))


if __name__ == "__main__":
    unittest.main()
