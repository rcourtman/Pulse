#!/usr/bin/env python3
"""Tests for scripts/sync_chart_release_metadata.py."""

from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).resolve().parents[1] / "sync_chart_release_metadata.py"
SPEC = importlib.util.spec_from_file_location("sync_chart_release_metadata", MODULE_PATH)
assert SPEC is not None and SPEC.loader is not None
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class SyncChartReleaseMetadataTest(unittest.TestCase):
    def test_sync_chart_metadata_updates_versioned_links(self) -> None:
        original = """apiVersion: v2
name: pulse
version: 5.1.7
appVersion: "5.1.7"
icon: https://raw.githubusercontent.com/rcourtman/Pulse/main/docs/images/pulse-logo.svg
annotations:
  artifacthub.io/links: |
    - name: Documentation
      url: https://github.com/rcourtman/Pulse/blob/main/docs/KUBERNETES.md
    - name: Support
      url: https://github.com/rcourtman/Pulse/discussions
"""

        updated = MODULE.sync_chart_metadata(original, "6.0.0-rc.1", "example/pulse")

        self.assertIn("version: 6.0.0-rc.1", updated)
        self.assertIn('appVersion: "6.0.0-rc.1"', updated)
        self.assertIn(
            "icon: https://raw.githubusercontent.com/example/pulse/v6.0.0-rc.1/docs/images/pulse-logo.svg",
            updated,
        )
        self.assertIn(
            "url: https://github.com/example/pulse/blob/v6.0.0-rc.1/docs/KUBERNETES.md",
            updated,
        )


if __name__ == "__main__":
    unittest.main()
