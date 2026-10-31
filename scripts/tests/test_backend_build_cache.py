#!/usr/bin/env python3
"""Build objects accelerate CI; they must never become a test verdict."""

import os
from pathlib import Path
import subprocess
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


if __name__ == "__main__":
    unittest.main()
