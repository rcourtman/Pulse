#!/usr/bin/env python3

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "check-github-release-immutability.sh"


class CheckGitHubReleaseImmutabilityTest(unittest.TestCase):
    def run_check(
        self,
        response: object = None,
        *,
        api_succeeds: bool = True,
        include_token: bool = True,
        repository: str = "rcourtman/Pulse",
    ):
        if response is None:
            response = {"enabled": True, "enforced_by_owner": False}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            calls = root / "calls"
            fake_gh = root / "gh"
            fake_gh.write_text(
                textwrap.dedent(
                    f"""\
                    #!/usr/bin/env bash
                    set -euo pipefail
                    printf '%s\\n' "$*" >> {calls!s}
                    cat <<'JSON'
                    {json.dumps(response)}
                    JSON
                    exit {0 if api_succeeds else 1}
                    """
                ),
                encoding="utf-8",
            )
            fake_gh.chmod(0o755)
            env = os.environ.copy()
            env["PATH"] = f"{root}:{env['PATH']}"
            if include_token:
                env["GH_TOKEN"] = "test-token"
            else:
                env.pop("GH_TOKEN", None)
            result = subprocess.run(
                [str(SCRIPT), repository],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            call_text = calls.read_text(encoding="utf-8") if calls.exists() else ""
            return result, call_text

    def test_accepts_enabled_repository_setting(self) -> None:
        result, calls = self.run_check()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("immutable releases are enabled", result.stdout)
        self.assertIn("repos/rcourtman/Pulse/immutable-releases", calls)
        self.assertIn("X-GitHub-Api-Version: 2026-03-10", calls)

    def test_rejects_disabled_repository_setting(self) -> None:
        result, _ = self.run_check(
            {"enabled": False, "enforced_by_owner": False}
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("refusing to cross the publication boundary", result.stderr)

    def test_rejects_unavailable_or_unauthorized_setting(self) -> None:
        result, _ = self.run_check(api_succeeds=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("may be disabled or the token may lack", result.stderr)

    def test_rejects_malformed_success_response(self) -> None:
        result, _ = self.run_check({"enforced_by_owner": False})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not enabled", result.stderr)

        result, _ = self.run_check(
            {"enabled": True, "enforced_by_owner": "false"}
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not enabled", result.stderr)

    def test_requires_explicit_administration_read_token(self) -> None:
        result, calls = self.run_check(include_token=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Administration (read) is required", result.stderr)
        self.assertEqual(calls, "")

    def test_rejects_invalid_repository_before_api_call(self) -> None:
        result, calls = self.run_check(repository="not-a-repository")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid GitHub repository", result.stderr)
        self.assertEqual(calls, "")


if __name__ == "__main__":
    unittest.main()
