#!/usr/bin/env python3
"""Keep storage help from turning write-pressure diagnosis into history loss.

These are documentation safety checks, not a database migration or proof of
installed persistence. The normal documentation/smoke harness discovers them.
"""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDES = ("DOCKER.md", "CONFIGURATION.md", "METRICS_HISTORY.md")


def volatile_metrics_recipes(source):
    blocks = re.findall(r"```[^\n]*\n(.*?)```", source, re.S)
    return [block for block in blocks if re.search(
        r"PULSE_METRICS_DB_PATH\s*[:=]\s*[\"']?/(?:dev/shm|mnt/ramdisk|metrics-tmpfs)/",
        block,
    )]


class MetricsStorageDocsTest(unittest.TestCase):
    def test_all_three_shipped_guides_match_the_canonical_documents(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                self.assertEqual((ROOT / "docs" / name).read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / name).read_bytes())

    def test_copyable_examples_do_not_move_history_to_volatile_storage(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                self.assertEqual(volatile_metrics_recipes((ROOT / "docs" / name).read_text()), [])
        # Cover the shell assignment and Compose forms that previously shipped.
        for example in (
            "```bash\nPULSE_METRICS_DB_PATH=/dev/shm/pulse/metrics.db\n```",
            "```bash\nPULSE_METRICS_DB_PATH=/mnt/ramdisk/pulse/metrics.db\n```",
            "```yaml\nenvironment:\n  PULSE_METRICS_DB_PATH: /metrics-tmpfs/metrics.db\n```",
        ):
            with self.subTest(example=example):
                self.assertEqual(len(volatile_metrics_recipes(example)), 1)

    def test_storage_explanation_distinguishes_the_three_lifetimes(self):
        text = (ROOT / "docs/METRICS_HISTORY.md").read_text().split(
            "## Advanced: Disk Write Tuning\n", 1)[1].split("\n## ", 1)[0]
        rows = re.findall(r"^\| (.+?) \| (.+?) \|$", text, re.M)
        self.assertEqual(len(rows), 4)  # Header and the three storage types.
        persistent, writable, ram = [description for _, description in rows[1:]]
        self.assertIn("same storage remains attached", persistent)
        self.assertIn("persistence is not a backup", persistent)
        self.assertIn("process restart can retain it", writable)
        self.assertIn("container recreation discards it", writable)
        self.assertIn("when that mount is discarded", ram)
        self.assertIn("service restart alone is not guaranteed", ram)
        normal = " ".join(text.split())
        for boundary in ("supported storage-location override", "default organization",
                         "does not migrate the existing history or merge stores", "Keep the original store intact",
                         "Do not restart, recreate a container, switch paths again",
                         "copy an open database", "Do not post configuration",
                         "cannot recover observations lost"):
            with self.subTest(boundary=boundary):
                self.assertIn(boundary, normal)

    def test_entry_points_preserve_the_override_and_link_to_storage_checks(self):
        for name in ("DOCKER.md", "CONFIGURATION.md"):
            text = (ROOT / "docs" / name).read_text()
            with self.subTest(guide=name):
                self.assertIn("PULSE_METRICS_DB_PATH", text)
                self.assertIn("METRICS_HISTORY.md#advanced-disk-write-tuning", text)
                self.assertIn("persistent volume", text)
        history = (ROOT / "docs/METRICS_HISTORY.md").read_text()
        self.assertIn("## Advanced: Disk Write Tuning", history)
        self.assertIn("TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth", history)
        self.assertIn("TROUBLESHOOTING.md#-getting-help", history)


if __name__ == "__main__":
    unittest.main()
