#!/usr/bin/env python3
"""Exercise documented agent setup without installing a service or real token.

TLS listeners are guest-local: run with pulse-worker-source-proof. Windows
checks cover the documented source contract, not native ACL/service execution.
"""

import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tempfile
import unittest

from test_pbs_docs import certificate, fixture_environment, server


ROOT = Path(__file__).resolve().parents[2]
NAMES = ("UNIFIED_AGENT.md", "TEMPERATURE_MONITORING.md")
TOKEN = "synthetic-agent-docs-secret"
CLEANUP_PATH = "/rcourtman/Pulse/main/scripts/uninstall-sensor-proxy.sh"


def blocks(name, language="bash"):
    text = re.sub(r"^ {1,3}", "", (ROOT / "docs" / name).read_text(), flags=re.MULTILINE)
    return re.findall(r"```" + language + r"\n(.*?)```", text, re.DOTALL)


def recipe(name, needle):
    found = [block for block in blocks(name) if needle in block]
    if len(found) != 1:
        raise AssertionError(f"expected one {name} recipe containing {needle!r}")
    return found[0]


def run_recorded_recipe(command, env):
    # A copied command may exit successfully without invoking the installer.
    # Its evidence must not come from the preceding profile or uninstall.
    receipt = Path(env["INSTALL_RECEIPT"])
    receipt.unlink(missing_ok=True)
    result = subprocess.run(["bash", "-eu", "-c", command], env=env,
                            capture_output=True, timeout=10)
    args = json.loads(receipt.read_text()) if receipt.is_file() else None
    return result, args


class AgentDocsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory()
        cls.cert, cls.key = certificate(Path(cls.temporary.name), "agent-docs")
        cls.other_cert, _ = certificate(Path(cls.temporary.name), "untrusted")

    @classmethod
    def tearDownClass(cls):
        cls.temporary.cleanup()

    def test_mirrors_and_no_secret_or_unchecked_bootstrap_commands(self):
        for name in NAMES:
            with self.subTest(name=name):
                doc = (ROOT / "docs" / name).read_bytes()
                self.assertEqual(doc, (ROOT / "frontend-modern/public/docs" / name).read_bytes())
                shell = "\n".join(blocks(name))
                self.assertNotRegex(shell, r"--token(?:\s|=)|PULSE_TOKEN=|curl[^\n]*http://")
                self.assertNotRegex(shell, r"curl[^`]*\|\s*(?:sudo\s+)?bash")
                boundary = ("Stop if the download fails" if name == "UNIFIED_AGENT.md"
                            else "Stop if preparation or download fails")
                self.assertIn(boundary, doc.decode())
        windows = "\n".join(blocks(NAMES[0], "powershell"))
        self.assertNotRegex(windows, r"(?i)\biex\b|\birm\b|Invoke-Expression|http://|\$env:PULSE_TOKEN\s*=")

    def test_changed_unix_recipes_parse(self):
        count = 0
        for name in NAMES:
            for block in blocks(name):
                if any(word in block for word in ("agent-install.sh", "agent-token", "sensor-proxy-uninstall.sh")):
                    with self.subTest(name=name, command=block):
                        subprocess.run(["bash", "-n", "-c", block], check=True, capture_output=True)
                    count += 1
        self.assertEqual(count, 18)

    def test_private_file_preparation_preserves_existing_token(self):
        command = recipe(NAMES[0], 'vi "$HOME/.config/pulse/agent-token"')
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            tools = home / "tools"
            tools.mkdir()
            editor = tools / "vi"
            editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ -f "$1" ]\n')
            editor.chmod(0o700)
            env = fixture_environment(home)
            env["PATH"] = f"{tools}:{os.environ['PATH']}"
            token = home / ".config/pulse/agent-token"
            for existing in (False, True):
                if existing:
                    token.write_text(TOKEN + "\n")
                    token.chmod(0o644)
                    token.parent.chmod(0o755)
                result = subprocess.run(["bash", "-eu", "-c", command], env=env, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual(stat.S_IMODE(token.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(token.parent.stat().st_mode), 0o700)
                self.assertEqual(token.read_text(), TOKEN + "\n" if existing else "")
                self.assertNotIn(TOKEN.encode(), result.stdout + result.stderr)

    def test_windows_file_handoff_matches_installer_contract(self):
        windows = "\n".join(blocks(NAMES[0], "powershell"))
        for expected in ("-ErrorAction Stop", "/inheritance:r", "*${userSid}:(OI)(CI)F",
                         "*S-1-5-18:(OI)(CI)F", "$LASTEXITCODE -ne 0",
                         "-FilePath notepad.exe", "-Wait", "-OutFile $installerFile",
                         "Remove-Item Env:PULSE_TOKEN", "-TokenFile $tokenFile"):
            self.assertIn(expected, windows)
        source = (ROOT / "scripts/install.ps1").read_text()
        self.assertIn('[string]$TokenFile = $env:PULSE_TOKEN_FILE,', source)
        self.assertIn('if ([string]::IsNullOrWhiteSpace($Token) -and -not [string]::IsNullOrWhiteSpace($TokenFile))', source)
        self.assertIn('(Get-Content -Path $resolvedTokenFile -Raw -ErrorAction Stop).Trim()', source)

    def download(self, home, command, port, ca=None, hostname="localhost"):
        private = home / ".config/pulse"
        private.mkdir(parents=True, exist_ok=True, mode=0o700)
        tools = home / "tools"
        tools.mkdir(exist_ok=True)
        recorder = tools / "curl"
        recorder.write_text(
            "#!/usr/bin/env python3\nimport json, os, sys\nfrom pathlib import Path\n"
            "Path(os.environ['CURL_RECEIPT']).write_text(json.dumps(sys.argv[1:]))\n"
            "os.execv(os.environ['REAL_CURL'], [os.environ['REAL_CURL'], *sys.argv[1:]])\n"
        )
        recorder.chmod(0o700)
        env = fixture_environment(home)
        env.update(PATH=f"{tools}:{os.environ['PATH']}", REAL_CURL=shutil.which("curl"),
                   CURL_RECEIPT=str(home / "curl-argv.json"))
        command = command.replace("https://pulse.example.com", f"https://{hostname}:{port}")
        command = command.replace("https://raw.githubusercontent.com", f"https://{hostname}:{port}")
        if ca is not None:
            if "curl --disable " in command:
                command = command.replace("curl --disable ", f'curl --disable --cacert "{ca}" ', 1)
            else:
                command = command.replace("curl ", f'curl --cacert "{ca}" ', 1)
        result = subprocess.run(["bash", "-eu", "-c", command], env=env, capture_output=True, timeout=20)
        argv = json.loads((home / "curl-argv.json").read_text())
        self.assertNotIn(TOKEN, " ".join(argv))
        self.assertNotIn(TOKEN.encode(), result.stdout + result.stderr)
        for argument in ("--connect-timeout", "--max-time", "--fail", "--output"):
            self.assertIn(argument, argv)
        self.assertNotIn("--insecure", argv)
        self.assertNotIn("-k", argv)
        if "sensor-cleanup-download.XXXXXX" in command:
            self.assertEqual(argv[0], "--disable")
        return result

    def test_downloads_require_verified_tls_and_success(self):
        for name, path, needle, output in (
            (NAMES[0], "/install.sh", '--output "$HOME/.config/pulse/agent-install.sh"', "agent-install.sh"),
            (NAMES[1], CLEANUP_PATH, 'helper_file="$config_dir/sensor-proxy-uninstall.sh"', "sensor-proxy-uninstall.sh"),
        ):
            for status, ca, hostname, expected in (
                (200, self.cert, "localhost", 0),
                (403, self.cert, "localhost", 22),
                (200, None, "localhost", 60),
                (200, self.other_cert, "localhost", 60),
                (200, self.cert, "127.0.0.1", 60),
            ):
                with self.subTest(name=name, status=status, expected=expected):
                    with tempfile.TemporaryDirectory() as temporary:
                        with server(self.cert, self.key, status, b"# harmless installer fixture\n", path) as (port, requests):
                            home = Path(temporary)
                            result = self.download(home, recipe(name, needle), port, ca, hostname)
                            self.assertEqual(result.returncode, expected, result.stderr.decode())
                            if expected == 0:
                                self.assertEqual((home / ".config/pulse" / output).read_bytes(), b"# harmless installer fixture\n")
                            if expected == 60:
                                self.assertEqual(requests, [])
                            for requested_path, headers in requests:
                                self.assertEqual(requested_path, path)
                                self.assertNotIn("Authorization", headers)

    def test_every_profile_passes_only_private_file_not_token_arguments(self):
        profiles = []
        for name in NAMES:
            profiles.extend(block for block in blocks(name) if 'bash "$HOME/.config/pulse/agent-install.sh"' in block and "--token-file" in block)
        self.assertEqual(len(profiles), 11)
        fixture = '''#!/bin/bash
python3 - "$@" <<'PY'
import json, os, stat, sys
from pathlib import Path
args = sys.argv[1:]
token = Path(args[args.index('--token-file') + 1])
assert args.count('--token-file') == 1
assert token.read_text().strip() == os.environ['EXPECTED_TOKEN']
assert stat.S_IMODE(token.stat().st_mode) == 0o600
assert os.environ['EXPECTED_TOKEN'] not in ' '.join(args)
assert '--token' not in args
Path(os.environ['INSTALL_RECEIPT']).write_text(json.dumps(args))
PY
'''
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            private = home / ".config/pulse"
            private.mkdir(parents=True, mode=0o700)
            (private / "agent-install.sh").write_text(fixture)
            token = private / "agent-token"
            token.write_text(TOKEN + "\n")
            token.chmod(0o600)
            env = fixture_environment(home)
            env.update(EXPECTED_TOKEN=TOKEN, INSTALL_RECEIPT=str(home / "installer-argv.json"))
            for command in profiles:
                with self.subTest(command=command):
                    result, args = run_recorded_recipe(command, env)
                    self.assertEqual(result.returncode, 0, result.stderr.decode())
                    self.assertIsNotNone(args, "copied profile did not invoke its installer")
                    self.assertEqual(args[:4], ["--url", "https://pulse.example.com", "--token-file", str(token)])
                    self.assertNotIn(TOKEN.encode(), result.stdout + result.stderr)
            expected_profiles = [[], ["--enable-proxmox", "--proxmox-type", "pve"],
                                 ["--enable-proxmox", "--proxmox-type", "pbs"],
                                 ["--enable-docker"], ["--enable-docker=false"],
                                 ["--enable-kubernetes"], ["--enable-host=false", "--enable-docker"],
                                 ["--disable-auto-update"], ["--enable-proxmox"]]
            actual_profiles = [re.findall(r"--(?:enable-[a-z]+(?:=false)?|disable-auto-update|proxmox-type\s+(?:pve|pbs))", p) for p in profiles]
            for expected in expected_profiles:
                # Compare the actual argv tokens, retaining the type's value.
                self.assertIn(" ".join(expected), [" ".join(a) for a in actual_profiles])
        source = (ROOT / "scripts/install.sh").read_text()
        self.assertIn('--token-file) TOKEN_FILE_PATH="$2"; shift 2 ;;', source)
        self.assertIn('read_collector_token_file_safely "$TOKEN_FILE_PATH" true', source)

    def test_skipped_or_failed_recipe_cannot_reuse_a_previous_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            receipt = home / "argv.json"
            env = fixture_environment(home)
            env["INSTALL_RECEIPT"] = str(receipt)
            for command, expected_exit in ((":", 0), ("exit 7", 7)):
                with self.subTest(command=command):
                    receipt.write_text(json.dumps(["--previous-profile"]))
                    result, args = run_recorded_recipe(command, env)
                    self.assertEqual(result.returncode, expected_exit)
                    self.assertIsNone(args)
                    self.assertFalse(receipt.exists())

    def test_retarget_uninstall_and_cleanup_use_saved_script_without_new_token(self):
        expected = (["--retarget", "--url", "https://pulse.example.com"], ["--uninstall"],
                    ["--uninstall", "--purge", "--local-only"],
                    ["--uninstall", "--purge", "--remove-proxmox-access", "--local-only"])
        commands = [recipe(NAMES[0], "--retarget --url"), recipe(NAMES[0], 'agent-install.sh" --uninstall'),
                    recipe(NAMES[1], "--uninstall --purge --local-only"),
                    recipe(NAMES[1], "--uninstall --purge --remove-proxmox-access")]
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            private = home / ".config/pulse"
            private.mkdir(parents=True, mode=0o700)
            fixture = '#!/bin/bash\npython3 - "$@" <<\'PY\'\nimport json, os, sys\nfrom pathlib import Path\nPath(os.environ["INSTALL_RECEIPT"]).write_text(json.dumps(sys.argv[1:]))\nPY\n'
            for name in ("agent-install.sh", "sensor-proxy-uninstall.sh"):
                (private / name).write_text(fixture)
            env = fixture_environment(home)
            env["INSTALL_RECEIPT"] = str(home / "argv.json")
            for command, args in zip(commands, expected):
                result, recorded_args = run_recorded_recipe(command, env)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual(recorded_args, args)


if __name__ == "__main__":
    unittest.main()
