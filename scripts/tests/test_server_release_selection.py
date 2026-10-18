#!/usr/bin/env python3
"""Execute the server installer's selectors with offline release-list fixtures.

These are selector controls only: no download, service or native install proof.
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
# No installer main, systemd operation, package install or network call occurs.
print_info() { :; }
print_warn() { :; }
print_error() { printf '%s\n' "$*" >&2; }
timeout() { shift; "$@"; }
curl() {
    local url="${@: -1}"
    printf '%s\n' "$url" >> "$FIXTURE/calls"
    case "$url" in
        https://api.github.com/repos/rcourtman/Pulse/releases)
            printf '%s' "$RELEASES_JSON"
            return "$API_EXIT"
            ;;
        https://github.com/rcourtman/Pulse/releases/latest)
            [[ -n "$REDIRECT_TAG" ]] || return 22
            printf 'https://github.com/rcourtman/Pulse/releases/tag/%s' "$REDIRECT_TAG"
            ;;
        https://api.github.com/repos/rcourtman/Pulse/releases/tags/*) return 0 ;;
        *) printf 'unexpected transport: %s\n' "$url" >&2; return 97 ;;
    esac
}
if [[ "$NO_JQ" == true ]]; then
    command() {
        [[ "$*" != '-v jq' ]] || return 1
        builtin command "$@"
    }
elif [[ "$FAILED_JQ" == true ]]; then
    jq() { printf 'v99.0.0\n'; return 3; }
fi
FORCE_VERSION="$PINNED_VERSION"
FORCE_CHANNEL="$CHANNEL"
UPDATE_CHANNEL="$CHANNEL"
LATEST_RELEASE="$CACHED_VERSION"
CURRENT_VERSION="$INSTALLED_VERSION"
case "$SELECTOR" in
    parser) latest_pulse_release_tag_from_json "$RELEASES_JSON" "$CHANNEL" ;;
    tag) resolve_latest_release_tag_for_channel "$CHANNEL" ;;
    url) resolve_install_script_download_url ;;
    target) resolve_target_release; printf '%s\n' "$LATEST_RELEASE" ;;
    *) exit 98 ;;
esac
'''


class ServerReleaseSelectionTest(unittest.TestCase):
    def exercise(self, data, channel="rc", selector="tag", redirect="", *,
                 no_jq=False, failed_jq=False, api_exit=0, pinned="", cached="", installed=""):
        payload = data if isinstance(data, str) else json.dumps(data)
        with tempfile.TemporaryDirectory() as temporary:
            fixture = Path(temporary)
            env = {**os.environ, "FIXTURE": str(fixture), "INSTALLER": str(INSTALLER),
                   "PULSE_CONFIG_DIR": str(fixture / "config"), "RELEASES_JSON": payload,
                   "CHANNEL": channel, "SELECTOR": selector, "REDIRECT_TAG": redirect,
                   "NO_JQ": str(no_jq).lower(), "FAILED_JQ": str(failed_jq).lower(),
                   "API_EXIT": str(api_exit), "PINNED_VERSION": pinned,
                   "CACHED_VERSION": cached, "INSTALLED_VERSION": installed}
            result = subprocess.run(["bash", "-c", HARNESS], env=env,
                                    capture_output=True, text=True, timeout=15)
            calls = (fixture / "calls").read_text().splitlines() if (fixture / "calls").exists() else []
            return result, calls

    def assert_selection(self, data, channel, expected, **kwargs):
        for selector in ("tag", "url", "target"):
            with self.subTest(selector=selector, channel=channel, options=kwargs):
                result, calls = self.exercise(data, channel, selector, **kwargs)
                if expected:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    want = (f"https://github.com/rcourtman/Pulse/releases/download/{expected}/install.sh"
                            if selector == "url" else expected)
                    self.assertEqual(result.stdout.strip(), want)
                else:
                    self.assertNotEqual(result.returncode, 0, result.stdout)
                    self.assertEqual(result.stdout, "")
                # Only release discovery is admitted by the transport double.
                self.assertLessEqual(len(calls), 2, calls)
                self.assertFalse(any("/download/" in url for url in calls), calls)

    def test_chart_and_draft_entries_never_select_server_or_installer(self):
        data = [release("helm-chart-6.6.0-rc.1", prerelease=True),
                release("v6.7.0-rc.1", draft=True, prerelease=True),
                release("helm-chart-6.5.0"), release("v6.6.0-rc.1", prerelease=True),
                release("v6.5.0")]
        for payload in (data, json.dumps(data, indent=2)):
            self.assert_selection(payload, "rc", "v6.6.0-rc.1")
            self.assert_selection(payload, "stable", "v6.5.0")

    def test_channel_and_published_order_are_preserved(self):
        for tag in ("v6.6.0-beta.2", "v6.6.0-rc.2", "6.6.0-rc.2", "v6.6.0", "6.6.0"):
            data = [release(tag, prerelease="-" in tag), release("v6.5.0")]
            self.assert_selection(data, "rc", tag)
            self.assert_selection(data, "stable", "v6.5.0" if "-" in tag else tag)
        # Preview permits a newer published stable, not only prereleases.
        self.assert_selection([release("v6.5.0"), release("v6.5.0-rc.1", prerelease=True)], "rc", "v6.5.0")
        # A prerelease-shaped tag is not stable even with a false API flag.
        self.assert_selection([release("v6.6.0-rc.1"), release("v6.5.0")], "stable", "v6.5.0")

    def test_complete_top_level_metadata_not_tag_shaped_text_is_required(self):
        valid = json.dumps([release("v6.5.0")])
        for data in (valid + " trailing", valid + valid, valid[:-1],
                     json.dumps(release("v6.5.0")), "null", "", '"v6.5.0"'):
            for channel in ("stable", "rc"):
                self.assert_selection(data, channel, "", redirect="helm-chart-6.5.0")
        data = [None, False, "v99.0.0", {"nested": release("v99.0.0")},
                release("v99.0.0", draft="false"), release("v99.0.0", prerelease="false"),
                {"tag_name": "v99.0.0", "draft": False}, release(123),
                release("v6.7.0\n"), release("v6.7.0;touch owned"), release("latest"),
                release("v6.7.0-preview.1", prerelease=True), release("v6.5.0")]
        for channel in ("stable", "rc"):
            self.assert_selection(data, channel, "v6.5.0")

    def test_missing_or_failed_parser_does_not_guess_draft_tag(self):
        data = [release("v99.0.0", draft=True), release("v6.5.0")]
        for options in ({"no_jq": True}, {"failed_jq": True}):
            for channel in ("stable", "rc"):
                self.assert_selection(data, channel, "", **options)
                self.assert_selection(data, channel, "", redirect="helm-chart-6.5.0", **options)
                self.assert_selection(data, channel, "v6.5.0", redirect="v6.5.0", **options)

    def test_failed_transport_never_admits_even_complete_returned_list(self):
        data = [release("v99.0.0")]
        for channel in ("stable", "rc"):
            self.assert_selection(data, channel, "", api_exit=22)
            self.assert_selection(data, channel, "v6.5.0", redirect="v6.5.0", api_exit=22)

    def test_redirect_obeys_same_server_and_channel_tag_scope(self):
        for tag in ("helm-chart-6.5.0", "null", "latest", "v6.5"):
            for channel in ("stable", "rc"):
                self.assert_selection("", channel, "", redirect=tag)
        # Preserve the redirect extractor's intermediate-path/query tolerance:
        # only its canonical server tag enters a download URL, not the suffix.
        for tag in ("v6.5.0/other", "v6.5.0?other", "v6.5.0#other"):
            self.assert_selection("", "stable", "v6.5.0", redirect=tag)
        self.assert_selection("", "stable", "", redirect="v6.6.0-rc.1")
        self.assert_selection("", "rc", "v6.6.0-rc.1", redirect="v6.6.0-rc.1")
        self.assert_selection("", "rc", "v6.6.0-beta.1", redirect="v6.6.0-beta.1")

    def test_unknown_channel_is_rejected_before_transport(self):
        result, calls = self.exercise([release("v6.5.0")], channel="other")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])

    def test_explicit_pins_and_cached_target_remain_intentional(self):
        for selector in ("url", "target"):
            result, calls = self.exercise([], selector=selector, pinned="v6.5.0-rc.1")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("v6.5.0-rc.1", result.stdout)
            self.assertTrue(all("/releases/tags/" in call for call in calls))
        result, calls = self.exercise([], selector="target", cached="v6.5.0")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "v6.5.0")
        self.assertEqual(calls, [])

    def test_automatic_downgrade_refusal_is_preserved(self):
        result, _ = self.exercise([release("v6.5.0")], selector="target", installed="v6.6.0-rc.1")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "")
        self.assertIn("refusing to downgrade automatically", result.stderr)

    def test_refused_selection_has_specific_safe_recovery_guidance(self):
        result, _ = self.exercise([], selector="target", redirect="helm-chart-6.5.0")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("published Pulse server release", result.stderr)
        self.assertIn("rc channel", result.stderr)


if __name__ == "__main__":
    unittest.main()
