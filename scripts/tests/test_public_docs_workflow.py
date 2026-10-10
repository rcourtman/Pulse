#!/usr/bin/env python3
"""Docs-only CI must execute every existing safety suite and fail closed.

Exercise the actual workflow commands with disposable test adapters. Real
recipes run separately in the source-proof VM, never against customer services.
"""

from __future__ import annotations

import fnmatch
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/public-docs.yml"
SAFETY_STEP = "Exercise documentation safety and diagnostic recipes"
VERDICT_STEP = "Require every documentation recipe shard"
# Independent witnesses: all twelve formerly covered suites, plus the critical
# privacy/access/recovery checks that the former literal workflow omitted.
REQUIRED_SUITES = (
    "test_production_rollout_docs.py", "test_pmg_docs.py",
    "test_agent_profile_docs.py", "test_pbs_docs.py", "test_audit_recovery_docs.py",
    "test_notification_troubleshooting_docs.py", "test_webhook_verification_docs.py",
    "test_pve_backup_troubleshooting_docs.py", "test_vm_disk_diagnostics.py",
    "test_memory_troubleshooting_docs.py", "test_performance_troubleshooting_docs.py",
    "test_troubleshooting_logs.py", "test_admin_docs.py", "test_api_auth_docs.py",
    "test_authentication_diagnostic_docs.py", "test_update_recovery_docs.py",
    "test_guest_disk_reading_docs.py", "test_guest_network_troubleshooting_docs.py",
    "test_truenas_docs.py", "test_repo_docs_link_drift.py", "test_security_docs.py",
)
LEGACY_SUITES = {
    "test_vm_disk_diagnostics.py", "test_troubleshooting_logs.py",
    "test-retired-trial-acquisition-docs.sh",
}


def documentation_suites() -> list[str]:
    return sorted(path.name for path in (ROOT / "scripts/tests").iterdir()
                  if path.is_file() and (fnmatch.fnmatch(path.name, "test_*docs*.py")
                                         or path.name in LEGACY_SUITES))


def job_body(source: str, name: str) -> str:
    marker = f"\n  {name}:\n"
    if source.count(marker) != 1:
        raise AssertionError(f"expected exactly one {name} job")
    return re.split(r"\n  [\w-]+:\n", source.split(marker, 1)[1], maxsplit=1)[0]


def step_body(source: str, name: str) -> str:
    marker = f"      - name: {name}\n"
    if source.count(marker) != 1:
        raise AssertionError(f"expected exactly one {name} step")
    return source.split(marker, 1)[1].split("\n      - name:", 1)[0]


def safety_command(source: str, shard: int) -> str:
    body = step_body(job_body(source, "recipes"), SAFETY_STEP)
    run = re.findall(r"^        run: (.+)$", body, re.MULTILINE)
    if len(run) != 1:
        raise AssertionError("expected one literal docs-shard command")
    return run[0].replace("${{ matrix.shard }}", str(shard))


def verdict_command(source: str) -> str:
    body = step_body(job_body(source, "check"), VERDICT_STEP)
    marker = "        run: |\n"
    if body.count(marker) != 1:
        raise AssertionError("expected one literal verdict command")
    return textwrap.dedent(body.split(marker, 1)[1]).strip()


class PublicDocsWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.source = WORKFLOW.read_text(encoding="utf-8")

    def test_both_events_cover_all_suite_and_helper_changes(self):
        triggers = self.source.split("\non:\n", 1)[1].split("\npermissions:", 1)[0]
        required = {
            "*.md", "docs/**", "frontend-modern/public/docs/**",
            ".github/ISSUE_TEMPLATE/**", ".github/PULL_REQUEST_TEMPLATE.md",
            "scripts/check_public_docs.py", "scripts/check_docs_mirror.py",
            "scripts/readme_entrypoints_test.py", "scripts/tests/**",
            "scripts/test-vm-disk.sh", ".github/workflows/public-docs.yml",
        }
        for event in ("pull_request", "push"):
            with self.subTest(event=event):
                block = triggers.split(f"  {event}:\n", 1)[1]
                block = re.split(r"\n  [a-z_]+:\n", block, maxsplit=1)[0]
                paths = re.findall(r'^      - "([^"]+)"$', block, re.MULTILINE)
                self.assertTrue(required.issubset(paths), required.difference(paths))
                self.assertEqual(len(paths), len(set(paths)))
        self.assertIn("  push:\n    branches:\n      - main\n", triggers)

    def test_shards_discover_current_suites_without_dependency_setup(self):
        self.assertTrue(set(REQUIRED_SUITES).issubset(documentation_suites()))
        for shard in range(4):
            self.assertEqual(shlex.split(safety_command(self.source, shard)),
                             ["scripts/tests/run.sh", "--docs-shard", str(shard)])
        self.assertNotIn("actions/setup-", self.source)
        self.assertNotRegex(self.source, r"\b(?:npm|pip|go) (?:ci|install|build|test)\b")

    def test_existing_document_admissions_are_preserved(self):
        check = job_body(self.source, "check")
        for name, command in (
            ("Validate public documentation", "python3 scripts/check_public_docs.py"),
            ("Check getting-started safety and current plans", "python3 scripts/readme_entrypoints_test.py"),
            ("Check shipped docs mirror sync", "python3 scripts/check_docs_mirror.py"),
            ("Check documentation safety workflow contract", "python3 scripts/tests/test_public_docs_workflow.py"),
        ):
            with self.subTest(step=name):
                self.assertIn(f"        run: {command}\n", step_body(check, name))

    def test_all_four_isolated_shards_are_mandatory_and_bounded(self):
        recipes = job_body(self.source, "recipes")
        self.assertIn("        shard: [0, 1, 2, 3]\n", recipes)
        self.assertIn("      fail-fast: false\n", recipes)
        self.assertNotRegex(recipes, r"(?m)^\s*(?:if|continue-on-error|exclude):")
        self.assertNotIn("continue-on-error:", self.source)
        for name in ("recipes", "check"):
            job = job_body(self.source, name)
            self.assertIn("    runs-on: ubuntu-24.04\n", job)
            self.assertIn("    timeout-minutes: 10\n", job)
            self.assertIn("          persist-credentials: false\n", job)
        self.assertIn("permissions:\n  contents: read\n", self.source)

    def test_required_check_runs_even_when_shards_fail_or_are_skipped(self):
        check = job_body(self.source, "check")
        self.assertIn("    needs: recipes\n", check)
        self.assertIn("    if: ${{ always() }}\n", check)
        self.assertNotRegex(check.replace("    if: ${{ always() }}\n", ""), r"(?m)^\s*if:")
        self.assertIn("          RECIPE_RESULT: ${{ needs.recipes.result }}\n", check)
        self.assertLess(check.index(VERDICT_STEP), check.index("Validate public documentation"))
        for state in ("success", "failure", "cancelled", "skipped", "", "unknown"):
            with self.subTest(state=state):
                result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", verdict_command(self.source)],
                                        env=dict(os.environ, RECIPE_RESULT=state), text=True,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode == 0, state == "success", result.stdout + result.stderr)

    def run_adapters(self, shard: int, failing: str = ""):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tests = root / "scripts/tests"
            tests.mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/tests/run.sh", tests / "run.sh")
            (tests / "run.sh").chmod(0o700)
            for name in documentation_suites():
                status = 1 if name == failing else 0
                if name.endswith(".sh"):
                    code = f'#!/bin/bash\nprintf "%s\\n" {shlex.quote(name)} >> "$CALLS"\nexit {status}\n'
                else:
                    code = ("import os\nfrom pathlib import Path\n"
                            f"with Path(os.environ['CALLS']).open('a') as log: log.write({name!r} + '\\n')\n"
                            f"raise SystemExit({status})\n")
                (tests / name).write_text(code, encoding="utf-8")
                (tests / name).chmod(0o700)
            calls = root / "calls"
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", safety_command(self.source, shard)],
                                    cwd=root, env=dict(os.environ, CALLS=str(calls)),
                                    text=True, capture_output=True, timeout=15)
            return result, calls.read_text().splitlines() if calls.exists() else []

    def test_actual_workflow_commands_cover_every_suite_once(self):
        seen = []
        for shard in range(4):
            result, calls = self.run_adapters(shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(calls, documentation_suites()[shard::4])
            self.assertIn(f"Summary: {len(calls)}/{len(calls)} passed", result.stdout)
            seen.extend(calls)
        self.assertEqual(sorted(seen), documentation_suites())
        self.assertEqual(len(seen), len(set(seen)))

    def test_each_failed_suite_fails_its_actual_workflow_command(self):
        for index, failing in enumerate(documentation_suites()):
            with self.subTest(failing=failing):
                shard = index % 4
                result, calls = self.run_adapters(shard, failing)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(calls, documentation_suites()[shard::4])
                self.assertIn(f"Summary: {len(calls)-1}/{len(calls)} passed", result.stdout)
                self.assertIn("Failures: 1", result.stdout)


if __name__ == "__main__":
    unittest.main()
