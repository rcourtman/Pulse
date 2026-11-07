#!/usr/bin/env python3
"""Every smoke suite remains required when CI runs two isolated inventories."""

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

import yaml

import yaml

from test_frontend_real_embed import recipe as real_embed_recipe
from test_public_docs_workflow import job_body, step_body, workflow_fixture_environment


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
VERDICT_STEP = "Require every script smoke shard"
REQUIRED_NAME = "Script smoke tests & backend build"


def smoke_suites() -> list[str]:
    return sorted(path.name for path in (ROOT / "scripts/tests").iterdir()
                  if path.is_file() and (fnmatch.fnmatch(path.name, "test-*.sh")
                                         or fnmatch.fnmatch(path.name, "test_*.py")))


def smoke_command(source: str, shard: int) -> str:
    body = step_body(job_body(source, "script-smoke"), "Script smoke tests")
    commands = re.findall(r"^        run: (.+)$", body, re.MULTILINE)
    if len(commands) != 1:
        raise AssertionError("expected one literal smoke-shard command")
    return commands[0].replace("${{ matrix.shard }}", str(shard))


def verdict_command(source: str) -> str:
    body = step_body(job_body(source, "scripts-and-build"), VERDICT_STEP)
    marker = "        run: |\n"
    if body.count(marker) != 1:
        raise AssertionError("expected one literal smoke verdict command")
    return textwrap.dedent(body.split(marker, 1)[1]).strip()


class SmokeWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.source = WORKFLOW.read_text(encoding="utf-8")

    def test_two_isolated_complete_shards_keep_source_setup_and_deadlines(self):
        smoke = job_body(self.source, "script-smoke")
        for required in (
            "    needs: changes\n", "    if: needs.changes.outputs.code == 'true'\n",
            "    runs-on: ubuntu-24.04\n", "    timeout-minutes: 30\n",
            "      fail-fast: false\n", "        shard: [0, 1]\n",
            "          persist-credentials: false\n", "          fetch-depth: 0\n",
            "          go-version-file: go.mod\n", "          cache: true\n",
            "      - name: Provide frontend embed stub\n",
        ):
            self.assertIn(required, smoke)
        self.assertNotRegex(smoke, r"(?m)^\s*(?:include|exclude|continue-on-error):")
        self.assertEqual(re.findall(r"(?m)^\s*if:.*$", smoke),
                         ["    if: needs.changes.outputs.code == 'true'", "        if: always()"])
        self.assertNotIn("continue-on-error:", job_body(self.source, "scripts-and-build"))
        self.assertIn("permissions:\n  contents: read\n", self.source)
        for shard in range(2):
            self.assertEqual(shlex.split(smoke_command(self.source, shard)),
                             ["scripts/tests/run.sh", "--smoke-shard", str(shard)])

    def test_returned_suite_observations_are_bound_and_retained_after_failure(self):
        smoke = yaml.safe_load(self.source)['jobs']['script-smoke']
        execute = next(step for step in smoke['steps'] if step['name'] == 'Script smoke tests')
        upload = next(step for step in smoke['steps']
                      if step['name'] == 'Retain script smoke observations')
        destination = '${{ runner.temp }}/script-smoke-${{ matrix.shard }}-${{ github.run_attempt }}.json'
        self.assertEqual(execute['env'], {'PULSE_SCRIPT_TEST_RECEIPT': destination,
                                         'PULSE_SCRIPT_TEST_SOURCE_SHA': '${{ github.sha }}'})
        self.assertEqual(upload['if'], 'always()')
        self.assertEqual(upload['with'], {
            'name': 'script-smoke-${{ matrix.shard }}-${{ github.run_attempt }}',
            'path': destination, 'if-no-files-found': 'warn', 'retention-days': 7})
        self.assertTrue(upload['uses'].startswith('actions/upload-artifact@'))
        self.assertNotIn('continue-on-error', upload)

    def test_existing_required_check_is_fail_closed(self):
        build = job_body(self.source, "scripts-and-build")
        self.assertIn("    needs: [changes, script-smoke, frontend-checks]\n", build)
        self.assertIn("    if: always() && needs.changes.outputs.code == 'true'\n", build)
        self.assertEqual(self.source.count(f"    name: {REQUIRED_NAME}\n"), 1)
        self.assertIn("          SMOKE_RESULT: ${{ needs.script-smoke.result }}\n", build)
        self.assertIn("          BACKEND_RESULT: ${{ needs.frontend-checks.result }}\n", build)
        states = ("success", "failure", "cancelled", "skipped", "", "unknown")
        for smoke_state in states:
            for backend_state in states:
                with self.subTest(smoke=smoke_state, backend=backend_state):
                    result = subprocess.run(
                        ["bash", "-e", "-u", "-o", "pipefail", "-c", verdict_command(self.source)],
                        env=dict(os.environ, SMOKE_RESULT=smoke_state, BACKEND_RESULT=backend_state),
                        text=True, capture_output=True, timeout=5,
                    )
                    self.assertEqual(result.returncode == 0,
                                     smoke_state == backend_state == "success",
                                     result.stdout + result.stderr)
                    if result.returncode == 0:
                        self.assertIn("this run's real-embed backend proof passed", result.stdout)

    def test_missing_result_cannot_admit_the_required_check(self):
        for missing in ("SMOKE_RESULT", "BACKEND_RESULT"):
            with self.subTest(missing=missing):
                env = dict(os.environ, SMOKE_RESULT="success", BACKEND_RESULT="success")
                env.pop(missing)
                result = subprocess.run(
                    ["bash", "-e", "-u", "-o", "pipefail", "-c", verdict_command(self.source)],
                    env=env, text=True, capture_output=True, timeout=5,
                )
                self.assertNotEqual(result.returncode, 0)

    def test_verdict_reuses_required_same_run_real_embed_build_without_another_compiler(self):
        jobs = yaml.safe_load(self.source)["jobs"]
        verdict = jobs["scripts-and-build"]
        self.assertEqual(verdict["runs-on"], "ubuntu-24.04")
        self.assertEqual(verdict["timeout-minutes"], 5)
        self.assertEqual(verdict["needs"], ["changes", "script-smoke", "frontend-checks"])
        self.assertEqual(verdict["if"], "always() && needs.changes.outputs.code == 'true'")
        self.assertNotIn("continue-on-error", verdict)
        self.assertEqual(len(verdict["steps"]), 1)
        step = verdict["steps"][0]
        self.assertEqual(step["env"], {
            "SMOKE_RESULT": "${{ needs.script-smoke.result }}",
            "BACKEND_RESULT": "${{ needs.frontend-checks.result }}",
        })
        self.assertNotIn("if", step)
        self.assertNotIn("continue-on-error", step)
        self.assertNotIn("uses", step)
        self.assertNotRegex(step["run"], r"\b(?:go|npm|git)\b")

        # The reused producer is in this same workflow/DAG, not a cached status
        # or an unrelated successful job. Check its actual production-asset
        # transfer, declared compiler, readonly build/vet recipe and ordering.
        real_embed_recipe(self.source)
        backend = jobs["frontend-checks"]
        self.assertEqual(backend["if"], "${{ !cancelled() && needs.changes.outputs.code == 'true' && needs.frontend-build.outputs.unit_admitted == 'true' }}")
        self.assertNotIn("continue-on-error", backend)
        required = {"Verify backend with real frontend embed",
                    "Check current-source unattended recovery on native systemd",
                    "Check ordinary notification policy and restart"}
        found = {step["name"] for step in backend["steps"] if step["name"] in required}
        self.assertEqual(found, required)
        for step in backend["steps"]:
            if step["name"] in required:
                self.assertNotIn("if", step)
                self.assertNotIn("continue-on-error", step)

    def run_adapters(self, shard: int, failing: str = ""):
        """Run actual workflow/runner commands on every source-selected name.

        Only this fixture replaces suites with exit adapters; the full real
        suites run separately in the source-proof VM.
        """
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            tests = root / "scripts/tests"
            tests.mkdir(parents=True)
            shutil.copyfile(ROOT / "scripts/tests/run.sh", tests / "run.sh")
            shutil.copyfile(ROOT / "scripts/run-script-tests.py", root / "scripts/run-script-tests.py")
            shutil.copyfile(ROOT / "scripts/select-smoke-test-shard.py",
                            root / "scripts/select-smoke-test-shard.py")
            hints = root / ".github/scripts/smoke-test-seconds.txt"
            hints.parent.mkdir(parents=True)
            shutil.copyfile(ROOT / ".github/scripts/smoke-test-seconds.txt", hints)
            for name in smoke_suites():
                status = 7 if name == failing else 0
                if name.endswith(".sh"):
                    code = f'#!/bin/bash\nprintf "%s\\n" {shlex.quote(name)} >> "$CALLS"\nexit {status}\n'
                else:
                    code = ("import os\n"
                            f"with open(os.environ['CALLS'], 'a', encoding='utf-8') as log: log.write({name!r} + '\\n')\n"
                            f"raise SystemExit({status})\n")
                (tests / name).write_text(code, encoding="utf-8")
                (tests / name).chmod(0o700)
            calls = root / "calls"
            # Bash is the hosted step's interpreter; execute the runner itself.
            (tests / "run.sh").chmod(0o700)
            result = subprocess.run(
                ["bash", "-e", "-o", "pipefail", "-c", smoke_command(self.source, shard)],
                cwd=root, env=workflow_fixture_environment(root, calls), text=True,
                capture_output=True, timeout=15,
            )
            return result, calls.read_text().splitlines() if calls.exists() else []

    def test_actual_workflow_commands_cover_every_current_suite_once(self):
        suites = smoke_suites()
        self.assertIn("test_smoke_workflow.py", suites)
        self.assertIn("test_performance_troubleshooting_docs.py", suites)
        self.assertTrue(any(name.endswith(".sh") for name in suites))
        seen = []
        for shard in range(2):
            result, calls = self.run_adapters(shard)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertTrue(calls)
            self.assertEqual(calls, sorted(calls))
            self.assertIn(f"Summary: {len(calls)}/{len(calls)} passed", result.stdout)
            seen.extend(calls)
        self.assertEqual(sorted(seen), suites)
        self.assertEqual(len(seen), len(set(seen)))

    def test_failure_in_either_shard_keeps_remaining_suites_and_required_gate(self):
        for shard in range(2):
            # Observe the actual source-bound selection, not the old filename
            # parity. The failing adapter must not hide any later selected suite.
            selected, names = self.run_adapters(shard)
            self.assertEqual(selected.returncode, 0, selected.stdout + selected.stderr)
            self.assertTrue(names)
            result, calls = self.run_adapters(shard, names[0])
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(calls, names)
            self.assertIn(f"Summary: {len(names)-1}/{len(names)} passed", result.stdout)
            gate = subprocess.run(
                ["bash", "-e", "-o", "pipefail", "-c", verdict_command(self.source)],
                env=dict(os.environ, SMOKE_RESULT="failure", BACKEND_RESULT="success"), text=True,
                capture_output=True, timeout=5,
            )
            self.assertNotEqual(gate.returncode, 0)


if __name__ == "__main__":
    unittest.main()
