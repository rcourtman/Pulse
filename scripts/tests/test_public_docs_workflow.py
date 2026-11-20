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
import sys
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/public-docs.yml"
SAFETY_STEP = "Exercise documentation safety and diagnostic recipes"
VERDICT_STEP = "Require every documentation recipe shard"
HELP_DIFF_STEP = "Detect in-app Help changes"
HELP_TEST_STEP = "Check in-app Help content and links"
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


def check_step_command(source: str, name: str) -> str:
    body = step_body(job_body(source, "check"), name)
    marker = "        run: |\n"
    if body.count(marker) != 1:
        raise AssertionError(f"expected a literal script for {name}")
    return textwrap.dedent(body.split(marker, 1)[1]).strip()


def workflow_fixture_environment(root: Path, calls: Path, failing: str = "") -> dict[str, str]:
    # The real selector and sequencer still run in Python. For the generated
    # Python suite paths only, the interpreter double records the actual
    # invocation and exits like the synthetic suite, without a Python startup
    # for each suite in each single-failure control. Real recipes do not use it.
    tools = root / "tools"
    tools.mkdir()
    python = tools / "python3"
    python.write_text(
        '#!/bin/sh\n'
        'if [ "$#" -eq 1 ] && [ "${1%/*}" = "$FIXTURE_SUITE_DIR" ] && [ -f "$1" ]; then\n'
        '    case "${1##*/}" in\n'
        '        test_*.py)\n'
        '            suite=${1##*/}\n'
        '            printf "%s\\n" "$suite" >> "$CALLS" || exit 90\n'
        '            [ "$suite" != "$FIXTURE_FAIL_SUITE" ]\n'
        '            exit $?\n'
        '            ;;\n'
        '    esac\n'
        'fi\n'
        f'exec {shlex.quote(sys.executable)} -I -S "$@"\n', encoding="utf-8")
    python.chmod(0o700)
    return dict(os.environ, PATH=f"{tools}{os.pathsep}{os.environ['PATH']}", CALLS=str(calls),
                FIXTURE_SUITE_DIR=str(root / "scripts/tests"), FIXTURE_FAIL_SUITE=failing)


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
            "scripts/select-smoke-test-shard.py", ".github/scripts/docs-test-seconds.txt",
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
        recipes = job_body(self.source, "recipes")
        self.assertNotIn("actions/setup-", recipes)
        self.assertNotRegex(recipes, r"\b(?:npm|pip|go) (?:ci|install|build|test)\b")

    def test_existing_document_admissions_are_preserved(self):
        check = job_body(self.source, "check")
        for name, command in (
            ("Validate public documentation", "python3 scripts/check_public_docs.py"),
            ("Check getting-started safety and current plans", "python3 scripts/readme_entrypoints_test.py"),
            ("Check shipped docs mirror sync", "python3 scripts/check_docs_mirror.py"),
            ("Check documentation safety workflow contract", "python3 scripts/tests/test_public_docs_workflow.py"),
        ):
            with self.subTest(step=name):
                step = step_body(check, name)
                self.assertIn(f"        run: {command}\n", step)
                self.assertNotRegex(step, r"(?m)^        if:")

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
        self.assertEqual(re.findall(r"(?m)^    if: .+$", check), ["    if: ${{ always() }}"])
        self.assertIn("          RECIPE_RESULT: ${{ needs.recipes.result }}\n", check)
        self.assertNotRegex(step_body(check, VERDICT_STEP), r"(?m)^        if:")
        self.assertLess(check.index(VERDICT_STEP), check.index("Validate public documentation"))
        for state in ("success", "failure", "cancelled", "skipped", "", "unknown"):
            with self.subTest(state=state):
                result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c",
                                         check_step_command(self.source, VERDICT_STEP)],
                                        env=dict(os.environ, RECIPE_RESULT=state), text=True,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode == 0, state == "success", result.stdout + result.stderr)

    def test_help_proof_is_part_of_the_required_check(self):
        check = job_body(self.source, "check")
        checkout = step_body(check, "Check out repository")
        self.assertIn("          fetch-depth: 2\n", checkout)
        detector = step_body(check, HELP_DIFF_STEP)
        self.assertIn("        id: help\n", detector)
        self.assertIn("github.event.pull_request.base.sha || github.event.before", detector)
        setup, install, test = (
            "Set up Node.js for Help assertions",
            "Install locked Help test dependencies",
            HELP_TEST_STEP,
        )
        for name in (setup, install, test):
            self.assertIn("        if: steps.help.outputs.required != 'false'\n",
                          step_body(check, name))
        self.assertIn("cache-dependency-path: frontend-modern/package-lock.json",
                      step_body(check, setup))
        self.assertIn("node-version: '24'", step_body(check, setup))
        for name, command in (
            (install, "npm ci --no-audit --no-fund"),
            (test, "npm run test -- src/features/docs/__tests__ src/utils/__tests__/docsLinks.test.ts"),
        ):
            step = step_body(check, name)
            self.assertIn("        working-directory: frontend-modern\n", step)
            self.assertEqual(re.findall(r"^        run: (.+)$", step, re.MULTILINE), [command])
        order = [check.index(name) for name in (
            "Check shipped docs mirror sync", HELP_DIFF_STEP, setup, install, test
        )]
        self.assertEqual(order, sorted(order))

    def test_help_diff_admission_uses_exact_trees_and_fails_safe(self):
        command = check_step_command(self.source, HELP_DIFF_STEP)
        with tempfile.TemporaryDirectory(prefix="help diff ") as temporary:
            root = Path(temporary)
            env = dict(os.environ, GIT_AUTHOR_NAME="Fixture", GIT_AUTHOR_EMAIL="fixture@example.invalid",
                       GIT_COMMITTER_NAME="Fixture", GIT_COMMITTER_EMAIL="fixture@example.invalid")

            def git(*args):
                return subprocess.run(["git", *args], cwd=root, env=env, text=True,
                                      check=True, capture_output=True, timeout=5).stdout.strip()

            def required(base):
                output = root / "job-output"
                output.write_text("", encoding="utf-8")
                result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", command],
                                        cwd=root, env=dict(env, BASE_SHA=base, GITHUB_OUTPUT=str(output)),
                                        capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)
                return output.read_text(encoding="utf-8").splitlines()

            git("init", "-q")
            git("commit", "--allow-empty", "-qm", "base")
            for path, want in (
                ("docs/releases/example.md", False),
                ("docs/release-control/internal.md", False),
                ("scripts/tests/test_fixture_docs.py", False),
                ("docs/DOCKER.md", True),
                ("frontend-modern/public/docs/local/example.md", True),
                ("CONTRIBUTING.md", True),
                (".github/workflows/public-docs.yml", True),
            ):
                with self.subTest(path=path):
                    base = git("rev-parse", "HEAD")
                    file = root / path
                    file.parent.mkdir(parents=True, exist_ok=True)
                    file.write_text("fixture\n", encoding="utf-8")
                    git("add", "--", path)
                    git("commit", "-qm", path)
                    self.assertEqual(required(base), [f"required={str(want).lower()}"])

            base = git("rev-parse", "HEAD")
            git("mv", "docs/DOCKER.md", "docs/releases/moved.md")
            git("commit", "-qm", "remove a guide by moving it out of the Help paths")
            self.assertEqual(required(base), ["required=true"])
            self.assertEqual(required(git("rev-parse", "HEAD")), ["required=false"])
            for unknown in ("", "0000000000000000000000000000000000000000",
                            "1111111111111111111111111111111111111111", "--stat"):
                with self.subTest(base=unknown):
                    self.assertEqual(required(unknown), ["required=true"])

    def test_exit_adapters_use_the_selected_interpreter_without_site_startup(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            result = subprocess.run(
                ["python3", "-c", "import sys; print(sys.executable); "
                 "print(sys.flags.isolated, sys.flags.no_site)"],
                env=workflow_fixture_environment(root, root / "calls"),
                text=True, capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(result.stdout.splitlines(), [sys.executable, "1 1"])

    def test_python_exit_double_applies_only_to_generated_suite_invocations(self):
        with tempfile.TemporaryDirectory(prefix="docs adapter ") as temporary:
            root = Path(temporary)
            calls = root / "calls"
            name = "test_fixture_docs.py"
            env = workflow_fixture_environment(root, calls, failing=name)
            suite = root / "scripts/tests" / name
            suite.parent.mkdir(parents=True)
            suite.write_text("raise SystemExit(92)\n", encoding="utf-8")
            outside = root / name
            outside.write_text("raise SystemExit(93)\n", encoding="utf-8")
            missing = suite.with_name("test_missing_docs.py")
            for args, expected in (([suite], 1), ([outside], 93), ([suite, "extra"], 92), ([missing], 2)):
                with self.subTest(args=args):
                    result = subprocess.run(["python3", *args], env=env, capture_output=True, timeout=5)
                    self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            self.assertEqual(calls.read_text().splitlines(), [name])

    def run_adapters(self, shard: int, failing: str = ""):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tests = root / "scripts/tests"
            tests.mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/tests/run.sh", tests / "run.sh")
            shutil.copyfile(ROOT / "scripts/run-script-tests.py", root / "scripts/run-script-tests.py")
            (tests / "run.sh").chmod(0o700)
            shutil.copyfile(ROOT / "scripts/select-smoke-test-shard.py",
                            root / "scripts/select-smoke-test-shard.py")
            hints = root / ".github/scripts/docs-test-seconds.txt"
            hints.parent.mkdir(parents=True)
            shutil.copyfile(ROOT / ".github/scripts/docs-test-seconds.txt", hints)
            for name in documentation_suites():
                suite = tests / name
                if name.endswith(".sh"):
                    status = int(name == failing)
                    suite.write_text(f'#!/bin/bash\nprintf "%s\\n" {shlex.quote(name)} >> "$CALLS" || exit 90\n'
                                     f'exit {status}\n', encoding="utf-8")
                    suite.chmod(0o700)
                else:
                    # Discovery sees a real file. Bypassing the interpreter
                    # contract must fail rather than silently log a pass.
                    suite.write_text("raise SystemExit(92)\n", encoding="utf-8")
            calls = root / "calls"
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", safety_command(self.source, shard)],
                                    cwd=root, env=workflow_fixture_environment(root, calls, failing),
                                    text=True, capture_output=True, timeout=15)
            return result, calls.read_text().splitlines() if calls.exists() else []

    def test_actual_workflow_commands_cover_every_suite_once(self):
        seen = []
        for shard in range(4):
            result, calls = self.run_adapters(shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertTrue(calls)
            self.assertEqual(calls, sorted(calls))
            self.assertIn(f"Summary: {len(calls)}/{len(calls)} passed", result.stdout)
            seen.extend(calls)
        self.assertEqual(sorted(seen), documentation_suites())
        self.assertEqual(len(seen), len(set(seen)))

    def test_each_failed_suite_fails_its_actual_workflow_command(self):
        seen = []
        for shard in range(4):
            # Observe actual membership, not obsolete filename parity. The
            # complete-union assertion independently catches missing coverage.
            selected, names = self.run_adapters(shard)
            self.assertEqual(selected.returncode, 0, selected.stdout + selected.stderr)
            self.assertTrue(names)
            for failing in names:
                with self.subTest(failing=failing, shard=shard):
                    result, calls = self.run_adapters(shard, failing)
                    self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(calls, names)
                    self.assertIn(f"Summary: {len(calls)-1}/{len(calls)} passed", result.stdout)
                    self.assertIn("Failures: 1", result.stdout)
                    self.assertIn(f"  scripts/tests/{failing} (exit 1)\n", result.stdout)
                    seen.append(failing)
        self.assertEqual(sorted(seen), documentation_suites())
        self.assertEqual(len(seen), len(set(seen)))


if __name__ == "__main__":
    unittest.main()
