#!/usr/bin/env python3
"""Exercise same-run E2E image production/admission without a registry or daemon."""

import copy
import gzip
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("e2e_test_images", ROOT / "scripts/e2e_test_images.py")
IMAGES = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(IMAGES)
WORKFLOW = ROOT / ".github/workflows/test-e2e.yml"
SOURCE = "a" * 40
IDENTITY = {"source_sha": SOURCE, "run_id": "1234567", "run_attempt": "2"}


def job_block(workflow, job):
    return re.split(r"(?m)^  \S", workflow.split(f"\n  {job}:\n", 1)[1], maxsplit=1)[0]


class DockerDouble:
    """Record real helper invocations; supply local deterministic saved bytes."""

    def __init__(self):
        self.calls = []
        self.fail = None
        self.records = {
            name: {
                "Id": "sha256:" + str(index + 1) * 64,
                "Os": "linux", "Architecture": "amd64",
                "Config": {"Labels": {IMAGES.REVISION_LABEL: SOURCE}},
            }
            for index, name in enumerate(IMAGES.IMAGES)
        }
        # Upstream seed bytes have no candidate-specific revision label.
        self.records["pulse-e2e-seed:test"]["Config"]["Labels"] = None

    def __call__(self, *args, capture=False):
        self.calls.append(args)
        if self.fail and args[:len(self.fail)] == self.fail:
            raise subprocess.CalledProcessError(1, args)
        if args == ("git", "rev-parse", "HEAD"):
            return SOURCE + "\n"
        if args[:3] == ("docker", "image", "inspect"):
            return json.dumps(self.records[args[-1]])
        if args[:3] == ("docker", "image", "save"):
            # Real tar/gzip/file/hash operations run; only the daemon is doubled.
            with tarfile.open(args[4], "w") as archive:
                for image in args[5:]:
                    payload = self.records[image]["Id"].encode()
                    item = tarfile.TarInfo(image.replace(":", "-") + ".json")
                    item.size = len(payload)
                    archive.addfile(item, io.BytesIO(payload))
        return None


class E2EImageBundleTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name) / "bundle"
        self.output = Path(self.temp.name) / "outputs"
        self.docker = DockerDouble()
        self.env = {
            "GITHUB_SHA": SOURCE, "GITHUB_RUN_ID": IDENTITY["run_id"],
            "GITHUB_RUN_ATTEMPT": IDENTITY["run_attempt"], "GITHUB_OUTPUT": str(self.output),
        }
        self.addCleanup(patch.stopall)
        patch.dict(os.environ, self.env).start()
        patch.object(IMAGES, "run", self.docker).start()

    def produce(self):
        IMAGES.build(self.directory, IDENTITY)
        value = self.output.read_text().strip().split("=", 1)[1]
        os.environ["PULSE_E2E_MANIFEST_SHA256"] = value
        self.docker.calls.clear()
        return json.loads((self.directory / IMAGES.MANIFEST).read_text())

    def replace_manifest(self, manifest, rebind=True):
        path = self.directory / IMAGES.MANIFEST
        path.write_text(json.dumps(manifest))
        if rebind:
            os.environ["PULSE_E2E_MANIFEST_SHA256"] = IMAGES.digest(path)

    def assert_refused_before_docker(self):
        self.docker.calls.clear()
        with self.assertRaises((ValueError, OSError)):
            IMAGES.load(self.directory, IDENTITY)
        self.assertEqual(self.docker.calls, [])

    def test_real_tar_gzip_round_trip_and_exact_commands(self):
        IMAGES.build(self.directory, IDENTITY)
        builds = [args for args in self.docker.calls if args[:2] == ("docker", "build")]
        self.assertEqual(len(builds), 2)
        self.assertEqual(builds[0], (
            "docker", "build", "--platform=linux/amd64", "--label",
            f"{IMAGES.REVISION_LABEL}={SOURCE}", "-t", "pulse:test", "--target",
            "e2e_runtime", "--build-arg", "GO_BUILD_TAGS=", ".",
        ))
        self.assertEqual(builds[1][-3:], ("-t", "pulse-mock-github:test", "./tests/integration/mock-github-server"))
        pulls = [args for args in self.docker.calls if args[:3] == ("docker", "image", "pull")]
        self.assertEqual(pulls, [("docker", "image", "pull", "--platform=linux/amd64", IMAGES.seed_image())])
        saved = next(args for args in self.docker.calls if args[:3] == ("docker", "image", "save"))
        self.assertEqual(saved[5:], IMAGES.IMAGES)
        self.assertFalse((self.directory / "images.tar").exists())
        with gzip.open(self.directory / IMAGES.ARCHIVE, "rb") as stream:
            with tarfile.open(fileobj=stream, mode="r|") as archive:
                self.assertEqual([entry.name for entry in archive], [name.replace(":", "-") + ".json" for name in IMAGES.IMAGES])
        os.environ["PULSE_E2E_MANIFEST_SHA256"] = self.output.read_text().strip().split("=", 1)[1]
        self.docker.calls.clear()
        IMAGES.load(self.directory, IDENTITY)
        self.assertEqual(self.docker.calls[0], ("docker", "image", "load", "--input", str(self.directory / IMAGES.ARCHIVE)))
        self.assertEqual(len(self.docker.calls), 4)  # one load, three identity reads
        self.assertFalse(any("build" in args or "pull" in args for args in self.docker.calls))

    def test_binding_rejects_another_checkout_and_malformed_run(self):
        self.assertEqual(IMAGES.binding(), IDENTITY)
        for key, value in (("GITHUB_SHA", "b" * 40), ("GITHUB_SHA", "main"),
                           ("GITHUB_RUN_ID", "0"), ("GITHUB_RUN_ATTEMPT", "2\n3")):
            with self.subTest(key=key, value=value), patch.dict(os.environ, {key: value}):
                with self.assertRaises(ValueError):
                    IMAGES.binding()

    def test_build_load_cli_uses_authenticated_source_binding(self):
        with patch.object(sys, "argv", ["images", "build", str(self.directory)]):
            self.assertEqual(IMAGES.main(), 0)
        os.environ["PULSE_E2E_MANIFEST_SHA256"] = self.output.read_text().strip().split("=", 1)[1]
        with patch.object(sys, "argv", ["images", "load", str(self.directory)]):
            self.assertEqual(IMAGES.main(), 0)

    def test_provider_build_pull_save_refusals_are_not_replayed(self):
        for command in (("docker", "build"), ("docker", "image", "pull"), ("docker", "image", "save")):
            with self.subTest(command=command):
                self.directory = Path(self.temp.name) / ("-".join(command))
                self.docker.calls.clear()
                self.docker.fail = command
                with self.assertRaises(subprocess.CalledProcessError):
                    IMAGES.build(self.directory, IDENTITY)
                self.assertEqual(sum(args[:len(command)] == command for args in self.docker.calls), 1)
                self.assertFalse((self.directory / IMAGES.MANIFEST).exists())
                self.assertFalse(self.output.exists())

    def test_build_refuses_existing_destination(self):
        self.directory.mkdir()
        with self.assertRaises(FileExistsError):
            IMAGES.build(self.directory, IDENTITY)
        self.assertEqual(self.docker.calls, [])

    def test_producer_rejects_wrong_revision_platform_and_id(self):
        original = copy.deepcopy(self.docker.records)
        for field, value in (("Architecture", "arm64"), ("Os", "windows"), ("Id", "latest"),
                             ("Config", {"Labels": {IMAGES.REVISION_LABEL: "b" * 40}})):
            with self.subTest(field=field):
                self.docker.records = copy.deepcopy(original)
                self.docker.records["pulse:test"][field] = value
                with self.assertRaises(ValueError):
                    IMAGES.inspect_images(SOURCE)

    def test_manifest_job_output_is_required_not_a_sidecar_hint(self):
        self.produce()
        for value in ("", "a" * 64, "not-a-hash"):
            with self.subTest(value=value), patch.dict(os.environ, {"PULSE_E2E_MANIFEST_SHA256": value}):
                self.assert_refused_before_docker()

    def test_changed_manifest_refused_even_with_unchanged_archive(self):
        manifest = self.produce()
        manifest["images"]["pulse:test"] = "sha256:" + "4" * 64
        self.replace_manifest(manifest, rebind=False)
        self.assert_refused_before_docker()

    def test_other_source_run_attempt_schema_pin_or_images_refused(self):
        original = self.produce()
        for field, value in (("source_sha", "b" * 40), ("run_id", "7654321"), ("run_attempt", "1"),
                             ("schema_version", 2), ("schema_version", True), ("seed_image", "alpine:latest"),
                             ("images", {}), ("images", {**original["images"], "other:test": "sha256:" + "f" * 64}),
                             ("images", {**original["images"], "pulse:test": "latest"}),
                             ("archive_bytes", True), ("archive_bytes", 0), ("archive_bytes", "10"),
                             ("archive_sha256", "not-a-hash")):
            with self.subTest(field=field, value=value):
                manifest = {**original, field: value}
                self.replace_manifest(manifest)
                self.assert_refused_before_docker()
        self.replace_manifest({**original, "load_command": "anything"})
        self.assert_refused_before_docker()

    def test_tampered_truncated_and_oversized_archives_never_load(self):
        self.produce()
        archive = self.directory / IMAGES.ARCHIVE
        original = archive.read_bytes()
        for payload in (original[:-1], original + b"extra", b"x" * len(original)):
            with self.subTest(bytes=len(payload)):
                archive.write_bytes(payload)
                self.assert_refused_before_docker()
        archive.write_bytes(original)
        with patch.object(IMAGES, "MAX_ARCHIVE_BYTES", len(original) - 1):
            self.assert_refused_before_docker()

    def test_missing_symlink_and_oversized_manifest_never_load(self):
        self.produce()
        for name in (IMAGES.MANIFEST, IMAGES.ARCHIVE):
            with self.subTest(file=name):
                path = self.directory / name
                old = path.with_suffix(".owned")
                path.rename(old)
                self.assert_refused_before_docker()
                path.symlink_to(old)
                self.assert_refused_before_docker()
                path.unlink()
                old.rename(path)
        path = self.directory / IMAGES.MANIFEST
        path.write_bytes(b" " * 16385)
        os.environ["PULSE_E2E_MANIFEST_SHA256"] = IMAGES.digest(path)
        self.assert_refused_before_docker()

    def test_load_failure_is_terminal_without_pull_or_rebuild(self):
        self.produce()
        self.docker.fail = ("docker", "image", "load")
        with self.assertRaises(subprocess.CalledProcessError):
            IMAGES.load(self.directory, IDENTITY)
        self.assertEqual(self.docker.calls, [("docker", "image", "load", "--input", str(self.directory / IMAGES.ARCHIVE))])

    def test_loaded_image_must_match_source_platform_and_producer_id(self):
        self.produce()
        original = copy.deepcopy(self.docker.records)
        for field, value in (("Architecture", "arm64"), ("Os", "windows"), ("Id", "sha256:" + "e" * 64),
                             ("Config", {"Labels": {IMAGES.REVISION_LABEL: "b" * 40}})):
            with self.subTest(field=field):
                self.docker.records = copy.deepcopy(original)
                self.docker.records["pulse:test"][field] = value
                self.docker.calls.clear()
                with self.assertRaises(ValueError):
                    IMAGES.load(self.directory, IDENTITY)
                self.assertEqual(self.docker.calls[0][:3], ("docker", "image", "load"))
                self.assertFalse(any("build" in args or "pull" in args for args in self.docker.calls))

    def test_seed_pin_and_local_default_are_preserved(self):
        source = IMAGES.COMPOSE.read_text()
        pin = IMAGES.seed_image()
        self.assertIn("${PULSE_E2E_SEED_IMAGE:-" + pin + "}", source)
        self.assertNotIn("pull_policy: always", source)
        with patch.dict(os.environ, {"PULSE_E2E_SEED_IMAGE": "untrusted.example/image:latest"}):
            self.assertEqual(IMAGES.seed_image(), pin)


class E2EImageWorkflowTest(unittest.TestCase):
    def assert_fixture_admission(self, workflow):
        consumer = job_block(workflow, "e2e")
        image_admission = consumer.index("- name: Admit the source-bound test images")
        startup = consumer.index("- name: Start test containers")
        fixture_admission = consumer.index("- name: Admit the default mock runtime")
        stable = consumer.index("- name: Run stable-tier E2E suite")
        probation = consumer.index("- name: Run probation-tier E2E suite")
        self.assertLess(image_admission, startup)
        self.assertLess(startup, fixture_admission)
        self.assertLess(fixture_admission, stable)
        end = consumer.index("\n      - name:", fixture_admission)
        admission = consumer[fixture_admission:end]
        self.assertIn("npx playwright test --config=fixture-readiness.config.ts", admission)
        self.assertIn('PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY: "true"', admission)
        for ignored in ("continue-on-error", "if:", "||"):
            self.assertNotIn(ignored, admission)
        for receipt in ("Upload Playwright report", "Upload JUnit results"):
            start = consumer.index(f"- name: {receipt}\n")
            self.assertLess(stable, start)
            self.assertLess(start, probation)
            end = consumer.index("\n      - name:", start)
            self.assertIn("!cancelled() && steps.stable.outcome == 'failure'", consumer[start:end])

    def test_fixture_admission_cannot_skip_source_images_or_lose_completed_receipts(self):
        self.assert_fixture_admission(WORKFLOW.read_text())

    def test_ignored_fixture_and_late_receipt_mutants_are_rejected(self):
        source = WORKFLOW.read_text()
        for old, new in (
            ("run: npx playwright test --config=fixture-readiness.config.ts",
             "run: npx playwright test --config=fixture-readiness.config.ts || true"),
            ("        id: fixture\n", "        id: fixture\n        continue-on-error: true\n"),
            ('PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY: "true"', 'PULSE_E2E_REQUIRE_DEFAULT_MOCK_READY: "false"'),
            ("!cancelled() && steps.stable.outcome == 'failure'", "failure()"),
        ):
            with self.subTest(mutation=new):
                with self.assertRaises(AssertionError):
                    self.assert_fixture_admission(source.replace(old, new))

    def assert_workflow(self, workflow):
        producer = job_block(workflow, "test-images")
        self.assertIn("needs: tier-selection", producer)
        self.assertIn("timeout-minutes: 30", producer)
        self.assertIn('python3 scripts/e2e_test_images.py build "$RUNNER_TEMP/pulse-e2e-images"', producer)
        self.assertIn("artifact-id: ${{ steps.images.outputs.artifact-id }}", producer)
        self.assertIn("manifest-sha256: ${{ steps.bundle.outputs.manifest-sha256 }}", producer)
        self.assertIn("if-no-files-found: error", producer)
        self.assertIn("retention-days: 1", producer)
        self.assertNotIn("continue-on-error", producer)
        self.assertNotIn("registry", producer.split("steps:", 1)[0])
        for job in ("e2e", "agent-registration"):
            consumer = job_block(workflow, job)
            self.assertIn("needs: [tier-selection, test-images]", consumer)
            self.assertIn("PULSE_E2E_SEED_IMAGE: pulse-e2e-seed:test", consumer)
            self.assertIn("artifact-ids: ${{ needs.test-images.outputs.artifact-id }}", consumer)
            self.assertIn("PULSE_E2E_MANIFEST_SHA256: ${{ needs.test-images.outputs.manifest-sha256 }}", consumer)
            self.assertIn('python3 scripts/e2e_test_images.py load "$RUNNER_TEMP/pulse-e2e-images"', consumer)
            self.assertNotIn("docker build", consumer)
            self.assertNotIn("docker pull", consumer)
            self.assertNotIn("github-token:", consumer)
            self.assertNotIn("repository:", consumer)
            self.assertNotIn("run-id:", consumer)
            start = consumer.index("- name: Admit the source-bound test images")
            end = consumer.index("\n      - name:", start + 1)
            admission = consumer[start:end]
            for ignored in ("continue-on-error", "if:", "||"):
                self.assertNotIn(ignored, admission)
            self.assertLess(start, consumer.index("- name: Start "))
        self.assertIn("timeout-minutes: 60", job_block(workflow, "e2e"))
        self.assertIn("shard: [1, 2, 3, 4, 5, 6, 7, 8]", job_block(workflow, "e2e"))
        self.assertIn("timeout-minutes: 30", job_block(workflow, "agent-registration"))
        verdict = job_block(workflow, "e2e-verdict")
        self.assertIn("      - test-images\n", verdict)
        self.assertIn('if [ "${{ needs.test-images.result }}" != "success" ]; then', verdict)
        for job in ("tier-selection", "e2e", "agent-registration"):
            self.assertIn(f'if [ "${{{{ needs.{job}.result }}}}" != "success" ]; then', verdict)
        self.assertIn("if: always()", verdict)

    def test_workflow_builds_once_and_gates_every_consumer_and_verdict(self):
        self.assert_workflow(WORKFLOW.read_text())

    def test_proof_and_helper_trigger_and_precede_fanout(self):
        source = WORKFLOW.read_text()
        for event in ("pull_request", "push"):
            block = re.split(r"(?m)^  \S", source.split(f"  {event}:\n", 1)[1], maxsplit=1)[0]
            for file in ("scripts/e2e_test_images.py", "scripts/tests/test_e2e_test_images.py"):
                self.assertIn(f"'{file}'", block)
        selection = job_block(source, "tier-selection")
        self.assertLess(selection.index("python3 scripts/tests/test_e2e_test_images.py"), selection.index("npm ci"))

    def test_rebuild_cross_run_and_ignored_admission_mutants_rejected(self):
        source = WORKFLOW.read_text()
        for old, new in (
            ("needs: [tier-selection, test-images]", "needs: tier-selection"),
            ("artifact-ids: ${{ needs.test-images.outputs.artifact-id }}", "name: latest-test-images"),
            ('python3 scripts/e2e_test_images.py load "$RUNNER_TEMP/pulse-e2e-images"', 'docker build -t pulse:test .'),
            ('python3 scripts/e2e_test_images.py load "$RUNNER_TEMP/pulse-e2e-images"', 'python3 scripts/e2e_test_images.py load "$RUNNER_TEMP/pulse-e2e-images" || true'),
            ("PULSE_E2E_MANIFEST_SHA256: ${{ needs.test-images.outputs.manifest-sha256 }}", "PULSE_E2E_MANIFEST_SHA256: unchecked"),
            ("      - test-images\n", ""),
        ):
            with self.subTest(mutation=new):
                with self.assertRaises(AssertionError):
                    self.assert_workflow(source.replace(old, new))


if __name__ == "__main__":
    unittest.main()
