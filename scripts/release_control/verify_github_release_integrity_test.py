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
SCRIPT = ROOT / "scripts" / "verify-github-release-integrity.sh"
SOURCE_SHA = "a" * 40


class VerifyGitHubReleaseIntegrityTest(unittest.TestCase):
    def run_verifier(
        self,
        release: dict,
        *,
        verification_succeeds: bool = True,
        asset_verification_succeeds: bool = True,
    ):
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
                    if [ "$1" = api ]; then
                      cat <<'JSON'
                    {json.dumps(release)}
                    JSON
                      exit 0
                    fi
                    if [ "$1 $2" = "release verify" ]; then
                      printf '%s\\n' '{{"verified": true}}'
                      exit {0 if verification_succeeds else 1}
                    fi
                    if [ "$1 $2" = "release download" ]; then
                      while [ "$#" -gt 0 ]; do
                        if [ "$1" = --dir ]; then
                          mkdir -p "$2"
                          printf '%s\\n' '{{"schema_version": 1}}' > "$2/release-activation.json"
                          exit 0
                        fi
                        shift
                      done
                      exit 65
                    fi
                    if [ "$1 $2" = "release verify-asset" ]; then
                      printf '%s\\n' '{{"verified": true}}'
                      exit {0 if asset_verification_succeeds else 1}
                    fi
                    exit 64
                    """
                ),
                encoding="utf-8",
            )
            fake_gh.chmod(0o755)
            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{root}:{env['PATH']}",
                    "PULSE_RELEASE_ATTESTATION_ATTEMPTS": "1",
                    "PULSE_RELEASE_ATTESTATION_RETRY_DELAY": "0",
                }
            )
            result = subprocess.run(
                [str(SCRIPT), "v6.5.0", "rcourtman/Pulse", "123", SOURCE_SHA],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            call_text = calls.read_text(encoding="utf-8") if calls.exists() else ""
            return result, call_text

    @staticmethod
    def release(*, immutable: bool = True) -> dict:
        return {
            "id": 123,
            "tag_name": "v6.5.0",
            "target_commitish": SOURCE_SHA,
            "draft": False,
            "prerelease": False,
            "immutable": immutable,
            "published_at": "2026-08-29T18:00:00Z",
            "assets": [
                {
                    "name": "release-activation.json",
                    "state": "uploaded",
                    "size": 300,
                    "digest": "sha256:" + "b" * 64,
                }
            ],
        }

    def test_accepts_immutable_release_with_verified_attestation(self) -> None:
        result, calls = self.run_verifier(self.release())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("is immutable, attested, and activation-asset-bound", result.stdout)
        self.assertIn("release verify v6.5.0 --repo rcourtman/Pulse --format json", calls)
        self.assertIn("release verify-asset v6.5.0", calls)

    def test_rejects_mutable_release_before_attestation(self) -> None:
        result, calls = self.run_verifier(self.release(immutable=False))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not an immutable published packet", result.stderr)
        self.assertNotIn("release verify", calls)

    def test_rejects_missing_activation_marker(self) -> None:
        release = self.release()
        release["assets"] = []
        result, _ = self.run_verifier(release)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("activation marker", result.stderr)

    def test_rejects_failed_release_attestation(self) -> None:
        result, _ = self.run_verifier(
            self.release(), verification_succeeds=False
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("attestation verification failed", result.stderr)

    def test_rejects_activation_asset_outside_release_attestation(self) -> None:
        result, calls = self.run_verifier(
            self.release(), asset_verification_succeeds=False
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("activation asset verification failed", result.stderr)
        self.assertIn("release verify-asset v6.5.0", calls)


if __name__ == "__main__":
    unittest.main()
