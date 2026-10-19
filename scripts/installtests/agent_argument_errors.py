#!/usr/bin/env python3
"""Run the shipped Unix parser without installer, transport or service work.

Only synthetic input is used. The terminal controls give the child a real PTY
on stdin, so a hidden read/prompt cannot pass merely because stdin was closed.
"""

import json
import os
from pathlib import Path
import pty
import signal
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_ARGUMENT_SOURCE", ROOT / "scripts/install.sh"))
MESSAGE = "Unknown argument. Use --help for usage information."
CASES_RUN = 0
UNKNOWNS = (
    "synthetic-private-token",
    "--token=synthetic-private-token",
    "--token-file=/private/synthetic-credential",
    "--tokne=synthetic-private-token",
    "--bad=synthetic-private-token\r\n[INFO] forged-status\x1b[2J",
    "https://synthetic-user:synthetic-password@pulse.invalid/",
    "/private/synthetic-customer/credential",
    "--not-a-pulse-option",
)


class AgentArgumentErrors(unittest.TestCase):
    def run_installer(self, arguments, terminal=False):
        global CASES_RUN
        CASES_RUN += 1
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            bin_dir = root / "bin"
            bin_dir.mkdir(mode=0o700)
            marker = root / "activity"
            # Any accidental progression past argument admission must be
            # observable and inert. No real network or platform call is made.
            for name in ("curl", "wget", "systemctl", "launchctl", "service",
                         "rc-service", "sudo", "uname", "id", "stat"):
                peer = bin_dir / name
                peer.write_text('#!/bin/sh\nprintf attempted > "$ACTIVITY_MARKER"\nexit 125\n')
                peer.chmod(0o700)
            token = root / "caller-token"
            token.write_bytes(b"synthetic-retained-credential\n")
            token.chmod(0o600)
            before = (token.read_bytes(), token.stat())
            arguments = [str(token) if arg == "{token_file}" else arg for arg in arguments]
            env = {"PATH": str(bin_dir) + ":/usr/bin:/bin", "HOME": str(root),
                   "TMPDIR": str(root), "LC_ALL": "C", "TERM": "dumb",
                   "ACTIVITY_MARKER": str(marker)}
            master = slave = None
            if terminal:
                master, slave = pty.openpty()
                self.assertTrue(os.isatty(slave), "fixture did not provide a terminal")
            try:
                process = subprocess.Popen(
                    ["bash", str(SOURCE), *arguments], env=env,
                    stdin=slave if terminal else subprocess.DEVNULL,
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
                try:
                    output, errors = process.communicate(timeout=2)
                except subprocess.TimeoutExpired:
                    # This is only this fixture's owned process group. Drain
                    # it before closing the PTY or temporary directory.
                    os.killpg(process.pid, signal.SIGKILL)
                    process.communicate()
                    self.fail("argument handling waited for terminal input")
            finally:
                for descriptor in (master, slave):
                    if descriptor is not None:
                        os.close(descriptor)
            self.assertFalse(marker.exists(), "argument handling reached host or credential work")
            self.assertEqual(token.read_bytes(), before[0], "caller credential bytes changed")
            after = token.stat()
            self.assertEqual((after.st_mode, after.st_ino, after.st_size),
                             (before[1].st_mode, before[1].st_ino, before[1].st_size),
                             "caller credential identity or permissions changed")
            self.assertEqual(set(root.iterdir()), {bin_dir, token}, "unexpected installer state")
            return process.returncode, output.decode(), errors.decode()

    def assert_unknown(self, arguments, output_format, terminal=False):
        code, output, errors = self.run_installer(arguments, terminal)
        self.assertEqual(code, 16, "syntax error did not use missing-argument exit 16")
        self.assertEqual(errors, "", "syntax error escaped the fixed diagnostic")
        if output_format == "json":
            self.assertEqual(json.loads(output), {
                "phase": "error", "code": "install_failed", "message": MESSAGE, "exitCode": 16})
        else:
            self.assertEqual(output, "[ERROR] " + MESSAGE + "\n")

    def test_unknown_input_is_private_in_every_output_and_flag_order(self):
        for value in UNKNOWNS:
            for output_format in ("text", "json"):
                for placement in ("before", "after", "absent"):
                    with self.subTest(value=UNKNOWNS.index(value), output=output_format, flag=placement):
                        args = ["--output", output_format, "--token", "synthetic-known-token",
                                "--token-file", "{token_file}"]
                        if placement == "before":
                            args.append("--non-interactive")
                        args.append(value)
                        if placement == "after":
                            args.append("--non-interactive")
                        self.assert_unknown(args, output_format)

    def test_unknown_input_never_waits_on_a_terminal(self):
        for index, value in enumerate(UNKNOWNS):
            for output_format in ("text", "json"):
                with self.subTest(value=index, output=output_format):
                    # The trailing flag has not been parsed when rejection
                    # occurs. No key or newline is supplied to the terminal.
                    self.assert_unknown(["--output", output_format, value, "--non-interactive"],
                                        output_format, terminal=True)

    def test_supported_help_still_exits_without_reading_option_values(self):
        values = ([], ["--non-interactive"], ["--token", "synthetic-known-token"],
                  ["--token-file", "{token_file}"], ["--health-addr", ""],
                  ["--disk-exclude", "literal value with spaces"])
        for output_format in ("text", "json"):
            for index, args in enumerate(values):
                with self.subTest(output=output_format, value=index):
                    code, output, errors = self.run_installer(["--output", output_format, *args, "--help"])
                    self.assertEqual(code, 0)
                    self.assertIn("Usage:", output)
                    self.assertNotIn("synthetic-known-token", output)
                    self.assertNotIn("literal value with spaces", output)
                    self.assertEqual(errors, "")


if __name__ == "__main__":
    result = unittest.main(exit=False).result
    print(f"agent argument errors: {CASES_RUN} closed scenarios observed", flush=True)
    if not result.wasSuccessful() or CASES_RUN != 76:
        sys.exit(1)
