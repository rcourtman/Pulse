import json
import os
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from secure_runtime_attestation import (
    ARTIFACT_ARGUMENTS,
    AttestationError,
    DISPOSABLE_VM_GUARD_SHA256,
    EXPECTED_ARTIFACT_PACKAGES,
    REQUIRED_SCENARIOS,
    SCENARIO_REQUIRED_CLAIMS,
    SCENARIO_REQUIRED_OBSERVATIONS,
    SOURCE_MANIFEST_ID,
    SOURCE_MANIFEST_PATH,
    SOURCE_MANIFEST_SCHEMA_VERSION,
    create_attestation,
    sha256_bytes,
    verify_artifact_build_identity,
)
from repo_file_io import strip_local_git_env


class SecureRuntimeAttestationTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.git("init", "-b", "main")
        self.git("config", "user.name", "Pulse Test")
        self.git("config", "user.email", "pulse-test@example.invalid")
        manifest = {
            "schema_version": SOURCE_MANIFEST_SCHEMA_VERSION,
            "manifest_id": SOURCE_MANIFEST_ID,
            "target_os": "linux",
            "exact_paths": [
                "scripts/install.sh",
                "scripts/installtests/secure_runtime_systemd_lab_test.go",
                "scripts/release_control/secure_runtime_attestation.py",
                SOURCE_MANIFEST_PATH,
            ],
            "recursive_roots": ["boundary"],
            "include_suffixes": [".go"],
            "exclude_suffixes": ["_test.go"],
        }
        manifest_raw = (json.dumps(manifest, indent=2) + "\n").encode()
        source_values = {
            "scripts/install.sh": b"fixture installer\n",
            "scripts/installtests/secure_runtime_systemd_lab_test.go": b"fixture harness\n",
            "scripts/release_control/secure_runtime_attestation.py": Path(__file__).with_name("secure_runtime_attestation.py").read_bytes(),
            SOURCE_MANIFEST_PATH: manifest_raw,
            "boundary/runtime.go": b"package boundary\n",
        }
        self.source_hashes = {}
        for relative, value in sorted(source_values.items()):
            path = self.repo / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(value)
            self.source_hashes[relative] = sha256_bytes(value)
        self.git("add", ".")
        self.git("commit", "-m", "fixture")
        self.commit = self.git("rev-parse", "HEAD")
        self.git("branch", "origin/main")
        self.git("checkout", "--detach", self.commit)

        self.artifacts = {}
        artifact_hashes = {}
        for name in ARTIFACT_ARGUMENTS:
            path = self.root / name
            value = f"{name}\n".encode()
            path.write_bytes(value)
            self.artifacts[name] = path
            artifact_hashes[name] = sha256_bytes(value)
        self.receipt = self.root / "receipt.json"
        self.transcript = self.root / "transcript.jsonl"
        started_at = "2026-08-30T10:00:00+00:00"
        scenarios = []
        command_output = "fixture command output\n"
        transcript_events = [
            {
                "sequence": 1,
                "event_id": "event-0001",
                "observed_at": started_at,
                "kind": "command_output",
                "operation": "fixture command",
                "output": command_output,
                "output_sha256": sha256_bytes(command_output.encode()),
            }
        ]
        previous = started_at
        for index, name in enumerate(REQUIRED_SCENARIOS, 1):
            completed = f"2026-08-30T10:00:{index:02d}+00:00"
            event_sequence = index + 1
            event_id = f"event-{event_sequence:04d}"
            claims = sorted(SCENARIO_REQUIRED_CLAIMS[name])
            observations = dict(SCENARIO_REQUIRED_OBSERVATIONS[name])
            scenarios.append(
                {
                    "sequence": index,
                    "name": name,
                    "passed": True,
                    "started_at": previous,
                    "completed_at": completed,
                    "evidence": {
                        "kind": "runtime-observation-v1",
                        "summary": f"observed {name}",
                        "claims": claims,
                        "observations": observations,
                        "transcript_event_ids": [event_id],
                    },
                }
            )
            transcript_events.append(
                {
                    "sequence": event_sequence,
                    "event_id": event_id,
                    "observed_at": completed,
                    "kind": "scenario_result",
                    "scenario": name,
                    "claims": claims,
                    "observations": observations,
                    "summary": f"observed {name}",
                }
            )
            previous = completed
        transcript_raw = b"".join(
            (json.dumps(event, separators=(",", ":")) + "\n").encode()
            for event in transcript_events
        )
        self.transcript.write_bytes(transcript_raw)
        self.receipt.write_text(
            json.dumps(
                {
                    "schema_version": 4,
                    "record_path": "records/receipt.json",
                    "started_at": started_at,
                    "completed_at": "2026-08-30T10:00:13+00:00",
                    "source_manifest": {
                        "schema_version": SOURCE_MANIFEST_SCHEMA_VERSION,
                        "manifest_id": SOURCE_MANIFEST_ID,
                        "path": SOURCE_MANIFEST_PATH,
                        "sha256": sha256_bytes(manifest_raw),
                        "target_os": "linux",
                        "target_arch": "arm64",
                    },
                    "source_hashes": self.source_hashes,
                    "artifact_hashes": artifact_hashes,
                    "artifact_versions": {"collector_v1": "1.0.0", "collector_v2": "1.1.0"},
                    "disposable_vm_guard_sha256": DISPOSABLE_VM_GUARD_SHA256,
                    "os_release": 'PRETTY_NAME="Fixture Linux"',
                    "kernel": "Linux fixture",
                    "systemd_version": "systemd 255",
                    "architecture": "arm64",
                    "transcript": {
                        "format": "jsonl-v1",
                        "record_path": "records/transcript.jsonl",
                        "sha256": sha256_bytes(transcript_raw),
                        "event_count": len(transcript_events),
                    },
                    "collector_service_user": "pulse-agent",
                    "collector_process_uid": 1000,
                    "collector_authority": "monitoring-only",
                    "action_receipt_kind": "pulse.host_storage_cleanup_result",
                    "report_count": 3,
                    "first_report_at": "2026-08-30T10:00:01+00:00",
                    "last_report_at": "2026-08-30T10:00:12+00:00",
                    "scenarios": scenarios,
                },
                indent=2,
            )
            + "\n",
            encoding="utf-8",
        )
        self.build_identity = mock.patch(
            "secure_runtime_attestation.verify_artifact_build_identity",
            return_value={
                name: {"package": package, "vcs_revision": self.commit, "vcs_modified": "false"}
                for name, package in EXPECTED_ARTIFACT_PACKAGES.items()
            },
        )
        self.build_identity.start()

    def tearDown(self) -> None:
        self.build_identity.stop()
        self.temporary.cleanup()

    def git(self, *args: str) -> str:
        return subprocess.run(
            ["git", *args],
            cwd=self.repo,
            check=True,
            capture_output=True,
            text=True,
            env=strip_local_git_env(os.environ.copy()),
        ).stdout.strip()

    def attest(self, **overrides):
        arguments = {
            "checkout": self.repo,
            "commit": self.commit,
            "main_ref": "origin/main",
            "receipt_path": self.receipt,
            "receipt_record_path": "records/receipt.json",
            "transcript_path": self.transcript,
            "artifacts": self.artifacts,
            "elapsed_seconds": 113.43,
        }
        arguments.update(overrides)
        return create_attestation(**arguments)

    def test_accepts_exact_committed_main_evidence(self) -> None:
        result = self.attest()
        self.assertEqual(result["proof_classification"], "committed-main-artifact-bound-self-attested-systemd")
        self.assertEqual(result["qualified_commit"], self.commit)
        self.assertTrue(result["source_hashes_match_commit"])
        self.assertTrue(result["artifact_hashes_match_receipt"])
        self.assertEqual(result["scenario_count"], 12)
        self.assertTrue(result["receipt"]["path_bound_inside_receipt"])
        self.assertEqual(result["transcript"]["event_count"], 13)
        self.assertEqual(result["source_manifest"]["manifest_id"], SOURCE_MANIFEST_ID)
        self.assertEqual(len(result["attestation_tool_sha256"]), 64)

    def test_accepts_exact_release_candidate_ref(self) -> None:
        self.git("tag", "v9.0.0-rc.1", self.commit)
        result = self.attest(release_candidate_ref="v9.0.0-rc.1")
        self.assertEqual(result["proof_classification"], "release-candidate-artifact-bound-self-attested-systemd")
        self.assertNotIn("exact-release-candidate", result["residual_proof"])

    def test_rejects_attached_checkout(self) -> None:
        self.git("checkout", "main")
        with self.assertRaisesRegex(AttestationError, "must be detached"):
            self.attest()

    def test_rejects_tracked_checkout_change(self) -> None:
        (self.repo / "scripts/install.sh").write_text("changed\n", encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "tracked modifications"):
            self.attest()

    def test_rejects_untracked_source_outside_lab_artifacts(self) -> None:
        (self.repo / "unexpected.go").write_text("package unexpected\n", encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "unapproved untracked path"):
            self.attest()

    def test_rejects_source_digest_not_bound_to_commit(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["source_hashes"]["scripts/install.sh"] = "0" * 64
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "source digest mismatch"):
            self.attest()

    def test_rejects_source_set_that_omits_manifest_expansion(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        del receipt["source_hashes"]["boundary/runtime.go"]
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "exactly match the governed boundary manifest"):
            self.attest()

    def test_rejects_artifact_digest_mismatch(self) -> None:
        self.artifacts["runner"].write_bytes(b"tampered\n")
        with self.assertRaisesRegex(AttestationError, "artifact digest mismatch"):
            self.attest()

    def test_rejects_wrong_go_command_package(self) -> None:
        output = (
            "fixture: go1.25\n"
            "\tpath\tgithub.com/rcourtman/pulse-go-rewrite/cmd/not-pulse-agent\n"
            f"\tbuild\tvcs=git\n\tbuild\tvcs.revision={self.commit}\n\tbuild\tvcs.modified=false\n"
        )
        with mock.patch("secure_runtime_attestation.subprocess.run", return_value=subprocess.CompletedProcess([], 0, output, "")):
            with self.assertRaisesRegex(AttestationError, "expected Go command package"):
                verify_artifact_build_identity(self.receipt_dict(), self.artifacts, self.commit)

    def test_rejects_modified_go_build_identity(self) -> None:
        def inspect(command, **_kwargs):
            name = Path(command[-1]).name
            output = (
                "fixture: go1.25\n"
                f"\tpath\t{EXPECTED_ARTIFACT_PACKAGES[name]}\n"
                f"\tbuild\tvcs=git\n\tbuild\tvcs.revision={self.commit}\n\tbuild\tvcs.modified=true\n"
            )
            return subprocess.CompletedProcess(command, 0, output, "")

        with mock.patch("secure_runtime_attestation.subprocess.run", side_effect=inspect):
            with self.assertRaisesRegex(AttestationError, "modified checkout"):
                verify_artifact_build_identity(self.receipt_dict(), self.artifacts, self.commit)

    def test_rejects_failed_scenario(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["scenarios"][-1]["passed"] = False
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "did not pass"):
            self.attest()

    def test_rejects_tampered_transcript(self) -> None:
        self.transcript.write_bytes(self.transcript.read_bytes() + b"{}\n")
        with self.assertRaisesRegex(AttestationError, "transcript digest mismatch"):
            self.attest()

    def test_rejects_scenario_chronology_drift(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["scenarios"][2]["started_at"] = "2026-08-30T09:59:00+00:00"
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "chronology is invalid"):
            self.attest()

    def test_rejects_receipt_without_verified_mutation(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        scenario = next(item for item in receipt["scenarios"] if item["name"] == "typed_action_receipt")
        scenario["evidence"]["claims"].remove("typed_mutation_verified")
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "required causal claims"):
            self.attest()

    def test_rejects_scenario_without_typed_observation(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        scenario = next(item for item in receipt["scenarios"] if item["name"] == "typed_action_receipt")
        del scenario["evidence"]["observations"]["verification"]
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "required typed observations"):
            self.attest()

    def test_rejects_legacy_receipt_schema(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["schema_version"] = 3
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "schema_version 4"):
            self.attest()

    def test_rejects_missing_scenario(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["scenarios"].pop()
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "canonical 12-scenario"):
            self.attest()

    def test_rejects_mismatched_release_candidate_ref(self) -> None:
        (self.repo / "later").write_text("later\n", encoding="utf-8")
        self.git("add", "later")
        self.git("commit", "-m", "later")
        later = self.git("rev-parse", "HEAD")
        self.git("tag", "v9.0.0-rc.2", later)
        self.git("checkout", "--detach", self.commit)
        with self.assertRaisesRegex(AttestationError, "does not resolve"):
            self.attest(release_candidate_ref="v9.0.0-rc.2")

    def test_rejects_commit_not_reachable_from_main(self) -> None:
        self.git("checkout", "--orphan", "unreachable")
        self.git("rm", "-rf", ".")
        (self.repo / "orphan").write_text("orphan\n", encoding="utf-8")
        self.git("add", "orphan")
        self.git("commit", "-m", "orphan")
        orphan = self.git("rev-parse", "HEAD")
        self.git("checkout", "--detach", orphan)
        with self.assertRaisesRegex(AttestationError, "not reachable"):
            self.attest(commit=orphan)

    def test_rejects_sensitive_receipt_keys(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["secret"] = "not-allowed"
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "forbidden sensitive key"):
            self.attest()

    def test_rejects_noncanonical_receipt_record_path(self) -> None:
        with self.assertRaisesRegex(AttestationError, "canonical repository-relative"):
            self.attest(receipt_record_path="../receipt.json")

    def test_rejects_receipt_record_path_not_bound_inside_blob(self) -> None:
        with self.assertRaisesRegex(AttestationError, "not bound inside"):
            self.attest(receipt_record_path="records/other.json")

    def test_rejects_nonfinite_elapsed_time(self) -> None:
        with self.assertRaisesRegex(AttestationError, "must be positive"):
            self.attest(elapsed_seconds=float("nan"))

    def receipt_dict(self):
        return json.loads(self.receipt.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
