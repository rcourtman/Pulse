#!/usr/bin/env python3
"""Check recovery guidance and execute its private hash recipe with a recorder.

The recorder proves shell disclosure/permission/failure behaviour, not Apache
hash generation or native Docker/systemd password recovery. The connected Go
config controls exercise bcrypt acceptance, source precedence and token retention.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDES = ("TROUBLESHOOTING.md", "FAQ.md", "DOCKER.md")
RECORD = "pulse-recovery:$2y$12$" + "a" * 53 + "\n"  # Synthetic disclosure sentinel.


def recovery() -> str:
    return (ROOT / "docs/TROUBLESHOOTING.md").read_text().split(
        "### I forgot my password\n", 1)[1].split("### Port change", 1)[0]


def recipe() -> str:
    matches = re.findall(r"```bash\n(.*?)```", recovery(), re.S)
    if len(matches) != 1:
        raise AssertionError("expected one private hash preparation recipe")
    return matches[0]


HTPASSWD = '''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
assert sys.argv[1:] == ['-nB', '-C', '12', 'pulse-recovery']
Path(os.environ['ARGV_RECEIPT']).write_text(json.dumps(sys.argv[1:]))
if os.environ.get('FAIL_HASH'):
    print('hash preparation failed', file=sys.stderr)
    sys.exit(23)
sys.stdout.write(os.environ['FIXTURE_RECORD'])
'''


class PasswordRecoveryDocsTest(unittest.TestCase):
    def test_entry_points_do_not_delete_auth_or_restart_setup(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                text = (ROOT / "docs" / name).read_text()
                self.assertNotRegex(text, r"(?:rm\s+/(?:data|etc/pulse)/\.env|Delete `/etc/pulse/\.env`)")
                if name != "TROUBLESHOOTING.md":
                    self.assertIn("TROUBLESHOOTING.md#i-forgot-my-password", text)
        text = recovery()
        self.assertIn("Replace only the password", text)
        self.assertIn("setup replaces the primary API token", text)
        self.assertIn("same\n   local administrator username", text)

    def test_precedence_apply_and_failure_guidance_matches_current_sources(self):
        text = " ".join(recovery().split())
        for expected in ("take precedence", "pulse.service.d/override.conf",
                         "pulse-backend.service.d", "PULSE_DATA_DIR",
                         "inside the Pulse container", "owner-only backup",
                         "complete 60-character hash", "literal `$`",
                         "not just `docker restart`", "same image, mounted data",
                         "reload the service manager", "If it still fails, stop",
                         "existing sessions and API tokens need separate review",
                         "If no local password has ever been set"):
            with self.subTest(expected=expected):
                self.assertIn(expected, text)
        setup = (ROOT / "internal/api/security_setup_fix.go").read_text()
        self.assertIn('/etc/systemd/system/%s.service.d/override.conf', setup)
        self.assertIn('r.config.APITokens = []config.APITokenRecord{*tokenRecord}', setup)
        loader = (ROOT / "internal/config/config.go").read_text()
        self.assertIn('godotenv.Load(envFile)', loader)
        self.assertIn('normalizeEnvAuthPassword(authPass)', loader)

    def run_recipe(self, directory: Path, *, fail: bool = False):
        tools = directory / "tools"
        tools.mkdir()
        tool = tools / "htpasswd"
        tool.write_text(HTPASSWD)
        tool.chmod(0o700)
        env = {**os.environ, "TMPDIR": str(directory),
               "PATH": f"{tools}:{os.environ['PATH']}",
               "ARGV_RECEIPT": str(directory / "argv.json"),
               "FIXTURE_RECORD": RECORD, "FAIL_HASH": "yes" if fail else ""}
        result = subprocess.run(["bash", "-c", recipe()], env=env,
                                capture_output=True, text=True, timeout=10)
        self.assertNotIn(RECORD.strip(), result.stdout + result.stderr)
        self.assertNotIn(RECORD.split(":", 1)[1].strip(), result.stdout + result.stderr)
        self.assertEqual(json.loads((directory / "argv.json").read_text()),
                         ['-nB', '-C', '12', 'pulse-recovery'])
        records = list(directory.glob("pulse-password.*/password-record"))
        self.assertEqual(len(records), 1)
        record = records[0]
        self.assertEqual(stat.S_IMODE(record.parent.stat().st_mode), 0o700)
        self.assertEqual(stat.S_IMODE(record.stat().st_mode), 0o600)
        return result, record

    def test_copied_recipe_keeps_hash_out_of_output_and_arguments(self):
        # Include spaces in the directory to exercise the copied quoting.
        with tempfile.TemporaryDirectory(prefix="password docs ") as temporary:
            result, record = self.run_recipe(Path(temporary))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(record.read_text(), RECORD)
            self.assertEqual(result.stdout,
                             f"Private password record saved in {record}\n")

    def test_hash_failure_stops_before_success_and_retains_private_partial_file(self):
        with tempfile.TemporaryDirectory() as temporary:
            result, record = self.run_recipe(Path(temporary), fail=True)
            self.assertEqual(result.returncode, 23)
            self.assertEqual(result.stdout, "")
            self.assertEqual(record.read_text(), "")
            self.assertIn("hash preparation failed", result.stderr)

    def test_recovery_guides_match_shipped_copies(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                self.assertEqual((ROOT / "docs" / name).read_bytes(),
                                 (ROOT / "frontend-modern/public/docs" / name).read_bytes())
        subprocess.run(["bash", "-n", "-c", recipe()], check=True, capture_output=True)


if __name__ == "__main__":
    unittest.main()
