#!/usr/bin/env python3
"""Execute the chart publication shell with a fake GitHub CLI; no network/writes."""
from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
CHART = b"qualified chart fixture"
DIGEST = "sha256:" + hashlib.sha256(CHART).hexdigest()


class HelmPagesRetryTests(unittest.TestCase):
    def run_publication(self, *, draft=False, digest=DIGEST, prerelease=False,
                        version="6.4.3", exists=True):
        workflow = (ROOT / ".github/workflows/helm-pages.yml").read_text()
        step = workflow.split("      - name: Publish chart release and merge Pages index\n", 1)[1]
        script = textwrap.dedent(step.split("        run: |\n", 1)[1].split(
            "          git -C gh-pages config", 1)[0])
        script = script.replace("${{ github.repository }}", "test/pulse")
        script += '\nprintf "PAGES_BOUNDARY\\n"\n'
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "dist").mkdir()
            (root / "dist" / f"pulse-{version}.tgz").write_bytes(CHART)
            fake = root / "gh"
            fake.write_text(textwrap.dedent('''\
                #!/usr/bin/env python3
                import json, os, subprocess, sys
                args = sys.argv[1:]
                with open("calls.jsonl", "a") as log:
                    log.write(json.dumps(args) + "\\n")
                if args[:2] == ["release", "view"]:
                    if os.environ["EXISTS"] == "false":
                        sys.exit(1)
                    payload = {"isDraft": json.loads(os.environ["DRAFT"]),
                               "isPrerelease": json.loads(os.environ["PRERELEASE"])}
                elif args[0] == "api":
                    payload = {"assets": [{"name": "pulse-" + os.environ["VERSION"] + ".tgz",
                                           "digest": os.environ["DIGEST"]}]}
                else:
                    sys.exit(0)
                result = subprocess.run(["jq", "-r", args[args.index("--jq") + 1]],
                                        input=json.dumps(payload), text=True)
                sys.exit(result.returncode)
                '''))
            fake.chmod(0o755)
            env = {"PATH": f"{root}:{os.environ['PATH']}", "VERSION": version,
                   "GITHUB_REPOSITORY": "test/pulse", "TARGET_COMMITISH": "a" * 40,
                   "DRAFT": json.dumps(draft), "PRERELEASE": json.dumps(prerelease),
                   "DIGEST": digest, "EXISTS": json.dumps(exists)}
            result = subprocess.run(["bash", "-c", script], cwd=root, env=env,
                                    capture_output=True, text=True)
            calls = [json.loads(line) for line in (root / "calls.jsonl").read_text().splitlines()]
            return result, calls

    def test_draft_with_matching_asset_cannot_reach_pages(self):
        result, calls = self.run_publication(draft=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("PAGES_BOUNDARY", result.stdout)
        self.assertFalse(any(c[:2] in (["release", "edit"], ["release", "upload"],
                                     ["release", "create"]) for c in calls))

    def test_draft_without_asset_is_not_modified(self):
        result, calls = self.run_publication(draft=True, digest="")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 1)

    def test_unknown_draft_state_fails_closed(self):
        result, _ = self.run_publication(draft=None)
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("PAGES_BOUNDARY", result.stdout)

    def test_published_matching_chart_retry_is_read_only(self):
        result, calls = self.run_publication()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("PAGES_BOUNDARY", result.stdout)
        self.assertEqual(len(calls), 2)

    def test_published_mismatched_chart_is_not_replaced(self):
        result, calls = self.run_publication(digest="sha256:" + "c" * 64)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls), 2)
        self.assertNotIn("PAGES_BOUNDARY", result.stdout)

    def test_published_stable_classification_is_corrected(self):
        result, calls = self.run_publication(prerelease=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls[-1][:2], ["release", "edit"])
        self.assertIn("--prerelease=false", calls[-1])
        self.assertIn("--latest=false", calls[-1])

    def test_new_stable_and_preview_classification(self):
        for version, expected in (("6.4.3", "false"), ("6.4.3-rc.2", "true")):
            with self.subTest(version=version):
                result, calls = self.run_publication(exists=False, version=version)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls[-1][:2], ["release", "create"])
                self.assertIn(f"--prerelease={expected}", calls[-1])
                self.assertIn("--latest=false", calls[-1])


if __name__ == "__main__":
    unittest.main()
