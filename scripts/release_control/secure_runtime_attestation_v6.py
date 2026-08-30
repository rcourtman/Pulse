#!/usr/bin/env python3
"""Verify schema-v6 secure-runtime systemd evidence and provenance."""

from __future__ import annotations

import argparse
import base64
import binascii
import contextlib
import hashlib
import json
import math
import os
import re
import stat
import subprocess
import sys
import tempfile
from pathlib import Path, PurePosixPath
from typing import Any, Iterator, Sequence

import secure_runtime_attestation as v5


RECEIPT_SCHEMA_VERSION = 6
ATTESTATION_SCHEMA_VERSION = 6
SOURCE_MANIFEST_PATH = "scripts/release_control/secure_runtime_source_manifest_v6.json"
SOURCE_MANIFEST_SCHEMA_VERSION = 1
SOURCE_MANIFEST_ID = "secure-runtime-linux-v6"
ATTESTATION_TOOL_PATH = "scripts/release_control/secure_runtime_attestation_v6.py"
CANONICAL_REPOSITORY = "rcourtman/Pulse"
CANONICAL_ORIGIN_URL = "https://github.com/rcourtman/Pulse.git"
CANONICAL_MAIN_REF = "origin/main"
ASSEMBLY_SIGNER_WORKFLOW = "github.com/rcourtman/Pulse/.github/workflows/build-release-candidate.yml"
COMPILER_SIGNER_WORKFLOW = "github.com/rcourtman/Pulse/.github/workflows/compile-release-payload.yml"
RELEASE_TAG_RE = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+-rc\.[1-9][0-9]*$")
UPDATE_KEY_FINGERPRINT_RE = re.compile(r"^SHA256:[A-Za-z0-9+/]{43}=$")
GO_VERSION_RE = re.compile(r"^go1\.[0-9]+(?:\.[0-9]+)?$")
BUILD_CONTRACT_NAME = "secure-runtime-build-contract-v1.json"
CHECKSUMS_NAME = "checksums.txt"
ASSEMBLY_PROVENANCE_NAME = "release-build-provenance.sigstore.json"
COMPILER_PROVENANCE_NAME = "secure-runtime-compiler-provenance.sigstore.json"

REQUIRED_SCENARIOS = (
    "legacy_root_command_capable_install",
    "read_only_inspect",
    "drop_in_fail_closed_rehearsal",
    "safe_profile_apply",
    "explicit_safe_profile_rollback",
    "automatic_failure_rollback",
    "ordinary_update_non_migration",
    "final_safe_profile_apply",
    "helper_service_override_rejection",
    "helper_resource_limit_override_rejection",
    "helper_socket_override_rejection",
    "helper_network_namespace_isolation",
    "helper_update_authoritative_commit",
    "helper_update_watchdog_rollback",
    "helper_update_interrupted_recovery",
    "separate_action_runner_install",
    "action_runner_override_rejection",
    "typed_action_receipt",
    "action_runner_credential_rotation",
    "action_runner_self_revoke",
)
SCENARIO_REQUIRED_CLAIMS = {
    **v5.SCENARIO_REQUIRED_CLAIMS,
    "helper_service_override_rejection": {"helper_service_effective_override_detected"},
    "helper_resource_limit_override_rejection": {
        "helper_resource_limits_enforced",
        "helper_resource_limit_override_detected",
    },
    "helper_socket_override_rejection": {"helper_socket_effective_override_detected"},
    "helper_network_namespace_isolation": {
        "helper_host_interface_tcp_denied",
        "helper_network_namespace_isolated",
    },
    "action_runner_override_rejection": {"action_runner_effective_override_detected"},
}
SCENARIO_REQUIRED_OBSERVATIONS = {
    **v5.SCENARIO_REQUIRED_OBSERVATIONS,
    "helper_service_override_rejection": {"override_directive": "PrivateNetwork=false"},
    "helper_resource_limit_override_rejection": {
        "override_directive": "TasksMax=infinity",
        "tasks_max": "64",
        "limit_nofile": "256",
        "memory_max_bytes": "268435456",
    },
    "helper_socket_override_rejection": {"override_directive": "SocketMode=0666"},
    "helper_network_namespace_isolation": {
        "canary_scope": "host-interface-tcp",
        "host_canary_reachable": True,
        "helper_namespace_connection": "denied",
    },
    "action_runner_override_rejection": {
        "override_directive": "EnvironmentFile=-/tmp/unsafe-runner.env"
    },
}


@contextlib.contextmanager
def v6_contract() -> Iterator[None]:
    replacements = {
        "RECEIPT_SCHEMA_VERSION": RECEIPT_SCHEMA_VERSION,
        "SOURCE_MANIFEST_PATH": SOURCE_MANIFEST_PATH,
        "SOURCE_MANIFEST_SCHEMA_VERSION": SOURCE_MANIFEST_SCHEMA_VERSION,
        "SOURCE_MANIFEST_ID": SOURCE_MANIFEST_ID,
        "REQUIRED_SCENARIOS": REQUIRED_SCENARIOS,
        "SCENARIO_REQUIRED_CLAIMS": SCENARIO_REQUIRED_CLAIMS,
        "SCENARIO_REQUIRED_OBSERVATIONS": SCENARIO_REQUIRED_OBSERVATIONS,
    }
    previous = {name: getattr(v5, name) for name in replacements}
    try:
        for name, value in replacements.items():
            setattr(v5, name, value)
        yield
    finally:
        for name, value in previous.items():
            setattr(v5, name, value)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    try:
        with path.open("rb") as handle:
            for chunk in iter(lambda: handle.read(1024 * 1024), b""):
                digest.update(chunk)
    except OSError as exc:
        raise v5.AttestationError(f"unable to read {path}: {exc}") from exc
    return digest.hexdigest()


def load_json_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise v5.AttestationError(f"unable to read {label} {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise v5.AttestationError(f"{label} must be a JSON object")
    return value


def run_checked(command: Sequence[str], *, cwd: Path, label: str) -> subprocess.CompletedProcess[bytes]:
    try:
        return subprocess.run(command, cwd=cwd, check=True, capture_output=True)
    except (OSError, subprocess.CalledProcessError) as exc:
        detail = ""
        if isinstance(exc, subprocess.CalledProcessError):
            detail = exc.stderr.decode("utf-8", errors="replace").strip()
        suffix = f": {detail}" if detail else ""
        raise v5.AttestationError(f"{label} failed{suffix}") from exc


def parse_remote_refs(raw: bytes) -> dict[str, str]:
    result: dict[str, str] = {}
    for line in raw.decode("ascii", errors="strict").splitlines():
        fields = line.split("\t")
        if len(fields) != 2 or not v5.COMMIT_RE.fullmatch(fields[0]):
            raise v5.AttestationError("remote ref query returned malformed output")
        if fields[1] in result:
            raise v5.AttestationError("remote ref query returned duplicate refs")
        result[fields[1]] = fields[0]
    return result


def verify_canonical_main_identity(checkout: Path, main_ref: str) -> str:
    if main_ref != CANONICAL_MAIN_REF:
        raise v5.AttestationError("committed-main classification requires canonical origin/main")
    origin = v5.run_git(checkout, "remote", "get-url", "origin").stdout.decode().strip()
    if origin != CANONICAL_ORIGIN_URL:
        raise v5.AttestationError("origin remote URL is not the canonical Pulse repository URL")
    local_main = v5.resolve_commit(checkout, CANONICAL_MAIN_REF, "canonical origin/main")
    remote = v5.run_git(checkout, "ls-remote", "origin", "refs/heads/main")
    remote_refs = parse_remote_refs(remote.stdout)
    if remote_refs != {"refs/heads/main": local_main}:
        raise v5.AttestationError("canonical origin/main does not match the remote main commit")
    return local_main


def verify_release_candidate_tag_identity(
    checkout: Path,
    qualified_commit: str,
    tag: str,
    repository: str,
) -> dict[str, str]:
    if not RELEASE_TAG_RE.fullmatch(tag):
        raise v5.AttestationError("release candidate identity must be an exact vX.Y.Z-rc.N tag")
    if repository != CANONICAL_REPOSITORY:
        raise v5.AttestationError("release candidate repository is not the canonical Pulse repository")
    origin = v5.run_git(checkout, "remote", "get-url", "origin").stdout.decode().strip()
    if origin != CANONICAL_ORIGIN_URL:
        raise v5.AttestationError("origin remote URL is not the canonical Pulse repository URL")
    ref = f"refs/tags/{tag}"
    local_object = v5.run_git(checkout, "rev-parse", "--verify", ref).stdout.decode().strip()
    if not v5.COMMIT_RE.fullmatch(local_object):
        raise v5.AttestationError("release candidate tag object is invalid")
    object_type = v5.run_git(checkout, "cat-file", "-t", ref).stdout.decode().strip()
    if object_type != "tag":
        raise v5.AttestationError("release candidate tag must be an annotated tag, not a lightweight ref")
    tag_body = v5.run_git(checkout, "cat-file", "tag", ref).stdout.decode("utf-8", errors="strict")
    if (
        f"\ntag {tag}\n" not in tag_body
        or not tag_body.endswith(f"\n\nRelease {tag}\n")
        or "BEGIN PGP SIGNATURE" in tag_body
        or "BEGIN SSH SIGNATURE" in tag_body
    ):
        raise v5.AttestationError(
            "release candidate tag object is not the canonical unsigned workflow tag; tag signatures are not authority"
        )
    peeled_commit = v5.resolve_commit(checkout, ref, "release candidate tag")
    if peeled_commit != qualified_commit:
        raise v5.AttestationError("release candidate tag does not resolve to the qualified commit")
    remote = v5.run_git(
        checkout,
        "ls-remote",
        "--tags",
        "origin",
        ref,
        f"{ref}^{{}}",
    )
    remote_refs = parse_remote_refs(remote.stdout)
    if remote_refs != {ref: local_object, f"{ref}^{{}}": qualified_commit}:
        raise v5.AttestationError("remote release candidate tag object or peeled commit does not match locally")
    return {
        "tag": tag,
        "tag_object": local_object,
        "peeled_commit": qualified_commit,
        "origin_url": origin,
        "tag_authority": "immutable-signed-github-release-packet",
    }


def parse_checksums(path: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise v5.AttestationError(f"unable to read release checksums: {exc}") from exc
    for line_number, line in enumerate(lines, 1):
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9][A-Za-z0-9._-]*)", line)
        if not match:
            raise v5.AttestationError(f"release checksums line {line_number} is not canonical")
        digest, name = match.groups()
        if name in result:
            raise v5.AttestationError(f"release checksums contain duplicate asset {name}")
        result[name] = digest
    if not result:
        raise v5.AttestationError("release checksums are empty")
    return result


def require_canonical_sidecar(path: Path, expected_name: str) -> Path:
    if path.name != expected_name:
        raise v5.AttestationError(f"release sidecar must be a regular {expected_name} file")
    try:
        path_stat = path.lstat()
    except OSError as exc:
        raise v5.AttestationError(f"unable to inspect release sidecar {path}: {exc}") from exc
    if stat.S_ISLNK(path_stat.st_mode) or not stat.S_ISREG(path_stat.st_mode):
        raise v5.AttestationError(f"release sidecar must be a regular {expected_name} file")
    return Path(os.path.abspath(path))


def copy_immutable_input(source: Path, destination: Path, label: str) -> str:
    source = Path(os.path.abspath(source))
    try:
        source_lstat = source.lstat()
    except OSError as exc:
        raise v5.AttestationError(f"{label} changed before it could be copied") from exc
    if stat.S_ISLNK(source_lstat.st_mode) or not stat.S_ISREG(source_lstat.st_mode):
        raise v5.AttestationError(f"{label} must be a regular file")
    flags = os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0)
    try:
        source_fd = os.open(source, flags)
    except OSError as exc:
        raise v5.AttestationError(f"unable to open {label} {source}: {exc}") from exc
    digest = hashlib.sha256()
    try:
        opened_stat = os.fstat(source_fd)
        if (
            not stat.S_ISREG(opened_stat.st_mode)
            or (opened_stat.st_dev, opened_stat.st_ino) != (source_lstat.st_dev, source_lstat.st_ino)
        ):
            raise v5.AttestationError(f"{label} changed before it could be copied")
        try:
            with os.fdopen(source_fd, "rb", closefd=False) as source_handle, destination.open("xb") as target:
                for chunk in iter(lambda: source_handle.read(1024 * 1024), b""):
                    digest.update(chunk)
                    target.write(chunk)
                target.flush()
                os.fsync(target.fileno())
        except OSError as exc:
            raise v5.AttestationError(f"unable to snapshot {label}: {exc}") from exc
        completed_stat = os.fstat(source_fd)
        try:
            path_after = source.lstat()
        except OSError as exc:
            raise v5.AttestationError(f"{label} changed while it was copied") from exc
        stable_identity = (opened_stat.st_dev, opened_stat.st_ino, opened_stat.st_size, opened_stat.st_mtime_ns)
        if (
            stable_identity
            != (completed_stat.st_dev, completed_stat.st_ino, completed_stat.st_size, completed_stat.st_mtime_ns)
            or (path_after.st_dev, path_after.st_ino) != (opened_stat.st_dev, opened_stat.st_ino)
            or stat.S_ISLNK(path_after.st_mode)
        ):
            raise v5.AttestationError(f"{label} changed while it was copied")
    finally:
        os.close(source_fd)
    destination.chmod(0o400)
    copied_digest = sha256_file(destination)
    if copied_digest != digest.hexdigest():
        raise v5.AttestationError(f"{label} snapshot digest is inconsistent")
    return copied_digest


def copy_release_sidecar(source: Path, destination: Path, expected_name: str) -> str:
    source = require_canonical_sidecar(source, expected_name)
    return copy_immutable_input(source, destination, f"release sidecar {expected_name}")


@contextlib.contextmanager
def immutable_release_sidecar_snapshot(
    checksums_path: Path,
    assembly_provenance_path: Path,
    compiler_provenance_path: Path,
    build_contract_path: Path,
) -> Iterator[tuple[dict[str, Path], dict[str, str]]]:
    sources = {
        CHECKSUMS_NAME: checksums_path,
        ASSEMBLY_PROVENANCE_NAME: assembly_provenance_path,
        COMPILER_PROVENANCE_NAME: compiler_provenance_path,
        BUILD_CONTRACT_NAME: build_contract_path,
    }
    with tempfile.TemporaryDirectory(prefix="pulse-secure-runtime-release-") as temporary:
        snapshot_root = Path(temporary)
        snapshot_root.chmod(0o700)
        snapshots: dict[str, Path] = {}
        digests: dict[str, str] = {}
        for name, source in sources.items():
            snapshot = snapshot_root / name
            digests[name] = copy_release_sidecar(source, snapshot, name)
            snapshots[name] = snapshot
        yield snapshots, digests


def verify_release_sidecar_snapshot_unchanged(
    snapshots: dict[str, Path], expected_digests: dict[str, str]
) -> None:
    for name, path in snapshots.items():
        if sha256_file(path) != expected_digests[name]:
            raise v5.AttestationError(f"private release sidecar snapshot {name} changed during verification")


@contextlib.contextmanager
def immutable_artifact_snapshot(
    artifacts: dict[str, Path],
) -> Iterator[tuple[dict[str, Path], dict[str, str]]]:
    if set(artifacts) != set(v5.ARTIFACT_ARGUMENTS):
        raise v5.AttestationError("qualification artifact set is incomplete")
    with tempfile.TemporaryDirectory(prefix="pulse-secure-runtime-artifacts-") as temporary:
        snapshot_root = Path(temporary)
        snapshot_root.chmod(0o700)
        snapshots: dict[str, Path] = {}
        digests: dict[str, str] = {}
        for name in v5.ARTIFACT_ARGUMENTS:
            snapshot = snapshot_root / name
            digests[name] = copy_immutable_input(
                artifacts[name], snapshot, f"qualification artifact {name}"
            )
            snapshots[name] = snapshot
        yield snapshots, digests
        for name, path in snapshots.items():
            if sha256_file(path) != digests[name]:
                raise v5.AttestationError(
                    f"private qualification artifact snapshot {name} changed during verification"
                )


def verify_release_build_contract(
    *,
    path: Path,
    tag: str,
    qualified_commit: str,
    repository: str,
    expected_update_key_fingerprint: str,
    receipt: dict[str, Any],
    artifact_hashes: dict[str, str],
    checksums: dict[str, str],
) -> dict[str, Any]:
    contract = load_json_object(path, "secure-runtime build contract")
    if contract.get("schema_version") != 1:
        raise v5.AttestationError("secure-runtime build contract schema_version must be 1")
    expected_version = tag[1:]
    expected_identity = {
        "repository": repository,
        "assembly_signer_workflow": ASSEMBLY_SIGNER_WORKFLOW,
        "compiler_signer_workflow": COMPILER_SIGNER_WORKFLOW,
        "compiler_runner_trust": "github-hosted-deny-self-hosted",
        "tag": tag,
        "version": expected_version,
        "source_sha": qualified_commit,
        "update_key_fingerprint": expected_update_key_fingerprint,
    }
    for key, expected in expected_identity.items():
        if contract.get(key) != expected:
            raise v5.AttestationError(f"secure-runtime build contract {key} does not match the release")
    if not UPDATE_KEY_FINGERPRINT_RE.fullmatch(expected_update_key_fingerprint):
        raise v5.AttestationError("expected release update-key fingerprint is invalid")
    update_public_keys = contract.get("update_public_keys")
    if not isinstance(update_public_keys, str) or "," in update_public_keys:
        raise v5.AttestationError("secure-runtime build contract must bind one release update public key")
    try:
        update_public_key = base64.b64decode(update_public_keys, validate=True)
    except (binascii.Error, ValueError) as exc:
        raise v5.AttestationError("secure-runtime build contract update public key is invalid") from exc
    if len(update_public_key) != 32:
        raise v5.AttestationError("secure-runtime build contract update public key is not raw Ed25519")
    derived_fingerprint = "SHA256:" + base64.b64encode(hashlib.sha256(update_public_key).digest()).decode("ascii")
    if derived_fingerprint != expected_update_key_fingerprint:
        raise v5.AttestationError("secure-runtime build contract update public key fingerprint does not match")
    if checksums.get(BUILD_CONTRACT_NAME) != sha256_file(path):
        raise v5.AttestationError("secure-runtime build contract is not digest-bound by release checksums")
    artifacts = contract.get("artifacts")
    if not isinstance(artifacts, dict) or set(artifacts) != set(v5.ARTIFACT_ARGUMENTS):
        raise v5.AttestationError("secure-runtime build contract artifact set is incomplete")
    receipt_versions = receipt.get("artifact_versions")
    if not isinstance(receipt_versions, dict):
        raise v5.AttestationError("receipt artifact_versions are unavailable")
    verified: dict[str, Any] = {}
    seen_release_assets: set[str] = set()
    for name, expected_package in v5.EXPECTED_ARTIFACT_PACKAGES.items():
        entry = artifacts[name]
        if not isinstance(entry, dict):
            raise v5.AttestationError(f"secure-runtime build contract artifact {name} is invalid")
        release_asset = entry.get("release_asset")
        if (
            not isinstance(release_asset, str)
            or PurePosixPath(release_asset).name != release_asset
            or release_asset in seen_release_assets
        ):
            raise v5.AttestationError(f"secure-runtime build contract release asset for {name} is invalid")
        seen_release_assets.add(release_asset)
        if entry.get("sha256") != artifact_hashes[name] or checksums.get(release_asset) != artifact_hashes[name]:
            raise v5.AttestationError(f"artifact {name} is not bound to the signed release checksums")
        build = entry.get("build")
        if not isinstance(build, dict):
            raise v5.AttestationError(f"artifact {name} has no exact build contract")
        required_build = {
            "tool": "go build",
            "package": expected_package,
            "target_os": "linux",
            "target_arch": receipt.get("architecture"),
            "cgo_enabled": 0,
            "trimpath": True,
            "buildvcs": False,
            "build_args": ["-buildvcs=false", "-trimpath"],
            "update_key_fingerprint": expected_update_key_fingerprint,
        }
        for key, expected in required_build.items():
            if build.get(key) != expected:
                raise v5.AttestationError(f"artifact {name} build field {key} does not match the hosted compiler contract")
        if not GO_VERSION_RE.fullmatch(str(build.get("go_version", ""))):
            raise v5.AttestationError(f"artifact {name} Go toolchain is invalid")
        artifact_version = build.get("version")
        if not isinstance(artifact_version, str) or not artifact_version:
            raise v5.AttestationError(f"artifact {name} version is invalid")
        normalized_artifact_version = artifact_version.removeprefix("v")
        if name.startswith("collector_v"):
            receipt_version = receipt_versions.get(name)
            if (
                not isinstance(receipt_version, str)
                or normalized_artifact_version != receipt_version.removeprefix("v")
            ):
                raise v5.AttestationError(f"artifact {name} version does not match the receipt")
        elif normalized_artifact_version != expected_version:
            raise v5.AttestationError(f"artifact {name} version does not match the release")
        expected_ldflags = ""
        if name != "runner":
            embedded_version = artifact_version if artifact_version.startswith("v") else f"v{artifact_version}"
            expected_ldflags = (
                f"-s -w -X main.Version={embedded_version} "
                "-X github.com/rcourtman/pulse-go-rewrite/internal/updatesignature."
                f"EmbeddedTrustedPublicKeys={update_public_keys}"
            )
        if build.get("ldflags") != expected_ldflags:
            raise v5.AttestationError(f"artifact {name} ldflags do not match the canonical release invocation")
        ldflags_sha256 = hashlib.sha256(expected_ldflags.encode()).hexdigest()
        if build.get("ldflags_sha256") != ldflags_sha256:
            raise v5.AttestationError(f"artifact {name} ldflags digest is invalid")
        verified[name] = {
            "release_asset": release_asset,
            "sha256": artifact_hashes[name],
            "package": expected_package,
            "go_version": build["go_version"],
            "buildvcs": False,
            "trimpath": True,
            "ldflags_sha256": ldflags_sha256,
            "version": artifact_version,
            "update_key_fingerprint": expected_update_key_fingerprint,
        }
    return verified


def verify_release_candidate_packet(
    *,
    checkout: Path,
    qualified_commit: str,
    tag: str,
    repository: str,
    release_id: str,
    checksums_path: Path,
    assembly_provenance_path: Path,
    compiler_provenance_path: Path,
    build_contract_path: Path,
    expected_update_key_fingerprint: str,
    receipt: dict[str, Any],
    artifacts: dict[str, Path],
    artifact_hashes: dict[str, str],
) -> dict[str, Any]:
    tag_identity = verify_release_candidate_tag_identity(checkout, qualified_commit, tag, repository)
    if not re.fullmatch(r"[1-9][0-9]*", release_id):
        raise v5.AttestationError("release id must be a positive GitHub release id")
    integrity_script = checkout / "scripts/verify-github-release-integrity.sh"
    run_checked(
        [str(integrity_script), tag, repository, release_id, qualified_commit],
        cwd=checkout,
        label="immutable GitHub release integrity verification",
    )
    with immutable_release_sidecar_snapshot(
        checksums_path,
        assembly_provenance_path,
        compiler_provenance_path,
        build_contract_path,
    ) as (snapshots, snapshot_digests):
        checksums_snapshot = snapshots[CHECKSUMS_NAME]
        assembly_provenance_snapshot = snapshots[ASSEMBLY_PROVENANCE_NAME]
        compiler_provenance_snapshot = snapshots[COMPILER_PROVENANCE_NAME]
        build_contract_snapshot = snapshots[BUILD_CONTRACT_NAME]
        for sidecar in snapshots.values():
            run_checked(
                [
                    "gh",
                    "release",
                    "verify-asset",
                    tag,
                    str(sidecar),
                    "--repo",
                    repository,
                    "--format",
                    "json",
                ],
                cwd=checkout,
                label=f"release attestation verification for {sidecar.name}",
            )
        run_checked(
            [
                "gh",
                "attestation",
                "verify",
                str(checksums_snapshot),
                "--repo",
                repository,
                "--signer-workflow",
                ASSEMBLY_SIGNER_WORKFLOW,
                "--source-digest",
                qualified_commit,
                "--deny-self-hosted-runners",
                "--predicate-type",
                "https://slsa.dev/provenance/v1",
                "--bundle",
                str(assembly_provenance_snapshot),
            ],
            cwd=checkout,
            label="hosted candidate-assembly provenance verification",
        )
        for artifact_name, artifact_path in artifacts.items():
            run_checked(
                [
                    "gh",
                    "attestation",
                    "verify",
                    str(artifact_path),
                    "--repo",
                    repository,
                    "--signer-workflow",
                    COMPILER_SIGNER_WORKFLOW,
                    "--source-digest",
                    qualified_commit,
                    "--deny-self-hosted-runners",
                    "--predicate-type",
                    "https://slsa.dev/provenance/v1",
                    "--bundle",
                    str(compiler_provenance_snapshot),
                ],
                cwd=checkout,
                label=f"hosted compiler provenance verification for {artifact_name}",
            )
        verify_release_sidecar_snapshot_unchanged(snapshots, snapshot_digests)
        checksums = parse_checksums(checksums_snapshot)
        build_identity = verify_release_build_contract(
            path=build_contract_snapshot,
            tag=tag,
            qualified_commit=qualified_commit,
            repository=repository,
            expected_update_key_fingerprint=expected_update_key_fingerprint,
            receipt=receipt,
            artifact_hashes=artifact_hashes,
            checksums=checksums,
        )
        verify_release_sidecar_snapshot_unchanged(snapshots, snapshot_digests)
        return {
            **tag_identity,
            "release_id": release_id,
            "checksums_sha256": snapshot_digests[CHECKSUMS_NAME],
            "assembly_provenance_sha256": snapshot_digests[ASSEMBLY_PROVENANCE_NAME],
            "compiler_provenance_sha256": snapshot_digests[COMPILER_PROVENANCE_NAME],
            "build_contract_sha256": snapshot_digests[BUILD_CONTRACT_NAME],
            "assembly_signer_workflow": ASSEMBLY_SIGNER_WORKFLOW,
            "compiler_signer_workflow": COMPILER_SIGNER_WORKFLOW,
            "compiler_runner_trust": "github-hosted-deny-self-hosted",
            "build_identity": build_identity,
            "update_key_fingerprint": expected_update_key_fingerprint,
        }


def _create_attestation_with_snapshotted_artifacts(
    *,
    checkout: Path,
    commit: str,
    main_ref: str,
    receipt_path: Path,
    receipt_record_path: str,
    transcript_path: Path,
    artifacts: dict[str, Path],
    elapsed_seconds: float,
    release_candidate_tag: str | None = None,
    release_repository: str | None = None,
    release_id: str | None = None,
    release_checksums_path: Path | None = None,
    release_assembly_provenance_path: Path | None = None,
    release_compiler_provenance_path: Path | None = None,
    release_build_contract_path: Path | None = None,
    expected_release_update_key_fingerprint: str | None = None,
) -> dict[str, Any]:
    checkout = checkout.resolve()
    qualified_commit = v5.resolve_commit(checkout, commit, "qualified commit")
    if commit != qualified_commit:
        raise v5.AttestationError("qualified commit must be the full canonical commit SHA")
    v5.require_detached_clean_checkout(checkout, qualified_commit)
    main_commit = verify_canonical_main_identity(checkout, main_ref)
    v5.require_ancestor(checkout, qualified_commit, CANONICAL_MAIN_REF)
    with v6_contract():
        receipt, receipt_bytes = v5.load_receipt(receipt_path)
        record_path = v5.canonical_repo_path(receipt_record_path, "receipt record path")
        if receipt.get("record_path") != record_path:
            raise v5.AttestationError("receipt record path is not bound inside the supplied receipt")
        transcript_events, transcript_bytes = v5.load_and_verify_transcript(transcript_path, receipt)
        v5.verify_scenarios(receipt, transcript_events)
        v5.verify_runtime_claims(receipt)
        source_hashes, source_manifest = v5.verify_source_hashes(checkout, qualified_commit, receipt)
    attestation_tool_hash = sha256_file(Path(__file__))
    if source_hashes.get(ATTESTATION_TOOL_PATH) != attestation_tool_hash:
        raise v5.AttestationError("executing schema-v6 attestation tool does not match the qualified commit")
    artifact_hashes = v5.verify_artifacts(receipt, artifacts)
    if not isinstance(elapsed_seconds, (float, int)) or not math.isfinite(elapsed_seconds) or elapsed_seconds <= 0:
        raise v5.AttestationError("elapsed seconds must be positive")

    release_arguments = (
        release_repository,
        release_id,
        release_checksums_path,
        release_assembly_provenance_path,
        release_compiler_provenance_path,
        release_build_contract_path,
        expected_release_update_key_fingerprint,
    )
    release_packet: dict[str, Any] | None = None
    if release_candidate_tag is None:
        if any(value is not None for value in release_arguments):
            raise v5.AttestationError("release packet inputs require --release-candidate-tag")
        artifact_build_identity = v5.verify_artifact_build_identity(receipt, artifacts, qualified_commit)
        classification = "committed-main-artifact-bound-self-attested-systemd"
    else:
        if any(value is None for value in release_arguments):
            raise v5.AttestationError(
                "release-candidate classification requires repository, release id, signed checksums, hosted assembly and compiler provenance, build contract, and update-key fingerprint"
            )
        release_packet = verify_release_candidate_packet(
            checkout=checkout,
            qualified_commit=qualified_commit,
            tag=release_candidate_tag,
            repository=str(release_repository),
            release_id=str(release_id),
            checksums_path=Path(release_checksums_path),
            assembly_provenance_path=Path(release_assembly_provenance_path),
            compiler_provenance_path=Path(release_compiler_provenance_path),
            build_contract_path=Path(release_build_contract_path),
            expected_update_key_fingerprint=str(expected_release_update_key_fingerprint),
            receipt=receipt,
            artifacts=artifacts,
            artifact_hashes=artifact_hashes,
        )
        artifact_build_identity = release_packet["build_identity"]
        classification = "release-candidate-hosted-compiler-chain-artifact-bound-self-attested-systemd"

    return {
        "schema_version": ATTESTATION_SCHEMA_VERSION,
        "attestation_tool": ATTESTATION_TOOL_PATH,
        "attestation_tool_sha256": attestation_tool_hash,
        "proof_classification": classification,
        "qualified_commit": qualified_commit,
        "qualified_ref_at_run": release_candidate_tag or qualified_commit,
        "main_ref_verified": main_ref,
        "main_ref_commit_at_attestation": main_commit,
        "qualified_commit_reachable_from_main": True,
        "build_checkout": "detached-worktree",
        "build_checkout_clean_except_lab_artifacts": True,
        "disposable_vm_guard_receipt_claim_validated": True,
        "execution_receipt_authentication": "none-secret-free-self-attestation",
        "receipt": {
            "record_path": record_path,
            "sha256": v5.sha256_bytes(receipt_bytes),
            "path_bound_inside_receipt": True,
        },
        "transcript": {
            "record_path": receipt["transcript"]["record_path"],
            "sha256": v5.sha256_bytes(transcript_bytes),
            "event_count": len(transcript_events),
            "scenario_event_count": sum(event.get("kind") == "scenario_result" for event in transcript_events),
            "command_output_event_count": sum(event.get("kind") == "command_output" for event in transcript_events),
            "format": "jsonl-v1",
        },
        "source_manifest": source_manifest,
        "source_hashes_match_commit": True,
        "source_hashes": source_hashes,
        "artifact_hashes_match_receipt": True,
        "artifact_hashes": artifact_hashes,
        "artifact_build_identity": artifact_build_identity,
        "release_packet": release_packet,
        "host": {
            "os": v5._os_name(receipt.get("os_release")),
            "kernel": receipt.get("kernel"),
            "systemd": receipt.get("systemd_version"),
            "architecture": receipt.get("architecture"),
        },
        "scenario_count": len(REQUIRED_SCENARIOS),
        "all_scenarios_passed": True,
        "test_elapsed_seconds": float(elapsed_seconds),
        "default_changed": False,
        "residual_proof": (
            ["representative-provider-and-appliance", "external-security-review"]
            if release_packet
            else [
                "exact-release-candidate-hosted-compiler-chain-artifacts",
                "representative-provider-and-appliance",
                "external-security-review",
            ]
        ),
    }


def create_attestation(
    *,
    checkout: Path,
    commit: str,
    main_ref: str,
    receipt_path: Path,
    receipt_record_path: str,
    transcript_path: Path,
    artifacts: dict[str, Path],
    elapsed_seconds: float,
    release_candidate_tag: str | None = None,
    release_repository: str | None = None,
    release_id: str | None = None,
    release_checksums_path: Path | None = None,
    release_assembly_provenance_path: Path | None = None,
    release_compiler_provenance_path: Path | None = None,
    release_build_contract_path: Path | None = None,
    expected_release_update_key_fingerprint: str | None = None,
) -> dict[str, Any]:
    # Hash, inspect, and externally verify the same private artifact bytes.
    # Caller-owned paths can otherwise be swapped between receipt hashing,
    # Go build-identity inspection, and compiler provenance verification.
    with immutable_artifact_snapshot(artifacts) as (snapshots, _):
        return _create_attestation_with_snapshotted_artifacts(
            checkout=checkout,
            commit=commit,
            main_ref=main_ref,
            receipt_path=receipt_path,
            receipt_record_path=receipt_record_path,
            transcript_path=transcript_path,
            artifacts=snapshots,
            elapsed_seconds=elapsed_seconds,
            release_candidate_tag=release_candidate_tag,
            release_repository=release_repository,
            release_id=release_id,
            release_checksums_path=release_checksums_path,
            release_assembly_provenance_path=release_assembly_provenance_path,
            release_compiler_provenance_path=release_compiler_provenance_path,
            release_build_contract_path=release_build_contract_path,
            expected_release_update_key_fingerprint=expected_release_update_key_fingerprint,
        )


def parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--checkout", type=Path, required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--main-ref", default=CANONICAL_MAIN_REF)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--receipt-record-path", required=True)
    parser.add_argument("--transcript", type=Path, required=True)
    parser.add_argument("--collector-v1", type=Path, required=True)
    parser.add_argument("--collector-v2", type=Path, required=True)
    parser.add_argument("--collector-v3", type=Path, required=True)
    parser.add_argument("--collector-v4", type=Path, required=True)
    parser.add_argument("--helper", type=Path, required=True)
    parser.add_argument("--runner", type=Path, required=True)
    parser.add_argument("--elapsed-seconds", type=float, required=True)
    parser.add_argument("--release-candidate-tag")
    parser.add_argument("--release-repository")
    parser.add_argument("--release-id")
    parser.add_argument("--release-checksums", type=Path)
    parser.add_argument("--release-assembly-provenance", type=Path)
    parser.add_argument("--release-compiler-provenance", type=Path)
    parser.add_argument("--release-build-contract", type=Path)
    parser.add_argument("--expected-release-update-key-fingerprint")
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(sys.argv[1:] if argv is None else argv)
    artifacts = {
        "collector_v1": args.collector_v1,
        "collector_v2": args.collector_v2,
        "collector_v3": args.collector_v3,
        "collector_v4": args.collector_v4,
        "helper": args.helper,
        "runner": args.runner,
    }
    try:
        attestation = create_attestation(
            checkout=args.checkout,
            commit=args.commit,
            main_ref=args.main_ref,
            receipt_path=args.receipt,
            receipt_record_path=args.receipt_record_path,
            transcript_path=args.transcript,
            artifacts=artifacts,
            elapsed_seconds=args.elapsed_seconds,
            release_candidate_tag=args.release_candidate_tag,
            release_repository=args.release_repository,
            release_id=args.release_id,
            release_checksums_path=args.release_checksums,
            release_assembly_provenance_path=args.release_assembly_provenance,
            release_compiler_provenance_path=args.release_compiler_provenance,
            release_build_contract_path=args.release_build_contract,
            expected_release_update_key_fingerprint=args.expected_release_update_key_fingerprint,
        )
        args.output.write_text(json.dumps(attestation, indent=2) + "\n", encoding="utf-8")
    except v5.AttestationError as exc:
        print(f"secure runtime schema-v6 attestation failed: {exc}", file=sys.stderr)
        return 1
    print(f"secure runtime schema-v6 attestation passed: {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
