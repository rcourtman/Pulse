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
        gh_fail_first: int = 0,
        docker_fail_first: int = 0,
        attempts: str = "4",
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
                    # A transient registry fault prints a half answer and fails, as a dropped
                    # connection does; the first DOCKER_FAIL_FIRST calls behave that way.
                    n=$(cat "$DOCKER_COUNT" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" > "$DOCKER_COUNT"
                    if [ "$n" -le "$DOCKER_FAIL_FIRST" ]; then
                      printf '{"digest":"sha256:bbbb'
                      echo "ERROR: failed to copy: unexpected status 502 Bad Gateway" >&2
                      exit 1
                    fi
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
                    n=$(wc -l < "$GH_LOG")
                    if [ "$n" -le "$GH_FAIL_FIRST" ]; then
                      echo "ERROR: failed to copy: unexpected status 502 Bad Gateway" >&2
                      exit 1
                    fi
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
                    "GH_FAIL_FIRST": str(gh_fail_first),
                    "DOCKER_FAIL_FIRST": str(docker_fail_first),
                    "DOCKER_COUNT": str(temp_path / "docker.count"),
                    "VERIFY_REGISTRY_ATTEMPTS": attempts,
                    "VERIFY_REGISTRY_WAIT_SECONDS": "0",
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
        # A real failure is identical on every attempt: it is retried, then it still fails.
        result, calls = self.run_verifier(gh_exit=1)

        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls.splitlines()), 4)
        self.assertIn("registry read failed after 4 attempts", result.stderr)
        self.assertEqual(result.stdout, "")

    def test_retries_transient_attestation_reads_and_emits_only_the_proof(self) -> None:
        result, calls = self.run_verifier(gh_fail_first=2)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout.splitlines(),
            [f"server_digest={DIGEST}", f"control_plane_digest={DIGEST}"],
        )
        # Two failed reads, then every one of the four subjects verified.
        self.assertEqual(len(calls.splitlines()), 6)
        self.assertEqual(result.stderr.count("retrying in 0s"), 2)

    def test_a_failed_digest_read_cannot_leak_partial_output_into_the_proof(self) -> None:
        result, _ = self.run_verifier(docker_fail_first=3)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout.splitlines(),
            [f"server_digest={DIGEST}", f"control_plane_digest={DIGEST}"],
        )
        self.assertNotIn("bbbb\n", result.stdout.replace(DIGEST, ""))
        self.assertEqual(result.stderr.count("registry read failed on attempt"), 3)

    def test_gives_up_after_the_bounded_number_of_digest_attempts(self) -> None:
        result, calls = self.run_verifier(docker_fail_first=99, attempts="3")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("registry read failed after 3 attempts", result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertEqual(calls, "")

    def test_rejects_unbounded_retry_settings(self) -> None:
        for attempts in ("0", "100", "x", "-1", "2.5"):
            with self.subTest(attempts=attempts):
                result, calls = self.run_verifier(attempts=attempts)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertEqual(calls, "")

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
