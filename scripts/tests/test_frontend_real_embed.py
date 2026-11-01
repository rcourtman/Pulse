#!/usr/bin/env python3
"""Check ordinary CI's real-embed phase without a host build or network.

The exact workflow shell runs with private asset fixtures and compiler/Git
doubles. This is recipe/admission proof, not real frontend/backend acceptance.
"""

import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import tarfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = Path(os.environ.get("PULSE_BUILD_WORKFLOW_UNDER_TEST", ROOT / ".github/workflows/build-and-test.yml")).resolve()
GO_SETUP = "Set up Go for real-embed verification"
VERIFY = "Verify backend with real frontend embed"


def workflow_steps(source, job_name):
    job = yaml.safe_load(source)["jobs"][job_name]
    steps = {step["name"]: step for step in job["steps"]}
    if len(steps) != len(job["steps"]):
        raise AssertionError("duplicate named frontend step")
    return job, steps


def recipe(source):
    build, producer = workflow_steps(source, "frontend-build")
    job, steps = workflow_steps(source, "frontend-checks")
    order = list(steps)
    required = ["Checkout repository", "Download this run's validated frontend embed",
                "Verify and restore the validated frontend embed", GO_SETUP, VERIFY]
    if any(name not in order for name in required) or [order.index(name) for name in required] != sorted(order.index(name) for name in required):
        raise AssertionError("same-run build transfer or real-embed ordering missing")
    if job["needs"] != ["changes", "frontend-build"] or job["timeout-minutes"] != 30:
        raise AssertionError("ordinary runtime admission/deadline changed")
    if producer["Build frontend bundle (with embed copy)"]["run"] != "make frontend":
        raise AssertionError("must use the producer's production frontend build")
    for checkout in (producer["Checkout repository"], steps["Checkout repository"]):
        if checkout["with"] != {"persist-credentials": False}:
            raise AssertionError("frontend proof must use the ordinary exact checkout")
    setup = steps[GO_SETUP]
    if setup["with"].get("go-version-file") != "go.mod" or setup["uses"] != "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e":
        raise AssertionError("declared compiler setup missing")
    for name in (GO_SETUP, VERIFY):
        if any(field in steps[name] for field in ("if", "continue-on-error", "working-directory")):
            raise AssertionError("real-embed verification must run at root and retain failure")
    shell = steps[VERIFY]["run"]
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


class FrontendEmbedTransfer(unittest.TestCase):
    SHA = "0123456789012345678901234567890123456789"

    def setUp(self):
        self.source = WORKFLOW.read_text()
        _, self.producer = workflow_steps(self.source, "frontend-build")
        _, self.consumer = workflow_steps(self.source, "frontend-checks")

    def test_transfer_is_fixed_to_this_run_source_and_attempt(self):
        upload = self.producer["Transfer the validated frontend embed"]
        download = self.consumer["Download this run's validated frontend embed"]
        self.assertEqual(upload["uses"], "actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9")
        self.assertEqual(upload["with"], {
            "name": "ci-frontend-embed-${{ github.sha }}-${{ github.run_attempt }}",
            "path": "${{ runner.temp }}/frontend-embed.tar", "if-no-files-found": "error",
            "retention-days": 1, "compression-level": 0,
        })
        self.assertEqual(download["uses"], "actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333")
        self.assertEqual(download["with"], {
            "name": upload["with"]["name"], "path": "${{ runner.temp }}/frontend-embed",
        })
        self.assertEqual(self.producer["Bind the validated frontend embed"]["env"], {"EXPECTED_SOURCE_SHA": "${{ github.sha }}"})
        self.assertEqual(self.consumer["Verify and restore the validated frontend embed"]["env"], {
            "EXPECTED_EMBED_SHA256": "${{ needs.frontend-build.outputs.embed_sha256 }}",
            "EXPECTED_SOURCE_SHA": "${{ github.sha }}",
        })

    def transfer(self, mutation=""):
        with tempfile.TemporaryDirectory(prefix="pulse-embed-transfer-") as directory:
            base = Path(directory)
            producer, consumer = base / "producer", base / "consumer"
            producer.mkdir(); consumer.mkdir()
            tools = base / "tools"; tools.mkdir()
            git = tools / "git"
            git.write_text('#!/bin/sh\n[ "$*" = "rev-parse HEAD" ] || exit 97\necho ' + self.SHA + '\n')
            git.chmod(0o700)
            for tree in ("frontend-modern/dist", "internal/api/frontend-modern/dist"):
                (producer / tree / "assets").mkdir(parents=True)
                (producer / tree / "index.html").write_bytes(b"<html>real build fixture</html>\n")
                (producer / tree / "assets/app.js").write_bytes(b"fixture application bytes\n")
            runner = base / "runner"; runner.mkdir()
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       RUNNER_TEMP=str(runner), EXPECTED_SOURCE_SHA=self.SHA,
                       GITHUB_OUTPUT=str(base / "outputs"))
            if mutation == "producer source": env["EXPECTED_SOURCE_SHA"] = "a" * 40
            if mutation == "producer mismatch":
                (producer / "internal/api/frontend-modern/dist/assets/app.js").write_bytes(b"changed\n")
            bound = subprocess.run(["bash", "-c", self.producer["Bind the validated frontend embed"]["run"]],
                                   cwd=producer, env=env, capture_output=True, text=True, timeout=10)
            packet = runner / "frontend-embed.tar"
            if mutation.startswith("producer "):
                self.assertNotEqual(bound.returncode, 0)
                self.assertFalse(packet.exists())
                self.assertFalse((base / "outputs").exists())
                return
            self.assertEqual(bound.returncode, 0, bound.stdout + bound.stderr)
            digest = (base / "outputs").read_text().strip().removeprefix("sha256=")
            self.assertEqual(digest, hashlib.sha256(packet.read_bytes()).hexdigest())
            if mutation.startswith("member:"):
                kind = mutation.removeprefix("member:")
                with tarfile.open(packet, "a") as archive:
                    member = tarfile.TarInfo({"parent": "frontend-modern/dist/../../../outside",
                                               "foreign": "Makefile", "absolute": "/tmp/outside"}.get(kind, "frontend-modern/dist/assets/link"))
                    if kind == "symlink": member.type = tarfile.SYMTYPE; member.linkname = "/tmp/outside"
                    if kind == "hardlink": member.type = tarfile.LNKTYPE; member.linkname = "frontend-modern/dist/index.html"
                    if kind == "fifo": member.type = tarfile.FIFOTYPE
                    archive.addfile(member, io.BytesIO())
                digest = hashlib.sha256(packet.read_bytes()).hexdigest()
            if mutation == "empty archive":
                with tarfile.open(packet, "w"): pass
                digest = hashlib.sha256(packet.read_bytes()).hexdigest()
            if mutation == "corrupt bytes":
                packet.write_bytes(packet.read_bytes() + b"unexpected bytes")
            if mutation == "wrong digest": digest = "f" * 64
            if mutation == "missing digest": digest = ""
            if mutation == "malformed digest": digest = "not-a-digest"
            target = runner / "frontend-embed"; target.mkdir()
            packet.rename(target / packet.name)
            if mutation == "missing packet": (target / packet.name).unlink()
            env["EXPECTED_EMBED_SHA256"] = digest
            if mutation == "consumer source": env["EXPECTED_SOURCE_SHA"] = "b" * 40
            restored = subprocess.run(["bash", "-c", self.consumer["Verify and restore the validated frontend embed"]["run"]],
                                      cwd=consumer, env=env, capture_output=True, text=True, timeout=10)
            if mutation:
                self.assertNotEqual(restored.returncode, 0, mutation)
                self.assertFalse((consumer / "frontend-modern/dist/index.html").exists())
            else:
                self.assertEqual(restored.returncode, 0, restored.stdout + restored.stderr)
                expected = {p.relative_to(producer).as_posix(): p.read_bytes() for p in producer.rglob("*") if p.is_file()}
                actual = {p.relative_to(consumer).as_posix(): p.read_bytes() for p in consumer.rglob("*") if p.is_file()}
                self.assertEqual(actual, expected)

    def test_actual_producer_and_consumer_preserve_every_built_byte(self):
        self.transfer()

    def test_source_or_content_uncertainty_never_restores_assets(self):
        for mutation in ("producer source", "producer mismatch", "consumer source", "corrupt bytes",
                         "wrong digest", "missing digest", "malformed digest", "missing packet", "empty archive"):
            with self.subTest(mutation=mutation): self.transfer(mutation)

    def test_archive_members_are_validated_before_any_extraction(self):
        for kind in ("parent", "foreign", "absolute", "symlink", "hardlink", "fifo"):
            with self.subTest(kind=kind): self.transfer("member:" + kind)


if __name__ == "__main__":
    unittest.main()
