#!/usr/bin/env python3
"""Exercise the actual documented-key validator with local synthetic signatures."""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
VALIDATOR = Path(os.environ.get("PULSE_RELEASE_VALIDATOR_UNDER_TEST", ROOT / "scripts/validate-release.sh"))


class DocumentedInstallerKey(unittest.TestCase):
    def exercise(self, fault=""):
        source = VALIDATOR.read_text()
        starts = ('info "Validating documented installation signature key',
                  'info "Validating README pinned signature key')
        start = next(source.index(s) for s in starts if s in source)
        section = source[start:source.index("# Validate individual .sha256 files", start)]
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "docs").mkdir()
            (root / "scratch").mkdir()
            installer = root / "install.sh"
            installer.write_text("#!/bin/bash\n# Inert signed fixture; never executed.\n")
            for name in ("signer", "other"):
                subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(root / name)],
                               check=True, capture_output=True)
            subprocess.run(["ssh-keygen", "-q", "-Y", "sign", "-f", str(root / "signer"),
                            "-n", "pulse-install", str(installer)], check=True, capture_output=True)
            (root / "install.sh.sig").rename(root / "install.sh.sshsig")
            public = " ".join((root / "signer.pub").read_text().split()[:2]) + " pulse-installer"
            other = " ".join((root / "other.pub").read_text().split()[:2]) + " pulse-installer"
            readme = root / "README.md"
            readme.write_text("[Signed install](docs/INSTALL.md#quick-start-recommended)\n")
            guide = root / "docs/INSTALL.md"
            guide.write_text(public + "\n" + public + "\n")
            if fault == "missing_guide":
                guide.unlink()
            elif fault == "unlinked":
                readme.write_text("No supported install link.\n")
            elif fault == "missing_key":
                guide.write_text("No signing key.\n")
            elif fault == "conflicting_keys":
                guide.write_text(public + "\n" + other + "\n")
            elif fault == "wrong_key":
                guide.write_text(other + "\n")
            elif fault == "changed_installer":
                installer.write_text("changed unsigned fixture\n")
            elif fault == "missing_signature":
                (root / "install.sh.sshsig").unlink()
            script = ('set -euo pipefail\n'
                      'info() { :; }; success() { :; }; error() { printf "%s\\n" "$*" >&2; }\n'
                      'REPO_ROOT="$1"\n' + section)
            result = subprocess.run(["bash", "-c", script, "bash", str(root)], cwd=root,
                                    env={**os.environ, "TMPDIR": str(root / "scratch")},
                                    text=True, capture_output=True, timeout=10)
            return result, list((root / "scratch").iterdir())

    def test_linked_guide_verifies_without_a_duplicate_readme_key(self):
        result, scratch = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(scratch, [])

    def test_missing_unlinked_or_conflicting_documentation_refuses(self):
        for fault in ("missing_guide", "unlinked", "missing_key", "conflicting_keys"):
            with self.subTest(fault=fault):
                result, scratch = self.exercise(fault)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(scratch, [])

    def test_wrong_key_changed_bytes_and_missing_signature_refuse(self):
        for fault in ("wrong_key", "changed_installer", "missing_signature"):
            with self.subTest(fault=fault):
                result, scratch = self.exercise(fault)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(scratch, [])


if __name__ == "__main__":
    unittest.main()
