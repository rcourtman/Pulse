#!/usr/bin/env python3
"""Validate and evaluate the closed Pulse Intelligence release-gate contract.

The tracked matrix contains requirements only. Concrete results are read from
an external evidence document whose records are bound to the audited Git SHA.
This program never executes proof or mutation-gated commands.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable


REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_MATRIX = REPO_ROOT / "docs/release-control/v6/internal/pulse-intelligence-release-gate.json"
GATE_IDS = tuple(f"RG-{number:02d}" for number in range(1, 13))
APPROVED_OWNERS = {
    "RG-01": "task-01",
    "RG-02": "task-02",
    "RG-03": "task-03",
    "RG-04": "task-04",
    "RG-05": "task-05",
    "RG-06": "task-08",
    "RG-07": "task-07",
    "RG-08": "task-06",
    "RG-09": "task-09",
    "RG-10": "task-10",
    "RG-11": "task-11",
    "RG-12": "task-12",
}
EVIDENCE_TIERS = ("unit", "integration", "browser", "real-lab", "physical-device", "live-relay")
RESULTS = {"PASS", "FAIL", "NOT_RUN"}
KIND_MAX_TIER = {
    "source": "unit",
    "mock": "integration",
    "task-summary": "unit",
    "automated-test": "integration",
    "browser-artifact": "browser",
    "real-lab-artifact": "real-lab",
    "physical-device-artifact": "physical-device",
    "live-relay-artifact": "live-relay",
}
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
TRIPLE_ZERO_FIELDS = ("persistence_writes", "dispatch_attempts", "external_mutations")
TRIPLE_ZERO_SEMANTICS = {
    "persistence_writes": "Unauthorized or revoked requests persist no action, approval, or authority state.",
    "dispatch_attempts": "Unauthorized or revoked requests create no executor, outbox, or transport dispatch attempt.",
    "external_mutations": "Unauthorized or revoked requests change no provider, device, Relay, or infrastructure state.",
}
RG12_ARTIFACT_CONTRACT = "physical-live-relay-v1"
RG12_PROOF_NAMES = (
    "fresh-pairing",
    "notifications",
    "relaunch-reconnect",
    "action-detail-r3",
    "revoked-access-r3",
    "unpaired-cleanup",
)


class MatrixError(ValueError):
    """Raised for malformed governance or evidence input."""


@dataclass(frozen=True)
class Evaluation:
    audited_sha: str
    gate_results: dict[str, str]
    details: tuple[str, ...]

    @property
    def verdict(self) -> str:
        return "GO" if all(value == "PASS" for value in self.gate_results.values()) else "NO-GO"


def _object(value: Any, context: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise MatrixError(f"{context} must be an object")
    return value


def _array(value: Any, context: str) -> list[Any]:
    if not isinstance(value, list):
        raise MatrixError(f"{context} must be an array")
    return value


def _string(value: Any, context: str) -> str:
    if not isinstance(value, str) or not value:
        raise MatrixError(f"{context} must be a non-empty string")
    return value


def _full_sha(value: Any, context: str) -> str:
    result = _string(value, context)
    if not SHA_RE.fullmatch(result):
        raise MatrixError(f"{context} must be a full lowercase Git SHA")
    return result


def _exact_keys(value: dict[str, Any], required: set[str], optional: set[str], context: str) -> None:
    missing = sorted(required - value.keys())
    unknown = sorted(value.keys() - required - optional)
    if missing:
        raise MatrixError(f"{context} missing fields: {', '.join(missing)}")
    if unknown:
        raise MatrixError(f"{context} has unknown fields: {', '.join(unknown)}")


def _validate_command(command: Any, context: str, *, mutation_gated: bool) -> str:
    value = _object(command, context)
    required = {"id", "argv"} | ({"authorization"} if mutation_gated else set())
    _exact_keys(value, required, set(), context)
    command_id = _string(value["id"], f"{context}.id")
    argv = _array(value["argv"], f"{context}.argv")
    if not argv or any(not isinstance(part, str) or not part for part in argv):
        raise MatrixError(f"{context}.argv must contain non-empty string arguments")
    if mutation_gated and value["authorization"] != "EXPLICIT_RELEASE_REQUIRED":
        raise MatrixError(f"{context}.authorization must be EXPLICIT_RELEASE_REQUIRED")
    return command_id


def validate_matrix(payload: Any) -> dict[str, Any]:
    root = _object(payload, "matrix")
    _exact_keys(
        root,
        {
            "schema_version",
            "matrix_id",
            "program",
            "result_semantics",
            "evidence_tiers",
            "triple_zero_semantics",
            "gates",
        },
        set(),
        "matrix",
    )
    if root["schema_version"] != 3:
        raise MatrixError("matrix.schema_version must be 3")
    if root["matrix_id"] != "pulse-intelligence-rg-01-rg-12":
        raise MatrixError("matrix.matrix_id is not the canonical Pulse Intelligence matrix")
    if root["program"] != "pulse-intelligence":
        raise MatrixError("matrix.program must be pulse-intelligence")
    if root["result_semantics"] != {
        "PASS": "External evidence satisfies the gate at the audited Git SHA.",
        "FAIL": "External evidence bound to the audited Git SHA records a failed proof or an invalid binding.",
        "NOT_RUN": "No qualifying external evidence exists for the gate at the audited Git SHA.",
    }:
        raise MatrixError("matrix.result_semantics must define the canonical PASS/FAIL/NOT_RUN meanings")
    if tuple(root["evidence_tiers"]) != EVIDENCE_TIERS:
        raise MatrixError("matrix.evidence_tiers must contain the canonical ordered tiers")
    if root["triple_zero_semantics"] != TRIPLE_ZERO_SEMANTICS:
        raise MatrixError("matrix.triple_zero_semantics must define the canonical persistence/dispatch/external oracle")

    gates = _array(root["gates"], "matrix.gates")
    gate_ids: list[str] = []
    all_command_ids: set[str] = set()
    for index, raw_gate in enumerate(gates):
        context = f"matrix.gates[{index}]"
        gate = _object(raw_gate, context)
        _exact_keys(
            gate,
            {
                "id",
                "title",
                "owner",
                "required_evidence_tier",
                "requires_triple_zero",
                "allows_ancestor_provenance",
                "non_mutating_proof_commands",
                "mutation_gated_commands",
            },
            {"artifact_contract"},
            context,
        )
        gate_id = _string(gate["id"], f"{context}.id")
        gate_ids.append(gate_id)
        _string(gate["title"], f"{context}.title")
        if gate.get("owner") != APPROVED_OWNERS.get(gate_id):
            raise MatrixError(f"{gate_id} owner must be {APPROVED_OWNERS.get(gate_id)!r}")
        if gate["required_evidence_tier"] not in EVIDENCE_TIERS:
            raise MatrixError(f"{gate_id} has an unknown required evidence tier")
        for field in ("requires_triple_zero", "allows_ancestor_provenance"):
            if not isinstance(gate[field], bool):
                raise MatrixError(f"{gate_id}.{field} must be boolean")
        artifact_contract = gate.get("artifact_contract")
        if artifact_contract is not None:
            if gate_id != "RG-12" or artifact_contract != RG12_ARTIFACT_CONTRACT:
                raise MatrixError(f"{gate_id}.artifact_contract is invalid")
        if gate_id == "RG-12" and gate["required_evidence_tier"] == "live-relay" and artifact_contract != RG12_ARTIFACT_CONTRACT:
            raise MatrixError(f"RG-12.artifact_contract must be {RG12_ARTIFACT_CONTRACT!r}")
        proof_commands = _array(gate["non_mutating_proof_commands"], f"{context}.non_mutating_proof_commands")
        if not proof_commands:
            raise MatrixError(f"{gate_id} must declare at least one non-mutating proof command")
        for command_index, command in enumerate(proof_commands):
            command_id = _validate_command(
                command,
                f"{context}.non_mutating_proof_commands[{command_index}]",
                mutation_gated=False,
            )
            if command_id in all_command_ids:
                raise MatrixError(f"duplicate proof command id {command_id!r}")
            all_command_ids.add(command_id)
        for command_index, command in enumerate(
            _array(gate["mutation_gated_commands"], f"{context}.mutation_gated_commands")
        ):
            command_id = _validate_command(
                command,
                f"{context}.mutation_gated_commands[{command_index}]",
                mutation_gated=True,
            )
            if command_id in all_command_ids:
                raise MatrixError(f"duplicate proof command id {command_id!r}")
            all_command_ids.add(command_id)
    if len(gate_ids) != len(set(gate_ids)):
        raise MatrixError("matrix.gates contains duplicate gate ids")
    if tuple(gate_ids) != GATE_IDS:
        raise MatrixError(f"matrix.gates must contain exactly {', '.join(GATE_IDS)} in order")
    return root


def _gate_command_ids(gate: dict[str, Any]) -> set[str]:
    return {
        command["id"]
        for field in ("non_mutating_proof_commands", "mutation_gated_commands")
        for command in gate[field]
    }


def validate_evidence(payload: Any, matrix: dict[str, Any]) -> dict[str, Any]:
    root = _object(payload, "evidence")
    _exact_keys(root, {"schema_version", "matrix_id", "audited_git_sha", "records"}, set(), "evidence")
    if root["schema_version"] != 2:
        raise MatrixError("evidence.schema_version must be 2")
    if root["matrix_id"] != matrix["matrix_id"]:
        raise MatrixError("evidence.matrix_id does not match the selected matrix")
    _full_sha(root["audited_git_sha"], "evidence.audited_git_sha")
    gates = {gate["id"]: gate for gate in matrix["gates"]}
    record_ids: set[str] = set()
    for index, raw_record in enumerate(_array(root["records"], "evidence.records")):
        context = f"evidence.records[{index}]"
        record = _object(raw_record, context)
        _exact_keys(
            record,
            {"id", "gate_id", "result", "tier", "kind", "git_sha", "command_id"},
            {"triple_zero", "artifact", "note"},
            context,
        )
        record_id = _string(record["id"], f"{context}.id")
        if record_id in record_ids:
            raise MatrixError(f"evidence has duplicate record id {record_id!r}")
        record_ids.add(record_id)
        gate_id = _string(record["gate_id"], f"{context}.gate_id")
        if gate_id not in gates:
            raise MatrixError(f"{context}.gate_id is not in the closed matrix")
        if record["result"] not in RESULTS:
            raise MatrixError(f"{context}.result is invalid")
        tier = record["tier"]
        kind = record["kind"]
        if tier not in EVIDENCE_TIERS:
            raise MatrixError(f"{context}.tier is invalid")
        if kind not in KIND_MAX_TIER:
            raise MatrixError(f"{context}.kind is invalid")
        if EVIDENCE_TIERS.index(tier) > EVIDENCE_TIERS.index(KIND_MAX_TIER[kind]):
            raise MatrixError(f"{context} overstates {kind!r} as {tier!r} evidence")
        _full_sha(record["git_sha"], f"{context}.git_sha")
        if record["command_id"] not in _gate_command_ids(gates[gate_id]):
            raise MatrixError(f"{context}.command_id does not name a command declared by {gate_id}")
        if "triple_zero" in record:
            triple_zero = _object(record["triple_zero"], f"{context}.triple_zero")
            _exact_keys(
                triple_zero,
                set(TRIPLE_ZERO_FIELDS),
                set(),
                f"{context}.triple_zero",
            )
            if any(not isinstance(value, int) or isinstance(value, bool) or value < 0 for value in triple_zero.values()):
                raise MatrixError(f"{context}.triple_zero values must be non-negative integers")
        if "artifact" in record:
            if gate_id != "RG-12":
                raise MatrixError(f"{context}.artifact is only valid for RG-12")
            artifact = _object(record["artifact"], f"{context}.artifact")
            _exact_keys(
                artifact,
                {"contract", "path", "sha256sums_sha256", "mobile_git_sha"},
                set(),
                f"{context}.artifact",
            )
            if artifact["contract"] != RG12_ARTIFACT_CONTRACT:
                raise MatrixError(f"{context}.artifact.contract is invalid")
            _string(artifact["path"], f"{context}.artifact.path")
            digest = _string(artifact["sha256sums_sha256"], f"{context}.artifact.sha256sums_sha256")
            if not SHA256_RE.fullmatch(digest):
                raise MatrixError(f"{context}.artifact.sha256sums_sha256 must be a lowercase SHA-256")
            _full_sha(artifact["mobile_git_sha"], f"{context}.artifact.mobile_git_sha")
    return root


def _read_json(path: Path, context: str) -> dict[str, Any] | list[Any]:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise MatrixError(f"{context} is unreadable: {exc}") from exc


def _sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _validate_rg12_manifest(root: Path, expected_manifest_digest: str) -> None:
    manifest = root / "SHA256SUMS"
    if not manifest.is_file() or _sha256_file(manifest) != expected_manifest_digest:
        raise MatrixError("RG-12 SHA256SUMS is absent or does not match the evidence record")
    declared: dict[str, str] = {}
    for line_number, line in enumerate(manifest.read_text(encoding="utf-8").splitlines(), 1):
        match = re.fullmatch(r"([0-9a-f]{64})  (.+)", line)
        if match is None:
            raise MatrixError(f"RG-12 SHA256SUMS line {line_number} is malformed")
        digest, relative_text = match.groups()
        relative = Path(relative_text)
        if relative.is_absolute() or ".." in relative.parts or relative_text == "SHA256SUMS":
            raise MatrixError(f"RG-12 SHA256SUMS line {line_number} has an unsafe path")
        if relative_text in declared:
            raise MatrixError(f"RG-12 SHA256SUMS repeats {relative_text!r}")
        declared[relative_text] = digest
    actual = {
        str(candidate.relative_to(root))
        for candidate in root.rglob("*")
        if candidate.is_file() and candidate != manifest
    }
    if set(declared) != actual:
        missing = sorted(actual - set(declared))
        stale = sorted(set(declared) - actual)
        raise MatrixError(f"RG-12 SHA256SUMS coverage mismatch; missing={missing}, stale={stale}")
    for relative_text, expected in declared.items():
        if _sha256_file(root / relative_text) != expected:
            raise MatrixError(f"RG-12 artifact checksum mismatch for {relative_text}")


def _zero(value: Any, context: str) -> None:
    if value != 0:
        raise MatrixError(f"{context} must be zero")


def _validate_rg12_action_semantics(root: Path, environment: dict[str, Any]) -> None:
    action_ids = environment.get("action_ids")
    if not isinstance(action_ids, list) or len(action_ids) != 2 or any(not isinstance(value, str) or not value for value in action_ids):
        raise MatrixError("RG-12 environment.action_ids must identify exactly two actions")
    action_id_set = set(action_ids)
    if len(action_id_set) != 2:
        raise MatrixError("RG-12 environment.action_ids must be unique")

    api = _object(_read_json(root / "canonical-action-api.json", "RG-12 canonical action API"), "RG-12 canonical action API")
    if api.get("approve_http_status") != 200 or api.get("deny_http_status") != 200 or api.get("matching_pending_after_count") != 0:
        raise MatrixError("RG-12 canonical decisions must return 200 and leave zero matching pending actions")
    for name, expected_state, expected_outcome in (
        ("approve", "approved", "approved"),
        ("deny", "rejected", "rejected"),
    ):
        decision = _object(api.get(name), f"RG-12 {name}")
        if decision.get("state") != expected_state or decision.get("execute_endpoint_called") is not False:
            raise MatrixError(f"RG-12 {name} state or execute boundary is invalid")
        approval = _object(decision.get("approval"), f"RG-12 {name}.approval")
        audit = _object(decision.get("audit"), f"RG-12 {name}.audit")
        request = _object(audit.get("request"), f"RG-12 {name}.audit.request")
        params = _object(request.get("params"), f"RG-12 {name}.audit.request.params")
        if approval.get("outcome") != expected_outcome or decision.get("actionId") not in action_id_set:
            raise MatrixError(f"RG-12 {name} decision correlation is invalid")
        if params.get("target_type") != "test" or params.get("executor") != "none":
            raise MatrixError(f"RG-12 {name} is not an inert target_type=test action")

    store = _object(_read_json(root / "action-store-observations.json", "RG-12 action store"), "RG-12 action store")
    for field in ("dispatch_attempts", "dispatch_outbox", "dispatch_receipts"):
        _zero(store.get(field), f"RG-12 action store {field}")
    records = _array(store.get("records"), "RG-12 action store records")
    decision_records = [
        record
        for record in records
        if isinstance(record, dict) and record.get("id") in action_id_set
    ]
    if {record.get("id") for record in decision_records} != action_id_set:
        raise MatrixError("RG-12 action-store records do not match the correlated action IDs")
    if {record.get("state") for record in decision_records} != {"approved", "rejected"}:
        raise MatrixError("RG-12 action-store records do not contain one approval and one rejection")
    if any(
        record.get("target_type") != "test" or record.get("executor") != "none"
        for record in decision_records
    ):
        raise MatrixError("RG-12 action-store records are not inert test actions")
    barrier_records = [
        record
        for record in records
        if isinstance(record, dict) and record.get("id") == environment.get("revoked_barrier_action_id")
    ]
    if len(barrier_records) != 1 or barrier_records[0].get("state") != "pending_approval":
        raise MatrixError("RG-12 action store does not preserve the revoked pending barrier")


def _validate_rg12_revocation_and_cleanup(root: Path, environment: dict[str, Any]) -> None:
    barrier = _object(_read_json(root / "revocation-negative-barrier.json", "RG-12 revocation barrier"), "RG-12 revocation barrier")
    if barrier.get("token_delete_http_status") != 204:
        raise MatrixError("RG-12 exact token deletion did not return 204")
    if barrier.get("post_revoke_pending_http_status") != 401 or barrier.get("post_revoke_decision_http_status") != 401:
        raise MatrixError("RG-12 revoked token did not fail closed with HTTP 401")
    if barrier.get("admin_bypass_enabled") is not False:
        raise MatrixError("RG-12 revocation barrier was not run with admin bypass disabled")
    triple_zero = _object(barrier.get("triple_zero"), "RG-12 revocation triple-zero")
    if triple_zero != {field: 0 for field in TRIPLE_ZERO_FIELDS}:
        raise MatrixError("RG-12 revocation barrier does not prove canonical triple zero")
    residue = _object(barrier.get("transport_residue"), "RG-12 revocation transport residue")
    for field in ("dispatch_outbox", "dispatch_receipts"):
        _zero(residue.get(field), f"RG-12 revocation {field}")
    before = _array(barrier.get("before"), "RG-12 revocation before")
    after = _array(barrier.get("after"), "RG-12 revocation after")
    if len(before) != 1 or len(after) != 1:
        raise MatrixError("RG-12 revocation barrier must contain one before and after record")
    expected_id = environment.get("revoked_barrier_action_id")
    for record in (before[0], after[0]):
        record = _object(record, "RG-12 revocation record")
        if record.get("id") != expected_id or record.get("state") != "pending_approval" or record.get("decision_revision", record.get("decisionRevision")) != 0:
            raise MatrixError("RG-12 revoked request changed the pending barrier action")

    cleanup = _object(_read_json(root / "cleanup.json", "RG-12 cleanup"), "RG-12 cleanup")
    for field in (
        "temporary_api_token_present_after",
        "temporary_license_activation_files_present_after",
    ):
        if cleanup.get(field) is not False:
            raise MatrixError(f"RG-12 cleanup {field} must be false")
    for field in ("local_environment_restored_from_pre_run_backup", "ipad_local_pairing_removed"):
        if cleanup.get(field) is not True:
            raise MatrixError(f"RG-12 cleanup {field} must be true")
    relay_after = _array(cleanup.get("relay_after"), "RG-12 relay cleanup")
    counts = {
        entry.get("surface"): entry.get("temp_count")
        for entry in relay_after
        if isinstance(entry, dict)
    }
    if counts != {"instances": 0, "device_tokens": 0, "connection_log": 0, "original_preserved": 1}:
        raise MatrixError("RG-12 Relay cleanup did not remove temporary state while preserving the original instance")
    for document in _array(cleanup.get("local_action_store_after"), "RG-12 local action cleanup"):
        rows = _array(document, "RG-12 local action cleanup document")
        for row in rows:
            row = _object(row, "RG-12 local action cleanup row")
            for value in row.values():
                _zero(value, "RG-12 local action residue")


def _validate_rg12_mobile_proofs(root: Path) -> None:
    digest_records = _array(_read_json(root / "xcresult-digests.json", "RG-12 xcresult digests"), "RG-12 xcresult digests")
    by_name = {
        record.get("proof"): record
        for record in digest_records
        if isinstance(record, dict)
    }
    if set(by_name) != set(RG12_PROOF_NAMES):
        raise MatrixError("RG-12 xcresult digests do not cover the exact physical proof sequence")
    for proof_name in RG12_PROOF_NAMES:
        proof_dir = root / "mobile-proofs" / proof_name
        summary = _object(_read_json(proof_dir / "test-summary.json", f"RG-12 {proof_name} summary"), f"RG-12 {proof_name} summary")
        if summary.get("result") != "Passed" or not isinstance(summary.get("totalTestCount"), int) or summary["totalTestCount"] < 1:
            raise MatrixError(f"RG-12 {proof_name} did not record a passed physical test")
        failed_tests = summary.get("failedTests")
        if failed_tests not in (0, []):
            raise MatrixError(f"RG-12 {proof_name} contains failed tests")
        screenshots = list((proof_dir / "attachments").glob("*.png"))
        if not screenshots:
            raise MatrixError(f"RG-12 {proof_name} has no sealed screenshot")

        record = _object(by_name[proof_name], f"RG-12 {proof_name} xcresult digest")
        members = _array(record.get("members"), f"RG-12 {proof_name} xcresult members")
        if record.get("file_count") != len(members) or not members:
            raise MatrixError(f"RG-12 {proof_name} xcresult member count is invalid")
        lines: list[str] = []
        member_paths: set[str] = set()
        for member in members:
            member = _object(member, f"RG-12 {proof_name} xcresult member")
            member_path = _string(member.get("path"), f"RG-12 {proof_name} xcresult member path")
            member_digest = _string(member.get("sha256"), f"RG-12 {proof_name} xcresult member digest")
            if member_path in member_paths or not SHA256_RE.fullmatch(member_digest):
                raise MatrixError(f"RG-12 {proof_name} xcresult member manifest is invalid")
            member_paths.add(member_path)
            lines.append(f"{member_digest}  {member_path}\n")
        aggregate = hashlib.sha256("".join(lines).encode()).hexdigest()
        if record.get("aggregate_sha256") != aggregate:
            raise MatrixError(f"RG-12 {proof_name} xcresult aggregate digest is invalid")
        source = Path(_string(record.get("source"), f"RG-12 {proof_name} xcresult source")).resolve()
        if not source.is_dir():
            raise MatrixError(f"RG-12 {proof_name} xcresult source is unavailable")
        source_members = {
            str(candidate.relative_to(source)): _sha256_file(candidate)
            for candidate in source.rglob("*")
            if candidate.is_file()
        }
        expected_members = {member["path"]: member["sha256"] for member in members}
        if source_members != expected_members:
            raise MatrixError(f"RG-12 {proof_name} underlying xcresult no longer matches its sealed digest")


def _validate_rg12_artifact(record: dict[str, Any], audited_sha: str) -> None:
    descriptor = _object(record.get("artifact"), "RG-12 evidence artifact")
    root = Path(_string(descriptor.get("path"), "RG-12 evidence artifact path")).resolve()
    if not root.is_dir():
        raise MatrixError("RG-12 evidence artifact directory is unavailable")
    _validate_rg12_manifest(root, descriptor["sha256sums_sha256"])
    environment = _object(_read_json(root / "environment.json", "RG-12 environment"), "RG-12 environment")
    if environment.get("core_sha") != audited_sha or environment.get("mobile_sha") != descriptor["mobile_git_sha"]:
        raise MatrixError("RG-12 artifact core/mobile SHA binding is invalid")
    if environment.get("credential_material_in_bundle") is not False:
        raise MatrixError("RG-12 artifact may not contain credential material")
    correlation = _object(_read_json(root / "correlation.json", "RG-12 correlation"), "RG-12 correlation")
    for field in ("core_sha", "mobile_sha", "action_ids", "token_record_id"):
        expected = environment.get(field if field != "token_record_id" else "temporary_token_record_id")
        if correlation.get(field) != expected:
            raise MatrixError(f"RG-12 correlation.{field} does not match environment")
    if correlation.get("target_type") != "test" or correlation.get("executor") != "none":
        raise MatrixError("RG-12 correlation does not bind an inert test action")
    if tuple(correlation.get("physical_proof_sequence", ())) != RG12_PROOF_NAMES:
        raise MatrixError("RG-12 physical proof sequence is incomplete")
    _validate_rg12_action_semantics(root, environment)
    _validate_rg12_revocation_and_cleanup(root, environment)
    _validate_rg12_mobile_proofs(root)


def evaluate_matrix(
    matrix: dict[str, Any],
    audited_sha: str,
    evidence: dict[str, Any] | None,
    *,
    is_ancestor: Callable[[str, str], bool] | None = None,
) -> Evaluation:
    _full_sha(audited_sha, "audited SHA")
    details: list[str] = []
    results: dict[str, str] = {}
    records = evidence["records"] if evidence is not None else []
    evidence_binding_valid = evidence is None or evidence["audited_git_sha"] == audited_sha
    if evidence is not None and not evidence_binding_valid:
        details.append(
            f"external evidence is bound to {evidence['audited_git_sha']}, not audited SHA {audited_sha}"
        )

    for gate in matrix["gates"]:
        gate_id = gate["id"]
        gate_records = [record for record in records if record["gate_id"] == gate_id]
        qualifying = False
        failed = not evidence_binding_valid
        for record in gate_records:
            if record["git_sha"] != audited_sha:
                ancestor = (
                    gate["allows_ancestor_provenance"]
                    and is_ancestor is not None
                    and is_ancestor(record["git_sha"], audited_sha)
                )
                if ancestor:
                    details.append(
                        f"{gate_id} provenance {record['id']} is ancestor-bound to {record['git_sha']} and does not certify {audited_sha}"
                    )
                else:
                    details.append(
                        f"{gate_id} evidence {record['id']} is bound to wrong SHA {record['git_sha']}; expected {audited_sha}"
                    )
                    failed = True
                continue
            if record["result"] == "FAIL":
                failed = True
                details.append(f"{gate_id} evidence {record['id']} records FAIL")
                continue
            if record["result"] != "PASS":
                continue
            if EVIDENCE_TIERS.index(record["tier"]) < EVIDENCE_TIERS.index(gate["required_evidence_tier"]):
                details.append(
                    f"{gate_id} evidence {record['id']} is only {record['tier']}; requires {gate['required_evidence_tier']}"
                )
                continue
            if gate["requires_triple_zero"]:
                triple_zero = record.get("triple_zero")
                if triple_zero is None:
                    failed = True
                    details.append(f"{gate_id} evidence {record['id']} is missing required triple-zero fields")
                    continue
                if any(
                    triple_zero[field] != 0
                    for field in TRIPLE_ZERO_FIELDS
                ):
                    failed = True
                    details.append(f"{gate_id} evidence {record['id']} does not prove triple zero")
                    continue
            if gate_id == "RG-12" and gate.get("artifact_contract") == RG12_ARTIFACT_CONTRACT:
                if record.get("artifact") is None:
                    failed = True
                    details.append(f"RG-12 evidence {record['id']} is missing the required sealed artifact descriptor")
                    continue
                try:
                    _validate_rg12_artifact(record, audited_sha)
                except MatrixError as exc:
                    failed = True
                    details.append(f"RG-12 evidence {record['id']} artifact validation failed: {exc}")
                    continue
            qualifying = True
        if failed:
            results[gate_id] = "FAIL"
        elif qualifying:
            results[gate_id] = "PASS"
        else:
            results[gate_id] = "NOT_RUN"
    return Evaluation(audited_sha=audited_sha, gate_results=results, details=tuple(details))


def _git_head() -> str:
    return subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=REPO_ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()


def _require_commit_sha(value: str) -> str:
    sha = _full_sha(value, "audited SHA")
    result = subprocess.run(
        ["git", "cat-file", "-e", f"{sha}^{{commit}}"],
        cwd=REPO_ROOT,
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if result.returncode != 0:
        raise MatrixError(f"audited SHA {sha} is not a commit in this repository")
    return sha


def _require_external_evidence_path(path: Path) -> Path:
    resolved = path.resolve()
    try:
        relative = resolved.relative_to(REPO_ROOT)
    except ValueError:
        return resolved
    tracked = subprocess.run(
        ["git", "ls-files", "--error-unmatch", "--", str(relative)],
        cwd=REPO_ROOT,
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if tracked.returncode == 0:
        raise MatrixError("--evidence must reference an external or untracked JSON file")
    return resolved


def _is_ancestor(older: str, newer: str) -> bool:
    return subprocess.run(
        ["git", "merge-base", "--is-ancestor", older, newer],
        cwd=REPO_ROOT,
        check=False,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    ).returncode == 0


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true", help="derive a verdict from external evidence")
    mode.add_argument("--validate-only", action="store_true", help="validate the tracked requirements schema")
    parser.add_argument("--matrix", type=Path, default=DEFAULT_MATRIX)
    parser.add_argument("--sha", help="full Git SHA to audit; defaults to current HEAD")
    parser.add_argument("--evidence", type=Path, help="external, untracked SHA-bound evidence JSON")
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    try:
        matrix = validate_matrix(json.loads(args.matrix.read_text(encoding="utf-8")))
        if args.validate_only:
            if args.sha or args.evidence:
                raise MatrixError("--validate-only accepts the tracked matrix only")
            print(f"Pulse Intelligence gate matrix schema valid: {len(matrix['gates'])} requirement-only gates")
            return 0
        audited_sha = _require_commit_sha(args.sha or _git_head())
        evidence = None
        if args.evidence is not None:
            evidence_path = _require_external_evidence_path(args.evidence)
            evidence = validate_evidence(json.loads(evidence_path.read_text(encoding="utf-8")), matrix)
        evaluation = evaluate_matrix(matrix, audited_sha, evidence, is_ancestor=_is_ancestor)
    except (OSError, json.JSONDecodeError, MatrixError, subprocess.CalledProcessError) as exc:
        print(f"BLOCKED: {exc}")
        return 1

    print(f"Pulse Intelligence audited SHA: {evaluation.audited_sha}")
    for gate_id in GATE_IDS:
        print(f"{gate_id}: {evaluation.gate_results[gate_id]}")
    for detail in evaluation.details:
        print(f"DETAIL: {detail}")
    print(f"Pulse Intelligence release verdict: {evaluation.verdict}")
    return 0 if evaluation.verdict == "GO" else 1


if __name__ == "__main__":
    raise SystemExit(main())
