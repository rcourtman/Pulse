#!/usr/bin/env python3
"""Contract tests for the shadow release lifecycle rehearsal."""

from __future__ import annotations

import importlib.util
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
RESOLVER_PATH = REPO_ROOT / "scripts" / "release_lifecycle_rehearsal_versions.py"
HARNESS_PATH = REPO_ROOT / "scripts" / "release_lifecycle_rehearsal.sh"
WORKFLOW_PATH = REPO_ROOT / ".github" / "workflows" / "release-lifecycle-rehearsal.yml"
SMOKE_BODY_PATH = REPO_ROOT / ".github" / "workflows" / "install-sh-smoke-body.yml"

SPEC = importlib.util.spec_from_file_location("release_lifecycle_rehearsal_versions", RESOLVER_PATH)
assert SPEC and SPEC.loader
versions = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = versions
SPEC.loader.exec_module(versions)


def release(tag: str, *, draft: bool = False, prerelease: bool | None = None) -> dict:
    return {
        "tag_name": tag,
        "draft": draft,
        "prerelease": ("-" in tag) if prerelease is None else prerelease,
    }


RELEASES = [
    release("v6.4.5"),
    release("helm-chart-6.4.5"),
    release("v6.4.5-rc.5"),
    release("v6.4.5-rc.10"),
    release("v6.4.5-beta.3"),
    release("v6.4.1"),
    release("v6.4.0"),
    release("v6.4.0-rc.9"),
    release("v6.4.6-rc.1", draft=True),
]


class SemverOrderingTest(unittest.TestCase):
    def test_release_outranks_its_prereleases(self) -> None:
        self.assertGreater(versions.compare_tags("v6.4.5", "v6.4.5-rc.10"), 0)

    def test_numeric_identifiers_compare_numerically(self) -> None:
        self.assertGreater(versions.compare_tags("v6.4.5-rc.10", "v6.4.5-rc.5"), 0)

    def test_alphanumeric_identifiers_compare_lexically(self) -> None:
        self.assertGreater(versions.compare_tags("v6.4.5-rc.1", "v6.4.5-beta.3"), 0)

    def test_rejects_non_release_tags(self) -> None:
        for tag in ("6.4.5", "helm-chart-6.4.5", "v6.4", "v6.4.5;id", "v06.4.5"):
            with self.subTest(tag=tag), self.assertRaises(versions.ResolutionError):
                versions.parse_tag(tag)


class ResolvePairTest(unittest.TestCase):
    def test_defaults_to_newer_prerelease_after_latest_stable(self) -> None:
        releases = RELEASES + [release("v6.4.6-rc.2"), release("v6.4.6-beta.1")]
        selection = versions.resolve(releases, "v6.4.5")
        self.assertEqual((selection.from_tag, selection.to_tag), ("v6.4.5", "v6.4.6-rc.2"))

    def test_defaults_fall_back_to_previous_stable_when_nothing_is_newer(self) -> None:
        selection = versions.resolve(RELEASES, "v6.4.5")
        self.assertEqual((selection.from_tag, selection.to_tag), ("v6.4.1", "v6.4.5"))
        self.assertIn("previous stable", selection.reason)

    def test_draft_releases_are_never_targets(self) -> None:
        selection = versions.resolve(RELEASES, "v6.4.5")
        self.assertNotEqual(selection.to_tag, "v6.4.6-rc.1")
        with self.assertRaises(versions.ResolutionError):
            versions.resolve(RELEASES, "v6.4.5", "v6.4.5", "v6.4.6-rc.1")

    def test_explicit_from_selects_newest_newer_release(self) -> None:
        selection = versions.resolve(RELEASES, "v6.4.5", "v6.4.0")
        self.assertEqual((selection.from_tag, selection.to_tag), ("v6.4.0", "v6.4.5"))

    def test_explicit_from_without_newer_release_fails_instead_of_passing(self) -> None:
        with self.assertRaises(versions.ResolutionError):
            versions.resolve(RELEASES, "v6.4.5", "v6.4.5")

    def test_explicit_pair_must_move_forward(self) -> None:
        for from_tag, to_tag in (("v6.4.5", "v6.4.1"), ("v6.4.5", "v6.4.5")):
            with self.subTest(pair=(from_tag, to_tag)), self.assertRaises(versions.ResolutionError):
                versions.resolve(RELEASES, "v6.4.5", from_tag, to_tag)

    def test_unpublished_or_malformed_requests_fail(self) -> None:
        for from_tag, to_tag in (("v6.3.0", ""), ("v6.4.1", "v6.9.9"), ("v6.4.1 ; id", "")):
            with self.subTest(pair=(from_tag, to_tag)), self.assertRaises(versions.ResolutionError):
                versions.resolve(RELEASES, "v6.4.5", from_tag, to_tag)

    def test_latest_must_be_published_stable(self) -> None:
        for latest in ("", "v6.4.5-rc.5", "v9.9.9"):
            with self.subTest(latest=latest), self.assertRaises(versions.ResolutionError):
                versions.resolve(RELEASES, latest)

    def test_cli_emits_key_value_pair(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "releases.json"
            path.write_text(json.dumps(RELEASES), encoding="utf-8")
            result = subprocess.run(
                [sys.executable, str(RESOLVER_PATH), "--releases-json", str(path),
                 "--latest-tag", "v6.4.5", "--from", "v6.4.0", "--to", "v6.4.1"],
                capture_output=True, text=True, check=False,
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("from_tag=v6.4.0\n", result.stdout)
        self.assertIn("to_tag=v6.4.1\n", result.stdout)


class HarnessContractTest(unittest.TestCase):
    harness = HARNESS_PATH.read_text(encoding="utf-8")

    def test_upgrade_and_rollback_use_the_installed_update_helper(self) -> None:
        self.assertIn("cexec '/bin/update --version \"$TARGET\"'", self.harness)
        self.assertIn('run_updater "$TO_TAG" upgrade', self.harness)
        self.assertIn('run_updater "$FROM_TAG" rollback', self.harness)
        self.assertIn('grep -q "^# Pulse update command" /bin/update', self.harness)
        # The updater invocation carries no flags that would bypass the
        # behavior users get (archives, preflight skips, forced auto-update).
        updater_lines = [line for line in self.harness.splitlines() if "/bin/update --" in line]
        for line in updater_lines:
            for flag in ("--archive", "--skip-upgrade-preflight", "--disable-auto-updates"):
                self.assertNotIn(flag, line)

    def test_initial_install_uses_signed_published_installer(self) -> None:
        for needle in (
            "releases/download/${tag}",
            "grep -oE 'ssh-ed25519 [A-Za-z0-9+/=]+ pulse-installer'",
            "-I pulse-installer",
            "-n pulse-install",
            "bash /rehearsal/install.sh --version \"$FROM_TAG\"",
        ):
            self.assertIn(needle, self.harness)

    def test_uses_the_same_pinned_systemd_image_as_install_smoke(self) -> None:
        digest = re.search(r"jrei/systemd-debian:12@sha256:[0-9a-f]{64}", self.harness)
        self.assertIsNotNone(digest)
        self.assertIn(digest.group(0), SMOKE_BODY_PATH.read_text(encoding="utf-8"))

    def test_every_post_change_phase_checks_identity_health_settings_and_data(self) -> None:
        for phase in ("phase_upgrade", "phase_rollback"):
            body = self.harness.split(f"{phase}() {{", 1)[1].split("\n}\n", 1)[0]
            for check in ("assert_runtime", "check_auto_update_intent", "check_settings", "check_datadir", "record_phase"):
                with self.subTest(phase=phase, check=check):
                    self.assertIn(check, body)

    def test_settings_are_checked_against_fixed_expectations_not_only_baseline(self) -> None:
        body = self.harness.split("check_settings() {", 1)[1].split("\n}\n", 1)[0]
        self.assertIn("check_snapshot_shape", body)
        self.assertIn("settings.baseline.json", body)

    def test_harness_passes_shellcheck_syntax(self) -> None:
        result = subprocess.run(["bash", "-n", str(HARNESS_PATH)], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_rejects_unsafe_tags_before_touching_a_container(self) -> None:
        for args in (["--from", "v6.4.1", "--to", "v6.4.1"],
                     ["--from", "v6.4.1;id", "--to", "v6.4.5"],
                     ["--from", "", "--to", "v6.4.5"]):
            with self.subTest(args=args):
                result = subprocess.run(
                    ["bash", str(HARNESS_PATH), *args],
                    capture_output=True, text=True,
                    env={"PATH": "/usr/bin:/bin", "PULSE_REHEARSAL_ENGINE": "none-such"},
                )
                self.assertEqual(result.returncode, 2, result.stdout + result.stderr)


class HostedWorkflowCommandFilesTest(unittest.TestCase):
    def test_bound_pair_uses_the_reviewed_collision_safe_output_writer(self) -> None:
        source = (REPO_ROOT / '.github/workflows/qualify-browser-update-release.yml').read_text()
        for field in ('from', 'from_commit', 'to_commit'):
            self.assertIn(f'python3 scripts/write_github_output.py {field} "${field}"', source)
        self.assertNotIn('echo "to_commit=', source)
        node_setup = source.split('uses: actions/setup-node@', 1)[1].split('      - name:', 1)[0]
        self.assertNotRegex(node_setup, r'(?m)^\s*cache:')
        self.assertIn('package-manager-cache: false', source)


def harness_functions(*names):
    source = HARNESS_PATH.read_text()
    return '\n'.join(name + '() {' + source.split(name + '() {', 1)[1].split('\n}\n', 1)[0]
                     + '\n}' for name in names) + '\n'


class SelectedInstallerControlTest(unittest.TestCase):
    """Execute the actual controller with synthetic installers, never systemd.

    Command selection/failure propagation are proved here. Published native
    install, state and timer acceptance still need the hosted matrix's receipts.
    """

    def test_changed_staged_installer_cannot_execute(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'assets/v6.6.0-rc.4').mkdir(parents=True)
            (root / 'assets/v6.6.0-rc.4/install.sh').write_text('changed after signature admission')
            (root / 'packet.json').write_text(json.dumps({'releases': [
                {'tag': 'v6.6.0-rc.4', 'installer_sha256': '0' * 64}]}))
            script = f'WORK_DIR={shlex.quote(tmp)}\nPULSE_REHEARSAL_PACKET_MANIFEST="$WORK_DIR/packet.json"\n'
            script += harness_functions('execute_hosted_installer')
            script += 'cexec() { echo unexpected-execution; exit 91; }\n'
            script += 'execute_hosted_installer v6.6.0-rc.4 v6.6.0-rc.4 upgrade\n'
            result = subprocess.run(['bash', '-c', script], capture_output=True, text=True)
            self.assertEqual(1, result.returncode, result.stdout + result.stderr)
            self.assertEqual('', result.stdout)

    def test_installed_and_running_executables_must_both_match_the_signed_archive(self):
        for running in ('a' * 64, 'b' * 64, 'unreadable'):
            with self.subTest(running=running), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / 'state').mkdir()
                (root / 'packet.json').write_text(json.dumps({'releases': [
                    {'tag': 'v6.6.0-rc.4', 'binary_sha256': 'a' * 64}]}))
                script = 'set -euo pipefail\n'
                script += f'WORK_DIR={shlex.quote(tmp)}; RUNNING={running}\n'
                script += 'HOSTED_RUNNER=true; PULSE_REHEARSAL_PACKET_MANIFEST="$WORK_DIR/packet.json"\n'
                script += 'BASELINE_FAILED_UNITS=""; CONTAINER_TOLERATED_UNITS=""; CURRENT_FAILURE=""; API=synthetic\n'
                script += harness_functions('note_failure', 'assert_runtime')
                script += '''
cexec() {
    case "$1" in
        'systemctl is-active pulse') echo active ;;
        *'/api/health'*) echo '{"status":"healthy"}' ;;
        *'/api/version'*) echo '{"version":"6.6.0-rc.4"}' ;;
        *'/opt/pulse/bin/pulse --version'*) echo v6.6.0-rc.4 ;;
        *'sha256sum /opt/pulse/bin/pulse'*) printf '%064d\n' 0 | tr 0 a ;;
        *'MainPID'*) [[ "$RUNNING" != unreadable ]] && echo "$RUNNING" ;;
        'systemctl is-enabled pulse') echo enabled ;;
        *'list-units --state=failed'*) : ;;
        *'PULSE_TELEMETRY=false'*) : ;;
        *'ExecMainStartTimestampMonotonic'*) echo 0 ;;
        *) echo unexpected-command >&2; return 95 ;;
    esac
}
assert_runtime v6.6.0-rc.4 upgrade
[[ -z "$CURRENT_FAILURE" ]]
'''
                result = subprocess.run(['bash', '-c', script], capture_output=True, text=True)
                self.assertEqual(0 if running == 'a' * 64 else 1, result.returncode, result.stdout + result.stderr)
                identity = root / 'state/installed-upgrade.json'
                self.assertEqual(running != 'unreadable', identity.exists())
                if identity.exists():
                    self.assertEqual(running, json.loads(identity.read_text())['running_binary_sha256'])

    def test_selected_rc_installer_runs_both_directions_and_retains_failure(self):
        for forward_exit in (0, 23):
            with self.subTest(forward_exit=forward_exit), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                (root / 'state').mkdir()
                for tag in ('v6.5.0', 'v6.6.0-rc.4'):
                    folder = root / 'assets' / tag
                    folder.mkdir(parents=True)
                    (folder / 'install.sh').write_text(
                        '#!/bin/bash\nset -eu\n'
                        'printf "%s %s %s\\n" "$INSTALLER_TAG" "$1" "$2" >> "$FIXTURE_CALLS"\n'
                        '[[ "$1" == --version ]]\n'
                        f'if [[ "$2" == v6.6.0-rc.4 ]]; then exit {forward_exit}; fi\n')
                manifest = root / 'packet.json'
                manifest.write_text(json.dumps({'releases': [{'tag': tag, 'installer_sha256':
                    hashlib.sha256((root / f'assets/{tag}/install.sh').read_bytes()).hexdigest()}
                    for tag in ('v6.5.0', 'v6.6.0-rc.4')]}))
                script = 'set -euo pipefail\n'
                script += f'WORK_DIR={shlex.quote(tmp)}; FIXTURE_CALLS="$WORK_DIR/calls"\n'
                script += 'PULSE_REHEARSAL_PACKET_MANIFEST="$WORK_DIR/packet.json"\n'
                script += 'FROM_TAG=v6.5.0; TO_TAG=v6.6.0-rc.4; HOSTED_NATIVE=true; HOSTED_BROWSER=false\n'
                script += 'CURRENT_FAILURE=""; OVERALL_STATUS=0; FAILURES=(); PHASE_ROWS=()\n'
                script += harness_functions('note_failure', 'reset_phase_cells', 'record_phase',
                                            'execute_hosted_installer', 'run_selected_installer',
                                            'phase_upgrade', 'phase_rollback')
                # Substitute only the root execution boundary. The actual
                # command and selected file run in an ordinary temporary dir.
                script += r'''
cexec() {
    local program="$1"; shift
    program="${program//\/rehearsal/$WORK_DIR/assets}"
    env -i PATH=/usr/bin:/bin "FIXTURE_CALLS=$FIXTURE_CALLS" "$@" bash -c "$program"
}
run_updater() { echo unexpected-stable-bootstrap; exit 99; }
run_browser_journey() { echo unexpected-browser; exit 98; }
assert_runtime() { printf 'runtime %s %s\n' "$1" "$2" >> "$WORK_DIR/checks"; }
check_auto_update_intent() { echo "timer $1" >> "$WORK_DIR/checks"; }
check_settings() { echo "settings $1" >> "$WORK_DIR/checks"; }
check_datadir() { echo "data $1" >> "$WORK_DIR/checks"; }
phase_upgrade || true
phase_rollback || true
exit "$OVERALL_STATUS"
'''
                result = subprocess.run(['bash', '-c', script], capture_output=True, text=True)
                self.assertEqual(1 if forward_exit else 0, result.returncode, result.stdout + result.stderr)
                self.assertEqual('v6.6.0-rc.4 --version v6.6.0-rc.4\n'
                                 'v6.6.0-rc.4 --version v6.5.0\n', (root / 'calls').read_text())
                self.assertEqual(['runtime v6.6.0-rc.4 upgrade', 'timer upgrade', 'settings upgrade', 'data upgrade',
                                  'runtime v6.5.0 rollback', 'timer rollback', 'settings rollback', 'data rollback'],
                                 (root / 'checks').read_text().splitlines())
                digest = hashlib.sha256((root / 'assets/v6.6.0-rc.4/install.sh').read_bytes()).hexdigest()
                for label, target, observed in (('upgrade', 'v6.6.0-rc.4', forward_exit), ('rollback', 'v6.5.0', 0)):
                    self.assertEqual({'installer_tag': 'v6.6.0-rc.4', 'target': target,
                                      'installer_sha256': digest, 'command_exit': observed},
                                     json.loads((root / f'state/installer-{label}.json').read_text()))

    def test_native_and_browser_are_independent_first_level_vms(self):
        workflow = (REPO_ROOT / '.github/workflows/qualify-browser-update-release.yml').read_text()
        self.assertIn('mode: [native, browser]', workflow)
        self.assertIn('fail-fast: false', workflow)
        self.assertNotIn('needs:', workflow)
        steps = workflow.split('      - name: ')[1:]
        native = next(step for step in steps if step.startswith('Exercise signed selected-installer'))
        self.assertIn("if: matrix.mode == 'native'", native)
        self.assertIn('sudo env -i', native)
        self.assertIn('--hosted-native --from "$FROM" --to "$TAG"', native)
        for excluded in ('secrets.', 'TS_', 'ORIGIN', 'Docker', '--hosted-browser'):
            self.assertNotIn(excluded, native)
        for step in steps:
            if any(marker in step for marker in ('secrets.', 'tailscale status', 'uses: tailscale/',
                                                 'actions/setup-node@', '--hosted-browser')):
                self.assertIn("if: matrix.mode == 'browser'", step, step.splitlines()[0])
        artifact = next(step for step in steps if step.startswith('Retain bounded'))
        self.assertIn('name: ${{ matrix.mode }}-release-${{ github.run_id }}', artifact)
        self.assertIn('native-cleanup.json', artifact)
        self.assertIn('installer-*.json', artifact)
        self.assertNotIn('private-', artifact)


class SignedInstallerStagingTest(unittest.TestCase):
    def test_both_exact_installers_verify_before_first_mutation_with_no_network_fallback(self):
        variants = ('valid', 'changed-size', 'changed-bytes', 'bad-signature',
                    'missing-identity', 'symlink', 'signed-agent')
        for variant in variants:
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                key = root / 'synthetic-key'
                subprocess.run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-C', 'pulse-installer',
                                '-f', str(key)], check=True, capture_output=True)
                readme = root / 'README.md'
                readme.write_text(key.with_suffix('.pub').read_text())
                packet_root = root / 'packet'
                packet_root.mkdir()
                releases = []
                for tag in ('v6.5.0', 'v6.6.0-rc.4'):
                    folder = packet_root / tag
                    folder.mkdir()
                    installer = folder / 'install.sh'
                    installer.write_text('# Pulse Installer Script\ncase "$1" in\n'
                                         '  --version) : ;;\nesac\n')
                    if variant == 'signed-agent' and '-rc.' in tag:
                        installer.write_text(installer.read_text() + '# Pulse Unified Agent Installer\n')
                    subprocess.run(['ssh-keygen', '-Y', 'sign', '-f', str(key), '-n', 'pulse-install', str(installer)],
                                   check=True, capture_output=True)
                    # OpenSSH writes .sig; Pulse publishes that as .sshsig.
                    (folder / 'install.sh.sig').rename(folder / 'install.sh.sshsig')
                    if variant == 'bad-signature' and '-rc.' in tag:
                        installer.with_suffix('.sh.sshsig').write_text('invalid signature')
                    assets = [{'name': p.name, 'size': p.stat().st_size,
                               'digest': 'sha256:' + hashlib.sha256(p.read_bytes()).hexdigest()}
                              for p in (installer, folder / 'install.sh.sshsig')]
                    releases.append({'tag': tag, 'assets': assets})
                target = packet_root / 'v6.6.0-rc.4/install.sh'
                if variant == 'changed-size':
                    target.write_text(target.read_text() + '# mutation\n')
                if variant == 'changed-bytes':
                    target.write_text(target.read_text().replace('Script', 'ScripX'))
                if variant == 'missing-identity':
                    releases[1]['assets'] = []
                if variant == 'symlink':
                    real = target.with_name('other-file')
                    target.rename(real)
                    target.symlink_to(real)
                manifest = packet_root / 'packet.json'
                manifest.write_text(json.dumps({'releases': releases}))
                work = root / 'work'
                (work / 'assets').mkdir(parents=True)
                (work / 'state').mkdir()
                script = 'set -euo pipefail\n'
                for name, value in {'WORK_DIR': str(work), 'README_PATH': str(readme),
                                    'PULSE_REHEARSAL_PACKET_MANIFEST': str(manifest)}.items():
                    script += f'{name}={shlex.quote(value)}\n'
                script += 'HOSTED_RUNNER=true; HOSTED_MODE=native; FROM_TAG=v6.5.0; TO_TAG=v6.6.0-rc.4; REPO=rcourtman/Pulse\n'
                script += harness_functions('fetch_and_verify_installer', 'main')
                script += '''
log() { :; }
record_phase() { return 1; }
abort_run() { exit 2; }
curl() { echo forbidden-network-fallback; exit 90; }
start_container() { echo first-mutation-boundary > "$WORK_DIR/started"; exit 0; }
main
'''
                result = subprocess.run(['bash', '-c', script], capture_output=True, text=True)
                self.assertEqual(0 if variant == 'valid' else 2, result.returncode, result.stdout + result.stderr)
                self.assertEqual(variant == 'valid', (work / 'started').exists())
                self.assertNotIn('forbidden-network-fallback', result.stdout)
                if variant == 'valid':
                    for tag in ('v6.5.0', 'v6.6.0-rc.4'):
                        self.assertTrue(json.loads((work / f'state/installer-{tag}.json').read_text())['signature_verified'])


class BrowserRecoveryAccessStopTest(unittest.TestCase):
    def test_refused_upgrade_does_not_launch_another_login(self) -> None:
        source = HARNESS_PATH.read_text(encoding="utf-8")
        function = "run_browser_journey() {" + source.split("run_browser_journey() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
        for status in (401, 403):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as tmp:
                work = Path(tmp)
                (work / "state").mkdir()
                (work / "state/browser-upgrade.json").write_text(json.dumps({"status": "failed", "access_refused": True, "login_status": status}))
                script = "set -euo pipefail\nWORK_DIR=" + json.dumps(tmp) + "\n"
                script += function + "\nrun_browser_journey recovery v6.4.5\n"
                # NODE/auth/origin variables are intentionally absent. Reaching
                # process/auth preparation would fail this exact guard test.
                result = subprocess.run(["bash", "-c", script], capture_output=True, text=True)
                self.assertEqual(1, result.returncode, result.stderr)
                self.assertEqual({"status": "not-executed", "reason": "stopped-access-no-reauthentication"},
                    json.loads((work / "state/browser-recovery.json").read_text()))
                self.assertFalse((work / "state/browser-auth.json").exists())
                self.assertEqual("", result.stderr)

    def test_missing_or_malformed_upgrade_receipt_stops_before_auth_preparation(self) -> None:
        source = HARNESS_PATH.read_text(encoding="utf-8")
        function = "run_browser_journey() {" + source.split("run_browser_journey() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
        for receipt in (None, "not JSON", "[]", '{"status":"failed"}',
                        '{"status":"incomplete","access_refused":false}',
                        '{"status":"failed","access_refused":"false"}'):
            with self.subTest(receipt=receipt), tempfile.TemporaryDirectory() as tmp:
                work = Path(tmp)
                (work / "state").mkdir()
                if receipt is not None:
                    (work / "state/browser-upgrade.json").write_text(receipt)
                result = subprocess.run(["bash", "-c", "set -euo pipefail\nWORK_DIR="
                    + json.dumps(tmp) + "\n" + function + "\nrun_browser_journey recovery v6.4.5\n"],
                    capture_output=True, text=True)
                self.assertEqual(1, result.returncode, result.stderr)
                self.assertEqual({"status": "not-executed", "reason": "upgrade-receipt-unavailable"},
                    json.loads((work / "state/browser-recovery.json").read_text()))
                self.assertFalse((work / "state/browser-auth.json").exists())
                self.assertEqual("", result.stderr)


class AutoUpdateIntentTest(unittest.TestCase):
    """Execute the real observer/comparator, with guest systemctl observations."""

    def check(self, baseline: str | None, config: object = None, *,
              load: str = "not-found", enabled: str = "disabled",
              active: str = "inactive") -> subprocess.CompletedProcess:
        harness = HARNESS_PATH.read_text(encoding="utf-8")
        functions = "\n".join(
            name + "() {" + harness.split(name + "() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
            for name in ("auto_update_snapshot", "check_auto_update_intent")
        )
        script = r'''set -euo pipefail
PHASE_UNITS="pulse active/enabled"
note_failure() { echo "::error::$*"; }
systemctl() {
    case "$*" in
        "show -p LoadState --value pulse-update.timer") echo "$LOAD" ;;
        "is-enabled pulse-update.timer") echo "$ENABLED"; [[ "$ENABLED" == enabled ]] ;;
        "is-active pulse-update.timer") echo "$ACTIVE"; [[ "$ACTIVE" == active ]] ;;
        *) return 1 ;;
    esac
}
export -f systemctl
cexec() { bash -c "$1"; }
''' + functions + '\ncheck_auto_update_intent rollback\nprintf "%s\\n" "$PHASE_UNITS"\n'
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            (work / "state").mkdir()
            data = work / "data"
            data.mkdir()
            if baseline is not None:
                (work / "state" / "auto-updates.baseline.tsv").write_text(baseline + "\n", encoding="utf-8")
            if config is not None:
                (data / "system.json").write_text(
                    config if isinstance(config, str) else json.dumps(config), encoding="utf-8")
            return subprocess.run(
                ["bash", "-c", script], capture_output=True, text=True, check=False,
                env={**os.environ, "WORK_DIR": str(work), "DATA_DIR": str(data),
                     "LOAD": load, "ENABLED": enabled, "ACTIVE": active},
            )

    def test_absent_timer_and_unset_choice_stay_absent(self) -> None:
        result = self.check("absent\tabsent\tabsent")
        self.assertEqual(0, result.returncode, result.stdout + result.stderr)
        self.assertIn("auto-updates absent/absent/absent preserved", result.stdout)

    def test_disabled_enabled_and_masked_choices_are_preserved(self) -> None:
        for config, enabled, active in ((False, "disabled", "inactive"),
                                        (True, "enabled", "active"),
                                        (False, "masked", "inactive")):
            with self.subTest(config=config, timer=enabled):
                result = self.check(f"{'enabled' if config else 'disabled'}\t{enabled}\t{active}",
                                    {"autoUpdateEnabled": config}, load="loaded", enabled=enabled, active=active)
                self.assertEqual(0, result.returncode, result.stdout + result.stderr)

    def test_persisted_false_is_not_a_new_opt_in(self) -> None:
        result = self.check("absent\tabsent\tabsent", {"autoUpdateEnabled": False})
        self.assertEqual(0, result.returncode, result.stdout + result.stderr)

    def test_config_timer_or_activity_changes_fail(self) -> None:
        for config, enabled, active in ((True, "disabled", "inactive"),
                                        (False, "enabled", "inactive"),
                                        (False, "disabled", "active")):
            with self.subTest(config=config, timer=enabled, active=active):
                result = self.check("disabled\tdisabled\tinactive", {"autoUpdateEnabled": config},
                                    load="loaded", enabled=enabled, active=active)
                self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                self.assertIn("auto-update choice or timer enablement/activity changed", result.stdout)

    def test_enabled_choice_cannot_be_silently_disabled(self) -> None:
        result = self.check("enabled\tenabled\tactive", {"autoUpdateEnabled": False},
                            load="loaded", enabled="enabled", active="active")
        self.assertEqual(1, result.returncode, result.stdout + result.stderr)

    def test_unreadable_choice_or_timer_is_not_assumed_disabled(self) -> None:
        for config, load, enabled, active in (("not json", "not-found", "disabled", "inactive"),
                                              ({"autoUpdateEnabled": "true"}, "not-found", "disabled", "inactive"),
                                              ({"autoUpdateEnabled": None}, "not-found", "disabled", "inactive"),
                                              (None, "", "disabled", "inactive"),
                                              (None, "loaded", "", "inactive"),
                                              (None, "loaded", "disabled", "unknown")):
            with self.subTest(config=config, load=load, enabled=enabled, active=active):
                result = self.check("absent\tabsent\tabsent", config, load=load, enabled=enabled, active=active)
                self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                self.assertIn("could not be read", result.stdout)

    def test_missing_baseline_cannot_pass(self) -> None:
        result = self.check(None)
        self.assertEqual(1, result.returncode, result.stdout + result.stderr)
        self.assertIn("has no baseline", result.stdout)


GOOD_SNAPSHOT = {
    "auth": {"api_token": "200", "password": "200", "requiresAuth": True, "unauthenticated": "401"},
    "node": {
        "hasPassword": False,
        "hasToken": True,
        "host": "https://192.168.77.10:8006",
        "name": "test-lifecycle-rehearsal",
        "tokenName": "root@pam!rehearsal",
        "type": "pve",
        "verifySSL": False,
    },
    "webhook": {
        "enabled": False,
        "header_keys": ["X-Rehearsal-Secret"],
        "id": "webhook-123",
        "method": "POST",
        "name": "lifecycle-rehearsal-webhook",
        "service": "generic",
        "url": "https://example.com/pulse-lifecycle-rehearsal",
    },
}


def mutated(path: tuple[str, ...], value: object) -> dict:
    snapshot = json.loads(json.dumps(GOOD_SNAPSHOT))
    target = snapshot
    for key in path[:-1]:
        target = target[key]
    target[path[-1]] = value
    return snapshot


class SnapshotExpectationTest(unittest.TestCase):
    """Lost or altered settings must fail even when the baseline also lost them."""

    def check(self, snapshot: object) -> subprocess.CompletedProcess:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "snapshot.json"
            path.write_text(json.dumps(snapshot) if not isinstance(snapshot, str) else snapshot,
                            encoding="utf-8")
            return subprocess.run(
                ["bash", str(HARNESS_PATH), "--check-snapshot", str(path)],
                capture_output=True, text=True, check=False,
                env={"PATH": "/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin"},
            )

    def test_intact_snapshot_passes(self) -> None:
        result = self.check(GOOD_SNAPSHOT)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_lost_or_changed_settings_fail(self) -> None:
        cases = {
            "node token secret lost": (("node", "hasToken"), False),
            "node token name reset": (("node", "tokenName"), ""),
            "node TLS verification flipped": (("node", "verifySSL"), True),
            "node host rewritten": (("node", "host"), "https://192.168.77.11:8006"),
            "node missing": (("node",), None),
            "webhook missing": (("webhook",), None),
            "webhook header lost": (("webhook", "header_keys"), []),
            "webhook re-enabled": (("webhook", "enabled"), True),
            "webhook URL changed": (("webhook", "url"), "https://example.org/"),
            "auth no longer required": (("auth", "requiresAuth"), False),
            "anonymous access allowed": (("auth", "unauthenticated"), "200"),
            "API token rejected": (("auth", "api_token"), "401"),
            "password rejected": (("auth", "password"), "401"),
        }
        for label, (path, value) in cases.items():
            with self.subTest(case=label):
                result = self.check(mutated(path, value))
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn("::error::settings:", result.stdout)

    def test_unparseable_snapshot_fails(self) -> None:
        self.assertEqual(self.check("not json").returncode, 1)


BASELINE_MANIFEST = {
    ".encryption.key": "a" * 64,
    ".env": "b" * 64,
    "metrics.db": "c" * 64,
    "nodes.enc": "d" * 64,
    "system.json": "e" * 64,
    "webhooks.enc": "f" * 64,
}


class DataDirComparisonTest(unittest.TestCase):
    def compare(self, current: dict[str, str]) -> subprocess.CompletedProcess:
        def manifest(entries: dict[str, str]) -> str:
            return "".join(f"{path}\t{digest}\n" for path, digest in sorted(entries.items()))

        with tempfile.TemporaryDirectory() as tmp:
            baseline_path = Path(tmp) / "baseline.tsv"
            current_path = Path(tmp) / "current.tsv"
            baseline_path.write_text(manifest(BASELINE_MANIFEST), encoding="utf-8")
            current_path.write_text(manifest(current), encoding="utf-8")
            return subprocess.run(
                ["bash", str(HARNESS_PATH), "--compare-datadir", str(baseline_path), str(current_path)],
                capture_output=True, text=True, check=False,
                env={"PATH": "/usr/bin:/bin:/usr/local/bin:/opt/homebrew/bin"},
            )

    def test_rewritten_plain_files_and_new_files_pass(self) -> None:
        current = dict(BASELINE_MANIFEST, **{"system.json": "1" * 64, "metrics.db": "2" * 64,
                                              "new-store.db": "3" * 64})
        result = self.compare(current)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_missing_files_and_rewritten_secret_stores_fail(self) -> None:
        cases = {
            "file deleted": {k: v for k, v in BASELINE_MANIFEST.items() if k != "metrics.db"},
            "encryption key replaced": dict(BASELINE_MANIFEST, **{".encryption.key": "0" * 64}),
            "node secrets rewritten": dict(BASELINE_MANIFEST, **{"nodes.enc": "0" * 64}),
            "webhook secrets rewritten": dict(BASELINE_MANIFEST, **{"webhooks.enc": "0" * 64}),
            "data dir emptied": {},
        }
        for label, current in cases.items():
            with self.subTest(case=label):
                result = self.compare(current)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn("::error::", result.stdout)


class WorkflowContractTest(unittest.TestCase):
    workflow = WORKFLOW_PATH.read_text(encoding="utf-8")

    def test_triggers_are_dispatch_and_daily_schedule_only(self) -> None:
        self.assertIn("workflow_dispatch:", self.workflow)
        self.assertIn("from_version:", self.workflow)
        self.assertIn("to_version:", self.workflow)
        self.assertRegex(self.workflow, r"(?m)^  schedule:\n    # [^\n]*\n(?:    #[^\n]*\n)*    - cron: '23 13 \* \* \*'$")
        for trigger in ("pull_request", "push:", "workflow_run", "workflow_call", "release:"):
            self.assertNotIn(trigger, self.workflow)

    def test_read_only_hosted_and_report_only(self) -> None:
        self.assertRegex(self.workflow, r"(?m)^permissions:\n  contents: read$")
        self.assertNotIn(": write", self.workflow)
        self.assertNotIn("secrets.", self.workflow)
        self.assertNotIn("self-hosted", self.workflow)
        self.assertIn("runs-on: ubuntu-24.04", self.workflow)
        self.assertIn("persist-credentials: false", self.workflow)
        self.assertNotIn("upload-artifact", self.workflow)

    def test_dispatch_inputs_reach_shell_only_through_env(self) -> None:
        for line in self.workflow.splitlines():
            if "${{" in line and ("inputs." in line or "steps." in line):
                self.assertRegex(line.strip(), r"^[A-Z_]+: \$\{\{ [a-z_.]+ \}\}$")

    def test_no_release_workflow_depends_on_the_rehearsal(self) -> None:
        for path in (REPO_ROOT / ".github" / "workflows").glob("*.yml"):
            if path == WORKFLOW_PATH:
                continue
            with self.subTest(workflow=path.name):
                self.assertNotIn("release-lifecycle-rehearsal", path.read_text(encoding="utf-8"))


def verify_browser_recovery_controls(controls_path: Path) -> None:
    """Feed exact Chromium receipts into the real guard in a jq-equipped proof VM.

    The guard's permitted process is a local sentinel, not native recovery.
    Source proof binds both this source and the browser input's exact hash.
    """
    controls = json.loads(controls_path.read_text(encoding="utf-8"))
    assert controls["schema_version"] == 1
    assert controls["scope"] == "offline adversarial observer controls only"
    assert controls["client_version"] == "1.56.1"
    fixed = {(kind, "upgrade") for kind in ("success", "anonymous-probes", "restart-gap",
                                           "wrong-preview", "silent-stream", "stuck-modal")}
    fixed.add(("success", "recovery"))
    fixed.update((f"cleanup-{status}", "upgrade") for status in (401, 403))
    for endpoint in ("navigation", "authenticated-navigation", "login", "initial-version",
                     "authenticated-api", "check", "apply", "stream", "version", "health", "read"):
        fixed.update((f"{endpoint}-{status}", "upgrade") for status in (401, 403))
    results = controls["results"]
    assert len(results) == len(fixed) == 31
    assert {(row["fixture"], row["mode"]) for row in results} == fixed
    source = HARNESS_PATH.read_text(encoding="utf-8")
    function = "run_browser_journey() {" + source.split("run_browser_journey() {", 1)[1].split("\n}\n", 1)[0] + "\n}"
    observed = []
    for row in results:
        kind, mode, receipt = row["fixture"], row["mode"], row["receipt"]
        refused = re.search(r"-(401|403)$", kind) is not None
        assert receipt["mode"] == mode and receipt["context_closed"] is True
        assert receipt["access_refused"] is refused
        assert row["requests_after_denial"] == 0
        if refused:
            assert receipt["status"] == "failed" and row["denials"] == 1
            assert receipt["access_refusals"][0]["status"] == int(kind[-3:])
        if mode != "upgrade":
            continue
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            (work / "state").mkdir()
            (work / "state/browser-upgrade.json").write_text(json.dumps(receipt), encoding="utf-8")
            started = work / "process-started"
            sentinel = work / "process-sentinel.sh"
            sentinel.write_text("#!/bin/sh\nprintf started > " + json.dumps(str(started)) + "\n", encoding="utf-8")
            sentinel.chmod(0o700)
            result = subprocess.run(["bash", "-c", "set -euo pipefail\n" + function
                + "\nrun_browser_journey recovery v6.4.5\n"], capture_output=True, text=True, timeout=5,
                env={"PATH": "/usr/local/bin:/usr/bin:/bin", "WORK_DIR": tmp, "ROOT_DIR": str(REPO_ROOT),
                     "SEED_ADMIN_USER": "synthetic", "ADMIN_PASSWORD": "synthetic", "PULSE_REHEARSAL_NODE": str(sentinel),
                     "PULSE_REHEARSAL_BROWSER_ORIGIN": "https://ephemeral.tawny-powan.ts.net",
                     "FROM_TAG": "v6.4.5", "TO_TAG": "v6.4.6-rc.1"})
            assert result.returncode == (1 if refused else 0) and result.stderr == ""
            assert started.exists() is (not refused)
            auth_prepared = (work / "state/browser-auth.json").exists()
            assert auth_prepared is (not refused)
            if refused:
                assert json.loads((work / "state/browser-recovery.json").read_text()) == {
                    "status": "not-executed", "reason": "stopped-access-no-reauthentication"}
            observed.append({"fixture": kind, "guard_exit": result.returncode,
                             "browser_process_started": started.exists(), "auth_prepared": auth_prepared})
    print(json.dumps({"scope": "actual shell guard with bound offline Chromium receipts, not native recovery",
                      "browser_controls": len(results), "executed_recovery_guards": len(observed), "results": observed}))


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--browser-controls":
        try:
            verify_browser_recovery_controls(Path(sys.argv[2]))
        except Exception:
            print("Bound browser receipts failed recovery controls; no installed proof.", file=sys.stderr)
            sys.exit(1)
    else:
        unittest.main()
