"""Exercise the local delivery gate, not a live advisory service or Go suite."""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
PREPUSH = ROOT / "scripts/dev-prepush.sh"
AUDIT = ROOT / "scripts/npm-audit-retry.sh"


class DevPrepushFrontendTest(unittest.TestCase):
    def setUp(self) -> None:
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.repo = Path(self.directory.name) / "repo"
        self.repo.mkdir()
        self.bin = Path(self.directory.name) / "bin"
        self.bin.mkdir()
        self.calls = Path(self.directory.name) / "npm-calls"
        self.guard_calls = Path(self.directory.name) / "guard-calls"
        self.env = os.environ.copy()
        for key in list(self.env):
            if key.startswith(("GIT_", "NPM_AUDIT_")):
                self.env.pop(key)
        self.env.update(
            PATH=str(self.bin) + os.pathsep + self.env["PATH"],
            GIT_CONFIG_NOSYSTEM="1",
            GIT_CONFIG_GLOBAL=os.devnull,
            GIT_AUTHOR_NAME="Local gate fixture",
            GIT_AUTHOR_EMAIL="gate@example.invalid",
            GIT_COMMITTER_NAME="Local gate fixture",
            GIT_COMMITTER_EMAIL="gate@example.invalid",
            PREPUSH_NPM_CALLS=str(self.calls),
            PREPUSH_GUARD_CALLS=str(self.guard_calls),
            PREPUSH_AUDIT_MODE="clean",
            PREPUSH_TYPE_STATUS="0",
            PREPUSH_GUARD_STATUS="0",
            # Own the canned-provider budget, not the real runner's default.
            NPM_AUDIT_ATTEMPTS="2",
            NPM_AUDIT_ATTEMPT_TIMEOUT="3",
            NPM_AUDIT_MAX_SECONDS="15",
            NPM_AUDIT_RETRY_DELAY="0",
        )
        self.write("scripts/dev-prepush.sh", PREPUSH.read_text())
        self.write("scripts/npm-audit-retry.sh", AUDIT.read_text())
        self.write("frontend-modern/package.json", '{"private":true}\n')
        self.write("frontend-modern/package-lock.json", '{"lockfileVersion":3}\n')
        self.write("frontend-modern/src/example.ts", "export const value = 1;\n")
        self.write("README.md", "Local gate fixture\n")
        # The completion guard has its own exact-source tests. This fixture
        # records the real prepush invocation and exit handling only; it does
        # not claim canonical contract acceptance.
        self.write(
            "scripts/release_control/canonical_completion_guard.py",
            "import json,os,sys\n"
            "with open(os.environ['PREPUSH_GUARD_CALLS'],'a') as output:\n"
            " output.write(json.dumps({'argv':sys.argv[1:],"
            "'reason':os.environ.get('PULSE_ALLOW_CONTRACT_NEUTRAL_COMMIT'),"
            "'files':sys.stdin.read().splitlines()})+'\\n')\n"
            "sys.exit(int(os.environ['PREPUSH_GUARD_STATUS']))\n",
        )
        npm = self.bin / "npm"
        npm.write_text(
            "#!/usr/bin/env python3\n"
            "import json,os,sys\n"
            "args=sys.argv[1:]\n"
            "with open(os.environ['PREPUSH_NPM_CALLS'],'a') as output:\n"
            " output.write(json.dumps({'args':args,'cwd':os.getcwd(),"
            "'require':os.environ.get('NPM_AUDIT_REQUIRE_RESULT')})+'\\n')\n"
            "if args == ['run','type-check']:\n"
            " sys.exit(int(os.environ['PREPUSH_TYPE_STATUS']))\n"
            "if args != ['audit','--json']:\n"
            " sys.exit(64)\n"
            "mode=os.environ['PREPUSH_AUDIT_MODE']\n"
            "counts=dict.fromkeys(['info','low','moderate','high','critical','total'],0)\n"
            "if mode in ('moderate','critical'):\n"
            " counts[mode]=counts['total']=1\n"
            "if mode == 'unavailable':\n"
            " print(json.dumps({'error':{'code':'ENOAUDIT'}}))\n"
            "else:\n"
            " print(json.dumps({'metadata':{'vulnerabilities':counts}}))\n"
            "sys.exit(0 if mode == 'clean' else 42)\n",
            encoding="utf-8",
        )
        npm.chmod(0o755)
        self.git("init", "--quiet")
        self.git("add", ".")
        self.git("commit", "--quiet", "-m", "Fixture base")
        self.base = self.git("rev-parse", "HEAD").stdout.strip()

    def write(self, relative: str, content: str) -> None:
        path = self.repo / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")

    def git(self, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            ["git", *args], cwd=self.repo, env=self.env,
            text=True, capture_output=True, check=True, timeout=20,
        )

    def change(self, relative: str, *, trailer: str | None = None) -> str:
        path = self.repo / relative
        previous = path.read_text() if path.exists() else ""
        if relative.endswith(".json"):
            content = json.loads(previous)
            content["fixture_delta"] = True
            self.write(relative, json.dumps(content) + "\n")
        else:
            self.write(relative, previous + "\n# fixture delta\n")
        self.git("add", "--", relative)
        message = ["-m", "Fixture change"]
        if trailer:
            message += ["-m", "Contract-Neutral: " + trailer]
        self.git("commit", "--quiet", *message)
        return self.git("rev-parse", "HEAD").stdout.strip()

    def run_gate(self, *, dependencies: bool = True):
        if dependencies:
            (self.repo / "frontend-modern/node_modules").mkdir(exist_ok=True)
        result = subprocess.run(
            ["bash", "scripts/dev-prepush.sh", self.base],
            cwd=self.repo, env=self.env, text=True, capture_output=True,
            check=False, timeout=30,
        )
        calls = [json.loads(line) for line in self.calls.read_text().splitlines()] if self.calls.exists() else []
        guards = [json.loads(line) for line in self.guard_calls.read_text().splitlines()] if self.guard_calls.exists() else []
        return result, calls, guards

    def assert_refused_before_source_checks(self, result, calls, guards, reads=1):
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertNotIn("All pre-push checks passed", result.stdout)
        self.assertIn("remaining source checks have not run", result.stderr)
        self.assertEqual([call["args"] for call in calls], [["audit", "--json"]] * reads)
        self.assertTrue(all(call["require"] == "true" for call in calls))
        self.assertEqual(guards, [])

    def test_changed_manifest_rejects_critical_findings(self):
        self.change("frontend-modern/package.json")
        self.env["PREPUSH_AUDIT_MODE"] = "critical"
        # An ambient tolerant caller must not relax this changed-graph gate.
        self.env["NPM_AUDIT_REQUIRE_RESULT"] = "false"
        self.assert_refused_before_source_checks(*self.run_gate())

    def test_changed_lock_rejects_moderate_findings(self):
        self.change("frontend-modern/package-lock.json")
        self.env["PREPUSH_AUDIT_MODE"] = "moderate"
        self.assert_refused_before_source_checks(*self.run_gate())

    def test_changed_audit_runner_requires_a_result(self):
        self.change("scripts/npm-audit-retry.sh")
        self.env["PREPUSH_AUDIT_MODE"] = "unavailable"
        self.assert_refused_before_source_checks(*self.run_gate(), reads=2)

    def test_failed_zero_report_is_not_a_passing_local_gate(self):
        self.change("frontend-modern/package-lock.json")
        self.env["PREPUSH_AUDIT_MODE"] = "zero-failure"
        self.assert_refused_before_source_checks(*self.run_gate(), reads=2)

    def test_clean_changed_graph_runs_typecheck_and_preserves_commit_binding(self):
        trailer = "fixture graph has no API delta"
        head = self.change("frontend-modern/package-lock.json", trailer=trailer)
        result, calls, guards = self.run_gate()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("All pre-push checks passed", result.stdout)
        self.assertEqual([call["args"] for call in calls], [["audit", "--json"], ["run", "type-check"]])
        self.assertEqual(calls[0]["require"], "true")
        self.assertTrue(all(call["cwd"] == str(self.repo / "frontend-modern") for call in calls))
        self.assertEqual(len(guards), 1)
        self.assertEqual(guards[0]["reason"].strip(), trailer)
        self.assertEqual({key: value for key, value in guards[0].items() if key != "reason"}, {
            "argv": ["--files-from-stdin", "--diff-base", head + "^", "--commit", head],
            "files": ["frontend-modern/package-lock.json"],
        })

    def test_large_change_list_does_not_skip_strict_audit(self):
        self.change("frontend-modern/package.json")
        for index in range(600):
            self.write("zzz/" + str(index) + "x" * 140 + ".txt", "fixture\n")
        self.git("add", "--", "zzz")
        self.git("commit", "--quiet", "-m", "Large changed-file fixture")
        self.assertGreater(len(self.git("diff", "--name-only", self.base, "HEAD").stdout), 65536)
        self.env["PREPUSH_AUDIT_MODE"] = "critical"
        self.assert_refused_before_source_checks(*self.run_gate())

    def test_unchanged_graph_is_not_queried(self):
        self.change("frontend-modern/src/example.ts")
        self.env["PREPUSH_AUDIT_MODE"] = "critical"
        result, calls, guards = self.run_gate()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual([call["args"] for call in calls], [["run", "type-check"]])
        self.assertEqual(len(guards), 1)

    def test_missing_frontend_dependencies_do_not_pass(self):
        self.change("frontend-modern/src/example.ts")
        result, calls, guards = self.run_gate(dependencies=False)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("frontend-modern/node_modules missing", result.stdout)
        self.assertNotIn("All pre-push checks passed", result.stdout)
        self.assertEqual(calls, [])
        self.assertEqual(len(guards), 1)

    def test_typecheck_failure_remains_a_failure(self):
        self.change("frontend-modern/src/example.ts")
        self.env["PREPUSH_TYPE_STATUS"] = "7"
        result, calls, guards = self.run_gate()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertNotIn("All pre-push checks passed", result.stdout)
        self.assertEqual([call["args"] for call in calls], [["run", "type-check"]])
        self.assertEqual(len(guards), 1)

    def test_canonical_failure_is_not_cleared_by_a_clean_audit(self):
        self.change("frontend-modern/package.json")
        self.env["PREPUSH_GUARD_STATUS"] = "4"
        result, calls, guards = self.run_gate()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertNotIn("All pre-push checks passed", result.stdout)
        self.assertEqual([call["args"] for call in calls], [["audit", "--json"], ["run", "type-check"]])
        self.assertEqual(len(guards), 1)

    def test_documentation_only_has_no_frontend_checks(self):
        self.change("README.md")
        result, calls, guards = self.run_gate(dependencies=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(calls, [])
        self.assertEqual(len(guards), 1)

    def test_no_outgoing_commit_has_no_checks(self):
        result, calls, guards = self.run_gate(dependencies=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("No commits ahead", result.stdout)
        self.assertEqual(calls, [])
        self.assertEqual(guards, [])

    def test_audit_path_lookalike_does_not_query(self):
        self.change("scripts/npm-audit-retry.sh.extra")
        result, calls, guards = self.run_gate(dependencies=False)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(calls, [])
        self.assertEqual(len(guards), 1)

    def test_local_strict_paths_match_ci(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/build-and-test.yml").read_text())
        classification = next(step for step in workflow["jobs"]["changes"]["steps"] if step.get("id") == "filter")["run"]
        paths = r"^frontend-modern/package(-lock)?\.json$|^scripts/npm-audit-retry\.sh$"
        self.assertIn(paths, classification)
        self.assertIn(paths, PREPUSH.read_text())


if __name__ == "__main__":
    unittest.main()
