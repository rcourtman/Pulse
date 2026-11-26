"""Exercise the hosted memory watch's stable source selection without network access."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/memory-regression.yml"
REPO, TAG, SHA, TAG_SHA = "rcourtman/Pulse", "v6.8.0", "a" * 40, "b" * 40
LATEST = f"repos/{REPO}/releases/latest"
REF = f"repos/{REPO}/git/ref/tags/{TAG}"
OBJECT = f"repos/{REPO}/git/tags/{TAG_SHA}"
RELEASE = {"tag_name": TAG, "draft": False, "prerelease": False, "immutable": True}
LIGHTWEIGHT = {"ref": f"refs/tags/{TAG}", "object": {"type": "commit", "sha": SHA}}
ANNOTATED = {"ref": f"refs/tags/{TAG}", "object": {"type": "tag", "sha": TAG_SHA}}
PEELED = {"tag": TAG, "sha": TAG_SHA, "object": {"type": "commit", "sha": SHA}}
CANONICAL_MAIN = "github.repository == 'rcourtman/Pulse' && github.ref == 'refs/heads/main'"


class MemoryWatchSources(unittest.TestCase):
    def setUp(self):
        self.workflow = yaml.safe_load(WORKFLOW.read_text())
        self.jobs = self.workflow["jobs"]

    def run_resolver(self, responses):
        with tempfile.TemporaryDirectory() as tmp:
            scratch = Path(tmp)
            fixture = scratch / "gh"
            fixture.write_text(
                '#!/bin/bash\nset -eu\n'
                '[[ $# == 2 && "$1" == api ]]\n'
                'printf "%s\\n" "$2" >> "$CALLS"\n'
                'if [[ ! -f "$FIXTURES/$2" ]]; then echo "gh: HTTP 403" >&2; exit 1; fi\n'
                'cat "$FIXTURES/$2"\n'
            )
            fixture.chmod(0o700)
            fixtures = scratch / "fixtures"
            for endpoint, response in responses.items():
                path = fixtures / endpoint
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(json.dumps(response))
            github_output, calls = scratch / "github-output", scratch / "calls"
            result = subprocess.run(
                ["bash", "-c", self.jobs["resolve-stable"]["steps"][0]["run"]],
                env={"PATH": f"{scratch}:{os.environ['PATH']}", "GITHUB_REPOSITORY": REPO,
                     "GITHUB_OUTPUT": str(github_output), "FIXTURES": str(fixtures), "CALLS": str(calls)},
                capture_output=True, text=True, timeout=10,
            )
            return result, calls.read_text().splitlines(), (
                github_output.read_text() if github_output.exists() else None
            )

    def test_resolves_one_exact_stable_reference_for_both_git_tag_types(self):
        for tag_ref, object_reads in ((LIGHTWEIGHT, []), (ANNOTATED, [OBJECT])):
            with self.subTest(type=tag_ref["object"]["type"]):
                result, calls, output = self.run_resolver({LATEST: RELEASE, REF: tag_ref, OBJECT: PEELED})
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(calls, [LATEST, REF, *object_reads])
                self.assertEqual(output, f"tag={TAG}\nsha={SHA}\n")

    def test_invalid_stable_identity_and_refusal_never_yield_a_baseline(self):
        for responses, expected_calls in (
            ({}, [LATEST]),
            ({LATEST: RELEASE}, [LATEST, REF]),
            ({LATEST: RELEASE, REF: ANNOTATED}, [LATEST, REF, OBJECT]),
            ({LATEST: RELEASE | {"draft": True}}, [LATEST]),
            ({LATEST: RELEASE | {"prerelease": True}}, [LATEST]),
            ({LATEST: RELEASE | {"tag_name": "dev"}}, [LATEST]),
            ({LATEST: RELEASE, REF: LIGHTWEIGHT | {"ref": "refs/heads/main"}}, [LATEST, REF]),
            ({LATEST: RELEASE, REF: ANNOTATED | {"object": {"type": "tag", "sha": "bad"}}}, [LATEST, REF]),
            ({LATEST: RELEASE, REF: ANNOTATED, OBJECT: PEELED | {"tag": "v6.7.0"}}, [LATEST, REF, OBJECT]),
            ({LATEST: RELEASE, REF: ANNOTATED, OBJECT: PEELED | {"sha": SHA}}, [LATEST, REF, OBJECT]),
            ({LATEST: RELEASE, REF: ANNOTATED, OBJECT: PEELED | {"object": {"type": "tree", "sha": SHA}}},
             [LATEST, REF, OBJECT]),
        ):
            with self.subTest(responses=responses):
                result, calls, output = self.run_resolver(responses)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, expected_calls)
                self.assertIsNone(output)

    def test_mutable_or_unverified_stable_never_reaches_tag_lookup(self):
        unverified = {key: value for key, value in RELEASE.items() if key != "immutable"}
        for release in (RELEASE | {"immutable": False}, unverified):
            with self.subTest(release=release):
                result, calls, output = self.run_resolver({LATEST: release, REF: LIGHTWEIGHT})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, [LATEST])
                self.assertIsNone(output)
                self.assertIn("published immutable stable release", result.stderr)

    def test_builds_and_scheduled_reporter_cannot_select_a_different_ref(self):
        resolver_if = " ".join(self.jobs["resolve-stable"].get("if", "").split())
        self.assertEqual(resolver_if, CANONICAL_MAIN +
                         " && (github.event_name == 'schedule' || github.event_name == 'workflow_dispatch')")
        self.assertEqual(self.workflow["permissions"], {"contents": "read"})
        reporter = self.jobs["report-scheduled-result"]
        self.assertEqual(" ".join(reporter["if"].split()),
                         "${{ always() && " + CANONICAL_MAIN + " && github.event_name == 'schedule' }}")
        self.assertEqual(reporter["permissions"], {"contents": "read", "issues": "write"})

    def test_all_estates_use_the_same_pair_and_require_its_identity(self):
        stable_sha = "${{ needs.resolve-stable.outputs.sha }}"
        stable_tag = "${{ needs.resolve-stable.outputs.tag }}"
        for job_name, resource in (("compare", "memory"), ("compare-agents", "memory-agents"),
                                   ("compare-proxmox", "memory-proxmox")):
            with self.subTest(job=job_name), tempfile.TemporaryDirectory() as tmp:
                job = self.jobs[job_name]
                self.assertEqual(job["needs"], "resolve-stable")
                self.assertNotIn("if", job)  # A non-successful resolver skips every comparison.
                self.assertNotIn("permissions", job)
                checkouts = [step["with"] for step in job["steps"] if step.get("uses", "").startswith("actions/checkout@")]
                self.assertEqual([(step["path"], step["ref"]) for step in checkouts],
                                 [("candidate", "${{ github.sha }}")])
                self.assertTrue(all(step.get("persist-credentials") is False for step in checkouts))
                setup_go_index, setup_go = next(
                    (idx, step) for idx, step in enumerate(job["steps"])
                    if step.get("uses", "").startswith("actions/setup-go@")
                )
                self.assertIs(setup_go["with"].get("cache"), False)
                baseline_index, baseline_step = next(
                    (idx, step) for idx, step in enumerate(job["steps"])
                    if step.get("name") == "Checkout verified stable baseline"
                )
                self.assertLess(setup_go_index, baseline_index)
                self.assertEqual(baseline_step["env"]["BASELINE_TAG"], stable_tag)
                self.assertEqual(baseline_step["env"]["BASELINE_SHA"], stable_sha)
                step, = [step for step in job["steps"] if step["name"].startswith("Measure both builds")]
                self.assertEqual(step["env"]["BASELINE_TAG"], stable_tag)
                self.assertEqual(step["env"]["BASELINE_SHA"], stable_sha)

                root = Path(tmp)
                launcher = root / "candidate/scripts/memory/compare_release_memory.sh"
                launcher.parent.mkdir(parents=True)
                launcher.write_text('printf "%s\\n" "$@"\n')
                env = {"PATH": os.environ["PATH"], "RUNNER_TEMP": tmp, "BASELINE_TAG": TAG,
                       "BASELINE_SHA": SHA, "GITHUB_SHA": TAG_SHA}
                result = subprocess.run(["bash", "-c", step["run"]], cwd=root, env=env, text=True,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(),
                                 ["baseline", "candidate", str(root / resource), TAG, SHA, TAG_SHA])
                for key in ("BASELINE_TAG", "BASELINE_SHA", "GITHUB_SHA"):
                    result = subprocess.run(["bash", "-c", step["run"]], cwd=root, env=env | {key: ""},
                                            capture_output=True, timeout=5)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.stdout, b"")

    def test_verified_baseline_checkout_checks_identity_and_canonical_tag(self):
        baseline_step, = [
            step for step in self.jobs["compare"]["steps"]
            if step.get("name") == "Checkout verified stable baseline"
        ]
        for job_name in ("compare-agents", "compare-proxmox"):
            other, = [
                step for step in self.jobs[job_name]["steps"]
                if step.get("name") == "Checkout verified stable baseline"
            ]
            self.assertEqual(other["run"], baseline_step["run"])

        def run_baseline_checkout(tag, expected_sha, *, resolved_sha=SHA, precreate_baseline=False):
            with tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                bin_dir = root / "bin"
                bin_dir.mkdir()
                calls = root / "git-calls"
                git_stub = bin_dir / "git"
                git_stub.write_text(
                    '#!/bin/bash\nset -eu\n'
                    'printf "%s\\n" "$*" >> "$CALLS"\n'
                    'if [[ "$*" == *" init baseline" ]]; then mkdir -p baseline; exit 0; fi\n'
                    'if [[ "$*" == *" rev-parse --verify "* ]]; then printf "%s\\n" "$RESOLVED_SHA"; exit 0; fi\n'
                    'exit 0\n'
                )
                git_stub.chmod(0o700)
                workdir = root / "work"
                workdir.mkdir()
                if precreate_baseline:
                    (workdir / "baseline").mkdir()
                result = subprocess.run(
                    ["bash", "-c", baseline_step["run"]],
                    cwd=workdir,
                    env={
                        "PATH": f"{bin_dir}:{os.environ['PATH']}",
                        "CALLS": str(calls),
                        "RESOLVED_SHA": resolved_sha,
                        "BASELINE_TAG": tag,
                        "BASELINE_SHA": expected_sha,
                    },
                    capture_output=True, text=True, timeout=10,
                )
                recorded = calls.read_text().splitlines() if calls.exists() else []
                return result, recorded

        result, calls = run_baseline_checkout(TAG, SHA, resolved_sha=SHA)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(calls, [
            "-c init.defaultBranch=main init baseline",
            f"-C baseline fetch --no-tags --no-recurse-submodules --depth=1 https://github.com/rcourtman/Pulse.git refs/tags/{TAG}:refs/remotes/baseline/{TAG}",
            f"-C baseline rev-parse --verify refs/remotes/baseline/{TAG}^{{commit}}",
            f"-C baseline checkout --detach {SHA}",
        ])

        result, calls = run_baseline_checkout(TAG, SHA, resolved_sha=TAG_SHA)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("resolved to", result.stderr)
        self.assertFalse(any("checkout" in call for call in calls))

        for case, tag, expected_sha, precreate in (
            ("invalid_tag_branch", "refs/heads/main", SHA, False),
            ("invalid_tag_prerelease", "v6.9.0-rc.1", SHA, False),
            ("invalid_sha", TAG, "not-a-sha", False),
            ("existing_baseline", TAG, SHA, True),
        ):
            with self.subTest(rejected=case):
                result, calls = run_baseline_checkout(tag, expected_sha, precreate_baseline=precreate)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, [])

    def test_resolution_failure_remains_a_scheduled_watch_failure(self):
        reporter = self.jobs["report-scheduled-result"]
        self.assertEqual(reporter["needs"], ["resolve-stable", "compare"])
        self.assertEqual(reporter["steps"][-1]["env"]["RESULT"],
                         "${{ needs.resolve-stable.result == 'success' && needs.compare.result || needs.resolve-stable.result }}")


if __name__ == "__main__":
    unittest.main()
