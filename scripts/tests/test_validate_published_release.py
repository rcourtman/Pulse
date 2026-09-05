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
            "install-docker.sh": b"#!/bin/sh\necho docker installer\n",
            "install-mcp.sh": b"#!/bin/sh\necho mcp installer\n",
            "install-mcp.ps1": b"Write-Output 'mcp installer'\r\n",
            "install.ps1": b"Write-Output 'agent installer'\r\n",
            "install.sh": b"#!/bin/sh\necho pulse\n",
            "pulse-auto-update.sh": b"#!/bin/sh\necho update\n",
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

    def sign(
        self, path: Path, *, key: Path | None = None, namespace: str = "pulse-install"
    ) -> None:
        subprocess.run(
            [
                "ssh-keygen",
                "-q",
                "-Y",
                "sign",
                "-f",
                str(key or self.private_key),
                "-n",
                namespace,
                str(path),
            ],
            check=True,
            stdout=subprocess.DEVNULL,
        )
        path.with_suffix(path.suffix + ".sig").replace(
            path.with_suffix(path.suffix + ".sshsig")
        )

    def replace_signed_checksums(self, content: str) -> None:
        checksums = self.assets / "checksums.txt"
        checksums.write_text(content, encoding="utf-8")
        checksums.with_suffix(".txt.sshsig").unlink()
        self.sign(checksums)

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

    def test_rejects_valid_signatures_from_an_untrusted_key(self) -> None:
        other_key = self.root / "untrusted-key"
        subprocess.run(
            ["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(other_key)],
            check=True,
        )
        for name in ("checksums.txt", "install.sh"):
            with self.subTest(asset=name):
                path = self.assets / name
                self.sign(path, key=other_key)
                result = self.run_validator()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(
                    f"SSH signature verification failed for {name}", result.stderr
                )
                if name == "checksums.txt":
                    self.assertNotIn("Verifying install.sh", result.stdout)
                self.sign(path)

    def test_rejects_trusted_key_signatures_for_another_namespace(self) -> None:
        for name in ("checksums.txt", "install.sh"):
            with self.subTest(asset=name):
                path = self.assets / name
                self.sign(path, namespace="not-pulse-install")
                result = self.run_validator()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(
                    f"SSH signature verification failed for {name}", result.stderr
                )
                if name == "checksums.txt":
                    self.assertNotIn("Verifying install.sh", result.stdout)
                self.sign(path)

    def test_rejects_unsigned_published_mcp_installer(self) -> None:
        checksums = (self.assets / "checksums.txt").read_text(encoding="utf-8")
        filtered = "".join(
            line
            for line in checksums.splitlines(keepends=True)
            if not line.endswith("  install-mcp.sh\n")
        )
        self.replace_signed_checksums(filtered)

        result = self.run_validator()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "Authenticated checksums.txt must contain exactly one valid entry for published installer install-mcp.sh.",
            result.stderr,
        )

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

    def test_rejects_authenticated_manifest_with_duplicate_filename(self) -> None:
        checksums = (self.assets / "checksums.txt").read_text(encoding="utf-8")
        install_line = next(
            line for line in checksums.splitlines() if line.endswith("  install.sh")
        )
        self.replace_signed_checksums(f"{checksums}{install_line}\n")

        result = self.run_validator()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "Duplicate release asset filename in checksums.txt: install.sh",
            result.stderr,
        )

    def test_rejects_authenticated_manifest_with_trailing_fields(self) -> None:
        checksums = (self.assets / "checksums.txt").read_text(encoding="utf-8")
        malformed = checksums.replace("  install.sh\n", "  install.sh unexpected\n")
        self.replace_signed_checksums(malformed)

        result = self.run_validator()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn(
            "Malformed checksums line (unexpected fields for install.sh).",
            result.stderr,
        )

    def test_validates_final_manifest_entry_without_newline(self) -> None:
        checksums = (self.assets / "checksums.txt").read_text(encoding="utf-8")
        self.replace_signed_checksums(checksums.rstrip("\n"))
        (self.assets / "install.sh.sshsig").write_text(
            "not a signature\n", encoding="utf-8"
        )

        result = self.run_validator()

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("SSH signature verification failed for install.sh", result.stderr)


if __name__ == "__main__":
    unittest.main()
