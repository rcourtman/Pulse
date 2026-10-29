#!/usr/bin/env python3
"""Keep the optional Docker healthcheck free of unqualified resource caps.

This validates the copyable configuration and its safety context, not Pulse's
memory requirement, Docker enforcement or reporter recovery.
"""

import json
from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[2]


def section():
    return (ROOT / "docs/DOCKER.md").read_text().split(
        "### Healthcheck and resource sizing\n", 1
    )[1].split("\n---", 1)[0]


class DockerResourceSizingDocsTest(unittest.TestCase):
    def test_shipped_document_matches_the_source(self):
        self.assertEqual((ROOT / "docs/DOCKER.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/DOCKER.md").read_bytes())

    def test_healthcheck_does_not_apply_arbitrary_cpu_or_memory_caps(self):
        blocks = re.findall(r"```yaml\n(.*?)```", section(), re.S)
        self.assertEqual(len(blocks), 1)
        self.assertEqual(blocks[0].splitlines()[:3],
                         ["services:", "  pulse:", "    healthcheck:"])
        self.assertNotRegex(blocks[0], r"(?m)^\s*(deploy|resources|limits|cpus|memory|mem_limit):")
        test = re.search(r"(?m)^      test: (.*)$", blocks[0]).group(1)
        self.assertEqual(json.loads(test),
                         ["CMD", "wget", "--spider", "-q", "http://localhost:7655/api/health"])
        self.assertIn("      interval: 30s\n      timeout: 10s\n      retries: 3", blocks[0])

    def test_limits_and_health_are_not_sold_as_monitoring_recovery(self):
        text = section()
        for fact in ("not a complete", "not restart the container", "no universal CPU or memory limit",
                     "not an instruction to remove an existing limit", "current resource policy",
                     "out-of-memory kill", "delay polling and notification delivery",
                     "Neither repairs an increased baseline", "not a safe container limit by itself",
                     "existing screenshot or unavailable reading is valid evidence",
                     "Do not restart, induce load, shorten retention or slow polling",
                     "even when memory no longer grows"):
            self.assertIn(fact, text)

    def test_safe_measurement_links_exist_in_the_actual_help(self):
        source = (ROOT / "docs/TROUBLESHOOTING.md").read_text()
        for anchor, heading in (
            ("excessive-cpu-writes-or-database-growth", "#### Excessive CPU, writes or database growth"),
            ("memory-use-keeps-growing", "#### Memory use keeps growing"),
        ):
            self.assertIn(f"TROUBLESHOOTING.md#{anchor}", section())
            self.assertIn(heading, source)

    def test_image_storage_is_not_sold_as_lightweight_or_persistent_data(self):
        guide = (ROOT / "docs/DOCKER.md").read_text()
        self.assertNotIn("lightweight", guide.split("\n## ", 1)[0])
        text = guide.split("## Image size and disk use\n", 1)[1].split("\n## ", 1)[0]
        for fact in ("compressed layers", "cached layers", "Docker version and image store",
                     "shared between images and tags", "Container writable-layer size",
                     "Mounted volume or bind-mount contents", "Persistent configuration, credentials",
                     "replacing an image does not shrink or back up this data",
                     "GB** (decimal)", "GiB** (binary)", "changes\nits ownership"):
            with self.subTest(fact=fact):
                self.assertIn(fact, text)

    def test_image_comparison_preserves_state_and_uses_existing_evidence(self):
        guide = (ROOT / "docs/DOCKER.md").read_text()
        text = guide.split("## Image size and disk use\n", 1)[1].split("\n## ", 1)[0]
        for fact in ("exact image reference or digest", "linux/amd64", "linux/arm64",
                     "measurement label", "multi-platform image index",
                     "configured image and image ID", "Unknown is\nvalid",
                     "do not pull or build another image", "export full\ninspect/Compose output",
                     "Do not prune images, delete volumes or clear `/data`",
                     "Keep the previous image and consistent data backup",
                     "does not reclaim old images", "without changing retention or restarting"):
            with self.subTest(fact=fact):
                self.assertIn(fact, text)
        # No fresh diagnostic command, broad inspect or cleanup recipe is added.
        self.assertNotIn("```", text)
        self.assertIn("docker inspect pulse --format '{{.Config.Image}} {{.Image}}'", guide)
        self.assertIn("## 🔄 Updates", guide)
        self.assertIn("TROUBLESHOOTING.md#excessive-cpu-writes-or-database-growth", text)


if __name__ == "__main__":
    unittest.main()
