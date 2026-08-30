import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from secure_runtime_attestation import (
    ARTIFACT_ARGUMENTS,
    AttestationError,
    REQUIRED_SCENARIOS,
    create_attestation,
    sha256_bytes,
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
        self.source_hashes = {}
        for relative, value in {
            "scripts/install.sh": b"installer\n",
            "scripts/installtests/secure_runtime_systemd_lab_test.go": b"lab\n",
        }.items():
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
        self.receipt.write_text(
            json.dumps(
                {
                    "schema_version": 2,
                    "source_hashes": self.source_hashes,
                    "artifact_hashes": artifact_hashes,
                    "os_release": 'PRETTY_NAME="Fixture Linux"',
                    "kernel": "Linux fixture",
                    "systemd_version": "systemd 255",
                    "architecture": "arm64",
                    "scenarios": [
                        {"name": name, "passed": True, "detail": "passed"}
                        for name in REQUIRED_SCENARIOS
                    ],
                },
                indent=2,
            )
            + "\n",
            encoding="utf-8",
        )

    def tearDown(self) -> None:
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
            "artifacts": self.artifacts,
            "elapsed_seconds": 113.43,
        }
        arguments.update(overrides)
        return create_attestation(**arguments)

    def test_accepts_exact_committed_main_evidence(self) -> None:
        result = self.attest()
        self.assertEqual(result["proof_classification"], "exact-committed-main")
        self.assertEqual(result["qualified_commit"], self.commit)
        self.assertTrue(result["source_hashes_match_commit"])
        self.assertTrue(result["artifact_hashes_match_receipt"])
        self.assertEqual(result["scenario_count"], 12)
        self.assertEqual(len(result["attestation_tool_sha256"]), 64)

    def test_accepts_exact_release_candidate_ref(self) -> None:
        self.git("tag", "v9.0.0-rc.1", self.commit)
        result = self.attest(release_candidate_ref="v9.0.0-rc.1")
        self.assertEqual(result["proof_classification"], "exact-release-candidate")
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

    def test_rejects_artifact_digest_mismatch(self) -> None:
        self.artifacts["runner"].write_bytes(b"tampered\n")
        with self.assertRaisesRegex(AttestationError, "artifact digest mismatch"):
            self.attest()

    def test_rejects_failed_scenario(self) -> None:
        receipt = json.loads(self.receipt.read_text(encoding="utf-8"))
        receipt["scenarios"][-1]["passed"] = False
        self.receipt.write_text(json.dumps(receipt), encoding="utf-8")
        with self.assertRaisesRegex(AttestationError, "did not pass"):
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

    def test_rejects_nonfinite_elapsed_time(self) -> None:
        with self.assertRaisesRegex(AttestationError, "must be positive"):
            self.attest(elapsed_seconds=float("nan"))


if __name__ == "__main__":
    unittest.main()
