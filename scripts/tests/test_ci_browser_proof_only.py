#!/usr/bin/env python3
"""Execute narrow CI admission and the workflows' actual verdict paths."""

import importlib.util
import fnmatch
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import yaml


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/ci_browser_proof_only.py"
BUILD = ROOT / ".github/workflows/build-and-test.yml"
E2E = ROOT / ".github/workflows/test-e2e.yml"
SPEC = importlib.util.spec_from_file_location("ci_browser_proof_only", SCRIPT)
SCOPE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCOPE)
TRIAGE_SPEC = importlib.util.spec_from_file_location("ci_triage_only", ROOT / "scripts/ci_triage_only.py")
TRIAGE = importlib.util.module_from_spec(TRIAGE_SPEC)
TRIAGE_SPEC.loader.exec_module(TRIAGE)
COMPLETION_SPEC = importlib.util.spec_from_file_location("ci_completion_only", ROOT / "scripts/ci_completion_only.py")
COMPLETION = importlib.util.module_from_spec(COMPLETION_SPEC)
COMPLETION_SPEC.loader.exec_module(COMPLETION)
RECEIPT = "frontend-modern/browser-verification/2026-10-14-proof.json"
JOBS = ("tier-selection", "test-images", "e2e", "agent-registration")


def job_block(workflow, name):
    match = re.search(rf"^  {re.escape(name)}:\n(.*?)(?=^  [\w-]+:\n|\Z)",
                      workflow, re.M | re.S)
    if match is None:
        raise AssertionError(f"missing workflow job {name}")
    return match.group(1)


def step_script(block, name):
    # Execute only the actual YAML scalar. Comments between steps belong to
    # neither command; stripping ten columns can turn them into shell code.
    steps = yaml.safe_load(block)['steps']
    selected = [step for step in steps if step.get('name') == name]
    if len(selected) != 1 or not isinstance(selected[0].get('run'), str) or not selected[0]['run'].strip():
        raise AssertionError(f'expected one nonempty command for step {name}')
    return selected[0]['run']


def interpolate(script, expressions):
    return re.sub(r"\$\{\{\s*(.*?)\s*\}\}", lambda m: expressions[m[1]], script)


class StepScriptTest(unittest.TestCase):
    def test_command_scalar_excludes_adjacent_yaml_comments_and_steps(self):
        block = '''
    steps:
      - name: "Selected command"
        run: |
          # This comment really is inside the shell command.
          printf '%s\\n' 'actual command'

      # A deterministic control failure must not turn this into shell code.
      - name: Other command
        run: exit 19
'''
        self.assertEqual(step_script(block, 'Selected command'),
                         "# This comment really is inside the shell command.\n"
                         "printf '%s\\n' 'actual command'\n")
        self.assertEqual(step_script(block, 'Other command'), 'exit 19')

    def test_missing_duplicate_action_or_empty_command_cannot_supply_a_recipe(self):
        for steps in (
            '- name: Other\n  run: exit 0',
            '- name: Selected\n  run: exit 0\n- name: Selected\n  run: exit 0',
            '- name: Selected\n  uses: owner/action@' + 'a' * 40,
            '- name: Selected\n  run: ""',
        ):
            with self.subTest(steps=steps), self.assertRaises(AssertionError):
                step_script('steps:\n' + '\n'.join('  ' + line for line in steps.splitlines()),
                            'Selected')


class ReceiptScopeTest(unittest.TestCase):
    def test_only_top_level_receipt_json_is_eligible(self):
        paths = [RECEIPT.encode(), b"frontend-modern/browser-verification/a07b57622067316f.json"]
        self.assertTrue(SCOPE.receipt_only_paths(paths))
        self.assertFalse(SCOPE.receipt_only_paths([]))

    def test_source_graph_tests_controls_docs_and_lookalikes_retain_runtime(self):
        for path in (
            "frontend-modern/src/components/Machines.tsx", "frontend-modern/public/docs/VM_DISK_MONITORING.md",
            "frontend-modern/index.html", "frontend-modern/package.json", "frontend-modern/package-lock.json",
            "tests/integration/package-lock.json", "tests/integration/tests/recovery.spec.ts",
            "internal/api/server.go", "internal/api/embed.json", "go.mod", "go.sum", "Dockerfile",
            "scripts/ci_browser_proof_only.py", "scripts/tests/test_ci_browser_proof_only.py",
            ".github/workflows/build-and-test.yml", ".github/workflows/test-e2e.yml", "docs/guide.md",
            "frontend-modern/browser-verification.json", "frontend-modern/browser-verification/nested/a.json",
            "frontend-modern/browser-verification/a.ts", "frontend-modern/browser-verification/.json",
            "frontend-modern/browser-verification/a.json\ninternal/api/server.go",
        ):
            with self.subTest(path=path):
                self.assertFalse(SCOPE.receipt_only_paths([RECEIPT.encode(), path.encode()]))

    def test_missing_invalid_empty_failed_and_incomplete_diffs_retain_runtime(self):
        for base, head in (("", "a" * 40), ("HEAD^", "a" * 40), ("a" * 40, "main")):
            with patch.object(SCOPE.subprocess, "run") as run:
                self.assertFalse(SCOPE.receipt_only_diff(base, head))
                run.assert_not_called()
        for output, code in ((b"", 0), (RECEIPT.encode(), 0), (RECEIPT.encode() + b"\0", 1)):
            with patch.object(SCOPE.subprocess, "run", return_value=subprocess.CompletedProcess([], code, output)):
                self.assertFalse(SCOPE.receipt_only_diff("a" * 40, "b" * 40))
        for error in (OSError("fixture unavailable"), subprocess.TimeoutExpired("git", 30)):
            with patch.object(SCOPE.subprocess, "run", side_effect=error):
                self.assertFalse(SCOPE.receipt_only_diff("a" * 40, "b" * 40))

    def test_complete_null_delimited_diff_is_consumed_without_rename_or_external_diff(self):
        output = (RECEIPT.encode() + b"\0") * 4000 + b"internal/api/server.go\0"
        with patch.object(SCOPE.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, output)) as run:
            self.assertFalse(SCOPE.receipt_only_diff("a" * 40, "b" * 40))
            self.assertEqual(run.call_args.args[0], [
                "git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
                "a" * 40, "b" * 40, "--",
            ])


class TriageScopeTest(unittest.TestCase):
    def test_only_complete_exact_triage_inputs_are_eligible(self):
        self.assertTrue(TRIAGE.triage_only_paths(list(TRIAGE.TRIAGE_INPUTS)))
        self.assertFalse(TRIAGE.triage_only_paths([]))
        for path in TRIAGE.TRIAGE_INPUTS:
            with self.subTest(path=path):
                self.assertTrue(TRIAGE.triage_only_paths([path]))
                for suffix in (b".extra", b"/extra", b"\ninternal/api/server.go"):
                    self.assertFalse(TRIAGE.triage_only_paths([path + suffix]))

    def test_product_tests_controls_templates_workflows_and_unknown_files_keep_runtime(self):
        for path in (
            b"frontend-modern/src/App.tsx", b"frontend-modern/package-lock.json",
            b"internal/api/server.go", b"internal/api/server_test.go", b"go.mod", b"go.sum",
            b"install.sh", b"scripts/tests/run.sh", b"tests/integration/playwright.config.ts",
            b".github/workflows/build-and-test.yml", b".github/workflows/issue-version-label-sync.yml",
            b".github/ISSUE_TEMPLATE/bug_report.yml", b".github/scripts/other.cjs", b"docs/other.md",
            b"scripts/ci_triage_only.py", b"scripts/tests/test_ci_browser_proof_only.py",
            RECEIPT.encode(), b"other/.github/scripts/package.json",
        ):
            with self.subTest(path=path):
                self.assertFalse(TRIAGE.triage_only_paths([*TRIAGE.TRIAGE_INPUTS, path]))

    def test_unknown_failed_incomplete_or_missing_diff_keeps_runtime(self):
        for base, head in (("", "a" * 40), ("HEAD^", "a" * 40), ("a" * 40, "main")):
            with patch.object(TRIAGE.subprocess, "run") as run:
                self.assertFalse(TRIAGE.triage_only_diff(base, head))
                run.assert_not_called()
        path = b".github/scripts/issue-version-triage.cjs"
        for output, code in ((b"", 0), (path, 0), (path + b"\0", 1)):
            with patch.object(TRIAGE.subprocess, "run", return_value=subprocess.CompletedProcess([], code, output)):
                self.assertFalse(TRIAGE.triage_only_diff("a" * 40, "b" * 40))
        for error in (OSError("fixture unavailable"), subprocess.TimeoutExpired("git", 30)):
            with patch.object(TRIAGE.subprocess, "run", side_effect=error):
                self.assertFalse(TRIAGE.triage_only_diff("a" * 40, "b" * 40))

    def test_complete_null_delimited_diff_disables_rename_and_external_diff(self):
        path = b".github/scripts/issue-version-triage.cjs\0"
        for output, expected in ((path, True), (path * 4000 + b"internal/api/server.go\0", False)):
            with patch.object(TRIAGE.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, output)) as run:
                self.assertEqual(TRIAGE.triage_only_diff("a" * 40, "b" * 40), expected)
                self.assertEqual(run.call_args.args[0], [
                    "git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
                    "a" * 40, "b" * 40, "--",
                ])


class CompletionScopeTest(unittest.TestCase):
    def test_only_the_four_governance_tested_inputs_are_eligible(self):
        self.assertEqual(COMPLETION.COMPLETION_INPUTS, {
            f"scripts/release_control/canonical_completion_{helper}{suffix}.py".encode()
            for helper in ("guard", "history") for suffix in ("", "_test")
        })
        self.assertFalse(COMPLETION.completion_only_paths([]))
        for path in COMPLETION.COMPLETION_INPUTS:
            self.assertTrue(COMPLETION.completion_only_paths([path]))
            for suffix in (b".extra", b"/extra", b"\ninternal/api/server.go"):
                self.assertFalse(COMPLETION.completion_only_paths([path + suffix]))
        for path in (
            b"scripts/ci_completion_only.py", b"scripts/tests/test_ci_browser_proof_only.py",
            b"scripts/release_control/canonical_completion_history.json",
            b"scripts/release_control/browser_verification_guard.py", b"scripts/release_control/repo_file_io.py",
            b".github/workflows/canonical-governance.yml", b".github/workflows/build-and-test.yml",
            b"frontend-modern/package-lock.json", b"internal/api/server.go", b"go.mod", b"go.sum",
            b"install.sh", b"scripts/tests/run.sh", b"docs/other.md", RECEIPT.encode(),
        ):
            with self.subTest(path=path):
                self.assertFalse(COMPLETION.completion_only_paths([*COMPLETION.COMPLETION_INPUTS, path]))

    def test_unavailable_invalid_incomplete_or_failed_diff_keeps_runtime(self):
        for base, head in (("", "a" * 40), ("HEAD^", "a" * 40), ("a" * 40, "main")):
            with patch.object(COMPLETION.subprocess, "run") as run:
                self.assertFalse(COMPLETION.completion_only_diff(base, head))
                run.assert_not_called()
        path = next(iter(COMPLETION.COMPLETION_INPUTS))
        for output, code in ((b"", 0), (path, 0), (path + b"\0", 1)):
            with patch.object(COMPLETION.subprocess, "run", return_value=subprocess.CompletedProcess([], code, output)):
                self.assertFalse(COMPLETION.completion_only_diff("a" * 40, "b" * 40))
        for error in (OSError("fixture unavailable"), subprocess.TimeoutExpired("git", 30)):
            with patch.object(COMPLETION.subprocess, "run", side_effect=error):
                self.assertFalse(COMPLETION.completion_only_diff("a" * 40, "b" * 40))

    def test_complete_diff_disables_external_diff_and_rename_detection(self):
        path = next(iter(COMPLETION.COMPLETION_INPUTS)) + b"\0"
        for output, expected in ((path, True), (path * 4000 + b"internal/api/server.go\0", False)):
            with patch.object(COMPLETION.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, output)) as run:
                self.assertEqual(COMPLETION.completion_only_diff("a" * 40, "b" * 40), expected)
                self.assertEqual(run.call_args.args[0], [
                    "git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
                    "a" * 40, "b" * 40, "--",
                ])

    def test_governance_still_runs_the_real_helpers_suites_and_consumers(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/canonical-governance.yml").read_text())
        # PyYAML's YAML 1.1 loader recognises the unquoted Actions 'on' as True.
        for event in ("push", "pull_request", "workflow_dispatch"):
            self.assertIn(event, workflow[True])
            config = workflow[True][event] or {}
            self.assertNotIn("paths", config)
            self.assertNotIn("paths-ignore", config)
            if event != "workflow_dispatch":
                for branch in ("main", "release/v6.4", "release/v6.6.1-rc.1"):
                    self.assertTrue(any(fnmatch.fnmatchcase(branch, pattern)
                                        for pattern in config["branches"]))
        job = workflow["jobs"]["governance"]
        self.assertNotIn("if", job)
        self.assertNotIn("continue-on-error", job)
        steps = {step["name"]: step for step in job["steps"]}
        required = {
            "Run canonical completion guard against changed commits": "scripts/release_control/canonical_completion_guard.py",
            "Run canonical completion guard unit tests": "python3 scripts/release_control/canonical_completion_guard_test.py",
            "Run canonical completion history unit tests": "python3 scripts/release_control/canonical_completion_history_test.py",
            "Run registry audit": "python3 scripts/release_control/registry_audit.py --check --repo-scope pulse",
            "Run contract audit": "python3 scripts/release_control/contract_audit.py --check --repo-scope pulse",
            "Run status audit unit tests": "python3 scripts/release_control/status_audit_test.py",
            "Run subsystem lookup unit tests": "python3 scripts/release_control/subsystem_lookup_test.py",
        }
        for name, command in required.items():
            with self.subTest(step=name):
                self.assertIn(command, steps[name]["run"])
                self.assertNotIn("if", steps[name])
                self.assertNotIn("continue-on-error", steps[name])
        self.assertIn("scripts/release_control/canonical_completion_history.py",
                      steps["Run canonical completion guard against changed commits"]["run"])


class GitTreeScopeTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.git("init", "-q", "--initial-branch=main")
        for name in ("scripts/ci_browser_proof_only.py", "scripts/ci_triage_only.py",
                     "scripts/ci_completion_only.py",
                     "scripts/go-test-compile-required.sh"):
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / name, path)
        self.write("internal/api/fixture.go", "package fixture\n")
        self.base = self.commit()

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-c", "user.name=CI fixture", "-c", "user.email=ci@example.invalid",
             "-c", "core.hooksPath=/dev/null", *args], cwd=self.repo,
            stderr=subprocess.PIPE, text=True,
        ).strip()

    def write(self, name, text):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "Fixture tree change")
        return self.git("rev-parse", "HEAD")

    def scope(self, base, head):
        result = subprocess.run(
            ["python3", str(SCRIPT), "--base", base, "--head", head],
            cwd=self.repo, capture_output=True, text=True, check=True, timeout=10,
        )
        return result.stdout.strip()

    def test_real_git_receipt_add_edit_delete_and_empty_tree(self):
        self.assertEqual(self.scope(self.base, self.base), "false")
        self.assertEqual(self.scope("0" * 40, self.base), "false")
        self.write(RECEIPT, "{}\n")
        added = self.commit()
        self.assertEqual(self.scope(self.base, added), "true")
        self.write(RECEIPT, '{"updated":true}\n')
        edited = self.commit()
        self.assertEqual(self.scope(added, edited), "true")
        (self.repo / RECEIPT).unlink()
        deleted = self.commit()
        self.assertEqual(self.scope(edited, deleted), "true")

    def test_real_git_full_range_does_not_hide_source_behind_a_later_receipt(self):
        self.write("internal/api/fixture.go", "package fixture\nvar Updated = true\n")
        source = self.commit()
        self.write(RECEIPT, "{}\n")
        proof = self.commit()
        self.assertEqual(self.scope(source, proof), "true")
        self.assertEqual(self.scope(self.base, proof), "false")

    def test_real_git_rename_to_source_and_quoted_filename_cannot_skip(self):
        self.write(RECEIPT, "{}\n")
        receipt = self.commit()
        target = self.repo / "frontend-modern/src/fixture.json"
        target.parent.mkdir(parents=True, exist_ok=True)
        (self.repo / RECEIPT).rename(target)
        renamed = self.commit()
        self.assertEqual(self.scope(receipt, renamed), "false")
        self.write('frontend-modern/src/quoted\n"fixture.json', "{}\n")
        quoted = self.commit()
        self.assertEqual(self.scope(renamed, quoted), "false")

    def classify(self, base, head, target_branch="main"):
        script = step_script(job_block(BUILD.read_text(), "changes"), "Classify changed files")
        script = interpolate(script, {
            "github.event_name": "pull_request", "github.event.pull_request.base.sha": base,
            "github.event.before": base, "github.sha": head,
        })
        # Workflow output is runner evidence, not a source change. Keep it
        # outside the fixture repository so the next commit cannot stage it
        # and accidentally turn an isolated change into a mixed tree diff.
        with tempfile.TemporaryDirectory() as output_dir:
            output = Path(output_dir) / "outputs"
            result = subprocess.run(["bash", "-c", script], cwd=self.repo, capture_output=True,
                                    text=True, timeout=10, env={**os.environ, "GITHUB_OUTPUT": str(output),
                                                               "GOVERNANCE_TARGET_BRANCH": target_branch})
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            return dict(line.split("=", 1) for line in output.read_text().splitlines())

    def test_actual_build_classifier_omits_only_receipt_runtime_checks(self):
        self.write(RECEIPT, "{}\n")
        proof = self.commit()
        self.assertEqual(self.classify(self.base, proof), {
            "code": "false", "frontend_deps": "false", "go_compile": "false",
            "triage_helper": "false",
        })
        self.write("frontend-modern/package-lock.json", "{}\n")
        graph = self.commit()
        self.assertEqual(self.classify(self.base, graph), {
            "code": "true", "frontend_deps": "true", "go_compile": "false",
            "triage_helper": "false",
        })
        self.assertEqual(self.classify("0" * 40, graph), {
            "code": "true", "frontend_deps": "true", "go_compile": "true",
            "triage_helper": "true",
        })
        self.write("internal/api/fixture.go", "package fixture\nvar Updated = true\n")
        source = self.commit()
        self.assertEqual(self.classify(graph, source), {
            "code": "true", "frontend_deps": "false", "go_compile": "true",
            "triage_helper": "false",
        })

    def test_completion_inputs_keep_governance_without_runtime_fanout(self):
        for path in sorted(COMPLETION.COMPLETION_INPUTS):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path.decode(), "# governance helper fixture\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head), {
                    "code": "false", "frontend_deps": "false", "go_compile": "false",
                    "triage_helper": "false",
                })
                (self.repo / path.decode()).unlink()
                deleted = self.commit()
                self.assertEqual(self.classify(head, deleted)["code"], "false")
        self.assertEqual(self.classify(deleted, deleted)["code"], "true")  # conservative ordinary fallback
        empty = subprocess.run(
            ["python3", str(ROOT / "scripts/ci_completion_only.py"), "--base", deleted, "--head", deleted],
            cwd=self.repo, capture_output=True, text=True, check=True, timeout=10,
        )
        self.assertEqual(empty.stdout.strip(), "false")

    def test_completion_exception_requires_a_governance_covered_target_branch(self):
        self.write("scripts/release_control/canonical_completion_guard.py", "# helper fixture\n")
        helper = self.commit()
        for branch, code in (("main", "false"), ("release/v6.4", "false"),
                             ("release/v6.6.1-rc.1", "false"), ("pulse/v6-release", "true"),
                             ("", "true"), ("main-extra", "true"), ("other/main", "true")):
            with self.subTest(branch=branch):
                self.assertEqual(self.classify(self.base, helper, branch)["code"], code)

    def test_completion_mixed_whole_range_and_missing_base_keep_runtime(self):
        self.write("internal/api/fixture.go", "package fixture\nvar Changed = true\n")
        source = self.commit()
        self.write("scripts/release_control/canonical_completion_guard.py", "# helper fixture\n")
        helper = self.commit()
        self.assertEqual(self.classify(source, helper)["code"], "false")
        self.assertEqual(self.classify(self.base, helper)["code"], "true")
        self.assertEqual(self.classify("0" * 40, helper), {
            "code": "true", "frontend_deps": "true", "go_compile": "true", "triage_helper": "true",
        })
        self.write("frontend-modern/package-lock.json", "{}\n")
        graph = self.commit()
        self.assertEqual(self.classify(source, graph)["code"], "true")
        self.assertEqual(self.classify(source, graph)["frontend_deps"], "true")

    def test_completion_controls_and_pair_registry_cannot_skip_runtime(self):
        for path in ("scripts/ci_completion_only.py", "scripts/tests/test_ci_browser_proof_only.py",
                     ".github/workflows/build-and-test.yml", ".github/workflows/canonical-governance.yml",
                     "scripts/release_control/canonical_completion_history.json"):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path, (self.repo / path).read_text() + "\n# Changed control\n"
                           if (self.repo / path).exists() else "# Changed control\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head)["code"], "true")

    def test_completion_rename_and_quoted_path_cannot_hide_runtime_changes(self):
        self.write("scripts/release_control/canonical_completion_guard.py", "package fixture\n")
        helper = self.commit()
        (self.repo / "internal/api/fixture.go").rename(
            self.repo / "scripts/release_control/canonical_completion_history.py")
        renamed = self.commit()
        self.assertEqual(self.classify(helper, renamed)["code"], "true")
        self.write("scripts/release_control/canonical_completion_guard.py\ninternal/api/source.go", "# lookalike\n")
        quoted = self.commit()
        self.assertEqual(self.classify(renamed, quoted)["code"], "true")

    def test_failed_completion_selector_does_not_admit_a_skip(self):
        self.write("scripts/release_control/canonical_completion_guard.py", "# helper fixture\n")
        self.write("scripts/ci_completion_only.py", "raise SystemExit(42)\n")
        failed = self.commit()
        self.assertEqual(self.classify(self.base, failed)["code"], "true")

    def test_actual_classifier_selects_every_triage_input_and_policy_consumer(self):
        for path in (
            ".github/scripts/issue-version-triage.cjs",
            ".github/scripts/issue-version-triage.test.cjs",
            ".github/scripts/triage-comment-policy.test.cjs",
            ".github/scripts/package.json", ".github/scripts/package-lock.json",
            ".github/ISSUE_TEMPLATE/bug_report.yml", ".github/ISSUE_TEMPLATE/config.yml",
            ".github/workflows/issue-version-label-sync.yml",
            ".github/workflows/build-and-test.yml", ".github/workflows/policy-consumer.yaml",
            "docs/ISSUE_TRIAGE.md", "scripts/ci_triage_only.py", "scripts/tests/test_ci_browser_proof_only.py",
        ):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path, "triage input fixture\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head)["triage_helper"], "true")

    def test_triage_admission_uses_complete_range_and_keeps_mixed_runtime_checks(self):
        self.write("docs/ISSUE_TRIAGE.md", "Changed operator guidance\n")
        guidance = self.commit()
        self.assertEqual(self.classify(self.base, guidance), {
            "code": "false", "frontend_deps": "false", "go_compile": "false",
            "triage_helper": "true",
        })
        self.write(RECEIPT, "{}\n")
        proof = self.commit()
        self.assertEqual(self.classify(guidance, proof)["triage_helper"], "false")
        self.assertEqual(self.classify(self.base, proof)["triage_helper"], "true")
        self.write(".github/scripts/issue-version-triage.cjs", "// changed parser\n")
        parser = self.commit()
        self.assertEqual(self.classify(proof, parser), {
            "code": "false", "frontend_deps": "false", "go_compile": "false",
            "triage_helper": "true",
        })
        self.write("frontend-modern/package-lock.json", "{}\n")
        graph = self.commit()
        combined = self.classify(proof, graph)
        self.assertEqual(combined["frontend_deps"], "true")
        self.assertEqual(combined["code"], "true")
        self.assertEqual(combined["triage_helper"], "true")

    def test_isolated_triage_inputs_use_the_dedicated_check(self):
        for path in sorted(TRIAGE.TRIAGE_INPUTS):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path.decode(), "triage fixture\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head), {
                    "code": "false", "frontend_deps": "false", "go_compile": "false",
                    "triage_helper": "true",
                })

    def test_triage_skip_cannot_hide_source_renames_or_quoted_paths(self):
        self.write("internal/api/fixture.go", "package fixture\nvar Changed = true\n")
        product = self.commit()
        self.write(".github/scripts/issue-version-triage.cjs", "// parser fixture\n")
        parser = self.commit()
        self.assertEqual(self.classify(product, parser)["code"], "false")
        self.assertEqual(self.classify(self.base, parser)["code"], "true")
        (self.repo / "internal/api/fixture.go").rename(self.repo / ".github/scripts/issue-version-triage.test.cjs")
        renamed = self.commit()
        self.assertEqual(self.classify(parser, renamed)["code"], "true")
        self.write(".github/scripts/issue-version-triage.cjs\ninternal/api/fixture.go", "// quoted filename\n")
        quoted = self.commit()
        self.assertEqual(self.classify(renamed, quoted)["code"], "true")

    def test_selector_failure_cannot_be_mistaken_for_a_narrow_change(self):
        self.write(".github/scripts/issue-version-triage.cjs", "// parser fixture\n")
        self.write("scripts/ci_triage_only.py", "raise SystemExit(42)\n")
        head = self.commit()
        self.assertEqual(self.classify(self.base, head)["code"], "true")

    def test_controls_and_workflow_edits_cannot_skip_their_own_runtime_tests(self):
        for path in ("scripts/ci_triage_only.py", "scripts/tests/test_ci_browser_proof_only.py",
                     ".github/workflows/build-and-test.yml"):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                (self.repo / path).parent.mkdir(parents=True, exist_ok=True)
                with (self.repo / path).open("a") as control:
                    control.write("\n# Changed CI control\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head)["code"], "true")
                self.assertEqual(self.classify(before, head)["triage_helper"], "true")

    def test_triage_path_lookalikes_and_unrelated_code_do_not_select_it(self):
        for path in (
            ".github/scripts/issue-version-triage.cjs.extra",
            "other/.github/scripts/package-lock.json", "docs/ISSUE_TRIAGE.md.extra",
            ".github/scripts/select-go-package-shard.py", "internal/api/other.go",
            ".github/scripts/issue-version-triage.cjs\nother.txt",
        ):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path, "unrelated fixture\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head)["triage_helper"], "false")


class WorkflowScopeTest(unittest.TestCase):
    def test_completion_skip_keeps_required_runtime_verdicts_honest(self):
        workflow = BUILD.read_text()
        frontend = step_script(job_block(workflow, "frontend"), "Require complete frontend proof")
        env = {**os.environ, "CHANGE_RESULT": "success", "CODE_CHANGED": "false",
               "BUILD_RESULT": "skipped", "CHECKS_RESULT": "skipped", "UNIT_RESULT": "skipped"}
        def run(command, variables):
            return subprocess.run(["bash", "-c", command], env=variables,
                                  capture_output=True, text=True, timeout=5)
        result = run(frontend, env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("no frontend tests ran", result.stdout)
        self.assertNotIn("passed", result.stdout)
        for key in ("CHANGE_RESULT", "CODE_CHANGED", "BUILD_RESULT", "CHECKS_RESULT", "UNIT_RESULT"):
            self.assertNotEqual(run(frontend, {**env, key: ""}).returncode, 0)
        for job in ("backend", "backend-api"):
            command = step_script(job_block(workflow, job), "Require compiled backend tests")
            admitted = {**os.environ, "CODE_CHANGED": "false", "COMPILE_REQUIRED": "false",
                        "COMPILE_RESULT": "skipped"}
            self.assertEqual(run(command, admitted).returncode, 0)
            for key in ("CODE_CHANGED", "COMPILE_REQUIRED", "COMPILE_RESULT"):
                self.assertNotEqual(run(command, {**admitted, key: ""}).returncode, 0)
        api = step_script(job_block(workflow, "backend-api-verdict"), "Require every internal/api shard")
        api_env = {**env, "API_SHARDS_RESULT": "success"}
        result = run(api, api_env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("no internal/api tests ran", result.stdout)
        self.assertNotIn("passed", result.stdout)
        for state in ("skipped", "failure", "cancelled", ""):
            self.assertNotEqual(run(api, {**api_env, "API_SHARDS_RESULT": state}).returncode, 0)

    def test_triage_check_runs_locked_real_tests_without_mutation_or_tolerant_exit(self):
        workflow = BUILD.read_text()
        self.assertIn("triage_helper: ${{ steps.filter.outputs.triage_helper }}",
                      job_block(workflow, "changes"))
        job = job_block(workflow, "triage-helper")
        for required in (
            "needs: changes", "if: needs.changes.outputs.triage_helper == 'true'",
            "runs-on: ubuntu-24.04", "timeout-minutes: 5", "contents: read",
            "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
            "persist-credentials: false",
            "actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1",
            "node-version: '24'", "package-manager-cache: false",
            "npm ci --ignore-scripts --no-audit --no-fund --prefix .github/scripts",
            "npm run test:triage --prefix .github/scripts",
        ):
            self.assertIn(required, job)
        for forbidden in ("secrets.", "write", "create-github-app-token", "github-script@",
                          "continue-on-error", "|| true", "issue-version-triage.cjs",
                          "workflow_call", "self-hosted"):
            self.assertNotIn(forbidden, job)
        self.assertLess(job.index("npm ci "), job.index("npm run test:triage"))
        # Hosted test admission adds coverage; it never replaces the separate
        # issue-event tests that must still precede the label write.
        live = (ROOT / ".github/workflows/issue-version-label-sync.yml").read_text()
        self.assertLess(live.index("npm run test:triage"), live.index("Sync issue version metadata"))

    def test_actual_triage_recipes_preserve_install_and_test_failures(self):
        job = job_block(BUILD.read_text(), "triage-helper")
        recipes = [part.split("        run: ", 1)[1].strip()
                   for part in re.split(r"(?m)^      - name: ", job)[1:]
                   if "        run: " in part]
        self.assertEqual(len(recipes), 2)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            npm = root / "npm"
            npm.write_text('#!/bin/sh\nprintf "%s\\n" "$*" > "$CALLS"\nexit "$RESULT"\n')
            npm.chmod(0o700)
            calls = root / "calls"
            env = {**os.environ, "PATH": str(root) + ":" + os.environ["PATH"],
                   "CALLS": str(calls)}
            for recipe, expected in zip(recipes, (
                "ci --ignore-scripts --no-audit --no-fund --prefix .github/scripts",
                "run test:triage --prefix .github/scripts",
            )):
                for status in (0, 42, 127):
                    with self.subTest(recipe=recipe, exit=status):
                        result = subprocess.run(["bash", "-c", recipe], cwd=root,
                                                capture_output=True, text=True, timeout=5,
                                                env={**env, "RESULT": str(status)})
                        self.assertEqual(result.returncode, status, result.stdout + result.stderr)
                        self.assertEqual(calls.read_text().strip(), expected)

    def test_scope_is_checked_before_runtime_and_preserves_required_governance(self):
        e2e = E2E.read_text()
        scope = job_block(e2e, "scope")
        self.assertIn("fetch-depth: 0", scope)
        self.assertIn("persist-credentials: false", scope)
        self.assertIn("timeout-minutes: 5", scope)
        self.assertIn('proof_only=$(python3 scripts/ci_browser_proof_only.py --base "$DIFF_BASE" --head "$DIFF_HEAD")', scope)
        self.assertNotIn("continue-on-error", scope)
        for name in ("tier-selection", "offline-org-provisioning"):
            job = job_block(e2e, name)
            self.assertIn("needs: scope", job)
            self.assertIn("if: needs.scope.outputs.proof_only != 'true'", job)
        for event in ("pull_request", "push"):
            block = re.split(r"(?m)^  \S", e2e.split(f"  {event}:\n", 1)[1], maxsplit=1)[0]
            for path in ("scripts/ci_browser_proof_only.py", "scripts/tests/test_ci_browser_proof_only.py"):
                self.assertIn(f"'{path}'", block)
        verdict = job_block(e2e, "e2e-verdict")
        self.assertIn("      - scope\n", verdict)
        self.assertIn("if: always()", verdict)
        self.assertIn("PROOF_ONLY: ${{ needs.scope.outputs.proof_only }}", verdict)
        self.assertNotIn("${{ needs.scope.outputs.proof_only }}", step_script(verdict, "Check E2E results"))
        build = BUILD.read_text()
        classifier = yaml.safe_load(job_block(build, "changes"))["steps"]
        classifier = next(step for step in classifier if step.get("id") == "filter")
        self.assertEqual(classifier["env"]["GOVERNANCE_TARGET_BRANCH"],
                         "${{ github.event_name == 'pull_request' && github.base_ref || github.ref_name }}")
        self.assertIn("python3 scripts/tests/test_ci_browser_proof_only.py", job_block(build, "changes"))
        self.assertNotIn("if:", job_block(build, "secret-scan"))
        governance = (ROOT / ".github/workflows/canonical-governance.yml").read_text()
        self.assertIn("--base", governance)
        self.assertIn("scripts/release_control/browser_verification_guard.py", governance)
        self.assertNotIn("ci_browser_proof_only", governance)

    def verdict(self, proof_only, results, scope="success"):
        script = step_script(job_block(E2E.read_text(), "e2e-verdict"), "Check E2E results")
        script = interpolate(script, {"needs.scope.result": scope,
                                      **{f"needs.{job}.result": value for job, value in results.items()}})
        return subprocess.run(["bash", "-c", script], capture_output=True, text=True,
                              timeout=5, env={**os.environ, "PROOF_ONLY": proof_only})

    def test_actual_verdict_names_receipt_skip_without_claiming_runtime_pass(self):
        result = self.verdict("true", dict.fromkeys(JOBS, "skipped"))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("not applicable", result.stdout)
        self.assertNotIn("passed", result.stdout)
        for job in JOBS:
            for state in ("success", "failure", "cancelled", ""):
                results = dict.fromkeys(JOBS, "skipped")
                results[job] = state
                with self.subTest(job=job, state=state):
                    self.assertNotEqual(self.verdict("true", results).returncode, 0)

    def test_actual_runtime_verdict_still_requires_every_existing_success(self):
        results = dict.fromkeys(JOBS, "success")
        self.assertEqual(self.verdict("false", results).returncode, 0)
        for job in JOBS:
            for state in ("failure", "cancelled", "skipped", ""):
                adverse = {**results, job: state}
                with self.subTest(job=job, state=state):
                    self.assertNotEqual(self.verdict("false", adverse).returncode, 0)

    def test_unknown_or_failed_scope_cannot_report_runtime_success(self):
        for proof in ("", "unknown", "true\nfalse"):
            self.assertNotEqual(self.verdict(proof, dict.fromkeys(JOBS, "success")).returncode, 0)
        for scope in ("failure", "cancelled", "skipped", ""):
            for proof, state in (("true", "skipped"), ("false", "success")):
                with self.subTest(scope=scope, proof=proof):
                    self.assertNotEqual(self.verdict(proof, dict.fromkeys(JOBS, state), scope).returncode, 0)


if __name__ == "__main__":
    unittest.main()
