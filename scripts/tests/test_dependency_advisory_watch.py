#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import json
import os
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
        result, summary = self.run_summary("main", "failure", audit_log(FINDING, TRANSITIVE))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("`main`", summary)
        self.assertIn("GHSA-p98j-92pf-mc4p", summary)
        self.assertIn("GHSA-q2hr-2g5m-vwhr", summary)
        self.assertIn("npm audit fix --package-lock-only", summary)
        self.assertIn("dependencySecurity.test.ts", summary)
        annotations = [line for line in result.stdout.splitlines() if line.startswith("::error")]
        self.assertEqual(len(annotations), 1)
        self.assertIn("main", annotations[0])
        self.assertIn("GHSA-p98j-92pf-mc4p, GHSA-q2hr-2g5m-vwhr", annotations[0])
        self.assertIn("lockfile advisory floor bump", annotations[0])
        self.assertIn("qualify the resulting graph", summary)
        self.assertIn("land both on main", summary)
        self.assertIn("containing release", summary)
        self.assertIn("unchanged graphs only warn", summary)
        self.assertNotIn("block every pull request", summary)
        self.assertNotIn("blocks every PR and release", annotations[0])

    def test_registry_text_is_escaped_and_kept_out_of_annotation(self) -> None:
        hostile = dict(FINDING, name="evil\n::warning::x", via=[{"title": "x | <script>", "url": "javascript:"}])
        result, summary = self.run_summary("main", "failure", audit_log(hostile))
        self.assertNotIn("<script>", summary)
        self.assertIn("\\<script\\>", summary)
        self.assertIn("unnamed-package", result.stdout)
        self.assertEqual(sum(line.startswith("::") for line in result.stdout.splitlines()), 1)
        self.assertNotIn("script", result.stdout)

    def test_clean_run_has_no_annotation(self) -> None:
        result, summary = self.run_summary("main", "success", "npm audit (all): no vulnerabilities\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("::error", result.stdout)
        self.assertIn("No advisories reported.", summary)

    def test_unreachable_endpoint_is_not_called_an_advisory(self) -> None:
        log = "::error::npm audit (all) could not reach the advisory endpoint after 3 attempts\n"
        result, summary = self.run_summary("main", "failure", log)
        self.assertIn("could not be reached for main", result.stdout)
        self.assertNotIn("floor bump", result.stdout)
        self.assertIn("No advisory was observed", summary)

    def test_install_failure_without_audit_is_reported(self) -> None:
        result, _ = self.run_summary("main", "skipped", "")
        self.assertIn("failed without an advisory finding", result.stdout)

    def test_unexpected_branch_name_is_refused(self) -> None:
        result, summary = self.run_summary("feature/x;rm", "failure", audit_log(FINDING))
        self.assertEqual(result.returncode, 2)
        self.assertEqual(summary, "")

    def test_retired_release_branches_cannot_request_backport_guidance(self) -> None:
        for branch in ("release/v6.5", "release/v6.6"):
            with self.subTest(branch=branch):
                result, summary = self.run_summary(branch, "failure", audit_log(FINDING))
                self.assertEqual(result.returncode, 2)
                self.assertEqual(summary, "")
                self.assertEqual(result.stdout, "")


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

    def test_only_main_is_audited_without_release_discovery(self) -> None:
        self.assertEqual(set(self.workflow["jobs"]), {"audit"})
        audit = self.workflow["jobs"]["audit"]
        self.assertNotIn("strategy", audit)
        self.assertNotIn("needs", audit)
        self.assertNotIn("git ls-remote", self.text)
        self.assertNotIn("gh api", self.text)
        self.assertNotIn("GH_TOKEN", self.text)
        read = next(step for step in audit["steps"] if step.get("name") == "Read main's lockfile inputs")
        self.assertNotIn("BRANCH", read["env"])
        self.assertIn("git fetch --no-tags --depth=1 origin '+refs/heads/main:refs/remotes/audited/head'", read["run"])
        self.assertIn("AUDITED_MAIN_SHA=", read["run"])
        self.assertIn("for file in package.json package-lock.json", read["run"])
        summary = next(step for step in audit["steps"] if step.get("name") == "Report advisories and fix")
        self.assertEqual(summary["env"]["BRANCH"], "main")

    def test_real_input_recipe_is_fixed_main_and_never_executes_fetched_code(self) -> None:
        read = next(step for step in self.workflow["jobs"]["audit"]["steps"]
                    if step.get("name") == "Read main's lockfile inputs")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fake_git = root / "git"
            calls = root / "calls.jsonl"
            fake_git.write_text(
                f"#!{sys.executable}\n"
                "import json, os, sys\n"
                "args = sys.argv[1:]\n"
                "with open(os.environ['CALLS'], 'a') as stream: stream.write(json.dumps(args)+'\\n')\n"
                "if args == ['fetch', '--no-tags', '--depth=1', 'origin', '+refs/heads/main:refs/remotes/audited/head']: pass\n"
                "elif args == ['rev-parse', 'refs/remotes/audited/head']: print('a'*40)\n"
                "elif args == ['show', 'refs/remotes/audited/head:frontend-modern/package.json']: print('{}')\n"
                "elif args == ['show', 'refs/remotes/audited/head:frontend-modern/package-lock.json']: print('{\"lockfileVersion\":3}')\n"
                "else: sys.exit(19)\n", encoding="utf-8")
            fake_git.chmod(0o755)
            result = subprocess.run(["bash", "-c", read["run"]], capture_output=True, text=True,
                                    env={**os.environ, "PATH": f"{root}:{os.environ['PATH']}",
                                         "AUDITED": str(root / "audited"), "CALLS": str(calls),
                                         "BRANCH": "release/v6.6"}, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, "AUDITED_MAIN_SHA=" + "a" * 40 + "\n")
            self.assertEqual(json.loads((root / "audited/package.json").read_text()), {})
            self.assertEqual(json.loads((root / "audited/package-lock.json").read_text()), {"lockfileVersion": 3})
            commands = [json.loads(row) for row in calls.read_text().splitlines()]
            self.assertEqual(len(commands), 4)
            self.assertEqual(commands[0][-1], "+refs/heads/main:refs/remotes/audited/head")

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
        frontend = build["jobs"]["frontend-checks"]["steps"]
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

    def test_watch_fails_on_an_advisory_pull_requests_only_warn_on(self) -> None:
        # A pull request that leaves the lockfile alone only warns on an
        # advisory its base already has; this watch is where that advisory
        # must fail. Run the real runner with the watch's own environment.
        audit_step = next(
            item for item in self.workflow["jobs"]["audit"]["steps"]
            if item.get("name") == "Audit complete frontend dependency graph"
        )
        vulnerable = json.dumps({
            "metadata": {"vulnerabilities": {
                "info": 0, "low": 0, "moderate": 0, "high": 1, "critical": 0, "total": 1,
            }},
            "vulnerabilities": {"braces": {"name": "braces", "severity": "high"}},
        })
        with tempfile.TemporaryDirectory() as directory:
            fake_npm = Path(directory) / "npm"
            fake_npm.write_text(f"#!/bin/sh\nprintf '%s\\n' '{vulnerable}'\nexit 1\n", encoding="utf-8")
            fake_npm.chmod(0o755)
            outcomes = {}
            for mode in (audit_step["env"]["NPM_AUDIT_REQUIRE_RESULT"], "false"):
                result = subprocess.run(
                    ["bash", str(ROOT / "scripts" / "npm-audit-retry.sh"), "all"],
                    env={**os.environ, "NPM_AUDIT_CMD": str(fake_npm),
                         "NPM_AUDIT_REQUIRE_RESULT": mode, "NPM_AUDIT_RETRY_DELAY": "0"},
                    text=True, capture_output=True, check=False,
                )
                outcomes[mode] = result
        self.assertEqual(1, outcomes["true"].returncode, outcomes["true"].stdout)
        self.assertIn("::error::", outcomes["true"].stdout)
        self.assertEqual(0, outcomes["false"].returncode, outcomes["false"].stdout)
        self.assertIn("::warning::", outcomes["false"].stdout)

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
