#!/usr/bin/env python3

from __future__ import annotations

import base64
import contextlib
import hashlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))

import secure_runtime_attestation as v5
from secure_runtime_attestation_v6 import (
    ATTESTATION_SCHEMA_VERSION,
    ASSEMBLY_PROVENANCE_NAME,
    ASSEMBLY_SIGNER_WORKFLOW,
    BUILD_CONTRACT_NAME,
    CANONICAL_MAIN_REF,
    CANONICAL_ORIGIN_URL,
    CANONICAL_REPOSITORY,
    CHECKSUMS_NAME,
    COMPILER_PROVENANCE_NAME,
    COMPILER_SIGNER_WORKFLOW,
    RECEIPT_SCHEMA_VERSION,
    REQUIRED_SCENARIOS,
    SCENARIO_REQUIRED_CLAIMS,
    SCENARIO_REQUIRED_OBSERVATIONS,
    SOURCE_MANIFEST_ID,
    SOURCE_MANIFEST_PATH,
    copy_release_sidecar,
    create_attestation,
    immutable_artifact_snapshot,
    parse_args,
    verify_release_build_contract,
    verify_canonical_main_identity,
    verify_release_candidate_packet,
    verify_release_candidate_tag_identity,
)


class SecureRuntimeAttestationV6Test(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.tag = "v6.5.0-rc.1"
        self.commit = "b" * 40
        self.tag_object = "a" * 40
        self.update_public_keys = base64.b64encode(bytes(range(32))).decode("ascii")
        self.fingerprint = "SHA256:" + base64.b64encode(
            hashlib.sha256(bytes(range(32))).digest()
        ).decode("ascii")
        self.receipt = {
            "architecture": "arm64",
            "artifact_versions": {
                "collector_v1": "v6.5.0-lab.1",
                "collector_v2": "v6.5.0-lab.2",
                "collector_v3": "v6.5.0-lab.3",
                "collector_v4": "v6.5.0-lab.4",
            },
        }
        self.artifact_hashes = {
            name: hashlib.sha256(name.encode()).hexdigest()
            for name in v5.ARTIFACT_ARGUMENTS
        }
        self.artifacts = {}
        for name in v5.ARTIFACT_ARGUMENTS:
            path = self.root / f"artifact-{name}"
            path.write_bytes(name.encode())
            self.artifacts[name] = path
        contract = self.build_contract()
        self.collector_signatures = {}
        for name in ("collector_v1", "collector_v2", "collector_v3", "collector_v4"):
            path = self.root / f"{contract['artifacts'][name]['release_asset']}.sig"
            path.write_text(base64.b64encode(name.encode()).decode() + "\n", encoding="utf-8")
            self.collector_signatures[name] = path

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def build_contract(self) -> dict:
        artifacts = {}
        for name, package in v5.EXPECTED_ARTIFACT_PACKAGES.items():
            version = self.receipt["artifact_versions"].get(name, self.tag[1:])
            ldflags = ""
            if name != "runner":
                embedded_version = version if version.startswith("v") else f"v{version}"
                ldflags = (
                    f"-s -w -X main.Version={embedded_version} "
                    "-X github.com/rcourtman/pulse-go-rewrite/internal/updatesignature."
                    f"EmbeddedTrustedPublicKeys={self.update_public_keys}"
                )
            artifacts[name] = {
                "release_asset": f"pulse-secure-runtime-{name}-linux-arm64",
                "sha256": self.artifact_hashes[name],
                "build": {
                    "tool": "go build",
                    "package": package,
                    "target_os": "linux",
                    "target_arch": "arm64",
                    "cgo_enabled": 0,
                    "trimpath": True,
                    "buildvcs": False,
                    "build_args": ["-buildvcs=false", "-trimpath"],
                    "go_version": "go1.25.1",
                    "ldflags": ldflags,
                    "ldflags_sha256": hashlib.sha256(ldflags.encode()).hexdigest(),
                    "version": version,
                    "update_key_fingerprint": self.fingerprint,
                },
            }
        return {
            "schema_version": 1,
            "repository": CANONICAL_REPOSITORY,
            "assembly_signer_workflow": ASSEMBLY_SIGNER_WORKFLOW,
            "compiler_signer_workflow": COMPILER_SIGNER_WORKFLOW,
            "compiler_runner_trust": "github-hosted-deny-self-hosted",
            "tag": self.tag,
            "version": self.tag[1:],
            "source_sha": self.commit,
            "update_key_fingerprint": self.fingerprint,
            "update_public_keys": self.update_public_keys,
            "artifacts": artifacts,
        }

    def write_contract_and_checksums(self, contract: dict | None = None) -> tuple[Path, Path, dict[str, str]]:
        contract_path = self.root / BUILD_CONTRACT_NAME
        contract_path.write_text(json.dumps(contract or self.build_contract(), sort_keys=True), encoding="utf-8")
        checksums = {
            entry["release_asset"]: entry["sha256"]
            for entry in (contract or self.build_contract())["artifacts"].values()
        }
        checksums[BUILD_CONTRACT_NAME] = hashlib.sha256(contract_path.read_bytes()).hexdigest()
        checksums_path = self.root / CHECKSUMS_NAME
        checksums_path.write_text(
            "".join(f"{digest}  {name}\n" for name, digest in sorted(checksums.items())),
            encoding="utf-8",
        )
        return contract_path, checksums_path, checksums

    def git_result(self, *arguments: str) -> subprocess.CompletedProcess[bytes]:
        command = tuple(arguments)
        ref = f"refs/tags/{self.tag}"
        if command == ("remote", "get-url", "origin"):
            output = CANONICAL_ORIGIN_URL + "\n"
        elif command == ("rev-parse", "--verify", ref):
            output = self.tag_object + "\n"
        elif command == ("cat-file", "-t", ref):
            output = "tag\n"
        elif command == ("cat-file", "tag", ref):
            output = (
                f"object {self.commit}\ntype commit\ntag {self.tag}\n"
                "tagger github-actions[bot] <github-actions[bot]@users.noreply.github.com> 1 +0000\n"
                f"\nRelease {self.tag}\n"
            )
        elif command == ("rev-parse", "--verify", f"{ref}^{{commit}}"):
            output = self.commit + "\n"
        elif command == ("ls-remote", "--tags", "origin", ref, f"{ref}^{{}}"):
            output = f"{self.tag_object}\t{ref}\n{self.commit}\t{ref}^{{}}\n"
        else:
            raise AssertionError(f"unexpected git call {command}")
        return subprocess.CompletedProcess(command, 0, output.encode(), b"")

    def test_v6_is_a_new_contract_without_redefining_v5(self) -> None:
        self.assertEqual(RECEIPT_SCHEMA_VERSION, 6)
        self.assertEqual(ATTESTATION_SCHEMA_VERSION, 6)
        self.assertEqual(SOURCE_MANIFEST_ID, "secure-runtime-linux-v6")
        self.assertTrue(SOURCE_MANIFEST_PATH.endswith("_v6.json"))
        self.assertEqual(v5.RECEIPT_SCHEMA_VERSION, 5)
        self.assertEqual(len(v5.REQUIRED_SCENARIOS), 15)
        self.assertEqual(len(REQUIRED_SCENARIOS), 20)

    def test_v6_requires_override_and_helper_namespace_claims(self) -> None:
        self.assertEqual(
            SCENARIO_REQUIRED_CLAIMS["helper_network_namespace_isolation"],
            {"helper_host_interface_tcp_denied", "helper_network_namespace_isolated"},
        )
        self.assertEqual(
            SCENARIO_REQUIRED_OBSERVATIONS["helper_network_namespace_isolation"],
            {
                "canary_scope": "host-interface-tcp",
                "host_canary_reachable": True,
                "helper_namespace_connection": "denied",
            },
        )
        self.assertEqual(
            SCENARIO_REQUIRED_OBSERVATIONS["helper_resource_limit_override_rejection"],
            {
                "override_directive": "TasksMax=infinity",
                "tasks_max": "64",
                "limit_nofile": "256",
                "memory_max_bytes": "268435456",
            },
        )

    def test_v6_manifest_binds_the_trusted_candidate_build_and_release_pipeline(self) -> None:
        manifest = json.loads((Path(__file__).resolve().parents[2] / SOURCE_MANIFEST_PATH).read_text())
        exact_paths = set(manifest["exact_paths"])
        recursive_roots = set(manifest["recursive_roots"])
        required = {
            ".github/workflows/build-release-candidate.yml",
            ".github/workflows/compile-release-payload.yml",
            ".github/workflows/create-release.yml",
            "scripts/build-release-binaries.sh",
            "scripts/build-release.sh",
            "scripts/release_asset_common.sh",
            "scripts/release_build_targets.sh",
            "scripts/release_candidate_manifest.py",
            "scripts/release_ldflags.sh",
            "scripts/release_update_key.go",
            "scripts/require-safe-gh-attestation.sh",
            "scripts/validate-release.sh",
            "scripts/verify-github-release-integrity.sh",
        }
        self.assertEqual(required - exact_paths, set())
        self.assertEqual(
            {"internal/collectorlifecycle", "pkg/tlsutil"} - recursive_roots,
            set(),
        )

    def test_release_sidecar_snapshot_rejects_symlink_before_resolution(self) -> None:
        source = self.root / "source-checksums.txt"
        source.write_text("contents\n", encoding="utf-8")
        symlink = self.root / CHECKSUMS_NAME
        symlink.symlink_to(source)
        with self.assertRaisesRegex(v5.AttestationError, "regular checksums.txt"):
            copy_release_sidecar(symlink, self.root / "copy" / CHECKSUMS_NAME, CHECKSUMS_NAME)

    def test_release_sidecar_snapshot_rejects_source_path_swap(self) -> None:
        source = self.root / CHECKSUMS_NAME
        source.write_text("original\n", encoding="utf-8")
        destination_root = self.root / "private"
        destination_root.mkdir(mode=0o700)
        real_fstat = os.fstat
        calls = 0

        def swap_after_open(file_descriptor):
            nonlocal calls
            result = real_fstat(file_descriptor)
            calls += 1
            if calls == 1:
                source.rename(self.root / "original-checksums.txt")
                source.write_text("replacement\n", encoding="utf-8")
            return result

        with (
            mock.patch("secure_runtime_attestation_v6.os.fstat", side_effect=swap_after_open),
            self.assertRaisesRegex(v5.AttestationError, "changed while it was copied"),
        ):
            copy_release_sidecar(source, destination_root / CHECKSUMS_NAME, CHECKSUMS_NAME)

    def test_create_attestation_uses_private_immutable_artifact_copies(self) -> None:
        observed: dict[str, bytes] = {}

        def inspect_snapshots(**kwargs):
            for name, path in kwargs["artifacts"].items():
                self.assertNotEqual(path, self.artifacts[name])
                observed[name] = path.read_bytes()
                self.artifacts[name].write_bytes(b"swapped-after-snapshot")
                self.assertEqual(path.read_bytes(), name.encode())
            return {"proof_classification": "test"}

        with mock.patch(
            "secure_runtime_attestation_v6._create_attestation_with_snapshotted_artifacts",
            side_effect=inspect_snapshots,
        ):
            result = create_attestation(
                checkout=self.root,
                commit=self.commit,
                main_ref=CANONICAL_MAIN_REF,
                receipt_path=self.root / "receipt.json",
                receipt_record_path="record.json",
                transcript_path=self.root / "transcript.jsonl",
                artifacts=self.artifacts,
                elapsed_seconds=1,
            )
        self.assertEqual(result["proof_classification"], "test")
        self.assertEqual(observed, {name: name.encode() for name in v5.ARTIFACT_ARGUMENTS})

    def test_private_artifact_snapshot_mutation_is_rejected(self) -> None:
        with self.assertRaisesRegex(v5.AttestationError, "snapshot collector_v1 changed"):
            with immutable_artifact_snapshot(self.artifacts) as (snapshots, _):
                snapshots["collector_v1"].chmod(0o600)
                snapshots["collector_v1"].write_bytes(b"mutated")

    def test_rejects_head_branch_and_non_rc_release_identities(self) -> None:
        for value in ("HEAD", "main", "refs/heads/main", "refs/tags/v6.5.0-rc.1", "v6.5.0"):
            with self.subTest(value=value), self.assertRaisesRegex(v5.AttestationError, "exact vX.Y.Z-rc.N"):
                verify_release_candidate_tag_identity(self.root, self.commit, value, CANONICAL_REPOSITORY)

    def test_committed_main_identity_requires_canonical_remote_main(self) -> None:
        def canonical(_checkout, *args, **_kwargs):
            command = tuple(args)
            if command == ("remote", "get-url", "origin"):
                output = CANONICAL_ORIGIN_URL + "\n"
            elif command == ("rev-parse", "--verify", f"{CANONICAL_MAIN_REF}^{{commit}}"):
                output = self.commit + "\n"
            elif command == ("ls-remote", "origin", "refs/heads/main"):
                output = f"{self.commit}\trefs/heads/main\n"
            else:
                raise AssertionError(f"unexpected git call {command}")
            return subprocess.CompletedProcess(args, 0, output.encode(), b"")

        with mock.patch.object(v5, "run_git", side_effect=canonical):
            self.assertEqual(
                verify_canonical_main_identity(self.root, CANONICAL_MAIN_REF),
                self.commit,
            )
        for caller_ref in ("HEAD", "main", "refs/heads/main", "scratch"):
            with self.subTest(caller_ref=caller_ref), self.assertRaisesRegex(
                v5.AttestationError, "canonical origin/main"
            ):
                verify_canonical_main_identity(self.root, caller_ref)

        def moved_remote(_checkout, *args, **kwargs):
            result = canonical(_checkout, *args, **kwargs)
            if tuple(args) == ("ls-remote", "origin", "refs/heads/main"):
                return subprocess.CompletedProcess(args, 0, f"{'c' * 40}\trefs/heads/main\n".encode(), b"")
            return result

        with (
            mock.patch.object(v5, "run_git", side_effect=moved_remote),
            self.assertRaisesRegex(v5.AttestationError, "does not match the remote main commit"),
        ):
            verify_canonical_main_identity(self.root, CANONICAL_MAIN_REF)

    def test_accepts_canonical_annotated_tag_only_as_release_packet_identity(self) -> None:
        with mock.patch.object(v5, "run_git", side_effect=lambda _checkout, *args, **_kwargs: self.git_result(*args)):
            identity = verify_release_candidate_tag_identity(
                self.root, self.commit, self.tag, CANONICAL_REPOSITORY
            )
        self.assertEqual(identity["tag_object"], self.tag_object)
        self.assertEqual(identity["peeled_commit"], self.commit)
        self.assertEqual(identity["tag_authority"], "immutable-signed-github-release-packet")

    def test_rejects_wrong_origin_lightweight_moved_and_signed_tag_objects(self) -> None:
        base = lambda _checkout, *args, **_kwargs: self.git_result(*args)
        cases = {
            "wrong origin": (("remote", "get-url", "origin"), b"https://github.com/attacker/Pulse.git\n", "origin remote"),
            "lightweight": (("cat-file", "-t", f"refs/tags/{self.tag}"), b"commit\n", "annotated tag"),
            "moved": (
                ("ls-remote", "--tags", "origin", f"refs/tags/{self.tag}", f"refs/tags/{self.tag}^{{}}"),
                f"{'c' * 40}\trefs/tags/{self.tag}\n{self.commit}\trefs/tags/{self.tag}^{{}}\n".encode(),
                "does not match locally",
            ),
            "wrong-key signed tag": (
                ("cat-file", "tag", f"refs/tags/{self.tag}"),
                (
                    f"object {self.commit}\ntype commit\ntag {self.tag}\ntagger attacker <a@example.net> 1 +0000\n\n"
                    f"Release {self.tag}\n-----BEGIN PGP SIGNATURE-----\nwrong-key\n"
                ).encode(),
                "tag signatures are not authority",
            ),
        }
        for name, (target_call, replacement, message) in cases.items():
            def dispatch(_checkout, *args, **kwargs):
                if tuple(args) == target_call:
                    return subprocess.CompletedProcess(args, 0, replacement, b"")
                return base(_checkout, *args, **kwargs)

            with self.subTest(name=name), mock.patch.object(v5, "run_git", side_effect=dispatch):
                with self.assertRaisesRegex(v5.AttestationError, message):
                    verify_release_candidate_tag_identity(
                        self.root, self.commit, self.tag, CANONICAL_REPOSITORY
                    )

    def test_accepts_buildvcs_stripped_artifacts_only_with_exact_signed_contract(self) -> None:
        contract_path, _, checksums = self.write_contract_and_checksums()
        verified = verify_release_build_contract(
            path=contract_path,
            tag=self.tag,
            qualified_commit=self.commit,
            repository=CANONICAL_REPOSITORY,
            expected_update_key_fingerprint=self.fingerprint,
            receipt=self.receipt,
            artifact_hashes=self.artifact_hashes,
            checksums=checksums,
        )
        self.assertEqual(set(verified), set(v5.ARTIFACT_ARGUMENTS))
        self.assertTrue(all(item["buildvcs"] is False for item in verified.values()))
        self.assertNotIn("main.Version=vv", self.build_contract()["artifacts"]["collector_v1"]["build"]["ldflags"])

    def test_rejects_local_vcs_stamped_or_wrong_key_build_contract(self) -> None:
        for name, mutate, message in (
            (
                "vcs stamped",
                lambda contract: contract["artifacts"]["collector_v4"]["build"].__setitem__("buildvcs", True),
                "build field buildvcs",
            ),
            (
                "wrong update key",
                lambda contract: contract.__setitem__("update_key_fingerprint", "SHA256:" + "B" * 43 + "="),
                "update_key_fingerprint",
            ),
            (
                "missing ldflags",
                lambda contract: contract["artifacts"]["helper"]["build"].__setitem__("ldflags_sha256", "0" * 64),
                "ldflags digest",
            ),
        ):
            contract = self.build_contract()
            mutate(contract)
            contract_path, _, checksums = self.write_contract_and_checksums(contract)
            with self.subTest(name=name), self.assertRaisesRegex(v5.AttestationError, message):
                verify_release_build_contract(
                    path=contract_path,
                    tag=self.tag,
                    qualified_commit=self.commit,
                    repository=CANONICAL_REPOSITORY,
                    expected_update_key_fingerprint=self.fingerprint,
                    receipt=self.receipt,
                    artifact_hashes=self.artifact_hashes,
                    checksums=checksums,
                )

    def test_release_packet_requires_hosted_assembly_and_compiler_provenance(self) -> None:
        contract_path, checksums_path, _ = self.write_contract_and_checksums()
        assembly_provenance_path = self.root / ASSEMBLY_PROVENANCE_NAME
        assembly_provenance_path.write_text("{}\n", encoding="utf-8")
        compiler_provenance_path = self.root / COMPILER_PROVENANCE_NAME
        compiler_provenance_path.write_text("{}\n", encoding="utf-8")
        calls: list[list[str]] = []

        def record(command, **_kwargs):
            calls.append(list(command))
            return subprocess.CompletedProcess(command, 0, b"{}\n", b"")

        with (
            mock.patch(
                "secure_runtime_attestation_v6.verify_release_candidate_tag_identity",
                return_value={
                    "tag": self.tag,
                    "tag_object": self.tag_object,
                    "peeled_commit": self.commit,
                    "origin_url": CANONICAL_ORIGIN_URL,
                    "tag_authority": "immutable-signed-github-release-packet",
                },
            ),
            mock.patch("secure_runtime_attestation_v6.subprocess.run", side_effect=record),
        ):
            packet = verify_release_candidate_packet(
                checkout=self.root,
                qualified_commit=self.commit,
                tag=self.tag,
                repository=CANONICAL_REPOSITORY,
                release_id="12345",
                checksums_path=checksums_path,
                assembly_provenance_path=assembly_provenance_path,
                compiler_provenance_path=compiler_provenance_path,
                build_contract_path=contract_path,
                expected_update_key_fingerprint=self.fingerprint,
                receipt=self.receipt,
                artifacts=self.artifacts,
                artifact_hashes=self.artifact_hashes,
                collector_signatures=self.collector_signatures,
            )
        self.assertEqual(packet["assembly_signer_workflow"], ASSEMBLY_SIGNER_WORKFLOW)
        self.assertEqual(packet["compiler_signer_workflow"], COMPILER_SIGNER_WORKFLOW)
        self.assertEqual(packet["compiler_runner_trust"], "github-hosted-deny-self-hosted")
        self.assertTrue(any(Path(call[0]).name == "verify-github-release-integrity.sh" for call in calls))
        self.assertEqual(sum(call[:3] == ["gh", "release", "verify-asset"] for call in calls), 8)
        provenance_calls = [call for call in calls if call[:3] == ["gh", "attestation", "verify"]]
        self.assertEqual(len(provenance_calls), 2 + len(v5.ARTIFACT_ARGUMENTS))
        self.assertTrue(all("--deny-self-hosted-runners" in call for call in provenance_calls))
        self.assertIn(ASSEMBLY_SIGNER_WORKFLOW, provenance_calls[0])
        self.assertTrue(all(COMPILER_SIGNER_WORKFLOW in call for call in provenance_calls[1:]))
        signature_calls = [call for call in calls if call[:4] == ["go", "run", "./scripts/release_update_key.go", "verify"]]
        self.assertEqual(len(signature_calls), 4)
        self.assertTrue(all(self.update_public_keys in call for call in signature_calls))
        self.assertEqual(set(packet["collector_signatures"]), {"collector_v1", "collector_v2", "collector_v3", "collector_v4"})

    def test_release_packet_rejects_misnamed_collector_signature(self) -> None:
        contract_path, checksums_path, _ = self.write_contract_and_checksums()
        assembly_provenance_path = self.root / ASSEMBLY_PROVENANCE_NAME
        assembly_provenance_path.write_text("{}\n", encoding="utf-8")
        compiler_provenance_path = self.root / COMPILER_PROVENANCE_NAME
        compiler_provenance_path.write_text("{}\n", encoding="utf-8")
        signatures = dict(self.collector_signatures)
        signatures["collector_v1"] = self.root / "wrong.sig"
        signatures["collector_v1"].write_text("bad\n", encoding="utf-8")

        with (
            mock.patch(
                "secure_runtime_attestation_v6.verify_release_candidate_tag_identity",
                return_value={"tag": self.tag},
            ),
            mock.patch(
                "secure_runtime_attestation_v6.subprocess.run",
                return_value=subprocess.CompletedProcess([], 0, b"{}\n", b""),
            ),
            self.assertRaisesRegex(v5.AttestationError, "must be named"),
        ):
            verify_release_candidate_packet(
                checkout=self.root,
                qualified_commit=self.commit,
                tag=self.tag,
                repository=CANONICAL_REPOSITORY,
                release_id="12345",
                checksums_path=checksums_path,
                assembly_provenance_path=assembly_provenance_path,
                compiler_provenance_path=compiler_provenance_path,
                build_contract_path=contract_path,
                expected_update_key_fingerprint=self.fingerprint,
                receipt=self.receipt,
                artifacts=self.artifacts,
                artifact_hashes=self.artifact_hashes,
                collector_signatures=signatures,
            )

    def test_release_packet_rejects_private_snapshot_swap_during_verification(self) -> None:
        contract_path, checksums_path, _ = self.write_contract_and_checksums()
        assembly_provenance_path = self.root / ASSEMBLY_PROVENANCE_NAME
        assembly_provenance_path.write_text("{}\n", encoding="utf-8")
        compiler_provenance_path = self.root / COMPILER_PROVENANCE_NAME
        compiler_provenance_path.write_text("{}\n", encoding="utf-8")

        def mutate_snapshot(command, **_kwargs):
            if command[:3] == ["gh", "release", "verify-asset"] and Path(command[4]).name == CHECKSUMS_NAME:
                snapshot = Path(command[4])
                snapshot.chmod(0o600)
                snapshot.write_text("swapped\n", encoding="utf-8")
            return subprocess.CompletedProcess(command, 0, b"{}\n", b"")

        with (
            mock.patch(
                "secure_runtime_attestation_v6.verify_release_candidate_tag_identity",
                return_value={"tag": self.tag},
            ),
            mock.patch("secure_runtime_attestation_v6.subprocess.run", side_effect=mutate_snapshot),
            self.assertRaisesRegex(v5.AttestationError, "changed during verification"),
        ):
            verify_release_candidate_packet(
                checkout=self.root,
                qualified_commit=self.commit,
                tag=self.tag,
                repository=CANONICAL_REPOSITORY,
                release_id="12345",
                checksums_path=checksums_path,
                assembly_provenance_path=assembly_provenance_path,
                compiler_provenance_path=compiler_provenance_path,
                build_contract_path=contract_path,
                expected_update_key_fingerprint=self.fingerprint,
                receipt=self.receipt,
                artifacts=self.artifacts,
                artifact_hashes=self.artifact_hashes,
                collector_signatures=self.collector_signatures,
            )

    def test_cli_does_not_accept_v5_release_candidate_ref_shortcut(self) -> None:
        with contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit):
            parse_args(["--release-candidate-ref", self.tag])


if __name__ == "__main__":
    unittest.main()
