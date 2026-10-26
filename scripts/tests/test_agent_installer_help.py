#!/usr/bin/env python3
"""Exercise the shipped CLI guidance without installing or contacting a server."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "scripts/install.sh"
TOKEN = "synthetic-help-token-not-for-argv"


def help_text(flag="--help"):
    with tempfile.TemporaryDirectory() as directory:
        home = Path(directory)
        tools = home / "tools"
        tools.mkdir()
        marker = home / "unexpected-operation"
        # Help must return before network, privilege or service operations.
        for name in ("curl", "wget", "sudo", "systemctl", "launchctl", "service"):
            tool = tools / name
            tool.write_text('#!/bin/sh\nprintf "%s\\n" "$0" >> "$HELP_OPERATION_LOG"\nexit 91\n')
            tool.chmod(0o700)
        env = dict(os.environ, HOME=str(home), PULSE_TOKEN=TOKEN,
                   PATH=str(tools) + os.pathsep + os.environ["PATH"],
                   HELP_OPERATION_LOG=str(marker))
        result = subprocess.run([shutil.which("bash"), str(INSTALLER), flag],
                                env=env, capture_output=True, text=True, timeout=5)
        if result.returncode or result.stderr or marker.exists():
            raise AssertionError(f"help was not side-effect free: {result.returncode}, {result.stderr!r}")
        if TOKEN in result.stdout or str(home) in result.stdout:
            raise AssertionError("help expanded caller data instead of printing reusable guidance")
        return result.stdout


def examples(text):
    usage = text.split("Usage:\n", 1)[1].split("\n\n", 1)[0]
    commands = [line.strip().removesuffix(" [options]") for line in usage.splitlines()
                if line.strip()]
    if len(commands) != 2 or not all(line.startswith("bash ") for line in commands):
        raise AssertionError("expected install and saved-state update commands")
    return commands


class AgentInstallerHelpTest(unittest.TestCase):
    def test_both_help_flags_return_before_mutation(self):
        self.assertEqual(help_text("--help"), help_text("-h"))

    def test_help_prefers_private_file_and_explains_preparation(self):
        text = help_text()
        self.assertLess(text.index("--token-file <path>"), text.index("--token <token>"))
        for fact in ("verified HTTPS", "inspect it before running", "private-file preparation",
                     "root/admin shell", "caller owns input-file cleanup",
                     "exposes the token in process arguments", "#private-file-installation-linux-macos-and-nas"):
            self.assertIn(fact, text)
        usage = "\n".join(examples(text))
        self.assertNotRegex(usage, r"--token(?:\s|=)|http://|\|\s*(?:sudo\s+)?bash")
        self.assertIn('https://pulse.example.com --token-file "$HOME/.config/pulse/agent-token"', usage)

    def test_help_links_to_immutable_private_file_preparation(self):
        links = re.findall(r"https://github\.com/\S+", help_text())
        self.assertEqual(len(links), 1)
        self.assertRegex(
            links[0],
            r"^https://github\.com/rcourtman/Pulse/blob/[0-9a-f]{40}/"
            r"docs/UNIFIED_AGENT\.md#private-file-installation-linux-macos-and-nas$",
        )
        self.assertIn("### Private-file installation: Linux, macOS and NAS",
                      (ROOT / "docs/UNIFIED_AGENT.md").read_text())

    def test_source_header_and_shipped_help_recommend_the_same_commands(self):
        header = INSTALLER.read_text().split("# Usage:\n", 1)[1].split("\n# Options:", 1)[0]
        commands = [line.removeprefix("#   ").removesuffix(" [options]")
                    for line in header.splitlines() if line.startswith("#   bash ")]
        self.assertEqual(commands, examples(help_text()))
        self.assertNotRegex(header, r"--token(?:\s|=)|http://|curl[^\n]*\|")
        self.assertIn("inspect it first", header)

    def test_copied_help_commands_preserve_private_paths_and_saved_update(self):
        commands = examples(help_text())
        with tempfile.TemporaryDirectory(prefix="pulse help space ") as directory:
            home = Path(directory)
            private = home / ".config/pulse"
            private.mkdir(parents=True, mode=0o700)
            token = private / "agent-token"
            token.write_text(TOKEN + "\n")
            token.chmod(0o600)
            receipt = home / "argv.json"
            # Only a closed argument receiver, never the real installer.
            stub = private / "agent-install.sh"
            stub.write_text('''#!/usr/bin/env bash
set -eu
python3 - "$@" <<'PY'
import json, os, pathlib, stat, sys
args = sys.argv[1:]
if args != ["--update"]:
    assert args == ["--url", "https://pulse.example.com", "--token-file",
                    str(pathlib.Path.home() / ".config/pulse/agent-token")]
    token = pathlib.Path(args[-1])
    assert stat.S_IMODE(token.stat().st_mode) == 0o600
    assert token.read_text().strip() == os.environ["EXPECTED_TOKEN"]
assert os.environ["EXPECTED_TOKEN"] not in args
pathlib.Path(os.environ["ARGV_RECEIPT"]).write_text(json.dumps(args))
PY
''')
            env = dict(os.environ, HOME=str(home), EXPECTED_TOKEN=TOKEN, ARGV_RECEIPT=str(receipt))
            for command in commands:
                with self.subTest(command=command):
                    receipt.unlink(missing_ok=True)
                    result = subprocess.run(["bash", "-eu", "-c", command], env=env,
                                            capture_output=True, text=True, timeout=5)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertTrue(receipt.is_file(), "copied example did not invoke its installer")
                    args = json.loads(receipt.read_text())
                    self.assertEqual(args, ["--update"] if command.endswith("--update") else
                                     ["--url", "https://pulse.example.com", "--token-file", str(token)])
                    self.assertNotIn(TOKEN, result.stdout + result.stderr + receipt.read_text())
                    self.assertEqual(token.read_text(), TOKEN + "\n", "caller-owned credential changed")


if __name__ == "__main__":
    unittest.main()
