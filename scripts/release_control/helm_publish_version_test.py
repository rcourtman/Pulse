#!/usr/bin/env python3
"""Exercise hosted Helm version resolution without packaging or publication."""
from pathlib import Path
import os
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]


class HelmPublishVersionTests(unittest.TestCase):
    def resolve(self, chart="", app="", tag=""):
        workflow = (ROOT / ".github/workflows/publish-helm-chart.yml").read_text()
        step = workflow.split("      - name: Determine chart version\n", 1)[1]
        script = textwrap.dedent(step.split("        run: |\n", 1)[1].split(
            "      - name:", 1)[0])
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "output"
            result = subprocess.run(
                ["bash", "-euo", "pipefail", "-c", script], cwd=ROOT,
                env={"PATH": os.environ["PATH"], "INPUT_CHART_VERSION": chart,
                     "INPUT_APP_VERSION": app, "RELEASE_TAG_NAME": tag,
                     "GITHUB_OUTPUT": str(output)},
                capture_output=True, text=True)
            return result, output.read_text() if output.exists() else ""

    def test_matching_and_default_application_versions(self):
        for version in ("6.4.3", "6.4.3-rc.2", "6.5.0-beta.1", "6.5.0-alpha.1"):
            for app in ("", version):
                with self.subTest(version=version, app=app):
                    result, output = self.resolve(version, app)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertIn("\n" + version + "\n", output)
                    self.assertIn("is_prerelease=" + ("true" if "-" in version else "false"), output)

    def test_release_event_uses_tag(self):
        result, output = self.resolve(tag="v6.4.3-rc.2")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("\n6.4.3-rc.2\n", output)

    def test_mismatched_application_rejected_before_outputs(self):
        for chart, app in (("6.4.3", "6.4.2"), ("6.4.3", "6.4.3-rc.2"),
                           ("6.4.3-rc.2", "6.4.3"), ("6.4.3", "latest")):
            with self.subTest(chart=chart, app=app):
                result, output = self.resolve(chart, app)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(output, "")

    def test_missing_version_rejected(self):
        result, output = self.resolve()
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(output, "")


if __name__ == "__main__":
    unittest.main()
