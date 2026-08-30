#!/usr/bin/env python3

from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "verify-release-helm-chart.sh"
SOURCE_SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64


class VerifyReleaseHelmChartTests(unittest.TestCase):
    def run_verifier(
        self,
        *,
        digest: str = DIGEST,
        expected_digest: str = "",
        gh_exit: int = 0,
        gh_version: str = "2.97.0",
        source_sha: str = SOURCE_SHA,
    ) -> tuple[subprocess.CompletedProcess[str], str]:
        with tempfile.TemporaryDirectory() as temp:
            temp_path = Path(temp)
            bin_path = temp_path / "bin"
            bin_path.mkdir()
            gh_log = temp_path / "gh.log"
            (bin_path / "helm").write_text(
                textwrap.dedent(
                    """\
                    #!/bin/sh
                    printf 'Pulled: ghcr.io/rcourtman/pulse-chart/pulse:6.4.1\n'
                    printf 'Digest: %s\n' "$CHART_DIGEST"
                    : > "$6/pulse-6.4.1.tgz"
                    """
                ),
                encoding="utf-8",
            )
            (bin_path / "gh").write_text(
                textwrap.dedent(
                    """\
                    #!/bin/sh
                    if [ "$1" = version ]; then
                      printf 'gh version %s (test)\n' "$GH_VERSION"
                      exit 0
                    fi
                    printf '%s\n' "$*" >> "$GH_LOG"
                    exit "$GH_EXIT"
                    """
                ),
                encoding="utf-8",
            )
            for command in (bin_path / "helm", bin_path / "gh"):
                command.chmod(0o755)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{bin_path}:{env['PATH']}",
                    "CHART_DIGEST": digest,
                    "GH_LOG": str(gh_log),
                    "GH_EXIT": str(gh_exit),
                    "GH_VERSION": gh_version,
                }
            )
            args = [str(SCRIPT), "v6.4.1", source_sha, "rcourtman/Pulse"]
            if expected_digest:
                args.append(expected_digest)
            result = subprocess.run(
                args,
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            return result, gh_log.read_text(encoding="utf-8") if gh_log.exists() else ""

    def test_emits_digest_after_exact_hosted_provenance_verification(self) -> None:
        result, calls = self.run_verifier()

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), f"chart_digest={DIGEST}")
        self.assertIn(f"oci://ghcr.io/rcourtman/pulse-chart/pulse@{DIGEST}", calls)
        self.assertIn("--repo rcourtman/Pulse", calls)
        self.assertIn("--bundle-from-oci", calls)
        self.assertIn(
            "--signer-workflow github.com/rcourtman/Pulse/.github/workflows/publish-helm-chart.yml",
            calls,
        )
        self.assertIn(f"--source-digest {SOURCE_SHA}", calls)
        self.assertIn("--deny-self-hosted-runners", calls)
        self.assertIn("--predicate-type https://slsa.dev/provenance/v1", calls)

    def test_rejects_a_chart_that_moved_after_activation(self) -> None:
        expected = "sha256:" + "c" * 64
        result, calls = self.run_verifier(expected_digest=expected)

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Exact-version Helm chart moved", result.stderr)
        self.assertEqual(calls, "")

    def test_rejects_missing_or_malformed_registry_digest(self) -> None:
        result, calls = self.run_verifier(digest="unknown")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("did not report one exact OCI digest", result.stderr)
        self.assertEqual(calls, "")

    def test_rejects_unsafe_github_cli_before_registry_resolution(self) -> None:
        result, calls = self.run_verifier(gh_version="2.96.1")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("too old for release attestation policy enforcement", result.stderr)
        self.assertEqual(calls, "")

    def test_rejects_invalid_source_identity(self) -> None:
        result, calls = self.run_verifier(source_sha="main")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid release source SHA", result.stderr)
        self.assertEqual(calls, "")


if __name__ == "__main__":
    unittest.main()
