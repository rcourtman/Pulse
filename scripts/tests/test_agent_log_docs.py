#!/usr/bin/env python3
"""Exercise the copied agent log reader against synthetic journal processes.

No real journal, service, credential, agent or runtime is accessed. These
controls establish recipe bounds and failure handling, not native recovery.
"""

from pathlib import Path
import re
import subprocess
import unittest

from test_troubleshooting_logs import exercise_log_recipe


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/UNIFIED_AGENT.md"
LOG = "Podman runtime detected\n"
PRIVATE_ERROR = "synthetic-private-endpoint-and-token\n"


def section(heading):
    text = DOC.read_text(encoding="utf-8")
    marker = "### " + heading + "\n"
    if marker not in text:
        raise AssertionError(f"missing section: {heading}")
    return re.split(r"\n#{2,3} ", text.split(marker, 1)[1], 1)[0]


def recipe():
    blocks = re.findall(r"```bash\n(.*?)```", section("Collect agent logs safely"), re.DOTALL)
    if len(blocks) != 1:
        raise AssertionError("expected one shared systemd journal reader")
    return blocks[0]


class AgentLogDocsTest(unittest.TestCase):
    def exercise(self, **settings):
        return exercise_log_recipe("journalctl", recipe(), **settings)

    def test_shipped_guide_matches_source(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/UNIFIED_AGENT.md").read_bytes())

    def test_copied_reader_is_bounded_and_read_only(self):
        copied = recipe()
        subprocess.run(["bash", "-n", "-c", copied], check=True, capture_output=True)
        result, argv = self.exercise(stdout=LOG)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, LOG)
        self.assertEqual(result.stderr, "")
        self.assertEqual(argv, ["-u", "pulse-agent.service", "--since", "15 minutes ago",
                                "--lines", "200", "--no-pager", "--output=cat"])
        self.assertIn("timeout --signal=TERM --kill-after=1s 8s", copied)
        self.assertIn("2>&1", copied)
        self.assertNotRegex(copied, r"\b(?:grep|curl|docker|systemctl|printenv|sudo)\b|--follow|--token")

    def test_both_streams_survive_success_without_claiming_redaction(self):
        result, _ = self.exercise(stdout=LOG, stderr=PRIVATE_ERROR)
        self.assertEqual(result.returncode, 0, result.stderr)
        # Independently buffered streams do not establish a total order.
        self.assertCountEqual(result.stdout.splitlines(), (LOG + PRIVATE_ERROR).splitlines())
        self.assertEqual(result.stderr, "")

    def test_reader_failure_withholds_partial_logs_and_keeps_exit(self):
        for output in ("", LOG):
            with self.subTest(partial=bool(output)):
                result, _ = self.exercise(stdout=output, stderr=PRIVATE_ERROR, exit_code=2)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, "")
                self.assertIn("Agent log read unavailable (exit 2)", result.stderr)
                self.assertNotIn(PRIVATE_ERROR.strip(), result.stderr)

    def test_hung_reader_is_stopped_by_the_documented_deadline(self):
        result, _ = self.exercise(stdout=LOG, stderr=PRIVATE_ERROR, hang="term")
        self.assertEqual(result.returncode, 124, result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn("Agent log read unavailable (exit 124)", result.stderr)
        self.assertNotIn(PRIVATE_ERROR.strip(), result.stderr)

    def test_reader_ignoring_term_is_killed_after_the_documented_grace(self):
        result, _ = self.exercise(stdout=LOG, hang="ignore-term")
        self.assertEqual(result.returncode, 137, result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn("Agent log read unavailable (exit 137)", result.stderr)

    def test_missing_timeout_does_not_start_the_reader(self):
        result, argv = self.exercise(missing_timeout=True)
        self.assertIsNone(argv)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, "")
        self.assertIn("no unbounded fallback", result.stderr)

    def test_successful_empty_read_is_inconclusive(self):
        result, _ = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout + result.stderr, "")
        self.assertIn("successful empty read is inconclusive", section("Collect agent logs safely"))

    def test_shared_reader_preserves_scope_and_private_output_boundaries(self):
        text = " ".join(section("Collect agent logs safely").split())
        for boundary in ("Linux systemd agent host", "already authorised", "affected agent host",
                         "not the Pulse server", "actual service name", "original time window",
                         "does not contact Pulse", "partial output is withheld", "not sanitised",
                         "manually redacted", "Tokens, cookies, URLs", "do not post the full excerpt",
                         "service environment, connection files or saved identity",
                         "broaden permissions", "Do not restart, re-enrol or enable debug logging",
                         "container agent or a non-systemd host", "evidence as unavailable"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, text)

    def test_diagnostic_entry_points_reuse_the_reader_without_live_followers(self):
        text = DOC.read_text(encoding="utf-8")
        log_level = text.split("### Agent log level\n", 1)[1].split("\n## ", 1)[0]
        for entry in (log_level, section("Agent Not Updating"),
                      section("Commands enabled but remote control blocked"),
                      section("Docker Swarm Not Detected")):
            self.assertIn("[bounded agent log reader](#collect-agent-logs-safely)", entry)
            self.assertNotIn("journalctl", entry)
        self.assertIn("not evidence-collection steps", log_level)
        self.assertIn("restart changes the run", log_level)
        self.assertNotRegex(text, r"journalctl[^\n]*(?:\s-f\b|--follow|\|\s*grep)")
        swarm = " ".join(section("Docker Swarm Not Detected").split())
        self.assertIn("An empty search does not establish", swarm)
        self.assertIn("not enable debug logging, restart the agent or change its runtime", swarm)


if __name__ == "__main__":
    unittest.main()
