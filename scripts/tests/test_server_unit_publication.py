#!/usr/bin/env python3
"""Real server-unit rendering/publication, confined to ordinary-user fixtures.

Only the fixed unit directory is relocated. No system manager or host path is
mutated; passing these controls is not installed-service acceptance.
"""

import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()

HARNESS = r'''
set -euo pipefail
SERVICE_NAME="$SELECTED_SERVICE"
SERVICE_NAME_EXPLICIT="$EXPLICIT"
INSTALL_DIR="$FIXTURE/install folder"
CONFIG_DIR="$FIXTURE/saved config"
FRONTEND_PORT="$PORT"
unit_cat_count=0
record() { printf '%s\n' "$*" >> "$FIXTURE/calls"; }
print_info() { printf '%s\n' "$*"; }
print_warn() { printf 'WARN: %s\n' "$*"; }
print_error() { printf 'ERROR: %s\n' "$*"; }
detect_service_name() { printf '%s\n' "$DETECTED_SERVICE"; }
safe_systemctl() { record "$*"; }
mktemp() {
    record allocate
    [[ "$FAULT" != allocation ]] || { echo INJECTED_ALLOCATION >&2; return 71; }
    command mktemp "$@"
}
cat() {
    unit_cat_count=$((unit_cat_count + 1))
    record "write $unit_cat_count"
    if [[ "$FAULT" == "write-$unit_cat_count" || ( "$FAULT" == cleanup && "$unit_cat_count" == 1 ) ]]; then
        printf 'partial unit\n'
        echo INJECTED_WRITE >&2
        return 72
    fi
    if [[ "$FAULT" == redirected-during-render && "$unit_cat_count" == 1 ]]; then
        ln -s "$FIXTURE/redirect-target" "$FIXTURE/units/$SERVICE_NAME.service"
    fi
    command cat "$@"
}
chmod() {
    record mode
    [[ "$FAULT" != mode ]] || { echo INJECTED_MODE >&2; return 73; }
    command chmod "$@"
}
mv() {
    record publish
    [[ "$FAULT" != rename ]] || { echo INJECTED_RENAME >&2; return 74; }
    [[ "$#" == 4 && "$1" == -fT && "$2" == -- && "$3" == "$4".staged.* ]] || return 95
    # The destination is still old/absent, and the staged bytes are complete,
    # with the intended mode, at the only publication boundary.
    if [[ "$EXISTING" == true ]]; then
        cmp "$FIXTURE/original" "$4" || return 96
    else
        [[ ! -e "$4" && ! -L "$4" ]] || return 96
    fi
    cmp "$FIXTURE/expected" "$3" || return 97
    [[ "$(stat -c %a "$3")" == 644 ]] || return 98
    command mv "$@"
}
rm() {
    record cleanup
    [[ "$FAULT" != cleanup ]] || { echo INJECTED_CLEANUP >&2; return 75; }
    command rm "$@"
}
source "$FIXTURE/unit-function.sh"
status=0
# Match main's OR-list context: failures must not rely on implicit errexit.
install_systemd_service || status=$?
exit "$status"
'''


def expected_unit(install, config, port):
    header = f'''[Unit]
Description=Pulse Monitoring Server
After=network.target

[Service]
Type=simple
User=pulse
Group=pulse
WorkingDirectory={install}
ExecStart={install}/bin/pulse
Restart=always
RestartSec=3
StandardOutput=journal
StandardError=journal
SyslogLevelPrefix=true
Environment="HOME={install}"
Environment="PATH={install}/.local/bin:{install}/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
Environment="PULSE_DATA_DIR={config}"
Environment="PULSE_DEPLOYMENT_METHOD=systemd"
# Pulse prefixes each JSON line with its zerolog priority. systemd consumes
# the prefix into PRIORITY and retains the original JSON as MESSAGE.
Environment="PULSE_LOG_JOURNAL_LEVEL_PREFIX=true"
EnvironmentFile=-{config}/.env
'''
    if port != "7655":
        header += f'Environment="FRONTEND_PORT={port}"\n'
    return header + f'''
# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths={install} {config}
# ICMP availability probes exec the system ping binary; NoNewPrivileges strips
# its setuid/file capabilities, so grant CAP_NET_RAW ambiently instead (#1554).
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW

[Install]
WantedBy=multi-user.target
'''


class ServerUnitPublication(unittest.TestCase):
    def exercise(self, *, existing=False, port="7655", fault="none", destination="file",
                 selected="pulse", explicit=True, detected="pulse"):
        with tempfile.TemporaryDirectory(prefix="pulse-unit-publication-") as directory:
            f = Path(directory)
            (f / "units").mkdir()
            # The legacy detection branch checks that its selected unit exists.
            service = "pulse-backend" if not explicit and detected == "pulse-backend" else selected
            unit = f / "units" / (service + ".service")
            original = "previous complete service definition\n"
            (f / "original").write_text(original)
            target = f / "redirect-target"
            target.write_text("redirected unit must survive\n")
            if destination == "mask":
                unit.symlink_to("/dev/null")
            elif destination == "symlink":
                unit.symlink_to(target)
            elif destination == "dangling":
                unit.symlink_to(f / "not-created")
            elif destination == "directory":
                unit.mkdir()
            elif destination == "fifo":
                os.mkfifo(unit)
            elif existing:
                unit.write_text(original)
                unit.chmod(0o640)
            expected = expected_unit(f / "install folder", f / "saved config", port)
            (f / "expected").write_text(expected)
            match = re.search(r"(?ms)^install_systemd_service\(\) \{\n.*?^\}", INSTALLER.read_text())
            self.assertIsNotNone(match)
            (f / "unit-function.sh").write_text(match[0].replace("/etc/systemd/system/", str(f / "units") + "/"))
            env = dict(os.environ, FIXTURE=str(f), FAULT=fault, PORT=port,
                       SELECTED_SERVICE=selected, EXPLICIT=str(explicit).lower(),
                       DETECTED_SERVICE=detected, EXISTING=str(existing).lower())
            result = subprocess.run(["bash", "-c", HARNESS], env=env, cwd=f,
                                    capture_output=True, text=True, timeout=10)
            stages = list((f / "units").glob("*.staged.*"))
            return dict(result=result, output=result.stdout + result.stderr,
                        calls=(f / "calls").read_text().splitlines() if (f / "calls").exists() else [],
                        contents=unit.read_text() if unit.is_file() and not unit.is_symlink() else None,
                        mode=stat.S_IMODE(unit.stat().st_mode) if unit.is_file() and not unit.is_symlink() else None,
                        link=os.readlink(unit) if unit.is_symlink() else None,
                        original=original, expected=expected, target=target.read_text(),
                        stages=[(p.read_text(), stat.S_IMODE(p.stat().st_mode)) for p in stages],
                        is_directory=unit.is_dir(), is_fifo=unit.exists() and stat.S_ISFIFO(unit.lstat().st_mode))

    def test_complete_publication_preserves_rendering_and_service_selection(self):
        for port in ("7655", "8765"):
            for selected, explicit, detected in (("pulse", True, "pulse"),
                                                  ("pulse-site-b", True, "pulse-backend"),
                                                  ("pulse", False, "pulse-backend")):
                with self.subTest(port=port, selected=selected, explicit=explicit):
                    e = self.exercise(existing=True, port=port, selected=selected,
                                      explicit=explicit, detected=detected)
                    self.assertEqual(e["result"].returncode, 0, e["output"])
                    self.assertEqual(e["contents"], e["expected"])
                    self.assertEqual(e["mode"], 0o644)
                    self.assertEqual(e["calls"].count("publish"), 1)
                    self.assertEqual(e["calls"][-1], "daemon-reload")
                    self.assertEqual(e["stages"], [])

    def test_failures_never_publish_partial_units_or_reload(self):
        for existing in (False, True):
            for port in ("7655", "8765"):
                faults = ["allocation", "write-1", "write-2", "mode", "rename", "cleanup"]
                if port != "7655":
                    faults.append("write-3")
                for fault in faults:
                    with self.subTest(existing=existing, port=port, fault=fault):
                        e = self.exercise(existing=existing, port=port, fault=fault)
                        self.assertEqual(e["result"].returncode, 1, e["output"])
                        self.assertIn("INJECTED_", e["output"])
                        self.assertIn("installation is incomplete", e["output"])
                        self.assertNotIn("daemon-reload", e["calls"])
                        self.assertEqual(e["contents"], e["original"] if existing else None)
                        self.assertEqual(e["mode"], 0o640 if existing else None)
                        if fault == "cleanup":
                            self.assertIn("Could not remove staged service definition", e["output"])
                            self.assertEqual(e["stages"], [("partial unit\n", 0o600)])
                        else:
                            self.assertEqual(e["stages"], [])

    def test_new_units_are_complete_before_becoming_visible(self):
        for port in ("7655", "8765"):
            with self.subTest(port=port):
                e = self.exercise(port=port)
                self.assertEqual(e["result"].returncode, 0, e["output"])
                self.assertEqual(e["contents"], e["expected"])
                self.assertEqual(e["mode"], 0o644)
                self.assertEqual(e["stages"], [])

    def test_redirects_masks_and_nonregular_destinations_are_not_replaced(self):
        for destination in ("mask", "symlink", "dangling", "directory", "fifo"):
            with self.subTest(destination=destination):
                e = self.exercise(destination=destination)
                self.assertEqual(e["result"].returncode, 1, e["output"])
                self.assertIn("redirected or non-regular", e["output"])
                self.assertEqual(e["calls"], [])
                self.assertEqual(e["target"], "redirected unit must survive\n")
                self.assertEqual(e["stages"], [])
                if destination == "mask":
                    self.assertEqual(e["link"], "/dev/null")
                elif destination in ("symlink", "dangling"):
                    self.assertIsNotNone(e["link"])
                elif destination == "directory":
                    self.assertTrue(e["is_directory"])
                else:
                    self.assertTrue(e["is_fifo"])

    def test_redirect_appearing_during_render_is_not_overwritten(self):
        e = self.exercise(fault="redirected-during-render")
        self.assertEqual(e["result"].returncode, 1, e["output"])
        self.assertIn("Could not publish", e["output"])
        self.assertNotIn("publish", e["calls"])
        self.assertNotIn("daemon-reload", e["calls"])
        self.assertIsNotNone(e["link"])
        self.assertEqual(e["target"], "redirected unit must survive\n")
        self.assertEqual(e["stages"], [])


if __name__ == "__main__":
    unittest.main()
