#!/usr/bin/env python3
"""Execute early CI admission, exact Git traversal and its unchanged consumers."""

from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
CONTROL = "scripts/release_control/"
WORKFLOWS = ("build-and-test.yml", "test-e2e.yml", "canonical-governance.yml")


def workflow(name):
    return yaml.safe_load((ROOT / ".github/workflows" / name).read_text())


def early_step(name):
    job = {"build-and-test.yml": "changes", "test-e2e.yml": "scope", "canonical-governance.yml": "governance"}[name]
    return next(s for s in workflow(name)["jobs"][job]["steps"]
                if s.get("name") == "Check canonical source before dependency work")


class SourceAdmissionFixture(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name) / "pulse"
        self.repo.mkdir()
        self.calls = Path(self.temp.name) / "calls.jsonl"
        self.installs = Path(self.temp.name) / "dependency-installs"
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        self.env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
                        GIT_AUTHOR_NAME="Admission fixture", GIT_AUTHOR_EMAIL="ci@example.invalid",
                        GIT_COMMITTER_NAME="Admission fixture", GIT_COMMITTER_EMAIL="ci@example.invalid",
                        CALLS=str(self.calls), HISTORY_STATUS="3", CONTRACT_STATUS="0",
                        PULSE_ALLOW_CONTRACT_NEUTRAL_COMMIT="ambient must not bypass")
        for name in ("check_source_contracts.py", "repo_file_io.py", "browser_verification_guard.py", "format_staged_frontend.py"):
            self.write(CONTROL + name, (ROOT / CONTROL / name).read_text())
        recorder = (
            "import json,os,sys\n"
            "with open(os.environ['CALLS'],'a') as out:\n"
            " out.write(json.dumps({'guard':__file__.rsplit('/',1)[-1],'argv':sys.argv[1:],"
            "'reason':os.environ.get('PULSE_ALLOW_CONTRACT_NEUTRAL_COMMIT',''),"
            "'files':sys.stdin.read().splitlines()})+'\\n')\n"
        )
        self.write(CONTROL + "canonical_completion_history.py", recorder + "sys.exit(int(os.environ['HISTORY_STATUS']))\n")
        self.write(CONTROL + "canonical_completion_guard.py", recorder + "sys.exit(int(os.environ['CONTRACT_STATUS']))\n")
        self.write("README.md", "Fixture\n")
        self.write("internal/example.go", "package example\n")
        self.git("init", "--quiet", "--initial-branch=main")
        self.base = self.commit("base")

    def write(self, name, text):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def git(self, *args):
        result = subprocess.run(["git", *args], cwd=self.repo, env=self.env,
                                capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, f"fixture git {args[0]}: {result.stderr}")
        return result.stdout.strip()

    def commit(self, message="candidate", trailer=None):
        self.git("add", "--all")
        args = ["commit", "--quiet", "--allow-empty", "-m", message]
        if trailer is not None:
            args += ["-m", "Contract-Neutral: " + trailer]
        self.git(*args)
        return self.git("rev-parse", "HEAD")

    def changed(self, path="internal/example.go", trailer=None):
        self.write(path, "candidate content\n")
        return self.commit(trailer=trailer)

    def run_guard(self, *, base=None, head=None, early=False):
        args = ["python3", CONTROL + "check_source_contracts.py", "--base", base or self.base,
                "--head", head or self.git("rev-parse", "HEAD")]
        if early:
            args += ["--early"]
        return subprocess.run(args, cwd=self.repo, env=self.env, capture_output=True, text=True, timeout=20)

    def recorded(self):
        return [json.loads(line) for line in self.calls.read_text().splitlines()] if self.calls.exists() else []

    def run_early_workflow(self, name, event="pull_request", base=None):
        result = subprocess.run(
            ["bash", "-c", early_step(name)["run"] + '\nprintf "installed\\n" >> "$INSTALLS"\n'],
            cwd=self.repo, env={**self.env, "SOURCE_EVENT": event,
                "SOURCE_BASE": base if base is not None else self.base,
                "SOURCE_HEAD": self.git("rev-parse", "HEAD"), "INSTALLS": str(self.installs)},
            capture_output=True, text=True, timeout=20,
        )
        return result


class ContractRangeTest(SourceAdmissionFixture):
    def test_each_commit_uses_its_own_parent_files_and_trailer(self):
        first = self.changed(trailer="fixture has no contract delta")
        self.write("README.md", "second delta\n")
        second = self.commit()
        result = self.run_guard()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        history, guard, later_history, later_guard = self.recorded()
        self.assertEqual(history["reason"], "")
        self.assertEqual(later_history["reason"], "")
        self.assertEqual(guard["argv"], ["--files-from-stdin", "--diff-base", self.base, "--commit", first])
        self.assertEqual(guard["reason"], "fixture has no contract delta")
        self.assertEqual(guard["files"], ["internal/example.go"])
        self.assertEqual(later_guard["argv"], ["--files-from-stdin", "--diff-base", first, "--commit", second])
        self.assertEqual(later_guard["reason"], "")
        self.assertEqual(later_guard["files"], ["README.md"])
        self.assertEqual(history["argv"], ["--commit", first, "--head", second])

    def test_full_range_does_not_hide_invalid_ancestor_behind_receipt_tip(self):
        self.changed()
        self.write("frontend-modern/browser-verification/new.json", "{}\n")
        self.commit()
        self.env["CONTRACT_STATUS"] = "1"
        result = self.run_guard(early=True)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(len(self.recorded()), 4)

    def test_registered_completion_is_checked_and_not_directly_rejected_again(self):
        self.changed()
        self.env["HISTORY_STATUS"] = "0"
        self.assertEqual(self.run_guard().returncode, 0)
        self.assertEqual([c["guard"] for c in self.recorded()], ["canonical_completion_history.py"])

    def test_failed_history_never_falls_back_to_an_override(self):
        self.changed(trailer="not permission to ignore an invalid completion")
        self.env["HISTORY_STATUS"] = "1"
        self.assertEqual(self.run_guard().returncode, 1)
        self.assertEqual(len(self.recorded()), 1)
        self.assertEqual(self.recorded()[0]["reason"], "")

    def test_missing_guard_process_is_a_failure(self):
        self.changed()
        (self.repo / CONTROL / "canonical_completion_history.py").unlink()
        self.assertEqual(self.run_guard().returncode, 1)

    def test_unavailable_base_does_not_fall_back_to_only_the_tip(self):
        self.changed()
        self.assertEqual(self.run_guard(base="b" * 40).returncode, 2)
        self.assertEqual(self.recorded(), [])

    def test_ref_names_option_like_inputs_and_zero_base_fail_closed(self):
        self.changed()
        for base in ("main", "HEAD^", "--all", "0" * 40):
            with self.subTest(base=base):
                self.assertEqual(self.run_guard(base=base).returncode, 2)
        self.assertEqual(self.recorded(), [])

    def test_a_different_checkout_cannot_answer_for_the_requested_head(self):
        self.changed()
        result = self.run_guard(head=self.base)
        self.assertEqual(result.returncode, 2)
        self.assertIn("checked-out source", result.stderr)
        self.assertEqual(self.recorded(), [])

    def test_unrelated_history_cannot_supply_a_range(self):
        self.git("checkout", "--quiet", "--orphan", "unrelated")
        unrelated = self.commit("unrelated root")
        self.assertEqual(self.run_guard(head=unrelated).returncode, 2)
        self.assertEqual(self.recorded(), [])

    def test_empty_known_range_is_valid_and_does_not_check_an_ancestor(self):
        self.assertEqual(self.run_guard().returncode, 0)
        self.assertEqual(self.recorded(), [])

    def test_early_frontend_deferral_is_explicit_but_full_gate_still_checks(self):
        self.changed("frontend-modern/src/example.ts")
        self.env["CONTRACT_STATUS"] = "1"
        early = self.run_guard(early=True)
        self.assertEqual(early.returncode, 0)
        self.assertIn("defer early admission", early.stdout)
        self.assertEqual(self.recorded(), [])
        self.assertEqual(self.run_guard().returncode, 1)
        self.assertEqual(len(self.recorded()), 2)

    def test_reverted_frontend_source_still_needs_locked_formatter(self):
        path = "frontend-modern/src/example.ts"
        self.changed(path)
        (self.repo / path).unlink()
        self.commit("revert")
        result = self.run_guard(early=True)
        self.assertEqual(result.returncode, 0)
        self.assertIn("defer early admission", result.stdout)
        self.assertEqual(self.recorded(), [])

    def test_merge_only_frontend_resolution_is_deferred_not_treated_as_no_source(self):
        self.git("checkout", "--quiet", "-b", "side")
        self.write("side.md", "side\n")
        self.commit()
        self.git("checkout", "--quiet", "main")
        self.write("main.md", "main\n")
        self.commit()
        self.git("merge", "--quiet", "--no-ff", "--no-commit", "side")
        self.write("frontend-modern/src/merge.ts", "export const resolution = true;\n")
        self.commit("merge resolution")
        result = self.run_guard(early=True)
        self.assertEqual(result.returncode, 0)
        self.assertIn("defer early admission", result.stdout)
        self.assertEqual(self.recorded(), [])

    def test_receipt_rules_run_actual_browser_guard_not_an_assumed_skip(self):
        self.changed("frontend-modern/browser-verification.json")
        result = self.run_guard(early=True)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("BLOCKED", result.stderr)


class WorkflowAdmissionTest(SourceAdmissionFixture):
    def test_actual_early_steps_prevent_dependency_work_on_contract_failure(self):
        self.changed()
        self.env["CONTRACT_STATUS"] = "1"
        for name in WORKFLOWS:
            with self.subTest(workflow=name):
                result = self.run_early_workflow(name)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertFalse(self.installs.exists())

    def test_actual_early_steps_allow_valid_nonfrontend_source_to_continue(self):
        self.changed()
        for name in WORKFLOWS:
            with self.subTest(workflow=name):
                result = self.run_early_workflow(name)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.installs.read_text().splitlines(), ["installed"] * 3)

    def test_manual_dispatch_uses_exact_head_parent_not_a_missing_push_base(self):
        head = self.changed()
        result = self.run_early_workflow("build-and-test.yml", "workflow_dispatch", "")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.recorded()[1]["argv"], ["--files-from-stdin", "--diff-base", self.base, "--commit", head])

    def test_full_manual_governance_range_resolves_exact_commits(self):
        head = self.changed()
        steps = workflow("canonical-governance.yml")["jobs"]["governance"]["steps"]
        diff = next(s for s in steps if s.get("name") == "Determine governance diff range")
        command = diff["run"]
        for expression, value in (("github.event_name", "workflow_dispatch"),
                                  ("github.event.pull_request.base.sha", ""),
                                  ("github.event.before", ""), ("github.sha", head)):
            command = command.replace("${{ " + expression + " }}", value)
        output = Path(self.temp.name) / "diff-output"
        result = subprocess.run(["bash", "-c", command], cwd=self.repo,
                                env={**self.env, "GITHUB_OUTPUT": str(output)},
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(output.read_text().strip(), "range=" + self.base + "..." + head)
        full = next(s for s in steps if s.get("name") == "Run canonical completion guard against changed commits")
        result = subprocess.run(["bash", "-c", full["run"]], cwd=self.repo,
                                env={**self.env, "WORKFLOW_OUTPUT_1": self.base + "..." + head},
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_missing_push_base_stops_instead_of_claiming_an_empty_change(self):
        self.changed()
        result = self.run_early_workflow("test-e2e.yml", "push", "")
        self.assertEqual(result.returncode, 2)
        self.assertFalse(self.installs.exists())

    def test_retired_receipt_blocks_before_install_and_runtime_verdict(self):
        self.changed("frontend-modern/browser-verification.json")
        result = self.run_early_workflow("test-e2e.yml")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(self.installs.exists())

    def test_all_dependency_fanout_transitively_depends_on_admission(self):
        for name, root, independent in (
            ("build-and-test.yml", "changes", {"secret-scan"}),
            ("test-e2e.yml", "scope", set()),
        ):
            jobs = workflow(name)["jobs"]
            def depends(job):
                if job == root:
                    return True
                needs = jobs[job].get("needs", [])
                if isinstance(needs, str):
                    needs = [needs]
                return any(depends(parent) for parent in needs)
            for job in set(jobs) - independent:
                with self.subTest(workflow=name, job=job):
                    self.assertTrue(depends(job))
            admission = jobs[root]
            names = [s.get("name", "") for s in admission["steps"]]
            self.assertEqual(names[1], "Check canonical source before dependency work")
            self.assertEqual(admission["steps"][0]["with"]["fetch-depth"], 0)
            self.assertFalse(admission["steps"][0]["with"]["persist-credentials"])

    def test_secret_scan_and_full_canonical_checks_remain_independent_and_required(self):
        build = workflow("build-and-test.yml")["jobs"]
        self.assertNotIn("needs", build["secret-scan"])
        canonical = workflow("canonical-governance.yml")["jobs"]["governance"]
        steps = canonical["steps"]
        names = [s.get("name", "") for s in steps]
        self.assertLess(names.index("Check canonical source before dependency work"), names.index("Set up Go"))
        self.assertLess(names.index("Install locked frontend governance dependencies"), names.index("Run canonical completion guard against changed commits"))
        for expected in ("Validate final frontend browser evidence", "Run canonical mutation registry audits", "Run registry audit", "Run contract audit", "Run early source admission tests"):
            self.assertIn(expected, names)
        full = next(s for s in steps if s.get("name") == "Run canonical completion guard against changed commits")
        self.assertNotIn("--early", full["run"])
        self.assertEqual(set(workflow("canonical-governance.yml")["permissions"].items()), {("contents", "read")})

    def test_e2e_dispatches_for_its_admission_producers_and_test_changes(self):
        config = workflow("test-e2e.yml")
        events = config.get("on", config.get(True))
        for event in ("push", "pull_request"):
            for path in (CONTROL + "check_source_contracts.py", CONTROL + "canonical_completion_guard.py", CONTROL + "canonical_completion_history.py", CONTROL + "canonical_completion_history.json", CONTROL + "browser_verification_guard.py", "scripts/tests/test_ci_source_admission.py"):
                self.assertIn(path, events[event]["paths"])


class RealGuardConsumerTest(SourceAdmissionFixture):
    def install_real_guards(self):
        for name in ("canonical_completion_history.py", "canonical_completion_guard.py", "control_plane.py", "subsystem_contracts.py"):
            self.write(CONTROL + name, (ROOT / CONTROL / name).read_text())
        self.write(CONTROL + "canonical_completion_history.json", '{"version":1,"completions":[]}\n')
        for path in ("docs/release-control/control_plane.json",
                     "docs/release-control/v6/internal/subsystems/registry.json"):
            self.write(path, (ROOT / path).read_text())
        self.base = self.commit("real public guards")

    def test_real_contract_guard_rejects_missing_contract_before_install(self):
        self.install_real_guards()
        self.changed("internal/monitoring/poller.go")
        result = self.run_early_workflow("build-and-test.yml")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("BLOCKED", result.stderr)
        self.assertFalse(self.installs.exists())

    def test_real_guard_accepts_commit_bound_neutral_reason_not_ambient_reason(self):
        self.install_real_guards()
        self.changed("internal/monitoring/poller.go", trailer="fixture refactor has no observable contract change")
        result = self.run_early_workflow("build-and-test.yml")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue(self.installs.exists())
        self.assertNotIn("ambient must not bypass", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
