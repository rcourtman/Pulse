#!/usr/bin/env python3
"""Generate the typed frontend platform-support projection from governance JSON."""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
MANIFEST_PATH = REPO_ROOT / "docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json"
OUTPUT_PATH = REPO_ROOT / "frontend-modern/src/utils/platformSupportManifest.generated.ts"
SOURCE_RELATIVE_PATH = "docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json"
DEFAULT_GENERIC_PRESENTATION = {
    "label": "Generic",
    "tone": "bg-surface-alt text-base-content",
}
VALID_GOVERNANCE_STATES = {"supported", "admitted", "presentation-only"}
VALID_STORAGE_FAMILIES = {"onprem", "container", "virtualization", "cloud"}
VALID_ONBOARDING_PATHS = {"install-workspace", "platform-connections"}


def require_dict(value: Any, label: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ValueError(f"expected {label} to be an object")
    return value


def require_string(value: Any, label: str) -> str:
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"expected {label} to be a non-empty string")
    return value.strip()


def require_lowercase_identifier(value: Any, label: str) -> str:
    normalized = require_string(value, label)
    if normalized != normalized.lower():
        raise ValueError(f"expected {label} to be lowercase")
    return normalized


def require_string_list(value: Any, label: str) -> list[str]:
    if not isinstance(value, list):
        raise ValueError(f"expected {label} to be an array")
    return [require_string(item, f"{label}[{index}]") for index, item in enumerate(value)]


def unique_preserve_order(values: list[str]) -> list[str]:
    seen: set[str] = set()
    ordered: list[str] = []
    for value in values:
        if value in seen:
            continue
        seen.add(value)
        ordered.append(value)
    return ordered


def normalize_manifest(raw_manifest: dict[str, Any]) -> dict[str, Any]:
    schema_version = raw_manifest.get("schema_version")
    if not isinstance(schema_version, int) or schema_version < 1:
        raise ValueError("expected schema_version to be a positive integer")

    raw_platforms = raw_manifest.get("platforms")
    if not isinstance(raw_platforms, list) or not raw_platforms:
        raise ValueError("expected platforms to be a non-empty array")

    platforms: list[dict[str, Any]] = []
    known_ids: set[str] = set()
    alias_map: dict[str, str] = {}
    platform_records: list[tuple[int, dict[str, Any], str]] = []

    for index, raw_platform in enumerate(raw_platforms):
        record = require_dict(raw_platform, f"platforms[{index}]")
        platform_id = require_lowercase_identifier(record.get("id"), f"platforms[{index}].id")
        if platform_id in known_ids:
            raise ValueError(f"duplicate platform id {platform_id}")
        known_ids.add(platform_id)
        platform_records.append((index, record, platform_id))

    all_platform_ids = set(known_ids)

    for index, record, platform_id in platform_records:
        governance_state = require_string(
            record.get("governance_state"), f"platforms[{index}].governance_state"
        )
        if governance_state not in VALID_GOVERNANCE_STATES:
            raise ValueError(
                f"expected platforms[{index}].governance_state to be one of "
                f"{sorted(VALID_GOVERNANCE_STATES)}"
            )
        storage_family = require_string(
            record.get("storage_family"), f"platforms[{index}].storage_family"
        )
        if storage_family not in VALID_STORAGE_FAMILIES:
            raise ValueError(
                f"expected platforms[{index}].storage_family to be one of "
                f"{sorted(VALID_STORAGE_FAMILIES)}"
            )
        onboarding_paths = unique_preserve_order(
            require_string_list(
                record.get("onboarding_paths"),
                f"platforms[{index}].onboarding_paths",
            )
        )
        invalid_onboarding_paths = sorted(
            path for path in onboarding_paths if path not in VALID_ONBOARDING_PATHS
        )
        if invalid_onboarding_paths:
            raise ValueError(
                f"expected platforms[{index}].onboarding_paths to contain only "
                f"{sorted(VALID_ONBOARDING_PATHS)}; got {invalid_onboarding_paths}"
            )
        if governance_state == "presentation-only" and onboarding_paths:
            raise ValueError(
                f"presentation-only platform {platform_id} must not declare onboarding paths"
            )
        if governance_state != "presentation-only" and not onboarding_paths:
            raise ValueError(
                f"{governance_state} platform {platform_id} must declare at least one onboarding path"
            )

        aliases = unique_preserve_order(
            [
                require_lowercase_identifier(alias, f"platforms[{index}].aliases")
                for alias in require_string_list(record.get("aliases"), f"platforms[{index}].aliases")
            ]
        )
        for alias in aliases:
            if alias == platform_id:
                raise ValueError(f"alias {alias} duplicates its platform id")
            if alias in all_platform_ids or alias in alias_map:
                raise ValueError(f"duplicate alias {alias}")
            alias_map[alias] = platform_id

        display_tokens = unique_preserve_order(
            require_string_list(record.get("display_tokens"), f"platforms[{index}].display_tokens")
        )

        platforms.append(
            {
                "id": platform_id,
                "governanceState": governance_state,
                "onboardingPaths": onboarding_paths,
                "uiLabel": require_string(record.get("ui_label"), f"platforms[{index}].ui_label"),
                "uiTone": require_string(record.get("ui_tone"), f"platforms[{index}].ui_tone"),
                "aliases": aliases,
                "displayTokens": display_tokens,
                "storageFamily": storage_family,
            }
        )

    default_order = unique_preserve_order(
        [
            require_lowercase_identifier(
                platform_id,
                f"default_infrastructure_source_order[{index}]",
            )
            for index, platform_id in enumerate(
                require_string_list(
                    raw_manifest.get("default_infrastructure_source_order"),
                    "default_infrastructure_source_order",
                )
            )
        ]
    )
    supported_ids = [platform["id"] for platform in platforms if platform["governanceState"] == "supported"]
    supported_id_set = set(supported_ids)
    for platform_id in default_order:
        if platform_id not in supported_id_set:
            raise ValueError(
                "default_infrastructure_source_order may only contain supported platforms; "
                f"got {platform_id}"
            )

    audit_tokens = unique_preserve_order(
        [
            token
            for platform in platforms
            for token in [platform["id"], *platform["aliases"]]
        ]
    )
    display_tokens = unique_preserve_order(
        [
            token
            for platform in platforms
            for token in [platform["uiLabel"], *platform["displayTokens"]]
        ]
    )
    presentation = {
        platform["id"]: {"label": platform["uiLabel"], "tone": platform["uiTone"]}
        for platform in platforms
    }
    onboarding_paths_by_id = {
        platform["id"]: platform["onboardingPaths"] for platform in platforms
    }
    storage_family_by_id = {
        platform["id"]: platform["storageFamily"] for platform in platforms
    }
    admitted_ids = [
        platform["id"] for platform in platforms if platform["governanceState"] == "admitted"
    ]
    presentation_only_ids = [
        platform["id"]
        for platform in platforms
        if platform["governanceState"] == "presentation-only"
    ]
    platform_type_keys = [*supported_ids, *admitted_ids]
    known_source_platform_keys = [*platform_type_keys, *presentation_only_ids, "generic"]
    onboarding_path_keys = unique_preserve_order(
        [
            path
            for platform in platforms
            for path in platform["onboardingPaths"]
        ]
    )

    return {
        "schemaVersion": schema_version,
        "defaultInfrastructureSourceOrder": default_order,
        "platforms": platforms,
        "supportedPlatformIds": supported_ids,
        "admittedPlatformIds": admitted_ids,
        "presentationOnlyPlatformIds": presentation_only_ids,
        "platformTypeKeys": platform_type_keys,
        "knownSourcePlatformKeys": known_source_platform_keys,
        "aliasMap": alias_map,
        "auditTokens": audit_tokens,
        "displayTokens": display_tokens,
        "onboardingPathKeys": onboarding_path_keys,
        "onboardingPathsById": onboarding_paths_by_id,
        "presentation": {**presentation, "generic": DEFAULT_GENERIC_PRESENTATION},
        "storageFamilyById": storage_family_by_id,
    }


def render_const(name: str, value: Any) -> str:
    rendered = json.dumps(value, indent=2, ensure_ascii=True)
    return f"export const {name} = {rendered} as const;\n"


def render_module(normalized: dict[str, Any], manifest_hash: str) -> str:
    sections = [
        "// This file is generated by scripts/release_control/generate_platform_support_frontend_module.py.\n",
        "// Do not edit by hand.\n",
        f"// Source: {SOURCE_RELATIVE_PATH}\n",
        f"// Source SHA256: {manifest_hash}\n\n",
        render_const(
            "PLATFORM_SUPPORT_MANIFEST_SOURCE",
            {
                "path": SOURCE_RELATIVE_PATH,
                "sha256": manifest_hash,
            },
        ),
        render_const(
            "PLATFORM_SUPPORT_MANIFEST",
            {
                "schemaVersion": normalized["schemaVersion"],
                "defaultInfrastructureSourceOrder": normalized["defaultInfrastructureSourceOrder"],
                "platforms": normalized["platforms"],
            },
        ),
        "export const SOURCE_PLATFORM_MANIFEST_ENTRIES = PLATFORM_SUPPORT_MANIFEST.platforms;\n",
        render_const("SUPPORTED_PLATFORM_IDS", normalized["supportedPlatformIds"]),
        render_const("ADMITTED_PLATFORM_IDS", normalized["admittedPlatformIds"]),
        render_const("PRESENTATION_ONLY_PLATFORM_IDS", normalized["presentationOnlyPlatformIds"]),
        render_const("PLATFORM_TYPE_KEYS", normalized["platformTypeKeys"]),
        render_const("KNOWN_SOURCE_PLATFORM_KEYS", normalized["knownSourcePlatformKeys"]),
        render_const(
            "DEFAULT_INFRASTRUCTURE_SOURCE_ORDER",
            normalized["defaultInfrastructureSourceOrder"],
        ),
        render_const("SOURCE_PLATFORM_ALIAS_MAP", normalized["aliasMap"]),
        render_const("SOURCE_PLATFORM_AUDIT_TOKENS", normalized["auditTokens"]),
        render_const("SOURCE_PLATFORM_DISPLAY_TOKENS", normalized["displayTokens"]),
        render_const("SOURCE_PLATFORM_ONBOARDING_PATH_KEYS", normalized["onboardingPathKeys"]),
        render_const("SOURCE_PLATFORM_ONBOARDING_PATHS", normalized["onboardingPathsById"]),
        render_const("SOURCE_PLATFORM_PRESENTATION", normalized["presentation"]),
        render_const("SOURCE_PLATFORM_STORAGE_FAMILY", normalized["storageFamilyById"]),
        "export type PlatformGovernanceState =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['governanceState'];\n",
        "export type GeneratedSourcePlatformOnboardingPath =\n"
        "  (typeof SOURCE_PLATFORM_ONBOARDING_PATH_KEYS)[number];\n",
        "export type SourcePlatformStorageFamily =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['storageFamily'];\n",
        "export type GeneratedPlatformType = (typeof PLATFORM_TYPE_KEYS)[number];\n",
        "export type GeneratedKnownSourcePlatform =\n"
        "  (typeof KNOWN_SOURCE_PLATFORM_KEYS)[number];\n",
        "export type GeneratedSourcePlatformManifestEntry =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number];\n",
    ]
    return "".join(sections)


def format_typescript(source: str) -> str:
    result = subprocess.run(
        [
            "npx",
            "prettier",
            "--stdin-filepath",
            "src/utils/platformSupportManifest.generated.ts",
        ],
        cwd=REPO_ROOT / "frontend-modern",
        input=source,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            "failed to format generated TypeScript:\n"
            f"{result.stderr.strip() or result.stdout.strip()}"
        )
    return result.stdout


def generate(check: bool) -> int:
    manifest_bytes = MANIFEST_PATH.read_bytes()
    manifest_hash = hashlib.sha256(manifest_bytes).hexdigest()
    raw_manifest = require_dict(json.loads(manifest_bytes), "manifest root")
    normalized = normalize_manifest(raw_manifest)
    expected = format_typescript(render_module(normalized, manifest_hash))

    if check:
        current = OUTPUT_PATH.read_text(encoding="utf-8") if OUTPUT_PATH.exists() else ""
        if current != expected:
            print(
                "platform support frontend projection is stale; run "
                "scripts/release_control/generate_platform_support_frontend_module.py"
            )
            return 1
        print("platform support frontend projection is up to date")
        return 0

    OUTPUT_PATH.write_text(expected, encoding="utf-8")
    print(f"wrote {OUTPUT_PATH.relative_to(REPO_ROOT)}")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail if output would change")
    return parser.parse_args()


if __name__ == "__main__":
    raise SystemExit(generate(parse_args().check))
