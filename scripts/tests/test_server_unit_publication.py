#!/usr/bin/env python3
"""Real server-unit rendering/publication, confined to ordinary-user fixtures.

Only the fixed unit directory is relocated. No system manager or host path is
mutated; passing these controls is not installed-service acceptance.
"""

import io
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tarfile
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
            functions = []
            for name in ("preflight_systemd_service_installation", "install_systemd_service"):
                match = re.search(r"(?ms)^" + name + r"\(\) \{\n.*?^\}", INSTALLER.read_text())
                self.assertIsNotNone(match)
                functions.append(match[0].replace("/etc/systemd/system/", str(f / "units") + "/"))
            (f / "unit-function.sh").write_text("\n".join(functions))
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


CALLER_MANAGER = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (f / "manager-calls").open("a") as out:
    out.write(json.dumps(args) + "\n")
if args == ["list-unit-files", "--no-legend", "--no-pager", "--full", "--",
            "pulse-backend.service", "pulse.service"]:
    for name in ("pulse-backend", "pulse"):
        unit = f / "units" / (name + ".service")
        if unit.exists() or unit.is_symlink():
            print(name + ".service linked enabled")
    sys.exit(0)
service = os.environ["EXPECTED_SERVICE"]
if args == ["show", service, "--property=ActiveState", "--value"]:
    print((f / "active").read_text().strip())
elif args == ["is-active", "--quiet", service]:
    sys.exit(0 if (f / "active").read_text() == "active\n" else 3)
elif args == ["stop", service]:
    (f / "active").write_text("inactive\n")
elif args == ["start", service]:
    (f / "active").write_text("active\n")
elif args not in (["enable", service], ["daemon-reload"]):
    sys.exit(97)
'''


CALLER = r'''
source "$INSTALLER"
source "$FIXTURE/relocated-units.sh"
IN_CONTAINER=true
IN_DOCKER=false
ENABLE_AUTO_UPDATES=false
FORCE_VERSION=""
FORCE_CHANNEL=stable
BUILD_FROM_SOURCE=false
[[ "$FLOW" != pinned ]] || FORCE_VERSION=v6.6.0
[[ "$FLOW" != source && "$FLOW" != fresh-source ]] || BUILD_FROM_SOURCE=true
ARCHIVE_OVERRIDE="$FIXTURE/pulse-$PAYLOAD_VERSION-linux-amd64.tar.gz"
record() { printf '%s\n' "$*" >> "$FIXTURE/events"; }
print_header() { :; }
print_info() { printf '%s\n' "$*"; }
print_warn() { printf 'WARN: %s\n' "$*" >&2; }
print_error() { printf 'ERROR: %s\n' "$*" >&2; }
print_success() { printf '%s\n' "$*"; }
check_root() { :; }
check_proxmox_host() { return 1; }
detect_os() { :; }
check_docker_environment() { :; }
if [[ "$FLOW" == fresh* ]]; then
    # Exercise the fresh-entry branch even with a retained linked definition.
    # The independent detection/manager controls live in service discovery.
    check_existing_installation() { return 1; }
fi
resolve_latest_release_tag_for_channel() { printf 'v6.6.0\n'; }
safe_read() { local selection=1; [[ "$FLOW" != reinstall ]] || selection=2; printf -v "$2" '%s' "$selection"; }
offer_existing_auto_updates() { :; }
run_upgrade_readiness_preflight() { :; }
backup_existing() { record backup; }
create_user() { record user; }
install_dependencies() { record packages; }
setup_update_command() { record helper; printf 'changed helper\n' > "$FIXTURE/helper"; }
update_timer_exists() { return 1; }
setup_auto_updates() { record unexpected-timer; return 97; }
refresh_auto_updates() { record unexpected-refresh; return 97; }
create_marker_file() { record marker; }
print_completion() { record complete; }
# All transport, ownership and compiler inputs are inert. Keep actual main,
# directory setup, archive/source replacement, unit publication and start/
# stop/readback logic: a late refusal really leaves the confined service down.
verify_release_signature() { :; } # synthetic archive, not release trust proof
detect_pulse_architecture() { printf 'amd64\n'; }
validate_pulse_binary_architecture() { :; }
ensure_update_disk_headroom() { :; }
install_additional_agent_binaries() { record agents; }
restore_selinux_contexts() { :; }
chown() { :; }
apt-get() { record build-dependencies; }
go() { printf 'go version go1.99.0 linux/amd64\n'; }
npm() { record frontend-build; }
make() { record backend-build; cp "$FIXTURE/new-pulse" pulse; }
git() {
    if [[ "$1" == clone ]]; then
        record clone
        mkdir -p "${@: -1}/frontend-modern"
    else
        printf 'abc1234\n'
    fi
}
mktemp() {
    if [[ "$*" == '-d /tmp/pulse-build-XXXXXX' ]]; then
        command mktemp -d "$FIXTURE/pulse-build-XXXXXX"
    else
        command mktemp "$@"
    fi
}
sleep() { :; }
# Conditional callers disable errexit throughout; explicit failure admission
# must still stop before any service or saved-file mutation.
status=0
main || status=$?
exit "$status"
'''


class ServerUnitCallerAdmission(unittest.TestCase):
    def exercise(self, flow, *, service="pulse", explicit=False, destination="symlink",
                 decoy=False):
        with tempfile.TemporaryDirectory(prefix="pulse-unit-caller-") as directory:
            f = Path(directory)
            for name in ("tools", "units", "install/bin", "config"):
                (f / name).mkdir(parents=True)
            old = b"#!/bin/sh\n# old binary\necho 'Pulse v6.5.0'\n"
            version = "v6.5.0" if flow == "reinstall" else "v6.6.0"
            new = ("#!/bin/sh\n# new binary\necho 'Pulse " + version + "'\n").encode()
            saved = {"install/bin/pulse": old, "install/VERSION": b"v6.5.0\n",
                     "install/BUILD_FROM_SOURCE": b"saved-source-choice\n",
                     "config/.env": b"PULSE_MOCK_MODE=false\n# saved environment\n",
                     "config/system.json": b'{"autoUpdateEnabled":false}\n',
                     "config/.encryption.key": b"synthetic retained key\n",
                     "config/agent-id": b"retained synthetic identity\n",
                     "helper": b"previous helper\n", "public-link": b"previous link destination\n"}
            for name, contents in saved.items():
                path = f / name
                path.write_bytes(contents)
                path.chmod(0o755 if name == "install/bin/pulse" else 0o600)
            (f / "new-pulse").write_bytes(new)
            (f / "active").write_text("active\n")
            (f / "tools/systemctl").write_text(CALLER_MANAGER)
            (f / "tools/systemctl").chmod(0o755)
            target = f / "owned-unit"
            target.write_text("previous externally managed service\n")
            unit = f / "units" / (service + ".service")
            if destination == "symlink":
                unit.symlink_to(target)
            elif destination == "dangling":
                unit.symlink_to(f / "missing-unit")
            elif destination == "mask":
                unit.symlink_to("/dev/null")
            elif destination == "directory":
                unit.mkdir()
            elif destination == "fifo":
                os.mkfifo(unit)
            elif destination == "regular":
                unit.write_text(target.read_text())
            if decoy:
                (f / "units/pulse-backend.service").symlink_to(target)
            archive = f / ("pulse-" + version + "-linux-amd64.tar.gz")
            with tarfile.open(archive, "w:gz") as stream:
                for name, data, mode in (("pulse", new, 0o755),
                                         ("VERSION", (version + "\n").encode(), 0o644)):
                    info = tarfile.TarInfo(name)
                    info.size, info.mode = len(data), mode
                    stream.addfile(info, io.BytesIO(data))
            Path(str(archive) + ".sshsig").write_text("synthetic signature input\n")
            # Definitions only; never execute an installer's CLI/host entry.
            functions = []
            source = INSTALLER.read_text()
            for name in ("preflight_systemd_service_installation", "install_systemd_service",
                         "ensure_systemd_service_installed", "start_pulse"):
                match = re.search(r"(?ms)^" + name + r"\(\) \{\n.*?^\}", source)
                # Earlier exact-source adverse witnesses predate the helper.
                if match is not None:
                    functions.append(match[0].replace("/etc/systemd/system/", str(f / "units") + "/"))
            (f / "relocated-units.sh").write_text("\n".join(functions))

            def state():
                result = {}
                for root in (f / "install", f / "config", f / "units"):
                    for path in (root, *root.rglob("*")):
                        mode = stat.S_IMODE(path.lstat().st_mode)
                        value = ("link", os.readlink(path)) if path.is_symlink() else (
                            ("file", path.read_bytes()) if path.is_file() else ("special", None))
                        result[str(path.relative_to(f))] = (mode, value)
                for name in ("helper", "public-link", "owned-unit"):
                    result[name] = (stat.S_IMODE((f / name).stat().st_mode), (f / name).read_bytes())
                return result

            before = state()
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(FIXTURE=str(f), INSTALLER=str(INSTALLER), FLOW=flow, PAYLOAD_VERSION=version,
                       EXPECTED_SERVICE=service, PATH=str(f / "tools") + os.pathsep + os.environ["PATH"],
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"),
                       PULSE_BINARY_LINK_PATH=str(f / "public-link"))
            if explicit:
                env["PULSE_SERVICE_NAME"] = service
            result = subprocess.run(["bash", "-c", CALLER], env=env, cwd=f,
                                    capture_output=True, text=True, timeout=10)

            def lines(name):
                return (f / name).read_text().splitlines() if (f / name).exists() else []

            return dict(result=result, flow=flow, before=before, after=state(), active=(f / "active").read_text(),
                        events=lines("events"), calls=[json.loads(line) for line in lines("manager-calls")],
                        new=new, binary=(f / "install/bin/pulse").read_bytes())

    def assert_refused_before_mutation(self, e):
        self.assertEqual(e["result"].returncode, 1, e["result"].stdout + e["result"].stderr)
        self.assertIn("redirected or non-regular", e["result"].stderr)
        self.assertEqual(e["active"], "active\n", "predictable refusal stopped monitoring")
        self.assertEqual(e["before"], e["after"], "predictable refusal changed installed state")
        self.assertEqual(e["events"], [], "refusal reached build/install/helper/completion steps")
        if not e["flow"].startswith("fresh"):
            self.assertTrue(e["calls"], "real main must first read the initially running service")
        self.assertTrue(all(call[0] in ("show", "list-unit-files") for call in e["calls"]), e["calls"])

    def test_reinstall_and_source_rewrite_refusals_preserve_running_install(self):
        for flow in ("reinstall", "source", "fresh", "fresh-source"):
            for service, explicit in (("pulse", False), ("pulse-backend", False),
                                      ("pulse-site-b", True)):
                with self.subTest(flow=flow, service=service):
                    self.assert_refused_before_mutation(self.exercise(flow, service=service, explicit=explicit))

    def test_masks_and_nonregular_destinations_refuse_all_rewriting_main_paths(self):
        for flow in ("reinstall", "source", "fresh", "fresh-source", "pinned", "update"):
            for destination in ("dangling", "mask", "directory", "fifo"):
                with self.subTest(flow=flow, destination=destination):
                    self.assert_refused_before_mutation(self.exercise(flow, destination=destination))

    def test_ensure_only_updates_keep_sound_linked_units_and_restore_activity(self):
        for flow in ("pinned", "update"):
            for service, explicit in (("pulse", False), ("pulse-backend", False), ("pulse-site-b", True)):
                with self.subTest(flow=flow, service=service):
                    e = self.exercise(flow, service=service, explicit=explicit)
                    self.assertEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                    self.assertEqual(e["active"], "active\n")
                    self.assertEqual(e["binary"], e["new"])
                    self.assertIn(["stop", service], e["calls"])
                    self.assertIn(["start", service], e["calls"])
                    self.assertNotIn(["daemon-reload"], e["calls"])
                    self.assertEqual(e["events"][-2:], ["marker", "complete"])
                    for name in e["before"]:
                        if name.startswith("config/") or name.startswith("units/") or name == "owned-unit":
                            self.assertEqual(e["before"][name], e["after"][name], name)

    def test_explicit_names_do_not_admit_or_rewrite_the_decoy_backend(self):
        for flow in ("reinstall", "source"):
            for service in ("pulse", "pulse-site-b"):
                with self.subTest(flow=flow, service=service):
                    e = self.exercise(flow, service=service, explicit=True, destination="regular", decoy=True)
                    self.assertEqual(e["result"].returncode, 0, e["result"].stdout + e["result"].stderr)
                    self.assertEqual(e["active"], "active\n")
                    self.assertEqual(e["binary"], e["new"])
                    self.assertIn(["stop", service], e["calls"])
                    self.assertIn(["start", service], e["calls"])
                    self.assertIn(["daemon-reload"], e["calls"])
                    self.assertEqual(e["before"]["units/pulse-backend.service"],
                                     e["after"]["units/pulse-backend.service"])
                    self.assertEqual(e["before"]["owned-unit"], e["after"]["owned-unit"])


if __name__ == "__main__":
    unittest.main()
