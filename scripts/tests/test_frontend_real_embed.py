#!/usr/bin/env python3
"""Check ordinary CI's real-embed phase without a host build or network.

The exact workflow shell runs with private asset fixtures and compiler/Git
doubles. This is recipe/admission proof, not real frontend/backend acceptance.
"""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = Path(os.environ.get("PULSE_BUILD_WORKFLOW_UNDER_TEST", ROOT / ".github/workflows/build-and-test.yml")).resolve()
GO_SETUP = "Set up Go for real-embed verification"
VERIFY = "Verify backend with real frontend embed"


def recipe(source):
    job = re.search(r"(?ms)^  frontend:\n(.*?)(?=^  [a-z][a-z0-9-]*:|\Z)", source)
    if not job:
        raise AssertionError("frontend job missing")
    job = job[1]
    if "timeout-minutes: 30" not in job or "if: needs.changes.outputs.code == 'true'" not in job:
        raise AssertionError("ordinary frontend admission/deadline changed")
    parsed = re.findall(r"(?ms)^      - name: ([^\n]+)\n(.*?)(?=^      - name:|\Z)", job)
    steps = dict(parsed)
    if len(parsed) != len(steps):
        raise AssertionError("duplicate named frontend step")
    order = list(steps)
    required = ["Checkout repository", "Build frontend bundle (with embed copy)",
                "Check frontend bundle size budget", "Frontend unit tests", GO_SETUP, VERIFY,
                "Require frontend dependency audit"]
    if any(name not in order for name in required) or [order.index(name) for name in required] != sorted(order.index(name) for name in required):
        raise AssertionError("same-checkout real build, tests, compiler or audit ordering missing")
    if sum("uses: actions/checkout@" in step for step in steps.values()) != 1 or "run: make frontend" not in steps[required[1]]:
        raise AssertionError("must use this checkout's production frontend build")
    if re.search(r"^          (ref|repository):", steps[required[0]], re.M):
        raise AssertionError("frontend proof must use the ordinary exact checkout")
    if "go-version-file: go.mod" not in steps[GO_SETUP] or "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e" not in steps[GO_SETUP]:
        raise AssertionError("declared compiler setup missing")
    for name in (GO_SETUP, VERIFY):
        if re.search(r"^        (if|continue-on-error|working-directory):", steps[name], re.M):
            raise AssertionError("real-embed verification must run at root and retain failure")
    run = re.search(r"(?ms)^        run: \|\n((?:          [^\n]*\n)+)", steps[VERIFY])
    if not run:
        raise AssertionError("real-embed shell missing")
    shell = "\n".join(line[10:] for line in run[1].splitlines()) + "\n"
    for command in ("set -euo pipefail", "checkout_sha=$(git rev-parse HEAD)", "test -s frontend-modern/dist/index.html",
                    "test -d frontend-modern/dist/assets",
                    "diff -qr --no-dereference frontend-modern/dist internal/api/frontend-modern/dist",
                    'go build -mod=readonly -p=2 -o "$RUNNER_TEMP/pulse-real-embed" ./cmd/pulse',
                    "go vet -mod=readonly -p=2 ./cmd/pulse ./internal/api"):
        if command not in shell:
            raise AssertionError("required real-embed command missing: " + command)
    return shell


COMPILER = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with Path(os.environ["TRACE"]).open("a") as out:
    out.write(json.dumps(args) + "\n")
if args[0] == os.environ["FAIL_PHASE"]:
    sys.exit(43)
if args[0] == "build":
    Path(args[args.index("-o") + 1]).write_bytes(b"fixture binary, not a real build\n")
elif args != ["version"] and args[0] != "vet":
    sys.exit(97)
'''


class FrontendRealEmbed(unittest.TestCase):
    def run_recipe(self, *, mutation="", fail_phase="", git_exit=0):
        shell = recipe(WORKFLOW.read_text())
        with tempfile.TemporaryDirectory(prefix="pulse-real-embed-") as directory:
            f = Path(directory)
            for tree in ("frontend-modern/dist", "internal/api/frontend-modern/dist"):
                (f / tree / "assets").mkdir(parents=True)
                (f / tree / "index.html").write_text("<html>fixture production index</html>\n")
                (f / tree / "assets/app.js").write_text("fixture application bytes\n")
            embed = f / "internal/api/frontend-modern/dist"
            if mutation == "missing index": (f / "frontend-modern/dist/index.html").unlink()
            if mutation == "empty index": (f / "frontend-modern/dist/index.html").write_text("")
            if mutation == "missing assets":
                (f / "frontend-modern/dist/assets/app.js").unlink()
                (f / "frontend-modern/dist/assets").rmdir()
            if mutation == "changed served byte": (embed / "assets/app.js").write_text("different bytes\n")
            if mutation == "extra served file": (embed / "extra.js").write_text("extra bytes\n")
            if mutation == "missing embedded file": (embed / "assets/app.js").unlink()
            for name in ("tools", "runner-temp"): (f / name).mkdir()
            (f / "tools/go").write_text(COMPILER)
            (f / "tools/git").write_text("#!/bin/sh\n[ \"$*\" = 'rev-parse HEAD' ] || exit 97\n[ \"$GIT_EXIT\" = 0 ] || exit \"$GIT_EXIT\"\necho 0123456789012345678901234567890123456789\n")
            for name in ("go", "git"): (f / "tools" / name).chmod(0o700)
            env = dict(os.environ, PATH=str(f / "tools") + os.pathsep + os.environ["PATH"],
                       TRACE=str(f / "trace"), FAIL_PHASE=fail_phase, GIT_EXIT=str(git_exit), RUNNER_TEMP=str(f / "runner-temp"))
            result = subprocess.run(["bash", "-c", shell], cwd=f, env=env, capture_output=True, text=True, timeout=10)
            calls = [json.loads(line) for line in (f / "trace").read_text().splitlines()] if (f / "trace").exists() else []
            output = f / "runner-temp/pulse-real-embed"
            return result, calls, str(output), output.exists()

    def test_success_uses_readonly_graph_and_same_checkout_assets(self):
        result, calls, output, built = self.run_recipe()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(calls, [["version"], ["build", "-mod=readonly", "-p=2", "-o", output, "./cmd/pulse"],
                                ["vet", "-mod=readonly", "-p=2", "./cmd/pulse", "./internal/api"]])
        self.assertTrue(built)
        self.assertIn("REAL_EMBED_CHECKOUT=0123456789012345678901234567890123456789", result.stdout)

    def test_missing_or_changed_embed_refuses_before_compiler(self):
        for mutation in ("missing index", "empty index", "missing assets", "changed served byte", "extra served file", "missing embedded file"):
            with self.subTest(mutation=mutation):
                result, calls, _, built = self.run_recipe(mutation=mutation)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, [])
                self.assertFalse(built)

    def test_build_and_vet_failures_are_terminal(self):
        for phase, called in (("build", ["version", "build"]), ("vet", ["version", "build", "vet"])):
            with self.subTest(phase=phase):
                result, calls, _, _ = self.run_recipe(fail_phase=phase)
                self.assertEqual(result.returncode, 43)
                self.assertEqual([call[0] for call in calls], called)

    def test_failed_checkout_read_is_not_a_build_grant(self):
        result, calls, _, built = self.run_recipe(git_exit=46)
        self.assertEqual(result.returncode, 46)
        self.assertEqual(calls, [])
        self.assertFalse(built)
        self.assertNotIn("REAL_EMBED_CHECKOUT=", result.stdout)

    def test_missing_hidden_or_foreign_workflow_phases_are_rejected(self):
        source = WORKFLOW.read_text()
        for before, after in (("run: make frontend", "run: echo stub"),
                              ("go-version-file: go.mod", "go-version: stable"),
                              (f"- name: {VERIFY}", "- name: unrelated phase"),
                              (f"- name: {VERIFY}\n", f"- name: {VERIFY}\n        continue-on-error: true\n"),
                              (f"- name: {VERIFY}\n", f"- name: {VERIFY}\n        if: false\n"),
                              (f"- name: {VERIFY}\n", f"- name: {VERIFY}\n        working-directory: frontend-modern\n"),
                              (f"- name: {GO_SETUP}\n", "- name: Checkout repository\n"),
                              ("persist-credentials: false\n", "persist-credentials: false\n          ref: other-source\n"),
                              ("go vet -mod=readonly -p=2 ./cmd/pulse ./internal/api", "echo skipped vet")):
            with self.subTest(after=after):
                self.assertIn(before, source)
                with self.assertRaises(AssertionError): recipe(source.replace(before, after))


if __name__ == "__main__":
    unittest.main()
