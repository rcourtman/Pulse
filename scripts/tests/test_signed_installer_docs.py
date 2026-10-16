#!/usr/bin/env python3
"""Execute both copied installer recipes; transport is fake, SSH verification real."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
PRODUCTION_KEY = "AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm"
VERSION = "v6.6.0-rc.1"

# No network, service, privilege or production signing key is used. Check the
# actual arguments the copied recipe supplies, then simulate each result.
CURL = r'''
import json, os, pathlib, stat, sys
args = sys.argv[1:]
assert args[0] == '--disable', args
assert args[1:5] == ['--fail', '--silent', '--show-error', '--location'], args
assert args[5:13] == ['--proto', '=https', '--proto-redir', '=https',
                       '--connect-timeout', '10', '--max-time', '60'], args
assert args[13] == '--output' and args[15:17] == ['--write-out', '%{http_code}'], args
assert len(args) == 18, args
name, url = args[14], args[-1]
assert name in ('install.sh', 'install.sh.sshsig'), name
assert url == 'https://github.com/rcourtman/Pulse/releases/download/' + os.environ['EXPECTED_VERSION'] + '/' + name, url
failure = os.environ['FAILURE']
phase = 'installer' if name == 'install.sh' else 'signature'
source = pathlib.Path(os.environ['FIXTURES']) / name
if failure == 'signature-key' and phase == 'signature':
    source = source.with_name('wrong-key.sshsig')
if failure == 'signature-namespace' and phase == 'signature':
    source = source.with_name('wrong-namespace.sshsig')
body = source.read_bytes()
if failure == 'payload-tampered' and phase == 'installer':
    body += b'\n# changed after signing\n'
if failure == phase + '-download' or failure == phase + '-timeout':
    body = body[:24]  # A partial file must not become an installer.
pathlib.Path(name).write_bytes(body)
with open(os.environ['CURL_RECEIPT'], 'a') as log:
    log.write(json.dumps({'args': args, 'directory': os.getcwd(),
                          'directory_mode': stat.S_IMODE(os.stat('.').st_mode),
                          'file_mode': stat.S_IMODE(os.stat(name).st_mode)}) + '\n')
if failure == phase + '-download':
    sys.exit(22)
if failure == phase + '-timeout':
    sys.exit(28)
print('206' if failure == phase + '-http' else '200', end='')
'''


@unittest.skipUnless(shutil.which("ssh-keygen"), "OpenSSH SSH-signature support required")
class SignedInstallerDocsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory()
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.fixtures = Path(cls.temporary.name)
        installer = cls.fixtures / "install.sh"
        installer.write_text('#!/bin/bash\nprintf "%s\\n" "$@" > "$INSTALL_RECEIPT"\n'
                             'exit "${INSTALL_EXIT:-0}"\n')
        cls.run_keygen("-q", "-t", "ed25519", "-N", "", "-f", str(cls.fixtures / "key"))
        cls.public_key = (cls.fixtures / "key.pub").read_text().split()[1]
        cls.run_keygen("-Y", "sign", "-f", str(cls.fixtures / "key"),
                       "-n", "pulse-install", str(installer))
        (cls.fixtures / "install.sh.sig").rename(cls.fixtures / "install.sh.sshsig")
        cls.run_keygen("-q", "-t", "ed25519", "-N", "", "-f", str(cls.fixtures / "other-key"))
        for fixture, key, namespace in (("wrong-key", "other-key", "pulse-install"),
                                        ("wrong-namespace", "key", "different-purpose")):
            payload = cls.fixtures / fixture
            payload.write_bytes(installer.read_bytes())
            cls.run_keygen("-Y", "sign", "-f", str(cls.fixtures / key),
                           "-n", namespace, str(payload))
            (cls.fixtures / (fixture + ".sig")).rename(cls.fixtures / (fixture + ".sshsig"))
        cls.blocks = [block for block in re.findall(
            r"```bash\n(.*?)\n```", (ROOT / "docs/INSTALL.md").read_text(), re.S)
            if "ssh-keygen -Y verify" in block]
        if len(cls.blocks) != 2:
            raise AssertionError("Exercise both LXC and systemd recipes")

    @classmethod
    def run_keygen(cls, *args):
        subprocess.run(["ssh-keygen", *args], check=True, capture_output=True, timeout=10)

    def exercise(self, recipe, failure):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            commands = root / "bin"
            commands.mkdir()
            curl = commands / "curl"
            curl.write_text(f"#!{sys.executable}\n" + CURL)
            curl.chmod(0o700)
            sudo = commands / "sudo"
            sudo.write_text('#!/bin/bash\nprintf "%s\\n" "$@" >> "$SUDO_RECEIPT"\nexec "$@"\n')
            sudo.chmod(0o700)
            install_receipt, curl_receipt, sudo_receipt = (root / name for name in ("executed", "curl", "sudo"))
            block = self.blocks[recipe]
            self.assertEqual(block.count(PRODUCTION_KEY), 1, "The pinned public key must remain explicit")
            # Only the fixture's synthetic public key replaces the documented
            # production public key. All namespace/identity/verification flags
            # and the script handed to bash remain exactly as copied.
            block = block.replace(PRODUCTION_KEY, self.public_key).replace("vX.Y.Z", VERSION)
            env = {**os.environ, "PATH": f"{commands}:{os.environ['PATH']}", "TMPDIR": str(root),
                   "FAILURE": failure, "FIXTURES": str(self.fixtures), "EXPECTED_VERSION": VERSION,
                   "INSTALL_RECEIPT": str(install_receipt), "CURL_RECEIPT": str(curl_receipt),
                   "SUDO_RECEIPT": str(sudo_receipt), "INSTALL_EXIT": "42" if failure == "installer-runtime" else "0"}
            result = subprocess.run(["bash", "-c", block], cwd=root, env=env,
                                    capture_output=True, text=True, timeout=15)
            calls = [json.loads(line) for line in curl_receipt.read_text().splitlines()]
            self.assertEqual(len(calls), 1 if failure.startswith("installer-") and failure != "installer-runtime" else 2,
                             result.stderr)
            for call in calls:
                self.assertEqual(call["directory_mode"], 0o700)
                self.assertEqual(call["file_mode"], 0o600)
                self.assertFalse(Path(call["directory"]).exists(), "Private temporary files survived recipe exit")
            executes = failure in ("none", "installer-runtime")
            if executes:
                self.assertEqual(result.returncode, 42 if failure == "installer-runtime" else 0, result.stderr)
                self.assertEqual(install_receipt.read_text(), f"--version\n{VERSION}\n")
            else:
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(install_receipt.exists(), "Unverified or incomplete installer was executed")
            self.assertEqual(sudo_receipt.exists(), executes and recipe == 1,
                             "sudo must not run before both downloads/status/signature checks")
            if sudo_receipt.exists():
                self.assertEqual(sudo_receipt.read_text(), f"bash\ninstall.sh\n--version\n{VERSION}\n")
            return result

    def test_verified_script_runs_once_with_exact_release(self):
        for recipe in range(2):
            with self.subTest(recipe=recipe):
                self.exercise(recipe, "none")

    def test_download_exit_timeout_and_non_200_never_execute(self):
        # Exit 28 simulates curl's deadline result. This proves the recipe's
        # response, not real-network deadline enforcement or GitHub availability.
        for recipe in range(2):
            for phase in ("installer", "signature"):
                for failure in ("download", "timeout", "http"):
                    with self.subTest(recipe=recipe, phase=phase, failure=failure):
                        self.exercise(recipe, phase + "-" + failure)

    def test_native_signature_rejects_changed_bytes_wrong_key_and_namespace(self):
        for recipe in range(2):
            for failure in ("payload-tampered", "signature-key", "signature-namespace"):
                with self.subTest(recipe=recipe, failure=failure):
                    self.exercise(recipe, failure)

    def test_installer_failure_is_returned_without_automatic_retry(self):
        for recipe in range(2):
            with self.subTest(recipe=recipe):
                self.exercise(recipe, "installer-runtime")

    def test_shipped_copy_is_identical(self):
        self.assertEqual((ROOT / "docs/INSTALL.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/INSTALL.md").read_bytes())

    def test_guidance_qualifies_deadlines_and_uncertain_execution(self):
        text = " ".join((ROOT / "docs/INSTALL.md").read_text().split())
        for phrase in ("limit each download to 60 seconds", "10-second connection deadline",
                       "Keep `--disable` first", "Download deadlines do not limit the installer",
                       "result is uncertain", "not another installation attempt",
                       "A verified script is not proof of a completed installation"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)


if __name__ == "__main__":
    unittest.main()
