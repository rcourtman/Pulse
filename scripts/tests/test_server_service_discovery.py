#!/usr/bin/env python3
"""Execute server service discovery and its callers with confined manager doubles.

Archive/source fixtures use fake versioned executables, not trusted releases or
real builds. Only the unit-writer fixture relocates its fixed /etc destination.
The optional real inventory uses systemctl --root, never the host unit manager.
"""

import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh")).resolve()
UPDATER = Path(os.environ.get("PULSE_AUTO_UPDATER_UNDER_TEST", ROOT / "scripts/pulse-auto-update.sh")).resolve()
REAL_SYSTEMCTL = shutil.which("systemctl")
QUERY = ["list-unit-files", "--no-legend", "--no-pager", "--full", "--",
         "pulse-backend.service", "pulse.service"]

SYSTEMCTL = r'''#!/usr/bin/env python3
import json, os, sys, time
from pathlib import Path
f = Path(os.environ["FIXTURE"])
args = sys.argv[1:]
with (f / "calls").open("a") as stream:
    stream.write(json.dumps(args) + "\n")
if args[0] == "list-unit-files":
    mode = os.environ["MODE"]
    if mode == "real":
        os.execv(os.environ["REAL_SYSTEMCTL"],
                 [os.environ["REAL_SYSTEMCTL"], "--root=" + str(f / "root")] + args)
    if mode == "hang":
        time.sleep(30)
    if mode == "large":
        # An early grep match followed by more than a pipe buffer makes the
        # parent's short read fail deterministically, not just emit a warning.
        os.write(1, b"pulse-backend.service enabled enabled\n")
        if "--" not in args:
            try:
                for i in range(512):
                    os.write(1, b"unrelated.service disabled enabled\n" * 128)
            except BrokenPipeError:
                (f / "broken-pipe").write_text("producer failed after early close\n")
                os.write(2, b"Failed to print table: Broken pipe\n")
                os._exit(1)
        sys.exit(0)
    os.write(1, os.environ["ROWS"].encode())
    sys.exit(int(os.environ["QUERY_EXIT"]))
if args[0] == "show":
    state = "inactive" if (f / "stopped").exists() else "active"
    if "--value" in args:
        print(state)
    else:
        print("LoadState=loaded\nActiveState=" + state)
    sys.exit(0)
if args[0] == "is-active":
    sys.exit(1 if (f / "stopped").exists() else 0)
if args[0] == "stop":
    (f / "stopped").write_text(args[1])
    sys.exit(0)
if args[0] == "daemon-reload":
    sys.exit(0)
sys.exit(97)
'''

HARNESS = r'''
source "$INSTALLER"
if [[ "$PRODUCER" == updater ]]; then source "$UPDATER"; fi
print_header() { :; }
print_info() { echo "$*" >&2; }
print_warn() { echo "$*" >&2; }
print_error() { echo "$*" >&2; }
print_success() { echo "$*" >&2; }
log() { echo "$*" >&2; }
check_root() { :; }
check_proxmox_host() { return 1; }
detect_os() { :; }
check_docker_environment() { :; }
IN_CONTAINER=true
IN_DOCKER=false
mark_mutation() { echo "$*" >> "$FIXTURE/mutations"; return 97; }
case "$FLOW" in
    unavailable)
        command() {
            [[ "$*" != '-v systemctl' ]] || return 1
            builtin command "$@"
        }
        ;;
    main|late-main)
        FORCE_VERSION=v6.6.0
        offer_existing_auto_updates() { :; }
        run_upgrade_readiness_preflight() { :; }
        backup_existing() { mark_mutation backup; }
        create_user() { mark_mutation user; }
        install_dependencies() { mark_mutation packages; }
        setup_directories() { mark_mutation directories; }
        download_pulse() { mark_mutation download; }
        install_systemd_service() { mark_mutation unit; }
        ensure_systemd_service_installed() { mark_mutation unit; }
        setup_update_command() { mark_mutation helper; }
        setup_auto_updates() { mark_mutation timer; }
        refresh_auto_updates() { mark_mutation refresh; }
        start_pulse() { mark_mutation start; }
        create_marker_file() { mark_mutation marker; }
        print_completion() { :; }
        if [[ "$FLOW" == late-main ]]; then
            check_existing_installation() { CURRENT_VERSION=v6.5.0; return 0; }
        fi
        ;;
    archive|build|automatic)
        verify_release_signature() { :; }
        validate_pulse_binary_architecture() { :; }
        detect_pulse_architecture() { echo amd64; }
        chown() { :; }
        install_additional_agent_binaries() { :; }
        deploy_agent_scripts() { :; }
        if [[ "$FLOW" == automatic ]]; then
            curl() {
                # Fixed fixture response, never a provider request.
                local target="${@: -1}"
                if [[ "$*" == *.sshsig* ]]; then
                    printf 'fixture signature\n' > "$target"
                else
                    cp "$FIXTURE/served-installer.sh" "$target"
                fi
            }
        fi
        if [[ "$FLOW" == build ]]; then
            apt-get() { :; }
            go() { echo 'go version go1.26.9 linux/amd64'; }
            npm() { :; }
            make() { cp "$FIXTURE/new-pulse" pulse; }
            git() {
                if [[ "$1" == clone ]]; then
                    mkdir -p "${@: -1}/frontend-modern"
                else
                    echo abc1234
                fi
            }
        fi
        ;;
    uninstall)
        quiesce_pulse_for_removal() { mark_mutation "remove $*"; }
        ;;
    reset)
        read_pulse_reset_unit_state() { mark_mutation "reset $*"; }
        ;;
    unit|ensure-unit)
        # Only fixed path relocation; the function's discovery, branch and
        # redirection remain production code. Never write the host's /etc.
        source "$FIXTURE/unit-writer.sh"
        ;;
esac
status=0
# Deliberately invoke inside an OR-list: callers must handle discovery errors
# explicitly, even when Bash disables errexit throughout the function.
case "$FLOW" in
    query|unavailable) selected=$(detect_service_name) || status=$?
        [[ "$status" -ne 0 ]] || printf 'SERVICE=%s\n' "$selected" ;;
    existing) check_existing_installation || status=$?
        [[ "$status" -ne 0 ]] || printf 'SERVICE=%s\n' "$SERVICE_NAME" ;;
    main|late-main) main || status=$? ;;
    archive) install_pulse_archive "$FIXTURE/pulse-v6.6.0-linux-amd64.tar.gz" v6.6.0 || status=$? ;;
    build) build_from_source main || status=$? ;;
    automatic) perform_update v6.6.0 || status=$? ;;
    uninstall) uninstall_pulse || status=$? ;;
    reset) reset_pulse || status=$? ;;
    unit) install_systemd_service || status=$? ;;
    ensure-unit) ensure_systemd_service_installed || status=$? ;;
    *) exit 98 ;;
esac
exit "$status"
'''


class ServiceDiscovery(unittest.TestCase):
    def run_fixture(self, *, rows="", mode="rows", query_exit=0, flow="query", explicit=None,
                    producer="installer"):
        with tempfile.TemporaryDirectory(prefix="pulse-service-discovery-") as directory:
            f = Path(directory)
            for name in ["bin", "install/bin", "config", "units", "root/etc/systemd/system"]:
                (f / name).mkdir(parents=True)
            (f / "bin/systemctl").write_text(SYSTEMCTL)
            (f / "bin/systemctl").chmod(0o755)
            old = b"#!/bin/sh\necho 'Pulse v6.5.0'\n"
            new = b"#!/bin/sh\necho 'Pulse v6.6.0'\n"
            (f / "install/bin/pulse").write_bytes(old)
            (f / "install/bin/pulse").chmod(0o755)
            (f / "new-pulse").write_bytes(new)
            (f / "new-pulse").chmod(0o755)
            (f / "served-installer.sh").write_text(
                '#!/bin/bash\nset -eu\n'
                'printf "%s\\n" "$PULSE_SERVICE_NAME" > "$FIXTURE/installer-service"\n'
                'cp "$FIXTURE/new-pulse" "$PULSE_INSTALL_DIR/bin/pulse"\n')
            archive = f / "pulse-v6.6.0-linux-amd64.tar.gz"
            with tarfile.open(archive, "w:gz") as stream:
                entry = tarfile.TarInfo("pulse")
                entry.size, entry.mode = len(new), 0o755
                stream.addfile(entry, io.BytesIO(new))
            Path(str(archive) + ".sshsig").write_text("signature verification is a double\n")
            source = INSTALLER.read_text()
            start = source.index("install_systemd_service() {")
            end = source.index("\n}\n", start) + 3
            body = source[start:end].replace("/etc/systemd/system/", str(f / "units") + "/")
            (f / "unit-writer.sh").write_text(body)
            unit = "[Unit]\nDescription=Fixture only\n[Service]\nExecStart=/bin/true\n"
            (f / "root/etc/systemd/system/pulse-backend.service").write_text(unit)
            (f / "root/etc/systemd/system/pulse.service").write_text(unit)
            env = {k: v for k, v in os.environ.items() if not k.startswith("PULSE_")}
            env.update(FIXTURE=str(f), INSTALLER=str(INSTALLER), UPDATER=str(UPDATER),
                       PRODUCER=producer, FLOW=flow, MODE=mode,
                       ROWS=rows, QUERY_EXIT=str(query_exit), REAL_SYSTEMCTL=REAL_SYSTEMCTL or "",
                       PATH=str(f / "bin") + os.pathsep + os.environ["PATH"],
                       PULSE_INSTALL_DIR=str(f / "install"), PULSE_CONFIG_DIR=str(f / "config"),
                       PULSE_BINARY_LINK_PATH=str(f / "pulse-link"))
            if explicit is not None:
                env["PULSE_SERVICE_NAME"] = explicit
            start_time = time.monotonic()
            result = subprocess.run(["bash", "-c", HARNESS], env=env, cwd=f,
                                    text=True, capture_output=True, timeout=15)
            return dict(result=result, seconds=time.monotonic() - start_time,
                        calls=[json.loads(line) for line in (f / "calls").read_text().splitlines()]
                        if (f / "calls").exists() else [],
                        mutations=(f / "mutations").read_text() if (f / "mutations").exists() else "",
                        binary=(f / "install/bin/pulse").read_bytes(), old=old, new=new,
                        units=[p.name for p in (f / "units").iterdir()],
                        installer_service=(f / "installer-service").read_text()
                        if (f / "installer-service").exists() else None,
                        broken_pipe=(f / "broken-pipe").exists())

    def assert_refused(self, e):
        self.assertNotEqual(e["result"].returncode, 0, e["result"])
        self.assertIn("Cannot identify the Pulse systemd service", e["result"].stderr)
        self.assertNotIn("SERVICE=", e["result"].stdout)
        self.assertEqual(e["binary"], e["old"])
        self.assertEqual(e["mutations"], "")
        self.assertEqual(e["units"], [])
        self.assertTrue(all(call == QUERY for call in e["calls"]), e["calls"])

    def test_large_inventory_cannot_short_read_and_hide_legacy_service(self):
        e = self.run_fixture(mode="large")
        self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
        self.assertEqual(e["result"].stdout, "SERVICE=pulse-backend\n")
        self.assertEqual(e["calls"], [QUERY])
        self.assertFalse(e["broken_pipe"])

    def test_exact_names_and_legacy_precedence_with_state_columns(self):
        for rows, expected in [("", "pulse"), ("\n", "pulse"),
                               ("pulse.service disabled enabled\n", "pulse"),
                               ("pulse-backend.service disabled\n", "pulse-backend"),
                               ("pulse.service static -\npulse-backend.service masked-runtime enabled\n", "pulse-backend"),
                               ("pulse-backend.service enabled enabled\npulse.service alias -\n", "pulse-backend")]:
            with self.subTest(rows=rows):
                e = self.run_fixture(rows=rows)
                self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
                self.assertEqual(e["result"].stdout, "SERVICE=" + expected + "\n")
                self.assertEqual(e["calls"], [QUERY])

    def test_explicit_instances_skip_default_inventory(self):
        for service in ["pulse", "pulse-backend", "pulse.department" + "x" * 140]:
            with self.subTest(service=service):
                e = self.run_fixture(explicit=service, query_exit=1)
                self.assertEqual(e["result"].returncode, 0)
                self.assertEqual(e["result"].stdout, "SERVICE=" + service + "\n")
                self.assertEqual(e["calls"], [])

    def test_non_systemd_install_keeps_default(self):
        e = self.run_fixture(flow="unavailable")
        self.assertEqual(e["result"].returncode, 0)
        self.assertEqual(e["result"].stdout, "SERVICE=pulse\n")
        self.assertEqual(e["calls"], [])

    def test_partial_failed_inventory_is_not_absence(self):
        for rows in ["", "pulse-backend.service enabled enabled\n", "pulse.service disabled enabled\n"]:
            with self.subTest(rows=rows):
                self.assert_refused(self.run_fixture(rows=rows, query_exit=1))

    def test_malformed_duplicate_impostor_and_unknown_inventory_stop(self):
        for rows in ["pulse-backendXservice enabled\n", "pulse-backend.service.extra enabled\n",
                     "pulseXservice enabled\n", "unrelated.service enabled\n",
                     "pulse-backend.service\n", "pulse.service bad enabled\n",
                     "pulse.service future-state enabled\n", "pulse.service enabled enabled extra\n",
                     "pulse.service enabled\npulse.service disabled\n",
                     "pulse-backend.service enabled\npulse-backend.service enabled\n"]:
            with self.subTest(rows=rows):
                self.assert_refused(self.run_fixture(rows=rows))

    def test_inventory_timeout_is_bounded_and_refuses(self):
        e = self.run_fixture(mode="hang")
        self.assert_refused(e)
        self.assertLess(e["seconds"], 7)

    def test_existing_install_retains_legacy_service(self):
        e = self.run_fixture(flow="existing", mode="large")
        self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
        self.assertEqual(e["result"].stdout, "SERVICE=pulse-backend\n")

    def test_failed_discovery_is_distinct_from_no_installation(self):
        e = self.run_fixture(flow="existing", query_exit=1)
        self.assert_refused(e)
        self.assertEqual(e["result"].returncode, 2)

    def test_all_mutating_callers_refuse_when_errexit_is_disabled(self):
        for flow in ["main", "late-main", "archive", "build", "uninstall", "reset", "unit", "ensure-unit"]:
            with self.subTest(flow=flow):
                self.assert_refused(self.run_fixture(flow=flow, query_exit=1))

    def test_archive_and_source_paths_stop_only_the_selected_legacy_unit(self):
        for flow in ["archive", "build"]:
            with self.subTest(flow=flow):
                e = self.run_fixture(flow=flow, mode="large")
                self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
                self.assertEqual(e["binary"], e["new"])
                self.assertIn(["stop", "pulse-backend"], e["calls"])
                self.assertNotIn(["stop", "pulse"], e["calls"])
                self.assertFalse(e["broken_pipe"])

    @unittest.skipUnless(REAL_SYSTEMCTL, "systemctl unavailable")
    def test_real_systemctl_fixture_inventory_has_compatible_rows(self):
        e = self.run_fixture(mode="real")
        self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
        self.assertEqual(e["result"].stdout, "SERVICE=pulse-backend\n")
        self.assertEqual(e["calls"], [QUERY])

    def test_automatic_helper_preserves_exact_names_and_explicit_instances(self):
        for rows, explicit, expected in [("", None, "pulse"),
                                         ("pulse.service disabled enabled\n", None, "pulse"),
                                         ("pulse-backend.service masked-runtime enabled\n", None, "pulse-backend"),
                                         ("", "pulse.department" + "x" * 140, "pulse.department" + "x" * 140)]:
            with self.subTest(rows=rows, explicit=explicit):
                e = self.run_fixture(producer="updater", rows=rows, explicit=explicit)
                self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
                self.assertEqual(e["result"].stdout, "SERVICE=" + expected + "\n")
                self.assertEqual(e["calls"], [] if explicit else [QUERY])

    def test_automatic_helper_cannot_short_read_the_installer_handoff(self):
        e = self.run_fixture(producer="updater", mode="large", flow="automatic")
        self.assertEqual(e["result"].returncode, 0, e["result"].stderr)
        self.assertEqual(e["installer_service"], "pulse-backend\n")
        self.assertEqual(e["binary"], e["new"])
        self.assertEqual(e["calls"][0], QUERY)
        self.assertFalse(e["broken_pipe"])

    def test_automatic_helper_refuses_before_backup_or_installer_even_in_or_list(self):
        for rows, status in [("", 1), ("pulse-backend.service enabled enabled\n", 1),
                             ("pulse-backend.service future-state\n", 0),
                             ("pulse-backend.service enabled\npulse-backend.service enabled\n", 0),
                             ("pulse-backendXservice enabled\n", 0),
                             ("pulse.service enabled enabled extra\n", 0)]:
            with self.subTest(rows=rows, status=status):
                e = self.run_fixture(producer="updater", rows=rows, query_exit=status, flow="automatic")
                self.assert_refused(e)
                self.assertIsNone(e["installer_service"])

    def test_automatic_helper_timeout_stops_before_service_observation(self):
        e = self.run_fixture(producer="updater", mode="hang", flow="automatic")
        self.assert_refused(e)
        self.assertLess(e["seconds"], 7)
        self.assertIsNone(e["installer_service"])


if __name__ == "__main__":
    unittest.main()
