#!/usr/bin/env python3

from __future__ import annotations

import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "verify-stable-container-aliases.sh"
SERVER_DIGEST = "sha256:" + "a" * 64
CONTROL_PLANE_DIGEST = "sha256:" + "b" * 64


class VerifyStableContainerAliasesTests(unittest.TestCase):
    def run_verifier(
        self,
        *,
        overrides: dict[str, str] | None = None,
        tag: str = "v6.4.1",
        owner: str = "rcourtman",
    ) -> tuple[subprocess.CompletedProcess[str], list[str]]:
        with tempfile.TemporaryDirectory() as temp:
            temp_path = Path(temp)
            bin_path = temp_path / "bin"
            bin_path.mkdir()
            digest_file = temp_path / "digests"
            call_log = temp_path / "calls"
            values: dict[str, str] = {}
            for image, digest in (
                ("pulse", SERVER_DIGEST),
                ("pulse-control-plane", CONTROL_PLANE_DIGEST),
            ):
                for registry in ("docker.io/rcourtman", f"ghcr.io/{owner}"):
                    for alias in ("6.4", "6", "latest"):
                        values[f"{registry}/{image}:{alias}"] = digest
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
                    printf '%s\n' "$reference" >> "$CALL_LOG"
                    digest=$(awk -v ref="$reference" '$1 == ref { print $2 }' "$DIGEST_FILE")
                    [ -n "$digest" ] || exit 1
                    printf '{"digest":"%s"}\n' "$digest"
                    """
                ),
                encoding="utf-8",
            )
            (bin_path / "docker").chmod(0o755)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{bin_path}:{env['PATH']}",
                    "DIGEST_FILE": str(digest_file),
                    "CALL_LOG": str(call_log),
                }
            )
            result = subprocess.run(
                [
                    str(SCRIPT),
                    tag,
                    SERVER_DIGEST,
                    CONTROL_PLANE_DIGEST,
                    owner,
                ],
                cwd=ROOT,
                env=env,
                text=True,
                capture_output=True,
                check=False,
            )
            calls = call_log.read_text(encoding="utf-8").splitlines() if call_log.exists() else []
            return result, calls

    def test_verifies_every_stable_alias_in_both_registries(self) -> None:
        result, calls = self.run_verifier()

        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 12)
        self.assertIn("docker.io/rcourtman/pulse:latest", calls)
        self.assertIn("ghcr.io/rcourtman/pulse-control-plane:6.4", calls)

    def test_rejects_a_moved_alias(self) -> None:
        moved = "sha256:" + "c" * 64
        result, calls = self.run_verifier(
            overrides={"ghcr.io/rcourtman/pulse:latest": moved}
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("resolved to", result.stderr)
        self.assertIn("ghcr.io/rcourtman/pulse:latest", calls)

    def test_rejects_prerelease_without_registry_calls(self) -> None:
        result, calls = self.run_verifier(tag="v6.4.2-rc.1")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid stable release tag", result.stderr)
        self.assertEqual(calls, [])

    def test_rejects_invalid_digest_without_registry_calls(self) -> None:
        result = subprocess.run(
            [str(SCRIPT), "v6.4.2", "latest", CONTROL_PLANE_DIGEST],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Invalid server image digest", result.stderr)


if __name__ == "__main__":
    unittest.main()
