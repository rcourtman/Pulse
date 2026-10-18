#!/usr/bin/env python3
"""Execute receipt-only diff admission and the workflows' actual verdict paths."""

import importlib.util
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/ci_browser_proof_only.py"
BUILD = ROOT / ".github/workflows/build-and-test.yml"
E2E = ROOT / ".github/workflows/test-e2e.yml"
SPEC = importlib.util.spec_from_file_location("ci_browser_proof_only", SCRIPT)
SCOPE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCOPE)
RECEIPT = "frontend-modern/browser-verification/2026-10-14-proof.json"
JOBS = ("tier-selection", "test-images", "e2e", "agent-registration")


def job_block(workflow, name):
    match = re.search(rf"^  {re.escape(name)}:\n(.*?)(?=^  [\w-]+:\n|\Z)",
                      workflow, re.M | re.S)
    if match is None:
        raise AssertionError(f"missing workflow job {name}")
    return match.group(1)


def step_script(block, name):
    step = re.split(r"(?m)^      - name: ", block)[1:]
    selected = next(part for part in step if part.startswith(name + "\n"))
    return "\n".join(line[10:] for line in selected.split("        run: |\n", 1)[1].splitlines())


def interpolate(script, expressions):
    return re.sub(r"\$\{\{\s*(.*?)\s*\}\}", lambda m: expressions[m[1]], script)


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


class GitTreeScopeTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.git("init", "-q", "--initial-branch=main")
        for name in ("scripts/ci_browser_proof_only.py", "scripts/go-test-compile-required.sh"):
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

    def classify(self, base, head):
        script = step_script(job_block(BUILD.read_text(), "changes"), "Classify changed files")
        script = interpolate(script, {
            "github.event_name": "pull_request", "github.event.pull_request.base.sha": base,
            "github.event.before": base, "github.sha": head,
        })
        output = self.repo / "outputs"
        if output.exists():
            output.unlink()
        result = subprocess.run(["bash", "-c", script], cwd=self.repo, capture_output=True,
                                text=True, timeout=10, env={**os.environ, "GITHUB_OUTPUT": str(output)})
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

    def test_actual_classifier_selects_every_triage_input_and_policy_consumer(self):
        for path in (
            ".github/scripts/issue-version-triage.cjs",
            ".github/scripts/issue-version-triage.test.cjs",
            ".github/scripts/triage-comment-policy.test.cjs",
            ".github/scripts/package.json", ".github/scripts/package-lock.json",
            ".github/ISSUE_TEMPLATE/bug_report.yml", ".github/ISSUE_TEMPLATE/config.yml",
            ".github/workflows/issue-version-label-sync.yml",
            ".github/workflows/build-and-test.yml", ".github/workflows/policy-consumer.yaml",
            "docs/ISSUE_TRIAGE.md", "scripts/tests/test_ci_browser_proof_only.py",
        ):
            with self.subTest(path=path):
                before = self.git("rev-parse", "HEAD")
                self.write(path, "triage input fixture\n")
                head = self.commit()
                self.assertEqual(self.classify(before, head)["triage_helper"], "true")

    def test_triage_admission_uses_complete_range_and_does_not_expand_runtime_skip(self):
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
            "code": "true", "frontend_deps": "false", "go_compile": "false",
            "triage_helper": "true",
        })
        self.write("frontend-modern/package-lock.json", "{}\n")
        graph = self.commit()
        combined = self.classify(proof, graph)
        self.assertEqual(combined["frontend_deps"], "true")
        self.assertEqual(combined["code"], "true")
        self.assertEqual(combined["triage_helper"], "true")

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
        # issue-event tests that must still precede credential acquisition.
        live = (ROOT / ".github/workflows/issue-version-label-sync.yml").read_text()
        self.assertLess(live.index("npm run test:triage"), live.index("Mint triage bot token"))

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
