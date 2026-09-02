#!/usr/bin/env python3
"""Validate the secret-free opt-in rootful runtime qualification receipt.

This validator qualifies only local, artifact-bound Docker and Podman
typed-helper summary collection. It does not qualify a published release,
authorize the safe profile as the default, or substitute for external review.
"""

from __future__ import annotations

import argparse
import contextlib
import hashlib
import json
import re
import stat
import sys
from datetime import datetime
from pathlib import Path
from typing import Any

import secure_runtime_rootless_attestation_v1 as hardened


RECEIPT_SCHEMA_VERSION = 1
ATTESTATION_SCHEMA_VERSION = 1
RECEIPT_KIND = "pulse-secure-runtime-rootful-qualification"
ATTESTATION_KIND = "pulse-secure-runtime-rootful-receipt-attestation"
CLASSIFICATION = "local-opt-in-rootful-runtime-artifact-bound-self-attestation"
SOURCE_MANIFEST_SCHEMA_VERSION = 1
SOURCE_MANIFEST_ID = "secure-runtime-rootful-v1"
SOURCE_MANIFEST_PATH = "scripts/release_control/secure_runtime_rootful_source_manifest_v1.json"
MAX_RECEIPT_BYTES = hardened.MAX_RECEIPT_BYTES

REQUIRED_RUNTIMES = ("docker", "podman")
REQUIRED_SCENARIOS = (
    "fresh_install",
    "legacy_migration",
    "collector_restart",
    "helper_restart",
    "helper_loss",
    "helper_recovery",
    "operation_bounds",
    "update_preservation",
    "authority_isolation",
    "cleanup",
)

ValidationError = hardened.ValidationError
fail = hardened.fail
require = hardened.require
require_object = hardened.require_object
require_bool = hardened.require_bool
require_int = hardened.require_int
require_text = hardened.require_text
require_digest = hardened.require_digest
parse_timestamp = hardened.parse_timestamp
reject_sensitive_evidence = hardened.reject_sensitive_evidence
read_immutable_receipt = hardened.read_immutable_receipt
immutable_artifact_snapshot = hardened.immutable_artifact_snapshot
inspect_go_build_identity = hardened.inspect_go_build_identity
verify_source_commit = hardened.verify_source_commit
IDENTITY_RE = hardened.IDENTITY_RE
VERSION_RE = hardened.VERSION_RE
GO_VERSION_RE = hardened.GO_VERSION_RE
COMMIT_RE = hardened.COMMIT_RE
BASE_IMAGE_RE = re.compile(r"^ubuntu@sha256:[0-9a-f]{64}$")


SUMMARY_KEYS = {
    "collector_pid",
    "helper_pid",
    "collection_mode",
    "inventory_complete",
    "inventory_count",
    "semantic_sha256",
    "full_fields_present",
    "stats_present",
    "secondary_structure_sha256",
    "container_updates_enabled",
    "container_actions_enabled",
    "direct_socket_access",
    "daemon_id",
    "daemon_rootless",
}


def expected_socket_path(runtime: str) -> str:
    return "/var/run/docker.sock" if runtime == "docker" else "/run/podman/podman.sock"


def validate_socket_mode(value: Any, label: str) -> str:
    mode = require_text(value, label, pattern=re.compile(r"^0[0-7]{3}$"), maximum=4)
    numeric = int(mode, 8)
    require(numeric & 0o600 == 0o600, f"{label} must grant owner read and write")
    require(numeric & 0o006 == 0, f"{label} must not grant world read or write")
    return mode


def validate_summary(
    value: Any,
    label: str,
    runtime: dict[str, Any],
    *,
    extra_keys: set[str] | None = None,
    expected_collector_pid: int | None = None,
    expected_helper_pid: int | None = None,
) -> dict[str, Any]:
    item = require_object(value, label, SUMMARY_KEYS | (extra_keys or set()))
    collector_pid = require_int(item["collector_pid"], f"{label}.collector_pid", minimum=2)
    helper_pid = require_int(item["helper_pid"], f"{label}.helper_pid", minimum=2)
    if expected_collector_pid is not None:
        require(collector_pid == expected_collector_pid, f"{label} unexpectedly changed collector PID")
    if expected_helper_pid is not None:
        require(helper_pid == expected_helper_pid, f"{label} unexpectedly changed helper PID")
    require(item["collection_mode"] == "typed-helper-summary", f"{label}.collection_mode differs")
    require_bool(item["inventory_complete"], True, f"{label}.inventory_complete")
    require_int(item["inventory_count"], f"{label}.inventory_count", minimum=1)
    require_digest(item["semantic_sha256"], f"{label}.semantic_sha256")
    require_bool(item["full_fields_present"], False, f"{label}.full_fields_present")
    require_bool(item["stats_present"], False, f"{label}.stats_present")
    require(item["secondary_structure_sha256"] == "", f"{label}.secondary_structure_sha256 must be empty")
    for name in ("container_updates_enabled", "container_actions_enabled", "direct_socket_access"):
        require_bool(item[name], False, f"{label}.{name}")
    require(item["daemon_id"] == runtime["daemon_id"], f"{label}.daemon_id differs")
    require_bool(item["daemon_rootless"], False, f"{label}.daemon_rootless")
    return item


def require_summary_continuity(baseline: dict[str, Any], current: dict[str, Any], label: str) -> None:
    require(current["inventory_count"] == baseline["inventory_count"], f"{label} inventory count differs")
    require(current["semantic_sha256"] == baseline["semantic_sha256"], f"{label} semantic digest differs")


def validate_scenario(
    value: Any,
    expected_name: str,
    label: str,
    receipt_start: datetime,
    receipt_end: datetime,
) -> tuple[dict[str, Any], datetime, datetime]:
    item = require_object(
        value,
        label,
        {"name", "result", "started_at", "completed_at", "report_stream_id", "report_sequence", "evidence"},
    )
    require(item["name"] == expected_name, f"{label}.name must be {expected_name}")
    require(item["result"] == "passed", f"{label}.result must be passed")
    started = parse_timestamp(item["started_at"], f"{label}.started_at")
    completed = parse_timestamp(item["completed_at"], f"{label}.completed_at")
    require(receipt_start <= started < completed <= receipt_end, f"{label} chronology is outside the receipt window")
    if expected_name in {"authority_isolation", "cleanup"}:
        require(item["report_stream_id"] is None, f"{label}.report_stream_id must be null")
        require(item["report_sequence"] is None, f"{label}.report_sequence must be null")
    else:
        require_text(item["report_stream_id"], f"{label}.report_stream_id", pattern=IDENTITY_RE)
        require_int(item["report_sequence"], f"{label}.report_sequence", minimum=1)
    return item, started, completed


def validate_run(run_value: Any, expected_runtime: str, index: int, receipt_start: datetime, receipt_end: datetime) -> dict[str, Any]:
    label = f"runs[{index}]"
    run = require_object(run_value, label, {"host", "runtime", "scenarios"})
    host = require_object(run["host"], f"{label}.host", {"machine_id", "architecture", "kernel", "systemd_version"})
    require_text(host["machine_id"], f"{label}.host.machine_id", pattern=IDENTITY_RE)
    require(host["architecture"] in {"amd64", "arm64"}, f"{label}.host.architecture is unsupported")
    require_text(host["kernel"], f"{label}.host.kernel", maximum=128)
    require_text(host["systemd_version"], f"{label}.host.systemd_version", maximum=128)

    runtime = require_object(
        run["runtime"],
        f"{label}.runtime",
        {"runtime", "runtime_version", "daemon_id", "daemon_rootless", "socket_path", "socket_uid", "socket_gid", "socket_mode", "socket_type", "socket_symlink"},
    )
    require(runtime["runtime"] == expected_runtime, f"{label}.runtime.runtime must be {expected_runtime}")
    require_text(runtime["runtime_version"], f"{label}.runtime.runtime_version", pattern=VERSION_RE, maximum=128)
    require_text(runtime["daemon_id"], f"{label}.runtime.daemon_id", pattern=IDENTITY_RE)
    require_bool(runtime["daemon_rootless"], False, f"{label}.runtime.daemon_rootless")
    require(runtime["socket_path"] == expected_socket_path(expected_runtime), f"{label}.runtime.socket_path differs")
    require_int(runtime["socket_uid"], f"{label}.runtime.socket_uid")
    require(runtime["socket_uid"] == 0, f"{label}.runtime.socket_uid must be root")
    require_int(runtime["socket_gid"], f"{label}.runtime.socket_gid")
    validate_socket_mode(runtime["socket_mode"], f"{label}.runtime.socket_mode")
    require(runtime["socket_type"] == "unix", f"{label}.runtime.socket_type must be unix")
    require_bool(runtime["socket_symlink"], False, f"{label}.runtime.socket_symlink")

    scenarios = run["scenarios"]
    require(isinstance(scenarios, list) and len(scenarios) == len(REQUIRED_SCENARIOS), f"{label}.scenarios must contain exactly ten scenarios")
    parsed: dict[str, dict[str, Any]] = {}
    previous_completed: datetime | None = None
    for scenario_index, name in enumerate(REQUIRED_SCENARIOS):
        scenario, started, completed = validate_scenario(
            scenarios[scenario_index], name, f"{label}.scenarios[{scenario_index}]", receipt_start, receipt_end
        )
        if previous_completed is not None:
            require(previous_completed <= started, f"{label}.scenarios are not chronological")
        previous_completed = completed
        parsed[name] = scenario

    fresh = validate_summary(parsed["fresh_install"]["evidence"], f"{label}.fresh_install", runtime)

    migration = validate_summary(
        parsed["legacy_migration"]["evidence"],
        f"{label}.legacy_migration",
        runtime,
        extra_keys={"legacy_profile", "target_profile", "authority_reduced", "legacy_collector_pid"},
    )
    require(migration["legacy_profile"] == "root-command-capable", f"{label}.legacy_migration legacy profile differs")
    require(migration["target_profile"] == "typed-helper-monitoring-only", f"{label}.legacy_migration target profile differs")
    require_bool(migration["authority_reduced"], True, f"{label}.legacy_migration.authority_reduced")
    legacy_pid = require_int(migration["legacy_collector_pid"], f"{label}.legacy_migration.legacy_collector_pid", minimum=2)
    require(migration["collector_pid"] != legacy_pid, f"{label}.legacy_migration retained legacy collector PID")
    require_summary_continuity(fresh, migration, f"{label}.legacy_migration")

    collector_restart = validate_summary(
        parsed["collector_restart"]["evidence"],
        f"{label}.collector_restart",
        runtime,
        extra_keys={"previous_collector_pid", "previous_report_stream_id"},
    )
    require(collector_restart["previous_collector_pid"] == migration["collector_pid"], f"{label}.collector_restart predecessor PID differs")
    require(collector_restart["collector_pid"] != migration["collector_pid"], f"{label}.collector_restart did not change PID")
    require(collector_restart["previous_report_stream_id"] == parsed["legacy_migration"]["report_stream_id"], f"{label}.collector_restart predecessor stream differs")
    require_summary_continuity(fresh, collector_restart, f"{label}.collector_restart")

    install_streams = [parsed[name]["report_stream_id"] for name in ("fresh_install", "legacy_migration", "collector_restart")]
    require(len(set(install_streams)) == 3, f"{label} install and collector restart report streams must be distinct")

    helper_restart = validate_summary(
        parsed["helper_restart"]["evidence"],
        f"{label}.helper_restart",
        runtime,
        extra_keys={"previous_helper_pid", "previous_helper_invocation_id", "helper_invocation_id"},
        expected_collector_pid=collector_restart["collector_pid"],
    )
    require(helper_restart["previous_helper_pid"] == collector_restart["helper_pid"], f"{label}.helper_restart predecessor PID differs")
    require(helper_restart["helper_pid"] != collector_restart["helper_pid"], f"{label}.helper_restart did not change helper PID")
    previous_invocation = require_text(helper_restart["previous_helper_invocation_id"], f"{label}.helper_restart.previous_helper_invocation_id", pattern=IDENTITY_RE)
    invocation = require_text(helper_restart["helper_invocation_id"], f"{label}.helper_restart.helper_invocation_id", pattern=IDENTITY_RE)
    require(previous_invocation != invocation, f"{label}.helper_restart did not change InvocationID")
    require_summary_continuity(fresh, helper_restart, f"{label}.helper_restart")

    steady_stream = parsed["collector_restart"]["report_stream_id"]
    require(parsed["helper_restart"]["report_stream_id"] == steady_stream, f"{label}.helper_restart changed report stream")
    require(parsed["helper_restart"]["report_sequence"] > parsed["collector_restart"]["report_sequence"], f"{label}.helper_restart report did not advance")

    loss_label = f"{label}.helper_loss"
    loss = require_object(
        parsed["helper_loss"]["evidence"],
        loss_label,
        {"collector_pid", "previous_helper_pid", "collection_mode", "helper_available", "status_only", "inventory_complete", "inventory_present", "authoritative_inventory_replacement", "previous_authoritative_inventory_count", "previous_authoritative_semantic_sha256", "operation_status", "operation", "container_updates_enabled", "container_actions_enabled", "direct_socket_access"},
    )
    require(loss["collector_pid"] == helper_restart["collector_pid"], f"{loss_label} changed collector PID")
    require(loss["previous_helper_pid"] == helper_restart["helper_pid"], f"{loss_label} previous helper PID differs")
    require(loss["collection_mode"] == "typed-helper-unavailable-status-only", f"{loss_label}.collection_mode differs")
    for name, expected in (("helper_available", False), ("status_only", True), ("inventory_complete", False), ("inventory_present", False), ("authoritative_inventory_replacement", False)):
        require_bool(loss[name], expected, f"{loss_label}.{name}")
    require(loss["previous_authoritative_inventory_count"] == fresh["inventory_count"], f"{loss_label} prior count differs")
    require(loss["previous_authoritative_semantic_sha256"] == fresh["semantic_sha256"], f"{loss_label} prior digest differs")
    require(loss["operation_status"] == "degraded", f"{loss_label}.operation_status differs")
    require(loss["operation"] == "container.inventory", f"{loss_label}.operation differs")
    for name in ("container_updates_enabled", "container_actions_enabled", "direct_socket_access"):
        require_bool(loss[name], False, f"{loss_label}.{name}")
    require(parsed["helper_loss"]["report_stream_id"] == steady_stream, f"{loss_label} changed report stream")
    require(parsed["helper_loss"]["report_sequence"] > parsed["helper_restart"]["report_sequence"], f"{loss_label} report did not advance")

    recovery = validate_summary(
        parsed["helper_recovery"]["evidence"],
        f"{label}.helper_recovery",
        runtime,
        extra_keys={"previous_helper_pid", "previous_status_report_sequence"},
        expected_collector_pid=helper_restart["collector_pid"],
    )
    require(recovery["previous_helper_pid"] == helper_restart["helper_pid"], f"{label}.helper_recovery previous helper PID differs")
    require(recovery["helper_pid"] != helper_restart["helper_pid"], f"{label}.helper_recovery did not replace helper PID")
    require(recovery["previous_status_report_sequence"] == parsed["helper_loss"]["report_sequence"], f"{label}.helper_recovery status sequence differs")
    require(parsed["helper_recovery"]["report_stream_id"] == steady_stream, f"{label}.helper_recovery changed report stream")
    require(parsed["helper_recovery"]["report_sequence"] > parsed["helper_loss"]["report_sequence"], f"{label}.helper_recovery report did not advance")
    require_summary_continuity(fresh, recovery, f"{label}.helper_recovery")

    bounds_label = f"{label}.operation_bounds"
    bounds = require_object(
        parsed["operation_bounds"]["evidence"],
        bounds_label,
        {"collector_pid", "helper_pid", "operation", "failure_class", "deadline_ms", "elapsed_ms", "bounded_failure_observed", "status_only_report_sequence", "recovery_report_sequence", "collection_mode", "inventory_complete", "previous_authoritative_inventory_count", "previous_authoritative_semantic_sha256", "recovery_inventory_count", "recovery_semantic_sha256", "full_fields_present", "stats_present", "secondary_structure_sha256", "authoritative_empty_replacement", "collector_alive", "helper_alive", "container_updates_enabled", "container_actions_enabled", "direct_socket_access"},
    )
    require(bounds["collector_pid"] == recovery["collector_pid"], f"{bounds_label} changed collector PID")
    require(bounds["helper_pid"] == recovery["helper_pid"], f"{bounds_label} changed helper PID")
    require(bounds["operation"] == "container.inventory", f"{bounds_label}.operation differs")
    require(bounds["failure_class"] == "bounded-timeout", f"{bounds_label}.failure_class differs")
    deadline = require_int(bounds["deadline_ms"], f"{bounds_label}.deadline_ms", minimum=1)
    elapsed = require_int(bounds["elapsed_ms"], f"{bounds_label}.elapsed_ms", minimum=1)
    require(deadline <= 30_000, f"{bounds_label}.deadline_ms exceeds the helper operation ceiling")
    require(elapsed <= deadline + 1000, f"{bounds_label} exceeded bounded deadline allowance")
    require_bool(bounds["bounded_failure_observed"], True, f"{bounds_label}.bounded_failure_observed")
    status_sequence = require_int(bounds["status_only_report_sequence"], f"{bounds_label}.status_only_report_sequence", minimum=1)
    recovery_sequence = require_int(bounds["recovery_report_sequence"], f"{bounds_label}.recovery_report_sequence", minimum=1)
    require(parsed["operation_bounds"]["report_sequence"] == recovery_sequence, f"{bounds_label} wrapper sequence differs")
    require(parsed["operation_bounds"]["report_stream_id"] == steady_stream, f"{bounds_label} changed report stream")
    require(parsed["helper_recovery"]["report_sequence"] < status_sequence < recovery_sequence, f"{bounds_label} report sequence is not causal")
    require(bounds["collection_mode"] == "typed-helper-summary", f"{bounds_label}.collection_mode differs")
    require_bool(bounds["inventory_complete"], True, f"{bounds_label}.inventory_complete")
    require(bounds["previous_authoritative_inventory_count"] == fresh["inventory_count"], f"{bounds_label} prior count differs")
    require(bounds["previous_authoritative_semantic_sha256"] == fresh["semantic_sha256"], f"{bounds_label} prior digest differs")
    require(bounds["recovery_inventory_count"] == fresh["inventory_count"], f"{bounds_label} recovery count differs")
    require(bounds["recovery_semantic_sha256"] == fresh["semantic_sha256"], f"{bounds_label} recovery digest differs")
    require_bool(bounds["full_fields_present"], False, f"{bounds_label}.full_fields_present")
    require_bool(bounds["stats_present"], False, f"{bounds_label}.stats_present")
    require(bounds["secondary_structure_sha256"] == "", f"{bounds_label}.secondary_structure_sha256 must be empty")
    require_bool(bounds["authoritative_empty_replacement"], False, f"{bounds_label}.authoritative_empty_replacement")
    for name in ("collector_alive", "helper_alive"):
        require_bool(bounds[name], True, f"{bounds_label}.{name}")
    for name in ("container_updates_enabled", "container_actions_enabled", "direct_socket_access"):
        require_bool(bounds[name], False, f"{bounds_label}.{name}")

    update = validate_summary(
        parsed["update_preservation"]["evidence"],
        f"{label}.update_preservation",
        runtime,
        extra_keys={"previous_collector_pid", "previous_helper_pid", "previous_report_stream_id", "update_applied", "collector_binary_sha256", "helper_binary_sha256"},
    )
    require(update["previous_collector_pid"] == recovery["collector_pid"], f"{label}.update_preservation predecessor PID differs")
    require(update["collector_pid"] != recovery["collector_pid"], f"{label}.update_preservation did not replace collector PID")
    require(update["previous_helper_pid"] == recovery["helper_pid"], f"{label}.update_preservation predecessor helper PID differs")
    require(update["helper_pid"] == recovery["helper_pid"], f"{label}.update_preservation unexpectedly replaced helper PID")
    require(update["previous_report_stream_id"] == steady_stream, f"{label}.update_preservation predecessor stream differs")
    require(parsed["update_preservation"]["report_stream_id"] != steady_stream, f"{label}.update_preservation did not create a new stream")
    require_bool(update["update_applied"], True, f"{label}.update_preservation.update_applied")
    require_digest(update["collector_binary_sha256"], f"{label}.update_preservation.collector_binary_sha256")
    require_digest(update["helper_binary_sha256"], f"{label}.update_preservation.helper_binary_sha256")
    require_summary_continuity(fresh, update, f"{label}.update_preservation")

    authority_label = f"{label}.authority_isolation"
    authority = require_object(
        parsed["authority_isolation"]["evidence"],
        authority_label,
        {"collector_pid", "collector_uid", "effective_uid", "effective_root", "safe_profile_enabled", "commands_enabled", "privileged_helper_enabled", "reduction_request_observed", "collector_command_transport_present", "collector_command_session_present", "container_actions_enabled", "container_updates_enabled", "rootful_socket_access", "direct_socket_access", "helper_network_access"},
    )
    require(authority["collector_pid"] == update["collector_pid"], f"{authority_label} collector PID differs")
    uid = require_int(authority["collector_uid"], f"{authority_label}.collector_uid", minimum=1)
    require(authority["effective_uid"] == uid, f"{authority_label}.effective_uid differs")
    require_bool(authority["effective_root"], False, f"{authority_label}.effective_root")
    require_bool(authority["safe_profile_enabled"], True, f"{authority_label}.safe_profile_enabled")
    require_bool(authority["privileged_helper_enabled"], True, f"{authority_label}.privileged_helper_enabled")
    require_bool(authority["reduction_request_observed"], True, f"{authority_label}.reduction_request_observed")
    for name in ("commands_enabled", "collector_command_transport_present", "collector_command_session_present", "container_actions_enabled", "container_updates_enabled", "rootful_socket_access", "direct_socket_access", "helper_network_access"):
        require_bool(authority[name], False, f"{authority_label}.{name}")

    cleanup = require_object(
        parsed["cleanup"]["evidence"],
        f"{label}.cleanup",
        {"collector_stopped", "helper_stopped", "runtime_stopped", "socket_absent", "fixtures_removed", "state_clean"},
    )
    for name in cleanup:
        require_bool(cleanup[name], True, f"{label}.cleanup.{name}")
    return run


def validate_receipt(receipt: Any) -> dict[str, Any]:
    reject_sensitive_evidence(receipt)
    root = require_object(receipt, "receipt", {"schema_version", "kind", "result", "source_commit", "base_image", "started_at", "completed_at", "artifacts", "source_hashes", "runs"})
    require(type(root["schema_version"]) is int and root["schema_version"] == RECEIPT_SCHEMA_VERSION, "receipt.schema_version must be 1")
    require(root["kind"] == RECEIPT_KIND, f"receipt.kind must be {RECEIPT_KIND}")
    require(root["result"] == "passed", "receipt.result must be passed")
    source_commit = require_text(root["source_commit"], "receipt.source_commit", pattern=COMMIT_RE, maximum=40)
    require_text(root["base_image"], "receipt.base_image", pattern=BASE_IMAGE_RE, maximum=78)
    started = parse_timestamp(root["started_at"], "receipt.started_at")
    completed = parse_timestamp(root["completed_at"], "receipt.completed_at")
    require(started < completed, "receipt chronology is invalid")

    artifacts = require_object(root["artifacts"], "receipt.artifacts", {"qualification_test", "collector", "helper", "installer"})
    expected = {
        "qualification_test": ("dockeragent.test", "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test"),
        "collector": ("pulse-agent", "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent"),
        "helper": ("pulse-agent-helper", "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper"),
    }
    for name, (basename, package) in expected.items():
        artifact = require_object(artifacts[name], f"receipt.artifacts.{name}", {"path_basename", "sha256", "package", "go_version", "vcs_revision", "vcs_modified"})
        require(artifact["path_basename"] == basename, f"receipt.artifacts.{name}.path_basename differs")
        require_digest(artifact["sha256"], f"receipt.artifacts.{name}.sha256")
        require(artifact["package"] == package, f"receipt.artifacts.{name}.package differs")
        require_text(artifact["go_version"], f"receipt.artifacts.{name}.go_version", pattern=GO_VERSION_RE, maximum=32)
        require(artifact["vcs_revision"] == source_commit, f"receipt.artifacts.{name}.vcs_revision differs")
        require_bool(artifact["vcs_modified"], False, f"receipt.artifacts.{name}.vcs_modified")
    installer = require_object(artifacts["installer"], "receipt.artifacts.installer", {"path_basename", "sha256"})
    require(installer["path_basename"] == "install.sh", "receipt.artifacts.installer.path_basename differs")
    require_digest(installer["sha256"], "receipt.artifacts.installer.sha256")

    source_hashes = root["source_hashes"]
    require(isinstance(source_hashes, dict) and source_hashes, "receipt.source_hashes must be a non-empty object")
    for source_path, digest in source_hashes.items():
        require_text(source_path, "receipt.source_hashes path", maximum=512)
        candidate = Path(source_path)
        require(source_path == candidate.as_posix() and not candidate.is_absolute() and ".." not in candidate.parts, "receipt.source_hashes contains a non-canonical path")
        require_digest(digest, f"receipt.source_hashes[{source_path}]")
    require(source_hashes.get("scripts/install.sh") == installer["sha256"], "installer digest must equal governed scripts/install.sh")

    runs = root["runs"]
    require(isinstance(runs, list) and len(runs) == 2, "receipt.runs must contain exactly two runtime runs")
    validated = [validate_run(runs[i], runtime, i, started, completed) for i, runtime in enumerate(REQUIRED_RUNTIMES)]
    require(validated[0]["host"]["machine_id"] != validated[1]["host"]["machine_id"], "Docker and Podman runs must use distinct hosts")
    require(validated[0]["runtime"]["daemon_id"] != validated[1]["runtime"]["daemon_id"], "Docker and Podman daemon identities must differ")
    docker_stream = validated[0]["scenarios"][2]["report_stream_id"]
    podman_stream = validated[1]["scenarios"][2]["report_stream_id"]
    require(docker_stream != podman_stream, "Docker and Podman report streams must differ")
    for index, run in enumerate(validated):
        update = run["scenarios"][REQUIRED_SCENARIOS.index("update_preservation")]["evidence"]
        require(update["collector_binary_sha256"] == artifacts["collector"]["sha256"], f"runs[{index}].update_preservation collector artifact digest differs")
        require(update["helper_binary_sha256"] == artifacts["helper"]["sha256"], f"runs[{index}].update_preservation helper artifact digest differs")
    return root


def _reject_duplicate_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            fail(f"receipt contains duplicate JSON key {key!r}")
        result[key] = value
    return result


def parse_receipt_bytes(data: bytes) -> dict[str, Any]:
    require(len(data) <= MAX_RECEIPT_BYTES, "receipt exceeds the maximum size")
    try:
        parsed = json.loads(data.decode("utf-8"), object_pairs_hook=_reject_duplicate_keys, parse_constant=lambda value: fail(f"receipt contains invalid number {value}"))
    except UnicodeDecodeError as exc:
        raise ValidationError("receipt is not UTF-8") from exc
    except json.JSONDecodeError as exc:
        raise ValidationError(f"receipt is not valid JSON: {exc}") from exc
    return validate_receipt(parsed)


def _canonical_relative_path(value: Any, label: str) -> str:
    text = require_text(value, label, maximum=512)
    candidate = Path(text)
    require(not candidate.is_absolute() and candidate.as_posix() == text and text not in {"", "."} and ".." not in candidate.parts, f"{label} is not a canonical repository-relative path")
    return text


def load_source_manifest(checkout: Path, manifest_path: Path) -> tuple[bytes, dict[str, str]]:
    manifest_bytes = read_immutable_receipt(manifest_path)
    try:
        manifest = json.loads(manifest_bytes.decode("utf-8"), object_pairs_hook=_reject_duplicate_keys)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValidationError(f"source manifest is invalid JSON: {exc}") from exc
    manifest = require_object(manifest, "source manifest", {"schema_version", "manifest_id", "target_os", "description", "exact_paths", "recursive_roots", "include_suffixes", "exclude_suffixes"})
    require(type(manifest["schema_version"]) is int and manifest["schema_version"] == SOURCE_MANIFEST_SCHEMA_VERSION, "source manifest schema differs")
    require(manifest["manifest_id"] == SOURCE_MANIFEST_ID, "source manifest ID differs")
    require(manifest["target_os"] == "linux", "source manifest target_os must be linux")
    require_text(manifest["description"], "source manifest description", maximum=1024)
    for field in ("exact_paths", "recursive_roots", "include_suffixes", "exclude_suffixes"):
        require(isinstance(manifest[field], list) and all(isinstance(item, str) for item in manifest[field]), f"source manifest {field} must be a text array")
        require(len(manifest[field]) == len(set(manifest[field])), f"source manifest {field} contains duplicates")
    require(manifest["exact_paths"], "source manifest exact_paths must not be empty")
    include_suffixes = tuple(manifest["include_suffixes"])
    exclude_suffixes = tuple(manifest["exclude_suffixes"])
    require(include_suffixes and all(item.startswith(".") for item in include_suffixes), "source manifest include_suffixes are invalid")
    require(all(item.startswith("_") or item.startswith(".") for item in exclude_suffixes), "source manifest exclude_suffixes are invalid")
    paths = {_canonical_relative_path(item, "source manifest exact path") for item in manifest["exact_paths"]}
    for raw_root in manifest["recursive_roots"]:
        root_text = _canonical_relative_path(raw_root, "source manifest recursive root")
        root = checkout / root_text
        require(root.is_dir() and not root.is_symlink(), f"source manifest recursive root is not a regular directory: {root_text}")
        for candidate in root.rglob("*"):
            if candidate.is_dir() or candidate.name.endswith(exclude_suffixes) or not candidate.name.endswith(include_suffixes):
                continue
            paths.add(candidate.relative_to(checkout).as_posix())
    hashes: dict[str, str] = {}
    checkout_resolved = checkout.resolve(strict=True)
    for relative in sorted(paths):
        candidate = checkout / relative
        try:
            metadata = candidate.lstat()
        except OSError as exc:
            raise ValidationError(f"unable to inspect governed source {relative}: {exc}") from exc
        require(stat.S_ISREG(metadata.st_mode) and not stat.S_ISLNK(metadata.st_mode), f"governed source is not a regular file: {relative}")
        require(candidate.resolve(strict=True).is_relative_to(checkout_resolved), f"governed source escapes checkout: {relative}")
        hashes[relative] = hashlib.sha256(read_immutable_receipt(candidate)).hexdigest()
    require(hashes, "source manifest expands to no source files")
    return manifest_bytes, hashes


def create_attestation(
    receipt_path: Path,
    qualification_test_path: Path,
    collector_path: Path,
    helper_path: Path,
    installer_path: Path,
    *,
    checkout: Path | None = None,
    manifest_path: Path | None = None,
    verify_git_commit: bool = True,
) -> dict[str, Any]:
    data = read_immutable_receipt(receipt_path)
    receipt = parse_receipt_bytes(data)
    checkout = (checkout or Path(__file__).resolve().parents[2]).resolve(strict=True)
    manifest_path = manifest_path or checkout / SOURCE_MANIFEST_PATH
    manifest_bytes, source_hashes = load_source_manifest(checkout, manifest_path)
    require(receipt["source_hashes"] == source_hashes, "receipt source_hashes do not match the canonical source manifest")
    if verify_git_commit:
        verify_source_commit(checkout, receipt["source_commit"], source_hashes)
    supplied = {
        "qualification_test": (qualification_test_path, "dockeragent.test", True),
        "collector": (collector_path, "pulse-agent", True),
        "helper": (helper_path, "pulse-agent-helper", True),
        "installer": (installer_path, "install.sh", False),
    }
    bindings: dict[str, dict[str, Any]] = {}
    with contextlib.ExitStack() as stack:
        snapshots: dict[str, Path] = {}
        for name, (path, basename, executable) in supplied.items():
            snapshot, digest = stack.enter_context(immutable_artifact_snapshot(path, basename, executable=executable))
            require(digest == receipt["artifacts"][name]["sha256"], f"{name} bytes do not match receipt SHA-256")
            snapshots[name] = snapshot
            bindings[name] = {"path_basename": basename, "sha256": digest}
        for name in ("qualification_test", "collector", "helper"):
            build = inspect_go_build_identity(snapshots[name])
            claimed = receipt["artifacts"][name]
            require(build["package"] == claimed["package"], f"{name} Go package differs from receipt")
            require(build["go_version"] == claimed["go_version"], f"{name} Go version differs from receipt")
            require(build["vcs_revision"] == claimed["vcs_revision"] == receipt["source_commit"], f"{name} VCS revision differs")
            require_bool(build["vcs_modified"], False, f"{name} vcs.modified")
            bindings[name].update(build)
    validator_bytes = read_immutable_receipt(Path(__file__).absolute())
    return {
        "schema_version": ATTESTATION_SCHEMA_VERSION,
        "kind": ATTESTATION_KIND,
        "classification": CLASSIFICATION,
        "receipt_sha256": hashlib.sha256(data).hexdigest(),
        "validator_source_sha256": hashlib.sha256(validator_bytes).hexdigest(),
        "source_manifest_schema_version": SOURCE_MANIFEST_SCHEMA_VERSION,
        "source_manifest_id": SOURCE_MANIFEST_ID,
        "source_manifest_sha256": hashlib.sha256(manifest_bytes).hexdigest(),
        "source_hash_count": len(source_hashes),
        "source_commit": receipt["source_commit"],
        "qualified_base_image": receipt["base_image"],
        "source_commit_verified": verify_git_commit,
        "artifact_bindings": bindings,
        "validated_runtimes": list(REQUIRED_RUNTIMES),
        "limitations": [
            "not-published-release-provenance",
            "not-default-profile-authorization",
            "not-independent-security-review",
            "production-exact-scope-proof-is-external-prior",
        ],
    }


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("receipt", type=Path, help="schema-v1 rootful qualification receipt")
    parser.add_argument("--qualification-test", required=True, type=Path, help="exact dockeragent.test qualification binary")
    parser.add_argument("--collector", required=True, type=Path, help="exact pulse-agent binary")
    parser.add_argument("--helper", required=True, type=Path, help="exact pulse-agent-helper binary")
    parser.add_argument("--installer", required=True, type=Path, help="exact install.sh blob")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    arguments = parse_args(argv)
    try:
        attestation = create_attestation(arguments.receipt, arguments.qualification_test, arguments.collector, arguments.helper, arguments.installer)
    except ValidationError as exc:
        print(f"rootful receipt validation failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(attestation, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
