#!/usr/bin/env python3
"""Verify schema-v7 secure-runtime systemd and rootful-Docker evidence."""

from __future__ import annotations

import contextlib
import json
import sys
from pathlib import Path
from typing import Any, Iterator, Sequence

import secure_runtime_attestation as v5
import secure_runtime_attestation_v6 as v6


RECEIPT_SCHEMA_VERSION = 7
ATTESTATION_SCHEMA_VERSION = 7
SOURCE_MANIFEST_PATH = "scripts/release_control/secure_runtime_source_manifest_v7.json"
SOURCE_MANIFEST_SCHEMA_VERSION = 1
SOURCE_MANIFEST_ID = "secure-runtime-linux-v7"
ATTESTATION_TOOL_PATH = "scripts/release_control/secure_runtime_attestation_v7.py"

REQUIRED_SCENARIOS = (
    "legacy_root_command_capable_install",
    "read_only_inspect",
    "drop_in_fail_closed_rehearsal",
    "safe_profile_apply",
    "rootful_docker_summary_migration",
    "explicit_safe_profile_rollback",
    "automatic_failure_rollback",
    "ordinary_update_non_migration",
    "final_safe_profile_apply",
    "rootful_docker_summary_restart",
    "rootful_docker_helper_loss_recovery",
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
    **v6.SCENARIO_REQUIRED_CLAIMS,
    "rootful_docker_summary_migration": {
        "legacy_rootful_inventory_observed",
        "collector_rootful_socket_authority_removed",
        "typed_helper_summary_inventory_observed",
        "summary_inventory_parity_observed",
        "summary_only_boundary_observed",
    },
    "rootful_docker_summary_restart": {
        "typed_helper_summary_survived_helper_restart",
        "summary_inventory_parity_observed",
        "new_report_sequence_accepted_after_restart",
        "collector_remained_non_root",
    },
    "rootful_docker_helper_loss_recovery": {
        "helper_loss_emitted_status_only_report",
        "helper_health_degradation_visible",
        "helper_recovery_restored_complete_inventory",
        "report_order_remained_monotonic",
        "collector_rootful_socket_fallback_denied",
    },
}

SCENARIO_REQUIRED_OBSERVATIONS = {
    **v6.SCENARIO_REQUIRED_OBSERVATIONS,
    "rootful_docker_summary_migration": {
        "runtime": "docker",
        "legacy_collection_mode": "",
        "collection_mode": "typed-helper-summary",
        "inventory_complete": True,
        "collector_in_docker_group": False,
        "collector_rootful_socket_access": False,
        "secondary_inventories_empty": True,
        "container_actions_available": False,
        "update_checks_available": False,
        "fixture_container_names": ["pulse-v7-exited", "pulse-v7-running"],
        "fixture_image": "pulse-secure-runtime-fixture:v7",
        "fixture_states": {
            "pulse-v7-exited": "exited",
            "pulse-v7-running": "running",
        },
        "legacy_container_count": 2,
        "migrated_container_count": 2,
    },
    "rootful_docker_summary_restart": {
        "runtime": "docker",
        "collection_mode": "typed-helper-summary",
        "inventory_complete": True,
        "collector_rootful_socket_access": False,
    },
    "rootful_docker_helper_loss_recovery": {
        "runtime": "docker",
        "collection_mode": "typed-helper-summary",
        "loss_inventory_complete": False,
        "loss_module_state": "degraded",
        "loss_container_count": 0,
        "collector_rootful_socket_access_during_loss": False,
        "recovery_inventory_complete": True,
        "recovery_module_state": "running",
    },
}

_V6_CREATE_ATTESTATION = v6.create_attestation


@contextlib.contextmanager
def v7_contract() -> Iterator[None]:
    replacements = {
        "RECEIPT_SCHEMA_VERSION": RECEIPT_SCHEMA_VERSION,
        "ATTESTATION_SCHEMA_VERSION": ATTESTATION_SCHEMA_VERSION,
        "SOURCE_MANIFEST_PATH": SOURCE_MANIFEST_PATH,
        "SOURCE_MANIFEST_SCHEMA_VERSION": SOURCE_MANIFEST_SCHEMA_VERSION,
        "SOURCE_MANIFEST_ID": SOURCE_MANIFEST_ID,
        "ATTESTATION_TOOL_PATH": ATTESTATION_TOOL_PATH,
        "REQUIRED_SCENARIOS": REQUIRED_SCENARIOS,
        "SCENARIO_REQUIRED_CLAIMS": SCENARIO_REQUIRED_CLAIMS,
        "SCENARIO_REQUIRED_OBSERVATIONS": SCENARIO_REQUIRED_OBSERVATIONS,
        "__file__": __file__,
        "create_attestation": create_attestation,
    }
    previous = {name: getattr(v6, name) for name in replacements}
    try:
        for name, value in replacements.items():
            setattr(v6, name, value)
        yield
    finally:
        for name, value in previous.items():
            setattr(v6, name, value)


def _scenario_observations(receipt: dict[str, Any], name: str) -> dict[str, Any]:
    scenarios = receipt.get("scenarios")
    if not isinstance(scenarios, list):
        raise v5.AttestationError("receipt scenarios must be a list")
    for scenario in scenarios:
        if isinstance(scenario, dict) and scenario.get("name") == name:
            evidence = scenario.get("evidence")
            observations = evidence.get("observations") if isinstance(evidence, dict) else None
            if isinstance(observations, dict):
                return observations
    raise v5.AttestationError(f"scenario {name} has no typed observations")


def _positive_int(value: Any, label: str) -> int:
    if not isinstance(value, int) or isinstance(value, bool) or value <= 0:
        raise v5.AttestationError(f"{label} must be a positive integer")
    return value


def _sha256(value: Any, label: str) -> str:
    if not isinstance(value, str) or not v5.SHA256_RE.fullmatch(value):
        raise v5.AttestationError(f"{label} must be a SHA-256 digest")
    return value


def verify_rootful_docker_observations(receipt: dict[str, Any]) -> None:
    if receipt.get("rootful_container_runtime") != "docker":
        raise v5.AttestationError("schema-v7 receipt does not bind the qualified rootful runtime")
    if receipt.get("rootful_container_summary_qualified") is not True:
        raise v5.AttestationError("schema-v7 receipt does not qualify rootful Docker summary inventory")

    migration = _scenario_observations(receipt, "rootful_docker_summary_migration")
    legacy_count = _positive_int(migration.get("legacy_container_count"), "legacy Docker container count")
    migrated_count = _positive_int(migration.get("migrated_container_count"), "migrated Docker container count")
    legacy_digest = _sha256(migration.get("legacy_inventory_sha256"), "legacy Docker inventory")
    migrated_digest = _sha256(migration.get("migrated_inventory_sha256"), "migrated Docker inventory")
    if legacy_count != migrated_count or legacy_digest != migrated_digest:
        raise v5.AttestationError("rootful Docker migration did not preserve the canonical summary inventory")

    restart = _scenario_observations(receipt, "rootful_docker_summary_restart")
    pre_restart_count = _positive_int(restart.get("pre_restart_container_count"), "pre-restart Docker container count")
    post_restart_count = _positive_int(restart.get("post_restart_container_count"), "post-restart Docker container count")
    pre_restart_digest = _sha256(restart.get("pre_restart_inventory_sha256"), "pre-restart Docker inventory")
    post_restart_digest = _sha256(restart.get("post_restart_inventory_sha256"), "post-restart Docker inventory")
    pre_pid = restart.get("pre_restart_helper_pid")
    post_pid = restart.get("post_restart_helper_pid")
    if not isinstance(pre_pid, str) or not pre_pid.isdigit() or int(pre_pid) <= 0:
        raise v5.AttestationError("pre-restart helper PID is invalid")
    if not isinstance(post_pid, str) or not post_pid.isdigit() or int(post_pid) <= 0 or post_pid == pre_pid:
        raise v5.AttestationError("helper restart did not prove a changed process identity")
    stream = restart.get("report_stream_id")
    if not isinstance(stream, str) or not stream:
        raise v5.AttestationError("helper restart report stream identity is invalid")
    pre_sequence = _positive_int(restart.get("pre_restart_sequence"), "pre-restart report sequence")
    post_sequence = _positive_int(restart.get("post_restart_sequence"), "post-restart report sequence")
    if (
        pre_restart_count != migrated_count
        or post_restart_count != migrated_count
        or pre_restart_digest != migrated_digest
        or post_restart_digest != migrated_digest
        or post_sequence <= pre_sequence
    ):
        raise v5.AttestationError("rootful Docker summary did not survive the helper restart")

    recovery = _scenario_observations(receipt, "rootful_docker_helper_loss_recovery")
    pre_loss_count = _positive_int(recovery.get("pre_loss_container_count"), "pre-loss Docker container count")
    recovered_count = _positive_int(recovery.get("recovered_container_count"), "recovered Docker container count")
    pre_loss_digest = _sha256(recovery.get("pre_loss_inventory_sha256"), "pre-loss Docker inventory")
    recovered_digest = _sha256(recovery.get("recovered_inventory_sha256"), "recovered Docker inventory")
    loss_sequence = _positive_int(recovery.get("loss_sequence"), "helper-loss report sequence")
    recovery_sequence = _positive_int(recovery.get("recovery_sequence"), "helper-recovery report sequence")
    if recovery.get("report_stream_id") != stream:
        raise v5.AttestationError("helper loss/recovery changed the Docker report stream")
    if (
        pre_loss_count != migrated_count
        or recovered_count != migrated_count
        or pre_loss_digest != migrated_digest
        or recovered_digest != migrated_digest
        or recovery_sequence <= loss_sequence
        or loss_sequence <= post_sequence
    ):
        raise v5.AttestationError("rootful Docker helper loss/recovery evidence is not causally monotonic")


def load_attested_receipt(path: Path, attestation: dict[str, Any]) -> dict[str, Any]:
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise v5.AttestationError(f"unable to load schema-v7 receipt {path}: {exc}") from exc
    binding = attestation.get("receipt")
    if not isinstance(binding, dict) or v5.sha256_bytes(raw) != binding.get("sha256"):
        raise v5.AttestationError("schema-v7 Docker validation receipt bytes differ from the attested receipt")
    try:
        receipt = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise v5.AttestationError(f"unable to decode schema-v7 receipt {path}: {exc}") from exc
    if not isinstance(receipt, dict) or receipt.get("schema_version") != RECEIPT_SCHEMA_VERSION:
        raise v5.AttestationError("rootful Docker validation requires the attested schema-v7 receipt")
    v5.reject_sensitive_keys(receipt)
    return receipt


def create_attestation(**kwargs: Any) -> dict[str, Any]:
    with v7_contract():
        result = _V6_CREATE_ATTESTATION(**kwargs)
        receipt = load_attested_receipt(Path(kwargs["receipt_path"]), result)
        verify_rootful_docker_observations(receipt)
        return result


def main(argv: Sequence[str] | None = None) -> int:
    with v7_contract():
        return v6.main(sys.argv[1:] if argv is None else argv)


if __name__ == "__main__":
    raise SystemExit(main())
