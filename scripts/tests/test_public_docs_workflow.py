#!/usr/bin/env python3
"""Docs-only CI must execute every existing safety suite and fail closed.

Exercise the actual workflow commands with disposable test adapters. Real
recipes run separately in the source-proof VM, never against customer services.
"""

from __future__ import annotations

import ast
import fnmatch
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import subprocess
import sys
import tempfile
import textwrap
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/public-docs.yml"
SAFETY_STEP = "Exercise documentation safety and diagnostic recipes"
DOCS_JOB = "check"
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


def workflow_paths(source: str, event: str) -> list[str]:
    triggers = source.split("\non:\n", 1)[1].split("\npermissions:", 1)[0]
    block = triggers.split(f"  {event}:\n", 1)[1]
    block = re.split(r"\n  [a-z_]+:\n", block, maxsplit=1)[0]
    return re.findall(r'^      - "([^"]+)"$', block, re.MULTILINE)


def matches_path(path: str, pattern: str) -> bool:
    # The workflow uses only recursive directory prefixes and segment-local
    # globs. A single '*' must not match across directories.
    if pattern.endswith("/**"):
        return path.startswith(pattern[:-2])
    parts, selectors = path.split("/"), pattern.split("/")
    return len(parts) == len(selectors) and all(
        fnmatch.fnmatchcase(part, selector) for part, selector in zip(parts, selectors)
    )


def local_fixture_imports(suites: list[str]) -> set[str]:
    # Direct in-tree Python fixture imports (including imports of fixtures).
    # Dynamic runner/control inputs are asserted separately below.
    pending = [ROOT / "scripts/tests" / name for name in suites if name.endswith(".py")]
    seen = set(pending)
    helpers = set()
    while pending:
        file = pending.pop()
        for node in ast.walk(ast.parse(file.read_bytes(), filename=str(file))):
            if isinstance(node, ast.Import):
                modules = [alias.name for alias in node.names]
            elif isinstance(node, ast.ImportFrom) and node.module:
                modules = [node.module]
            else:
                continue
            for module in modules:
                relative = Path(*module.split(".")).with_suffix(".py")
                for directory in ("scripts/tests", "scripts"):
                    helper = ROOT / directory / relative
                    if helper.is_file() and helper not in seen:
                        seen.add(helper)
                        pending.append(helper)
                        helpers.add(helper.relative_to(ROOT).as_posix())
    return helpers


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


def safety_command(source: str) -> str:
    body = step_body(job_body(source, DOCS_JOB), SAFETY_STEP)
    run = re.findall(r"^        run: (.+)$", body, re.MULTILINE)
    if len(run) != 1:
        raise AssertionError("expected one literal docs-shard runner command")
    return run[0]


def step_script(source: str, job: str, name: str) -> str:
    body = step_body(job_body(source, job), name)
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

    def test_both_events_cover_docs_inventory_and_control_changes(self):
        required = {
            "*.md", "docs/**", "frontend-modern/public/docs/**",
            ".github/ISSUE_TEMPLATE/**", ".github/PULL_REQUEST_TEMPLATE.md",
            "scripts/check_public_docs.py", "scripts/check_docs_mirror.py",
            "scripts/readme_entrypoints_test.py", "scripts/tests/test_*docs*.py",
            "scripts/tests/run.sh", "scripts/run-script-tests.py", "scripts/run-docs-shards.py",
            "scripts/ci_docs_only.py",
            "scripts/select-smoke-test-shard.py", ".github/scripts/docs-test-seconds.txt",
            "scripts/test-vm-disk.sh", ".github/workflows/public-docs.yml",
        }
        self.assertEqual(workflow_paths(self.source, "pull_request"),
                         workflow_paths(self.source, "push"))
        suites = documentation_suites()
        covered = {f"scripts/tests/{name}" for name in suites} | local_fixture_imports(suites)
        covered.update({
            "scripts/tests/agent_installer_test_support.py",
            "scripts/tests/test_new_infrastructure_docs.py",
        })
        for event in ("pull_request", "push"):
            with self.subTest(event=event):
                paths = workflow_paths(self.source, event)
                self.assertTrue(required.issubset(paths), required.difference(paths))
                self.assertEqual(len(paths), len(set(paths)))
                missing = {path for path in covered
                           if not any(matches_path(path, pattern) for pattern in paths)}
                self.assertFalse(missing, f"Docs suites or fixture imports without a trigger: {missing}")
        self.assertIn("  push:\n    branches:\n      - main\n", self.source)

    def test_unrelated_policy_and_smoke_suites_do_not_queue_docs_jobs(self):
        unrelated = (
            ".github/dependabot.yml", "scripts/tests/test_dependabot_config.py",
            "scripts/tests/test_smoke_runner.py", "scripts/tests/test_benchmark_workflow_contract.py",
        )
        for event in ("pull_request", "push"):
            for path in unrelated:
                with self.subTest(event=event, path=path):
                    self.assertFalse(any(matches_path(path, pattern)
                                         for pattern in workflow_paths(self.source, event)))

    def test_pushes_keep_the_active_verdict_and_prs_cancel_stale_checks(self):
        concurrency = self.source.split("\nconcurrency:\n", 1)[1].split("\njobs:\n", 1)[0]
        policy = [line.strip() for line in concurrency.splitlines()
                  if line.strip() and not line.lstrip().startswith("#")]
        self.assertEqual(policy, [
            "group: public-docs-${{ github.ref }}",
            "cancel-in-progress: ${{ github.event_name == 'pull_request' }}",
        ])

    def test_shards_discover_current_suites_without_dependency_setup(self):
        self.assertTrue(set(REQUIRED_SUITES).issubset(documentation_suites()))
        self.assertEqual(shlex.split(safety_command(self.source)),
                         ["python3", "scripts/run-docs-shards.py"])
        check = job_body(self.source, DOCS_JOB)
        before_help = check.split(f"      - name: {HELP_DIFF_STEP}\n", 1)[0]
        self.assertIn(f"      - name: {SAFETY_STEP}\n", before_help)
        self.assertNotIn("actions/setup-", before_help)
        self.assertNotRegex(before_help, r"\b(?:npm|pip|go) (?:ci|install|build|test)\b")

    def test_existing_document_admissions_are_preserved(self):
        validation = job_body(self.source, DOCS_JOB)
        for name, command in (
            ("Validate public documentation", "python3 scripts/check_public_docs.py"),
            ("Check getting-started safety and current plans", "python3 scripts/readme_entrypoints_test.py"),
            ("Check shipped docs mirror sync", "python3 scripts/check_docs_mirror.py"),
            ("Check documentation safety workflow contract", "python3 scripts/tests/test_public_docs_workflow.py"),
        ):
            with self.subTest(step=name):
                step = step_body(validation, name)
                self.assertIn(f"        run: {command}\n", step)
                self.assertNotRegex(step, r"(?m)^        if:")
                self.assertLess(validation.index(name), validation.index(SAFETY_STEP))

    def test_single_required_check_job_is_mandatory_and_bounded(self):
        jobs = self.source.split("\njobs:\n", 1)[1]
        self.assertEqual(re.findall(r"(?m)^  ([a-z0-9_-]+):\s*$", jobs), [DOCS_JOB])
        check = job_body(self.source, DOCS_JOB)
        self.assertNotRegex(check, r"(?m)^    (?:if|needs|strategy):")
        self.assertNotRegex(step_body(check, SAFETY_STEP), r"(?m)^        if:")
        self.assertNotIn("continue-on-error:", self.source)
        self.assertIn("    runs-on: ubuntu-24.04\n", check)
        self.assertIn("    timeout-minutes: 10\n", check)
        self.assertIn("          persist-credentials: false\n", check)
        self.assertIn("permissions:\n  contents: read\n", self.source)

    def test_help_proof_is_part_of_the_required_check(self):
        validation = job_body(self.source, DOCS_JOB)
        checkout = step_body(validation, "Check out repository")
        self.assertIn("          fetch-depth: 2\n", checkout)
        detector = step_body(validation, HELP_DIFF_STEP)
        self.assertIn("        id: help\n", detector)
        self.assertIn("github.event.pull_request.base.sha || github.event.before", detector)
        setup, install, test = (
            "Set up Node.js for Help assertions",
            "Install locked Help test dependencies",
            HELP_TEST_STEP,
        )
        for name in (setup, install, test):
            self.assertIn("        if: steps.help.outputs.required != 'false'\n",
                          step_body(validation, name))
        self.assertIn("cache-dependency-path: frontend-modern/package-lock.json",
                      step_body(validation, setup))
        self.assertIn("node-version: '24'", step_body(validation, setup))
        for name, command in (
            (install, "npm ci --no-audit --no-fund"),
            (test, "npm run test -- src/features/docs/__tests__ src/utils/__tests__/docsLinks.test.ts"),
        ):
            step = step_body(validation, name)
            self.assertIn("        working-directory: frontend-modern\n", step)
            self.assertEqual(re.findall(r"^        run: (.+)$", step, re.MULTILINE), [command])
        order = [validation.index(name) for name in (
            "Check shipped docs mirror sync", HELP_DIFF_STEP, setup, install, test
        )]
        self.assertEqual(order, sorted(order))

    def test_help_diff_admission_uses_exact_trees_and_fails_safe(self):
        command = step_script(self.source, DOCS_JOB, HELP_DIFF_STEP)
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

    def populate_adapter_tree(self, root: Path, failing: str = "") -> None:
        tests = root / "scripts/tests"
        tests.mkdir(parents=True)
        shutil.copyfile(ROOT / "scripts/tests/run.sh", tests / "run.sh")
        shutil.copyfile(ROOT / "scripts/run-script-tests.py", root / "scripts/run-script-tests.py")
        shutil.copyfile(ROOT / "scripts/run-docs-shards.py", root / "scripts/run-docs-shards.py")
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

    def run_adapters(self, shard: int | None, failing: str = ""):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            self.populate_adapter_tree(root, failing)
            calls = root / "calls"
            command = (
                safety_command(self.source)
                if shard is None
                else f"scripts/tests/run.sh --docs-shard {shard}"
            )
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", command],
                                    cwd=root, env=workflow_fixture_environment(root, calls, failing),
                                    text=True, capture_output=True, timeout=20)
            return result, calls.read_text().splitlines() if calls.exists() else []

    def test_actual_workflow_command_covers_every_suite_once_in_deterministic_shard_order(self):
        result, calls = self.run_adapters(None)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(sorted(calls), documentation_suites())
        self.assertEqual(len(calls), len(set(calls)))
        headers = re.findall(r"^=== Documentation shard (\d)/4 \(exit 0, [0-9.]+s\) ===$",
                             result.stdout, re.MULTILINE)
        self.assertEqual(headers, ["0", "1", "2", "3"])
        self.assertIn("Documentation shards summary: 4/4 passed", result.stdout)
        seen = []
        sections = re.split(r"(?m)^=== Documentation shard \d/4 .* ===\n", result.stdout)[1:]
        self.assertEqual(len(sections), 4)
        for section in sections:
            names = [line.removeprefix("==> scripts/tests/") for line in section.splitlines()
                     if line.startswith("==> scripts/tests/")]
            self.assertTrue(names)
            self.assertEqual(names, sorted(names))
            self.assertIn(f"Summary: {len(names)}/{len(names)} passed", section)
            seen.extend(names)
        self.assertEqual(sorted(seen), documentation_suites())
        self.assertEqual(len(seen), len(set(seen)))

    def test_each_failed_suite_fails_its_shard_and_parallel_runner_verdict(self):
        seen = []
        representative_by_shard = {}
        for shard in range(4):
            # Observe actual membership, not obsolete filename parity. The
            # complete-union assertion independently catches missing coverage.
            selected, names = self.run_adapters(shard)
            self.assertEqual(selected.returncode, 0, selected.stdout + selected.stderr)
            self.assertTrue(names)
            representative_by_shard[shard] = names[0]
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

        for shard, failing in representative_by_shard.items():
            with self.subTest(parallel_failing_shard=shard, failing=failing):
                result, calls = self.run_adapters(None, failing)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertEqual(sorted(calls), documentation_suites())
                self.assertIn("Documentation shards summary: 3/4 passed", result.stdout)
                self.assertIn(f"  shard {shard}/4 (exit 1)\n", result.stdout)

    def test_parallel_runner_executes_all_four_shards_concurrently_without_fail_fast(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "scripts/tests").mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/run-docs-shards.py", root / "scripts/run-docs-shards.py")
            barrier = root / "arrived"
            barrier.mkdir()
            runner = root / "scripts/tests/run.sh"
            runner.write_text(
                '#!/usr/bin/env bash\n'
                'set -euo pipefail\n'
                'test "$1" = "--docs-shard"\n'
                'shard="$2"\n'
                'touch "$BARRIER_DIR/$shard"\n'
                'for _ in $(seq 1 100); do\n'
                '  if [ "$(ls -1 "$BARRIER_DIR" | wc -l)" -eq 4 ]; then\n'
                '    printf "shard %s observed all 4 peers\\n" "$shard"\n'
                '    if [ "$shard" = "$FAIL_SHARD_A" ] || [ "$shard" = "$FAIL_SHARD_B" ]; then\n'
                '      exit 7\n'
                '    fi\n'
                '    exit 0\n'
                '  fi\n'
                '  sleep 0.05\n'
                'done\n'
                'echo "timed out waiting for concurrent shards" >&2\n'
                'exit 9\n',
                encoding="utf-8",
            )
            runner.chmod(0o700)
            env = dict(os.environ, BARRIER_DIR=str(barrier), FAIL_SHARD_A="", FAIL_SHARD_B="")
            passing = subprocess.run([sys.executable, str(root / "scripts/run-docs-shards.py")],
                                     cwd=root, env=env, text=True, capture_output=True, timeout=15)
            self.assertEqual(passing.returncode, 0, passing.stdout + passing.stderr)
            for shard in range(4):
                self.assertIn(f"shard {shard} observed all 4 peers", passing.stdout)
            self.assertIn("Documentation shards summary: 4/4 passed", passing.stdout)

            for entry in barrier.iterdir():
                entry.unlink()
            failing = subprocess.run(
                [sys.executable, str(root / "scripts/run-docs-shards.py")],
                cwd=root, env=dict(env, FAIL_SHARD_A="1", FAIL_SHARD_B="3"),
                text=True, capture_output=True, timeout=15,
            )
            self.assertEqual(failing.returncode, 1, failing.stdout + failing.stderr)
            for shard in range(4):
                self.assertIn(f"shard {shard} observed all 4 peers", failing.stdout)
            self.assertIn("Documentation shards summary: 2/4 passed", failing.stdout)
            self.assertIn("  shard 1/4 (exit 7)\n", failing.stdout)
            self.assertIn("  shard 3/4 (exit 7)\n", failing.stdout)

    def test_parallel_runner_stops_shard_groups_on_sigterm_and_refuses_pass(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "scripts/tests").mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/run-docs-shards.py", root / "scripts/run-docs-shards.py")
            ready = root / "ready"
            ready.mkdir()
            runner = root / "scripts/tests/run.sh"
            runner.write_text(
                '#!/usr/bin/env bash\n'
                'set -euo pipefail\n'
                'if [ "$2" = "3" ]; then trap "" TERM; fi\n'
                'sleep 60 &\n'
                'child=$!\n'
                'printf "%s %s\\n" "$$" "$child" > "$READY_DIR/$2"\n'
                'wait "$child"\n',
                encoding="utf-8",
            )
            runner.chmod(0o700)
            process = subprocess.Popen(
                [sys.executable, str(root / "scripts/run-docs-shards.py")],
                cwd=root, env=dict(os.environ, READY_DIR=str(ready)),
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                start_new_session=True,
            )
            try:
                deadline = time.monotonic() + 5
                while len(list(ready.iterdir())) < 4 and time.monotonic() < deadline:
                    time.sleep(0.05)
                self.assertEqual(len(list(ready.iterdir())), 4)
                pids = []
                for entry in ready.iterdir():
                    pids.extend(int(token) for token in entry.read_text().split())
                self.assertEqual(len(pids), 8)
                process.send_signal(signal.SIGTERM)
                stdout, stderr = process.communicate(timeout=15)
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.communicate(timeout=5)
            self.assertEqual(process.returncode, 128 + signal.SIGTERM, stdout + stderr)
            self.assertIn("interrupted (signal 15)", stderr)
            self.assertNotIn("Documentation shards summary:", stdout)
            for pid in pids:
                stat = Path(f"/proc/{pid}/stat")
                try:
                    state = stat.read_text(encoding="utf-8").rsplit(") ", 1)[1].split()[0]
                except (FileNotFoundError, ProcessLookupError):
                    state = None
                self.assertIn(state, (None, "Z"), f"pid {pid} survived SIGTERM cleanup in state {state}")

    def test_parallel_runner_fails_closed_when_runner_missing_or_symlink(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "scripts/tests").mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/run-docs-shards.py", root / "scripts/run-docs-shards.py")
            missing = subprocess.run([sys.executable, str(root / "scripts/run-docs-shards.py")],
                                     cwd=root, text=True, capture_output=True, timeout=5)
            self.assertEqual(missing.returncode, 1)
            self.assertIn("missing or not a regular file", missing.stderr)

            target = root / "elsewhere.sh"
            target.write_text("#!/bin/sh\nexit 0\n", encoding="utf-8")
            (root / "scripts/tests/run.sh").symlink_to(target)
            symlinked = subprocess.run([sys.executable, str(root / "scripts/run-docs-shards.py")],
                                       cwd=root, text=True, capture_output=True, timeout=5)
            self.assertEqual(symlinked.returncode, 1)
            self.assertIn("missing or not a regular file", symlinked.stderr)


if __name__ == "__main__":
    unittest.main()
