#!/usr/bin/env python3

from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "require-safe-gh-attestation.sh"


class RequireSafeGitHubAttestationTest(unittest.TestCase):
    def run_check(self, version_line: str, *, exit_code: int = 0):
        with tempfile.TemporaryDirectory() as directory:
            fake_gh = Path(directory) / "gh"
            fake_gh.write_text(
                textwrap.dedent(
                    f"""\
                    #!/bin/sh
                    printf '%s\\n' {version_line!r}
                    exit {exit_code}
                    """
                ),
                encoding="utf-8",
            )
            fake_gh.chmod(0o755)
            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{directory}:{env['PATH']}",
                }
            )
            return subprocess.run(
                [str(SCRIPT)],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )

    def test_accepts_floor_and_newer_major_version(self) -> None:
        for version in ("2.97.0", "2.101.3", "3.0.0"):
            with self.subTest(version=version):
                result = self.run_check(f"gh version {version} (test)")
                self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_versions_before_safe_matcher_fix(self) -> None:
        for version in ("2.96.9", "2.9.99", "1.120.0"):
            with self.subTest(version=version):
                result = self.run_check(f"gh version {version} (test)")
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("too old", result.stderr)

    def test_rejects_unparseable_versions(self) -> None:
        malformed = self.run_check("github cli current")
        self.assertNotEqual(malformed.returncode, 0)
        self.assertIn("Unable to determine", malformed.stderr)

    def test_rejects_a_broken_github_cli(self) -> None:
        result = self.run_check("", exit_code=1)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unable to run the GitHub CLI", result.stderr)


if __name__ == "__main__":
    unittest.main()
