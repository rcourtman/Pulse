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


if __name__ == "__main__":
    unittest.main()
