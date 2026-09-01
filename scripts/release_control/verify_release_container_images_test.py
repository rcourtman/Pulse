#!/usr/bin/env python3

from __future__ import annotations

import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "verify-release-container-images.sh"
SOURCE_SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64


class VerifyReleaseContainerImagesTests(unittest.TestCase):
    def run_verifier(
        self,
        *,
        overrides: dict[str, str] | None = None,
        gh_exit: int = 0,
        gh_version: str = "2.97.0",
        tag: str = "v6.4.1",
        source_sha: str = SOURCE_SHA,
    ) -> tuple[subprocess.CompletedProcess[str], str]:
        with tempfile.TemporaryDirectory() as temp:
            temp_path = Path(temp)
            bin_path = temp_path / "bin"
            bin_path.mkdir()
            gh_log = temp_path / "gh.log"
            digest_file = temp_path / "digests"
            values = {
                "docker.io/rcourtman/pulse:v6.4.1": DIGEST,
                "docker.io/rcourtman/pulse:6.4.1": DIGEST,
                "ghcr.io/rcourtman/pulse:v6.4.1": DIGEST,
                "ghcr.io/rcourtman/pulse:6.4.1": DIGEST,
                "docker.io/rcourtman/pulse-control-plane:v6.4.1": DIGEST,
                "docker.io/rcourtman/pulse-control-plane:6.4.1": DIGEST,
                "ghcr.io/rcourtman/pulse-control-plane:v6.4.1": DIGEST,
                "ghcr.io/rcourtman/pulse-control-plane:6.4.1": DIGEST,
            }
            values.update(overrides or {})
            digest_file.write_text(
                "\n".join(f"{reference} {digest}" for reference, digest in values.items())
                + "\n",
                encoding="utf-8",
            )
            (bin_path / "docker").write_text(
                textwrap.dedent(
                    """\
                    #!/bin/sh
                    reference="$4"
                    digest=$(awk -v ref="$reference" '$1 == ref { print $2 }' "$DIGEST_FILE")
                    [ -n "$digest" ] || exit 1
                    printf '{"digest":"%s"}\n' "$digest"
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
            for command in (bin_path / "docker", bin_path / "gh"):
                command.chmod(0o755)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{bin_path}:{env['PATH']}",
                    "DIGEST_FILE": str(digest_file),
                    "GH_LOG": str(gh_log),
                    "GH_EXIT": str(gh_exit),
                    "GH_VERSION": gh_version,
                }
            )
            result = subprocess.run(
                [str(SCRIPT), tag, source_sha, "rcourtman/Pulse"],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            return result, gh_log.read_text(encoding="utf-8") if gh_log.exists() else ""

    def test_emits_digest_proof_after_verifying_both_registries(self) -> None:
        result, calls = self.run_verifier()

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout.splitlines(),
            [f"server_digest={DIGEST}", f"control_plane_digest={DIGEST}"],
        )
        subjects = {
            f"oci://docker.io/rcourtman/pulse@{DIGEST}",
            f"oci://ghcr.io/rcourtman/pulse@{DIGEST}",
            f"oci://docker.io/rcourtman/pulse-control-plane@{DIGEST}",
            f"oci://ghcr.io/rcourtman/pulse-control-plane@{DIGEST}",
        }
        expected_policy = [
            "--repo",
            "rcourtman/Pulse",
            "--bundle-from-oci",
            "--signer-workflow",
            "github.com/rcourtman/Pulse/.github/workflows/publish-docker.yml",
            "--signer-digest",
            SOURCE_SHA,
            "--source-digest",
            SOURCE_SHA,
            "--deny-self-hosted-runners",
            "--predicate-type",
            "https://slsa.dev/provenance/v1",
        ]
        invocations = [shlex.split(call) for call in calls.splitlines()]
        self.assertEqual(len(invocations), len(subjects))
        self.assertEqual({invocation[2] for invocation in invocations}, subjects)
        for invocation in invocations:
            self.assertEqual(
                invocation,
                ["attestation", "verify", invocation[2], *expected_policy],
                f"incomplete attestation policy for {invocation[2]}",
            )

    def test_rejects_a_moved_exact_version_tag_before_attestation(self) -> None:
        changed = "sha256:" + "c" * 64
        result, calls = self.run_verifier(
            overrides={"ghcr.io/rcourtman/pulse:6.4.1": changed}
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("do not resolve to one digest", result.stderr)
        self.assertEqual(calls, "")

    def test_rejects_an_unverifiable_attestation(self) -> None:
        result, calls = self.run_verifier(gh_exit=1)

        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls.splitlines()), 1)

    def test_rejects_an_unsafe_github_cli_before_registry_calls(self) -> None:
        result, calls = self.run_verifier(gh_version="2.96.1")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("too old for release attestation policy enforcement", result.stderr)
        self.assertEqual(calls, "")

    def test_rejects_invalid_release_identity_without_registry_calls(self) -> None:
        result, calls = self.run_verifier(source_sha="main")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid release source SHA", result.stderr)
        self.assertEqual(calls, "")


if __name__ == "__main__":
    unittest.main()
