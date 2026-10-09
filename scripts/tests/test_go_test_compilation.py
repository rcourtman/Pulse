#!/usr/bin/env python3
"""Compilation admission must reject broken fixtures without executing tests."""

import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
COMPILER = ROOT / "scripts/compile-go-tests.sh"
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"


def job(source, name):
    match = re.search(rf"^  {re.escape(name)}:\n(.*?)(?=^  [\w-]+:\n|\Z)", source, re.M | re.S)
    if match is None:
        raise AssertionError(f"missing workflow job {name}")
    return match.group(1)


def prerequisite_script(source):
    match = re.search(
        r"      - name: Require compiled backend tests\n[\s\S]*?        run: \|\n((?:          [^\n]*\n|\n)+)",
        source,
    )
    if match is None:
        raise AssertionError("missing compilation prerequisite")
    return "\n".join(line[10:] for line in match.group(1).splitlines())


class GoTestCompilationTest(unittest.TestCase):
    def compile_fixture(self, files):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "go.mod").write_text("module example.invalid/compileproof\n\ngo 1.24\n")
            for name, content in files.items():
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(content)
            sentinel = root / "executed"
            result = subprocess.run(
                ["bash", str(COMPILER)], cwd=root,
                env={**os.environ, "GOWORK": "off", "GOPROXY": "off", "CI_COMPILE_SENTINEL": str(sentinel)},
                capture_output=True, text=True, timeout=120,
            )
            self.assertFalse(sentinel.exists(), "compile-only admission executed fixture code")
            return result

    def test_links_all_packages_without_init_testmain_tests_or_examples(self):
        result = self.compile_fixture({
            "proof_test.go": '''package compileproof
import ("os"; "testing")
func record() { os.WriteFile(os.Getenv("CI_COMPILE_SENTINEL"), []byte("executed"), 0600) }
func init() { record(); panic("initialiser must not execute") }
func TestMain(m *testing.M) { record(); os.Exit(99) }
func TestMustNotExecute(t *testing.T) { record(); t.Fatal("test must not execute") }
func ExampleMustNotExecute() { record(); panic("example must not execute") }
''',
            "child/proof_test.go": '''package child
import "testing"
func TestMustNotExecute(t *testing.T) { t.Fatal("child test must not execute") }
''',
        })
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("example.invalid/compileproof/child", result.stdout)
        self.assertIn("runtime tests remain required", result.stdout)

    def test_rejects_a_test_only_removed_api_before_runtime(self):
        result = self.compile_fixture({
            "api.go": "package compileproof\ntype Resource struct { ActiveAlerts []string }\n",
            "child/stale_test.go": '''package child
import ("testing"; api "example.invalid/compileproof")
func TestStaleFixture(t *testing.T) { _ = api.Resource{Alerts: []api.ResourceAlertFrontend{}} }
''',
        })
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("ResourceAlertFrontend", result.stderr)
        self.assertNotIn("runtime tests remain required", result.stdout)

    def test_rejects_a_non_test_compilation_error(self):
        result = self.compile_fixture({"broken.go": "package compileproof\nvar _ = removedAPI\n"})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("removedAPI", result.stderr)

    def test_admission_is_reserved_for_go_affecting_source_and_control(self):
        required = ["internal/api/server.go", "pkg/model_test.go", "scripts/installtests/contract_test.go",
                    "go.mod", "go.sum", "internal/api/embed.json", "cmd/pulse/main.go",
                    "pkg/native/helper.c", "scripts/compile-go-tests.sh",
                    "scripts/go-test-compile-required.sh", "scripts/ensure_test_assets.sh",
                    ".github/workflows/build-and-test.yml"]
        irrelevant = ["frontend-modern/src/index.tsx", "frontend-modern/package-lock.json",
                      "docs/guide.md", "README.md", "scripts/unrelated.sh"]
        for paths, expected in [(required, "true"), (irrelevant, "false"), ([], "false")]:
            for path in paths or [""]:
                with self.subTest(path=path):
                    result = subprocess.run(["bash", str(ROOT / "scripts/go-test-compile-required.sh")],
                                            input=path + "\n", capture_output=True, text=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout.strip(), expected)
        # Do not use grep -q: under pipefail an early match in a large source
        # update must not kill the writer with SIGPIPE and lose admission.
        script = 'set -euo pipefail; printf "%s\n" "$CHANGED" | bash scripts/go-test-compile-required.sh'
        result = subprocess.run(["bash", "-c", script], cwd=ROOT,
                                env={**os.environ, "CHANGED": "go.mod\n" + "frontend-modern/src/long.tsx\n" * 2000},
                                capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.strip(), "true")
        changes = job(WORKFLOW.read_text(), "changes")
        self.assertIn('echo "go_compile=true" >> "$GITHUB_OUTPUT"', changes)
        self.assertIn('go_compile=$(printf', changes)
        self.assertIn('echo "go_compile=$go_compile" >> "$GITHUB_OUTPUT"', changes)
        self.assertIn('go_compile=false', changes)

    def test_gate_keeps_source_toolchain_embed_and_no_runtime_contract(self):
        source = WORKFLOW.read_text()
        compile_job = job(source, "test-compile")
        for expected in (
            "needs: changes", "if: needs.changes.outputs.go_compile == 'true'",
            "runs-on: ubuntu-24.04", "timeout-minutes: 15",
            "persist-credentials: false", "go-version-file: go.mod",
            "run: bash scripts/ensure_test_assets.sh", "run: bash scripts/compile-go-tests.sh",
        ):
            self.assertIn(expected, compile_job)
        self.assertLess(compile_job.index("ensure_test_assets.sh"), compile_job.index("compile-go-tests.sh"))
        self.assertNotIn("continue-on-error:", compile_job)
        self.assertIn("go test -race -vet=off -count=1 -exec /bin/true ./...", COMPILER.read_text())
        # Frontend work does not wait for the Go-only gate.
        self.assertIn("needs: changes", job(source, "frontend"))

    def test_required_checks_fail_on_bad_compile_and_pass_docs_only_skip(self):
        source = WORKFLOW.read_text()
        for name in ("backend", "backend-api"):
            section = job(source, name)
            self.assertIn("needs: [changes, test-compile]", section)
            self.assertIn("if: ${{ !cancelled() && needs.changes.result == 'success' }}", section)
            self.assertIn("COMPILE_RESULT: ${{ needs.test-compile.result }}", section)
            self.assertLess(section.index("Require compiled backend tests"), section.index("Checkout repository"))
            script = prerequisite_script(section)
            for code in ("true", "false"):
                for result in ("success", "failure", "cancelled", "skipped", ""):
                    with self.subTest(job=name, code=code, result=result):
                        observed = subprocess.run(
                            ["bash", "-euo", "pipefail", "-c", script],
                            env={**os.environ, "COMPILE_REQUIRED": code, "COMPILE_RESULT": result},
                            capture_output=True, text=True,
                        )
                        expected = 0 if code == "false" or result == "success" else 1
                        self.assertEqual(observed.returncode, expected, observed.stdout + observed.stderr)
            # Compile-only success cannot replace these full race/vet runs.
            self.assertIn("go test -race -timeout 50m", section)
        self.assertIn("shard: [rest-0, rest-1]", job(source, "backend"))
        self.assertIn("index: [0, 1, 2, 3, 4]", job(source, "backend-api"))
        self.assertIn("name: Backend tests (api)", job(source, "backend-api-verdict"))
        self.assertIn("if: always()", job(source, "backend-api-verdict"))


if __name__ == "__main__":
    unittest.main()
