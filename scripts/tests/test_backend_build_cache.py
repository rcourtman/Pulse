#!/usr/bin/env python3
"""Build objects accelerate CI; they must never become a test verdict."""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/build-and-test.yml"
CACHE_ACTION = "55cc8345863c7cc4c66a329aec7e433d2d1c52a9"
CACHE_PREFIX = (
    "backend-race-objects-v1-${{ runner.os }}-${{ runner.arch }}-ubuntu24-"
    "${{ steps.go-setup.outputs.go-version }}-${{ hashFiles('go.mod', 'go.sum') }}-"
)
CACHE_PATH = "${{ steps.go-build-cache.outputs.path }}"
RESTORE = "Restore backend race build objects"
SAVE = "Save backend race build objects"
COMPILE = "Compile affected Go race-test binaries without executing them"


class BackendBuildCacheTest(unittest.TestCase):
    def setUp(self):
        self.workflow = yaml.safe_load(WORKFLOW.read_text())
        self.jobs = self.workflow["jobs"]

    def step(self, job, name):
        found = [step for step in self.jobs[job]["steps"] if step.get("name") == name]
        self.assertEqual(len(found), 1, (job, name))
        return found[0]

    def test_same_source_is_preferred_before_same_graph_build_objects(self):
        for job in ("test-compile", "backend", "backend-api"):
            with self.subTest(job=job):
                setup = self.step(job, "Set up Go")
                self.assertEqual(setup["id"], "go-setup")
                self.assertEqual(setup["with"], {"go-version-file": "go.mod", "cache": True})
                restore = self.step(job, RESTORE)
                self.assertEqual(restore["uses"], f"actions/cache/restore@{CACHE_ACTION}")
                self.assertEqual(restore["id"], "race-cache")
                self.assertEqual(restore["with"], {
                    "path": CACHE_PATH,
                    "key": CACHE_PREFIX + "${{ github.sha }}",
                    "restore-keys": CACHE_PREFIX,
                })
                # No module tree, source, binary, credentials or alternate ref
                # is exported. The Go tool validates each restored build object.
                locate = self.step(job, "Locate backend Go build cache")
                self.assertEqual(locate["id"], "go-build-cache")
                self.assertEqual(locate["run"], 'echo "path=$(go env GOCACHE)" >> "$GITHUB_OUTPUT"')
                names = [step["name"] for step in self.jobs[job]["steps"]]
                self.assertLess(names.index("Set up Go"), names.index(locate["name"]))
                self.assertLess(names.index(locate["name"]), names.index(RESTORE))
                next_step = "Provide frontend test embed stub" if job == "test-compile" else "Provide frontend embed stub"
                self.assertLess(names.index(RESTORE), names.index(next_step))
                for step in (setup, locate, restore):
                    if job == "test-compile":
                        self.assertNotIn("if", step)
                    else:
                        self.assertEqual(step["if"], "needs.changes.outputs.code == 'true'")

    def test_only_successful_compiler_writes_before_dependent_tests_start(self):
        job = self.jobs["test-compile"]
        self.assertEqual(job["needs"], "changes")
        self.assertEqual(job["if"], "needs.changes.outputs.go_compile == 'true'")
        compile_step = self.step("test-compile", COMPILE)
        self.assertEqual(compile_step["run"], "bash scripts/compile-go-tests.sh")
        self.assertNotIn("if", compile_step)
        self.assertNotIn("continue-on-error", compile_step)
        save = self.step("test-compile", SAVE)
        self.assertEqual(save, {
            "name": SAVE,
            # GitHub's implicit success() must remain in effect. A failed or
            # cancelled compiler must never publish a newly prepared cache.
            "if": "steps.race-cache.outputs.cache-hit != 'true'",
            "continue-on-error": True,
            "uses": f"actions/cache/save@{CACHE_ACTION}",
            "with": {"path": CACHE_PATH, "key": "${{ steps.race-cache.outputs.cache-primary-key }}"},
        })
        self.assertEqual(job["steps"][-2:], [compile_step, save])
        for name in ("backend", "backend-api"):
            self.assertEqual(self.jobs[name]["needs"], ["changes", "test-compile"])
            self.assertFalse(any("actions/cache/save@" in step.get("uses", "")
                                 for step in self.jobs[name]["steps"]))

    def test_cache_failure_or_miss_cannot_skip_validation_or_grant_admission(self):
        for name in ("test-compile", "backend", "backend-api"):
            job = self.jobs[name]
            self.assertNotIn("continue-on-error", job)
            for step in job["steps"]:
                optional = step.get("name") in (RESTORE, SAVE)
                self.assertEqual(step.get("continue-on-error", False), optional, step["name"])
                if not optional:
                    self.assertNotIn("race-cache", str(step))
                    self.assertNotIn("cache-hit", str(step))
            # A cache success/failure is not an output of a validation job.
            self.assertNotIn("outputs", job)
        compiler = (ROOT / "scripts/compile-go-tests.sh").read_text()
        self.assertIn("go test -race -vet=off -count=1 -exec /bin/true ./...", compiler)
        self.assertEqual(self.workflow["permissions"], {"contents": "read"})

    def test_cache_path_command_uses_actual_tool_and_quotes_spaced_paths(self):
        for name in ("test-compile", "backend", "backend-api"):
            with self.subTest(job=name), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                go = root / "go"
                go.write_text('#!/bin/sh\n[ "$*" = "env GOCACHE" ] || exit 91\nprintf "%s\\n" "$FIXTURE_CACHE"\n')
                go.chmod(0o700)
                output = root / "output with spaces"
                cache = root / "only build objects"
                result = subprocess.run(
                    ["bash", "-euo", "pipefail", "-c", self.step(name, "Locate backend Go build cache")["run"]],
                    env={**os.environ, "PATH": f"{root}:{os.environ['PATH']}",
                         "GITHUB_OUTPUT": str(output), "FIXTURE_CACHE": str(cache)},
                    capture_output=True, text=True, timeout=5,
                )
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(output.read_text(), f"path={cache}\n")

    def embed_step(self, job):
        name = "Provide frontend test embed stub" if job == "test-compile" else "Provide frontend embed stub"
        return self.step(job, name)

    def test_gate_and_consumers_produce_identical_embed_bytes(self):
        expected = (b"<!DOCTYPE html>\n<html>\n<head><title>Test Asset</title></head>\n"
                    b"<body><h1>Pulse Test Asset</h1></body>\n</html>\n")
        for job in ("test-compile", "backend", "backend-api"):
            with self.subTest(job=job), tempfile.TemporaryDirectory(prefix="embed cache ") as temporary:
                root = Path(temporary)
                (root / "scripts").mkdir()
                shutil.copyfile(ROOT / "scripts/ensure_test_assets.sh", root / "scripts/ensure_test_assets.sh")
                step = self.embed_step(job)
                self.assertEqual(step["run"], "bash scripts/ensure_test_assets.sh")
                result = subprocess.run(["bash", "-euo", "pipefail", "-c", step["run"]],
                                        cwd=root, capture_output=True, text=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)
                files = {str(path.relative_to(root)): path.read_bytes()
                         for path in (root / "internal").rglob("*") if path.is_file()}
                self.assertEqual(files, {"internal/api/frontend-modern/dist/index.html": expected})

    def test_every_cache_participant_preserves_an_existing_real_embed(self):
        for job in ("test-compile", "backend", "backend-api"):
            with self.subTest(job=job), tempfile.TemporaryDirectory(prefix="real embed ") as temporary:
                root = Path(temporary)
                (root / "scripts").mkdir()
                shutil.copyfile(ROOT / "scripts/ensure_test_assets.sh", root / "scripts/ensure_test_assets.sh")
                dist = root / "internal/api/frontend-modern/dist"
                (dist / "assets").mkdir(parents=True)
                expected = {"index.html": b"<!DOCTYPE html><script src='/assets/current.js'></script>\n",
                            "assets/current.js": b"console.log('existing checked production embed');\n"}
                for name, content in expected.items():
                    (dist / name).write_bytes(content)
                for _ in range(2):
                    result = subprocess.run(["bash", "-euo", "pipefail", "-c", self.embed_step(job)["run"]],
                                            cwd=root, capture_output=True, text=True, timeout=5)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual({str(path.relative_to(dist)): path.read_bytes()
                                      for path in dist.rglob("*") if path.is_file()}, expected)

    @unittest.skipUnless(shutil.which("go"), "native cache fixture requires Go (provided by script-smoke CI)")
    def test_real_ordinary_worker_reexecutes_tests_after_an_external_fixture_changes(self):
        # Go cannot observe files read by shell children for result-cache
        # invalidation. Exercise the actual worker, not a second flag recipe.
        # Compilation may stay warm; a previous passing verdict must not hide
        # the changed fixture. No Pulse runtime/database test is executed here.
        driver = """
import importlib.util, json, sys
from pathlib import Path
spec = importlib.util.spec_from_file_location('rest_worker', sys.argv[1])
worker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(worker)
worker.ROOT = Path(sys.argv[2])
result = worker.run_task(('example.com/ordinarycache',
                        ['bash', '-euo', 'pipefail', '-c', worker.ORDINARY_COMMAND]))
print(json.dumps(result))
raise SystemExit(result['exit'])
"""
        with tempfile.TemporaryDirectory(prefix="ordinary Go cache ") as temporary:
            root = Path(temporary)
            marker = root / "external fixture"
            (root / "go.mod").write_text("module example.com/ordinarycache\n\ngo 1.23\n")
            (root / "fixture_test.go").write_text(
                'package ordinarycache\nimport ("os/exec"; "testing")\n'
                'func TestExternalFixture(t *testing.T) {\n'
                ' b, err := exec.Command("/bin/cat", ' + json.dumps(str(marker)) + ').Output()\n'
                ' if err != nil {t.Fatal(err)}\n'
                ' if string(b) != "current fixture\\n" {t.Fatal("external fixture changed")}\n}\n'
            )
            for flags in ("-mod=readonly", "-mod=readonly -count=0 -p=8"):
                with self.subTest(inherited_flags=flags):
                    environment = {**os.environ, "GOWORK": "off", "GOPROXY": "off",
                                   "GOSUMDB": "off", "GOTOOLCHAIN": "local",
                                   "GOMAXPROCS": "2", "GOFLAGS": flags}

                    def execute():
                        return subprocess.run(
                            [sys.executable, "-I", "-c", driver,
                             str(ROOT / "scripts/release_control/run_go_rest_shard.py"), str(root)],
                            cwd=root, env=environment, capture_output=True, text=True, timeout=120,
                        )

                    marker.write_text("current fixture\n")
                    first = execute()
                    self.assertEqual(first.returncode, 0, first.stdout + first.stderr)
                    self.assertNotIn("(cached)", first.stdout)
                    marker.write_text("changed fixture\n")
                    changed = execute()
                    self.assertNotEqual(changed.returncode, 0, changed.stdout + changed.stderr)
                    self.assertIn("external fixture changed", changed.stdout)
                    self.assertNotIn("(cached)", changed.stdout)
                    observation = json.loads(changed.stdout.splitlines()[-1])
                    self.assertTrue(observation["started"])
                    self.assertEqual(observation["exit"], changed.returncode)


if __name__ == "__main__":
    unittest.main()
