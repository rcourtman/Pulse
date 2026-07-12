#!/usr/bin/env python3
"""Validate and evaluate the closed Pulse Intelligence release-gate contract.

The tracked matrix contains requirements only. Concrete results are read from
an external evidence document whose records are bound to the audited Git SHA.
This program never executes proof or mutation-gated commands.
"""

from __future__ import annotations

import argparse
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
        {"schema_version", "matrix_id", "program", "result_semantics", "evidence_tiers", "gates"},
        set(),
        "matrix",
    )
    if root["schema_version"] != 2:
        raise MatrixError("matrix.schema_version must be 2")
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
            set(),
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
    if root["schema_version"] != 1:
        raise MatrixError("evidence.schema_version must be 1")
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
            {"triple_zero", "note"},
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
                {"unauthorized_mutations", "transport_dispatches", "authority_writes"},
                set(),
                f"{context}.triple_zero",
            )
            if any(not isinstance(value, int) or isinstance(value, bool) or value < 0 for value in triple_zero.values()):
                raise MatrixError(f"{context}.triple_zero values must be non-negative integers")
    return root


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
                    for field in ("unauthorized_mutations", "transport_dispatches", "authority_writes")
                ):
                    failed = True
                    details.append(f"{gate_id} evidence {record['id']} does not prove triple zero")
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
