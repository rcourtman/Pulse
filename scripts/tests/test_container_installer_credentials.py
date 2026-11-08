#!/usr/bin/env python3
"""Run the shipped compatibility wrapper with inert local/remote installers.

No listener, network, real installation or service operation is used. The
canonical installer still owns credential-file validation and authentication.
"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
WRAPPER = ROOT / "scripts/install-container-agent.sh"
TOKEN = "synthetic-wrapper-token-not-for-argv"
RECEIVER = '''#!/usr/bin/env bash
set -eu
printf '%s\\0' "$@" > "$FIXTURE_ROOT/arguments"
printf '%s' "$0" > "$FIXTURE_ROOT/source"
exit "$FIXTURE_EXIT"
'''


class ContainerInstallerCredentialsTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse wrapper space ")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.scripts = self.root / "scripts"
        self.tools = self.root / "tools"
        self.downloads = self.root / "downloads"
        for directory in (self.scripts, self.tools, self.downloads):
            directory.mkdir(mode=0o700)
        self.wrapper = self.scripts / "install-container-agent.sh"
        self.wrapper.write_bytes(WRAPPER.read_bytes())
        self.token = self.root / "private $; token\nfile"
        self.token.write_text(TOKEN + "\n")
        self.token.chmod(0o600)
        self.env = dict(os.environ, PATH=str(self.tools) + os.pathsep + os.environ["PATH"],
                        TMPDIR=str(self.downloads), FIXTURE_ROOT=str(self.root),
                        FIXTURE_EXIT="37", DOWNLOAD_EXIT="0")
        self.tool("curl", '''#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ["FIXTURE_ROOT"])
args = sys.argv[1:]
(root / "curl-arguments").write_text(json.dumps(args))
pathlib.Path(args[args.index("--output") + 1]).write_bytes((root / "payload").read_bytes())
sys.exit(int(os.environ["DOWNLOAD_EXIT"]))
''')
        (self.root / "payload").write_text(RECEIVER)

    def tool(self, name, source):
        path = self.tools / name
        path.write_text(source)
        path.chmod(0o700)

    def run_wrapper(self, *arguments):
        result = subprocess.run([shutil.which("bash"), str(self.wrapper), *arguments],
                                env=self.env, capture_output=True, text=True, timeout=5)
        self.assertNotIn(TOKEN, result.stdout + result.stderr)
        self.assertEqual(self.token.read_text(), TOKEN + "\n", "caller-owned credential changed")
        self.assertEqual(list(self.downloads.iterdir()), [], "owned download was not removed")
        return result

    def assert_forwarded(self, arguments, source):
        receipt = (self.root / "arguments").read_bytes()
        self.assertEqual(receipt, b"\0".join(os.fsencode(arg) for arg in arguments) + b"\0")
        self.assertNotIn(TOKEN.encode(), receipt)
        self.assertEqual((self.root / "source").read_text(), source)

    def test_private_file_reaches_adjacent_installer_without_reading_it(self):
        installer = self.scripts / "install.sh"
        installer.write_text(RECEIVER)
        installer.chmod(0o700)
        arguments = ["--url", "https://pulse.example.invalid", "--token-file", str(self.token)]
        result = self.run_wrapper(*arguments)
        self.assertEqual(result.returncode, 37, result.stderr)
        self.assert_forwarded(arguments + ["--enable-docker", "--enable-host"], str(installer))
        self.assertFalse((self.root / "curl-arguments").exists())

    def test_private_file_reaches_downloaded_installer_with_original_exit_and_identity(self):
        for uninstall, status in ((False, "0"), (False, "37"), (True, "0"), (True, "37")):
            with self.subTest(uninstall=uninstall, status=status):
                self.env["FIXTURE_EXIT"] = status
                arguments = ["--url", "https://pulse.example.invalid", "--token-file", str(self.token),
                             "--interval", "7 s; literal"]
                flags = ["--uninstall"] if uninstall else ["--enable-docker", "--enable-host"]
                result = self.run_wrapper(*(arguments + (["--uninstall"] if uninstall else [])))
                self.assertEqual(result.returncode, int(status), result.stderr)
                self.assert_forwarded(arguments + flags, "bash")
                curl_args = json.loads((self.root / "curl-arguments").read_text())
                self.assertEqual(curl_args[:2], ["-q", "-fsSL"],
                                 "default configuration must not override verified TLS")
                self.assert_transport_deadlines(curl_args)
                self.assertEqual(curl_args[-2:], ["--", "https://pulse.example.invalid/install.sh"])
                self.assertNotIn(TOKEN, " ".join(curl_args))

    def test_wrapper_leaves_file_admission_with_the_canonical_installer(self):
        missing = self.root / "absent private file"
        arguments = ["--url", "https://pulse.example.invalid", "--token-file", str(missing)]
        result = self.run_wrapper(*arguments)
        self.assertEqual(result.returncode, 37, result.stderr)
        self.assert_forwarded(arguments + ["--enable-docker", "--enable-host"], "bash")
        self.assertFalse(missing.exists())

    def test_missing_or_empty_private_file_argument_stops_before_transport_or_execution(self):
        for tail in (("--token-file",), ("--token-file", "")):
            with self.subTest(tail=tail):
                result = self.run_wrapper("--url", "https://pulse.example.invalid", *tail)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("--token-file requires a path", result.stderr)
                self.assertFalse((self.root / "curl-arguments").exists())
                self.assertFalse((self.root / "arguments").exists())

    def assert_transport_deadlines(self, curl_args):
        for flag, value in (("--connect-timeout", "5"), ("--max-time", "60")):
            self.assertEqual(curl_args.count(flag), 1)
            self.assertEqual(curl_args[curl_args.index(flag) + 1], value)
        self.assertNotIn("--retry", curl_args, "failed installer downloads must not be replayed")

    def test_explicit_tls_opt_out_keeps_the_same_transport_deadlines(self):
        result = self.run_wrapper("--url", "https://pulse.example.invalid", "--insecure",
                                  "--token-file", str(self.token))
        self.assertEqual(result.returncode, 37, result.stderr)
        curl_args = json.loads((self.root / "curl-arguments").read_text())
        self.assertEqual(curl_args[:2], ["-q", "-kfsSL"])
        self.assert_transport_deadlines(curl_args)
        self.assert_forwarded(["--url", "https://pulse.example.invalid", "--token-file", str(self.token),
                               "--insecure", "--enable-docker", "--enable-host"], "bash")

    def test_failed_transfer_still_executes_nothing_and_preserves_private_file(self):
        for status in ("6", "18", "22", "28"):
            with self.subTest(curl_exit=status):
                self.env["DOWNLOAD_EXIT"] = status
                result = self.run_wrapper("--url", "https://pulse.example.invalid", "--token-file", str(self.token))
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("nothing was executed", result.stderr)
                self.assertFalse((self.root / "arguments").exists())
                self.assert_transport_deadlines(json.loads((self.root / "curl-arguments").read_text()))

    def test_inline_token_compatibility_and_private_file_precedence_are_forwarded(self):
        # Compatibility is not recommended, but existing callers must keep the
        # canonical installer's file-over-inline selection, not a wrapper copy.
        arguments = ["--url", "https://pulse.example.invalid", "--token", "compatibility-value",
                     "--token-file", str(self.token)]
        result = self.run_wrapper(*arguments)
        self.assertEqual(result.returncode, 37, result.stderr)
        self.assert_forwarded(arguments + ["--enable-docker", "--enable-host"], "bash")

    def test_help_prefers_private_file_and_preserves_explicit_profile_choice(self):
        result = self.run_wrapper("--help")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertLess(result.stdout.index("--token-file <path>"), result.stdout.index("--token <token>"))
        self.assertIn("exposes the token in process arguments", result.stdout)
        self.assertIn("caller owns input-file cleanup", result.stdout)
        self.assertIn("remote bootstrap ignores default curl configuration", result.stdout)
        self.assertIn("CURL_CA_BUNDLE", result.stdout)
        self.assertIn("proxy environment variables", result.stdout)
        self.assertIn("--enable-host=false", result.stdout)
        self.assertFalse((self.root / "curl-arguments").exists())
        self.assertFalse((self.root / "arguments").exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
