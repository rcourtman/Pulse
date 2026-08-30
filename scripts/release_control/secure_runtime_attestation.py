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
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath
from typing import Any, Sequence


COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
RECEIPT_SCHEMA_VERSION = 4
ATTESTATION_SCHEMA_VERSION = 4
SOURCE_MANIFEST_PATH = "scripts/release_control/secure_runtime_source_manifest_v4.json"
SOURCE_MANIFEST_SCHEMA_VERSION = 1
SOURCE_MANIFEST_ID = "secure-runtime-linux-v4"
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
SCENARIO_REQUIRED_CLAIMS = {
    "legacy_root_command_capable_install": {"legacy_root_command_authority_observed"},
    "read_only_inspect": {"inspection_left_stable_files_unchanged"},
    "drop_in_fail_closed_rehearsal": {"drop_in_rejected_before_mutation"},
    "safe_profile_apply": {
        "collector_non_root",
        "collector_monitoring_only",
        "helper_protocol_healthy",
        "collector_authority_reduction_observed",
    },
    "explicit_safe_profile_rollback": {"explicit_rollback_preserved_reduced_authority"},
    "automatic_failure_rollback": {"failed_activation_restored_prior_runtime"},
    "ordinary_update_non_migration": {"ordinary_update_preserved_selected_profile"},
    "final_safe_profile_apply": {"collector_reporting_continued_after_migration"},
    "separate_action_runner_install": {"action_runner_registered_separately"},
    "typed_action_receipt": {
        "typed_mutation_verified",
        "terminal_receipt_replayed",
        "stale_precondition_refused",
        "generic_command_denied",
    },
    "action_runner_credential_rotation": {"fixture_credential_replacement_observed"},
    "action_runner_self_revoke": {"exact_runner_binding_revoked"},
}
SCENARIO_REQUIRED_OBSERVATIONS = {
    "legacy_root_command_capable_install": {"collector_process_uid": 0, "commands_enabled": True},
    "read_only_inspect": {"stable_files_unchanged": True},
    "drop_in_fail_closed_rehearsal": {"rejected_before_mutation": True},
    "safe_profile_apply": {
        "collector_service_user": "pulse-agent",
        "collector_authority": "monitoring-only",
        "helper_status": "ok",
    },
    "explicit_safe_profile_rollback": {"restored_profile": "root-monitoring", "commands_enabled": False},
    "automatic_failure_rollback": {"activation_committed": False, "restored_profile": "root-monitoring"},
    "ordinary_update_non_migration": {"collector_v2_installed": True, "selected_profile": "root-monitoring"},
    "final_safe_profile_apply": {"collector_service_user": "pulse-agent", "continuity_report_observed": True},
    "separate_action_runner_install": {"runner_service_user": "root", "collector_service_user": "pulse-agent"},
    "typed_action_receipt": {
        "action_receipt_kind": "pulse.host_storage_cleanup_result",
        "mutation_started": True,
        "verification": "verified",
        "stale_precondition_mutation_started": False,
        "generic_command_denied": True,
    },
    "action_runner_credential_rotation": {
        "proof_scope": "in-memory-fixture",
        "superseded_session_invalidated": True,
        "replacement_registered": True,
    },
    "action_runner_self_revoke": {"revocation_count": 1, "collector_continuity": True},
}
ARTIFACT_ARGUMENTS = {
    "collector_v1": "collector_v1",
    "collector_v2": "collector_v2",
    "helper": "helper",
    "runner": "runner",
}
EXPECTED_ARTIFACT_PACKAGES = {
    "collector_v1": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent",
    "collector_v2": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent",
    "helper": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper",
    "runner": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-runner",
}
DISPOSABLE_VM_GUARD_SHA256 = hashlib.sha256(
    b"PULSE_SECURE_RUNTIME_SYSTEMD_LAB=disposable-v1\n"
).hexdigest()
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


def canonical_repo_path(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value:
        raise AttestationError(f"{label} must be a non-empty repository-relative path")
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts or str(path) != value:
        raise AttestationError(f"{label} must be a canonical repository-relative path")
    return value


def load_source_manifest(checkout: Path, commit: str) -> tuple[dict[str, Any], bytes, set[str]]:
    raw = run_git(checkout, "show", f"{commit}:{SOURCE_MANIFEST_PATH}").stdout
    try:
        manifest = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AttestationError(f"unable to decode {SOURCE_MANIFEST_PATH}: {exc}") from exc
    if not isinstance(manifest, dict):
        raise AttestationError("secure-runtime source manifest must be an object")
    if manifest.get("schema_version") != SOURCE_MANIFEST_SCHEMA_VERSION:
        raise AttestationError("secure-runtime source manifest schema is unsupported")
    if manifest.get("manifest_id") != SOURCE_MANIFEST_ID:
        raise AttestationError("secure-runtime source manifest identity is unsupported")
    if manifest.get("target_os") != "linux":
        raise AttestationError("secure-runtime source manifest must target Linux")

    exact_paths = manifest.get("exact_paths")
    recursive_roots = manifest.get("recursive_roots")
    include_suffixes = manifest.get("include_suffixes")
    exclude_suffixes = manifest.get("exclude_suffixes")
    if not isinstance(exact_paths, list) or not exact_paths:
        raise AttestationError("secure-runtime source manifest exact_paths must be a non-empty list")
    if not isinstance(recursive_roots, list) or not recursive_roots:
        raise AttestationError("secure-runtime source manifest recursive_roots must be a non-empty list")
    if not isinstance(include_suffixes, list) or not include_suffixes:
        raise AttestationError("secure-runtime source manifest include_suffixes must be a non-empty list")
    if not isinstance(exclude_suffixes, list):
        raise AttestationError("secure-runtime source manifest exclude_suffixes must be a list")
    if any(not isinstance(suffix, str) or not suffix for suffix in include_suffixes + exclude_suffixes):
        raise AttestationError("secure-runtime source manifest suffixes must be non-empty strings")

    expected = {canonical_repo_path(value, "source manifest exact path") for value in exact_paths}
    if SOURCE_MANIFEST_PATH not in expected:
        raise AttestationError("secure-runtime source manifest must include itself")
    for raw_root in recursive_roots:
        root = canonical_repo_path(raw_root, "source manifest recursive root").rstrip("/")
        tree = run_git(checkout, "ls-tree", "-r", "--name-only", commit, "--", root).stdout
        matched = 0
        for raw_path in tree.decode("utf-8", errors="strict").splitlines():
            source_path = canonical_repo_path(raw_path, "source manifest expanded path")
            if not any(source_path.endswith(suffix) for suffix in include_suffixes):
                continue
            if any(source_path.endswith(suffix) for suffix in exclude_suffixes):
                continue
            expected.add(source_path)
            matched += 1
        if matched == 0:
            raise AttestationError(f"secure-runtime source manifest root has no production sources: {root}")
    return manifest, raw, expected


def load_receipt(path: Path) -> tuple[dict[str, Any], bytes]:
    try:
        raw = path.read_bytes()
        receipt = json.loads(raw)
    except (OSError, json.JSONDecodeError) as exc:
        raise AttestationError(f"unable to load receipt {path}: {exc}") from exc
    if not isinstance(receipt, dict) or receipt.get("schema_version") != RECEIPT_SCHEMA_VERSION:
        raise AttestationError(f"receipt must be a schema_version {RECEIPT_SCHEMA_VERSION} object")
    reject_sensitive_keys(receipt)
    return receipt, raw


def parse_utc_timestamp(value: Any, label: str) -> datetime:
    if not isinstance(value, str) or not value:
        raise AttestationError(f"{label} must be an RFC3339 timestamp")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise AttestationError(f"{label} must be an RFC3339 timestamp") from exc
    if parsed.tzinfo is None or parsed.utcoffset() != timezone.utc.utcoffset(parsed):
        raise AttestationError(f"{label} must be UTC")
    return parsed


def load_and_verify_transcript(
    path: Path, receipt: dict[str, Any]
) -> tuple[list[dict[str, Any]], bytes]:
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise AttestationError(f"unable to read transcript {path}: {exc}") from exc
    transcript = receipt.get("transcript")
    if not isinstance(transcript, dict):
        raise AttestationError("receipt transcript binding must be an object")
    if transcript.get("format") != "jsonl-v1":
        raise AttestationError("receipt transcript format must be jsonl-v1")
    canonical_repo_path(transcript.get("record_path"), "transcript record path")
    expected_digest = transcript.get("sha256")
    if not isinstance(expected_digest, str) or not SHA256_RE.fullmatch(expected_digest):
        raise AttestationError("receipt transcript digest is invalid")
    if sha256_bytes(raw) != expected_digest:
        raise AttestationError("transcript digest mismatch")

    events: list[dict[str, Any]] = []
    previous_time: datetime | None = None
    event_ids: set[str] = set()
    for line_number, raw_line in enumerate(raw.splitlines(), 1):
        if not raw_line.strip():
            raise AttestationError(f"transcript line {line_number} is empty")
        try:
            event = json.loads(raw_line)
        except json.JSONDecodeError as exc:
            raise AttestationError(f"transcript line {line_number} is not JSON") from exc
        if not isinstance(event, dict):
            raise AttestationError(f"transcript line {line_number} must be an object")
        reject_sensitive_keys(event, f"transcript[{line_number}]")
        if event.get("sequence") != line_number:
            raise AttestationError("transcript event sequence is not contiguous")
        event_id = event.get("event_id")
        if not isinstance(event_id, str) or not event_id or event_id in event_ids:
            raise AttestationError("transcript event IDs must be non-empty and unique")
        event_ids.add(event_id)
        observed_at = parse_utc_timestamp(event.get("observed_at"), f"transcript event {event_id} observed_at")
        if previous_time is not None and observed_at < previous_time:
            raise AttestationError("transcript chronology is not monotonic")
        previous_time = observed_at
        kind = event.get("kind")
        if kind == "scenario_result":
            if not isinstance(event.get("scenario"), str):
                raise AttestationError(f"transcript event {event_id} has no scenario")
            claims = event.get("claims")
            if not isinstance(claims, list) or not claims or any(not isinstance(claim, str) or not claim for claim in claims):
                raise AttestationError(f"transcript event {event_id} claims are invalid")
            if len(set(claims)) != len(claims):
                raise AttestationError(f"transcript event {event_id} repeats a claim")
            observations = event.get("observations")
            if not isinstance(observations, dict) or not observations:
                raise AttestationError(f"transcript event {event_id} observations are invalid")
            if not isinstance(event.get("summary"), str) or not event["summary"].strip():
                raise AttestationError(f"transcript event {event_id} summary is invalid")
        elif kind == "command_output":
            if not isinstance(event.get("operation"), str) or not event["operation"].strip():
                raise AttestationError(f"transcript command event {event_id} operation is invalid")
            output = event.get("output")
            if not isinstance(output, str):
                raise AttestationError(f"transcript command event {event_id} output is invalid")
            output_digest = event.get("output_sha256")
            if not isinstance(output_digest, str) or output_digest != sha256_bytes(output.encode("utf-8")):
                raise AttestationError(f"transcript command event {event_id} output digest is invalid")
        else:
            raise AttestationError(f"transcript event {event_id} has an unsupported kind")
        events.append(event)
    scenario_event_count = sum(event.get("kind") == "scenario_result" for event in events)
    command_event_count = sum(event.get("kind") == "command_output" for event in events)
    if transcript.get("event_count") != len(events) or scenario_event_count != len(REQUIRED_SCENARIOS) or command_event_count < 1:
        raise AttestationError("transcript event count does not contain the canonical scenarios and raw command evidence")
    return events, raw


def verify_scenarios(receipt: dict[str, Any], transcript_events: list[dict[str, Any]]) -> None:
    scenarios = receipt.get("scenarios")
    if not isinstance(scenarios, list):
        raise AttestationError("receipt scenarios must be a list")
    names: list[str] = []
    run_started = parse_utc_timestamp(receipt.get("started_at"), "receipt started_at")
    run_completed = parse_utc_timestamp(receipt.get("completed_at"), "receipt completed_at")
    if run_completed < run_started:
        raise AttestationError("receipt chronology ends before it starts")
    for event in transcript_events:
        observed = parse_utc_timestamp(event.get("observed_at"), f"transcript event {event.get('event_id')} observed_at")
        if observed < run_started or observed > run_completed:
            raise AttestationError("transcript event falls outside the receipt chronology")
    previous_completed = run_started
    transcript_by_id = {event["event_id"]: event for event in transcript_events}
    for index, scenario in enumerate(scenarios, 1):
        if not isinstance(scenario, dict) or not isinstance(scenario.get("name"), str):
            raise AttestationError("every receipt scenario must have a string name")
        if scenario.get("passed") is not True:
            raise AttestationError(f"scenario {scenario['name']} did not pass")
        if scenario.get("sequence") != index:
            raise AttestationError("receipt scenario sequence is not contiguous")
        started = parse_utc_timestamp(scenario.get("started_at"), f"scenario {scenario['name']} started_at")
        completed = parse_utc_timestamp(scenario.get("completed_at"), f"scenario {scenario['name']} completed_at")
        if started < previous_completed or completed < started or completed > run_completed:
            raise AttestationError(f"scenario {scenario['name']} chronology is invalid")
        previous_completed = completed
        evidence = scenario.get("evidence")
        if not isinstance(evidence, dict) or evidence.get("kind") != "runtime-observation-v1":
            raise AttestationError(f"scenario {scenario['name']} evidence kind is invalid")
        if not isinstance(evidence.get("summary"), str) or not evidence["summary"].strip():
            raise AttestationError(f"scenario {scenario['name']} evidence summary is invalid")
        claims = evidence.get("claims")
        if not isinstance(claims, list) or any(not isinstance(claim, str) or not claim for claim in claims):
            raise AttestationError(f"scenario {scenario['name']} evidence claims are invalid")
        required_claims = SCENARIO_REQUIRED_CLAIMS.get(scenario["name"], set())
        if not required_claims.issubset(set(claims)):
            raise AttestationError(f"scenario {scenario['name']} omits required causal claims")
        observations = evidence.get("observations")
        if not isinstance(observations, dict) or not observations:
            raise AttestationError(f"scenario {scenario['name']} observations are invalid")
        required_observations = SCENARIO_REQUIRED_OBSERVATIONS.get(scenario["name"], {})
        if any(observations.get(key) != value for key, value in required_observations.items()):
            raise AttestationError(f"scenario {scenario['name']} omits required typed observations")
        event_ids = evidence.get("transcript_event_ids")
        if not isinstance(event_ids, list) or len(event_ids) != 1 or not isinstance(event_ids[0], str):
            raise AttestationError(f"scenario {scenario['name']} transcript evidence is invalid")
        event = transcript_by_id.get(event_ids[0])
        if (
            event is None
            or event.get("scenario") != scenario["name"]
            or event.get("claims") != claims
            or event.get("observations") != observations
        ):
            raise AttestationError(f"scenario {scenario['name']} is not causally bound to its transcript event")
        if parse_utc_timestamp(event.get("observed_at"), f"scenario {scenario['name']} transcript time") != completed:
            raise AttestationError(f"scenario {scenario['name']} completion is not bound to its transcript time")
        names.append(scenario["name"])
    if tuple(names) != REQUIRED_SCENARIOS:
        raise AttestationError("receipt scenario set or order does not match the canonical 12-scenario qualification")


def verify_runtime_claims(receipt: dict[str, Any]) -> None:
    if receipt.get("collector_service_user") != "pulse-agent":
        raise AttestationError("receipt collector service user is not pulse-agent")
    if not isinstance(receipt.get("collector_process_uid"), int) or receipt["collector_process_uid"] <= 0:
        raise AttestationError("receipt collector process UID is not non-root")
    if receipt.get("collector_authority") != "monitoring-only":
        raise AttestationError("receipt collector authority is not monitoring-only")
    if receipt.get("disposable_vm_guard_sha256") != DISPOSABLE_VM_GUARD_SHA256:
        raise AttestationError("receipt disposable-VM guard claim is missing or invalid")
    if receipt.get("action_receipt_kind") != "pulse.host_storage_cleanup_result":
        raise AttestationError("receipt action result kind is not the qualified typed mutation")
    report_count = receipt.get("report_count")
    if not isinstance(report_count, int) or report_count < 2:
        raise AttestationError("receipt report count does not prove continuity")
    first_report = parse_utc_timestamp(receipt.get("first_report_at"), "receipt first_report_at")
    last_report = parse_utc_timestamp(receipt.get("last_report_at"), "receipt last_report_at")
    if last_report <= first_report:
        raise AttestationError("receipt report chronology does not prove continuity")
    run_started = parse_utc_timestamp(receipt.get("started_at"), "receipt started_at")
    run_completed = parse_utc_timestamp(receipt.get("completed_at"), "receipt completed_at")
    if first_report < run_started or last_report > run_completed:
        raise AttestationError("receipt reports fall outside the qualification run chronology")


def verify_source_hashes(
    checkout: Path, commit: str, receipt: dict[str, Any]
) -> tuple[dict[str, str], dict[str, Any]]:
    manifest, manifest_raw, expected_paths = load_source_manifest(checkout, commit)
    binding = receipt.get("source_manifest")
    if not isinstance(binding, dict):
        raise AttestationError("receipt source_manifest binding must be an object")
    expected_binding = {
        "schema_version": SOURCE_MANIFEST_SCHEMA_VERSION,
        "manifest_id": SOURCE_MANIFEST_ID,
        "path": SOURCE_MANIFEST_PATH,
        "sha256": sha256_bytes(manifest_raw),
        "target_os": "linux",
        "target_arch": receipt.get("architecture"),
    }
    if binding != expected_binding:
        raise AttestationError("receipt source manifest binding does not match the qualified commit")
    source_hashes = receipt.get("source_hashes")
    if not isinstance(source_hashes, dict) or not source_hashes:
        raise AttestationError("receipt source_hashes must be a non-empty object")
    actual_paths = set(source_hashes)
    if actual_paths != expected_paths:
        missing = sorted(expected_paths.difference(actual_paths))
        unexpected = sorted(actual_paths.difference(expected_paths))
        raise AttestationError(
            f"receipt source_hashes do not exactly match the governed boundary manifest; missing={missing} unexpected={unexpected}"
        )
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
    return verified, {
        "schema_version": manifest["schema_version"],
        "manifest_id": manifest["manifest_id"],
        "path": SOURCE_MANIFEST_PATH,
        "sha256": sha256_bytes(manifest_raw),
        "target_os": manifest["target_os"],
        "target_arch": receipt.get("architecture"),
        "source_count": len(verified),
    }


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


def verify_artifact_build_identity(
    receipt: dict[str, Any], artifacts: dict[str, Path], qualified_commit: str
) -> dict[str, dict[str, str]]:
    versions = receipt.get("artifact_versions")
    if not isinstance(versions, dict) or set(versions) != {"collector_v1", "collector_v2"}:
        raise AttestationError("receipt artifact_versions must identify both collector artifacts")
    if any(not isinstance(value, str) or not value.strip() for value in versions.values()):
        raise AttestationError("receipt collector artifact version is invalid")
    if versions["collector_v1"] == versions["collector_v2"]:
        raise AttestationError("collector qualification artifacts must have distinct versions")

    verified: dict[str, dict[str, str]] = {}
    for name, artifact in artifacts.items():
        try:
            result = subprocess.run(
                ["go", "version", "-m", str(artifact)],
                check=True,
                capture_output=True,
                text=True,
            )
        except (OSError, subprocess.CalledProcessError) as exc:
            raise AttestationError(f"unable to inspect Go build identity for {name}") from exc
        package = ""
        settings: dict[str, str] = {}
        for line in result.stdout.splitlines():
            fields = line.strip().split("\t")
            if len(fields) >= 2 and fields[0] == "path":
                package = fields[1]
            elif len(fields) >= 2 and fields[0] == "build" and "=" in fields[1]:
                key, value = fields[1].split("=", 1)
                settings[key] = value
        if package != EXPECTED_ARTIFACT_PACKAGES[name]:
            raise AttestationError(f"artifact {name} is not the expected Go command package")
        if settings.get("vcs") != "git" or settings.get("vcs.revision") != qualified_commit:
            raise AttestationError(f"artifact {name} is not build-stamped from the qualified commit")
        if settings.get("vcs.modified") != "false":
            raise AttestationError(f"artifact {name} was built from a modified checkout")
        verified[name] = {
            "package": package,
            "vcs_revision": settings["vcs.revision"],
            "vcs_modified": settings["vcs.modified"],
        }
    return verified


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
    record_path = canonical_repo_path(receipt_record_path, "receipt record path")
    if receipt.get("record_path") != record_path:
        raise AttestationError("receipt record path is not bound inside the supplied receipt")
    transcript_events, transcript_bytes = load_and_verify_transcript(transcript_path, receipt)
    verify_scenarios(receipt, transcript_events)
    verify_runtime_claims(receipt)
    source_hashes, source_manifest = verify_source_hashes(checkout, qualified_commit, receipt)
    attestation_tool_hash = sha256_file(Path(__file__))
    if source_hashes["scripts/release_control/secure_runtime_attestation.py"] != attestation_tool_hash:
        raise AttestationError("executing attestation tool does not match the qualified commit")
    artifact_hashes = verify_artifacts(receipt, artifacts)
    artifact_build_identity = verify_artifact_build_identity(receipt, artifacts, qualified_commit)
    if (
        not isinstance(elapsed_seconds, (float, int))
        or not math.isfinite(elapsed_seconds)
        or elapsed_seconds <= 0
    ):
        raise AttestationError("elapsed seconds must be positive")

    classification = "release-candidate-artifact-bound-self-attested-systemd" if release_candidate_ref else "committed-main-artifact-bound-self-attested-systemd"
    return {
        "schema_version": ATTESTATION_SCHEMA_VERSION,
        "attestation_tool": "scripts/release_control/secure_runtime_attestation.py",
        "attestation_tool_sha256": attestation_tool_hash,
        "proof_classification": classification,
        "qualified_commit": qualified_commit,
        "qualified_ref_at_run": release_candidate_ref or qualified_commit,
        "main_ref_verified": main_ref,
        "main_ref_commit_at_attestation": main_commit,
        "qualified_commit_reachable_from_main": True,
        "build_checkout": "detached-worktree",
        "build_checkout_clean_except_lab_artifacts": True,
        "disposable_vm_guard_receipt_claim_validated": True,
        "execution_receipt_authentication": "none-secret-free-self-attestation",
        "receipt": {
            "record_path": record_path,
            "sha256": sha256_bytes(receipt_bytes),
            "path_bound_inside_receipt": True,
        },
        "transcript": {
            "record_path": receipt["transcript"]["record_path"],
            "sha256": sha256_bytes(transcript_bytes),
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
    parser.add_argument("--transcript", type=Path, required=True)
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
            transcript_path=args.transcript,
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
