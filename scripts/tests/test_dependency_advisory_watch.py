#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "dependency_advisory_watch.py"
WORKFLOW_PATH = ROOT / ".github" / "workflows" / "dependency-advisory-watch.yml"
BUILD_AND_TEST = ROOT / ".github" / "workflows" / "build-and-test.yml"
SPEC = importlib.util.spec_from_file_location("dependency_advisory_watch", SCRIPT)
watch = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(watch)

HEADS = "\n".join((
    "29069d4139ef908862a179e1b0dd05364d895776\trefs/heads/release/5.1",
    "e261cd3eb8c2abed21357721df6506d30e2a0ec8\trefs/heads/release/v6.3.2",
    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\trefs/heads/release/v6.3",
    "b50aeb6a8d9e38ab362b23f4ed15a010f675a90f\trefs/heads/release/v6.4",
    "26476f317a68746fd353967470cd8be3bc6b7670\trefs/heads/release/v6.4.2",
    "5f88b2002d1e1085053bf7bf3969aa61cdb0f637\trefs/heads/release/v6.5",
    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\trefs/heads/release/v6.10",
    "cccccccccccccccccccccccccccccccccccccccc\trefs/heads/release/v6.5-hotfix",
))

FINDING = {
    "name": "dompurify", "severity": "moderate", "isDirect": True, "range": "<3.2.8",
    "via": [{"source": 1, "name": "dompurify", "title": "DOMPurify `bypass` | <x>",
             "url": "https://github.com/advisories/GHSA-p98j-92pf-mc4p", "severity": "moderate"}],
    "fixAvailable": {"name": "dompurify", "version": "3.2.8", "isSemVerMajor": False},
}
TRANSITIVE = {
    "name": "brace-expansion", "severity": "low",
    "via": [{"url": "https://github.com/advisories/GHSA-q2hr-2g5m-vwhr", "title": "ReDoS"}, "minimatch"],
    "fixAvailable": True,
}


def audit_log(*findings: dict) -> str:
    rows = ["npm audit (all) attempt 1/3 (limit 60s, 240s of budget left)",
            "npm audit (all): vulnerabilities present (total=2)",
            "::error::npm audit (all) found vulnerabilities: total=2"]
    rows += ["audit finding " + json.dumps(f, sort_keys=True) for f in findings]
    return "\n".join(rows) + "\n"


class ReleaseLinesTest(unittest.TestCase):
    def test_newest_stable_line_and_newer_plus_main(self) -> None:
        self.assertEqual(watch.release_lines(HEADS, "v6.4.5"),
                         ["main", "release/v6.4", "release/v6.5", "release/v6.10"])

    def test_patch_and_legacy_branches_are_never_lines(self) -> None:
        lines = watch.release_lines(HEADS, "")
        for branch in ("release/5.1", "release/v6.3.2", "release/v6.4.2", "release/v6.5-hotfix"):
            self.assertNotIn(branch, lines)

    def test_unknown_or_prerelease_stable_keeps_every_line(self) -> None:
        for latest in ("", "v6.5.0-rc.1", "garbage"):
            with self.subTest(latest=latest):
                self.assertEqual(watch.release_lines(HEADS, latest),
                                 ["main", "release/v6.3", "release/v6.4", "release/v6.5", "release/v6.10"])

    def test_main_is_audited_even_without_release_lines(self) -> None:
        self.assertEqual(watch.release_lines("", "v6.4.5"), ["main"])

    def test_cli_emits_json_list(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            heads = Path(directory) / "heads"
            heads.write_text(HEADS, encoding="utf-8")
            result = subprocess.run([sys.executable, str(SCRIPT), "lines", "--heads", str(heads),
                                     "--latest-stable", "v6.5.0"], capture_output=True, text=True, check=True)
        self.assertEqual(json.loads(result.stdout), ["main", "release/v6.5", "release/v6.10"])


class SummaryTest(unittest.TestCase):
    def run_summary(self, branch: str, outcome: str, log: str) -> tuple[subprocess.CompletedProcess, str]:
        with tempfile.TemporaryDirectory() as directory:
            log_path = Path(directory) / "audit.log"
            summary = Path(directory) / "summary.md"
            log_path.write_text(log, encoding="utf-8")
            result = subprocess.run([sys.executable, str(SCRIPT), "summary", "--branch", branch,
                                     "--outcome", outcome, "--log", str(log_path), "--summary-file", str(summary)],
                                    capture_output=True, text=True, check=False)
            return result, summary.read_text(encoding="utf-8") if summary.exists() else ""

    def test_advisory_names_branch_advisories_and_fix(self) -> None:
        result, summary = self.run_summary("release/v6.4", "failure", audit_log(FINDING, TRANSITIVE))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("`release/v6.4`", summary)
        self.assertIn("GHSA-p98j-92pf-mc4p", summary)
        self.assertIn("GHSA-q2hr-2g5m-vwhr", summary)
        self.assertIn("npm audit fix --package-lock-only", summary)
        self.assertIn("dependencySecurity.test.ts", summary)
        annotations = [line for line in result.stdout.splitlines() if line.startswith("::error")]
        self.assertEqual(len(annotations), 1)
        self.assertIn("release/v6.4", annotations[0])
        self.assertIn("GHSA-p98j-92pf-mc4p, GHSA-q2hr-2g5m-vwhr", annotations[0])
        self.assertIn("lockfile advisory floor bump", annotations[0])

    def test_registry_text_is_escaped_and_kept_out_of_annotation(self) -> None:
        hostile = dict(FINDING, name="evil\n::warning::x", via=[{"title": "x | <script>", "url": "javascript:"}])
        result, summary = self.run_summary("main", "failure", audit_log(hostile))
        self.assertNotIn("<script>", summary)
        self.assertIn("\\<script\\>", summary)
        self.assertIn("unnamed-package", result.stdout)
        self.assertEqual(sum(line.startswith("::") for line in result.stdout.splitlines()), 1)
        self.assertNotIn("script", result.stdout)

    def test_clean_run_has_no_annotation(self) -> None:
        result, summary = self.run_summary("release/v6.5", "success", "npm audit (all): no vulnerabilities\n")
        self.assertNotIn("::error", result.stdout)
        self.assertIn("No advisories reported.", summary)

    def test_unreachable_endpoint_is_not_called_an_advisory(self) -> None:
        log = "::error::npm audit (all) could not reach the advisory endpoint after 3 attempts\n"
        result, summary = self.run_summary("main", "failure", log)
        self.assertIn("could not be reached for main", result.stdout)
        self.assertNotIn("floor bump", result.stdout)
        self.assertIn("No advisory was observed", summary)

    def test_install_failure_without_audit_is_reported(self) -> None:
        result, _ = self.run_summary("release/v6.5", "skipped", "")
        self.assertIn("failed without an advisory finding", result.stdout)

    def test_unexpected_branch_name_is_refused(self) -> None:
        result, summary = self.run_summary("feature/x;rm", "failure", audit_log(FINDING))
        self.assertEqual(result.returncode, 2)
        self.assertEqual(summary, "")


class WorkflowContractTest(unittest.TestCase):
    text = WORKFLOW_PATH.read_text(encoding="utf-8")
    workflow = yaml.safe_load(text)

    def test_triggers_are_daily_schedule_and_dispatch_only(self) -> None:
        triggers = self.workflow[True]
        self.assertEqual(set(triggers), {"schedule", "workflow_dispatch"})
        self.assertRegex(triggers["schedule"][0]["cron"], r"^\d+ \d+ \* \* \*$")

    def test_read_only_hosted_without_secrets_or_uploads(self) -> None:
        self.assertEqual(self.workflow["permissions"], {"contents": "read"})
        self.assertNotIn(": write", self.text)
        self.assertNotIn("secrets.", self.text)
        self.assertNotIn("upload-artifact", self.text)
        for job in self.workflow["jobs"].values():
            self.assertEqual(job["runs-on"], "ubuntu-24.04")
            for step in job["steps"]:
                if str(step.get("uses", "")).startswith("actions/checkout@"):
                    self.assertIs(step["with"]["persist-credentials"], False)

    def test_matrix_covers_every_listed_branch_without_fail_fast(self) -> None:
        audit = self.workflow["jobs"]["audit"]
        self.assertFalse(audit["strategy"]["fail-fast"])
        self.assertEqual(audit["strategy"]["matrix"]["branch"], "${{ fromJSON(needs.lines.outputs.branches) }}")
        read = next(step for step in audit["steps"] if step.get("name") == "Read the audited branch's lockfile inputs")
        self.assertEqual(read["env"]["BRANCH"], "${{ matrix.branch }}")
        self.assertIn('git fetch --no-tags --depth=1 origin "+refs/heads/${BRANCH}:refs/remotes/audited/head"', read["run"])
        self.assertIn("for file in package.json package-lock.json", read["run"])

    def test_never_checks_out_installs_or_runs_another_branchs_code(self) -> None:
        # A scheduled job runs in the default branch's cache and token scope,
        # so it reads only the audited line's two lockfile inputs as data:
        # no checkout of that branch, no install, no dependency cache.
        audit = self.workflow["jobs"]["audit"]["steps"]
        checkouts = [step for step in audit if str(step.get("uses", "")).startswith("actions/checkout@")]
        self.assertEqual(1, len(checkouts))
        self.assertNotIn("ref", checkouts[0].get("with", {}))
        self.assertNotIn("npm ci", self.text)
        self.assertNotIn("npm install", self.text)
        self.assertNotIn("actions/cache", self.text)
        self.assertNotRegex(self.text, r"(?m)^\s+cache(-dependency-path)?:")
        # setup-node turns npm caching on automatically when package.json
        # declares packageManager, so the absence of `cache:` is not enough.
        setup_node = [step for step in audit if str(step.get("uses", "")).startswith("actions/setup-node@")]
        self.assertEqual(1, len(setup_node))
        self.assertIs(False, setup_node[0].get("with", {}).get("package-manager-cache"))

    def test_audit_matches_the_required_build_and_test_audit(self) -> None:
        build = yaml.safe_load(BUILD_AND_TEST.read_text(encoding="utf-8"))
        frontend = build["jobs"]["frontend"]["steps"]
        audit = self.workflow["jobs"]["audit"]["steps"]

        def step(steps: list[dict], name: str) -> dict:
            return next(item for item in steps if item.get("name") == name)
        for name in ("Set up Node.js",):
            self.assertEqual(step(audit, name)["uses"], step(frontend, name)["uses"])
            self.assertEqual(step(audit, name)["with"]["node-version"], step(frontend, name)["with"]["node-version"])
        required = step(frontend, "Audit complete frontend dependency graph")["run"]
        self.assertIn(required, step(audit, "Audit complete frontend dependency graph")["run"])
        self.assertNotIn("continue-on-error", step(audit, "Audit complete frontend dependency graph"))
        self.assertEqual(step(audit, "Audit complete frontend dependency graph")["env"]["NPM_AUDIT_REQUIRE_RESULT"],
                         "true")

    def test_checkout_pins_match_build_and_test(self) -> None:
        pins = set(re.findall(r"uses: (\S+@[0-9a-f]{40})", BUILD_AND_TEST.read_text(encoding="utf-8")))
        for pin in re.findall(r"uses: (\S+@\S+)", self.text):
            self.assertIn(pin, pins)

    def test_matrix_values_reach_shell_only_through_env(self) -> None:
        for line in self.text.splitlines():
            if "${{" in line and ("matrix." in line or "steps." in line or "github." in line):
                stripped = line.strip()
                if stripped.startswith(("name:", "ref:", "branch:", "branches:")):
                    continue
                self.assertRegex(stripped, r"^[A-Z_]+: \$\{\{ [a-z_.]+ \}\}$")


if __name__ == "__main__":
    unittest.main()
