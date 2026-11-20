#!/usr/bin/env python3
"""Exercise the actual existing-installation menu without host mutations.

Release transport and all mutation boundaries are confined doubles. These are
ordinary-user selection/consent controls, not a native installation proof.
"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))
SIGNED_MAX = 2**63 - 1
SIGNED_OVERFLOW = SIGNED_MAX + 1
WORD_WRAP = 2**64


def release(tag, *, draft=False, prerelease=False):
    return {"tag_name": tag, "draft": draft, "prerelease": prerelease}


HARNESS = r'''
source "$INSTALLER"
FORCE_VERSION="$PIN"
FORCE_CHANNEL="$CHANNEL"
IN_CONTAINER=true
IN_DOCKER="$DOCKER"
print_header() { :; }
print_info() { printf '%s\n' "$*"; }
print_warn() { printf '%s\n' "$*"; }
print_error() { printf '%s\n' "$*" >&2; }
check_root() { :; }
detect_os() { :; }
check_docker_environment() { :; }
check_proxmox_host() { return 1; }
create_lxc_container() { exit 96; }
systemctl() {
    [[ "$*" == 'show pulse --property=ActiveState --value' ]] || return 97
    printf 'active\n'
}
timeout() {
    if [[ "$1" == -k ]]; then
        if [[ "$4" == "$INSTALL_DIR/bin/pulse" ]]; then
            # Exercise the real deadline on our disposable --version reader.
            command timeout "$@"
            return
        fi
        [[ "$4" == systemctl ]] || return 97
        shift 3
    else
        shift
    fi
    "$@"
}
curl() {
    local url="${@: -1}"
    printf '%s\n' "$url" >> "$FIXTURE/transport"
    case "$url" in
        https://api.github.com/repos/rcourtman/Pulse/releases)
            printf '%s' "$RELEASES_JSON"
            return "$API_EXIT" ;;
        https://github.com/rcourtman/Pulse/releases/latest)
            [[ -n "$REDIRECT_TAG" ]] || return 22
            printf 'https://github.com/rcourtman/Pulse/releases/tag/%s' "$REDIRECT_TAG" ;;
        *) printf 'unexpected transport: %s\n' "$url" >&2; exit 97 ;;
    esac
}
if [[ "$NO_JQ" == true ]]; then
    command() {
        [[ "$*" != '-v jq' ]] || return 1
        builtin command "$@"
    }
fi
safe_read() {
    [[ "$MENU_CHOICE" != eof ]] || return 1
    printf -v "$2" '%s' "$MENU_CHOICE"
}
# Every reachable mutation is stopped before the real implementation, including
# removal. A wrong automatic choice is evidence, never an actual uninstall.
offer_existing_auto_updates() { echo CONSENT_PROMPT >> "$FIXTURE/mutations"; }
detect_service_name() { echo pulse; }
run_upgrade_readiness_preflight() { echo PREFLIGHT >> "$FIXTURE/mutations"; }
preflight_systemd_service_installation() { echo UNIT_PREFLIGHT >> "$FIXTURE/mutations"; }
backup_existing() { echo BACKUP >> "$FIXTURE/mutations"; }
create_user() { echo USER >> "$FIXTURE/mutations"; }
install_dependencies() { echo PACKAGES >> "$FIXTURE/mutations"; exit 96; }
download_pulse() {
    printf 'DOWNLOAD=%s CHANNEL=%s\n' "${LATEST_RELEASE:-unpinned}" "${UPDATE_CHANNEL:-}" >> "$FIXTURE/mutations"
    exit 0
}
quiesce_pulse_for_removal() { echo REMOVE >> "$FIXTURE/mutations"; exit 77; }
main
'''


class ServerUpdateMenuTest(unittest.TestCase):
    def exercise(self, releases, installed="v6.5.0", channel="stable", *,
                 choice="eof", docker=False, configured="stable", api_exit=0,
                 redirect="", no_jq=False, pinned="", compact=False,
                 version_exit=0, version_delay=0):
        payload = (releases if isinstance(releases, str) else
                   json.dumps(releases, indent=None if compact else 2))
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            config = fixture / "config"
            config.mkdir()
            (config / "system.json").write_text(json.dumps({"updateChannel": configured}))
            install = fixture / "install"
            (install / "bin").mkdir(parents=True)
            if installed is not None:
                binary = install / "bin" / "pulse"
                binary.write_text(
                    '#!/bin/sh\n'
                    '[ "$#" = 1 ] && [ "$1" = --version ] || exit 97\n'
                    'printf "Pulse %s\\nBuilt: fixture\\n" "$INSTALLED"\n'
                    'if [ "$VERSION_DELAY" -ne 0 ]; then sleep "$VERSION_DELAY"; fi\n'
                    'exit "$VERSION_EXIT"\n'
                )
                binary.chmod(0o755)
            env = {**os.environ, "INSTALLER": str(INSTALLER), "FIXTURE": str(fixture),
                   "PULSE_CONFIG_DIR": str(config), "PULSE_INSTALL_DIR": str(install),
                   "PULSE_SERVICE_NAME": "pulse", "RELEASES_JSON": payload,
                   "INSTALLED": installed or "", "VERSION_EXIT": str(version_exit),
                   "VERSION_DELAY": str(version_delay), "CHANNEL": channel, "MENU_CHOICE": choice,
                   "DOCKER": str(docker).lower(), "API_EXIT": str(api_exit),
                   "REDIRECT_TAG": redirect, "NO_JQ": str(no_jq).lower(), "PIN": pinned}
            result = subprocess.run(["bash", "-c", HARNESS], env=env,
                                    capture_output=True, text=True, timeout=15)
            def lines(name):
                return (fixture / name).read_text().splitlines() if (fixture / name).exists() else []
            return result, lines("mutations"), lines("transport")

    def assert_unchanged(self, result, mutations, *, refused=False):
        self.assertEqual(result.returncode, 1 if refused else 0, result.stdout + result.stderr)
        self.assertEqual(mutations, [], result.stdout + result.stderr)

    def assert_download(self, result, mutations, version, channel):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(mutations[-1:], [f"DOWNLOAD={version} CHANNEL={channel}"])
        self.assertNotIn("REMOVE", mutations)

    def test_current_preview_without_stable_never_autoselects_removal(self):
        data = [release("v6.6.0-rc.1", prerelease=True)]
        for docker in (False, True):
            with self.subTest(docker=docker):
                r, mutations, _ = self.exercise(data, "v6.6.0-rc.1", "rc", docker=docker)
                self.assert_unchanged(r, mutations)

    def test_preview_only_row_is_selected_by_its_actual_index(self):
        data = [release("v6.6.0-rc.1", prerelease=True), release("v6.5.0")]
        for docker in (False, True):
            r, mutations, _ = self.exercise(data, channel="rc", docker=docker)
            self.assert_download(r, mutations, "v6.6.0-rc.1", "rc")

    def test_stable_at_current_version_does_not_switch_to_preview_or_reinstall(self):
        data = [release("v6.6.0-rc.1", prerelease=True), release("v6.5.0")]
        for docker in (False, True):
            r, mutations, _ = self.exercise(data, docker=docker)
            self.assert_unchanged(r, mutations)
        r, mutations, _ = self.exercise([release("v6.5.0")])
        self.assert_unchanged(r, mutations)

    def test_chart_and_draft_metadata_do_not_enter_the_menu(self):
        data = [release("helm-chart-6.7.0-rc.1", prerelease=True),
                release("v6.7.0-rc.1", draft=True, prerelease=True),
                release("helm-chart-6.6.0"), release("v6.6.0-rc.1", prerelease=True),
                release("v6.5.0")]
        for compact in (False, True):
            r, mutations, _ = self.exercise(data, channel="rc", compact=compact)
            self.assert_download(r, mutations, "v6.6.0-rc.1", "rc")
            self.assertNotIn("Update to helm-chart", r.stdout)
            self.assertNotIn("Update to v6.7.0", r.stdout)

    def test_new_stable_and_preview_keep_distinct_targets(self):
        data = [release("v6.7.0-rc.1", prerelease=True), release("v6.6.0")]
        for channel, target in (("stable", "v6.6.0"), ("rc", "v6.7.0-rc.1")):
            r, mutations, _ = self.exercise(data, channel=channel)
            self.assert_download(r, mutations, target, channel)

    def test_preview_channel_can_follow_stable_without_losing_channel_intent(self):
        data = [release("v6.6.0"), release("v6.6.0-rc.1", prerelease=True)]
        r, mutations, _ = self.exercise(data, installed="v6.6.0-rc.1", channel="rc")
        self.assert_download(r, mutations, "v6.6.0", "rc")

    def test_configured_channel_and_explicit_override_are_respected(self):
        data = [release("v6.6.0-rc.1", prerelease=True), release("v6.5.0")]
        r, mutations, _ = self.exercise(data, channel="", configured="rc")
        self.assert_download(r, mutations, "v6.6.0-rc.1", "rc")
        r, mutations, _ = self.exercise(data, channel="stable", configured="rc")
        self.assert_unchanged(r, mutations)

    def test_unavailable_or_malformed_metadata_never_falls_into_another_action(self):
        for payload in ([], "", "null", '[{"tag_name":"v6.6.0"',
                        [release("helm-chart-6.6.0")], [release("v6.6.0", draft=True)]):
            for channel in ("stable", "rc"):
                with self.subTest(payload=payload, channel=channel):
                    r, mutations, _ = self.exercise(payload, channel=channel)
                    self.assert_unchanged(r, mutations, refused=True)
                    self.assertIn("published Pulse server release", r.stderr)

    def test_failed_api_payload_and_missing_parser_do_not_admit_preview(self):
        data = [release("v6.6.0-rc.1", prerelease=True), release("v6.5.0")]
        for options in ({"api_exit": 22}, {"no_jq": True}):
            r, mutations, _ = self.exercise(data, channel="rc", **options)
            self.assert_unchanged(r, mutations, refused=True)
            r, mutations, _ = self.exercise(data, channel="rc", redirect="v6.5.0", **options)
            self.assert_unchanged(r, mutations)

    def test_automatic_downgrade_is_refused_but_deliberate_menu_choice_is_retained(self):
        data = [release("v6.5.0")]
        r, mutations, _ = self.exercise(data, "v6.6.0-rc.1")
        self.assert_unchanged(r, mutations, refused=True)
        self.assertIn("refusing to downgrade automatically", r.stderr)
        r, mutations, _ = self.exercise(data, "v6.6.0-rc.1", choice="1")
        self.assert_download(r, mutations, "v6.5.0", "stable")

    def test_unobserved_version_refuses_both_automatic_entry_paths(self):
        data = [release("v6.6.0"), release("v6.7.0-rc.1", prerelease=True)]
        for installed in ("dev", "v6.5.0-dev", "dev\nBuilt: v6.5.0"):
            for channel, docker in (("stable", False), ("rc", True)):
                with self.subTest(installed=installed, channel=channel, docker=docker):
                    r, mutations, _ = self.exercise(data, installed, channel, docker=docker)
                    self.assert_unchanged(r, mutations, refused=True)
                    self.assertIn("Cannot determine the installed Pulse version", r.stderr)
                    self.assertIn("Existing installation is unchanged", r.stderr)
                    self.assertIn("--version", r.stderr)

    def test_unknown_edition_stops_before_discovery_or_a_pinned_public_update(self):
        data = [release("v6.6.0")]
        for installed, version_exit in ((None, 0), ("", 0), ("unrecognised", 0), ("v6.5.0", 23)):
            for pinned in ("", "v6.6.0"):
                with self.subTest(installed=installed, version_exit=version_exit, pinned=pinned):
                    r, mutations, transport = self.exercise(
                        data, installed, choice="1", pinned=pinned, version_exit=version_exit)
                    self.assert_unchanged(r, mutations, refused=True)
                    self.assertEqual(transport, [])
                    self.assertIn("Cannot verify the installed Pulse edition", r.stderr)
                    self.assertIn("Existing installation is unchanged", r.stderr)

    def test_version_reader_deadline_does_not_admit_partial_stdout(self):
        # The reader prints a valid version, but cannot exit before the
        # installer's five-second limit. Even on the old code the delay is
        # finite and shorter than this harness's command-level fail-stop.
        r, mutations, transport = self.exercise([release("v6.6.0")], version_delay=8)
        self.assert_unchanged(r, mutations, refused=True)
        self.assertEqual(transport, [])
        self.assertIn("Cannot verify the installed Pulse edition", r.stderr)

    def test_verified_community_development_build_keeps_explicit_repair_routes(self):
        r, mutations, transport = self.exercise([], installed="dev", pinned="v6.6.0")
        self.assert_download(r, mutations, "v6.6.0", "")
        self.assertEqual(transport, [])
        r, mutations, _ = self.exercise([release("v6.6.0")], "dev", choice="1")
        self.assert_download(r, mutations, "v6.6.0", "stable")

    def test_noncanonical_installed_components_stop_automatic_selection(self):
        for installed, target, docker in (
            ("v06.9.0", "v6.8.1", False),
            ("v6.09.0", "v6.8.1", False),
            ("v6.9.01", "v6.9.2", True),
            ("v6.9.0-beta.09", "v6.9.0-beta.10", False),
            ("v6.9.0-rc.01", "v6.9.0-rc.2", True),
        ):
            with self.subTest(installed=installed, target=target, docker=docker):
                preview = "-" in target
                r, mutations, _ = self.exercise([release(target, prerelease=preview)], installed,
                                                "rc" if preview else "stable", docker=docker)
                self.assert_unchanged(r, mutations, refused=True)
                self.assertIn("Cannot determine the installed Pulse version", r.stderr)
                self.assertNotIn("value too great for base", r.stderr)

    def test_out_of_range_installed_components_stop_automatic_selection(self):
        for installed, target, docker in (
            (f"v{WORD_WRAP}.9.0", "v6.8.1", True),
            (f"v6.{WORD_WRAP}.0", "v6.8.1", False),
            (f"v6.9.{SIGNED_OVERFLOW}", "v7.0.0", True),
            (f"v6.9.0-beta.{SIGNED_OVERFLOW}", "v6.9.0-beta.1", True),
            (f"v6.9.0-rc.{WORD_WRAP}", "v6.9.0-rc.1", False),
        ):
            with self.subTest(installed=installed, target=target, docker=docker):
                preview = "-" in target
                r, mutations, _ = self.exercise([release(target, prerelease=preview)], installed,
                                                "rc" if preview else "stable", docker=docker)
                self.assert_unchanged(r, mutations, refused=True)
                self.assertIn("Cannot determine the installed Pulse version", r.stderr)

    def test_unorderable_selected_version_stops_before_any_update(self):
        for target, installed, docker in (
            ("v7.0.09", "v6.9.0", False),
            (f"v{SIGNED_OVERFLOW}.0.0", "v6.9.0", True),
            (f"v6.{WORD_WRAP + 1}.0", "v6.0.0", False),
            (f"v7.0.{SIGNED_OVERFLOW}", "v6.9.0", True),
            (f"v6.9.0-beta.{WORD_WRAP + 1}", "v6.9.0-beta.0", False),
            (f"v6.9.0-rc.{SIGNED_OVERFLOW}", "v6.9.0-beta.1", True),
            ("v6.9.0-rc.01", "v6.9.0-beta.1", False),
        ):
            with self.subTest(target=target, installed=installed, docker=docker):
                preview = "-" in target
                data = [release(target, prerelease=preview), release("v6.9.1")]
                r, mutations, _ = self.exercise(data, installed, "rc" if preview else "stable",
                                                docker=docker)
                self.assert_unchanged(r, mutations, refused=True)
                self.assertIn("Cannot compare the installed Pulse version", r.stderr)

    def test_decimal_comparison_at_signed_maximum_remains_exact(self):
        for newest, previous, channel in (
            (f"v{SIGNED_MAX}.0.0", f"v{SIGNED_MAX - 1}.0.0", "stable"),
            (f"v6.9.0-rc.{SIGNED_MAX}", f"v6.9.0-rc.{SIGNED_MAX - 1}", "rc"),
        ):
            with self.subTest(newest=newest, previous=previous):
                r, mutations, _ = self.exercise([release(newest, prerelease=channel == "rc")],
                                                previous, channel)
                self.assert_download(r, mutations, newest, channel)
                r, mutations, _ = self.exercise([release(previous, prerelease=channel == "rc")],
                                                newest, channel)
                self.assert_unchanged(r, mutations, refused=True)
                self.assertIn("refusing to downgrade automatically", r.stderr)

    def test_unorderable_installed_identity_keeps_deliberate_recovery(self):
        version = "v6.8.1"
        r, mutations, transport = self.exercise([], installed="v6.09.0", pinned=version)
        self.assert_download(r, mutations, version, "")
        self.assertEqual(transport, [])
        self.assertEqual(r.stderr, "")
        r, mutations, _ = self.exercise([release(version)], "v6.09.0", choice="1")
        self.assert_download(r, mutations, version, "stable")
        self.assertEqual(r.stderr, "")
        r, mutations, _ = self.exercise([release(version)], "v6.09.0", choice="2")
        self.assert_unchanged(r, mutations, refused=True)
        self.assertIn("Cannot reinstall an unknown current server version", r.stderr)

    def test_equivalent_version_spelling_is_an_automatic_noop(self):
        r, mutations, _ = self.exercise([release("6.5.0")])
        self.assert_unchanged(r, mutations)

    def test_preview_revision_upgrade_uses_numeric_not_lexical_order(self):
        for stage in ("beta", "rc"):
            with self.subTest(stage=stage):
                target = f"v6.6.0-{stage}.10"
                data = [release(target, prerelease=True), release("v6.5.0")]
                r, mutations, _ = self.exercise(data, f"v6.6.0-{stage}.9", "rc")
                self.assert_download(r, mutations, target, "rc")
                r, mutations, _ = self.exercise(data, target, "rc")
                self.assert_unchanged(r, mutations)

    def test_preview_stage_and_reverse_revision_are_not_downgrades_in_disguise(self):
        data = [release("v6.6.0-rc.1", prerelease=True), release("v6.5.0")]
        r, mutations, _ = self.exercise(data, "v6.6.0-beta.99", "rc")
        self.assert_download(r, mutations, "v6.6.0-rc.1", "rc")
        for target, installed in (("v6.6.0-rc.9", "v6.6.0-rc.10"),
                                  ("v6.6.0-beta.99", "v6.6.0-rc.1"),
                                  ("v6.6.0-rc.1", "v6.6.0")):
            data = [release(target, prerelease=True), release("v6.5.0")]
            r, mutations, _ = self.exercise(data, installed, "rc")
            self.assert_unchanged(r, mutations, refused=True)

    def test_explicit_reinstall_pins_current_version_not_latest(self):
        data = [release("v6.7.0-rc.1", prerelease=True), release("v6.6.0")]
        r, mutations, _ = self.exercise(data, choice="3")
        self.assert_download(r, mutations, "v6.5.0", "stable")
        r, mutations, _ = self.exercise(data, installed="unknown", choice="3")
        self.assert_unchanged(r, mutations, refused=True)

    def test_deliberate_update_rows_remove_and_cancel_have_exact_meaning(self):
        data = [release("v6.7.0-rc.1", prerelease=True), release("v6.6.0")]
        for choice, target, channel in (("1", "v6.6.0", "stable"),
                                        ("2", "v6.7.0-rc.1", "rc")):
            r, mutations, _ = self.exercise(data, choice=choice)
            self.assert_download(r, mutations, target, channel)
        r, mutations, _ = self.exercise(data, choice="4")
        self.assertEqual(r.returncode, 77)
        self.assertEqual(mutations, ["REMOVE"])
        r, mutations, _ = self.exercise(data, choice="5")
        self.assert_unchanged(r, mutations)

    def test_manual_rows_stay_correct_when_stable_update_is_absent(self):
        data = [release("v6.6.0-rc.1", prerelease=True), release("v6.5.0")]
        r, mutations, _ = self.exercise(data, choice="1")
        self.assert_download(r, mutations, "v6.6.0-rc.1", "rc")
        r, mutations, _ = self.exercise(data, choice="2")
        self.assert_download(r, mutations, "v6.5.0", "stable")
        r, mutations, _ = self.exercise(data, choice="3")
        self.assertEqual(r.returncode, 77)
        self.assertEqual(mutations, ["REMOVE"])
        r, mutations, _ = self.exercise(data, choice="4")
        self.assert_unchanged(r, mutations)

    def test_invalid_or_empty_manual_choice_never_mutates(self):
        for choice in ("", "0", "99", "not-a-number"):
            r, mutations, _ = self.exercise([release("v6.6.0")], choice=choice)
            self.assert_unchanged(r, mutations, refused=True)

    def test_explicit_version_bypasses_menu_and_discovery(self):
        r, mutations, transport = self.exercise([], pinned="v6.6.0-rc.1")
        self.assert_download(r, mutations, "v6.6.0-rc.1", "")
        self.assertEqual(transport, [])
        self.assertNotIn("What would you like to do?", r.stdout)


if __name__ == "__main__":
    unittest.main()
