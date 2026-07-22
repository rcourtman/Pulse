#!/usr/bin/env python3
"""Verify Pulse core against the checked-in Pulse Mobile consumer contract."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import subprocess
import sys
from pathlib import Path
from typing import Any, Iterable

from generate_mobile_compatibility import (
    MANIFEST_REL,
    ManifestError,
    check_outputs,
    generated_outputs,
    load_manifest,
    manifest_sha256,
)


REPO_ROOT = Path(__file__).resolve().parents[2]
CONSUMER_REL = Path("config/mobile-api-surface.json")
VALID_FIELD_TYPES = {"array", "boolean", "integer", "number", "object", "string"}


class CompatibilityError(ValueError):
    """Raised when the mobile consumer contract is invalid or incompatible."""


def load_consumer(mobile_repo: Path) -> dict[str, Any]:
    path = mobile_repo / CONSUMER_REL
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise CompatibilityError(f"cannot read mobile consumer contract {path}: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise CompatibilityError(f"invalid JSON in mobile consumer contract {path}: {exc}") from exc
    if not isinstance(payload, dict):
        raise CompatibilityError("mobile consumer contract root must be an object")
    validate_consumer(payload)
    return payload


def _string_list(value: Any, label: str, *, allow_empty: bool = True) -> list[str]:
    if not isinstance(value, list) or not all(isinstance(item, str) and item for item in value):
        raise CompatibilityError(f"{label} must be a list of non-empty strings")
    if not allow_empty and not value:
        raise CompatibilityError(f"{label} must not be empty")
    if len(value) != len(set(value)):
        raise CompatibilityError(f"{label} must not contain duplicates")
    return value


def _validate_consumer_fields(value: Any, label: str) -> None:
    if not isinstance(value, list):
        raise CompatibilityError(f"{label} must be a list")
    seen: set[str] = set()
    for index, entry in enumerate(value):
        if not isinstance(entry, dict):
            raise CompatibilityError(f"{label}[{index}] must be an object")
        path = entry.get("path")
        field_type = entry.get("type")
        if not isinstance(path, str) or not path:
            raise CompatibilityError(f"{label}[{index}].path must be non-empty")
        if field_type not in VALID_FIELD_TYPES:
            raise CompatibilityError(f"{label}[{index}].type is invalid")
        if "required" in entry and not isinstance(entry["required"], bool):
            raise CompatibilityError(f"{label}[{index}].required must be boolean")
        if path in seen:
            raise CompatibilityError(f"duplicate {label} path {path!r}")
        seen.add(path)


def validate_consumer(payload: dict[str, Any]) -> None:
    if payload.get("schemaVersion") != 1:
        raise CompatibilityError("consumer schemaVersion must be 1")
    contract = payload.get("coreContract")
    if not isinstance(contract, dict):
        raise CompatibilityError("coreContract must be an object")
    versions = contract.get("supportedVersions")
    if not isinstance(versions, dict):
        raise CompatibilityError("coreContract.supportedVersions must be an object")
    minimum = versions.get("minimum")
    maximum = versions.get("maximum")
    if not isinstance(minimum, int) or not isinstance(maximum, int) or minimum < 1:
        raise CompatibilityError("core contract versions must be positive integers")
    if minimum > maximum:
        raise CompatibilityError("minimum core contract version cannot exceed maximum")

    pairing = contract.get("pairing")
    if not isinstance(pairing, dict):
        raise CompatibilityError("pairing must be an object")
    for key in ("schema", "scheme", "host"):
        if not isinstance(pairing.get(key), str) or not pairing[key]:
            raise CompatibilityError(f"pairing.{key} must be a non-empty string")
    _string_list(pairing.get("requiredFields"), "pairing.requiredFields", allow_empty=False)

    push = contract.get("push")
    if not isinstance(push, dict):
        raise CompatibilityError("push must be an object")
    _string_list(push.get("requiredFields"), "push.requiredFields", allow_empty=False)
    actions = push.get("actions")
    if not isinstance(actions, list) or not actions:
        raise CompatibilityError("push.actions must be a non-empty list")
    action_values: set[str] = set()
    for index, action in enumerate(actions):
        if not isinstance(action, dict):
            raise CompatibilityError(f"push.actions[{index}] must be an object")
        value = action.get("value")
        destination = action.get("destination")
        if not isinstance(value, str) or not value:
            raise CompatibilityError(f"push.actions[{index}].value must be non-empty")
        if destination not in {"approval-detail", "finding-detail"}:
            raise CompatibilityError(f"push.actions[{index}].destination is invalid")
        if value in action_values:
            raise CompatibilityError(f"duplicate push action {value!r}")
        action_values.add(value)

    routes = payload.get("endpoints")
    if not isinstance(routes, list) or not routes:
        raise CompatibilityError("endpoints must be a non-empty list")
    route_fields = contract.get("routeFields")
    if not isinstance(route_fields, dict):
        raise CompatibilityError("coreContract.routeFields must be an object")
    route_ids: set[str] = set()
    for index, route in enumerate(routes):
        if not isinstance(route, dict):
            raise CompatibilityError(f"endpoints[{index}] must be an object")
        route_id = route.get("id")
        if not isinstance(route_id, str) or not route_id:
            raise CompatibilityError(f"endpoints[{index}].id must be non-empty")
        core_route_id = route.get("coreRouteId", route_id)
        if not isinstance(core_route_id, str) or not core_route_id:
            raise CompatibilityError(f"endpoint {route_id!r} coreRouteId must be non-empty")
        if route.get("method") not in {"DELETE", "GET", "PATCH", "POST", "PUT"}:
            raise CompatibilityError(f"endpoint {route_id!r} has invalid method")
        fields = route_fields.get(route_id)
        if not isinstance(fields, dict):
            raise CompatibilityError(
                f"coreContract.routeFields is missing endpoint {route_id!r}"
            )
        _validate_consumer_fields(fields.get("request"), f"endpoint {route_id} request fields")
        _validate_consumer_fields(fields.get("response"), f"endpoint {route_id} response fields")
        if route_id in route_ids:
            raise CompatibilityError(f"duplicate endpoint {route_id!r}")
        route_ids.add(route_id)
    stale_fields = set(route_fields) - route_ids
    if stale_fields:
        raise CompatibilityError(
            f"coreContract.routeFields has stale endpoints: {sorted(stale_fields)}"
        )


def _field_map(fields: Iterable[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    return {str(field["path"]): field for field in fields}


def compare_contracts(
    provider: dict[str, Any], consumer: dict[str, Any]
) -> list[str]:
    errors: list[str] = []
    version = provider["contract_version"]
    consumer_contract = consumer["coreContract"]
    versions = consumer_contract["supportedVersions"]
    if not versions["minimum"] <= version <= versions["maximum"]:
        errors.append(
            f"core contract version {version} is outside mobile's supported range "
            f"{versions['minimum']}..{versions['maximum']}"
        )

    provider_pairing = provider["pairing"]
    consumer_pairing = consumer_contract["pairing"]
    for key in ("schema", "scheme", "host"):
        if provider_pairing[key] != consumer_pairing[key]:
            errors.append(
                f"pairing {key} = {provider_pairing[key]!r}, mobile requires {consumer_pairing[key]!r}"
            )
    missing_pairing = set(consumer_pairing["requiredFields"]) - (
        set(provider_pairing["required_fields"]) | set(provider_pairing["optional_fields"])
    )
    if missing_pairing:
        errors.append(f"core pairing contract is missing fields: {sorted(missing_pairing)}")

    provider_push = provider["push"]
    consumer_push = consumer_contract["push"]
    missing_push_fields = set(consumer_push["requiredFields"]) - set(
        provider_push["required_fields"]
    )
    if missing_push_fields:
        errors.append(f"core push contract is missing fields: {sorted(missing_push_fields)}")
    provider_actions = {entry["value"]: entry for entry in provider_push["actions"]}
    for required_action in consumer_push["actions"]:
        actual = provider_actions.get(required_action["value"])
        if actual is None:
            errors.append(f"core push contract removed action {required_action['value']!r}")
            continue
        if actual["destination"] != required_action["destination"]:
            errors.append(
                f"push action {required_action['value']!r} destination changed from "
                f"{required_action['destination']!r} to {actual['destination']!r}"
            )

    provider_routes = {route["id"]: route for route in provider["routes"]}
    route_fields = consumer_contract["routeFields"]
    for required_route in consumer["endpoints"]:
        endpoint_id = required_route["id"]
        route_id = required_route.get("coreRouteId", endpoint_id)
        actual = provider_routes.get(route_id)
        if actual is None:
            errors.append(
                f"core contract removed mobile route {route_id!r} used by endpoint {endpoint_id!r}"
            )
            continue
        if actual["method"] != required_route["method"]:
            errors.append(
                f"route {route_id!r} method changed from {required_route['method']} "
                f"to {actual['method']}"
            )
        if actual["mobile_impact"] != "mobile-required":
            errors.append(
                f"route {route_id!r} is consumed by mobile but classified "
                f"{actual['mobile_impact']!r}"
            )
        compatible_scopes = {
            "relay:mobile:access",
            actual["required_scope"],
            *([actual["legacy_scope"]] if actual.get("legacy_scope") else []),
        }
        if "relay:mobile:access" not in compatible_scopes:
            errors.append(f"route {route_id!r} no longer accepts relay:mobile:access")
        for consumer_direction, provider_direction in (
            ("request", "request_fields"),
            ("response", "response_fields"),
        ):
            provider_fields = _field_map(actual[provider_direction])
            for field in route_fields[endpoint_id][consumer_direction]:
                provider_field = provider_fields.get(field["path"])
                if provider_field is None:
                    errors.append(
                        f"route {route_id!r} removed mobile {consumer_direction} field {field['path']!r}"
                    )
                    continue
                if provider_field["type"] != field["type"]:
                    errors.append(
                        f"route {route_id!r} field {field['path']!r} changed type from "
                        f"{field['type']} to {provider_field['type']}"
                    )
                consumer_requires = field.get("required", True)
                provider_guarantees = provider_field.get("required", True)
                if consumer_requires and not provider_guarantees:
                    errors.append(
                        f"route {route_id!r} field {field['path']!r} is no longer guaranteed"
                    )
    return errors


def git_revision(repo: Path) -> tuple[str, bool]:
    def run(*args: str) -> str:
        completed = subprocess.run(
            ["git", "-C", str(repo), *args],
            check=True,
            capture_output=True,
            text=True,
        )
        return completed.stdout.strip()

    try:
        revision = run("rev-parse", "HEAD")
        dirty = bool(run("status", "--porcelain"))
        return revision, dirty
    except (OSError, subprocess.CalledProcessError):
        return "unavailable", True


def read_candidate(mobile_repo: Path) -> dict[str, Any]:
    path = mobile_repo / "store/release-readiness.json"
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return {}
    candidate = payload.get("currentCandidate")
    return candidate if isinstance(candidate, dict) else {}


def build_evidence(
    pulse_root: Path,
    mobile_repo: Path,
    provider: dict[str, Any],
    manifest_raw: bytes,
) -> dict[str, Any]:
    pulse_revision, pulse_dirty = git_revision(pulse_root)
    mobile_revision, mobile_dirty = git_revision(mobile_repo)
    return {
        "schemaVersion": 1,
        "status": "passed",
        "generatedAt": dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat(),
        "contractVersion": provider["contract_version"],
        "contractSha256": manifest_sha256(manifest_raw),
        "pulseRevision": pulse_revision,
        "pulseDirty": pulse_dirty,
        "mobileRevision": mobile_revision,
        "mobileDirty": mobile_dirty,
        "mobileCandidate": read_candidate(mobile_repo),
        "checks": [
            "canonical-manifest-valid",
            "generated-projections-current",
            "route-method-scope-compatible",
            "request-response-fields-compatible",
            "pairing-contract-compatible",
            "push-routing-compatible",
        ],
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pulse-root", default=str(REPO_ROOT))
    parser.add_argument("--mobile-repo", required=True)
    parser.add_argument("--evidence-out", default="")
    parser.add_argument("--pretty", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    pulse_root = Path(args.pulse_root).resolve()
    mobile_repo = Path(args.mobile_repo).resolve()
    try:
        provider, raw = load_manifest(pulse_root)
        consumer = load_consumer(mobile_repo)
        errors = compare_contracts(provider, consumer)
        errors.extend(check_outputs(generated_outputs(pulse_root, mobile_repo)))
    except (CompatibilityError, ManifestError, OSError, subprocess.CalledProcessError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1

    evidence = build_evidence(pulse_root, mobile_repo, provider, raw)
    rendered = json.dumps(evidence, indent=2 if args.pretty else None, sort_keys=True) + "\n"
    if args.evidence_out:
        output = Path(args.evidence_out)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(rendered, encoding="utf-8")
    print(rendered, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
