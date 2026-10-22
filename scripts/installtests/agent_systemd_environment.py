#!/usr/bin/env python3
"""Read back installer environment values with systemd's real unit parser.

Only disposable files and `systemd-analyze verify` are used. No unit is
installed, started, reloaded or connected to a running service manager. This
proves the rendered unit's interpretation, not installed TLS/agent recovery.
"""

import hashlib
import json
import os
from pathlib import Path
import plistlib
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_AGENT_ENVIRONMENT_SOURCE", ROOT / "scripts/install.sh"))
PROOF = os.environ.get("PULSE_AGENT_ENVIRONMENT_PROOF")
VALUES = (
    "", "/etc/pulse/ca.pem", "/etc/pulse/CA with spaces.pem",
    '/etc/pulse/CA "quoted".pem', "/etc/pulse/CA 'single quotes'.pem",
    r"/etc/pulse/CA\literal\n\t.pem", "/etc/pulse/%n-%i-%h-%%.pem",
    "/etc/pulse/$HOME-${USER}.pem", "  /etc/pulse/ca.pem  ",
    "/etc/pulse/tab\tca.pem", "/etc/pulse/line\nca.pem",
    "/etc/pulse/return\rca.pem", "/etc/pulse/CRLF\r\nca.pem",
    "/etc/pulse/certificat-é-根.pem", "/etc/pulse/#hash;semicolon=equals.pem",
    '/etc/pulse/$(touch SHOULD_NOT_EXIST)-`touch SHOULD_NOT_EXIST`.pem',
    "/etc/pulse/" + "".join(chr(value) for value in range(1, 32)) + "\x7f.pem",
    '/etc/pulse/é-根 "quoted" \\ %n\nca.pem',
)


def functions():
    source = SOURCE.read_text()
    names = ["xml_escape", "append_plist_arg", "service_env_has_key", "shell_export_value",
             "append_service_env", "finalize_plist_env_block",
             "systemd_agent_requires_lxc_attach", "systemd_agent_may_attach_lxc",
             "render_systemd_agent_unit"]
    # Let the same semantic controls exercise the exact older runtime for an
    # adverse comparison; absence of the new helper does not skip any check.
    if "systemd_environment_value() {" in source:
        names.append("systemd_environment_value")
    result = []
    for name in names:
        match = re.search(r"^" + name + r"\(\) \{\n.*?^\}", source, re.M | re.S)
        if match is None:
            raise AssertionError("missing installer function: " + name)
        result.append(match.group())
    return "\n".join(result)


class AgentSystemdEnvironment(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.analyze = shutil.which("systemd-analyze")
        if cls.analyze is None:
            raise RuntimeError("systemd-analyze is required; never substitute an exit-only or model parser")
        cls.version = subprocess.run([cls.analyze, "--version"], check=True,
                                     capture_output=True, text=True, timeout=5).stdout.splitlines()[0]
        print("native parser: " + cls.version, flush=True)

    def render(self, root, entries, replacement_mode="default"):
        unit = root / "pulse-environment-fixture.service"
        script = r'''
set -euo pipefail
if [[ "$BASH_REPLACEMENT_MODE" != default ]] && shopt -s patsub_replacement 2>/dev/null; then
    [[ "$BASH_REPLACEMENT_MODE" != disabled ]] || shopt -u patsub_replacement
    shopt -p patsub_replacement > "$FIXTURE/bash-mode" || :
fi
APPLIED_SERVICE_ENV_KEYS="|"
SYSTEMD_ENV_LINES=""; SHELL_EXPORT_LINES=""; UPSTART_ENV_LINES=""; SED_EXPORT_LINES=""
PLIST_ENV_ENTRIES=""; PLIST_ENV_BLOCK=""
ENABLE_COMMANDS=false; ENABLE_PROXMOX=false; LEAST_PRIVILEGE=false
GRANT_SMART=false; GRANT_PCT=false
''' + functions() + r'''
PLIST_ARGS=""
append_plist_arg /usr/local/bin/pulse-agent
append_plist_arg --observers-file
append_plist_arg "$2"
printf '<plist version="1.0"><dict><key>ProgramArguments</key><array>%s</array></dict></plist>' "$PLIST_ARGS" > "$FIXTURE/arguments.plist"
while [[ $# -gt 0 ]]; do
    append_service_env "$1" "$2"
    shift 2
done
finalize_plist_env_block
render_systemd_agent_unit "$FIXTURE/unit.service" /bin/true fixture-argument network.target "" "" ""
printf '%s' "$SHELL_EXPORT_LINES" > "$FIXTURE/exports.sh"
printf '%s' "$UPSTART_ENV_LINES" > "$FIXTURE/upstart.txt"
printf '%s' "$SED_EXPORT_LINES" > "$FIXTURE/sed.txt"
printf '<plist version="1.0"><dict>%s</dict></plist>' "$PLIST_ENV_BLOCK" > "$FIXTURE/environment.plist"
'''
        env = {"PATH": os.defpath, "HOME": str(root), "LC_ALL": "C.UTF-8", "FIXTURE": str(root),
               "BASH_REPLACEMENT_MODE": replacement_mode}
        arguments = [part for entry in entries for part in entry]
        result = subprocess.run(["bash", "-c", script, "fixture", *arguments], env=env, cwd=root,
                                capture_output=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        self.assertEqual(result.stdout + result.stderr, b"")
        (root / "unit.service").rename(unit)
        return unit

    def consume(self, root, unit, label):
        # verify parses in a private manager without talking to PID 1. Its
        # debug dump contains the effective values, unlike its exit status.
        env = {"PATH": os.defpath, "HOME": str(root), "LC_ALL": "C.UTF-8",
               "SYSTEMD_LOG_LEVEL": "debug", "SYSTEMD_COLORS": "0", "SYSTEMD_OFFLINE": "1"}
        result = subprocess.run([self.analyze, "--man=no", "--generators=no", "verify", str(unit)],
                                env=env, cwd=root, capture_output=True, timeout=15)
        stdout = result.stdout.decode()
        stderr = result.stderr.decode()
        marker = "\t-> Unit " + unit.name + ":\n"
        self.assertEqual(stdout.count(marker), 1, "missing exact-unit native dump")
        dump = stdout.split(marker, 1)[1].split("\n\t-> Unit ", 1)[0]
        # Read the raw effective environment, preserving embedded newlines.
        # RuntimeDirectoryPreserve is a following parser-owned field, not a
        # second interpretation of the original Environment= source text.
        section = (dump.split("\t\tEnvironment: ", 1)[1].split("\n\t\tRuntimeDirectoryPreserve:", 1)[0]
                   if "\t\tEnvironment: " in dump else None)
        if PROOF:
            proof = Path(PROOF)
            proof.mkdir(parents=True, exist_ok=True)
            prefix = proof / label
            prefix.with_suffix(".service").write_bytes(unit.read_bytes())
            prefix.with_suffix(".stdout").write_bytes(result.stdout)
            prefix.with_suffix(".stderr").write_bytes(result.stderr)
            prefix.with_suffix(".json").write_text(json.dumps({
                "label": label, "parser": self.version, "exit_code": result.returncode,
                "source_sha256": hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
                "unit_sha256": hashlib.sha256(unit.read_bytes()).hexdigest(),
                "effective_environment": section,
            }, ensure_ascii=False, indent=2) + "\n")
        self.assertEqual(result.returncode, 0, stderr)
        self.assertIsNotNone(section, "the native parser lost the entire environment assignment")
        for warning in ("Invalid environment assignment", "Invalid syntax", "Unbalanced quoting",
                        "Failed to resolve specifiers", "Unknown escape", "Unknown key"):
            self.assertNotIn(warning, stderr)
        self.assertEqual(dump.count("Command Line: /bin/true fixture-argument\n"), 1)
        self.assertNotIn("Command Line: /bin/false", dump)
        self.assertIn("\t\tPrivateTmp: yes\n", dump)
        self.assertIn("\t\tRestrictSUIDSGID: yes\n", dump)
        self.assertIn("\t\tUMask: 0077\n", dump)
        # This systemd version does not include NoNewPrivileges in its dump.
        # Preserve the directive itself without inventing an effective readback.
        self.assertEqual(unit.read_text().count("\nNoNewPrivileges=true\n"), 1)
        self.assertFalse((root / "SHOULD_NOT_EXIST").exists())
        return section

    def test_literal_ca_paths_survive_the_native_parser(self):
        for number, value in enumerate(VALUES):
            with self.subTest(value=value), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                unit = self.render(root, [("SSL_CERT_FILE", value)])
                parsed = self.consume(root, unit, "literal-%02d" % number)
                self.assertEqual(parsed, "SSL_CERT_FILE=" + value)
                self.assertEqual(unit.read_text().count("\nEnvironment="), 1)

    def test_embedded_directives_remain_one_environment_value(self):
        value = '/tmp/ca"\nEnvironment=INJECTED=value\nExecStart=\nExecStart=/bin/false\n#end.pem'
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            unit = self.render(root, [("SSL_CERT_FILE", value)])
            parsed = self.consume(root, unit, "directives")
            self.assertEqual(parsed, "SSL_CERT_FILE=" + value)
            self.assertEqual(unit.read_text().count("\nEnvironment="), 1)
            self.assertEqual(unit.read_text().count("\nExecStart="), 1)

    def test_multiple_values_and_first_key_ownership_are_preserved(self):
        entries = [("SSL_CERT_FILE", '/tmp/CA "quoted" %n.pem'),
                   ("DOCKER_HOST", "unix:///run/user/1000/docker.sock"),
                   ("XDG_RUNTIME_DIR", "/run/user/1000"),
                   ("PULSE_AGENT_HELPER_SOCKET", "/run/pulse-agent/helper.sock")]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            unit = self.render(root, entries + [("SSL_CERT_FILE", "/tmp/ignored.pem")])
            parsed = self.consume(root, unit, "multiple")
            self.assertEqual(parsed, "\n\t\tEnvironment: ".join(key + "=" + value for key, value in entries))
            self.assertEqual(unit.read_text().count("\nEnvironment="), len(entries))
            self.assertNotIn("ignored", unit.read_text())

    def test_other_service_formats_keep_their_literal_contract(self):
        for value in (VALUES[1], VALUES[3], VALUES[5], VALUES[6], VALUES[-1], 'CA & <root> "quoted".pem'):
            with self.subTest(value=value), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                self.render(root, [("SSL_CERT_FILE", value)])
                script = (root / "exports.sh").read_text() + '\nprintf "%s" "$SSL_CERT_FILE"\n'
                result = subprocess.run(["sh", "-c", script], cwd=root,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual(result.stdout.decode(), value)
                self.assertEqual(result.stderr, b"")
                plist = plistlib.loads((root / "environment.plist").read_bytes())
                self.assertEqual(plist, {"EnvironmentVariables": {"SSL_CERT_FILE": value}})
                self.assertEqual(plistlib.loads((root / "arguments.plist").read_bytes()), {
                    "ProgramArguments": ["/usr/local/bin/pulse-agent", "--observers-file", value]})
                self.assertEqual((root / "upstart.txt").read_text(), "\nenv SSL_CERT_FILE=" + value)
                self.assertEqual((root / "sed.txt").read_text(), (root / "exports.sh").read_text().strip())
                self.assertFalse((root / "SHOULD_NOT_EXIST").exists())

    def test_plist_metacharacters_remain_literal_across_bash_replacement_modes(self):
        value = 'CA & <root> "quoted" and &lt;already-text&gt;.pem'
        for mode in ("enabled", "disabled"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                self.render(root, [("SSL_CERT_FILE", value)], mode)
                plist = plistlib.loads((root / "environment.plist").read_bytes())
                self.assertEqual(plist, {"EnvironmentVariables": {"SSL_CERT_FILE": value}})
                self.assertEqual(plistlib.loads((root / "arguments.plist").read_bytes()), {
                    "ProgramArguments": ["/usr/local/bin/pulse-agent", "--observers-file", value]})
                if (root / "bash-mode").exists():
                    self.assertEqual((root / "bash-mode").read_text(),
                                     "shopt -" + ("s" if mode == "enabled" else "u") + " patsub_replacement\n")


if __name__ == "__main__":
    unittest.main()
