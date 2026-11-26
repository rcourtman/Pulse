#!/usr/bin/env python3
"""Contract tests for the shadow release lifecycle rehearsal."""

from __future__ import annotations

import importlib.util
import contextlib
import io
import json
import os
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path
from unittest.mock import patch


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


class DiscoveryTest(unittest.TestCase):
    stream_error = "stream error: stream ID 1; CANCEL; received from peer\n"
    gateway_timeout = "gh: We couldn't respond to your request in time. (HTTP 504)\n"
    inventory_command = ["gh", "api", "--method", "GET", "--paginate", "--slurp",
                         "repos/rcourtman/Pulse/releases?per_page=20"]
    latest_command = ["gh", "api", "--method", "GET", "repos/rcourtman/Pulse/releases/latest"]
    pages = [[release("v6.5.0"), release("v6.6.0")], [release("v6.7.0-beta.1")]]

    @staticmethod
    def result(status=0, stdout="", stderr=""):
        return subprocess.CompletedProcess([], status, stdout=stdout, stderr=stderr)

    def test_recovered_inventory_uses_only_complete_fresh_pages(self):
        results = [
            self.result(1, json.dumps([[release("v99.0.0")]]), self.stream_error),
            self.result(stdout=json.dumps(self.pages)),
            self.result(stdout=json.dumps(release("v6.6.0"))),
        ]
        errors = io.StringIO()
        with patch.object(versions.subprocess, "run", side_effect=results) as command, \
                patch.object(versions.time, "sleep") as sleep, contextlib.redirect_stderr(errors):
            releases, latest = versions.discover("rcourtman/Pulse")
        self.assertEqual([self.inventory_command, self.inventory_command, self.latest_command],
                         [call.args[0] for call in command.call_args_list])
        self.assertTrue(all(call.kwargs["timeout"] == 60 for call in command.call_args_list))
        sleep.assert_called_once_with(2)
        self.assertIn(self.stream_error, errors.getvalue())
        self.assertEqual(versions.resolve(releases, latest).to_tag, "v6.7.0-beta.1")

    def test_latest_read_retries_without_reloading_inventory(self):
        results = [self.result(stdout=json.dumps(self.pages)),
                   self.result(1, "partial latest", self.stream_error),
                   self.result(stdout=json.dumps(release("v6.6.0")))]
        with patch.object(versions.subprocess, "run", side_effect=results) as command, \
                patch.object(versions.time, "sleep") as sleep, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(versions.discover("rcourtman/Pulse")[1], "v6.6.0")
        self.assertEqual([self.inventory_command, self.latest_command, self.latest_command],
                         [call.args[0] for call in command.call_args_list])
        sleep.assert_called_once_with(2)

    def test_retry_budget_exhaustion_emits_no_pair(self):
        output, errors = io.StringIO(), io.StringIO()
        with patch.object(versions.subprocess, "run", return_value=self.result(1, "[]", self.stream_error)) as command, \
                patch.object(versions.time, "sleep") as sleep, \
                contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
            self.assertEqual(versions.main(["--repository", "rcourtman/Pulse"]), 1)
        self.assertEqual(command.call_count, 3)
        self.assertEqual([call.args[0] for call in sleep.call_args_list], [2, 4])
        self.assertEqual(errors.getvalue().count(self.stream_error), 3)
        self.assertIn("attempt 3/3", errors.getvalue())
        self.assertEqual(output.getvalue(), "")

    def test_access_unknown_and_mixed_errors_never_retry(self):
        for detail in ("HTTP 401", "HTTP 403", "HTTP 404", "HTTP 429", "HTTP 503",
                       "capability refused", "connection refused", "", "unavailable",
                       self.stream_error.replace("CANCEL", "REFUSED_STREAM")):
            for error in (detail, self.stream_error + detail + "\n"):
                with self.subTest(error=error), \
                        patch.object(versions.subprocess, "run", return_value=self.result(1, "[]", error)) as command, \
                        patch.object(versions.time, "sleep") as sleep, contextlib.redirect_stderr(io.StringIO()):
                    with self.assertRaises(versions.ResolutionError):
                        versions.discover("rcourtman/Pulse")
                    self.assertEqual(command.call_count, 1)
                    sleep.assert_not_called()

    def test_signal_cancellation_and_other_exits_never_retry(self):
        for status in (-15, -2, 2, 124, 130, 137, 143):
            with self.subTest(status=status), \
                    patch.object(versions.subprocess, "run", return_value=self.result(status, "[]", self.stream_error)) as command, \
                    patch.object(versions.time, "sleep") as sleep, contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaisesRegex(versions.ResolutionError, f"exit {status}"):
                    versions.discover("rcourtman/Pulse")
                self.assertEqual(command.call_count, 1)
                sleep.assert_not_called()

    def test_timeout_never_retries(self):
        with patch.object(versions.subprocess, "run", side_effect=subprocess.TimeoutExpired("gh", 60)) as command, \
                patch.object(versions.time, "sleep") as sleep:
            with self.assertRaisesRegex(versions.ResolutionError, "timed out; not retrying"):
                versions.discover("rcourtman/Pulse")
        self.assertEqual(command.call_count, 1)
        sleep.assert_not_called()

    def test_malformed_inventory_stops_before_latest_read(self):
        for payload in ("not json", "{}", "[]", '[{"tag_name":"v6.6.0"}]', '[[],{}]', '[[null]]'):
            with self.subTest(payload=payload), \
                    patch.object(versions.subprocess, "run", return_value=self.result(stdout=payload)) as command, \
                    patch.object(versions.time, "sleep") as sleep:
                with self.assertRaises((versions.ResolutionError, json.JSONDecodeError)):
                    versions.discover("rcourtman/Pulse")
                self.assertEqual(command.call_count, 1)
                sleep.assert_not_called()

    def test_malformed_latest_stops_without_retry(self):
        for payload in ("not json", "[]", "{}", '{"tag_name":null}'):
            with self.subTest(payload=payload), \
                    patch.object(versions.subprocess, "run", side_effect=[
                        self.result(stdout=json.dumps(self.pages)), self.result(stdout=payload)]) as command, \
                    patch.object(versions.time, "sleep") as sleep:
                with self.assertRaises((versions.ResolutionError, json.JSONDecodeError)):
                    versions.discover("rcourtman/Pulse")
                self.assertEqual(command.call_count, 2)
                sleep.assert_not_called()

    def test_invalid_repository_and_live_latest_override_do_not_start_reads(self):
        with patch.object(versions.subprocess, "run") as command, \
                contextlib.redirect_stderr(io.StringIO()):
            for repository in ("../Pulse", "rcourtman/Pulse/other", "https://example.com", "-H", "rcourtman/Pulse\n"):
                with self.subTest(repository=repository), self.assertRaises(versions.ResolutionError):
                    versions.discover(repository)
            with self.assertRaises(SystemExit) as error:
                versions.main(["--repository", "rcourtman/Pulse", "--latest-tag", "v6.5.0"])
            self.assertEqual(error.exception.code, 2)
        command.assert_not_called()

    def workflow_resolution(self, outcomes, workflow=None, *, inventory_page_limit=None):
        # Execute the actual workflow shell and Python CLI, replacing only gh
        # with a process fixture. No installation, network or token is used.
        if workflow is None:
            workflow = WORKFLOW_PATH.read_text(encoding="utf-8")
        step = workflow.split("      - name: Resolve published release pair\n", 1)[1].split("\n      - name:", 1)[0]
        body = textwrap.dedent(step.split("        run: |\n", 1)[1])
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "outcomes.json").write_text(json.dumps({
                "responses": outcomes,
                "inventory_page_limit": inventory_page_limit,
                "gateway_timeout": self.gateway_timeout,
            }))
            gh = root / "gh"
            gh.write_text(f"#!{sys.executable}\n" + '''import json, os, pathlib, sys
root = pathlib.Path(os.environ['DISCOVERY_FIXTURE'])
with (root / 'calls.jsonl').open('a') as out:
    out.write(json.dumps(sys.argv[1:]) + '\\n')
count = len((root / 'calls.jsonl').read_text().splitlines())
fixture = json.loads((root / 'outcomes.json').read_text())
responses = fixture['responses']
status, output, error = responses[min(count - 1, len(responses) - 1)]
limit = fixture['inventory_page_limit']
endpoint, separator, page_size = sys.argv[-1].partition('?per_page=')
if limit is not None and endpoint == 'repos/rcourtman/Pulse/releases':
    if not separator or int(page_size) > limit:
        status, output, error = 1, '', fixture['gateway_timeout']
print(output, end='', file=sys.stdout)
print(error, end='', file=sys.stderr)
sys.exit(status)
''')
            gh.chmod(0o755)
            output, summary = root / "outputs", root / "summary"
            result = subprocess.run(["bash", "-c", body], cwd=REPO_ROOT, capture_output=True,
                                    text=True, timeout=15, env={**os.environ,
                                        "PATH": str(root) + os.pathsep + os.environ["PATH"],
                                        "DISCOVERY_FIXTURE": str(root), "GH_TOKEN": "synthetic",
                                        "REPOSITORY": "rcourtman/Pulse", "FROM_INPUT": "", "TO_INPUT": "",
                                        "GITHUB_OUTPUT": str(output), "GITHUB_STEP_SUMMARY": str(summary)})
            calls = [json.loads(line) for line in (root / "calls.jsonl").read_text().splitlines()]
            return (result, calls, output.read_text() if output.exists() else "",
                    summary.read_text() if summary.exists() else "")

    def test_workflow_recovers_only_metadata_and_writes_real_pair_outputs(self):
        result, calls, output, summary = self.workflow_resolution([
            (1, json.dumps([[release("v99.0.0")]]), self.stream_error),
            (0, json.dumps(self.pages), ""), (0, json.dumps(release("v6.6.0")), ""),
        ])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual([self.inventory_command[1:], self.inventory_command[1:], self.latest_command[1:]], calls)
        self.assertRegex(output, r"from_tag<<[^\n]+\nv6\.6\.0\n")
        self.assertRegex(output, r"to_tag<<[^\n]+\nv6\.7\.0-beta\.1\n")
        self.assertNotIn("v99.0.0", output + summary)
        self.assertIn("v6.7.0-beta.1", summary)
        self.assertIn(self.stream_error, result.stderr)

    def test_workflow_uses_smaller_pages_without_losing_previous_stable(self):
        # Model a provider that cannot return wide release pages. The old
        # stable remains eligible even when it only occurs on a later page.
        pages = [[release("v6.9.0"), release("helm-chart-6.9.0"),
                  release("v6.10.0-beta.1", draft=True)],
                 [release("v6.9.0-rc.4")], [release("v6.7.0"), release("v6.8.0")]]
        result, calls, output, summary = self.workflow_resolution([
            (0, json.dumps(pages), ""), (0, json.dumps(release("v6.9.0")), ""),
        ], inventory_page_limit=20)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [self.inventory_command[1:], self.latest_command[1:]])
        self.assertRegex(output, r"from_tag<<[^\n]+\nv6\.8\.0\n")
        self.assertRegex(output, r"to_tag<<[^\n]+\nv6\.9\.0\n")
        self.assertIn("previous stable -> latest stable", summary)

    def test_workflow_names_which_read_failed_without_retrying_504(self):
        inventory = (0, json.dumps(self.pages), "")
        timeout = (1, "", self.gateway_timeout)
        for outcomes, label, expected_calls in (
            ([timeout], "release inventory", [self.inventory_command[1:]]),
            ([inventory, timeout], "latest release", [self.inventory_command[1:], self.latest_command[1:]]),
        ):
            with self.subTest(read=label):
                result, calls, output, summary = self.workflow_resolution(outcomes)
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertIn(self.gateway_timeout, result.stderr)
                self.assertIn(f"GitHub {label} metadata read failed (exit 1, attempt 1/3)", result.stderr)
                self.assertEqual(calls, expected_calls)
                self.assertEqual(output + summary, "")

    def test_workflow_refusal_and_malformed_metadata_emit_no_outputs(self):
        for outcome in ((1, "[]", "gh: Forbidden (HTTP 403)\n"), (0, "not json", "")):
            with self.subTest(outcome=outcome):
                result, calls, output, summary = self.workflow_resolution([outcome])
                self.assertEqual(result.returncode, 1, result.stderr)
                self.assertEqual(calls, [self.inventory_command[1:]])
                self.assertEqual(output + summary, "")


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


class ShadowContainerCleanupTest(unittest.TestCase):
    container_id = "a" * 64
    removal = f"rm --force {container_id}"
    lookup = f"ps --all --no-trunc --quiet --filter id={container_id}"

    def run_cleanup(self, *, engine="docker", cid=container_id, start_exit=0, phase_exit=0,
                    removal_exit=0, lookup_exit=0, remaining=""):
        # Execute the actual launch and EXIT trap, skipping only the signed
        # download and application phases. No Docker/Podman service is used.
        source, separator, tail = HARNESS_PATH.read_text(encoding="utf-8").rpartition("\nmain\n")
        self.assertTrue(separator)
        self.assertEqual(tail, "")
        engine_fixture = textwrap.dedent(r'''
            docker() { fake_engine "$@"; }
            podman() { fake_engine "$@"; }
            print_diagnostics() { :; }
            fake_engine() {
                case "$1" in
                    run)
                        while [[ $# -gt 1 ]]; do
                            if [[ "$1" == --cidfile && -n "$FIXTURE_CID" ]]; then
                                printf '%s\n' "$FIXTURE_CID" > "$2"
                                break
                            fi
                            shift
                        done
                        return "$START_EXIT"
                        ;;
                    inspect) printf 'true\n' ;;
                    exec) if [[ "${!#}" == is-system-running ]]; then printf 'running\n'; fi ;;
                    rm)
                        printf '%s\n' "$*" >> "$CALLS"
                        return "$REMOVAL_EXIT"
                        ;;
                    ps)
                        printf '%s\n' "$*" >> "$CALLS"
                        printf '%s' "$REMAINING"
                        return "$LOOKUP_EXIT"
                        ;;
                    *) return 97 ;;
                esac
            }
            if ! start_container; then exit 3; fi
            printf 'Application phases reached (fixture).\n'
            exit "$PHASE_EXIT"
        ''')
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            script = root / "rehearsal.sh"
            script.write_text(source + "\n" + engine_fixture, encoding="utf-8")
            calls, summary = root / "calls", root / "summary"
            result = subprocess.run(
                ["bash", str(script), "--from", "v6.8.0", "--to", "v6.9.0"],
                capture_output=True, text=True, timeout=10,
                env={"PATH": os.environ["PATH"], "PULSE_REHEARSAL_ENGINE": engine,
                     "PULSE_REHEARSAL_WORKDIR": str(root), "GITHUB_STEP_SUMMARY": str(summary),
                     "FIXTURE_CID": cid, "START_EXIT": str(start_exit), "PHASE_EXIT": str(phase_exit),
                     "CALLS": str(calls), "REMAINING": remaining,
                     "REMOVAL_EXIT": str(removal_exit), "LOOKUP_EXIT": str(lookup_exit)},
            )
            return (result, calls.read_text().splitlines() if calls.exists() else [],
                    summary.read_text() if summary.exists() else "")

    def test_reports_verified_absence_only_after_exact_id_readback(self):
        for engine in ("docker", "podman"):
            with self.subTest(engine=engine):
                result, calls, summary = self.run_cleanup(engine=engine)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(calls, [self.removal, self.lookup])
                self.assertIn(f"verified absent ({self.container_id})", summary)

    def test_removal_error_never_passes_or_retries_and_keeps_an_earlier_failure(self):
        for phase_exit, expected_exit in ((0, 1), (7, 7)):
            with self.subTest(phase_exit=phase_exit):
                result, calls, summary = self.run_cleanup(phase_exit=phase_exit, removal_exit=42)
                self.assertIn("Application phases reached", result.stdout)
                self.assertEqual(result.returncode, expected_exit, result.stdout + result.stderr)
                self.assertEqual(calls, [self.removal])
                self.assertIn("unverified; container removal failed (exit 42", summary)

    def test_unconfirmed_absence_fails_even_after_remove_returned_zero(self):
        for lookup_exit, remaining, message in (
            (0, self.container_id, "container lookup still returned a result"),
            (5, "", "container lookup failed (exit 5"),
        ):
            with self.subTest(lookup_exit=lookup_exit):
                result, calls, summary = self.run_cleanup(lookup_exit=lookup_exit, remaining=remaining)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertEqual(calls, [self.removal, self.lookup])
                self.assertIn(f"unverified; {message}", summary)

    def test_failed_start_cleans_up_only_its_recorded_container(self):
        result, calls, summary = self.run_cleanup(start_exit=125)
        self.assertEqual(result.returncode, 3, result.stdout + result.stderr)
        self.assertNotIn("Application phases reached", result.stdout)
        self.assertIn("(exit 125)", result.stdout)
        self.assertEqual(calls, [self.removal, self.lookup])
        self.assertIn("verified absent", summary)

    def test_unknown_container_identity_never_falls_back_to_the_name(self):
        for start_exit, cid in ((125, ""), (0, "disposable-fixture")):
            with self.subTest(start_exit=start_exit):
                result, calls, summary = self.run_cleanup(start_exit=start_exit, cid=cid)
                self.assertEqual(result.returncode, 3, result.stdout + result.stderr)
                self.assertEqual(calls, [])
                self.assertIn("no container ID recorded by this launch. No removal attempted", summary)


class RunningExecutableReadbackTest(unittest.TestCase):
    def run_systemd_fixture(self, tail: str, *, failed_units: str = "", failed_units_exit: int = 0,
                            baseline: str = "", updater_properties: str = (
                                "LoadState=loaded\nExecMainStartTimestampMonotonic=0\n"
                            ), updater_exit: int = 0, hosted: bool = False,
                            matching: bool = True, unavailable: bool = False,
                            after_pid: int = 123, after_started: int = 100) -> subprocess.CompletedProcess:
        source = HARNESS_PATH.read_text(encoding="utf-8")
        names = ("read_container_id", "read_failed_units", "record_failed_unit_baseline", "start_container",
                 "read_pulse_runtime_identity", "read_unattended_update_start", "assert_runtime")
        functions = "\n".join(
            f"{name}() {{" + source.split(f"{name}() {{", 1)[1].split("\n}\n", 1)[0] + "\n}"
            for name in names if f"{name}() {{" in source
        )
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            installed = root / "installed"
            installed.write_bytes(b"same release fixture bytes")
            running = root / "running"
            if not unavailable:
                if matching:
                    os.link(installed, running)
                else:
                    running.write_bytes(installed.read_bytes())
            program = textwrap.dedent(r'''
                set -euo pipefail
                API=http://unused.test
                HOSTED_BROWSER="$FIXTURE_HOSTED"
                BASELINE_FAILED_UNITS="$FIXTURE_BASELINE"
                ENGINE=fake_engine
                CONTAINER_NAME=fixture
                CONTAINER_TOLERATED_UNITS="systemd-hostnamed.service systemd-timedated.service systemd-localed.service"
                WORK_DIR="$FIXTURE"
                SYSTEMD_IMAGE=unused
                CURRENT_FAILURE=""
                note_failure() { printf '%s\n' "$*" >&2; CURRENT_FAILURE=failed; }
                log() { printf '%s\n' "$*"; }
                fake_engine() {
                    case "$1" in
                        run)
                            shift
                            while [[ "$1" != --cidfile ]]; do shift; done
                            printf '%064d\n' 1 > "$2"
                            ;;
                        inspect) printf 'true\n' ;;
                        exec) printf 'running\n' ;;
                        *) return 97 ;;
                    esac
                }
                cexec() {
                    local program="$1"; shift
                    case "$program" in
                        '/opt/pulse/bin/pulse --version '*) printf 'v6.8.0\n' ;;
                        'mkdir -p /etc/systemd/'* | 'export DEBIAN_FRONTEND='*)
                            printf 'guest setup reached\n' ;;
                        *) env "$@" bash -c "$program" ;;
                    esac
                }
                systemctl() {
                    case "$1" in
                        is-active) printf 'active\n' ;;
                        is-enabled) printf 'enabled\n' ;;
                        list-units) printf '%s' "$FAILED_UNITS"; return "$FAILED_UNITS_EXIT" ;;
                        show)
                            case "$*" in
                                *Environment*) printf 'PULSE_TELEMETRY=false\n'; return ;;
                                *pulse-update.service)
                                    if [[ " $* " == *" --value "* ]]; then
                                        sed -n 's/^ExecMainStartTimestampMonotonic=//p' <<<"$UPDATER_PROPERTIES"
                                    else
                                        printf '%s' "$UPDATER_PROPERTIES"
                                    fi
                                    return "$UPDATER_EXIT" ;;
                            esac
                            if [[ -f "$FIXTURE/version-read" ]]; then
                                printf 'MainPID=%s\nExecMainStartTimestampMonotonic=%s\n' "$AFTER_PID" "$AFTER_STARTED"
                            else
                                printf 'MainPID=123\nExecMainStartTimestampMonotonic=100\n'
                            fi
                            printf 'ActiveState=active\n'
                            ;;
                        *) return 97 ;;
                    esac
                }
                stat() {
                    case "${!#}" in
                        /opt/pulse/bin/pulse) command stat -Lc '%d:%i' -- "$FIXTURE/installed" ;;
                        /proc/123/exe|/proc/124/exe) command stat -Lc '%d:%i' -- "$FIXTURE/running" ;;
                        *) return 97 ;;
                    esac
                }
                curl() {
                    case "${!#}" in
                        */api/health) printf '{"status":"healthy"}\n' ;;
                        */api/version)
                            : > "$FIXTURE/version-read"
                            printf '{"version":"6.8.0"}\n'
                            ;;
                        *) return 97 ;;
                    esac
                }
                export -f systemctl stat curl
            ''')
            return subprocess.run(
                ["bash", "-c", program + functions + "\n" + tail],
                capture_output=True, text=True, timeout=15,
                env={"PATH": os.environ["PATH"], "LC_ALL": "C", "FIXTURE": str(root),
                     "AFTER_PID": str(after_pid), "AFTER_STARTED": str(after_started),
                     "FIXTURE_HOSTED": str(hosted).lower(), "FIXTURE_BASELINE": baseline,
                     "FAILED_UNITS": failed_units, "FAILED_UNITS_EXIT": str(failed_units_exit),
                     "UPDATER_PROPERTIES": updater_properties, "UPDATER_EXIT": str(updater_exit)},
            )

    def check_runtime(self, **kwargs) -> subprocess.CompletedProcess:
        return self.run_systemd_fixture(
            'assert_runtime v6.8.0\n'
            'printf "%s / %s\n" "$PHASE_VERSION" "$PHASE_HEALTH"\n'
            'printf "%s\n" "$PHASE_UNITS"\n'
            '[[ -z "$CURRENT_FAILURE" ]]\n', **kwargs,
        )

    def test_requires_failed_unit_baseline_before_guest_setup(self) -> None:
        for hosted in (False, True):
            for units in ("", "older.service loaded failed failed Synthetic\n"):
                with self.subTest(hosted=hosted, partial=bool(units)):
                    result = self.run_systemd_fixture(
                        'if start_container; then printf "baseline=%s\n" "$BASELINE_FAILED_UNITS"; else exit 1; fi',
                        hosted=hosted, failed_units=units, failed_units_exit=1,
                    )
                    self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                    self.assertIn("systemd failed-unit baseline unavailable", result.stdout)
                    self.assertNotIn("guest setup reached", result.stdout)

    def test_retains_a_read_baseline_before_guest_setup(self) -> None:
        for hosted in (False, True):
            with self.subTest(hosted=hosted):
                result = self.run_systemd_fixture(
                    'if start_container; then printf "baseline=%s\n" "$BASELINE_FAILED_UNITS"; else exit 1; fi',
                    hosted=hosted, failed_units="older.service loaded failed failed Synthetic\n",
                )
                self.assertEqual(0, result.returncode, result.stdout + result.stderr)
                self.assertIn("baseline=older.service\n", result.stdout)
                self.assertIn("guest setup reached", result.stdout)

    def test_failed_unit_read_error_cannot_pass_empty_or_partial_result(self) -> None:
        for units in ("", "older.service loaded failed failed Synthetic\n"):
            with self.subTest(partial=bool(units)):
                result = self.check_runtime(baseline="older.service", failed_units=units, failed_units_exit=1)
                self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                self.assertIn("systemd failed-unit read unavailable", result.stderr)
                self.assertIn("6.8.0 / healthy", result.stdout)

    def test_checks_new_failed_units_without_counting_pre_existing_units(self) -> None:
        result = self.check_runtime(
            baseline="older.service",
            failed_units="older.service loaded failed failed Already failed\npulse-worker.service loaded failed failed New failure\n",
        )
        self.assertEqual(1, result.returncode, result.stdout + result.stderr)
        self.assertIn("systemd units failed during the rehearsal: pulse-worker.service", result.stderr)
        self.assertNotIn("older.service", result.stderr)

    def test_unattended_updater_read_error_cannot_pass_as_not_started(self) -> None:
        for properties, status in (("", 1), ("", 0), ("LoadState=loaded\n", 0),
                                   ("LoadState=loaded\nExecMainStartTimestampMonotonic=0\n", 1),
                                   ("LoadState=loaded\nExecMainStartTimestampMonotonic=unknown\n", 0)):
            with self.subTest(properties=properties, status=status):
                result = self.check_runtime(updater_properties=properties, updater_exit=status)
                self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                self.assertIn("pulse-update.service start read unavailable", result.stderr)
                self.assertNotIn("pulse-update.service ran during the rehearsal", result.stderr)

    def test_checks_explicitly_missing_and_started_unattended_updater(self) -> None:
        for properties in ("LoadState=not-found\n", "LoadState=not-found\nExecMainStartTimestampMonotonic=0\n"):
            with self.subTest(properties=properties):
                missing = self.check_runtime(updater_properties=properties)
                self.assertEqual(0, missing.returncode, missing.stdout + missing.stderr)
                self.assertIn("unattended updater not installed", missing.stdout)
        started = self.check_runtime(updater_properties=(
            "ExecMainStartTimestampMonotonic=523\nLoadState=loaded\n"
        ))
        self.assertEqual(1, started.returncode, started.stdout + started.stderr)
        self.assertIn("pulse-update.service ran during the rehearsal", started.stderr)

    def test_same_running_and_installed_file_passes(self) -> None:
        result = self.check_runtime()
        self.assertEqual(0, result.returncode, result.stderr)
        self.assertIn("6.8.0 / healthy", result.stdout)
        self.assertIn("active/enabled, executable verified", result.stdout)
        self.assertIn("unattended updater not started", result.stdout)

    def test_matching_versions_do_not_hide_missing_or_detached_executables(self) -> None:
        for overrides in ({"matching": False}, {"unavailable": True}, {"after_pid": 0}):
            with self.subTest(**overrides):
                result = self.check_runtime(**overrides)
                self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                self.assertIn("running executable could not be bound", result.stderr)
                self.assertIn("6.8.0 / healthy", result.stdout)
                self.assertIn("active/enabled, executable unverified", result.stdout)

    def test_restarted_unit_invalidates_the_version_readback(self) -> None:
        for overrides in ({"after_pid": 124}, {"after_started": 200}):
            with self.subTest(**overrides):
                result = self.check_runtime(**overrides)
                self.assertEqual(1, result.returncode, result.stdout + result.stderr)
                self.assertIn("pulse.service changed during runtime readback", result.stderr)


class HostedWorkflowCommandFilesTest(unittest.TestCase):
    def test_bound_pair_uses_the_reviewed_collision_safe_output_writer(self) -> None:
        source = (REPO_ROOT / '.github/workflows/qualify-browser-update-release.yml').read_text()
        for field in ('from', 'from_commit', 'to_commit'):
            self.assertIn(f'python3 scripts/write_github_output.py {field} "${field}"', source)
        self.assertNotIn('echo "to_commit=', source)
        node_setup = source.split('uses: actions/setup-node@', 1)[1].split('      - name:', 1)[0]
        self.assertNotRegex(node_setup, r'(?m)^\s*cache:')
        self.assertIn('package-manager-cache: false', source)


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
