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

from test_public_docs_workflow import job_body, step_body


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
                         ["    if: needs.changes.outputs.code == 'true'"])
        self.assertNotIn("continue-on-error:", job_body(self.source, "scripts-and-build"))
        self.assertIn("permissions:\n  contents: read\n", self.source)
        for shard in range(2):
            self.assertEqual(shlex.split(smoke_command(self.source, shard)),
                             ["scripts/tests/run.sh", "--smoke-shard", str(shard)])

    def test_existing_required_check_is_fail_closed(self):
        build = job_body(self.source, "scripts-and-build")
        self.assertIn("    needs: [changes, script-smoke]\n", build)
        self.assertIn("    if: always() && needs.changes.outputs.code == 'true'\n", build)
        self.assertEqual(self.source.count(f"    name: {REQUIRED_NAME}\n"), 1)
        self.assertIn("          SMOKE_RESULT: ${{ needs.script-smoke.result }}\n", build)
        for state in ("success", "failure", "cancelled", "skipped", "", "unknown"):
            with self.subTest(state=state):
                result = subprocess.run(
                    ["bash", "-e", "-o", "pipefail", "-c", verdict_command(self.source)],
                    env=dict(os.environ, SMOKE_RESULT=state), text=True,
                    capture_output=True, timeout=5,
                )
                self.assertEqual(result.returncode == 0, state == "success",
                                 result.stdout + result.stderr)

    def test_backend_build_waits_for_gate_and_is_still_required(self):
        build = job_body(self.source, "scripts-and-build")
        self.assertLess(build.index(VERDICT_STEP), build.index("Checkout repository"))
        self.assertLess(build.index(VERDICT_STEP), build.index("Build Pulse backend"))
        self.assertIn("    runs-on: ubuntu-24.04\n", build)
        self.assertIn("    timeout-minutes: 30\n", build)
        self.assertIn("          go-version-file: go.mod\n", build)
        self.assertIn("          persist-credentials: false\n", build)
        self.assertIn("      - name: Provide frontend embed stub\n", build)
        self.assertIn("        run: go build ./cmd/pulse\n",
                      step_body(build, "Build Pulse backend"))
        self.assertEqual(re.findall(r"(?m)^\s*if:.*$", build),
                         ["    if: always() && needs.changes.outputs.code == 'true'"])

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
            for name in smoke_suites():
                status = 7 if name == failing else 0
                if name.endswith(".sh"):
                    code = f'#!/bin/bash\nprintf "%s\\n" {shlex.quote(name)} >> "$CALLS"\nexit {status}\n'
                else:
                    code = ("import os\nfrom pathlib import Path\n"
                            f"with Path(os.environ['CALLS']).open('a') as log: log.write({name!r} + '\\n')\n"
                            f"raise SystemExit({status})\n")
                (tests / name).write_text(code, encoding="utf-8")
                (tests / name).chmod(0o700)
            calls = root / "calls"
            # Bash is the hosted step's interpreter; execute the runner itself.
            (tests / "run.sh").chmod(0o700)
            result = subprocess.run(
                ["bash", "-e", "-o", "pipefail", "-c", smoke_command(self.source, shard)],
                cwd=root, env=dict(os.environ, CALLS=str(calls)), text=True,
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
            self.assertEqual(calls, suites[shard::2])
            self.assertIn(f"Summary: {len(calls)}/{len(calls)} passed", result.stdout)
            seen.extend(calls)
        self.assertEqual(sorted(seen), suites)
        self.assertEqual(len(seen), len(set(seen)))

    def test_failure_in_either_shard_keeps_remaining_suites_and_required_gate(self):
        for shard in range(2):
            names = smoke_suites()[shard::2]
            result, calls = self.run_adapters(shard, names[0])
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertEqual(calls, names)
            self.assertIn(f"Summary: {len(names)-1}/{len(names)} passed", result.stdout)
            gate = subprocess.run(
                ["bash", "-e", "-o", "pipefail", "-c", verdict_command(self.source)],
                env=dict(os.environ, SMOKE_RESULT="failure"), text=True,
                capture_output=True, timeout=5,
            )
            self.assertNotEqual(gate.returncode, 0)


if __name__ == "__main__":
    unittest.main()
