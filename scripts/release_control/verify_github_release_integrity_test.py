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
        provenance_verification_succeeds: bool = True,
        gh_version: str = "2.97.0",
        partial_download_once: bool = False,
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
                    if [ "$1" = version ]; then
                      printf 'gh version %s (test)\\n' "$GH_VERSION"
                      exit 0
                    fi
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
                          if [ "$PARTIAL_DOWNLOAD_ONCE" = true ] && [ ! -e "$DOWNLOAD_STATE" ]; then
                            touch "$DOWNLOAD_STATE"
                            printf '%s\\n' 'abc  pulse-v6.5.0-linux-amd64.tar.gz' > "$2/checksums.txt"
                            exit 1
                          fi
                          if [ -e "$2/release-activation.json" ] || [ -e "$2/checksums.txt" ]; then
                            exit 66
                          fi
                          printf '%s\\n' '{{"schema_version": 1}}' > "$2/release-activation.json"
                          printf '%s\\n' 'abc  pulse-v6.5.0-linux-amd64.tar.gz' > "$2/checksums.txt"
                          if [ "$HAS_PORTABLE_PROVENANCE" = true ]; then
                            printf '%s\\n' '{{"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json"}}' \
                              > "$2/release-build-provenance.sigstore.json"
                          fi
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
                    if [ "$1 $2" = "attestation verify" ]; then
                      exit {0 if provenance_verification_succeeds else 1}
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
                    "PULSE_RELEASE_ATTESTATION_ATTEMPTS": (
                        "2" if partial_download_once else "1"
                    ),
                    "PULSE_RELEASE_ATTESTATION_RETRY_DELAY": "0",
                    "GH_VERSION": gh_version,
                    "PARTIAL_DOWNLOAD_ONCE": str(partial_download_once).lower(),
                    "DOWNLOAD_STATE": str(root / "download-state"),
                    "HAS_PORTABLE_PROVENANCE": str(
                        any(
                            asset.get("name")
                            == "release-build-provenance.sigstore.json"
                            for asset in release.get("assets", [])
                        )
                    ).lower(),
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
    def release(*, immutable: bool = True, portable_provenance: bool = False) -> dict:
        release = {
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
        if portable_provenance:
            release["assets"].append(
                {
                    "name": "release-build-provenance.sigstore.json",
                    "state": "uploaded",
                    "size": 1200,
                    "digest": "sha256:" + "c" * 64,
                }
            )
        return release

    def test_accepts_immutable_release_with_verified_attestation(self) -> None:
        result, calls = self.run_verifier(self.release())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(
            "is immutable, release-attested, activation-asset-bound, and build-provenance-bound",
            result.stdout,
        )
        self.assertIn("release verify v6.5.0 --repo rcourtman/Pulse --format json", calls)
        self.assertIn("release verify-asset v6.5.0", calls)
        self.assertIn("attestation verify ", calls)
        self.assertIn(
            "--signer-workflow github.com/rcourtman/Pulse/.github/workflows/create-release.yml",
            calls,
        )
        self.assertIn(f"--source-digest {SOURCE_SHA}", calls)
        self.assertIn("--deny-self-hosted-runners", calls)
        self.assertIn("--predicate-type https://slsa.dev/provenance/v1", calls)

    def test_prefers_portable_candidate_build_provenance(self) -> None:
        result, calls = self.run_verifier(self.release(portable_provenance=True))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("portable candidate-build provenance", result.stdout)
        self.assertIn(
            "--signer-workflow github.com/rcourtman/Pulse/.github/workflows/build-release-candidate.yml",
            calls,
        )
        self.assertIn("--bundle ", calls)
        self.assertIn("release-build-provenance.sigstore.json", calls)

    def test_rejects_invalid_portable_provenance_asset(self) -> None:
        release = self.release(portable_provenance=True)
        release["assets"][1]["size"] = 0
        result, calls = self.run_verifier(release)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("invalid portable provenance asset", result.stderr)
        self.assertNotIn("release verify", calls)

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
        result, _ = self.run_verifier(self.release(), verification_succeeds=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("attestation verification failed", result.stderr)

    def test_rejects_activation_asset_outside_release_attestation(self) -> None:
        result, calls = self.run_verifier(
            self.release(), asset_verification_succeeds=False
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("activation asset verification failed", result.stderr)
        self.assertIn("release verify-asset v6.5.0", calls)

    def test_rejects_checksum_manifest_without_build_provenance(self) -> None:
        result, calls = self.run_verifier(
            self.release(), provenance_verification_succeeds=False
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum manifest build provenance verification failed", result.stderr)
        self.assertIn("attestation verify", calls)

    def test_recovers_from_a_partial_multi_asset_download(self) -> None:
        result, calls = self.run_verifier(
            self.release(), partial_download_once=True
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls.count("release download"), 2)

    def test_rejects_an_unsafe_github_cli_before_release_lookup(self) -> None:
        result, calls = self.run_verifier(self.release(), gh_version="2.96.1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("too old for release attestation policy enforcement", result.stderr)
        self.assertEqual(calls, "")


if __name__ == "__main__":
    unittest.main()
