#!/usr/bin/env python3
"""Exercise MCP binary publication with real signatures and inert downloads.

All files and commands are confined to temporary fixtures. This is not native
platform installation, a published-release check or MCP client acceptance.
"""

import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_MCP_INSTALLER_UNDER_TEST", ROOT / "scripts/install-mcp.sh"))
RELEASE_KEY = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
OLD_BINARY = b"previous working adapter\n"
NEW_BINARY = b"verified replacement adapter\n"


class MCPInstallerPublicationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.keys = tempfile.TemporaryDirectory(prefix="pulse-mcp-test-key-")
        cls.key = Path(cls.keys.name) / "release-key"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(cls.key)],
                       check=True, capture_output=True, timeout=10)
        source = INSTALLER.read_text()
        if source.count(RELEASE_KEY) != 1:
            raise AssertionError("expected exactly one pinned release key")
        cls.source = source.replace(RELEASE_KEY, cls.key.with_suffix(".pub").read_text().strip())

    @classmethod
    def tearDownClass(cls):
        cls.keys.cleanup()

    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-mcp-publication-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.tools = self.root / "tools"
        self.fixtures = self.root / "fixtures"
        self.destination = self.root / "install with spaces"
        self.downloads = self.root / "downloads"
        for directory in (self.tools, self.fixtures, self.destination, self.downloads):
            directory.mkdir()
        self.binary = self.destination / "pulse-mcp"
        self.script = self.root / "install-mcp.sh"
        self.script.write_text(self.source)
        (self.fixtures / "pulse-mcp-linux-amd64").write_bytes(NEW_BINARY)
        manifest = self.fixtures / "checksums.txt"
        manifest.write_text(hashlib.sha256(NEW_BINARY).hexdigest() + "  pulse-mcp-linux-amd64\n")
        subprocess.run(["ssh-keygen", "-q", "-Y", "sign", "-f", str(self.key),
                        "-n", "pulse-install", str(manifest)],
                       check=True, capture_output=True, timeout=10)
        manifest.with_suffix(".txt.sig").rename(self.fixtures / "checksums.txt.sshsig")
        self.environment = {
            **os.environ,
            "HOME": str(self.root),
            "TMPDIR": str(self.downloads),
            "PATH": str(self.tools) + os.pathsep + os.environ["PATH"],
            "FIXTURE": str(self.root),
            "PULSE_MCP_BIN_DIR": str(self.destination),
            "PULSE_MCP_VERSION": "v-fixture",
            "PULSE_MCP_REPO": "rcourtman/Pulse",
            "FAULT": "",
            "REAL_INSTALL": shutil.which("install"),
            "REAL_MV": shutil.which("mv"),
            "REAL_MKTEMP": shutil.which("mktemp"),
        }
        self.tool("uname", "#!/bin/sh\ncase \"$1\" in -s) echo Linux ;; -m) echo x86_64 ;; *) exit 99 ;; esac\n")
        self.tool("curl", "#!/usr/bin/env python3\n" + r'''
import os, pathlib, shutil, sys
args = sys.argv[1:]
assert args[0] == "-q", "curlrc must not override verified transport"
for flag, value in (("--connect-timeout", "5"), ("--max-time", "60"),
                    ("--retry", "3"), ("--retry-max-time", "120"),
                    ("--proto", "=https"), ("--proto-redir", "=https")):
    assert args[args.index(flag) + 1] == value, (flag, args)
urls = [arg for arg in args if arg.startswith("https://")]
assert len(urls) == 1 and urls[0].startswith("https://github.com/rcourtman/Pulse/releases/download/v-fixture/")
root = pathlib.Path(os.environ["FIXTURE"])
with (root / "curl-events").open("a") as log:
    log.write(urls[0] + "\n")
shutil.copyfile(root / "fixtures" / urls[0].rsplit("/", 1)[1], args[args.index("-o") + 1])
if os.environ["FAULT"] == "download-" + urls[0].rsplit("/", 1)[1]:
    pathlib.Path(args[args.index("-o") + 1]).write_bytes(b"partial download")
    sys.exit(28)
if os.environ["FAULT"] == "destination-changed" and urls[0].endswith("checksums.txt.sshsig"):
    destination = pathlib.Path(os.environ["PULSE_MCP_BIN_DIR"]) / "pulse-mcp"
    destination.unlink()
    destination.symlink_to(root / "witness")
''')
        self.tool("mktemp", "#!/usr/bin/env python3\n" + r'''
import os, subprocess, sys
if os.environ["FAULT"] == "allocation" and "-d" in sys.argv[1:]:
    sys.exit(25)
sys.exit(subprocess.run([os.environ["REAL_MKTEMP"], *sys.argv[1:]]).returncode)
''')
        self.tool("install", "#!/usr/bin/env python3\n" + r'''
import json, os, pathlib, subprocess, sys
args = sys.argv[1:]
root = pathlib.Path(os.environ["FIXTURE"])
(root / "install-event").write_text(json.dumps(args))
if os.environ["FAULT"] in ("copy", "short-copy"):
    pathlib.Path(args[-1]).write_bytes(b"partial")
    sys.exit(23 if os.environ["FAULT"] == "copy" else 0)
sys.exit(subprocess.run([os.environ["REAL_INSTALL"], *args]).returncode)
''')
        self.tool("mv", "#!/usr/bin/env python3\n" + r'''
import json, os, pathlib, subprocess, sys
root = pathlib.Path(os.environ["FIXTURE"])
(root / "mv-event").write_text(json.dumps(sys.argv[1:]))
fault = os.environ["FAULT"]
if fault == "rename":
    sys.exit(24)
if fault == "rename-noop":
    sys.exit(0)
sys.exit(subprocess.run([os.environ["REAL_MV"], *sys.argv[1:]]).returncode)
''')

    def tool(self, name, source):
        path = self.tools / name
        path.write_text(source)
        path.chmod(0o700)

    def existing_binary(self):
        self.binary.write_bytes(OLD_BINARY)
        self.binary.chmod(0o700)

    def run_installer(self, fault=""):
        result = subprocess.run(["bash", str(self.script)],
                                env={**self.environment, "FAULT": fault},
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(list(self.downloads.iterdir()), [], "download temporaries must be cleaned")
        self.assertEqual(list(self.destination.glob(".pulse-mcp.*")), [], "private stages must be cleaned")
        return result

    def test_fresh_install_and_update_publish_verified_executable(self):
        for existing in (False, True):
            with self.subTest(existing=existing):
                if existing:
                    self.existing_binary()
                result = self.run_installer()
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn("release signature verified", result.stdout)
                self.assertIn("installed:", result.stdout)
                self.assertEqual(self.binary.read_bytes(), NEW_BINARY)
                self.assertEqual(stat.S_IMODE(self.binary.stat().st_mode), 0o755)
                args = json.loads((self.root / "install-event").read_text())
                staged = Path(args[-1])
                self.assertNotEqual(staged, self.binary, "copy must not overwrite the live executable")
                self.assertEqual(staged.parent.parent, self.destination)
                move = json.loads((self.root / "mv-event").read_text())
                self.assertEqual(move[-2:], [str(staged), str(self.binary)])

    def test_failed_copy_preserves_previous_binary_and_mode(self):
        self.existing_binary()
        result = self.run_installer("copy")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("installed:", result.stdout)
        self.assertEqual(self.binary.read_bytes(), OLD_BINARY)
        self.assertEqual(stat.S_IMODE(self.binary.stat().st_mode), 0o700)
        self.assertFalse((self.root / "mv-event").exists())

    def test_silent_short_copy_does_not_publish(self):
        self.existing_binary()
        result = self.run_installer("short-copy")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("installed:", result.stdout)
        self.assertEqual(self.binary.read_bytes(), OLD_BINARY)
        self.assertFalse((self.root / "mv-event").exists())

    def test_failed_fresh_copy_does_not_leave_partial_executable(self):
        for fault in ("copy", "short-copy"):
            with self.subTest(fault=fault):
                result = self.run_installer(fault)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("installed:", result.stdout)
                self.assertFalse(self.binary.exists())
                self.assertFalse((self.root / "mv-event").exists())

    def test_stage_allocation_failure_leaves_old_adapter_untouched(self):
        self.existing_binary()
        result = self.run_installer("allocation")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("installed:", result.stdout)
        self.assertEqual(self.binary.read_bytes(), OLD_BINARY)
        self.assertFalse((self.root / "install-event").exists())
        self.assertFalse((self.root / "mv-event").exists())

    def test_failed_or_skipped_rename_does_not_report_installation(self):
        for fault in ("rename", "rename-noop"):
            with self.subTest(fault=fault):
                self.existing_binary()
                result = self.run_installer(fault)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("installed:", result.stdout)
                self.assertEqual(self.binary.read_bytes(), OLD_BINARY)
                self.assertEqual(stat.S_IMODE(self.binary.stat().st_mode), 0o700)

    def test_signature_failure_preserves_previous_installation(self):
        self.existing_binary()
        (self.fixtures / "checksums.txt.sshsig").write_text("invalid signature\n")
        result = self.run_installer()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("cryptographic signature verification failed", result.stderr)
        self.assertEqual(self.binary.read_bytes(), OLD_BINARY)
        self.assertFalse((self.root / "install-event").exists())
        self.assertFalse((self.root / "mv-event").exists())

    def test_each_partial_download_fails_before_publication_and_cleans_owned_files(self):
        for asset, diagnostic in (
            ("pulse-mcp-linux-amd64", "download failed:"),
            ("checksums.txt", "could not fetch checksums.txt; refusing unverified install"),
            ("checksums.txt.sshsig", "could not fetch checksums.txt.sshsig; refusing unverified install"),
        ):
            with self.subTest(asset=asset):
                self.existing_binary()
                result = self.run_installer("download-" + asset)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(diagnostic, result.stderr)
                self.assertNotIn("installed:", result.stdout)
                self.assertEqual(self.binary.read_bytes(), OLD_BINARY)
                self.assertEqual(stat.S_IMODE(self.binary.stat().st_mode), 0o700)
                self.assertFalse((self.root / "install-event").exists())
                self.assertFalse((self.root / "mv-event").exists())

    def test_destination_changed_during_download_is_not_published_through(self):
        self.existing_binary()
        witness = self.root / "witness"
        witness.write_bytes(OLD_BINARY)
        result = self.run_installer("destination-changed")
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("installed:", result.stdout)
        self.assertTrue(self.binary.is_symlink())
        self.assertEqual(witness.read_bytes(), OLD_BINARY)
        self.assertFalse((self.root / "mv-event").exists())

    def test_unsafe_existing_destination_is_not_followed_or_replaced(self):
        for kind in ("symlink", "dangling-symlink", "directory", "fifo"):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory(dir=self.root) as temporary:
                directory = Path(temporary)
                destination = directory / "pulse-mcp"
                witness = directory / "outside"
                witness.write_bytes(OLD_BINARY)
                if kind == "symlink":
                    destination.symlink_to(witness)
                elif kind == "dangling-symlink":
                    destination.symlink_to(directory / "absent")
                elif kind == "directory":
                    destination.mkdir()
                else:
                    os.mkfifo(destination)
                result = subprocess.run(["bash", str(self.script)],
                                        env={**self.environment, "PULSE_MCP_BIN_DIR": str(directory)},
                                        capture_output=True, text=True, timeout=15)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("installed:", result.stdout)
                self.assertEqual(witness.read_bytes(), OLD_BINARY)
                self.assertFalse((directory / "absent").exists())
                self.assertFalse((self.root / "curl-events").exists(), "refuse before downloading")
                if kind in ("symlink", "dangling-symlink"):
                    self.assertTrue(destination.is_symlink())
                elif kind == "directory":
                    self.assertEqual(list(destination.iterdir()), [])
                else:
                    self.assertTrue(stat.S_ISFIFO(destination.lstat().st_mode))


if __name__ == "__main__":
    unittest.main()
