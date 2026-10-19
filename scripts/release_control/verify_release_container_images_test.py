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
        failure_diagnostic: str = "ERROR: failed to copy: unexpected status 502 Bad Gateway",
        failure_status: int = 1,
    ) -> tuple[subprocess.CompletedProcess[str], str, list[str], list[str]]:
        with tempfile.TemporaryDirectory() as temp:
            temp_path = Path(temp)
            bin_path = temp_path / "bin"
            bin_path.mkdir()
            gh_log = temp_path / "gh.log"
            docker_log = temp_path / "docker.log"
            sleep_log = temp_path / "sleep.log"
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
                    printf '%s\n' "$*" >> "$DOCKER_LOG"
                    n=$(cat "$DOCKER_COUNT" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" > "$DOCKER_COUNT"
                    if [ "$n" -le "$DOCKER_FAIL_FIRST" ]; then
                      printf '{"digest":"sha256:bbbb'
                      printf '%s\n' "$FAILURE_DIAGNOSTIC" >&2
                      exit "$FAILURE_STATUS"
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
                      printf 'unverified partial attestation\n'
                      printf '%s\n' "$FAILURE_DIAGNOSTIC" >&2
                      exit "$FAILURE_STATUS"
                    fi
                    exit "$GH_EXIT"
                    """
                ),
                encoding="utf-8",
            )
            (bin_path / "sleep").write_text(
                '#!/bin/sh\nprintf \'%s\\n\' "$*" >> "$SLEEP_LOG"\n',
                encoding="utf-8",
            )
            for command in (bin_path / "docker", bin_path / "gh", bin_path / "sleep"):
                command.chmod(0o755)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{bin_path}:{env['PATH']}",
                    "DIGEST_FILE": str(digest_file),
                    "GH_LOG": str(gh_log),
                    "DOCKER_LOG": str(docker_log),
                    "SLEEP_LOG": str(sleep_log),
                    "GH_EXIT": str(gh_exit),
                    "GH_VERSION": gh_version,
                    "GH_FAIL_FIRST": str(gh_fail_first),
                    "DOCKER_FAIL_FIRST": str(docker_fail_first),
                    "DOCKER_COUNT": str(temp_path / "docker.count"),
                    "FAILURE_DIAGNOSTIC": failure_diagnostic,
                    "FAILURE_STATUS": str(failure_status),
                    "VERIFY_REGISTRY_ATTEMPTS": attempts,
                    "VERIFY_REGISTRY_WAIT_SECONDS": "10",
                    "TMPDIR": str(temp_path),
                }
            )
            result = subprocess.run(
                [str(SCRIPT), tag, source_sha, "rcourtman/Pulse"],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
                timeout=10,
            )
            self.assertEqual(list(temp_path.glob("pulse-registry-read.*")), [],
                             "private diagnostic staging was not cleaned up")
            return (
                result,
                gh_log.read_text(encoding="utf-8") if gh_log.exists() else "",
                docker_log.read_text(encoding="utf-8").splitlines() if docker_log.exists() else [],
                sleep_log.read_text(encoding="utf-8").splitlines() if sleep_log.exists() else [],
            )

    def test_emits_digest_proof_after_verifying_both_registries(self) -> None:
        result, calls, docker, waits = self.run_verifier()

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
        self.assertEqual(len(docker), 8)
        self.assertEqual(waits, [])
        self.assertEqual({invocation[2] for invocation in invocations}, subjects)
        for invocation in invocations:
            self.assertEqual(
                invocation,
                ["attestation", "verify", invocation[2], *expected_policy],
                f"incomplete attestation policy for {invocation[2]}",
            )

    def test_rejects_a_moved_exact_version_tag_before_attestation(self) -> None:
        changed = "sha256:" + "c" * 64
        result, calls, _, waits = self.run_verifier(
            overrides={"ghcr.io/rcourtman/pulse:6.4.1": changed}
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("do not resolve to one digest", result.stderr)
        self.assertEqual(calls, "")
        self.assertEqual(waits, [])

    def test_rejects_an_unverifiable_attestation(self) -> None:
        # An unexplained verification failure is not a transient registry fault.
        result, calls, docker, waits = self.run_verifier(gh_exit=1)

        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(len(calls.splitlines()), 1)
        self.assertEqual(len(docker), 4)
        self.assertIn("not retrying", result.stderr)
        self.assertEqual(waits, [])
        self.assertEqual(result.stdout, "")

    def test_retries_transient_attestation_reads_and_emits_only_the_proof(self) -> None:
        result, calls, _, waits = self.run_verifier(gh_fail_first=2)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout.splitlines(),
            [f"server_digest={DIGEST}", f"control_plane_digest={DIGEST}"],
        )
        # Two failed reads, then every one of the four subjects verified.
        self.assertEqual(len(calls.splitlines()), 6)
        self.assertEqual(waits, ["10", "20"])
        self.assertNotIn("partial attestation", result.stdout)

    def test_a_failed_digest_read_cannot_leak_partial_output_into_the_proof(self) -> None:
        result, _, docker, waits = self.run_verifier(docker_fail_first=3)

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout.splitlines(),
            [f"server_digest={DIGEST}", f"control_plane_digest={DIGEST}"],
        )
        self.assertNotIn("bbbb\n", result.stdout.replace(DIGEST, ""))
        self.assertEqual(result.stderr.count("registry read failed on attempt"), 3)
        self.assertEqual(docker[:4], [docker[0]] * 4)
        self.assertEqual(waits, ["10", "20", "40"])

    def test_gives_up_after_the_bounded_number_of_digest_attempts(self) -> None:
        result, calls, docker, waits = self.run_verifier(docker_fail_first=99, attempts="3")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("registry read failed after 3 attempts", result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertEqual(calls, "")
        self.assertEqual(docker, [docker[0]] * 3)
        self.assertEqual(waits, ["10", "20"])

    def test_access_refusals_stop_without_retry_or_another_reference(self) -> None:
        refusals = (
            "ERROR: pull access denied, repository does not exist or may require authorization: "
            "server message: insufficient_scope: authorization failed",
            "gh: Forbidden (HTTP 403)",
            "unauthorized: authentication required",
            "denied: requested access to the resource is denied",
        )
        for diagnostic in refusals:
            for tool in ("docker", "gh"):
                with self.subTest(tool=tool, diagnostic=diagnostic):
                    result, calls, docker, waits = self.run_verifier(
                        failure_diagnostic=diagnostic,
                        docker_fail_first=1 if tool == "docker" else 0,
                        gh_fail_first=1 if tool == "gh" else 0,
                    )
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertEqual(result.stdout, "")
                    self.assertIn(diagnostic, result.stderr)
                    self.assertEqual(result.stderr.count(diagnostic), 1)
                    self.assertIn("not retrying", result.stderr)
                    self.assertEqual(waits, [])
                    self.assertEqual(len(docker), 1 if tool == "docker" else 4)
                    self.assertEqual(len(calls.splitlines()), 0 if tool == "docker" else 1)

    def test_unknown_tls_policy_and_mixed_errors_do_not_retry(self) -> None:
        diagnostics = (
            "",
            "unknown failure",
            "x509: certificate signed by unknown authority",
            "attestation verification failed: source digest does not match",
            "ERROR: failed to copy: unexpected status 502 Bad Gateway\ngh: Forbidden (HTTP 403)",
            "ERROR: failed to copy: unexpected status 502 Bad Gateway; insufficient_scope",
            "ERROR: failed to copy: DENIED; unexpected status 502 Bad Gateway",
            "ERROR: failed to copy: FORBIDDEN; unexpected status 502 Bad Gateway",
            "ERROR: failed to copy: Unauthorized; unexpected status 502 Bad Gateway",
            "ERROR: failed to copy: unexpected status 429 Too Many Requests",
        )
        for diagnostic in diagnostics:
            for tool in ("docker", "gh"):
                with self.subTest(tool=tool, diagnostic=diagnostic):
                    result, calls, docker, waits = self.run_verifier(
                        failure_diagnostic=diagnostic,
                        docker_fail_first=1 if tool == "docker" else 0,
                        gh_fail_first=1 if tool == "gh" else 0,
                    )
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assertEqual(result.stdout, "")
                    self.assertEqual(waits, [])
                    self.assertEqual(len(docker), 1 if tool == "docker" else 4)
                    self.assertEqual(len(calls.splitlines()), 0 if tool == "docker" else 1)

    def test_nonordinary_exits_are_preserved_even_with_a_transient_diagnostic(self) -> None:
        for status in (2, 4, 124, 130, 137, 143):
            for tool in ("docker", "gh"):
                with self.subTest(tool=tool, status=status):
                    result, calls, docker, waits = self.run_verifier(
                        failure_status=status,
                        docker_fail_first=1 if tool == "docker" else 0,
                        gh_fail_first=1 if tool == "gh" else 0,
                    )
                    self.assertEqual(result.returncode, status, result.stderr)
                    self.assertEqual(result.stdout, "")
                    self.assertEqual(waits, [])
                    self.assertEqual(len(docker), 1 if tool == "docker" else 4)
                    self.assertEqual(len(calls.splitlines()), 0 if tool == "docker" else 1)

    def test_only_known_transfer_5xx_diagnostics_allow_the_same_read(self) -> None:
        for diagnostic in (
            "ERROR: failed to copy: unexpected status 502 Bad Gateway",
            "ERROR: failed to copy: unexpected status 503 Service Unavailable",
            "ERROR: failed to copy: unexpected status 504 Gateway Timeout",
            "failed to copy: unexpected status from GET request to https://registry.example/v2/blob: 502 Bad Gateway",
        ):
            for tool in ("docker", "gh"):
                with self.subTest(tool=tool, diagnostic=diagnostic):
                    result, calls, docker, waits = self.run_verifier(
                        failure_diagnostic=diagnostic,
                        docker_fail_first=1 if tool == "docker" else 0,
                        gh_fail_first=1 if tool == "gh" else 0,
                    )
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(waits, ["10"])
                    self.assertEqual(result.stdout.splitlines(),
                                     [f"server_digest={DIGEST}", f"control_plane_digest={DIGEST}"])
                    repeated = docker if tool == "docker" else calls.splitlines()
                    self.assertEqual(repeated[0], repeated[1])

    def test_rejects_unbounded_retry_settings(self) -> None:
        for attempts in ("0", "100", "x", "-1", "2.5"):
            with self.subTest(attempts=attempts):
                result, calls, _, waits = self.run_verifier(attempts=attempts)
                self.assertEqual(result.returncode, 2, result.stderr)
                self.assertEqual(calls, "")
                self.assertEqual(waits, [])

    def test_rejects_an_unsafe_github_cli_before_registry_calls(self) -> None:
        result, calls, docker, _ = self.run_verifier(gh_version="2.96.1")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("too old for release attestation policy enforcement", result.stderr)
        self.assertEqual(calls, "")
        self.assertEqual(docker, [])

    def test_rejects_invalid_release_identity_without_registry_calls(self) -> None:
        result, calls, docker, _ = self.run_verifier(source_sha="main")

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid release source SHA", result.stderr)
        self.assertEqual(calls, "")
        self.assertEqual(docker, [])


if __name__ == "__main__":
    unittest.main()
