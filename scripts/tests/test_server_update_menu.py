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


def release(tag, *, draft=False, prerelease=False):
    return {"tag_name": tag, "draft": draft, "prerelease": prerelease}


HARNESS = r'''
source "$INSTALLER"
FORCE_VERSION="$PIN"
FORCE_CHANNEL="$CHANNEL"
CURRENT_VERSION="$INSTALLED"
IN_CONTAINER=true
IN_DOCKER="$DOCKER"
print_header() { :; }
print_info() { printf '%s\n' "$*"; }
print_warn() { printf '%s\n' "$*"; }
print_error() { printf '%s\n' "$*" >&2; }
check_root() { :; }
detect_os() { :; }
check_docker_environment() { :; }
check_existing_installation() { return 0; }
check_proxmox_host() { return 1; }
create_lxc_container() { exit 96; }
timeout() { shift; "$@"; }
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
backup_existing() { echo BACKUP >> "$FIXTURE/mutations"; }
create_user() { echo USER >> "$FIXTURE/mutations"; }
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
                 redirect="", no_jq=False, pinned="", compact=False):
        payload = (releases if isinstance(releases, str) else
                   json.dumps(releases, indent=None if compact else 2))
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            config = fixture / "config"
            config.mkdir()
            (config / "system.json").write_text(json.dumps({"updateChannel": configured}))
            env = {**os.environ, "INSTALLER": str(INSTALLER), "FIXTURE": str(fixture),
                   "PULSE_CONFIG_DIR": str(config), "RELEASES_JSON": payload,
                   "INSTALLED": installed, "CHANNEL": channel, "MENU_CHOICE": choice,
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
        r, mutations, _ = self.exercise(data, channel="rc")
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
                                  ("v6.6.0-beta.99", "v6.6.0-rc.1")):
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
