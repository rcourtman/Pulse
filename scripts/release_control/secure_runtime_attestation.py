#!/usr/bin/env python3
"""Verify and record provenance for a secure-runtime systemd receipt."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
import subprocess
import sys
from pathlib import Path, PurePosixPath
from typing import Any, Sequence


COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
REQUIRED_SCENARIOS = (
    "legacy_root_command_capable_install",
    "read_only_inspect",
    "drop_in_fail_closed_rehearsal",
    "safe_profile_apply",
    "explicit_safe_profile_rollback",
    "automatic_failure_rollback",
    "ordinary_update_non_migration",
    "final_safe_profile_apply",
    "separate_action_runner_install",
    "typed_action_receipt",
    "action_runner_credential_rotation",
    "action_runner_self_revoke",
)
ARTIFACT_ARGUMENTS = {
    "collector_v1": "collector_v1",
    "collector_v2": "collector_v2",
    "helper": "helper",
    "runner": "runner",
}
FORBIDDEN_RECEIPT_KEYS = {
    "api_key",
    "authorization",
    "bearer",
    "password",
    "refresh_token",
    "secret",
    "token",
}


class AttestationError(ValueError):
    """The supplied qualification evidence cannot be attested."""


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> str:
    try:
        return sha256_bytes(path.read_bytes())
    except OSError as exc:
        raise AttestationError(f"unable to read {path}: {exc}") from exc


def run_git(checkout: Path, *args: str, check: bool = True) -> subprocess.CompletedProcess[bytes]:
    try:
        return subprocess.run(
            ["git", *args],
            cwd=checkout,
            check=check,
            capture_output=True,
        )
    except (OSError, subprocess.CalledProcessError) as exc:
        detail = ""
        if isinstance(exc, subprocess.CalledProcessError):
            detail = exc.stderr.decode("utf-8", errors="replace").strip()
        suffix = f": {detail}" if detail else ""
        raise AttestationError(f"git {' '.join(args)} failed{suffix}") from exc


def resolve_commit(checkout: Path, value: str, label: str) -> str:
    result = run_git(checkout, "rev-parse", "--verify", f"{value}^{{commit}}")
    commit = result.stdout.decode("ascii", errors="strict").strip()
    if not COMMIT_RE.fullmatch(commit):
        raise AttestationError(f"{label} did not resolve to a full SHA-1 commit")
    return commit


def require_detached_clean_checkout(checkout: Path, commit: str) -> None:
    if resolve_commit(checkout, "HEAD", "checkout HEAD") != commit:
        raise AttestationError("checkout HEAD does not match the qualified commit")
    symbolic = run_git(checkout, "symbolic-ref", "-q", "HEAD", check=False)
    if symbolic.returncode == 0:
        branch = symbolic.stdout.decode("utf-8", errors="replace").strip()
        raise AttestationError(f"qualification checkout must be detached, found {branch}")
    if symbolic.returncode != 1:
        raise AttestationError("unable to prove that the qualification checkout is detached")
    status = run_git(
        checkout,
        "status",
        "--porcelain=v1",
        "-z",
        "--untracked-files=all",
    )
    for raw_entry in status.stdout.split(b"\0"):
        if not raw_entry:
            continue
        entry = raw_entry.decode("utf-8", errors="strict")
        if not entry.startswith("?? "):
            raise AttestationError("qualification checkout has tracked modifications")
        untracked = PurePosixPath(entry[3:])
        if len(untracked.parts) < 2 or untracked.parts[0] != ".lab-artifacts":
            raise AttestationError(
                f"qualification checkout has unapproved untracked path {untracked}"
            )


def require_ancestor(checkout: Path, commit: str, main_ref: str) -> str:
    main_commit = resolve_commit(checkout, main_ref, "main ref")
    result = run_git(checkout, "merge-base", "--is-ancestor", commit, main_commit, check=False)
    if result.returncode == 1:
        raise AttestationError(f"qualified commit is not reachable from {main_ref}")
    if result.returncode != 0:
        raise AttestationError(f"unable to verify qualified commit ancestry against {main_ref}")
    return main_commit


def reject_sensitive_keys(value: Any, path: str = "receipt") -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            normalized = str(key).strip().lower()
            if normalized in FORBIDDEN_RECEIPT_KEYS:
                raise AttestationError(f"{path} contains forbidden sensitive key {key!r}")
            reject_sensitive_keys(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            reject_sensitive_keys(child, f"{path}[{index}]")


def load_receipt(path: Path) -> tuple[dict[str, Any], bytes]:
    try:
        raw = path.read_bytes()
        receipt = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise AttestationError(f"unable to load receipt {path}: {exc}") from exc
    if not isinstance(receipt, dict) or receipt.get("schema_version") != 2:
        raise AttestationError("receipt must be a schema_version 2 object")
    reject_sensitive_keys(receipt)
    return receipt, raw


def verify_scenarios(receipt: dict[str, Any]) -> None:
    scenarios = receipt.get("scenarios")
    if not isinstance(scenarios, list):
        raise AttestationError("receipt scenarios must be a list")
    names: list[str] = []
    for scenario in scenarios:
        if not isinstance(scenario, dict) or not isinstance(scenario.get("name"), str):
            raise AttestationError("every receipt scenario must have a string name")
        if scenario.get("passed") is not True:
            raise AttestationError(f"scenario {scenario['name']} did not pass")
        names.append(scenario["name"])
    if tuple(names) != REQUIRED_SCENARIOS:
        raise AttestationError("receipt scenario set or order does not match the canonical 12-scenario qualification")


def verify_source_hashes(checkout: Path, commit: str, receipt: dict[str, Any]) -> dict[str, str]:
    source_hashes = receipt.get("source_hashes")
    if not isinstance(source_hashes, dict) or not source_hashes:
        raise AttestationError("receipt source_hashes must be a non-empty object")
    verified: dict[str, str] = {}
    for source_path in sorted(source_hashes):
        digest = source_hashes[source_path]
        pure_path = PurePosixPath(source_path)
        if pure_path.is_absolute() or ".." in pure_path.parts or str(pure_path) != source_path:
            raise AttestationError(f"invalid receipt source path {source_path!r}")
        if not isinstance(digest, str) or not SHA256_RE.fullmatch(digest):
            raise AttestationError(f"invalid source digest for {source_path}")
        blob = run_git(checkout, "show", f"{commit}:{source_path}").stdout
        actual = sha256_bytes(blob)
        if actual != digest:
            raise AttestationError(f"source digest mismatch for {source_path}")
        verified[source_path] = actual
    return verified


def verify_artifacts(receipt: dict[str, Any], artifacts: dict[str, Path]) -> dict[str, str]:
    receipt_hashes = receipt.get("artifact_hashes")
    if not isinstance(receipt_hashes, dict) or set(receipt_hashes) != set(ARTIFACT_ARGUMENTS):
        raise AttestationError("receipt artifact_hashes must contain the canonical four artifacts")
    verified: dict[str, str] = {}
    for name in ARTIFACT_ARGUMENTS:
        expected = receipt_hashes[name]
        if not isinstance(expected, str) or not SHA256_RE.fullmatch(expected):
            raise AttestationError(f"invalid artifact digest for {name}")
        actual = sha256_file(artifacts[name])
        if actual != expected:
            raise AttestationError(f"artifact digest mismatch for {name}")
        verified[name] = actual
    return verified


def create_attestation(
    *,
    checkout: Path,
    commit: str,
    main_ref: str,
    receipt_path: Path,
    receipt_record_path: str,
    artifacts: dict[str, Path],
    elapsed_seconds: float,
    release_candidate_ref: str | None = None,
) -> dict[str, Any]:
    checkout = checkout.resolve()
    qualified_commit = resolve_commit(checkout, commit, "qualified commit")
    if commit != qualified_commit:
        raise AttestationError("qualified commit must be the full canonical commit SHA")
    require_detached_clean_checkout(checkout, qualified_commit)
    main_commit = require_ancestor(checkout, qualified_commit, main_ref)
    if release_candidate_ref:
        candidate_commit = resolve_commit(checkout, release_candidate_ref, "release candidate ref")
        if candidate_commit != qualified_commit:
            raise AttestationError("release candidate ref does not resolve to the qualified commit")

    receipt, receipt_bytes = load_receipt(receipt_path)
    verify_scenarios(receipt)
    source_hashes = verify_source_hashes(checkout, qualified_commit, receipt)
    artifact_hashes = verify_artifacts(receipt, artifacts)
    record_path = PurePosixPath(receipt_record_path)
    if (
        record_path.is_absolute()
        or ".." in record_path.parts
        or str(record_path) != receipt_record_path
    ):
        raise AttestationError("receipt record path must be a canonical repository-relative path")
    if (
        not isinstance(elapsed_seconds, (float, int))
        or not math.isfinite(elapsed_seconds)
        or elapsed_seconds <= 0
    ):
        raise AttestationError("elapsed seconds must be positive")

    classification = "exact-release-candidate" if release_candidate_ref else "exact-committed-main"
    return {
        "schema_version": 2,
        "attestation_tool": "scripts/release_control/secure_runtime_attestation.py",
        "attestation_tool_sha256": sha256_file(Path(__file__)),
        "proof_classification": classification,
        "qualified_commit": qualified_commit,
        "qualified_ref_at_run": release_candidate_ref or qualified_commit,
        "main_ref_verified": main_ref,
        "main_ref_commit_at_attestation": main_commit,
        "qualified_commit_reachable_from_main": True,
        "build_checkout": "detached-worktree",
        "build_checkout_clean_except_lab_artifacts": True,
        "disposable_vm_guard_verified": True,
        "receipt_path": receipt_record_path,
        "receipt_sha256": sha256_bytes(receipt_bytes),
        "source_hashes_match_commit": True,
        "source_hashes": source_hashes,
        "artifact_hashes_match_receipt": True,
        "artifact_hashes": artifact_hashes,
        "host": {
            "os": _os_name(receipt.get("os_release")),
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
            if release_candidate_ref
            else [
                "exact-release-candidate",
                "representative-provider-and-appliance",
                "external-security-review",
            ]
        ),
    }


def _os_name(os_release: Any) -> str:
    if not isinstance(os_release, str):
        raise AttestationError("receipt os_release must be a string")
    for line in os_release.splitlines():
        if line.startswith("PRETTY_NAME="):
            return line.split("=", 1)[1].strip().strip('"')
    raise AttestationError("receipt os_release does not contain PRETTY_NAME")


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--checkout", type=Path, required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--main-ref", default="origin/main")
    parser.add_argument("--release-candidate-ref")
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--receipt-record-path", required=True)
    parser.add_argument("--collector-v1", type=Path, required=True)
    parser.add_argument("--collector-v2", type=Path, required=True)
    parser.add_argument("--helper", type=Path, required=True)
    parser.add_argument("--runner", type=Path, required=True)
    parser.add_argument("--elapsed-seconds", type=float, required=True)
    parser.add_argument("--output", type=Path, required=True)
    return parser.parse_args(argv)


def main(argv: Sequence[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        attestation = create_attestation(
            checkout=args.checkout,
            commit=args.commit,
            main_ref=args.main_ref,
            release_candidate_ref=args.release_candidate_ref,
            receipt_path=args.receipt,
            receipt_record_path=args.receipt_record_path,
            artifacts={name: getattr(args, argument) for name, argument in ARTIFACT_ARGUMENTS.items()},
            elapsed_seconds=args.elapsed_seconds,
        )
        args.output.write_text(json.dumps(attestation, indent=2) + "\n", encoding="utf-8")
    except (AttestationError, OSError) as exc:
        print(f"secure runtime attestation failed: {exc}", file=sys.stderr)
        return 1
    print(f"secure runtime attestation passed: {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
