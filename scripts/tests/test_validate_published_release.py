#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "validate-published-release.sh"
TAG = "v9.9.9"
SBOM = f"pulse-{TAG}-release.sbom.spdx.json"


@unittest.skipUnless(shutil.which("ssh-keygen"), "ssh-keygen is required")
class ValidatePublishedReleaseTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.root = Path(self.temp_dir.name)
        self.assets = self.root / "assets"
        self.bin_dir = self.root / "bin"
        self.assets.mkdir()
        self.bin_dir.mkdir()

        self.private_key = self.root / "release-key"
        subprocess.run(
            [
                "ssh-keygen",
                "-q",
                "-t",
                "ed25519",
                "-N",
                "",
                "-f",
                str(self.private_key),
            ],
            check=True,
        )
        self.ssh_public_key = self.private_key.with_suffix(".pub").read_text(
            encoding="utf-8"
        ).strip()

        payloads = {
            SBOM: b'{"spdxVersion":"SPDX-2.3"}\n',
            "install.sh": b"#!/bin/sh\necho pulse\n",
        }
        checksum_lines: list[str] = []
        for name, content in payloads.items():
            (self.assets / name).write_bytes(content)
            digest = hashlib.sha256(content).hexdigest()
            checksum_lines.append(f"{digest}  {name}\n")
            (self.assets / f"{name}.sha256").write_text(
                f"{digest}  {name}\n", encoding="utf-8"
            )
            self.sign(self.assets / name)

        checksums = self.assets / "checksums.txt"
        checksums.write_text("".join(checksum_lines), encoding="utf-8")
        self.sign(checksums)
        self.write_fake_tools()

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def sign(self, path: Path) -> None:
        subprocess.run(
            [
                "ssh-keygen",
                "-q",
                "-Y",
                "sign",
                "-f",
                str(self.private_key),
                "-n",
                "pulse-install",
                str(path),
            ],
            check=True,
            stdout=subprocess.DEVNULL,
        )
        path.with_suffix(path.suffix + ".sig").replace(
            path.with_suffix(path.suffix + ".sshsig")
        )

    def write_fake_tools(self) -> None:
        curl = self.bin_dir / "curl"
        curl.write_text(
            """#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do url="$arg"; done
name="${url##*/}"
cat "${FAKE_RELEASE_ASSETS}/${name}"
""",
            encoding="utf-8",
        )
        curl.chmod(0o755)

        go = self.bin_dir / "go"
        go.write_text(
            """#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "${FAKE_SSH_PUBLIC_KEY}"
""",
            encoding="utf-8",
        )
        go.chmod(0o755)

    def run_validator(
        self, *, include_key: bool = True
    ) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env.update(
            {
                "PATH": f"{self.bin_dir}:{env['PATH']}",
                "FAKE_RELEASE_ASSETS": str(self.assets),
                "FAKE_SSH_PUBLIC_KEY": self.ssh_public_key,
            }
        )
        if include_key:
            env["PULSE_UPDATE_SIGNING_PUBLIC_KEY"] = "test-configured-key"
        else:
            env.pop("PULSE_UPDATE_SIGNING_PUBLIC_KEY", None)
        return subprocess.run(
            [str(SCRIPT), TAG, "example/Pulse"],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_authenticates_manifest_and_each_artifact(self) -> None:
        result = self.run_validator()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("match authenticated checksums.txt", result.stdout)
        self.assertIn("verified *.sshsig sidecars", result.stdout)

    def test_rejects_forged_artifact_signature(self) -> None:
        (self.assets / "install.sh.sshsig").write_text(
            "not a signature\n", encoding="utf-8"
        )
        result = self.run_validator()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SSH signature verification failed for install.sh", result.stderr)

    def test_rejects_forged_checksum_manifest_before_using_it(self) -> None:
        with (self.assets / "checksums.txt").open("a", encoding="utf-8") as handle:
            handle.write(f"{'0' * 64}  attacker-payload\n")
        result = self.run_validator()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SSH signature verification failed for checksums.txt", result.stderr)
        self.assertNotIn("attacker-payload", result.stdout)

    def test_requires_configured_release_trust_root(self) -> None:
        result = self.run_validator(include_key=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("PULSE_UPDATE_SIGNING_PUBLIC_KEY is required", result.stderr)


if __name__ == "__main__":
    unittest.main()
