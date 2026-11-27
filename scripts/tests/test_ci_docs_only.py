#!/usr/bin/env python3
"""Exercise documentation admission, its actual shell and owning workflow.

An isolated edit keeps all Public docs suites, mirror/link checks, governance
and secret scanning. The diagnostic recipes and exact mirrored Help pushes
also keep a required verdict. Control changes, uncovered branches, mixed trees
and unavailable evidence keep application CI.
"""

import fnmatch
import importlib.util
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/ci_docs_only.py"
BUILD = ROOT / ".github/workflows/build-and-test.yml"
DOCS = ROOT / ".github/workflows/public-docs.yml"
SPEC = importlib.util.spec_from_file_location("ci_docs_only", SCRIPT)
SCOPE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCOPE)
SUITE = "scripts/tests/test_oidc_session_docs.py"
GUIDES = ("docs/OIDC.md", "frontend-modern/public/docs/OIDC.md")
HELP_TEST = "frontend-modern/src/features/docs/__tests__/oidcSetupDocs.test.ts"
DIAGNOSTICS = (
    "scripts/tests/test_troubleshooting_logs.py",
    "scripts/tests/test_vm_disk_diagnostics.py",
)


def classifier_script(source, name="Classify changed files"):
    marker = f"      - name: {name}\n"
    if source.count(marker) != 1:
        raise AssertionError("expected one classification step")
    body = source.split(marker, 1)[1].split("\n      - name:", 1)[0]
    marker = "        run: |\n"
    if body.count(marker) != 1:
        raise AssertionError("expected literal classification command")
    lines = []
    for line in body.split(marker, 1)[1].splitlines():
        if line and not line.startswith("          "):
            break
        lines.append(line[10:] if line else "")
    return "\n".join(lines) + "\n"


class DocsScopeTest(unittest.TestCase):
    def test_scalar_reader_handles_step_metadata_without_executing_yaml(self):
        source = """      - name: Classify changed files
        id: filter
        env:
          FIXTURE: metadata
        run: |
          # Real shell comment.
          printf '%s\\n' actual

      # Not part of the shell command.
      - name: Another step
        run: exit 19
"""
        self.assertEqual(classifier_script(source),
                         "# Real shell comment.\nprintf '%s\\n' actual\n\n")
        for bad in ("", source.replace("        run: |", "        uses: action@fixture"),
                    source + source):
            with self.subTest(source=bad), self.assertRaises(AssertionError):
                classifier_script(bad)

    def test_exact_suites_and_document_paths(self):
        self.assertEqual(len(SCOPE.DOC_SUITES), len(set(SCOPE.DOC_SUITES)))
        for suite in (
            SUITE,
            "scripts/tests/test_alerting_outage_help_docs.py",
            "scripts/tests/test_availability_help_docs.py",
            "scripts/tests/test_paid_docker_help_docs.py",
            "scripts/tests/test_scheduler_health_api_help_docs.py",
            "scripts/tests/test_stale_reading_help_docs.py",
            "scripts/tests/test_watchdog_help_docs.py",
        ):
            self.assertIn(suite.encode(), SCOPE.DOC_SUITES)
        self.assertTrue(SCOPE.docs_only_paths(list(SCOPE.DOC_SUITES)))
        self.assertFalse(SCOPE.docs_only_paths([]))
        for path in ("README.md", *GUIDES, "docs/help/nested-guide.md"):
            with self.subTest(path=path):
                self.assertTrue(SCOPE.docs_only_paths([SUITE.encode(), path.encode()]))

    def test_unreviewed_suites_controls_runtime_graphs_and_lookalikes_keep_ci(self):
        for path in (
            "scripts/tests/test_future_docs.py", "scripts/tests/test_ci_docs_only.py",
            "scripts/tests/test_public_docs_workflow.py", "scripts/tests/run.sh",
            "scripts/run-script-tests.py", "scripts/run-docs-shards.py",
            "scripts/check_docs_mirror.py", "scripts/ci_docs_only.py",
            "scripts/select-smoke-test-shard.py", ".github/scripts/docs-test-seconds.txt",
            ".github/workflows/public-docs.yml", ".github/workflows/build-and-test.yml",
            ".github/workflows/canonical-governance.yml", "install.sh", "go.mod", "go.sum",
            "internal/api/oidc_handlers.go", "frontend-modern/src/Docs.tsx",
            "frontend-modern/package-lock.json", "frontend-modern/public/index.html",
            "docs/fixture.json", "another/scripts/tests/test_oidc_session_docs.py",
            "scripts/tests/nested/test_oidc_session_docs.py", SUITE + ".extra",
            SUITE + "\ninternal/api/source.go", "docs/../internal/source.md",
            "docs//OIDC.md", "docs/OIDC.md\x00internal/api/source.go",
        ):
            with self.subTest(path=path):
                self.assertFalse(SCOPE.docs_only_paths([SUITE.encode(), path.encode()]))

    def test_invalid_empty_failed_incomplete_and_unavailable_diff_keep_ci(self):
        for base, head in (("main", "a" * 40), ("a" * 40, "HEAD"), ("", "a" * 40)):
            with patch.object(SCOPE.subprocess, "run") as run:
                self.assertFalse(SCOPE.docs_only_diff(base, head))
                run.assert_not_called()
        for output, code in ((b"", 0), (SUITE.encode(), 0), (SUITE.encode() + b"\0", 1)):
            with patch.object(SCOPE.subprocess, "run", return_value=subprocess.CompletedProcess([], code, output)):
                self.assertFalse(SCOPE.docs_only_diff("a" * 40, "b" * 40))
        for error in (OSError("fixture unavailable"), subprocess.TimeoutExpired("git", 30)):
            with patch.object(SCOPE.subprocess, "run", side_effect=error):
                self.assertFalse(SCOPE.docs_only_diff("a" * 40, "b" * 40))

    def test_whole_null_delimited_diff_cannot_hide_late_source_or_rename(self):
        docs = SUITE.encode() + b"\0"
        for output, expected in ((docs, True), (docs * 4000 + b"install.sh\0", False),
                                 (docs + b"\0", False)):
            with patch.object(SCOPE.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, output)) as run:
                self.assertEqual(SCOPE.docs_only_diff("a" * 40, "b" * 40), expected)
                self.assertEqual(run.call_args.args[0], [
                    "git", "diff", "--no-ext-diff", "--no-renames", "--name-only", "-z",
                    "a" * 40, "b" * 40, "--",
                ])
                self.assertEqual(run.call_args.kwargs["timeout"], 30)


class OwningChecksTest(unittest.TestCase):
    def test_every_allowlisted_suite_is_discovered_by_current_docs_runner(self):
        # Execute the real runner with exit-only adapters, not a second copy of
        # its discovery/partition algorithm. Production suites run in VM proof.
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tests = root / "scripts/tests"
            tests.mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/tests/run.sh", tests / "run.sh")
            shutil.copyfile(ROOT / "scripts/run-script-tests.py", root / "scripts/run-script-tests.py")
            shutil.copyfile(ROOT / "scripts/select-smoke-test-shard.py",
                            root / "scripts/select-smoke-test-shard.py")
            weights = root / ".github/scripts"
            weights.mkdir(parents=True)
            shutil.copyfile(ROOT / ".github/scripts/docs-test-seconds.txt", weights / "docs-test-seconds.txt")
            for path in SCOPE.DOC_SUITES:
                relative = Path(os.fsdecode(path))
                self.assertTrue((ROOT / relative).is_file(), relative)
                (root / relative).write_text("import os\nfrom pathlib import Path\n"
                    "with Path(os.environ['DOC_CASE_LOG']).open('a') as log:\n"
                    "    log.write(Path(__file__).name + '\\n')\n")
            calls = root / "calls"
            env = dict(os.environ, DOC_CASE_LOG=str(calls))
            for shard in range(4):
                result = subprocess.run(["bash", str(tests / "run.sh"), "--docs-shard", str(shard)],
                                        cwd=root, env=env, text=True, capture_output=True, timeout=20)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            actual = calls.read_text().splitlines()
            expected = sorted(Path(os.fsdecode(path)).name for path in SCOPE.DOC_SUITES)
            self.assertEqual(sorted(actual), expected)
            self.assertEqual(len(actual), len(set(actual)))

    def test_owning_workflow_covers_both_events_and_complete_mandatory_shards(self):
        source = DOCS.read_text()
        triggers = source.split("\non:\n", 1)[1].split("\npermissions:", 1)[0]
        for event in ("pull_request", "push"):
            block = re.split(r"\n  [a-z_]+:\n", triggers.split(f"  {event}:\n", 1)[1], maxsplit=1)[0]
            patterns = re.findall(r'^      - "([^"]+)"$', block, re.MULTILINE)
            for path in [*SCOPE.DOC_SUITES, *SCOPE.DIAGNOSTIC_RECIPES,
                         b"scripts/run-docs-shards.py",
                         *(p.encode() for p in ("README.md", *GUIDES))]:
                self.assertTrue(any(fnmatch.fnmatchcase(os.fsdecode(path), p) for p in patterns), path)
        self.assertIn("  push:\n    branches:\n      - main\n", triggers)
        jobs = source.split("\njobs:\n", 1)[1]
        self.assertEqual(re.findall(r"(?m)^  ([a-z0-9_-]+):\s*$", jobs), ["check"])
        check = jobs.split("  check:\n", 1)[1]
        self.assertNotRegex(check, r"(?m)^    (?:if|needs|strategy):")
        self.assertNotIn("continue-on-error:", source)
        self.assertIn("        run: python3 scripts/run-docs-shards.py\n", check)
        for command in ("scripts/check_public_docs.py", "scripts/readme_entrypoints_test.py",
                        "scripts/check_docs_mirror.py", "scripts/tests/test_public_docs_workflow.py"):
            self.assertIn(f"        run: python3 {command}\n", check)

    def test_independent_secret_scan_and_governance_are_unchanged_gates(self):
        source = BUILD.read_text()
        scan = source.split("\n  secret-scan:\n", 1)[1].split("\n  changes:\n", 1)[0]
        self.assertNotRegex(scan, r"(?m)^    if:")
        self.assertIn('bash scripts/scan-ci-secrets.sh "$EXPECTED_HEAD" "$DIFF_BASE"', scan)
        self.assertNotRegex(source.split("\njobs:\n", 1)[0], r"(?m)^\s*paths(?:-ignore)?:")
        governance = (ROOT / ".github/workflows/canonical-governance.yml").read_text()
        self.assertNotRegex(governance.split("\njobs:\n", 1)[0], r"(?m)^\s*paths(?:-ignore)?:")
        self.assertNotRegex(governance.split("\n  governance:\n", 1)[1], r"(?m)^    if:")
        self.assertIn("canonical_completion_guard.py", governance)
        self.assertIn("browser_verification_guard", governance)

    def test_diagnostic_gate_is_an_unconditional_job_in_build_and_test(self):
        source = BUILD.read_text()
        changes = source.split("\n  changes:\n", 1)[1].split("\n  triage-helper:\n", 1)[0]
        self.assertIn("    name: Detect changed areas\n", changes)
        self.assertNotRegex(changes, r"(?m)^    (?:if|needs):|^\s*continue-on-error:")
        classifier = changes.split("      - name: Classify changed files\n", 1)[1].split(
            "\n      - name:", 1)[0]
        self.assertNotRegex(classifier, r"(?m)^        if:")
        self.assertNotRegex(source.split("\njobs:\n", 1)[0], r"(?m)^\s*paths(?:-ignore)?:")
        self.assertEqual(SCOPE.DIAGNOSTIC_RECIPES, {path.encode() for path in DIAGNOSTICS})
        self.assertTrue(SCOPE.DOC_SUITES.isdisjoint(SCOPE.DIAGNOSTIC_RECIPES))


class GitClassifierTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.repo = Path(self.temporary.name)
        self.git("init", "-q", "--initial-branch=main")
        for name in ("ci_docs_only.py", "ci_browser_proof_only.py", "ci_triage_only.py",
                     "ci_completion_only.py", "go-test-compile-required.sh"):
            target = self.repo / "scripts" / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / "scripts" / name, target)
        self.write("internal/api/fixture.go", "package fixture\n")
        self.base = self.commit()

    def git(self, *args):
        return subprocess.check_output([
            "git", "-c", "user.name=CI fixture", "-c", "user.email=ci@example.invalid",
            "-c", "core.hooksPath=/dev/null", *args,
        ], cwd=self.repo, text=True, stderr=subprocess.PIPE).strip()

    def write(self, name, source):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(source)

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "Fixture tree change")
        return self.git("rev-parse", "HEAD")

    def run_help_step(self, name, **env):
        script = classifier_script(BUILD.read_text(), name)
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "output"
            result = subprocess.run(["bash", "-c", script], cwd=self.repo, capture_output=True,
                                    text=True, timeout=10, env=dict(os.environ, GITHUB_OUTPUT=str(output),
                                    **env))
            return result, output.read_text() if output.exists() else ""

    def help_scope(self, base, head):
        result, output = self.run_help_step("Identify a mirrored Help-only main push", BASE_SHA=base, TESTED_SHA=head)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return output

    def add_help_fixture(self):
        for guide in GUIDES:
            self.write(guide, "# Existing mirrored Help\n")
        self.write(HELP_TEST, "// Existing Help assertion\n")
        return self.commit()

    def update_help_fixture(self):
        for guide in GUIDES:
            self.write(guide, "# Updated mirrored Help\n")
        self.write(HELP_TEST, "// Updated Help assertion\n")

    def run_classifier(self, base, head, branch="main", event="pull_request"):
        values = {"github.event_name": event, "github.event.pull_request.base.sha": base,
                  "github.event.before": base, "github.sha": head}
        script = re.sub(r"\$\{\{\s*(.*?)\s*\}\}", lambda m: values[m[1]],
                        classifier_script(BUILD.read_text()))
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "output"
            result = subprocess.run(["bash", "-c", script], cwd=self.repo, capture_output=True,
                                    text=True, timeout=10, env=dict(os.environ, GITHUB_OUTPUT=str(output),
                                    GOVERNANCE_TARGET_BRANCH=branch))
            fields = dict(line.split("=", 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return result, fields

    def classify(self, base, head, branch="main", event="pull_request"):
        result, fields = self.run_classifier(base, head, branch, event)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return fields

    def add_recipe_fixtures(self):
        # Exercise the actual classifier command and runner using stdlib-only
        # exit adapters. Separate source proof runs both real recipe suites.
        for path in ("scripts/tests/run.sh", "scripts/run-script-tests.py"):
            dest = self.repo / path
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy(ROOT / path, dest)
        for recipe in DIAGNOSTICS:
            self.write(recipe, "print('recipe observed')\n")
        return self.commit()

    def test_each_isolated_diagnostic_edit_requires_both_recipes(self):
        start = self.add_recipe_fixtures()
        for recipe in DIAGNOSTICS:
            with self.subTest(recipe=recipe):
                self.write(recipe, "print('updated recipe observed')\n")
                changed = self.commit()
                for event in ("pull_request", "push"):
                    result, fields = self.run_classifier(start, changed, event=event)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(fields, dict.fromkeys(
                        ("code", "frontend_deps", "go_compile", "triage_helper"), "false"))
                    self.assertIn("Summary: 2/2 passed", result.stdout)
                    self.assertEqual([line[4:] for line in result.stdout.splitlines()
                                      if line.startswith("==> ")], list(DIAGNOSTICS))
                start = changed
        both = self.git("rev-parse", "HEAD~2")
        for branch in ("release/v6.4", "pulse/v6-release"):
            result, fields = self.run_classifier(both, changed, branch=branch)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(fields["code"], "true")
            self.assertNotIn("recipe observed", result.stdout)
        result, fields = self.run_classifier(both, changed)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(fields["code"], "false")
        self.assertIn("Summary: 2/2 passed", result.stdout)

    def test_direct_main_push_with_mirrored_help_and_existing_tests_selects_focus(self):
        base = self.add_help_fixture()
        self.update_help_fixture()
        head = self.commit()
        self.assertEqual(self.help_scope(base, head), "selected=true\n")
        # Ordinary classifier outputs do not themselves suppress the full
        # suite. Only the main-push step can add a focused verdict.
        self.assertEqual(self.classify(base, head), {
            "code": "true", "go_compile": "false", "frontend_deps": "false", "triage_helper": "false",
        })
        for wrong_base in (self.base, head, "0" * 40, "f" * 40, "main"):
            with self.subTest(base=wrong_base):
                self.assertEqual(self.help_scope(wrong_base, head), "")
        self.write("docs/another.md", "# Other edit\n")
        self.commit()
        self.assertEqual(self.help_scope(base, head), "")  # tested checkout does not match

    def test_protected_merge_compares_the_first_parent_not_the_topic(self):
        base = self.add_help_fixture()
        self.update_help_fixture()
        topic = self.commit()
        merge = self.git("commit-tree", f"{topic}^{{tree}}", "-p", base, "-p", topic,
                         "-m", "Merge the verified Help change")
        self.git("reset", "--hard", merge)
        self.assertEqual(self.help_scope(base, merge), "selected=true\n")
        self.assertEqual(self.help_scope(topic, merge), "")

    def test_only_exact_completed_help_command_emits_the_proof_marker(self):
        head = self.add_help_fixture()
        fake = self.repo / "bin/npm"
        self.write("bin/npm", '#!/bin/sh\n'
                   'printf "%s\\n" "$*"\n'
                   'test "$*" != "$NPM_FAIL_COMMAND"\n')
        fake.chmod(0o755)
        env = dict(PATH=f"{fake.parent}:{os.environ['PATH']}", EXPECTED_SOURCE_SHA=head, NPM_FAIL_COMMAND="")
        name = "Check Help test source and rendered guides"
        result, output = self.run_help_step(name, **env)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        commands = [
            "ci --no-audit", "run format:check", "run lint", "run lint:headers",
            "run lint:cpd", "run type-check",
            "run test -- src/features/docs/__tests__ src/utils/__tests__/docsLinks.test.ts",
        ]
        self.assertEqual(result.stdout.splitlines(), commands)
        self.assertEqual(output, "completed=true\n")
        for index, command in enumerate(commands):
            with self.subTest(command=command):
                result, output = self.run_help_step(name, **{**env, "NPM_FAIL_COMMAND": command})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout.splitlines(), commands[:index + 1])
                self.assertEqual(output, "")
        result, output = self.run_help_step(name, **{**env, "EXPECTED_SOURCE_SHA": self.base})
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(result.stdout, "")
        self.assertEqual(output, "")

    def test_help_selector_preserves_ordinary_ci_for_mixed_and_nonregular_changes(self):
        for path in ("frontend-modern/src/features/docs/docMarkdown.ts",
                     "frontend-modern/src/features/docs/__tests__/guideFixture.ts",
                     "frontend-modern/package-lock.json", "internal/api/fixture.go",
                     "install.sh", ".github/workflows/build-and-test.yml",
                     "docs/release-control/spec.md",
                     HELP_TEST.replace(".test.ts", "\nquoted.test.ts")):
            with self.subTest(path=path):
                self.git("reset", "--hard", self.base)
                base = self.add_help_fixture()
                self.update_help_fixture()
                self.write(path, "// Changed additional path\n")
                self.assertEqual(self.help_scope(base, self.commit()), "")

        for change in ("chmod", "symlink", "rename", "delete", "add", "one-sided", "different", "test-only"):
            with self.subTest(change=change):
                self.git("reset", "--hard", self.base)
                base = self.add_help_fixture()
                self.update_help_fixture()
                test = self.repo / HELP_TEST
                if change == "chmod":
                    test.chmod(0o755)
                elif change == "symlink":
                    test.unlink()
                    test.symlink_to("elsewhere.test.ts")
                elif change == "rename":
                    self.git("config", "diff.renames", "true")
                    test.rename(test.with_name("replacement.test.ts"))
                elif change == "delete":
                    test.unlink()
                elif change == "add":
                    test.with_name("new.test.ts").write_text("// New test\n")
                elif change == "one-sided":
                    self.write(GUIDES[1], "# Existing mirrored Help\n")
                elif change == "different":
                    self.write(GUIDES[1], "# Different Help\n")
                elif change == "test-only":
                    for guide in GUIDES:
                        self.write(guide, "# Existing mirrored Help\n")
                self.assertEqual(self.help_scope(base, self.commit()), "")

    def test_failed_or_missing_recipe_cannot_return_a_passing_required_result(self):
        start = self.add_recipe_fixtures()
        for recipe in DIAGNOSTICS:
            with self.subTest(recipe=recipe):
                self.write(recipe, "raise SystemExit(7)\n")
                failed = self.commit()
                result, fields = self.run_classifier(start, failed)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn(f"{recipe} (exit 7)", result.stdout)
                self.assertIn("Summary: 1/2 passed", result.stdout)
                self.assertNotIn("code", fields)

                (self.repo / recipe).unlink()
                missing = self.commit()
                result, fields = self.run_classifier(failed, missing)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("Required diagnostic recipe is missing", result.stdout)
                self.assertNotIn("code", fields)
                self.write(recipe, "print('recipe observed')\n")
                start = self.commit()

    def test_diagnostics_mixed_tree_and_wrong_checkout_cannot_select_shortcut(self):
        base = self.add_recipe_fixtures()
        self.write(DIAGNOSTICS[0], "raise SystemExit(7)\n")
        isolated = self.commit()
        self.write("docs/guide.md", "# Also changed\n")
        mixed = self.commit()
        result, fields = self.run_classifier(base, mixed)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(fields["code"], "true")
        self.assertNotIn("==> ", result.stdout)
        result, fields = self.run_classifier(base, isolated)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checkout does not match the admitted source", result.stdout)
        self.assertNotIn("code", fields)

    def test_actual_classifier_accepts_isolated_add_edit_delete_and_guides(self):
        self.write(SUITE, "# source-consistency fixture\n")
        added = self.commit()
        for guide in GUIDES:
            self.write(guide, "# Guide fixture\n")
        documented = self.commit()
        for start, end in ((self.base, added), (added, documented), (self.base, documented)):
            self.assertEqual(self.classify(start, end), {
                "code": "false", "go_compile": "false", "frontend_deps": "false", "triage_helper": "false",
            })
        self.write(SUITE, "# changed consistency fixture\n")
        edited = self.commit()
        self.assertEqual(self.classify(documented, edited)["code"], "false")
        (self.repo / SUITE).unlink()
        deleted = self.commit()
        self.assertEqual(self.classify(edited, deleted)["code"], "false")
        self.assertEqual(self.classify(deleted, deleted)["code"], "true")

    def test_https_recipe_and_mirrored_guides_use_public_docs_on_main(self):
        for path in ("docs/CONFIGURATION.md", "frontend-modern/public/docs/CONFIGURATION.md",
                     "scripts/tests/test_server_https_docs.py"):
            self.write(path, "# HTTPS guide or recipe fixture\n")
        documented = self.commit()
        for event in ("pull_request", "push"):
            with self.subTest(event=event):
                self.assertEqual(self.classify(self.base, documented, event=event), {
                    "code": "false", "go_compile": "false", "frontend_deps": "false",
                    "triage_helper": "false",
                })
        self.assertEqual(self.classify(self.base, documented, branch="release/v6.4")["code"],
                         "true")

        self.write("install.sh", "# Changed installer\n")
        mixed = self.commit()
        self.assertEqual(self.classify(self.base, mixed)["code"], "true")

    def test_mixed_source_or_dependency_and_entire_range_keep_runtime(self):
        for path, field in (("internal/api/fixture.go", "go_compile"),
                            ("frontend-modern/package-lock.json", "frontend_deps"),
                            ("frontend-modern/src/Docs.tsx", "code"), ("install.sh", "code")):
            with self.subTest(path=path):
                base = self.git("rev-parse", "HEAD")
                self.write(path, "// changed source\n")
                source = self.commit()
                self.write(SUITE, path + "\n")
                docs = self.commit()
                self.assertEqual(self.classify(source, docs)["code"], "false")
                actual = self.classify(base, docs)
                self.assertEqual(actual["code"], "true")
                self.assertEqual(actual[field], "true")

    def test_uncovered_branches_controls_and_new_suites_keep_runtime(self):
        self.write(SUITE, "# fixture\n")
        docs = self.commit()
        for branch in ("release/v6.4", "pulse/v6-release", "main-extra", "", "other/main"):
            with self.subTest(branch=branch):
                self.assertEqual(self.classify(self.base, docs, branch)["code"], "true")
        for path in ("scripts/tests/test_future_docs.py", "scripts/tests/test_ci_docs_only.py",
                     "scripts/tests/test_public_docs_workflow.py", "scripts/ci_docs_only.py",
                     "scripts/run-docs-shards.py",
                     ".github/workflows/public-docs.yml", ".github/workflows/build-and-test.yml"):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path, (self.repo / path).read_text() + "\n# Changed control\n"
                           if (self.repo / path).exists() else "# Changed control\n")
                changed = self.commit()
                self.assertEqual(self.classify(before, changed)["code"], "true")

    def test_missing_base_failed_or_misleading_selector_keeps_runtime(self):
        self.write(SUITE, "# fixture\n")
        docs = self.commit()
        self.assertEqual(self.classify("0" * 40, docs), {
            "code": "true", "go_compile": "true", "frontend_deps": "true", "triage_helper": "true",
        })
        for program in ("raise SystemExit(42)\n", "print('true')\nraise SystemExit(42)\n",
                        "print('true\\nfalse')\n"):
            with self.subTest(program=program):
                self.write("scripts/ci_docs_only.py", program)
                # Deliberately exercise a failed installed selector while its
                # tree diff is docs-only. A failure after 'true' must not skip.
                self.assertEqual(self.classify(self.base, docs)["code"], "true")

    def test_rename_and_quoted_filename_cannot_hide_source(self):
        self.write(SUITE, "# fixture\n")
        docs = self.commit()
        (self.repo / "internal/api/fixture.go").rename(self.repo / "scripts/tests/test_agent_docs.py")
        renamed = self.commit()
        self.assertEqual(self.classify(docs, renamed)["code"], "true")
        self.write(SUITE + '\n"source.go', "# quoted fixture\n")
        quoted = self.commit()
        self.assertEqual(self.classify(renamed, quoted)["code"], "true")

    def test_moves_to_docs_keep_the_removed_path_in_the_runtime_checks(self):
        self.git("config", "diff.renames", "true")
        for old, new, go_compile, frontend_deps in (
            ("internal/api/fixture.go", "docs/backend.md", "true", "false"),
            ("frontend-modern/package-lock.json", "docs/dependencies.md", "false", "true"),
        ):
            self.write(old, "unchanged fixture bytes for " + old + "\n")
            before = self.commit()
            destination = self.repo / new
            destination.parent.mkdir(exist_ok=True)
            (self.repo / old).rename(destination)
            after = self.commit()
            for branch, event in (("main", "pull_request"), ("main", "push"),
                                  ("release/v6.4", "pull_request")):
                with self.subTest(old=old, branch=branch, event=event):
                    self.assertEqual(self.classify(before, after, branch, event), {
                        "code": "true", "go_compile": go_compile,
                        "frontend_deps": frontend_deps, "triage_helper": "false",
                    })

    def test_moves_between_guides_keep_documentation_only_checks(self):
        self.git("config", "diff.renames", "true")
        self.write("docs/before.md", "# Guide fixture\n")
        before = self.commit()
        (self.repo / "docs/before.md").rename(self.repo / "docs/after.md")
        after = self.commit()
        self.assertEqual(self.classify(before, after), {
            "code": "false", "go_compile": "false", "frontend_deps": "false",
            "triage_helper": "false",
        })

    def test_triage_guidance_still_selects_its_policy_check(self):
        self.write(SUITE, "# fixture\n")
        self.write("docs/ISSUE_TRIAGE.md", "# guidance fixture\n")
        docs = self.commit()
        self.assertEqual(self.classify(self.base, docs), {
            "code": "false", "go_compile": "false", "frontend_deps": "false", "triage_helper": "true",
        })


if __name__ == "__main__":
    unittest.main()
