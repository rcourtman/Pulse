#!/usr/bin/env python3
"""Exercise copied NAS space guidance without an installer or appliance operation.

Only the guide's shell wrapper runs. Its installer is a synthetic recorder;
no service, real token, log, mount, network or platform state is accessed.
"""

import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/UNIFIED_AGENT.md"
SECRET = "synthetic-storage-docs-token"


def section():
    return DOC.read_text().split('### Installer Fails With "Not enough free disk space"\n', 1)[1].split(
        "\n### Agent Not Updating\n", 1
    )[0]


def recipe(volume):
    commands = re.findall(r"```bash\n(.*?)```", section(), re.DOTALL)
    if len(commands) != 1:
        raise AssertionError("expected one constrained-staging recipe")
    # Preserve the documented recipe; replace only its actual-volume placeholder.
    return commands[0].replace("/share/CACHEDEV1_DATA", shlex.quote(str(volume)))


INSTALLER = '''#!/bin/bash
python3 - "$@" <<'PY'
import json, os, stat, sys
from pathlib import Path
stage = Path(os.environ['TMPDIR'])
receipt = {
    'args': sys.argv[1:], 'staging': str(stage),
    'mode': stat.S_IMODE(stage.stat().st_mode) if stage.is_dir() else None,
}
Path(os.environ['INSTALL_RECEIPT']).write_text(json.dumps(receipt))
if os.environ.get('KEEP_STAGING'):
    (stage / 'diagnostic-note').write_text('synthetic incomplete staging evidence')
sys.exit(int(os.environ.get('INSTALL_EXIT', '0')))
PY
'''


class AgentStorageDocsTest(unittest.TestCase):
    def exercise(self, *, volume_state="present", install_exit="0", keep_staging=False,
                 mktemp_failure=False):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            volume = home / "data volume"
            if volume_state != "missing":
                volume.mkdir()
                if volume_state == "unwritable":
                    volume.chmod(0o500)
            private = home / ".config/pulse"
            private.mkdir(parents=True, mode=0o700)
            token = private / "agent-token"
            token.write_text(SECRET + "\n")
            token.chmod(0o600)
            (private / "agent-install.sh").write_text(INSTALLER)
            identity = home / "saved-agent-id"
            identity.write_text("unchanged-synthetic-agent\n")
            log = home / "legacy-agent.log"
            log.write_text("original diagnostic evidence\n")
            shared = home / "inherited-shared-staging"
            shared.mkdir(mode=0o777)
            (shared / "keep").write_text("unrelated staging evidence")
            receipt = home / "installer.json"
            env = dict(os.environ, HOME=str(home), TMPDIR=str(shared),
                       INSTALL_RECEIPT=str(receipt), INSTALL_EXIT=install_exit)
            if keep_staging:
                env["KEEP_STAGING"] = "1"
            if mktemp_failure:
                tools = home / "tools"
                tools.mkdir()
                mktemp = tools / "mktemp"
                mktemp.write_text("#!/bin/sh\nprintf 'synthetic staging creation failed\\n' >&2\nexit 73\n")
                mktemp.chmod(0o700)
                env["PATH"] = f"{tools}:{os.environ['PATH']}"
            command = recipe(volume)
            subprocess.run(["bash", "-n", "-c", command], check=True, capture_output=True)
            try:
                result = subprocess.run(["bash", "-c", command], env=env, umask=0o022,
                                        text=True, capture_output=True, timeout=10)
                self.assertNotIn(SECRET, result.stdout + result.stderr)
                self.assertEqual(token.read_text(), SECRET + "\n")
                self.assertEqual(stat.S_IMODE(token.stat().st_mode), 0o600)
                self.assertEqual(identity.read_text(), "unchanged-synthetic-agent\n")
                self.assertEqual(log.read_text(), "original diagnostic evidence\n")
                self.assertEqual((shared / "keep").read_text(), "unrelated staging evidence")
                record = json.loads(receipt.read_text()) if receipt.exists() else None
                remaining = []
                if volume.exists():
                    remaining = sorted(str(p.relative_to(volume)) for p in volume.rglob("*"))
                return result, record, remaining, str(volume), str(token)
            finally:
                if volume.exists():
                    volume.chmod(0o700)

    def assert_private_staging(self, record, volume, token):
        self.assertIsNotNone(record, "the copied recipe did not call its installer")
        self.assertEqual(record["mode"], 0o700, "staging must be a private existing directory")
        self.assertEqual(Path(record["staging"]).parent, Path(volume))
        self.assertRegex(Path(record["staging"]).name, r"^pulse-agent-stage\.[A-Za-z0-9]+$")
        self.assertEqual(record["args"], ["--url", "https://pulse.example.com", "--token-file", token])
        self.assertNotIn(SECRET, json.dumps(record))

    def test_staging_is_private_on_selected_volume(self):
        result, record, remaining, volume, token = self.exercise()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_private_staging(record, volume, token)
        self.assertEqual(remaining, [], "empty newly owned staging directory should be removed")

    def test_missing_volume_stops_before_installer(self):
        result, record, remaining, _, _ = self.exercise(volume_state="missing")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Selected data volume is unavailable", result.stderr)
        self.assertIsNone(record)
        self.assertEqual(remaining, [])

    def test_unwritable_volume_stops_before_installer(self):
        result, record, remaining, _, _ = self.exercise(volume_state="unwritable")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not writable", result.stderr)
        self.assertIsNone(record)
        self.assertEqual(remaining, [])

    def test_staging_creation_failure_never_runs_installer(self):
        result, record, remaining, _, _ = self.exercise(mktemp_failure=True)
        self.assertEqual(result.returncode, 73)
        self.assertIn("synthetic staging creation failed", result.stderr)
        self.assertIsNone(record)
        self.assertEqual(remaining, [])

    def test_installer_failure_remains_nonzero_and_preserves_saved_state(self):
        result, record, remaining, volume, token = self.exercise(install_exit="29")
        self.assertEqual(result.returncode, 29)
        self.assert_private_staging(record, volume, token)
        self.assertEqual(remaining, [])

    def test_nonempty_owned_staging_survives_success_and_failure(self):
        for install_exit in ("0", "29"):
            with self.subTest(install_exit=install_exit):
                result, record, remaining, volume, token = self.exercise(
                    install_exit=install_exit, keep_staging=True
                )
                self.assertEqual(result.returncode, int(install_exit))
                self.assert_private_staging(record, volume, token)
                name = Path(record["staging"]).name
                self.assertEqual(remaining, [name, name + "/diagnostic-note"])

    def test_document_matches_current_installer_and_preserves_evidence(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/UNIFIED_AGENT.md").read_bytes())
        text = section()
        for phrase in ("moving staging cannot fix a full installation filesystem",
                       "When `TMPDIR` is unset", "An\nexisting `TMPDIR` overrides",
                       "same token file", "same collector options", "do not\nforce installation",
                       "Do not\ndelete or truncate a live log", "Keep log contents",
                       "does not establish that high CPU"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, text)
        self.assertNotIn("delete that\nfile", text)
        command = recipe(Path("/synthetic-volume"))
        self.assertNotRegex(command, r"\brm\b|--uninstall|--token(?:\s|=)|PULSE_TOKEN=|\bcurl\b|chmod|chown")
        source = (ROOT / "scripts/install.sh").read_text()
        for fragment in ('INSTALL_DIR="${QNAP_EARLY_VOL}/.pulse-agent"',
                         'if [[ -z "${TMPDIR:-}" ]]; then',
                         'QNAP_STAGING_TMPDIR="${QNAP_EARLY_VOL}/.pulse-agent/tmp"',
                         'AGENT_LOG_FILE="${QNAP_LOG_DIR}/${AGENT_NAME}.log"',
                         'AGENT_LOG_FILE="${UNRAID_LOG_DIR}/${AGENT_NAME}.log"',
                         'combined_required_bytes=$((AGENT_MIN_TEMP_FREE_BYTES + AGENT_MIN_INSTALL_FREE_BYTES))'):
            self.assertIn(fragment, source)


if __name__ == "__main__":
    unittest.main()
