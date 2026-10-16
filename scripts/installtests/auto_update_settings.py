#!/usr/bin/env python3
"""Exercise the installer's settings publication using private real files.

Only service/ownership commands are closed doubles. jq, mktemp and rename use
their real implementations; no manager, network or installed configuration is
accessed. The old shared /tmp pathname is redirected into the fixture solely
to confine the exact-parent adverse control.
"""
from __future__ import annotations

import json
import os
from pathlib import Path
import shlex
import stat
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))


class AutoUpdateSettings(unittest.TestCase):
    def exercise(self, *, content='{"autoUpdateEnabled":false,"keep":{"text":"café"}}',
                 fault="", target="regular", channel="rc", conditional=False):
        source = SOURCE.read_text()
        function = "setup_auto_updates() {" + source.split("setup_auto_updates() {", 1)[1].split("\n}\n", 1)[0] + "\n}\n"
        function = function.replace('/tmp/system_$$.json', '${FIXTURE}/shared/system_$$.json')
        with tempfile.TemporaryDirectory(prefix="pulse-update-settings-") as temporary:
            root = Path(temporary)
            config = root / "config"; config.mkdir()
            shared = root / "shared"; shared.mkdir()
            unrelated = root / "unrelated"; unrelated.write_text("unrelated file must survive\n")
            saved = config / "system.json"
            if target == "symlink": saved.symlink_to(unrelated)
            elif target == "directory": saved.mkdir()
            elif target == "dangling": saved.symlink_to(root / "absent")
            elif content is not None: saved.write_text(content); saved.chmod(0o640)
            before = saved.read_bytes() if saved.is_file() else None
            setup = r'''
set -euo pipefail
umask 022
CONFIG_DIR="$FIXTURE/config" SERVICE_NAME=pulse-fixture
UPDATE_TIMER_PATH="$FIXTURE/pulse-fixture-update.timer" ENABLE_AUTO_UPDATES=true
print_info() { :; }
print_warn() { printf 'WARN: %s\n' "$*"; }
print_success() { printf 'SUCCESS: %s\n' "$*"; }
selected_update_channel() { printf '%s\n' "$CHANNEL"; }
install_auto_update_assets() { printf 'assets\n' >> "$FIXTURE/events"; [[ "$FAULT" != assets ]]; }
safe_systemctl() { printf 'manager:%s\n' "$*" >> "$FIXTURE/events"; }
chown() { printf 'owner\n' >> "$FIXTURE/events"; [[ "$FAULT" != owner ]]; }
command() {
    if [[ "$FAULT" == missing-jq && "$*" == '-v jq' ]]; then return 1; fi
    builtin command "$@"
}
mktemp() {
    [[ "$FAULT" != temporary ]] || return 1
    builtin command mktemp "$@"
}
chmod() { [[ "$FAULT" != mode ]] || return 1; builtin command chmod "$@"; }
jq() {
    "$FIXTURE_PYTHON" -c 'import os,pathlib,stat; r=pathlib.Path(os.environ["FIXTURE"]); (r/"stage-modes").write_text(" ".join(str(stat.S_IMODE(p.stat().st_mode)) for p in (r/"config").glob(".system.json.*")))'
    if [[ "$FAULT" == parse ]]; then printf 'partial output'; return 7; fi
    if [[ "$FAULT" == interrupted ]]; then kill -TERM "$BASHPID"; return 143; fi
    builtin command jq "$@"
}
mv() {
    if [[ "$FAULT" == rename ]]; then return 1; fi
    if [[ "$FAULT" == raced-directory ]]; then
        builtin command rm -- "$CONFIG_DIR/system.json"
        mkdir "$CONFIG_DIR/system.json"
    fi
    printf 'rename\n' >> "$FIXTURE/events"
    builtin command mv "$@"
}
# A pre-existing shared temporary link must never be followed by publication.
ln -s "$FIXTURE/unrelated" "$FIXTURE/shared/system_$$.json"
'''
            invoke = ("if ! setup_auto_updates; then exit 99; fi\n" if conditional
                      else "setup_auto_updates\n") + 'printf "FLAG:%s\\n" "$ENABLE_AUTO_UPDATES"\n'
            env = dict(os.environ, FIXTURE=str(root), FIXTURE_PYTHON=sys.executable,
                       FAULT=fault, CHANNEL=channel)
            result = subprocess.run(["bash", "-c", setup + function + invoke], cwd=root,
                                    env=env, capture_output=True, text=True, timeout=10)
            final = saved.read_bytes() if saved.is_file() else None
            modes = (root / "stage-modes").read_text().split() if (root / "stage-modes").exists() else []
            observation = dict(
                before=before, final=final, linked=saved.is_symlink(), directory=saved.is_dir(),
                mode=stat.S_IMODE(saved.stat().st_mode) if saved.is_file() else None,
                events=(root / "events").read_text().splitlines() if (root / "events").exists() else [],
                staged=list(config.glob(".system.json.*")), stage_modes=modes,
                unrelated=unrelated.read_text(), raced_children=list(saved.iterdir()) if saved.is_dir() else [],
            )
            return result, observation

    def assert_refused(self, result, observed):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("FLAG:false", result.stdout)
        self.assertNotIn("SUCCESS:", result.stdout)
        self.assertFalse(any(e.startswith("manager:") for e in observed["events"]))
        self.assertEqual(observed["final"], observed["before"])
        self.assertEqual(observed["staged"], [])
        self.assertEqual(observed["unrelated"], "unrelated file must survive\n")
        self.assertNotIn("café", result.stdout + result.stderr)

    def test_existing_object_preserves_values_and_uses_private_atomic_publication(self):
        for channel in ("stable", "rc"):
            with self.subTest(channel=channel):
                r, o = self.exercise(channel=channel)
                self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
                self.assertEqual(json.loads(o["final"]), dict(autoUpdateEnabled=True,
                                 updateChannel=channel, keep=dict(text="café")))
                self.assertEqual(o["mode"], 0o600)
                self.assertEqual(o["stage_modes"], [str(0o600)])
                self.assertEqual(o["staged"], [])
                self.assertEqual(o["unrelated"], "unrelated file must survive\n")
                self.assertIn("SUCCESS:", r.stdout)
                self.assertLess(o["events"].index("rename"), o["events"].index("manager:enable pulse-fixture-update.timer"))

    def test_fresh_settings_are_private_and_keep_the_existing_defaults(self):
        for fault in ("", "missing-jq"):
            with self.subTest(fault=fault):
                r, o = self.exercise(content=None, fault=fault)
                self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
                self.assertEqual(json.loads(o["final"]), dict(autoUpdateEnabled=True, updateChannel="rc", pollingInterval=5))
                self.assertEqual(o["mode"], 0o600)
                self.assertEqual(o["staged"], [])

    def test_missing_parser_preserves_existing_configuration_without_text_substitution(self):
        r, o = self.exercise(fault="missing-jq")
        self.assert_refused(r, o)

    def test_invalid_or_multiple_json_values_are_not_rewritten_or_enabled(self):
        for content in ('{"autoUpdateEnabled":false,', 'null', '[]', 'true',
                        '{}\n{}', '"café"', '{"autoUpdateEnabled":false}garbage'):
            with self.subTest(content=content):
                r, o = self.exercise(content=content)
                self.assert_refused(r, o)

    def test_linked_or_nonregular_configuration_is_not_followed(self):
        for target in ("symlink", "dangling", "directory"):
            with self.subTest(target=target):
                r, o = self.exercise(target=target)
                self.assert_refused(r, o)
                self.assertEqual(o["linked"], target != "directory")
                self.assertEqual(o["directory"], target == "directory")

    def test_failed_stage_parse_mode_owner_or_rename_retains_original_without_timer_changes(self):
        for fault in ("temporary", "parse", "mode", "owner", "rename"):
            for conditional in (False, True):
                with self.subTest(fault=fault, conditional=conditional):
                    r, o = self.exercise(fault=fault, conditional=conditional)
                    self.assert_refused(r, o)
        for fault in ("temporary", "mode", "owner", "rename"):
            with self.subTest(fresh=True, fault=fault):
                r, o = self.exercise(content=None, fault=fault)
                self.assert_refused(r, o)

    def test_interrupted_publication_cleans_its_private_staging_file(self):
        r, o = self.exercise(fault="interrupted")
        self.assert_refused(r, o)

    def test_raced_destination_directory_is_not_written_inside(self):
        r, o = self.exercise(fault="raced-directory")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("FLAG:false", r.stdout)
        self.assertNotIn("SUCCESS:", r.stdout)
        self.assertEqual(o["raced_children"], [])
        self.assertEqual(o["staged"], [])
        self.assertFalse(any(e.startswith("manager:") for e in o["events"]))
        self.assertEqual(o["unrelated"], "unrelated file must survive\n")

    def test_refused_assets_keep_saved_settings_and_never_enable_the_timer(self):
        r, o = self.exercise(fault="assets")
        self.assert_refused(r, o)


if __name__ == "__main__":
    unittest.main()
