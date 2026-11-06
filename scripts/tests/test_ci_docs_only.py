#!/usr/bin/env python3
"""Exercise documentation admission, its actual shell and owning workflow.

No application verification is waived: an isolated edit keeps all Public docs
suites, mirror/link checks, governance and secret scanning. Control changes,
uncovered branches, mixed trees and unavailable evidence keep application CI.
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


def classifier_script(source):
    marker = "      - name: Classify changed files\n"
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
        self.assertIn(SUITE.encode(), SCOPE.DOC_SUITES)
        self.assertTrue(SCOPE.docs_only_paths(list(SCOPE.DOC_SUITES)))
        self.assertFalse(SCOPE.docs_only_paths([]))
        for path in ("README.md", *GUIDES, "docs/help/nested-guide.md"):
            with self.subTest(path=path):
                self.assertTrue(SCOPE.docs_only_paths([SUITE.encode(), path.encode()]))

    def test_unreviewed_suites_controls_runtime_graphs_and_lookalikes_keep_ci(self):
        for path in (
            "scripts/tests/test_future_docs.py", "scripts/tests/test_ci_docs_only.py",
            "scripts/tests/test_public_docs_workflow.py", "scripts/tests/run.sh",
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
            for path in [*SCOPE.DOC_SUITES, *(p.encode() for p in ("README.md", *GUIDES))]:
                self.assertTrue(any(fnmatch.fnmatchcase(os.fsdecode(path), p) for p in patterns), path)
        self.assertIn("  push:\n    branches:\n      - main\n", triggers)
        self.assertIn("        shard: [0, 1, 2, 3]\n", source)
        self.assertIn("      fail-fast: false\n", source)
        self.assertIn('        run: scripts/tests/run.sh --docs-shard "${{ matrix.shard }}"\n', source)
        self.assertIn("    needs: recipes\n    if: ${{ always() }}\n", source)
        self.assertNotIn("continue-on-error:", source)
        for command in ("scripts/check_public_docs.py", "scripts/readme_entrypoints_test.py",
                        "scripts/check_docs_mirror.py", "scripts/tests/test_public_docs_workflow.py"):
            self.assertIn(f"        run: python3 {command}\n", source)
        self.assertIn('if [[ "$RECIPE_RESULT" != success ]]; then', source)
        self.assertIn('            exit 1\n', source)

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

    def classify(self, base, head, branch="main"):
        values = {"github.event_name": "pull_request", "github.event.pull_request.base.sha": base,
                  "github.event.before": base, "github.sha": head}
        script = re.sub(r"\$\{\{\s*(.*?)\s*\}\}", lambda m: values[m[1]],
                        classifier_script(BUILD.read_text()))
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "output"
            result = subprocess.run(["bash", "-c", script], cwd=self.repo, capture_output=True,
                                    text=True, timeout=10, env=dict(os.environ, GITHUB_OUTPUT=str(output),
                                    GOVERNANCE_TARGET_BRANCH=branch))
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            return dict(line.split("=", 1) for line in output.read_text().splitlines())

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

    def test_triage_guidance_still_selects_its_policy_check(self):
        self.write(SUITE, "# fixture\n")
        self.write("docs/ISSUE_TRIAGE.md", "# guidance fixture\n")
        docs = self.commit()
        self.assertEqual(self.classify(self.base, docs), {
            "code": "false", "go_compile": "false", "frontend_deps": "false", "triage_helper": "true",
        })


if __name__ == "__main__":
    unittest.main()
