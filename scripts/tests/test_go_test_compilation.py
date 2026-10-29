#!/usr/bin/env python3
"""Compilation admission must reject broken fixtures without executing tests."""

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
COMPILER = ROOT / "scripts/compile-go-tests.sh"
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
SELECTOR = ROOT / "scripts/select-go-test-compile-packages.py"
SPEC = importlib.util.spec_from_file_location("go_compile_selection", SELECTOR)
SELECTION = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SELECTION)


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
                    "scripts/select-go-test-compile-packages.py",
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
            "fetch-depth: 2", "PULSE_GO_COMPILE_HEAD: ${{ github.sha }}",
            "PULSE_GO_COMPILE_BASE: ${{ github.event_name == 'pull_request' && github.event.pull_request.base.sha || github.event.before }}",
        ):
            self.assertIn(expected, compile_job)
        self.assertLess(compile_job.index("ensure_test_assets.sh"), compile_job.index("compile-go-tests.sh"))
        self.assertNotIn("continue-on-error:", compile_job)
        self.assertIn("if: needs.changes.outputs.go_compile == 'true'", compile_job)
        self.assertIn("go test -race -vet=off -count=1 -exec /bin/true ./...", COMPILER.read_text())
        # Frontend work does not wait for the Go-only gate.
        self.assertIn("needs: changes", job(source, "frontend-checks"))
        self.assertIn("needs: [changes, frontend-checks]", job(source, "frontend-unit"))
        self.assertIn("needs: [changes, frontend-checks, frontend-unit]", job(source, "frontend"))

    def test_required_checks_admit_only_explicit_consistent_results(self):
        source = WORKFLOW.read_text()
        for name in ("backend", "backend-api"):
            section = job(source, name)
            self.assertIn("needs: [changes, test-compile]", section)
            self.assertIn("if: ${{ !cancelled() && needs.changes.result == 'success' }}", section)
            self.assertIn("CODE_CHANGED: ${{ needs.changes.outputs.code }}", section)
            self.assertIn("COMPILE_REQUIRED: ${{ needs.changes.outputs.go_compile }}", section)
            self.assertIn("COMPILE_RESULT: ${{ needs.test-compile.result }}", section)
            self.assertLess(section.index("Require compiled backend tests"), section.index("Checkout repository"))
            script = prerequisite_script(section)
            admitted = {("true", "true", "success"), ("true", "false", "skipped"),
                        ("false", "false", "skipped")}
            for code in ("true", "false", "", "unknown", "true\nfalse"):
                for required in ("true", "false", "", "unknown"):
                    for result in ("success", "failure", "cancelled", "skipped", ""):
                        with self.subTest(job=name, code=code, required=required, result=result):
                            observed = subprocess.run(
                                ["bash", "-euo", "pipefail", "-c", script],
                                env={**os.environ, "CODE_CHANGED": code,
                                     "COMPILE_REQUIRED": required, "COMPILE_RESULT": result},
                                capture_output=True, text=True, timeout=5,
                            )
                            expected = 0 if (code, required, result) in admitted else 1
                            self.assertEqual(observed.returncode, expected, observed.stdout + observed.stderr)
                            if expected:
                                self.assertIn("no runtime tests started", observed.stdout)
                                self.assertIn("Detect changed areas", observed.stdout)
            self.assertIn("- name: Report inapplicable backend runtime\n"
                          "        if: needs.changes.outputs.code == 'false'", section)
            self.assertIn("no backend tests ran in this shard", section)
            # Compile-only success cannot replace these full race/vet runs.
            self.assertIn("go test -race -timeout 50m", section)
        self.assertIn("shard: [rest-0, rest-1]", job(source, "backend"))
        self.assertIn("index: [0, 1, 2, 3, 4]", job(source, "backend-api"))
        self.assertIn("name: Backend tests (api)", job(source, "backend-api-verdict"))
        self.assertIn("if: always()", job(source, "backend-api-verdict"))

    def test_actual_api_verdict_rejects_unknown_results_and_distinguishes_no_tests(self):
        section = job(WORKFLOW.read_text(), "backend-api-verdict")
        self.assertIn("needs: [changes, backend-api]", section)
        for binding in ("CHANGE_RESULT: ${{ needs.changes.result }}",
                        "CODE_CHANGED: ${{ needs.changes.outputs.code }}",
                        "API_SHARDS_RESULT: ${{ needs.backend-api.result }}"):
            self.assertIn(binding, section)
        script = section.split("        run: |\n", 1)[1]
        script = "\n".join(line[10:] for line in script.splitlines())
        for change in ("success", "failure", "cancelled", "skipped", ""):
            for code in ("true", "false", "", "unknown"):
                for shards in ("success", "failure", "cancelled", "skipped", ""):
                    with self.subTest(change=change, code=code, shards=shards):
                        observed = subprocess.run(
                            ["bash", "-euo", "pipefail", "-c", script],
                            env={**os.environ, "CHANGE_RESULT": change,
                                 "CODE_CHANGED": code, "API_SHARDS_RESULT": shards},
                            capture_output=True, text=True, timeout=5,
                        )
                        expected = 0 if change == shards == "success" and code in ("true", "false") else 1
                        self.assertEqual(observed.returncode, expected, observed.stdout + observed.stderr)
                        if expected == 0:
                            if code == "true":
                                self.assertIn("Every internal/api runtime shard passed", observed.stdout)
                                self.assertNotIn("not applicable", observed.stdout)
                            else:
                                self.assertIn("not applicable", observed.stdout)
                                self.assertIn("no internal/api tests ran", observed.stdout)
                                self.assertNotIn("passed", observed.stdout)


class CompileSelectionTest(unittest.TestCase):
    def raw(self, path=b"child/proof_test.go", status=b"M", old_mode=b"100644", new_mode=b"100644"):
        old_blob = b"0" * 40 if status == b"A" else b"a" * 40
        return b":" + old_mode + b" " + new_mode + b" " + old_blob + b" " + b"b" * 40 + b" " + status + b"\0" + path + b"\0"

    def select(self, raw, *extra):
        tree = b"go.mod\0child/proof_test.go\0" + b"".join(path + b"\0" for path in extra)
        return SELECTION.changed_test_packages(raw, tree)

    def test_complete_regular_test_diff_is_sorted_and_deduplicates_packages(self):
        raw = self.raw() + self.raw(b"child/other_test.go", b"A", b"000000") + self.raw(b"root_test.go")
        self.assertEqual(self.select(raw, b"child/other_test.go", b"root_test.go"), [".", "./child"])

    def test_product_graph_control_docs_and_late_non_test_change_retain_full_compilation(self):
        paths = [f"child/proof{index}_test.go".encode() for index in range(4000)]
        raw = b"".join(self.raw(path) for path in paths)
        self.assertEqual(self.select(raw, *paths), ["./child"])
        for path in (b"go.mod", b"go.sum", b"internal/api/server.go", b"child/embed.json",
                     b".github/workflows/build-and-test.yml", b"docs/guide.md", b"install.sh",
                     b"scripts/compile-go-tests.sh", b"frontend-modern/src/App.tsx"):
            with self.subTest(path=path):
                self.assertEqual(self.select(raw + self.raw(path), *paths, path), SELECTION.FULL)
                self.assertEqual(self.select(self.raw() + self.raw(path), path), SELECTION.FULL)

    def test_deleted_renamed_symlink_noncanonical_and_ignored_paths_are_not_focused(self):
        for raw in (self.raw(status=b"D", new_mode=b"000000"), self.raw(status=b"R100"),
                    self.raw(new_mode=b"120000"), self.raw(old_mode=b"100755")):
            self.assertEqual(self.select(raw), SELECTION.FULL)
        for path in (b"child/proof_test.go\ninternal/api/server.go", b"child/../proof_test.go",
                     b"/proof_test.go", b"-proof_test.go", b"child/_ignored_test.go",
                     b".ignored/proof_test.go", b"child/quoted\"_test.go", b"child/space name_test.go"):
            with self.subTest(path=path):
                self.assertEqual(self.select(self.raw(path), path), SELECTION.FULL)
        for path in (b"vendor/child/proof_test.go", b"child/testdata/proof_test.go"):
            self.assertEqual(self.select(self.raw(path), path), SELECTION.FULL)

    def test_missing_empty_truncated_duplicate_or_inconsistent_evidence_is_not_focused(self):
        for raw in (b"", self.raw()[:-1], b"malformed\0child/proof_test.go\0",
                    self.raw() + b"partial\0", self.raw() * 2,
                    self.raw(status=b"A"), self.raw(old_mode=b"000000")):
            self.assertEqual(self.select(raw), SELECTION.FULL)
        for tree in (b"", b"go.mod\0child/proof_test.go", b"child/proof_test.go\0",
                     b"go.mod\0", b"go.mod\0child/proof_test.go\0child/proof_test.go\0"):
            self.assertEqual(SELECTION.changed_test_packages(self.raw(), tree), SELECTION.FULL)

    def test_nested_module_and_workspace_are_not_root_module_test_changes(self):
        self.assertEqual(self.select(self.raw(), b"child/go.mod"), SELECTION.FULL)
        self.assertEqual(self.select(self.raw(), b"go.work"), SELECTION.FULL)
        self.assertEqual(self.select(self.raw(), b"unrelated/go.mod"), ["./child"])

    def test_unavailable_or_wrong_source_never_narrows(self):
        for base, head in (("", "b" * 40), ("main", "b" * 40), ("a" * 40, "HEAD")):
            with patch.object(SELECTION, "read_git") as read:
                self.assertEqual(SELECTION.compile_packages(base, head), SELECTION.FULL)
                read.assert_not_called()
        for observations in ((b"nested/\n",), (b"\n", b"c" * 40 + b"\n")):
            with patch.object(SELECTION, "read_git", side_effect=observations):
                self.assertEqual(SELECTION.compile_packages("a" * 40, "b" * 40), SELECTION.FULL)
        for error in (OSError("unavailable"), subprocess.CalledProcessError(1, "git"),
                      subprocess.TimeoutExpired("git", 30)):
            with patch.object(SELECTION, "read_git", side_effect=error):
                self.assertEqual(SELECTION.compile_packages("a" * 40, "b" * 40), SELECTION.FULL)

    def test_source_diff_disables_renames_external_diff_and_abbreviation(self):
        tree = b"go.mod\0child/proof_test.go\0"
        with patch.object(SELECTION, "read_git", side_effect=[b"\n", b"b" * 40 + b"\n", b"", self.raw(), tree]) as read, \
                patch.object(SELECTION, "root_package_directories", return_value={str(Path("child").resolve())}):
            self.assertEqual(SELECTION.compile_packages("a" * 40, "b" * 40), ["./child"])
            self.assertEqual(read.call_args_list[2].args, ("diff", "--quiet", "--no-ext-diff", "b" * 40, "--"))
            self.assertEqual(read.call_args_list[3].args, (
                "diff", "--no-ext-diff", "--no-renames", "--raw", "--abbrev=40", "-z", "a" * 40, "b" * 40, "--"))

    def test_unlisted_or_unavailable_actual_root_packages_retain_full_compilation(self):
        observations = [b"\n", b"b" * 40 + b"\n", b"", self.raw(), b"go.mod\0child/proof_test.go\0"]
        with patch.object(SELECTION, "read_git", side_effect=observations), \
                patch.object(SELECTION, "root_package_directories", return_value=set()):
            self.assertEqual(SELECTION.compile_packages("a" * 40, "b" * 40), SELECTION.FULL)
        with patch.object(SELECTION, "read_git", side_effect=observations), \
                patch.object(SELECTION, "root_package_directories", side_effect=OSError("unavailable")):
            self.assertEqual(SELECTION.compile_packages("a" * 40, "b" * 40), SELECTION.FULL)


class ExactTreeCompileTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git("init", "-q", "--initial-branch=main")
        self.write("go.mod", "module example.invalid/compileproof\n\ngo 1.24\n")
        for script in (COMPILER, SELECTOR):
            path = self.root / "scripts" / script.name
            path.parent.mkdir(exist_ok=True)
            shutil.copyfile(script, path)
        self.write("api.go", "package compileproof\ntype Resource struct { ActiveAlerts []string }\n")
        for package in ("child", "other"):
            self.write(package + "/proof_test.go", '''package ''' + package + '''
import ("os"; "testing")
func record() { os.WriteFile(os.Getenv("CI_COMPILE_SENTINEL"), []byte("executed"), 0600) }
func init() { record(); panic("initialiser must not execute") }
func TestMain(m *testing.M) { record(); os.Exit(99) }
func TestMustNotExecute(t *testing.T) { record(); t.Fatal("must not execute") }
func ExampleMustNotExecute() { record(); panic("must not execute") }
''')
        self.base = self.commit()

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text)

    def git(self, *args):
        return subprocess.check_output(
            ["git", "-c", "user.name=CI fixture", "-c", "user.email=ci@example.invalid",
             "-c", "core.hooksPath=/dev/null", *args], cwd=self.root, stderr=subprocess.PIPE, text=True,
        ).strip()

    def commit(self):
        self.git("add", "-A")
        self.git("commit", "-q", "-m", "Fixture tree change")
        return self.git("rev-parse", "HEAD")

    def modify_test(self, suffix="\n// changed fixture\n"):
        path = self.root / "child/proof_test.go"
        path.write_text(path.read_text() + suffix)
        return self.commit()

    def plan(self, head, base=None):
        result = subprocess.run(["python3", str(SELECTOR), "--base", base or self.base, "--head", head],
                                cwd=self.root, capture_output=True, text=True, check=True, timeout=10)
        return result.stdout.strip()

    def compile(self, head, base=None):
        with tempfile.TemporaryDirectory() as evidence:
            sentinel = Path(evidence) / "executed"
            result = subprocess.run(
                ["bash", str(self.root / "scripts/compile-go-tests.sh")], cwd=self.root,
                env={**os.environ, "GOWORK": "off", "GOPROXY": "off", "CI_COMPILE_SENTINEL": str(sentinel),
                     "PULSE_GO_COMPILE_BASE": base or self.base, "PULSE_GO_COMPILE_HEAD": head},
                capture_output=True, text=True, timeout=120,
            )
            self.assertFalse(sentinel.exists(), "compile gate executed fixture code")
            return result

    def test_real_added_and_modified_tests_compile_once_per_changed_package_without_execution(self):
        self.write("child/added_test.go", "package child\n")
        head = self.modify_test()
        self.assertEqual(self.plan(head), "./child")
        result = self.compile(head)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Compiling 1 changed-test package(s)", result.stdout)
        self.assertIn("example.invalid/compileproof/child", result.stdout)
        self.assertNotIn("example.invalid/compileproof/other", result.stdout)
        self.assertIn("runtime tests remain required", result.stdout)

    def test_real_broken_changed_test_is_rejected_before_execution(self):
        head = self.modify_test("\nvar _ = removedAPI\n")
        result = self.compile(head)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("removedAPI", result.stderr)
        self.assertNotIn("runtime tests remain required", result.stdout)

    def test_real_mixed_source_range_preserves_full_inert_compilation(self):
        self.modify_test()
        self.write("api.go", "package compileproof\ntype Resource struct { ActiveAlerts []string; Changed bool }\n")
        head = self.commit()
        self.assertEqual(self.plan(head), "./...")
        result = self.compile(head)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Compiling all Go race-test binaries", result.stdout)
        self.assertIn("example.invalid/compileproof/child", result.stdout)
        self.assertIn("example.invalid/compileproof/other", result.stdout)

    def test_real_deleted_renamed_symlink_and_nested_module_tests_use_full_plan(self):
        (self.root / "child/proof_test.go").rename(self.root / "child/moved_test.go")
        renamed = self.commit()
        self.assertEqual(self.plan(renamed), "./...")
        (self.root / "child/moved_test.go").unlink()
        deleted = self.commit()
        self.assertEqual(self.plan(deleted, renamed), "./...")
        (self.root / "child/link_test.go").symlink_to("../other/proof_test.go")
        linked = self.commit()
        self.assertEqual(self.plan(linked, deleted), "./...")
        self.write("nested/go.mod", "module example.invalid/nested\n\ngo 1.24\n")
        self.write("nested/proof_test.go", "package nested\n")
        nested_base = self.commit()
        self.write("nested/proof_test.go", "package nested\n// changed\n")
        self.assertEqual(self.plan(self.commit(), nested_base), "./...")

    def test_real_empty_unknown_mismatching_and_subdirectory_source_use_full_plan(self):
        self.assertEqual(self.plan(self.base), "./...")
        head = self.modify_test()
        self.assertEqual(self.plan(head, "0" * 40), "./...")
        self.assertEqual(self.plan(self.base, head), "./...")
        result = subprocess.run(["python3", str(SELECTOR), "--base", self.base, "--head", head],
                                cwd=self.root / "child", capture_output=True, text=True, check=True)
        self.assertEqual(result.stdout.strip(), "./...")

    def test_broken_empty_or_invalid_selector_cannot_admit_success_without_compilation(self):
        head = self.modify_test()
        for body in ("raise SystemExit(7)\n", "", "print('--help')\n", "print('./../outside')\n"):
            with self.subTest(body=body):
                self.write("scripts/select-go-test-compile-packages.py", body)
                result = self.compile(head)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("runtime tests remain required", result.stdout)


if __name__ == "__main__":
    unittest.main()
