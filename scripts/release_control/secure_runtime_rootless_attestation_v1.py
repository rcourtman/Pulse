#!/usr/bin/env python3
"""Validate the secret-free opt-in rootless runtime qualification receipt.

This validator deliberately makes no claim about published release artifacts or
whether the safe collector profile is suitable as a default.  It validates one
local, artifact-bound qualification receipt for both Docker and Podman.
"""

from __future__ import annotations

import argparse
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
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, NoReturn


RECEIPT_SCHEMA_VERSION = 1
ATTESTATION_SCHEMA_VERSION = 1
RECEIPT_KIND = "pulse-secure-runtime-rootless-qualification"
ATTESTATION_KIND = "pulse-secure-runtime-rootless-receipt-attestation"
CLASSIFICATION = "local-opt-in-rootless-runtime-artifact-bound-self-attestation"
MAX_RECEIPT_BYTES = 2 * 1024 * 1024
MAX_ARTIFACT_BYTES = 256 * 1024 * 1024
SOURCE_MANIFEST_SCHEMA_VERSION = 1
SOURCE_MANIFEST_ID = "secure-runtime-rootless-v1"
SOURCE_MANIFEST_PATH = "scripts/release_control/secure_runtime_rootless_source_manifest_v1.json"
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
GO_VERSION_RE = re.compile(r"^go1\.[0-9]+(?:\.[0-9]+)?$")
IDENTITY_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:@+-]{0,255}$")
VERSION_RE = re.compile(r"^v?[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$")
TIMESTAMP_RE = re.compile(
    r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z$"
)
SENSITIVE_KEY_PARTS = frozenset(
    {
        "authorization",
        "bearer",
        "cookie",
        "credential",
        "credentials",
        "password",
        "passwd",
        "privatekey",
        "secret",
        "secrets",
        "token",
        "tokens",
    }
)
SENSITIVE_VALUE_PATTERNS = (
    re.compile(r"-----BEGIN (?:[A-Z ]+ )?PRIVATE KEY-----"),
    re.compile(r"(?i)(?:^|\s)(?:bearer|basic)\s+[A-Za-z0-9+/=_-]{8,}"),
    re.compile(r"^[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}$"),
    re.compile(r"^[a-z][a-z0-9+.-]*://[^/@\s:]+:[^/@\s]+@", re.IGNORECASE),
)

REQUIRED_RUNTIMES = ("docker", "podman")
REQUIRED_SCENARIOS = (
    "fresh_install",
    "legacy_migration",
    "collector_restart",
    "daemon_restart",
    "socket_loss_helper_fallback",
    "direct_recovery",
    "dual_socket_ambiguity_refusal",
    "exact_pin_recovery",
    "telemetry_parity",
    "authority_isolation",
    "cleanup",
)


class ValidationError(ValueError):
    """The receipt is malformed or does not prove the required contract."""


def fail(message: str) -> NoReturn:
    raise ValidationError(message)


def require(condition: bool, message: str) -> None:
    if not condition:
        fail(message)


def require_object(value: Any, label: str, keys: set[str]) -> dict[str, Any]:
    require(isinstance(value, dict), f"{label} must be an object")
    actual = set(value)
    require(actual == keys, f"{label} fields differ: missing={sorted(keys - actual)} extra={sorted(actual - keys)}")
    return value


def require_bool(value: Any, expected: bool, label: str) -> None:
    require(type(value) is bool and value is expected, f"{label} must be {str(expected).lower()}")


def require_int(value: Any, label: str, *, minimum: int = 0) -> int:
    require(type(value) is int and value >= minimum, f"{label} must be an integer >= {minimum}")
    return value


def require_text(
    value: Any,
    label: str,
    *,
    pattern: re.Pattern[str] | None = None,
    maximum: int = 256,
) -> str:
    require(isinstance(value, str) and 0 < len(value) <= maximum, f"{label} must be non-empty text <= {maximum} bytes")
    require("\x00" not in value and "\n" not in value and "\r" not in value, f"{label} contains control text")
    if pattern is not None:
        require(pattern.fullmatch(value) is not None, f"{label} has an invalid format")
    return value


def require_digest(value: Any, label: str, *, allow_empty: bool = False) -> str:
    if allow_empty and value == "":
        return ""
    return require_text(value, label, pattern=SHA256_RE, maximum=64)


def parse_timestamp(value: Any, label: str) -> datetime:
    text = require_text(value, label, pattern=TIMESTAMP_RE, maximum=40)
    try:
        parsed = datetime.fromisoformat(text[:-1] + "+00:00")
    except ValueError as exc:
        raise ValidationError(f"{label} is not a valid UTC timestamp") from exc
    require(parsed.tzinfo == timezone.utc, f"{label} must be UTC")
    return parsed


def reject_sensitive_evidence(value: Any, path: str = "receipt") -> None:
    if isinstance(value, dict):
        for key, child in value.items():
            require(isinstance(key, str), f"{path} contains a non-string key")
            if path != "receipt.source_hashes":
                normalized = re.sub(r"[^a-z0-9]", "", key.lower())
                segmented = re.sub(r"([a-z0-9])([A-Z])", r"\1_\2", key)
                parts = {part.lower() for part in re.split(r"[^A-Za-z0-9]+", segmented) if part}
                require(
                    normalized not in SENSITIVE_KEY_PARTS and not parts.intersection(SENSITIVE_KEY_PARTS),
                    f"{path}.{key} is a forbidden sensitive key",
                )
            reject_sensitive_evidence(child, f"{path}.{key}")
    elif isinstance(value, list):
        for index, child in enumerate(value):
            reject_sensitive_evidence(child, f"{path}[{index}]")
    elif isinstance(value, str):
        for pattern in SENSITIVE_VALUE_PATTERNS:
            require(pattern.search(value) is None, f"{path} contains secret-like evidence")
    elif isinstance(value, float):
        require(math.isfinite(value), f"{path} contains a non-finite number")


def expected_socket_path(runtime: str, uid: int) -> str:
    if runtime == "docker":
        return f"/run/user/{uid}/docker.sock"
    return f"/run/user/{uid}/podman/podman.sock"


DIRECT_KEYS = {
    "collector_pid",
    "service_pid",
    "collection_path",
    "inventory_complete",
    "inventory_count",
    "semantic_sha256",
    "full_fields_present",
    "stats_present",
    "secondary_structure_sha256",
    "daemon_id",
    "daemon_rootless",
    "socket_path",
    "socket_uid",
    "socket_gid",
    "socket_mode",
    "socket_type",
    "socket_symlink",
}


def validate_socket_mode(value: Any, label: str) -> str:
    mode = require_text(value, label, pattern=re.compile(r"^0[0-7]{3}$"), maximum=4)
    numeric_mode = int(mode, 8)
    require(numeric_mode & 0o600 == 0o600, f"{label} must grant owner read and write")
    require(numeric_mode & 0o006 == 0, f"{label} must not grant world read or write")
    return mode


def validate_socket_identity(value: Any, label: str, runtime: str, uid: int) -> dict[str, Any]:
    socket = require_object(
        value,
        label,
        {"runtime", "path", "uid", "gid", "mode", "type", "symlink"},
    )
    require(socket["runtime"] == runtime, f"{label}.runtime must be {runtime}")
    require(socket["path"] == expected_socket_path(runtime, uid), f"{label}.path is not the exact rootless path")
    require(socket["uid"] == uid, f"{label}.uid must match the collector")
    require_int(socket["gid"], f"{label}.gid", minimum=1)
    validate_socket_mode(socket["mode"], f"{label}.mode")
    require(socket["type"] == "unix", f"{label}.type must be unix")
    require_bool(socket["symlink"], False, f"{label}.symlink")
    return socket


def validate_direct_evidence(
    evidence: Any,
    label: str,
    runtime_result: dict[str, Any],
    *,
    expected_collector_pid: int | None = None,
    expected_daemon_id: str | None = None,
) -> dict[str, Any]:
    item = require_object(evidence, label, DIRECT_KEYS)
    collector_pid = require_int(item["collector_pid"], f"{label}.collector_pid", minimum=2)
    require_int(item["service_pid"], f"{label}.service_pid", minimum=2)
    require(item["service_pid"] == collector_pid, f"{label} collector and service PID must match")
    if expected_collector_pid is not None:
        require(collector_pid == expected_collector_pid, f"{label} unexpectedly restarted the collector")
    require(item["collection_path"] == "collector-owned-rootless-socket", f"{label}.collection_path differs")
    require_bool(item["inventory_complete"], True, f"{label}.inventory_complete")
    require_int(item["inventory_count"], f"{label}.inventory_count", minimum=1)
    for name in ("semantic_sha256", "secondary_structure_sha256"):
        require_digest(item[name], f"{label}.{name}")
    require_bool(item["full_fields_present"], True, f"{label}.full_fields_present")
    require_bool(item["stats_present"], True, f"{label}.stats_present")
    daemon_id = require_text(item["daemon_id"], f"{label}.daemon_id", pattern=IDENTITY_RE)
    if expected_daemon_id is not None:
        require(daemon_id == expected_daemon_id, f"{label} has the wrong daemon identity")
    require_bool(item["daemon_rootless"], True, f"{label}.daemon_rootless")
    require(item["socket_path"] == runtime_result["socket_path"], f"{label} used a different socket path")
    require(item["socket_uid"] == runtime_result["socket_uid"], f"{label} used a different socket owner")
    require(item["socket_gid"] == runtime_result["socket_gid"], f"{label} used a different socket group")
    require(item["socket_mode"] == runtime_result["socket_mode"], f"{label} used a different socket mode")
    require(item["socket_type"] == "unix", f"{label}.socket_type must be unix")
    require_bool(item["socket_symlink"], False, f"{label}.socket_symlink")
    return item


def validate_scenario(
    scenario: Any,
    expected_name: str,
    label: str,
    receipt_start: datetime,
    receipt_end: datetime,
) -> tuple[dict[str, Any], datetime, datetime]:
    item = require_object(
        scenario,
        label,
        {"name", "result", "started_at", "completed_at", "report_stream_id", "report_sequence", "evidence"},
    )
    require(item["name"] == expected_name, f"{label}.name must be {expected_name}")
    require(item["result"] == "passed", f"{label}.result must be passed")
    started = parse_timestamp(item["started_at"], f"{label}.started_at")
    completed = parse_timestamp(item["completed_at"], f"{label}.completed_at")
    require(receipt_start <= started < completed <= receipt_end, f"{label} chronology is outside the receipt window")
    non_reporting = expected_name in {"dual_socket_ambiguity_refusal", "authority_isolation", "cleanup"}
    if non_reporting:
        require(item["report_stream_id"] is None, f"{label}.report_stream_id must be null")
        require(item["report_sequence"] is None, f"{label}.report_sequence must be null")
    else:
        require_text(item["report_stream_id"], f"{label}.report_stream_id", pattern=IDENTITY_RE)
        require_int(item["report_sequence"], f"{label}.report_sequence", minimum=1)
    return item, started, completed


def stable_direct_parity(left: dict[str, Any], right: dict[str, Any], label: str) -> None:
    for name in ("inventory_count", "semantic_sha256", "secondary_structure_sha256"):
        require(left[name] == right[name], f"{label} did not retain {name} parity")
    require_bool(right["full_fields_present"], True, f"{label}.full_fields_present")
    require_bool(right["stats_present"], True, f"{label}.stats_present")


def validate_run(run: Any, expected_runtime: str, run_index: int, receipt_start: datetime, receipt_end: datetime) -> dict[str, Any]:
    label = f"runs[{run_index}]"
    run = require_object(run, label, {"host", "runtime", "scenarios"})
    host = require_object(run["host"], f"{label}.host", {"machine_id", "architecture", "kernel", "systemd_version"})
    require_text(host["machine_id"], f"{label}.host.machine_id", pattern=IDENTITY_RE)
    require(host["architecture"] in {"amd64", "arm64"}, f"{label}.host.architecture is unsupported")
    require_text(host["kernel"], f"{label}.host.kernel", maximum=128)
    require_text(host["systemd_version"], f"{label}.host.systemd_version", maximum=128)

    runtime = require_object(
        run["runtime"], f"{label}.runtime",
        {"runtime", "runtime_version", "daemon_id", "collector_uid", "socket_path", "socket_uid", "socket_gid", "socket_mode", "socket_type", "socket_symlink", "daemon_rootless"},
    )
    require(runtime["runtime"] == expected_runtime, f"{label}.runtime.runtime must be {expected_runtime}")
    require_text(runtime["runtime_version"], f"{label}.runtime.runtime_version", pattern=VERSION_RE, maximum=128)
    require_text(runtime["daemon_id"], f"{label}.runtime.daemon_id", pattern=IDENTITY_RE)
    uid = require_int(runtime["collector_uid"], f"{label}.runtime.collector_uid", minimum=1)
    socket_path = expected_socket_path(expected_runtime, uid)
    require(runtime["socket_path"] == socket_path, f"{label}.runtime.socket_path must be {socket_path}")
    require(runtime["socket_uid"] == uid, f"{label}.runtime.socket_uid must match collector_uid")
    require_int(runtime["socket_gid"], f"{label}.runtime.socket_gid", minimum=1)
    validate_socket_mode(runtime["socket_mode"], f"{label}.runtime.socket_mode")
    require(runtime["socket_type"] == "unix", f"{label}.runtime.socket_type must be unix")
    require_bool(runtime["socket_symlink"], False, f"{label}.runtime.socket_symlink")
    require_bool(runtime["daemon_rootless"], True, f"{label}.runtime.daemon_rootless")

    scenarios = run["scenarios"]
    require(isinstance(scenarios, list) and len(scenarios) == len(REQUIRED_SCENARIOS), f"{label}.scenarios must contain exactly eleven scenarios")
    parsed: dict[str, dict[str, Any]] = {}
    previous_completed: datetime | None = None
    for index, name in enumerate(REQUIRED_SCENARIOS):
        scenario, started, completed = validate_scenario(scenarios[index], name, f"{label}.scenarios[{index}]", receipt_start, receipt_end)
        if previous_completed is not None:
            require(previous_completed <= started, f"{label}.scenarios are not chronological")
        previous_completed = completed
        parsed[name] = scenario

    fresh = validate_direct_evidence(parsed["fresh_install"]["evidence"], f"{label}.fresh_install", runtime)
    migration_raw = require_object(
        parsed["legacy_migration"]["evidence"], f"{label}.legacy_migration",
        DIRECT_KEYS | {"legacy_profile", "target_profile", "authority_reduced", "legacy_collector_pid"},
    )
    require(migration_raw["legacy_profile"] == "root-command-capable", f"{label}.legacy_migration legacy profile differs")
    require(migration_raw["target_profile"] == "typed-helper-monitoring-only", f"{label}.legacy_migration target profile differs")
    require_bool(migration_raw["authority_reduced"], True, f"{label}.legacy_migration.authority_reduced")
    legacy_collector_pid = require_int(migration_raw["legacy_collector_pid"], f"{label}.legacy_migration.legacy_collector_pid", minimum=2)
    migration = validate_direct_evidence({key: migration_raw[key] for key in DIRECT_KEYS}, f"{label}.legacy_migration.direct", runtime)
    require(migration["collector_pid"] != legacy_collector_pid, f"{label}.legacy_migration did not replace the legacy collector")
    stable_direct_parity(fresh, migration, f"{label}.legacy_migration")

    restart_raw = require_object(
        parsed["collector_restart"]["evidence"], f"{label}.collector_restart",
        DIRECT_KEYS | {"previous_collector_pid", "previous_report_stream_id"},
    )
    require(restart_raw["previous_collector_pid"] == migration["collector_pid"], f"{label}.collector_restart predecessor differs")
    require(restart_raw["previous_report_stream_id"] == parsed["legacy_migration"]["report_stream_id"], f"{label}.collector_restart predecessor stream differs")
    restart = validate_direct_evidence({key: restart_raw[key] for key in DIRECT_KEYS}, f"{label}.collector_restart.direct", runtime)
    require(restart["collector_pid"] != migration["collector_pid"], f"{label}.collector_restart did not change PID")
    stable_direct_parity(fresh, restart, f"{label}.collector_restart")

    fresh_stream = parsed["fresh_install"]["report_stream_id"]
    migration_stream = parsed["legacy_migration"]["report_stream_id"]
    restart_stream = parsed["collector_restart"]["report_stream_id"]
    require(len({fresh_stream, migration_stream, restart_stream}) == 3, f"{label} install/migration/restart streams are not distinct")
    require(parsed["fresh_install"]["report_sequence"] >= 1, f"{label}.fresh_install sequence is invalid")
    require(parsed["legacy_migration"]["report_sequence"] >= 1, f"{label}.legacy_migration sequence is invalid")

    daemon_raw = require_object(
        parsed["daemon_restart"]["evidence"], f"{label}.daemon_restart",
        DIRECT_KEYS | {"previous_daemon_pid", "daemon_pid", "previous_daemon_invocation_id", "daemon_invocation_id"},
    )
    daemon = validate_direct_evidence({key: daemon_raw[key] for key in DIRECT_KEYS}, f"{label}.daemon_restart.direct", runtime, expected_collector_pid=restart["collector_pid"], expected_daemon_id=runtime["daemon_id"])
    previous_daemon_pid = require_int(daemon_raw["previous_daemon_pid"], f"{label}.daemon_restart.previous_daemon_pid", minimum=2)
    daemon_pid = require_int(daemon_raw["daemon_pid"], f"{label}.daemon_restart.daemon_pid", minimum=2)
    require(previous_daemon_pid != daemon_pid, f"{label}.daemon_restart did not change daemon PID")
    previous_invocation = require_text(daemon_raw["previous_daemon_invocation_id"], f"{label}.daemon_restart.previous_daemon_invocation_id", pattern=IDENTITY_RE)
    invocation = require_text(daemon_raw["daemon_invocation_id"], f"{label}.daemon_restart.daemon_invocation_id", pattern=IDENTITY_RE)
    require(previous_invocation != invocation, f"{label}.daemon_restart did not change InvocationID")
    stable_direct_parity(fresh, daemon, f"{label}.daemon_restart")

    fallback_label = f"{label}.socket_loss_helper_fallback"
    fallback = require_object(
        parsed["socket_loss_helper_fallback"]["evidence"], fallback_label,
        {"collector_pid", "collection_mode", "direct_runtime_available", "helper_fallback", "inventory_complete", "inventory_count", "rootful_baseline_inventory_count", "semantic_sha256", "rootful_baseline_semantic_sha256", "full_fields_present", "stats_present", "secondary_structure_sha256", "container_actions_enabled", "container_updates_enabled", "collector_restart_count"},
    )
    require(fallback["collector_pid"] == restart["collector_pid"], f"{fallback_label} restarted collector")
    require(fallback["collection_mode"] == "typed-helper-summary", f"{fallback_label} did not use helper summary")
    require_bool(fallback["direct_runtime_available"], False, f"{fallback_label}.direct_runtime_available")
    require_bool(fallback["helper_fallback"], True, f"{fallback_label}.helper_fallback")
    require_bool(fallback["inventory_complete"], True, f"{fallback_label}.inventory_complete")
    require_int(fallback["inventory_count"], f"{fallback_label}.inventory_count", minimum=1)
    require(fallback["inventory_count"] == fallback["rootful_baseline_inventory_count"], f"{fallback_label} rootful count parity differs")
    require_digest(fallback["semantic_sha256"], f"{fallback_label}.semantic_sha256")
    require(fallback["semantic_sha256"] == fallback["rootful_baseline_semantic_sha256"], f"{fallback_label} semantic parity differs")
    require_bool(fallback["full_fields_present"], False, f"{fallback_label}.full_fields_present")
    require(fallback["secondary_structure_sha256"] == "", f"{fallback_label}.secondary_structure_sha256 must be empty")
    require_bool(fallback["stats_present"], False, f"{fallback_label}.stats_present")
    for name in ("container_actions_enabled", "container_updates_enabled"):
        require_bool(fallback[name], False, f"{fallback_label}.{name}")
    require(fallback["collector_restart_count"] == 0, f"{fallback_label} restarted collector")

    recovery = validate_direct_evidence(parsed["direct_recovery"]["evidence"], f"{label}.direct_recovery", runtime, expected_collector_pid=restart["collector_pid"], expected_daemon_id=runtime["daemon_id"])
    stable_direct_parity(fresh, recovery, f"{label}.direct_recovery")

    for name in ("collector_restart", "daemon_restart", "socket_loss_helper_fallback", "direct_recovery"):
        require(parsed[name]["report_stream_id"] == restart_stream, f"{label}.{name} changed report stream")
    sequences = [parsed[name]["report_sequence"] for name in ("collector_restart", "daemon_restart", "socket_loss_helper_fallback", "direct_recovery")]
    require(all(left < right for left, right in zip(sequences, sequences[1:])), f"{label} post-restart report sequence is not monotonic")

    ambiguity_label = f"{label}.dual_socket_ambiguity_refusal"
    ambiguity = require_object(
        parsed["dual_socket_ambiguity_refusal"]["evidence"], ambiguity_label,
        {"protected_collector_pid", "live_sockets", "probe_kind", "admission_refused", "fail_closed", "daemon_probe_count", "container_actions_enabled", "collector_restart_count"},
    )
    require(ambiguity["protected_collector_pid"] == restart["collector_pid"], f"{ambiguity_label} changed protected collector PID")
    require(ambiguity["probe_kind"] == "separate-unpinned-collector", f"{ambiguity_label}.probe_kind differs")
    live_sockets = ambiguity["live_sockets"]
    require(isinstance(live_sockets, list) and len(live_sockets) == 2, f"{ambiguity_label}.live_sockets must contain two sockets")
    for index, runtime_name in enumerate(REQUIRED_RUNTIMES):
        validate_socket_identity(live_sockets[index], f"{ambiguity_label}.live_sockets[{index}]", runtime_name, uid)
    require_bool(ambiguity["admission_refused"], True, f"{ambiguity_label}.admission_refused")
    require_bool(ambiguity["fail_closed"], True, f"{ambiguity_label}.fail_closed")
    require(ambiguity["daemon_probe_count"] == 0, f"{ambiguity_label} probed daemon")
    require_bool(ambiguity["container_actions_enabled"], False, f"{ambiguity_label}.container_actions_enabled")
    require(ambiguity["collector_restart_count"] == 0, f"{ambiguity_label} restarted collector")

    pin_label = f"{label}.exact_pin_recovery"
    pin_raw = require_object(
        parsed["exact_pin_recovery"]["evidence"], pin_label,
        DIRECT_KEYS | {"previous_collector_pid", "previous_report_stream_id", "pin_source", "pinned_socket_path", "socket_absent_observed", "fallback_report_sequence", "recovery_report_sequence", "recovered_socket_path", "selected_socket_path", "recovered_socket_uid", "recovered_socket_gid", "recovered_socket_mode", "recovered_socket_type", "recovered_socket_symlink", "candidate_count", "daemon_probe_count", "collector_restart_count"},
    )
    require(pin_raw["previous_collector_pid"] == restart["collector_pid"], f"{pin_label} predecessor PID differs")
    require(pin_raw["previous_report_stream_id"] == restart_stream, f"{pin_label} predecessor stream differs")
    pin = validate_direct_evidence({key: pin_raw[key] for key in DIRECT_KEYS}, f"{pin_label}.direct", runtime, expected_daemon_id=runtime["daemon_id"])
    require(pin["collector_pid"] != restart["collector_pid"], f"{pin_label} update did not restart collector")
    pin_stream = parsed["exact_pin_recovery"]["report_stream_id"]
    require(pin_stream != restart_stream, f"{pin_label} did not create a new report stream")
    require(pin_raw["pin_source"] == "root-owned-systemd-unit", f"{pin_label} pin source differs")
    for name in ("pinned_socket_path", "recovered_socket_path", "selected_socket_path"):
        require(pin_raw[name] == socket_path, f"{pin_label}.{name} differs")
    require_bool(pin_raw["socket_absent_observed"], True, f"{pin_label}.socket_absent_observed")
    require(pin_raw["recovered_socket_uid"] == uid and pin_raw["recovered_socket_gid"] == runtime["socket_gid"], f"{pin_label} recovered owner differs")
    require(pin_raw["recovered_socket_mode"] == runtime["socket_mode"], f"{pin_label} recovered mode differs")
    require(pin_raw["recovered_socket_type"] == "unix", f"{pin_label}.recovered_socket_type must be unix")
    require_bool(pin_raw["recovered_socket_symlink"], False, f"{pin_label}.recovered_socket_symlink")
    fallback_sequence = require_int(pin_raw["fallback_report_sequence"], f"{pin_label}.fallback_report_sequence", minimum=1)
    recovery_sequence = require_int(pin_raw["recovery_report_sequence"], f"{pin_label}.recovery_report_sequence", minimum=1)
    require(fallback_sequence < recovery_sequence <= parsed["exact_pin_recovery"]["report_sequence"], f"{pin_label} sequence causality differs")
    require(pin_raw["candidate_count"] == 1 and pin_raw["daemon_probe_count"] == 1, f"{pin_label} did not select/probe exactly one endpoint")
    require(pin_raw["collector_restart_count"] == 1, f"{pin_label} must record exactly one update restart")
    stable_direct_parity(fresh, pin, pin_label)

    parity_label = f"{label}.telemetry_parity"
    parity = require_object(
        parsed["telemetry_parity"]["evidence"], parity_label,
        {"collector_pid", "baseline_kind", "baseline_inventory_count", "collector_inventory_count", "baseline_semantic_sha256", "collector_semantic_sha256", "collector_full_fields_present", "collector_stats_present", "collector_secondary_inventory_present"},
    )
    require(parity["collector_pid"] == pin["collector_pid"], f"{parity_label} changed collector PID")
    require(parity["baseline_kind"] == "root-client-same-rootless-daemon", f"{parity_label} baseline differs")
    baseline_count = require_int(parity["baseline_inventory_count"], f"{parity_label}.baseline_inventory_count", minimum=1)
    collector_count = require_int(parity["collector_inventory_count"], f"{parity_label}.collector_inventory_count", minimum=1)
    require(baseline_count == collector_count == fresh["inventory_count"], f"{parity_label} count differs")
    baseline_semantic = require_digest(parity["baseline_semantic_sha256"], f"{parity_label}.baseline_semantic_sha256")
    collector_semantic = require_digest(parity["collector_semantic_sha256"], f"{parity_label}.collector_semantic_sha256")
    require(baseline_semantic == collector_semantic == fresh["semantic_sha256"], f"{parity_label} semantic inventory differs")
    for name in ("collector_full_fields_present", "collector_stats_present", "collector_secondary_inventory_present"):
        require_bool(parity[name], True, f"{parity_label}.{name}")
    require(parsed["telemetry_parity"]["report_stream_id"] == pin_stream, f"{parity_label} changed updated stream")
    require(parsed["telemetry_parity"]["report_sequence"] > parsed["exact_pin_recovery"]["report_sequence"], f"{parity_label} sequence is not monotonic")

    authority_label = f"{label}.authority_isolation"
    authority = require_object(
        parsed["authority_isolation"]["evidence"], authority_label,
        {"collector_pid", "collector_uid", "effective_uid", "effective_root", "safe_profile_enabled", "commands_enabled", "privileged_helper_enabled", "reduction_request_observed", "collector_command_transport_present", "collector_command_session_present", "container_actions_enabled", "container_updates_enabled", "rootful_socket_access", "helper_network_access"},
    )
    require(authority["collector_pid"] == pin["collector_pid"], f"{authority_label} changed collector PID")
    require(authority["collector_uid"] == uid == authority["effective_uid"], f"{authority_label} UID binding differs")
    require_bool(authority["effective_root"], False, f"{authority_label}.effective_root")
    for name in ("safe_profile_enabled", "privileged_helper_enabled", "reduction_request_observed"):
        require_bool(authority[name], True, f"{authority_label}.{name}")
    for name in ("commands_enabled", "collector_command_transport_present", "collector_command_session_present", "container_actions_enabled", "container_updates_enabled", "rootful_socket_access", "helper_network_access"):
        require_bool(authority[name], False, f"{authority_label}.{name}")

    cleanup = require_object(parsed["cleanup"]["evidence"], f"{label}.cleanup", {"runtime_stopped", "socket_absent", "fixtures_removed", "user_state_clean"})
    for name in cleanup:
        require_bool(cleanup[name], True, f"{label}.cleanup.{name}")
    return run


def validate_receipt(receipt: Any) -> dict[str, Any]:
    reject_sensitive_evidence(receipt)
    root = require_object(
        receipt,
        "receipt",
        {
            "schema_version",
            "kind",
            "result",
            "source_commit",
            "started_at",
            "completed_at",
            "artifacts",
            "runs",
            "source_hashes",
        },
    )
    require(root["schema_version"] == RECEIPT_SCHEMA_VERSION, "receipt.schema_version must be 1")
    require(root["kind"] == RECEIPT_KIND, f"receipt.kind must be {RECEIPT_KIND}")
    require(root["result"] == "passed", "receipt.result must be passed")
    source_commit = require_text(root["source_commit"], "receipt.source_commit", pattern=COMMIT_RE, maximum=40)
    started = parse_timestamp(root["started_at"], "receipt.started_at")
    completed = parse_timestamp(root["completed_at"], "receipt.completed_at")
    require(started < completed, "receipt chronology is invalid")

    artifacts = require_object(root["artifacts"], "receipt.artifacts", {"qualification_test", "collector", "helper", "installer"})
    expected_basenames = {"qualification_test": "dockeragent.test", "collector": "pulse-agent", "helper": "pulse-agent-helper"}
    expected_packages = {
        "qualification_test": "github.com/rcourtman/pulse-go-rewrite/scripts/installtests.test",
        "collector": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent",
        "helper": "github.com/rcourtman/pulse-go-rewrite/cmd/pulse-agent-helper",
    }
    for name, basename in expected_basenames.items():
        artifact = require_object(artifacts[name], f"receipt.artifacts.{name}", {"path_basename", "sha256", "package", "go_version", "vcs_revision", "vcs_modified"})
        require(artifact["path_basename"] == basename, f"receipt.artifacts.{name}.path_basename differs")
        require_digest(artifact["sha256"], f"receipt.artifacts.{name}.sha256")
        require(artifact["package"] == expected_packages[name], f"receipt.artifacts.{name}.package differs")
        require_text(artifact["go_version"], f"receipt.artifacts.{name}.go_version", pattern=GO_VERSION_RE, maximum=32)
        require(artifact["vcs_revision"] == source_commit, f"receipt.artifacts.{name}.vcs_revision differs from source_commit")
        require_bool(artifact["vcs_modified"], False, f"receipt.artifacts.{name}.vcs_modified")
    installer = require_object(artifacts["installer"], "receipt.artifacts.installer", {"path_basename", "sha256"})
    require(installer["path_basename"] == "install.sh", "receipt.artifacts.installer.path_basename differs")
    require_digest(installer["sha256"], "receipt.artifacts.installer.sha256")

    source_hashes = root["source_hashes"]
    require(isinstance(source_hashes, dict) and source_hashes, "receipt.source_hashes must be a non-empty object")
    for source_path, digest in source_hashes.items():
        require_text(source_path, "receipt.source_hashes path", maximum=512)
        require(
            source_path == Path(source_path).as_posix()
            and not source_path.startswith("/")
            and ".." not in Path(source_path).parts,
            "receipt.source_hashes contains a non-canonical path",
        )
        require_digest(digest, f"receipt.source_hashes[{source_path}]")
    require(source_hashes.get("scripts/install.sh") == installer["sha256"], "installer digest must equal governed scripts/install.sh")

    runs = root["runs"]
    require(isinstance(runs, list) and len(runs) == 2, "receipt.runs must contain two disposable hosts")
    validated = [validate_run(runs[index], runtime, index, started, completed) for index, runtime in enumerate(REQUIRED_RUNTIMES)]
    require(validated[0]["host"]["machine_id"] != validated[1]["host"]["machine_id"], "Docker and Podman runs must use distinct hosts")
    require(validated[0]["runtime"]["daemon_id"] != validated[1]["runtime"]["daemon_id"], "Docker and Podman daemon identities must differ")
    docker_stream = validated[0]["scenarios"][REQUIRED_SCENARIOS.index("collector_restart")]["report_stream_id"]
    podman_stream = validated[1]["scenarios"][REQUIRED_SCENARIOS.index("collector_restart")]["report_stream_id"]
    require(docker_stream != podman_stream, "Docker and Podman report streams must differ")
    for current in validated:
        uid = current["runtime"]["collector_uid"]
        expected_live_sockets = [
            {"runtime": runtime, "path": expected_socket_path(runtime, uid), "uid": uid, "gid": current["runtime"]["socket_gid"], "mode": current["runtime"]["socket_mode"], "type": "unix", "symlink": False}
            for runtime in REQUIRED_RUNTIMES
        ]
        ambiguity = current["scenarios"][REQUIRED_SCENARIOS.index("dual_socket_ambiguity_refusal")]["evidence"]
        require(ambiguity["live_sockets"] == expected_live_sockets, "dual-socket evidence differs from run-local socket identities")
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
        text = data.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise ValidationError("receipt is not UTF-8") from exc
    try:
        parsed = json.loads(
            text,
            object_pairs_hook=_reject_duplicate_keys,
            parse_constant=lambda value: fail(f"receipt contains invalid number {value}"),
        )
    except json.JSONDecodeError as exc:
        raise ValidationError(f"receipt is not valid JSON: {exc}") from exc
    validate_receipt(parsed)
    return parsed


def read_immutable_receipt(path: Path) -> bytes:
    flags = os.O_RDONLY
    if hasattr(os, "O_CLOEXEC"):
        flags |= os.O_CLOEXEC
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    if hasattr(os, "O_NONBLOCK"):
        flags |= os.O_NONBLOCK
    try:
        descriptor = os.open(path, flags)
    except OSError as exc:
        raise ValidationError(f"unable to open receipt: {exc}") from exc
    try:
        before = os.fstat(descriptor)
        require(stat.S_ISREG(before.st_mode), "receipt must be a regular file")
        require(before.st_size <= MAX_RECEIPT_BYTES, "receipt exceeds the maximum size")
        chunks: list[bytes] = []
        remaining = MAX_RECEIPT_BYTES + 1
        while remaining:
            chunk = os.read(descriptor, min(1024 * 1024, remaining))
            if not chunk:
                break
            chunks.append(chunk)
            remaining -= len(chunk)
        data = b"".join(chunks)
        require(len(data) <= MAX_RECEIPT_BYTES, "receipt exceeds the maximum size")
        after = os.fstat(descriptor)
        require(
            (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
            == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns),
            "receipt changed while it was being read",
        )
        require(len(data) == before.st_size, "receipt read was incomplete")
        return data
    finally:
        os.close(descriptor)


def _canonical_relative_path(value: Any, label: str) -> str:
    text = require_text(value, label, maximum=512)
    candidate = Path(text)
    require(
        not candidate.is_absolute()
        and candidate.as_posix() == text
        and text not in {"", "."}
        and ".." not in candidate.parts,
        f"{label} is not a canonical repository-relative path",
    )
    return text


def load_source_manifest(checkout: Path, manifest_path: Path) -> tuple[bytes, dict[str, str]]:
    manifest_bytes = read_immutable_receipt(manifest_path)
    try:
        manifest = json.loads(manifest_bytes.decode("utf-8"), object_pairs_hook=_reject_duplicate_keys)
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValidationError(f"source manifest is invalid JSON: {exc}") from exc
    manifest = require_object(
        manifest,
        "source manifest",
        {"schema_version", "manifest_id", "target_os", "description", "exact_paths", "recursive_roots", "include_suffixes", "exclude_suffixes"},
    )
    require(manifest["schema_version"] == SOURCE_MANIFEST_SCHEMA_VERSION, "source manifest schema differs")
    require(manifest["manifest_id"] == SOURCE_MANIFEST_ID, "source manifest ID differs")
    require(manifest["target_os"] == "linux", "source manifest target_os must be linux")
    require_text(manifest["description"], "source manifest description", maximum=1024)
    for field in ("exact_paths", "recursive_roots", "include_suffixes", "exclude_suffixes"):
        require(isinstance(manifest[field], list) and all(isinstance(item, str) for item in manifest[field]), f"source manifest {field} must be a text array")
        require(len(manifest[field]) == len(set(manifest[field])), f"source manifest {field} contains duplicates")
    require(manifest["exact_paths"], "source manifest exact_paths must not be empty")
    include_suffixes = tuple(manifest["include_suffixes"])
    exclude_suffixes = tuple(manifest["exclude_suffixes"])
    require(include_suffixes and all(suffix.startswith(".") for suffix in include_suffixes), "source manifest include_suffixes are invalid")
    require(all(suffix.startswith("_") or suffix.startswith(".") for suffix in exclude_suffixes), "source manifest exclude_suffixes are invalid")

    paths: set[str] = set()
    for raw_path in manifest["exact_paths"]:
        paths.add(_canonical_relative_path(raw_path, "source manifest exact path"))
    for raw_root in manifest["recursive_roots"]:
        root_text = _canonical_relative_path(raw_root, "source manifest recursive root")
        root = checkout / root_text
        require(root.is_dir() and not root.is_symlink(), f"source manifest recursive root is not a regular directory: {root_text}")
        for candidate in root.rglob("*"):
            if candidate.is_dir():
                continue
            relative = candidate.relative_to(checkout).as_posix()
            if candidate.name.endswith(exclude_suffixes):
                continue
            if not candidate.name.endswith(include_suffixes):
                continue
            paths.add(relative)
    hashes: dict[str, str] = {}
    checkout_resolved = checkout.resolve(strict=True)
    for relative in sorted(paths):
        candidate = checkout / relative
        try:
            metadata = candidate.lstat()
        except OSError as exc:
            raise ValidationError(f"unable to inspect governed source {relative}: {exc}") from exc
        require(stat.S_ISREG(metadata.st_mode), f"governed source is not a regular file: {relative}")
        require(not stat.S_ISLNK(metadata.st_mode), f"governed source is a symlink: {relative}")
        resolved = candidate.resolve(strict=True)
        require(resolved.is_relative_to(checkout_resolved), f"governed source escapes checkout: {relative}")
        source_bytes = read_immutable_receipt(candidate)
        hashes[relative] = hashlib.sha256(source_bytes).hexdigest()
    require(hashes, "source manifest expands to no source files")
    return manifest_bytes, hashes


def verify_source_commit(checkout: Path, source_commit: str, source_hashes: dict[str, str]) -> None:
    def git(*arguments: str) -> bytes:
        try:
            result = subprocess.run(
                ["git", *arguments],
                cwd=checkout,
                check=True,
                capture_output=True,
            )
        except (OSError, subprocess.CalledProcessError) as exc:
            detail = exc.stderr.decode("utf-8", errors="replace").strip() if isinstance(exc, subprocess.CalledProcessError) else str(exc)
            raise ValidationError(f"unable to verify receipt source commit: {detail}") from exc
        return result.stdout

    top_level = Path(git("rev-parse", "--show-toplevel").decode("utf-8").strip()).resolve(strict=True)
    require(top_level == checkout, "source checkout is not the requested repository root")
    resolved = git("rev-parse", "--verify", f"{source_commit}^{{commit}}").decode("ascii").strip()
    require(resolved == source_commit, "receipt source_commit is not an exact commit")
    for relative, expected_digest in source_hashes.items():
        source_bytes = git("show", f"{source_commit}:{relative}")
        require(
            hashlib.sha256(source_bytes).hexdigest() == expected_digest,
            f"governed source {relative} does not match receipt source_commit",
        )


@contextlib.contextmanager
def immutable_artifact_snapshot(path: Path, expected_basename: str, *, executable: bool) -> Any:
    require(path.name == expected_basename, f"artifact path basename must be {expected_basename}")
    flags = os.O_RDONLY
    if hasattr(os, "O_CLOEXEC"):
        flags |= os.O_CLOEXEC
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    if hasattr(os, "O_NONBLOCK"):
        flags |= os.O_NONBLOCK
    try:
        descriptor = os.open(path, flags)
    except OSError as exc:
        raise ValidationError(f"unable to open artifact {expected_basename}: {exc}") from exc
    try:
        before = os.fstat(descriptor)
        require(stat.S_ISREG(before.st_mode), f"artifact {expected_basename} must be a regular file")
        require(0 < before.st_size <= MAX_ARTIFACT_BYTES, f"artifact {expected_basename} size is invalid")
        if executable:
            require(before.st_mode & 0o111 != 0, f"artifact {expected_basename} is not executable")
        with tempfile.TemporaryDirectory(prefix="pulse-rootless-artifact-") as temporary:
            snapshot = Path(temporary) / expected_basename
            target = os.open(snapshot, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o700 if executable else 0o600)
            digest = hashlib.sha256()
            copied = 0
            try:
                while True:
                    chunk = os.read(descriptor, 1024 * 1024)
                    if not chunk:
                        break
                    copied += len(chunk)
                    require(copied <= MAX_ARTIFACT_BYTES, f"artifact {expected_basename} exceeds the maximum size")
                    digest.update(chunk)
                    offset = 0
                    while offset < len(chunk):
                        offset += os.write(target, chunk[offset:])
                os.fsync(target)
            finally:
                os.close(target)
            after = os.fstat(descriptor)
            require(
                (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns)
                == (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns),
                f"artifact {expected_basename} changed while being snapshotted",
            )
            require(copied == before.st_size, f"artifact {expected_basename} snapshot is incomplete")
            yield snapshot, digest.hexdigest()
    finally:
        os.close(descriptor)


def inspect_go_build_identity(artifact: Path) -> dict[str, Any]:
    try:
        result = subprocess.run(
            ["go", "version", "-m", str(artifact)],
            check=True,
            capture_output=True,
            timeout=15,
        )
    except (OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
        detail = ""
        if isinstance(exc, subprocess.CalledProcessError):
            detail = exc.stderr.decode("utf-8", errors="replace").strip()
        raise ValidationError(f"unable to inspect Go artifact build identity: {detail or exc}") from exc
    require(len(result.stdout) <= 256 * 1024, "Go artifact metadata is unexpectedly large")
    try:
        lines = result.stdout.decode("utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise ValidationError("Go artifact metadata is not UTF-8") from exc
    require(lines and ": " in lines[0], "Go artifact metadata header is malformed")
    go_version = lines[0].rsplit(": ", 1)[1]
    require_text(go_version, "Go artifact go_version", pattern=GO_VERSION_RE, maximum=32)
    settings: dict[str, str] = {}
    package = ""
    for line in lines[1:]:
        fields = line.strip().split("\t")
        if len(fields) == 2 and fields[0] == "path":
            require(not package, "Go artifact metadata contains duplicate package path")
            package = fields[1]
            continue
        if len(fields) != 2 or fields[0] != "build" or "=" not in fields[1]:
            continue
        key, value = fields[1].split("=", 1)
        require(key not in settings, f"Go artifact metadata contains duplicate build setting {key}")
        settings[key] = value
    require(settings.get("vcs") == "git", "Go artifact is not git VCS stamped")
    require_text(package, "Go artifact package", maximum=256)
    revision = require_text(settings.get("vcs.revision"), "Go artifact vcs.revision", pattern=COMMIT_RE, maximum=40)
    require(settings.get("vcs.modified") == "false", "Go artifact is modified or lacks vcs.modified=false")
    return {"package": package, "go_version": go_version, "vcs_revision": revision, "vcs_modified": False}


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
    supplied_artifacts = {
        "qualification_test": (qualification_test_path, "dockeragent.test", True),
        "collector": (collector_path, "pulse-agent", True),
        "helper": (helper_path, "pulse-agent-helper", True),
        "installer": (installer_path, "install.sh", False),
    }
    artifact_bindings: dict[str, dict[str, Any]] = {}
    with contextlib.ExitStack() as stack:
        snapshots: dict[str, Path] = {}
        for name, (path, basename, executable) in supplied_artifacts.items():
            snapshot, artifact_digest = stack.enter_context(
                immutable_artifact_snapshot(path, basename, executable=executable)
            )
            require(artifact_digest == receipt["artifacts"][name]["sha256"], f"{name} bytes do not match receipt SHA-256")
            snapshots[name] = snapshot
            artifact_bindings[name] = {"path_basename": basename, "sha256": artifact_digest}
        for name in ("qualification_test", "collector", "helper"):
            build = inspect_go_build_identity(snapshots[name])
            claimed = receipt["artifacts"][name]
            require(build["package"] == claimed["package"], f"{name} Go package differs from receipt")
            require(build["go_version"] == claimed["go_version"], f"{name} Go version differs from receipt")
            require(build["vcs_revision"] == claimed["vcs_revision"] == receipt["source_commit"], f"{name} VCS revision differs")
            require_bool(build["vcs_modified"], False, f"{name} vcs.modified")
            artifact_bindings[name].update(build)
    validator_path = Path(__file__).absolute()
    validator_bytes = read_immutable_receipt(validator_path)
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
        "source_commit_verified": verify_git_commit,
        "artifact_bindings": artifact_bindings,
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
    parser.add_argument("receipt", type=Path, help="schema-v1 rootless qualification receipt")
    parser.add_argument("--qualification-test", required=True, type=Path, help="exact dockeragent.test qualification binary")
    parser.add_argument("--collector", required=True, type=Path, help="exact pulse-agent binary")
    parser.add_argument("--helper", required=True, type=Path, help="exact pulse-agent-helper binary")
    parser.add_argument("--installer", required=True, type=Path, help="exact install.sh blob")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    arguments = parse_args(argv)
    try:
        attestation = create_attestation(
            arguments.receipt,
            arguments.qualification_test,
            arguments.collector,
            arguments.helper,
            arguments.installer,
        )
    except ValidationError as exc:
        print(f"rootless receipt validation failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps(attestation, sort_keys=True, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
