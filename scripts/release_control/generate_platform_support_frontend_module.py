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
VALID_PRIMARY_MODES = {"api-backed", "agent-backed", "hybrid", "presentation-only"}
VALID_READINESS_STAGES = {"supported", "first-lab-ready", "presentation-only"}
VALID_STORAGE_FAMILIES = {"onprem", "container", "virtualization", "cloud"}
VALID_ONBOARDING_PATHS = {"install-workspace", "platform-connections"}
SUPPORT_FLOOR_FIELDS = (
    "setup",
    "visibility",
    "workloads",
    "storage",
    "recovery",
    "alerts",
    "assistant_read",
    "assistant_control",
)
VALID_SUPPORT_FLOOR_VALUES = {"supported", "augmentation-only", "read-only", "n/a"}
VALID_AGENT_HOST_PROFILE_GOVERNANCE_STATES = {"supported", "presentation-only"}
VALID_AGENT_HOST_PROFILE_READINESS_STAGES = {"supported", "presentation-only"}


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


def normalize_support_floor(support_floor_record: Any, label: str) -> dict[str, str]:
    record = require_dict(support_floor_record, label)
    support_floor: dict[str, str] = {}
    for field in SUPPORT_FLOOR_FIELDS:
        value = require_string(record.get(field), f"{label}.{field}")
        if value not in VALID_SUPPORT_FLOOR_VALUES:
            raise ValueError(
                f"expected {label}.{field} to be one of {sorted(VALID_SUPPORT_FLOOR_VALUES)}"
            )
        camel_field = field.split("_")[0] + "".join(part.title() for part in field.split("_")[1:])
        support_floor[camel_field] = value
    return support_floor


def normalize_manifest(raw_manifest: dict[str, Any]) -> dict[str, Any]:
    schema_version = raw_manifest.get("schema_version")
    if not isinstance(schema_version, int) or schema_version < 1:
        raise ValueError("expected schema_version to be a positive integer")

    raw_agent_host_profiles = raw_manifest.get("agent_host_profiles")
    if not isinstance(raw_agent_host_profiles, list) or not raw_agent_host_profiles:
        raise ValueError("expected agent_host_profiles to be a non-empty array")

    raw_platforms = raw_manifest.get("platforms")
    if not isinstance(raw_platforms, list) or not raw_platforms:
        raise ValueError("expected platforms to be a non-empty array")

    agent_host_profiles: list[dict[str, Any]] = []
    agent_host_profile_ids: list[str] = []
    for index, raw_profile in enumerate(raw_agent_host_profiles):
        record = require_dict(raw_profile, f"agent_host_profiles[{index}]")
        profile_id = require_lowercase_identifier(record.get("id"), f"agent_host_profiles[{index}].id")
        if profile_id in agent_host_profile_ids:
            raise ValueError(f"duplicate agent host profile id {profile_id}")

        governance_state = require_string(
            record.get("governance_state"), f"agent_host_profiles[{index}].governance_state"
        )
        if governance_state not in VALID_AGENT_HOST_PROFILE_GOVERNANCE_STATES:
            raise ValueError(
                "expected agent_host_profiles["
                f"{index}].governance_state to be one of "
                f"{sorted(VALID_AGENT_HOST_PROFILE_GOVERNANCE_STATES)}"
            )

        readiness_stage = require_string(
            record.get("readiness_stage"), f"agent_host_profiles[{index}].readiness_stage"
        )
        if readiness_stage not in VALID_AGENT_HOST_PROFILE_READINESS_STAGES:
            raise ValueError(
                "expected agent_host_profiles["
                f"{index}].readiness_stage to be one of "
                f"{sorted(VALID_AGENT_HOST_PROFILE_READINESS_STAGES)}"
            )
        if governance_state == "supported" and readiness_stage != "supported":
            raise ValueError(
                f"supported agent host profile {profile_id} must use readiness_stage supported"
            )
        if governance_state == "presentation-only" and readiness_stage != "presentation-only":
            raise ValueError(
                "presentation-only agent host profile "
                f"{profile_id} must use readiness_stage presentation-only"
            )

        host_identity_tokens = unique_preserve_order(
            [
                require_lowercase_identifier(
                    token,
                    f"agent_host_profiles[{index}].host_identity_tokens",
                )
                for token in require_string_list(
                    record.get("host_identity_tokens"),
                    f"agent_host_profiles[{index}].host_identity_tokens",
                )
            ]
        )
        if governance_state == "supported" and not host_identity_tokens:
            raise ValueError(
                f"supported agent host profile {profile_id} must declare host identity tokens"
            )
        if governance_state == "presentation-only" and host_identity_tokens:
            raise ValueError(
                f"presentation-only agent host profile {profile_id} must not declare host identity tokens"
            )

        support_floor = normalize_support_floor(
            record.get("support_floor"),
            f"agent_host_profiles[{index}].support_floor",
        )
        if governance_state == "presentation-only" and any(
            value != "n/a" for value in support_floor.values()
        ):
            raise ValueError(
                f"presentation-only agent host profile {profile_id} support floor must be n/a"
            )

        storage_family = require_string(
            record.get("storage_family"), f"agent_host_profiles[{index}].storage_family"
        )
        if storage_family not in VALID_STORAGE_FAMILIES:
            raise ValueError(
                "expected agent_host_profiles["
                f"{index}].storage_family to be one of "
                f"{sorted(VALID_STORAGE_FAMILIES)}"
            )

        agent_host_profiles.append(
            {
                "id": profile_id,
                "family": require_string(record.get("family"), f"agent_host_profiles[{index}].family"),
                "governanceState": governance_state,
                "readinessStage": readiness_stage,
                "hostIdentityTokens": host_identity_tokens,
                "supportFloor": support_floor,
                "storageFamily": storage_family,
            }
        )
        agent_host_profile_ids.append(profile_id)

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

        primary_mode = require_string(record.get("primary_mode"), f"platforms[{index}].primary_mode")
        if primary_mode not in VALID_PRIMARY_MODES:
            raise ValueError(
                f"expected platforms[{index}].primary_mode to be one of {sorted(VALID_PRIMARY_MODES)}"
            )
        readiness_stage = require_string(
            record.get("readiness_stage"), f"platforms[{index}].readiness_stage"
        )
        if readiness_stage not in VALID_READINESS_STAGES:
            raise ValueError(
                f"expected platforms[{index}].readiness_stage to be one of "
                f"{sorted(VALID_READINESS_STAGES)}"
            )
        if governance_state == "supported" and readiness_stage != "supported":
            raise ValueError(f"supported platform {platform_id} must use readiness_stage supported")
        if governance_state == "admitted" and readiness_stage == "supported":
            raise ValueError(f"admitted platform {platform_id} must not use readiness_stage supported")
        if governance_state == "presentation-only":
            if readiness_stage != "presentation-only":
                raise ValueError(
                    f"presentation-only platform {platform_id} must use readiness_stage presentation-only"
                )
            if primary_mode != "presentation-only":
                raise ValueError(
                    f"presentation-only platform {platform_id} must use primary_mode presentation-only"
                )

        canonical_projections = unique_preserve_order(
            require_string_list(
                record.get("canonical_projections"),
                f"platforms[{index}].canonical_projections",
            )
        )
        if governance_state != "presentation-only" and not canonical_projections:
            raise ValueError(f"{governance_state} platform {platform_id} must declare projections")
        if governance_state == "presentation-only" and canonical_projections:
            raise ValueError(f"presentation-only platform {platform_id} must not declare projections")

        support_floor = normalize_support_floor(
            record.get("support_floor"),
            f"platforms[{index}].support_floor",
        )
        if governance_state == "presentation-only" and any(value != "n/a" for value in support_floor.values()):
            raise ValueError(f"presentation-only platform {platform_id} support floor must be n/a")

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
                "family": require_string(record.get("family"), f"platforms[{index}].family"),
                "governanceState": governance_state,
                "readinessStage": readiness_stage,
                "primaryMode": primary_mode,
                "onboardingPaths": onboarding_paths,
                "canonicalProjections": canonical_projections,
                "supportFloor": support_floor,
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
    family_by_id = {
        platform["id"]: platform["family"] for platform in platforms
    }
    readiness_stage_by_id = {
        platform["id"]: platform["readinessStage"] for platform in platforms
    }
    agent_host_profile_family_by_id = {
        profile["id"]: profile["family"] for profile in agent_host_profiles
    }
    agent_host_profile_governance_state_by_id = {
        profile["id"]: profile["governanceState"] for profile in agent_host_profiles
    }
    agent_host_profile_readiness_stage_by_id = {
        profile["id"]: profile["readinessStage"] for profile in agent_host_profiles
    }
    agent_host_profile_host_identity_tokens_by_id = {
        profile["id"]: profile["hostIdentityTokens"] for profile in agent_host_profiles
    }
    agent_host_profile_support_floor_by_id = {
        profile["id"]: profile["supportFloor"] for profile in agent_host_profiles
    }
    agent_host_profile_storage_family_by_id = {
        profile["id"]: profile["storageFamily"] for profile in agent_host_profiles
    }
    primary_mode_by_id = {
        platform["id"]: platform["primaryMode"] for platform in platforms
    }
    canonical_projections_by_id = {
        platform["id"]: platform["canonicalProjections"] for platform in platforms
    }
    support_floor_by_id = {
        platform["id"]: platform["supportFloor"] for platform in platforms
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
    first_class_profile_ids = sorted(
        set(agent_host_profile_ids).intersection([*supported_ids, *admitted_ids])
    )
    if first_class_profile_ids:
        raise ValueError(
            "agent host profile ids must not also be supported or admitted platform ids: "
            + ", ".join(first_class_profile_ids)
        )
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
        "agentHostProfiles": agent_host_profiles,
        "agentHostProfileIds": agent_host_profile_ids,
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
        "agentHostProfileFamilyById": agent_host_profile_family_by_id,
        "agentHostProfileGovernanceStateById": agent_host_profile_governance_state_by_id,
        "agentHostProfileReadinessStageById": agent_host_profile_readiness_stage_by_id,
        "agentHostProfileHostIdentityTokensById": agent_host_profile_host_identity_tokens_by_id,
        "agentHostProfileSupportFloorById": agent_host_profile_support_floor_by_id,
        "agentHostProfileStorageFamilyById": agent_host_profile_storage_family_by_id,
        "familyById": family_by_id,
        "readinessStageById": readiness_stage_by_id,
        "primaryModeById": primary_mode_by_id,
        "canonicalProjectionsById": canonical_projections_by_id,
        "supportFloorById": support_floor_by_id,
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
                "agentHostProfiles": normalized["agentHostProfiles"],
                "platforms": normalized["platforms"],
            },
        ),
        "export const SOURCE_PLATFORM_MANIFEST_ENTRIES = PLATFORM_SUPPORT_MANIFEST.platforms;\n",
        "export const SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES =\n"
        "  PLATFORM_SUPPORT_MANIFEST.agentHostProfiles;\n",
        render_const("AGENT_HOST_PROFILE_IDS", normalized["agentHostProfileIds"]),
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
        render_const(
            "SOURCE_AGENT_HOST_PROFILE_FAMILY",
            normalized["agentHostProfileFamilyById"],
        ),
        render_const(
            "SOURCE_AGENT_HOST_PROFILE_GOVERNANCE_STATE",
            normalized["agentHostProfileGovernanceStateById"],
        ),
        render_const(
            "SOURCE_AGENT_HOST_PROFILE_READINESS_STAGE",
            normalized["agentHostProfileReadinessStageById"],
        ),
        render_const(
            "SOURCE_AGENT_HOST_PROFILE_HOST_IDENTITY_TOKENS",
            normalized["agentHostProfileHostIdentityTokensById"],
        ),
        render_const(
            "SOURCE_AGENT_HOST_PROFILE_SUPPORT_FLOOR",
            normalized["agentHostProfileSupportFloorById"],
        ),
        render_const(
            "SOURCE_AGENT_HOST_PROFILE_STORAGE_FAMILY",
            normalized["agentHostProfileStorageFamilyById"],
        ),
        render_const("SOURCE_PLATFORM_FAMILY", normalized["familyById"]),
        render_const("SOURCE_PLATFORM_READINESS_STAGE", normalized["readinessStageById"]),
        render_const("SOURCE_PLATFORM_PRIMARY_MODE", normalized["primaryModeById"]),
        render_const("SOURCE_PLATFORM_CANONICAL_PROJECTIONS", normalized["canonicalProjectionsById"]),
        render_const("SOURCE_PLATFORM_SUPPORT_FLOOR", normalized["supportFloorById"]),
        render_const("SOURCE_PLATFORM_ONBOARDING_PATHS", normalized["onboardingPathsById"]),
        render_const("SOURCE_PLATFORM_PRESENTATION", normalized["presentation"]),
        render_const("SOURCE_PLATFORM_STORAGE_FAMILY", normalized["storageFamilyById"]),
        "export type PlatformGovernanceState =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['governanceState'];\n",
        "export type PlatformReadinessStage =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['readinessStage'];\n",
        "export type PlatformPrimaryMode =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['primaryMode'];\n",
        "export type PlatformSupportFloor =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['supportFloor'];\n",
        "export type PlatformSupportFloorValue = PlatformSupportFloor[keyof PlatformSupportFloor];\n",
        "export type GeneratedSourcePlatformOnboardingPath =\n"
        "  (typeof SOURCE_PLATFORM_ONBOARDING_PATH_KEYS)[number];\n",
        "export type GeneratedAgentHostProfileId = (typeof AGENT_HOST_PROFILE_IDS)[number];\n",
        "export type GeneratedAgentHostProfileManifestEntry =\n"
        "  (typeof SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES)[number];\n",
        "export type AgentHostProfileGovernanceState =\n"
        "  (typeof SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES)[number]['governanceState'];\n",
        "export type AgentHostProfileReadinessStage =\n"
        "  (typeof SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES)[number]['readinessStage'];\n",
        "export type AgentHostProfileSupportFloor =\n"
        "  (typeof SOURCE_AGENT_HOST_PROFILE_MANIFEST_ENTRIES)[number]['supportFloor'];\n",
        "export type AgentHostProfileSupportFloorValue =\n"
        "  AgentHostProfileSupportFloor[keyof AgentHostProfileSupportFloor];\n",
        "export type SourcePlatformStorageFamily =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['storageFamily'];\n",
        "export type GeneratedPlatformType = (typeof PLATFORM_TYPE_KEYS)[number];\n",
        "export type GeneratedKnownSourcePlatform =\n"
        "  (typeof KNOWN_SOURCE_PLATFORM_KEYS)[number];\n",
        "export type GeneratedSourcePlatformManifestEntry =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number];\n",
        "export type SourcePlatformFamily =\n"
        "  (typeof SOURCE_PLATFORM_MANIFEST_ENTRIES)[number]['family'];\n",
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
