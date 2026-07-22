#!/usr/bin/env python3
"""Generate runtime and mobile projections of the Pulse Mobile contract."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
MANIFEST_REL = Path("docs/release-control/v6/internal/MOBILE_COMPATIBILITY_MANIFEST.json")
API_OUTPUT_REL = Path("internal/api/relay_mobile_capability_generated.go")
RELAY_OUTPUT_REL = Path("internal/relay/mobile_compatibility_generated.go")
MOBILE_OUTPUT_REL = Path("src/generated/coreCompatibility.ts")

METHOD_SYMBOLS = {
    "DELETE": "http.MethodDelete",
    "GET": "http.MethodGet",
    "PATCH": "http.MethodPatch",
    "POST": "http.MethodPost",
    "PUT": "http.MethodPut",
}

SCOPE_SYMBOLS = {
    "actions:approve": "config.ScopeActionsApprove",
    "actions:execute": "config.ScopeActionsExecute",
    "ai:chat": "config.ScopeAIChat",
    "ai:execute": "config.ScopeAIExecute",
    "monitoring:read": "config.ScopeMonitoringRead",
    "monitoring:write": "config.ScopeMonitoringWrite",
    "settings:read": "config.ScopeSettingsRead",
}

VALID_FIELD_TYPES = {"array", "boolean", "integer", "number", "object", "string"}
VALID_LIFECYCLES = {"compatibility", "current"}
VALID_IMPACTS = {"desktop-only", "mobile-handoff-only", "mobile-required"}
PLACEHOLDER_RE = re.compile(r"\{([a-z][a-z0-9_]*)\}")
ROUTE_ID_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
GO_SYMBOL_RE = re.compile(r"^[A-Za-z][A-Za-z0-9]*$")


class ManifestError(ValueError):
    """Raised when the canonical manifest is internally inconsistent."""


def load_manifest(pulse_root: Path) -> tuple[dict[str, Any], bytes]:
    path = pulse_root / MANIFEST_REL
    raw = path.read_bytes()
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ManifestError(f"invalid JSON in {path}: {exc}") from exc
    if not isinstance(payload, dict):
        raise ManifestError("mobile compatibility manifest root must be an object")
    validate_manifest(payload)
    return payload, raw


def _require_non_empty_string(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ManifestError(f"{label} must be a non-empty string")
    return value


def _reject_unknown_keys(
    value: dict[str, Any], allowed: set[str], label: str
) -> None:
    unknown = set(value) - allowed
    if unknown:
        raise ManifestError(f"{label} has unknown keys: {sorted(unknown)}")


def _validate_symbol_values(
    values: Any, label: str, *, extra_keys: set[str] | None = None
) -> None:
    if not isinstance(values, list) or not values:
        raise ManifestError(f"{label} must be a non-empty list")
    seen_values: set[str] = set()
    seen_symbols: set[str] = set()
    for index, entry in enumerate(values):
        if not isinstance(entry, dict):
            raise ManifestError(f"{label}[{index}] must be an object")
        _reject_unknown_keys(
            entry,
            {"value", "go_symbol"} | (extra_keys or set()),
            f"{label}[{index}]",
        )
        value = _require_non_empty_string(entry.get("value"), f"{label}[{index}].value")
        symbol = _require_non_empty_string(
            entry.get("go_symbol"), f"{label}[{index}].go_symbol"
        )
        if not GO_SYMBOL_RE.fullmatch(symbol):
            raise ManifestError(f"{label}[{index}].go_symbol is invalid")
        if value in seen_values:
            raise ManifestError(f"duplicate {label} value {value!r}")
        if symbol in seen_symbols:
            raise ManifestError(f"duplicate {label} Go symbol {symbol!r}")
        seen_values.add(value)
        seen_symbols.add(symbol)


def _validate_fields(fields: Any, label: str) -> None:
    if not isinstance(fields, list):
        raise ManifestError(f"{label} must be a list")
    seen: set[str] = set()
    for index, field in enumerate(fields):
        if not isinstance(field, dict):
            raise ManifestError(f"{label}[{index}] must be an object")
        _reject_unknown_keys(field, {"path", "type", "required"}, f"{label}[{index}]")
        path = _require_non_empty_string(field.get("path"), f"{label}[{index}].path")
        field_type = _require_non_empty_string(
            field.get("type"), f"{label}[{index}].type"
        )
        if field_type not in VALID_FIELD_TYPES:
            raise ManifestError(f"{label}[{index}] has unknown type {field_type!r}")
        if "required" in field and not isinstance(field["required"], bool):
            raise ManifestError(f"{label}[{index}].required must be boolean")
        if path in seen:
            raise ManifestError(f"duplicate field {path!r} in {label}")
        seen.add(path)


def validate_manifest(payload: dict[str, Any]) -> None:
    _reject_unknown_keys(
        payload,
        {
            "$schema",
            "schema_version",
            "contract_version",
            "mobile_role",
            "surface_classes",
            "pairing",
            "push",
            "routes",
        },
        "manifest",
    )
    if payload.get("schema_version") != 1:
        raise ManifestError("schema_version must be 1")
    if not isinstance(payload.get("contract_version"), int) or payload["contract_version"] < 1:
        raise ManifestError("contract_version must be a positive integer")
    _require_non_empty_string(payload.get("mobile_role"), "mobile_role")

    classes = payload.get("surface_classes")
    if not isinstance(classes, list):
        raise ManifestError("surface_classes must be a list")
    for index, entry in enumerate(classes):
        if not isinstance(entry, dict):
            raise ManifestError(f"surface_classes[{index}] must be an object")
        _reject_unknown_keys(
            entry, {"id", "description"}, f"surface_classes[{index}]"
        )
        _require_non_empty_string(
            entry.get("description"), f"surface_classes[{index}].description"
        )
    class_ids = [entry.get("id") for entry in classes if isinstance(entry, dict)]
    if set(class_ids) != VALID_IMPACTS or len(class_ids) != len(VALID_IMPACTS):
        raise ManifestError(
            "surface_classes must define mobile-required, mobile-handoff-only, and desktop-only exactly once"
        )

    pairing = payload.get("pairing")
    if not isinstance(pairing, dict):
        raise ManifestError("pairing must be an object")
    _reject_unknown_keys(
        pairing,
        {"schema", "scheme", "host", "required_fields", "optional_fields"},
        "pairing",
    )
    for key in ("schema", "scheme", "host"):
        _require_non_empty_string(pairing.get(key), f"pairing.{key}")
    required_pairing = pairing.get("required_fields")
    optional_pairing = pairing.get("optional_fields")
    if not isinstance(required_pairing, list) or not all(
        isinstance(value, str) and value for value in required_pairing
    ):
        raise ManifestError("pairing.required_fields must be a string list")
    if not isinstance(optional_pairing, list) or not all(
        isinstance(value, str) and value for value in optional_pairing
    ):
        raise ManifestError("pairing.optional_fields must be a string list")
    if len(required_pairing) != len(set(required_pairing)):
        raise ManifestError("pairing.required_fields must not contain duplicates")
    if len(optional_pairing) != len(set(optional_pairing)):
        raise ManifestError("pairing.optional_fields must not contain duplicates")
    overlap = set(required_pairing) & set(optional_pairing)
    if overlap:
        raise ManifestError(f"pairing fields cannot be both required and optional: {sorted(overlap)}")

    push = payload.get("push")
    if not isinstance(push, dict):
        raise ManifestError("push must be an object")
    _reject_unknown_keys(
        push, {"required_fields", "types", "priorities", "actions"}, "push"
    )
    required_push_fields = push.get("required_fields")
    if not isinstance(required_push_fields, list) or not all(
        isinstance(value, str) and value for value in required_push_fields
    ):
        raise ManifestError("push.required_fields must be a string list")
    if len(required_push_fields) != len(set(required_push_fields)):
        raise ManifestError("push.required_fields must not contain duplicates")
    _validate_symbol_values(push.get("types"), "push.types")
    _validate_symbol_values(push.get("priorities"), "push.priorities")
    _validate_symbol_values(
        push.get("actions"),
        "push.actions",
        extra_keys={"destination", "lifecycle"},
    )
    for index, action in enumerate(push["actions"]):
        if action.get("lifecycle") not in VALID_LIFECYCLES:
            raise ManifestError(f"push.actions[{index}] has invalid lifecycle")
        if action.get("destination") not in {"approval-detail", "finding-detail"}:
            raise ManifestError(f"push.actions[{index}] has invalid destination")

    routes = payload.get("routes")
    if not isinstance(routes, list) or not routes:
        raise ManifestError("routes must be a non-empty list")
    seen_ids: set[str] = set()
    seen_symbols: set[str] = set()
    seen_method_paths: set[tuple[str, str]] = set()
    for index, route in enumerate(routes):
        if not isinstance(route, dict):
            raise ManifestError(f"routes[{index}] must be an object")
        _reject_unknown_keys(
            route,
            {
                "id",
                "go_symbol",
                "method",
                "path",
                "required_scope",
                "legacy_scope",
                "lifecycle",
                "mobile_impact",
                "implementation_paths",
                "request_fields",
                "response_fields",
            },
            f"routes[{index}]",
        )
        route_id = _require_non_empty_string(route.get("id"), f"routes[{index}].id")
        symbol = _require_non_empty_string(
            route.get("go_symbol"), f"routes[{index}].go_symbol"
        )
        method = _require_non_empty_string(route.get("method"), f"routes[{index}].method")
        path = _require_non_empty_string(route.get("path"), f"routes[{index}].path")
        scope = _require_non_empty_string(
            route.get("required_scope"), f"routes[{index}].required_scope"
        )
        if not ROUTE_ID_RE.fullmatch(route_id):
            raise ManifestError(f"route id {route_id!r} is invalid")
        if not GO_SYMBOL_RE.fullmatch(symbol):
            raise ManifestError(f"route {route_id!r} Go symbol is invalid")
        if method not in METHOD_SYMBOLS:
            raise ManifestError(f"route {route_id!r} has unsupported method {method!r}")
        if scope not in SCOPE_SYMBOLS:
            raise ManifestError(f"route {route_id!r} has unsupported scope {scope!r}")
        legacy_scope = route.get("legacy_scope")
        if legacy_scope is not None and legacy_scope not in SCOPE_SYMBOLS:
            raise ManifestError(
                f"route {route_id!r} has unsupported legacy scope {legacy_scope!r}"
            )
        if route.get("lifecycle") not in VALID_LIFECYCLES:
            raise ManifestError(f"route {route_id!r} has invalid lifecycle")
        if route.get("mobile_impact") not in VALID_IMPACTS:
            raise ManifestError(f"route {route_id!r} has invalid mobile_impact")
        implementation_paths = route.get("implementation_paths")
        if (
            not isinstance(implementation_paths, list)
            or not implementation_paths
            or not all(
                isinstance(implementation_path, str) and implementation_path
                for implementation_path in implementation_paths
            )
        ):
            raise ManifestError(f"route {route_id!r} must name implementation_paths")
        if len(implementation_paths) != len(set(implementation_paths)):
            raise ManifestError(f"route {route_id!r} has duplicate implementation_paths")
        _validate_fields(route.get("request_fields"), f"route {route_id} request_fields")
        _validate_fields(route.get("response_fields"), f"route {route_id} response_fields")
        if route_id in seen_ids:
            raise ManifestError(f"duplicate route id {route_id!r}")
        if symbol in seen_symbols:
            raise ManifestError(f"duplicate route Go symbol {symbol!r}")
        if (method, path) in seen_method_paths:
            raise ManifestError(f"duplicate route {method} {path}")
        if not path.startswith("/api/"):
            raise ManifestError(f"route {route_id!r} path must start with /api/")
        placeholders = PLACEHOLDER_RE.findall(path)
        if len(placeholders) != len(set(placeholders)):
            raise ManifestError(f"route {route_id!r} repeats a path placeholder")
        seen_ids.add(route_id)
        seen_symbols.add(symbol)
        seen_method_paths.add((method, path))


def manifest_sha256(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def render_api_go(payload: dict[str, Any], digest: str) -> str:
    routes = payload["routes"]
    lines = [
        "// Code generated by scripts/release_control/generate_mobile_compatibility.py. DO NOT EDIT.",
        f"// Source SHA256: {digest}",
        "",
        "package api",
        "",
        "import (",
        '\t"net/http"',
        "",
        '\t"github.com/rcourtman/pulse-go-rewrite/internal/config"',
        ")",
        "",
        f'const onboardingSchemaVersion = {json.dumps(payload["pairing"]["schema"])}',
        "",
        "const (",
    ]
    for route in routes:
        lines.append(
            f'\t{route["go_symbol"]} relayMobileRuntimeRouteID = {json.dumps(route["id"])}'
        )
    lines.extend([")", "", "var relayMobileRuntimeRouteOrder = []relayMobileRuntimeRouteID{"])
    lines.extend(f'\t{route["go_symbol"]},' for route in routes)
    lines.extend(["}", "", "var relayMobileRuntimeRouteSpecs = map[relayMobileRuntimeRouteID]relayMobileRuntimeRouteSpec{"])
    for route in routes:
        lines.extend(
            [
                f'\t{route["go_symbol"]}: {{',
                f'\t\tid: {route["go_symbol"]},',
                f'\t\tmethod: {METHOD_SYMBOLS[route["method"]]},',
                f'\t\tpath: {json.dumps(route["path"])},',
                f'\t\trequiredScope: {SCOPE_SYMBOLS[route["required_scope"]]},',
            ]
        )
        if route.get("legacy_scope"):
            lines.append(f'\t\tlegacyScope: {SCOPE_SYMBOLS[route["legacy_scope"]]},')
        lines.append("\t},")
    lines.extend(["}", ""])
    return "\n".join(lines)


def render_relay_go(payload: dict[str, Any], digest: str) -> str:
    push = payload["push"]
    lines = [
        "// Code generated by scripts/release_control/generate_mobile_compatibility.py. DO NOT EDIT.",
        f"// Source SHA256: {digest}",
        "",
        "package relay",
        "",
        "// Notification type constants consumed by Pulse Mobile.",
        "const (",
    ]
    lines.extend(f'\t{entry["go_symbol"]} = {json.dumps(entry["value"])}' for entry in push["types"])
    lines.extend([")", "", "// Priority constants consumed by Pulse Mobile.", "const ("])
    lines.extend(
        f'\t{entry["go_symbol"]} = {json.dumps(entry["value"])}'
        for entry in push["priorities"]
    )
    lines.extend([")", "", "// Action type constants consumed by Pulse Mobile.", "const ("])
    lines.extend(
        f'\t{entry["go_symbol"]} = {json.dumps(entry["value"])}' for entry in push["actions"]
    )
    lines.extend([")", ""])
    return "\n".join(lines)


def render_mobile_typescript(payload: dict[str, Any], digest: str) -> str:
    routes = {
        route["id"]: {
            "method": route["method"],
            "path": route["path"],
            "requiredScope": route["required_scope"],
            "compatibleScopes": list(
                dict.fromkeys(
                    [
                        "relay:mobile:access",
                        route["required_scope"],
                        *([route["legacy_scope"]] if route.get("legacy_scope") else []),
                    ]
                )
            ),
            "lifecycle": route["lifecycle"],
            "mobileImpact": route["mobile_impact"],
        }
        for route in payload["routes"]
    }
    push_actions = {
        action["value"]: {
            "value": action["value"],
            "destination": action["destination"],
            "lifecycle": action["lifecycle"],
        }
        for action in payload["push"]["actions"]
    }
    return "\n".join(
        [
            "// This file is generated by Pulse's mobile compatibility generator.",
            "// Do not edit it in pulse-mobile; update MOBILE_COMPATIBILITY_MANIFEST.json in Pulse.",
            f"// Source SHA256: {digest}",
            "",
            f"export const CORE_MOBILE_CONTRACT_VERSION = {payload['contract_version']} as const;",
            f"export const CORE_MOBILE_CONTRACT_SHA256 = {json.dumps(digest)} as const;",
            "",
            "export const CORE_MOBILE_PAIRING = "
            + json.dumps(payload["pairing"], indent=2, ensure_ascii=False)
            + " as const;",
            "",
            "export const CORE_MOBILE_ROUTES = "
            + json.dumps(routes, indent=2, ensure_ascii=False)
            + " as const;",
            "",
            "export type CoreMobileRouteId = keyof typeof CORE_MOBILE_ROUTES;",
            "",
            "export const CORE_MOBILE_PUSH = "
            + json.dumps(
                {
                    "requiredFields": payload["push"]["required_fields"],
                    "types": [entry["value"] for entry in payload["push"]["types"]],
                    "priorities": [
                        entry["value"] for entry in payload["push"]["priorities"]
                    ],
                    "actions": push_actions,
                },
                indent=2,
                ensure_ascii=False,
            )
            + " as const;",
            "",
            "export type CoreMobilePushAction = keyof typeof CORE_MOBILE_PUSH.actions;",
            "",
        ]
    )


def format_go(source: str) -> str:
    completed = subprocess.run(
        ["gofmt"],
        input=source,
        check=True,
        capture_output=True,
        text=True,
    )
    return completed.stdout


def generated_outputs(
    pulse_root: Path, mobile_repo: Path | None
) -> dict[Path, str]:
    payload, raw = load_manifest(pulse_root)
    digest = manifest_sha256(raw)
    outputs = {
        pulse_root / API_OUTPUT_REL: format_go(render_api_go(payload, digest)),
        pulse_root / RELAY_OUTPUT_REL: format_go(render_relay_go(payload, digest)),
    }
    if mobile_repo is not None:
        outputs[mobile_repo / MOBILE_OUTPUT_REL] = render_mobile_typescript(payload, digest)
    return outputs


def write_outputs(outputs: dict[Path, str]) -> None:
    for path, content in outputs.items():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")


def check_outputs(outputs: dict[Path, str]) -> list[str]:
    errors: list[str] = []
    for path, expected in outputs.items():
        try:
            actual = path.read_text(encoding="utf-8")
        except OSError:
            errors.append(f"missing generated projection: {path}")
            continue
        if actual != expected:
            errors.append(f"generated projection is stale: {path}")
    return errors


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pulse-root", default=str(REPO_ROOT))
    parser.add_argument("--mobile-repo", default="")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--write", action="store_true")
    mode.add_argument("--check", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    pulse_root = Path(args.pulse_root).resolve()
    mobile_repo = Path(args.mobile_repo).resolve() if args.mobile_repo else None
    try:
        outputs = generated_outputs(pulse_root, mobile_repo)
        if args.write:
            write_outputs(outputs)
            print(f"[OK] Wrote {len(outputs)} mobile compatibility projections")
            return 0
        errors = check_outputs(outputs)
    except (ManifestError, OSError, subprocess.CalledProcessError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1
    if errors:
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        print(
            "Run scripts/release_control/generate_mobile_compatibility.py --write "
            "--mobile-repo ../pulse-mobile",
            file=sys.stderr,
        )
        return 1
    print(f"[OK] {len(outputs)} mobile compatibility projections are current")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
