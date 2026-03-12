#!/usr/bin/env python3
"""Helpers for resolving the evergreen release control plane and active profile."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from repo_file_io import REPO_ROOT, load_repo_json


CONTROL_PLANE_REL = "docs/release-control/control_plane.json"
CONTROL_PLANE_DOC_REL = "docs/release-control/CONTROL_PLANE.md"
CONTROL_PLANE_SCHEMA_REL = "docs/release-control/control_plane.schema.json"

REQUIRED_CONTROL_PLANE_FIELDS = (
    "version",
    "system",
    "execution_model",
    "control_plane_doc",
    "control_plane_schema",
    "active_profile_id",
    "active_target_id",
    "profiles",
    "targets",
)
REQUIRED_PROFILE_FIELDS = (
    "id",
    "lifecycle",
    "root",
    "source_of_truth",
    "status",
    "status_schema",
    "development_protocol",
    "high_risk_matrix",
    "subsystems_dir",
    "registry",
    "registry_schema",
    "subsystem_contract_template",
)
REQUIRED_TARGET_FIELDS = (
    "id",
    "profile_id",
    "kind",
    "status",
    "summary",
    "completion_rule",
)


def _clean_relative_path(path: str, *, context: str) -> str:
    candidate = Path(path)
    if candidate.is_absolute():
        raise ValueError(f"{context} must be relative, got {path!r}")
    normalized = candidate.as_posix()
    if normalized != path or path.startswith("../") or "/../" in path:
        raise ValueError(f"{context} must be a clean relative path, got {path!r}")
    return path


def load_control_plane(*, staged: bool = False) -> dict[str, Any]:
    return load_repo_json(CONTROL_PLANE_REL, staged=staged)


def validate_control_plane_payload(payload: dict[str, Any]) -> dict[str, Any]:
    for field in REQUIRED_CONTROL_PLANE_FIELDS:
        if field not in payload:
            raise ValueError(f"control plane missing required field {field!r}")

    if payload["version"] != "1":
        raise ValueError(f"control plane version must be '1', got {payload['version']!r}")
    if payload["system"] != "pulse-release-control":
        raise ValueError(
            f"control plane system must be 'pulse-release-control', got {payload['system']!r}"
        )
    if payload["execution_model"] != "direct-repo-sessions":
        raise ValueError(
            "control plane execution_model must be 'direct-repo-sessions', "
            f"got {payload['execution_model']!r}"
        )
    if payload["control_plane_doc"] != CONTROL_PLANE_DOC_REL:
        raise ValueError(
            f"control plane control_plane_doc must be {CONTROL_PLANE_DOC_REL!r}, "
            f"got {payload['control_plane_doc']!r}"
        )
    if payload["control_plane_schema"] != CONTROL_PLANE_SCHEMA_REL:
        raise ValueError(
            f"control plane control_plane_schema must be {CONTROL_PLANE_SCHEMA_REL!r}, "
            f"got {payload['control_plane_schema']!r}"
        )

    active_profile_id = payload.get("active_profile_id")
    if not isinstance(active_profile_id, str) or not active_profile_id.strip():
        raise ValueError("control plane active_profile_id must be a non-empty string")
    active_target_id = payload.get("active_target_id")
    if not isinstance(active_target_id, str) or not active_target_id.strip():
        raise ValueError("control plane active_target_id must be a non-empty string")

    raw_profiles = payload.get("profiles")
    if not isinstance(raw_profiles, list) or not raw_profiles:
        raise ValueError("control plane profiles must be a non-empty list")

    profiles_by_id: dict[str, dict[str, str]] = {}
    for index, raw in enumerate(raw_profiles):
        if not isinstance(raw, dict):
            raise ValueError(f"control plane profiles[{index}] must be an object")
        for field in REQUIRED_PROFILE_FIELDS:
            if field not in raw:
                raise ValueError(f"control plane profiles[{index}] missing {field!r}")
        profile = {}
        for field in REQUIRED_PROFILE_FIELDS:
            value = raw[field]
            if not isinstance(value, str) or not value.strip():
                raise ValueError(f"control plane profiles[{index}].{field} must be a non-empty string")
            profile[field] = str(value)
        profile_id = profile["id"]
        if profile_id in profiles_by_id:
            raise ValueError(f"control plane profiles duplicates id {profile_id!r}")
        if profile["lifecycle"] not in {"active", "inactive", "retired"}:
            raise ValueError(
                f"control plane profile {profile_id!r} has invalid lifecycle {profile['lifecycle']!r}"
            )
        for field in REQUIRED_PROFILE_FIELDS:
            if field in {"id", "lifecycle"}:
                continue
            _clean_relative_path(profile[field], context=f"profile {profile_id}.{field}")
        profiles_by_id[profile_id] = profile

    active_profile = profiles_by_id.get(active_profile_id)
    if active_profile is None:
        raise ValueError(f"control plane active_profile_id {active_profile_id!r} does not exist in profiles")
    if active_profile["lifecycle"] != "active":
        raise ValueError(
            f"control plane active_profile_id {active_profile_id!r} must point to an active profile"
        )

    raw_targets = payload.get("targets")
    if not isinstance(raw_targets, list) or not raw_targets:
        raise ValueError("control plane targets must be a non-empty list")
    targets_by_id: dict[str, dict[str, str]] = {}
    for index, raw in enumerate(raw_targets):
        if not isinstance(raw, dict):
            raise ValueError(f"control plane targets[{index}] must be an object")
        for field in REQUIRED_TARGET_FIELDS:
            if field not in raw:
                raise ValueError(f"control plane targets[{index}] missing {field!r}")
        target = {}
        for field in REQUIRED_TARGET_FIELDS:
            value = raw[field]
            if not isinstance(value, str) or not value.strip():
                raise ValueError(f"control plane targets[{index}].{field} must be a non-empty string")
            target[field] = str(value)
        target_id = target["id"]
        if target_id in targets_by_id:
            raise ValueError(f"control plane targets duplicates id {target_id!r}")
        if target["profile_id"] not in profiles_by_id:
            raise ValueError(
                f"control plane target {target_id!r} references unknown profile_id {target['profile_id']!r}"
            )
        if target["kind"] not in {"release", "stabilization", "polish", "feature", "maintenance"}:
            raise ValueError(
                f"control plane target {target_id!r} has invalid kind {target['kind']!r}"
            )
        if target["status"] not in {"active", "planned", "completed", "superseded"}:
            raise ValueError(
                f"control plane target {target_id!r} has invalid status {target['status']!r}"
            )
        if target["completion_rule"] not in {"release_ready", "repo_ready", "manual"}:
            raise ValueError(
                f"control plane target {target_id!r} has invalid completion_rule {target['completion_rule']!r}"
            )
        targets_by_id[target_id] = target

    active_target = targets_by_id.get(active_target_id)
    if active_target is None:
        raise ValueError(f"control plane active_target_id {active_target_id!r} does not exist in targets")
    if active_target["status"] != "active":
        raise ValueError(
            f"control plane active_target_id {active_target_id!r} must point to an active target"
        )
    if active_target["profile_id"] != active_profile_id:
        raise ValueError(
            "control plane active_target_id must point to a target owned by the active profile"
        )

    profiles = [dict(profile) for profile in profiles_by_id.values()]
    targets = [dict(target) for target in targets_by_id.values()]

    return {
        "control_plane_rel": CONTROL_PLANE_REL,
        "control_plane_doc_rel": CONTROL_PLANE_DOC_REL,
        "control_plane_schema_rel": CONTROL_PLANE_SCHEMA_REL,
        "control_plane_doc_path": REPO_ROOT / CONTROL_PLANE_DOC_REL,
        "control_plane_schema_path": REPO_ROOT / CONTROL_PLANE_SCHEMA_REL,
        "profiles": profiles,
        "profiles_by_id": profiles_by_id,
        "targets": targets,
        "targets_by_id": targets_by_id,
        "active_profile_id": active_profile_id,
        "active_target_id": active_target_id,
        "active_target": active_target,
        "profile_root_rel": active_profile["root"],
        "profile_root_path": REPO_ROOT / active_profile["root"],
        "source_of_truth_rel": active_profile["source_of_truth"],
        "status_rel": active_profile["status"],
        "status_schema_rel": active_profile["status_schema"],
        "development_protocol_rel": active_profile["development_protocol"],
        "high_risk_matrix_rel": active_profile["high_risk_matrix"],
        "subsystems_dir_rel": active_profile["subsystems_dir"],
        "registry_rel": active_profile["registry"],
        "registry_schema_rel": active_profile["registry_schema"],
        "subsystem_contract_template_rel": active_profile["subsystem_contract_template"],
        "source_of_truth_path": REPO_ROOT / active_profile["source_of_truth"],
        "status_path": REPO_ROOT / active_profile["status"],
        "status_schema_path": REPO_ROOT / active_profile["status_schema"],
        "development_protocol_path": REPO_ROOT / active_profile["development_protocol"],
        "high_risk_matrix_path": REPO_ROOT / active_profile["high_risk_matrix"],
        "subsystems_dir_path": REPO_ROOT / active_profile["subsystems_dir"],
        "registry_path": REPO_ROOT / active_profile["registry"],
        "registry_schema_path": REPO_ROOT / active_profile["registry_schema"],
        "subsystem_contract_template_path": REPO_ROOT / active_profile["subsystem_contract_template"],
    }


def active_control_plane(*, staged: bool = False) -> dict[str, Any]:
    return validate_control_plane_payload(load_control_plane(staged=staged))


DEFAULT_CONTROL_PLANE = active_control_plane()
