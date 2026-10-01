#!/usr/bin/env python3
"""Execute the workflow's source-selection shell against local Git fixtures."""
from pathlib import Path
import os
import re
import shutil
import subprocess
import tempfile
import textwrap
import unittest

from repo_file_io import REPO_ROOT, strip_local_git_env


WORKFLOW = REPO_ROOT / ".github/workflows/release-dry-run.yml"


def step(name):
    match = re.search(
        rf"(?ms)^      - name: {name}\n.*?^        run: \|\n"
        r"((?:          [^\n]*\n|\n)+)", WORKFLOW.read_text()
    )
    assert match
    return textwrap.dedent(match.group(1))


class RehearsalSourceTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repository"
        self.repo.mkdir()
        self.env = strip_local_git_env(dict(os.environ))
        self.env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL="/dev/null")
        self.git("init", "-b", "main")
        self.git("config", "user.email", "fixture@example.invalid")
        self.git("config", "user.name", "Fixture")
        self.git("config", "core.hooksPath", "/dev/null")
        for name in ("scripts/release_control/control_plane.py",
                     "scripts/release_control/repo_file_io.py",
                     "docs/release-control/control_plane.json"):
            target = self.repo / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(REPO_ROOT / name, target)
        self.version("6.4.3-rc.1")
        self.main = self.git("rev-parse", "HEAD")
        self.git("checkout", "-b", "release/v6.4")
        self.version("6.4.4-beta.1")
        self.release = self.git("rev-parse", "HEAD")
        self.git("checkout", "main")
        # A local read-only fetch exercises real FETCH_HEAD / detached checkout.
        self.git("remote", "add", "origin", str(self.repo))
        self.output = self.root / "output"

    def git(self, *args):
        return subprocess.check_output(
            ["git", *args], cwd=self.repo, env=self.env,
            stderr=subprocess.DEVNULL, text=True
        ).strip()

    def version(self, value):
        (self.repo / "VERSION").write_text(value + "\n")
        self.git("add", ".")
        self.git("commit", "-m", "fixture")

    def select(self, event="schedule", branch="main", sha=None, required="release/v6.4", watchdog=None):
        env = dict(self.env, EVENT_NAME=event, GITHUB_REF_NAME=branch,
                   GITHUB_SHA=sha or self.main, REQUIRED_BRANCH=required,
                   WATCHDOG_MODE=watchdog or ("true" if event == "schedule" else "false"),
                   GITHUB_OUTPUT=str(self.output))
        return subprocess.run(
            ["bash", "-euo", "pipefail", "-c", step("Select exact rehearsal source")],
            cwd=self.repo, env=env, text=True, capture_output=True
        )

    def test_schedule_selects_release_and_its_version_not_main(self):
        result = self.select()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.release)
        self.assertEqual((self.repo / "VERSION").read_text().strip(), "6.4.4-beta.1")
        self.assertIn("tested_sha=" + self.release, self.output.read_text())
        self.assertIn("tested_branch=release/v6.4", self.output.read_text())
        self.assertIn(self.main, result.stdout)
        self.assertNotEqual(self.main, self.release)

    def mode(self, **overrides):
        env = dict(self.env, EVENT_NAME="workflow_dispatch", WATCHDOG_INPUT="true",
                   EXPECTED_WORKFLOW_SHA_INPUT=self.main, GITHUB_SHA=self.main,
                   GITHUB_REF="refs/heads/main", GITHUB_REF_NAME="main",
                   GITHUB_OUTPUT=str(self.output))
        env.update(overrides)
        return subprocess.run(["bash", "-euo", "pipefail", "-c", step("Validate release ref")],
                              cwd=self.repo, env=env, text=True, capture_output=True)

    def test_fixed_watchdog_dispatch_selects_same_source_as_schedule(self):
        admitted = self.mode()
        self.assertEqual(admitted.returncode, 0, admitted.stderr)
        self.assertEqual(self.output.read_text(), "watchdog=true\n")
        result = self.select("workflow_dispatch", watchdog="true")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.release)
        self.assertIn(self.main, result.stdout)
        self.assertIn("tested_sha=" + self.release, self.output.read_text())

    def test_watchdog_rejects_moved_control_wrong_ref_and_unknown_event_before_selection(self):
        for overrides in (
            {"EXPECTED_WORKFLOW_SHA_INPUT": "a" * 40},
            {"EXPECTED_WORKFLOW_SHA_INPUT": "not-a-sha"},
            {"EXPECTED_WORKFLOW_SHA_INPUT": ""},
            {"GITHUB_REF": "refs/heads/release/v6.4"},
            {"GITHUB_REF": "refs/tags/v6.4.5"},
            {"EVENT_NAME": "push"}, {"WATCHDOG_INPUT": "yes"},
        ):
            with self.subTest(overrides=overrides):
                result = self.mode(**overrides)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertFalse(self.output.exists())
                self.assertEqual(self.git("rev-parse", "HEAD"), self.main)

    def test_watchdog_rejects_every_candidate_and_exception_input(self):
        for name in (
            "VERSION_INPUT", "PROMOTED_FROM_TAG_INPUT", "ROLLBACK_VERSION_INPUT",
            "GA_DATE_INPUT", "V5_EOS_DATE_INPUT", "HOTFIX_REASON_INPUT",
            "UNSIGNED_WINDOWS_REASON_INPUT", "NOTE_INPUT", "MOBILE_RELEASE_DECISION",
            "MOBILE_RELEASE_EVIDENCE", "HOTFIX_EXCEPTION_INPUT", "UNSIGNED_WINDOWS_EXCEPTION_INPUT",
        ):
            with self.subTest(name=name):
                self.assertNotEqual(self.mode(**{name: "true"}).returncode, 0)
                self.assertFalse(self.output.exists())
        self.assertEqual(self.mode(HOTFIX_EXCEPTION_INPUT="false",
                                   UNSIGNED_WINDOWS_EXCEPTION_INPUT="false").returncode, 0)

    def test_schedule_and_manual_candidate_envelopes_remain_separate(self):
        result = self.mode(EVENT_NAME="schedule", WATCHDOG_INPUT="", EXPECTED_WORKFLOW_SHA_INPUT="")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.output.read_text(), "watchdog=true\n")
        self.output.unlink()
        for overrides in ({}, {"ROLLBACK_VERSION_INPUT": "v6.4.1", "EXPECTED_WORKFLOW_SHA_INPUT": self.main}):
            with self.subTest(overrides=overrides):
                env = {"WATCHDOG_INPUT": "false", "EXPECTED_WORKFLOW_SHA_INPUT": ""}
                env.update(overrides)
                self.assertNotEqual(self.mode(**env).returncode, 0)
                self.assertFalse(self.output.exists())
        result = self.mode(WATCHDOG_INPUT="false", EXPECTED_WORKFLOW_SHA_INPUT="",
                           ROLLBACK_VERSION_INPUT="v6.4.1")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.output.read_text(), "watchdog=false\n")

    def test_manual_wrong_branch_rejected_without_switching(self):
        result = self.select("workflow_dispatch")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.main)
        self.assertFalse(self.output.exists())

    def test_manual_exact_release_source_preserved(self):
        self.git("checkout", "release/v6.4")
        result = self.select("workflow_dispatch", "release/v6.4", self.release)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.release)

    def test_manual_sha_mismatch_rejected(self):
        self.git("checkout", "release/v6.4")
        self.assertNotEqual(self.select("workflow_dispatch", "release/v6.4").returncode, 0)

    def test_missing_branch_fails_closed(self):
        self.assertNotEqual(self.select(required="release/missing").returncode, 0)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.main)
        self.assertFalse(self.output.exists())

    def test_explicit_watchdog_missing_branch_and_policy_drift_fail_closed(self):
        self.assertNotEqual(self.select("workflow_dispatch", watchdog="true",
                                        required="release/missing").returncode, 0)
        self.assertFalse(self.output.exists())
        self.git("checkout", "release/v6.4")
        self.version("6.6.0-beta.1")
        self.git("checkout", "main")
        self.assertNotEqual(self.select("workflow_dispatch", watchdog="true").returncode, 0)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.main)
        self.assertFalse(self.output.exists())

    def test_selected_version_must_still_belong_to_governed_branch(self):
        self.git("checkout", "release/v6.4")
        self.version("6.6.0-beta.1")
        self.git("checkout", "main")
        self.assertNotEqual(self.select().returncode, 0)
        self.assertEqual(self.git("rev-parse", "HEAD"), self.main)
        self.assertFalse(self.output.exists())

    def test_unknown_event_rejected(self):
        self.assertNotEqual(self.select("push").returncode, 0)

    def test_metadata_keeps_selected_branch_and_manual_rollback_guard(self):
        metadata = step("Resolve rehearsal metadata")
        self.assertIn('if [ "${TESTED_BRANCH}" != "$REQUIRED_BRANCH" ]; then', metadata)
        self.assertIn('if [ "$FILE_VERSION" != "$VERSION" ]; then', metadata)
        self.assertIn('if [ "${WATCHDOG_MODE}" = "true" ] && [ -z "${ROLLBACK_VERSION_INPUT:-}" ]; then', metadata)
        self.assertIn('--derive-rollback-latest-stable', metadata)
        workflow = WORKFLOW.read_text()
        self.assertIn('TESTED_SHA: ${{ needs.dry-run.outputs.tested_sha }}', workflow)
        self.assertIn('Workflow event SHA:', workflow)
        self.assertNotIn('echo "- Source SHA:', workflow)


if __name__ == "__main__":
    unittest.main()
