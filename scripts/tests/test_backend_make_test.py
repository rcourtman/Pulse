"""Exercise make test's inventory admission and actual fresh Go execution.

Disposable, dependency-free packages stand in for Pulse's backend here. These
are entrypoint controls, not proof of the application's race suite or real embed.
The ordinary scripts/tests/run.sh discovery runs this suite in CI.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
MAKEFILE = ROOT / "Makefile"
PACKAGE_PATTERNS = ["./cmd/...", "./internal/...", "./pkg/...", "./scripts/...", "./tests/..."]
PACKAGES = ["cmd/probe", "internal/probe", "pkg/probe", "scripts/probe", "tests/probe"]


class MakeFixture(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.tools = self.root / "tools"
        self.tools.mkdir()
        (self.root / "Makefile").write_bytes(MAKEFILE.read_bytes())
        scripts = self.root / "scripts"
        scripts.mkdir()
        assets = scripts / "ensure_test_assets.sh"
        assets.write_text("#!/bin/sh\nprintf 'prepared\\n' > assets-prepared\n")
        assets.chmod(0o700)
        # A file named test must never turn the command into 'up to date'.
        (self.root / "test").touch()
        self.env = {key: value for key, value in os.environ.items()
                    if key not in {"GOFLAGS", "MAKEFLAGS", "MFLAGS", "MAKELEVEL", "GNUMAKEFLAGS"}}
        self.env.update(PATH=str(self.tools) + os.pathsep + self.env["PATH"],
                        GOENV="off", GOWORK="off", GOPROXY="off", GOSUMDB="off",
                        GOTOOLCHAIN="local", MAKE_PROBE_ROOT=str(self.root))

    def run_make(self, **environment):
        return subprocess.run(["make", "--no-print-directory", "test", "GO_TEST_TIMEOUT=20s"],
                              cwd=self.root, env={**self.env, **environment},
                              capture_output=True, text=True, timeout=120)

    def calls(self):
        path = self.root / "go-calls.jsonl"
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def go_adapter(self, *, real_go=None):
        go = self.tools / "go"
        go.write_text("""#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
root = Path(os.environ['MAKE_PROBE_ROOT'])
with (root / 'go-calls.jsonl').open('a') as stream:
    stream.write(json.dumps({'args': sys.argv[1:], 'goflags': os.environ.get('GOFLAGS', '')})+'\\n')
""" + (f"os.execv({real_go!r}, [{real_go!r}, *sys.argv[1:]])\n" if real_go else """
if sys.argv[1] == 'list':
    print(os.environ.get('LIST_OUTPUT', 'example.invalid/probe/one\\nexample.invalid/probe/two'))
    if os.environ.get('LIST_STATUS', '0') != '0':
        print('original package listing diagnostic', file=sys.stderr)
    sys.exit(int(os.environ.get('LIST_STATUS', '0')))
if sys.argv[1] == 'test':
    print('fixture test command executed')
    sys.exit(int(os.environ.get('TEST_STATUS', '0')))
sys.exit(99)
"""))
        go.chmod(0o700)


class InventoryTest(MakeFixture):
    def setUp(self):
        super().setUp()
        self.go_adapter()

    def test_partial_failed_listing_never_starts_tests(self):
        for output in ("example.invalid/probe/one", ""):
            with self.subTest(output=output):
                (self.root / "go-calls.jsonl").unlink(missing_ok=True)
                result = self.run_make(LIST_STATUS="7", LIST_OUTPUT=output)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("original package listing diagnostic", result.stderr)
                self.assertIn("backend tests have not run", result.stderr)
                self.assertIn("Error 7", result.stderr)
                self.assertEqual([call["args"][0] for call in self.calls()], ["list"])

    def test_empty_successful_listing_never_falls_back_to_current_package(self):
        for output in ("", " \t\n "):
            with self.subTest(output=output):
                (self.root / "go-calls.jsonl").unlink(missing_ok=True)
                result = self.run_make(LIST_OUTPUT=output)
                self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("listing was empty", result.stderr)
                self.assertEqual([call["args"][0] for call in self.calls()], ["list"])

    def test_complete_inventory_and_product_test_bounds_are_preserved(self):
        result = self.run_make()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertTrue((self.root / "assets-prepared").exists())
        listed, tested = [call["args"] for call in self.calls()]
        self.assertEqual(listed, ["list", "-n=false", *PACKAGE_PATTERNS])
        self.assertEqual(tested[-2:], ["example.invalid/probe/one", "example.invalid/probe/two"])
        self.assertEqual(tested[:4], ["test", "-race", "-timeout", "20s"])
        self.assertIn("-count=1", tested)
        self.assertIn("-fuzz=", tested)
        self.assertIn("GO_TEST_TIMEOUT ?= 30m", MAKEFILE.read_text())

    def test_test_command_failure_remains_nonzero(self):
        result = self.run_make(TEST_STATUS="9")
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Error 9", result.stderr)
        self.assertEqual([call["args"][0] for call in self.calls()], ["list", "test"])


class RealGoTest(MakeFixture):
    def setUp(self):
        super().setUp()
        real_go = shutil.which("go")
        self.assertIsNotNone(real_go, "make test controls require the supplied Go toolchain")
        self.go_adapter(real_go=real_go)
        (self.root / "go.mod").write_text("module example.invalid/makeprobe\n\ngo 1.24.0\n")
        for package in PACKAGES:
            directory = self.root / package
            directory.mkdir(parents=True)
            (directory / "probe_test.go").write_text('''package probe

import (
    "os"
    "testing"
)

func mark(t *testing.T, name string) {
    t.Helper()
    file, err := os.OpenFile("executed", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil { t.Fatal(err) }
    if _, err := file.WriteString(name + "\\n"); err != nil { t.Fatal(err) }
    if err := file.Close(); err != nil { t.Fatal(err) }
}

func TestFirst(t *testing.T) {
    mark(t, "first")
    if os.Getenv("MAKE_PROBE_FAIL") == "yes" { t.Fatal("deliberate fixture failure") }
}
func TestFullRuntime(t *testing.T) {
    if testing.Short() { t.Skip("requires the full test entrypoint") }
    mark(t, "full")
}
func TestLast(t *testing.T) { mark(t, "last") }
''')
            (directory / "tagged_test.go").write_text('''//go:build makeprobe

package probe

import "testing"

func TestBuildTag(t *testing.T) { mark(t, "tagged") }
''')

    def assert_executed_once(self):
        for package in PACKAGES:
            marker = self.root / package / "executed"
            self.assertTrue(marker.exists(), f"no actual test execution in {package}")
            self.assertEqual(marker.read_text().splitlines(), ["first", "full", "last", "tagged"])
            marker.unlink()

    def test_ambient_modes_cannot_replace_fresh_complete_execution(self):
        compiler_flags = "-mod=readonly -p=1 -tags=makeprobe"
        for mode in ("", "-run=^$", "-skip=.", "-list=.", "-exec=/bin/true",
                     "-c", "-n", "-short", "-failfast", "-count=2"):
            with self.subTest(mode=mode):
                flags = compiler_flags + (" " + mode if mode else "")
                result = self.run_make(GOFLAGS=flags)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assert_executed_once()
                # Module/compiler/worker flags survive unchanged. The actual
                # build-tag-only test above demonstrates consumption, not
                # just the presence of an argument in an orchestration stub.
                self.assertTrue(all(call["goflags"] == flags for call in self.calls()[-2:]))

    def test_failed_root_does_not_hide_later_roots_or_change_failure_to_success(self):
        result = self.run_make(GOFLAGS="-mod=readonly -tags=makeprobe -failfast", MAKE_PROBE_FAIL="yes")
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("deliberate fixture failure", result.stdout)
        self.assert_executed_once()


if __name__ == "__main__":
    unittest.main()
