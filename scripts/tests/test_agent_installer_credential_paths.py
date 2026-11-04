#!/usr/bin/env python3
"""Exercise credential-file argument admission without any install operation."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import agent_installer_test_support


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "scripts/install.sh"
TOKEN = "synthetic-argument-test-not-a-credential"
OPTIONS = ("--token-file", "--action-token-file")


class AgentInstallerCredentialPathsTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse credential paths ")
        self.addCleanup(self.temporary.cleanup)
        self.home = Path(self.temporary.name)
        self.operations = self.home / "unexpected-operation"
        tools = self.home / "tools"
        tools.mkdir()
        # Both real-script refusals and the parser witness must precede these.
        for name in ("curl", "wget", "sudo", "systemctl", "launchctl", "service"):
            tool = tools / name
            tool.write_text('#!/bin/sh\nprintf "%s\\n" "$0" >> "$ARGUMENT_OPERATION_LOG"\nexit 91\n')
            tool.chmod(0o700)
        self.env = dict(os.environ, HOME=str(self.home), PULSE_TOKEN=TOKEN,
                        PATH=str(tools) + os.pathsep + os.environ["PATH"],
                        ARGUMENT_OPERATION_LOG=str(self.operations))
        # Preserve every byte of the actual installer through its parser. End
        # main there, with an inert witness instead of any lifecycle operation.
        source = INSTALLER.read_text()
        prefix, _ = source.split('if [[ "$RETARGET_ONLY" == "true" && -z "$PULSE_URL" ]]; then', 1)
        self.parser = self.home / "parser.sh"
        self.parser.write_text(prefix + r'''
python3 - "$TOKEN_FILE_PATH" "$ACTION_TOKEN_FILE_PATH" "$HEALTH_ADDR" <<'PY'
import json, sys
print("PARSER_ACCEPTED=" + json.dumps(sys.argv[1:]))
PY
}
main "$@"
''')

    def run_script(self, arguments, *, parser=False, output="json"):
        command = [shutil.which("bash"), str(self.parser if parser else INSTALLER),
                   "--non-interactive", "--output", output, *arguments]
        result = subprocess.run(command, env=self.env, cwd=self.home,
                                capture_output=True, text=True, timeout=5)
        self.assertFalse(self.operations.exists(), "argument parsing attempted an install operation")
        self.assertNotIn(TOKEN, result.stdout + result.stderr, "diagnostic exposed an earlier credential")
        return result

    def assert_refused(self, result, option, output="json"):
        self.assertEqual(result.returncode, 16, result.stdout + result.stderr)
        message = f"{option} requires a non-empty path"
        self.assertNotIn("PARSER_ACCEPTED=", result.stdout)
        self.assertNotIn("unbound variable", result.stdout + result.stderr)
        if output == "json":
            self.assertEqual(json.loads(result.stdout), {
                "phase": "error", "code": "install_failed",
                "message": message, "exitCode": 16,
            })
            self.assertEqual(result.stderr, "")
        else:
            self.assertIn(message, result.stdout + result.stderr)

    def test_missing_paths_have_real_structured_and_text_refusals(self):
        for option in OPTIONS:
            for output in ("json", "text"):
                with self.subTest(option=option, output=output):
                    self.assert_refused(self.run_script([option], output=output), option, output)

    def test_empty_paths_stop_the_real_script_before_later_validation(self):
        # The invalid state directory is a second fail-closed fence for the old
        # script: these controls can never proceed into an actual installation.
        for option in OPTIONS:
            with self.subTest(option=option):
                self.assert_refused(self.run_script([
                    "--token", TOKEN, option, "", "--state-dir", "relative",
                ]), option)

    def test_empty_paths_cannot_fall_through_to_other_credential_routes(self):
        for option in OPTIONS:
            for profile in ([], ["--update"], ["--enable-action-runner"]):
                for before in ([], ["--token", TOKEN], [option, str(self.home / "prior-file")]):
                    with self.subTest(option=option, profile=profile, before=bool(before)):
                        self.assert_refused(self.run_script([
                            *profile, *before, option, "",
                        ], parser=True), option)

    def test_nonempty_paths_remain_literal_and_caller_owned(self):
        path = self.home / "private path\nwith 'quotes';$(not-a-command)"
        path.write_text(TOKEN + "\n")
        path.chmod(0o600)
        for option in OPTIONS:
            for profile in ([], ["--update"], ["--enable-action-runner"]):
                with self.subTest(option=option, profile=profile):
                    result = self.run_script([*profile, "--token", TOKEN, option, str(path)], parser=True)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    parsed = json.loads(result.stdout.removeprefix("PARSER_ACCEPTED="))
                    self.assertEqual(parsed[:2], [str(path), ""] if option == "--token-file" else ["", str(path)])
                    self.assertEqual(path.read_text(), TOKEN + "\n")
                    self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_path_validation_stays_with_the_existing_credential_reader(self):
        # Parsing must not open a path or reinterpret a leading dash. Downstream
        # private-file validation still owns nonexistent and invalid file types.
        for option in OPTIONS:
            for path in (str(self.home / "absent"), "--literal-file-name"):
                with self.subTest(option=option, path=path):
                    result = self.run_script([option, path], parser=True)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertIn(path, json.loads(result.stdout.removeprefix("PARSER_ACCEPTED=")))

    def test_omitted_file_options_preserve_existing_credential_routes(self):
        for profile in ([], ["--update"], ["--enable-action-runner"]):
            for credential in ([], ["--token", TOKEN]):
                with self.subTest(profile=profile, inline=bool(credential)):
                    result = self.run_script([*profile, *credential], parser=True)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(json.loads(result.stdout.removeprefix("PARSER_ACCEPTED="))[:2], ["", ""])

    def test_intentionally_empty_health_address_remains_supported(self):
        result = self.run_script(["--health-addr", ""], parser=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(json.loads(result.stdout.removeprefix("PARSER_ACCEPTED=")), ["", "", ""])

    def test_documented_command_fixture_rejects_dropped_or_changed_file_paths(self):
        source = INSTALLER.read_text()
        assignment = '            TOKEN_FILE_PATH="$2"; shift 2 ;;'
        self.assertEqual(source.count(assignment), 1)
        token = self.home / "documented token"
        token.write_text(TOKEN + "\n")
        token.chmod(0o600)
        receipt = self.home / "documented-command.json"
        self.env.update(EXPECTED_TOKEN=TOKEN, INSTALL_RECEIPT=str(receipt))
        for replacement in ('shift 2 ;;', 'TOKEN_FILE_PATH="${2}/changed"; shift 2 ;;'):
            with self.subTest(replacement=replacement):
                changed = self.home / "changed-installer.sh"
                changed.write_text(source.replace(assignment, replacement))
                with patch.object(agent_installer_test_support, "INSTALLER", changed):
                    self.parser.write_text(agent_installer_test_support.credential_path_parser_fixture())
                result = self.run_script(["--token-file", str(token)], parser=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(receipt.exists(), "changed handoff produced passing evidence")

    def test_documented_command_fixture_refuses_a_missing_parser_boundary(self):
        changed = self.home / "changed-installer.sh"
        changed.write_text("#!/bin/bash\nexit 0\n")
        with patch.object(agent_installer_test_support, "INSTALLER", changed):
            with self.assertRaisesRegex(AssertionError, "parser boundary missing"):
                agent_installer_test_support.credential_path_parser_fixture()


if __name__ == "__main__":
    unittest.main()
