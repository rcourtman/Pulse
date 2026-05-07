#!/usr/bin/env python3
"""Generate the typed backend platform-support projection from governance JSON."""

from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
from pathlib import Path
from typing import Any


REPO_ROOT = Path(__file__).resolve().parents[2]
MANIFEST_PATH = REPO_ROOT / "docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json"
OUTPUT_PATH = REPO_ROOT / "internal/platformsupport/manifest_generated.go"
SOURCE_RELATIVE_PATH = "docs/release-control/v6/internal/PLATFORM_SUPPORT_MANIFEST.json"
VALID_AGENT_HOST_PROFILE_GOVERNANCE_STATES = {"supported", "presentation-only"}
VALID_AGENT_HOST_PROFILE_RUNTIME_PLATFORMS = {"linux", "macos", "windows", "freebsd"}


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

    raw_agent_host_profiles = raw_manifest.get("agent_host_profiles")
    if not isinstance(raw_agent_host_profiles, list) or not raw_agent_host_profiles:
        raise ValueError("expected agent_host_profiles to be a non-empty array")

    agent_host_profiles: list[dict[str, Any]] = []
    seen_ids: set[str] = set()
    for index, raw_profile in enumerate(raw_agent_host_profiles):
        record = require_dict(raw_profile, f"agent_host_profiles[{index}]")
        profile_id = require_lowercase_identifier(record.get("id"), f"agent_host_profiles[{index}].id")
        if profile_id in seen_ids:
            raise ValueError(f"duplicate agent host profile id {profile_id}")
        seen_ids.add(profile_id)

        governance_state = require_string(
            record.get("governance_state"), f"agent_host_profiles[{index}].governance_state"
        )
        if governance_state not in VALID_AGENT_HOST_PROFILE_GOVERNANCE_STATES:
            raise ValueError(
                "expected agent_host_profiles["
                f"{index}].governance_state to be one of "
                f"{sorted(VALID_AGENT_HOST_PROFILE_GOVERNANCE_STATES)}"
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

        runtime_platform = require_lowercase_identifier(
            record.get("runtime_platform"),
            f"agent_host_profiles[{index}].runtime_platform",
        )
        if runtime_platform not in VALID_AGENT_HOST_PROFILE_RUNTIME_PLATFORMS:
            raise ValueError(
                "expected agent_host_profiles["
                f"{index}].runtime_platform to be one of "
                f"{sorted(VALID_AGENT_HOST_PROFILE_RUNTIME_PLATFORMS)}"
            )

        agent_host_profiles.append(
            {
                "id": profile_id,
                "family": require_string(record.get("family"), f"agent_host_profiles[{index}].family"),
                "governanceState": governance_state,
                "readinessStage": require_string(
                    record.get("readiness_stage"),
                    f"agent_host_profiles[{index}].readiness_stage",
                ),
                "hostIdentityTokens": host_identity_tokens,
                "runtimePlatform": runtime_platform,
            }
        )

    return {
        "schemaVersion": schema_version,
        "agentHostProfiles": agent_host_profiles,
    }


def go_string(value: str) -> str:
    return json.dumps(value, ensure_ascii=True)


def go_string_slice(values: list[str]) -> str:
    if not values:
        return "nil"
    return "[]string{" + ", ".join(go_string(value) for value in values) + "}"


def render_module(normalized: dict[str, Any], manifest_hash: str) -> str:
    profile_entries = []
    for profile in normalized["agentHostProfiles"]:
        profile_entries.append(
            "\t{\n"
            f"\t\tID:                 {go_string(profile['id'])},\n"
            f"\t\tFamily:             {go_string(profile['family'])},\n"
            f"\t\tGovernanceState:    {go_string(profile['governanceState'])},\n"
            f"\t\tReadinessStage:     {go_string(profile['readinessStage'])},\n"
            f"\t\tHostIdentityTokens: {go_string_slice(profile['hostIdentityTokens'])},\n"
            f"\t\tRuntimePlatform:    {go_string(profile['runtimePlatform'])},\n"
            "\t},\n"
        )

    return (
        "// Code generated by scripts/release_control/generate_platform_support_backend_module.py.\n"
        "// DO NOT EDIT.\n"
        f"// Source: {SOURCE_RELATIVE_PATH}\n"
        f"// Source SHA256: {manifest_hash}\n\n"
        "package platformsupport\n\n"
        'import "strings"\n\n'
        "// ManifestSourcePath is the repo-relative path to the canonical manifest used\n"
        "// to produce this projection.\n"
        f"const ManifestSourcePath = {go_string(SOURCE_RELATIVE_PATH)}\n\n"
        "// ManifestSourceSHA256 is the sha256 of the manifest bytes used to produce\n"
        "// this projection.\n"
        f"const ManifestSourceSHA256 = {go_string(manifest_hash)}\n\n"
        "// ManifestSchemaVersion is the schema_version field of the manifest.\n"
        f"const ManifestSchemaVersion = {normalized['schemaVersion']}\n\n"
        "// AgentHostProfileEntry is the generated projection of an agent host profile.\n"
        "type AgentHostProfileEntry struct {\n"
        "\tID                 string\n"
        "\tFamily             string\n"
        "\tGovernanceState    string\n"
        "\tReadinessStage     string\n"
        "\tHostIdentityTokens []string\n"
        "\tRuntimePlatform    string\n"
        "}\n\n"
        "// agentHostProfileEntries is the generated agent host profile manifest projection.\n"
        "var agentHostProfileEntries = []AgentHostProfileEntry{\n"
        + "".join(profile_entries)
        + "}\n\n"
        "func AgentHostProfiles() []AgentHostProfileEntry {\n"
        "\tout := make([]AgentHostProfileEntry, len(agentHostProfileEntries))\n"
        "\tfor index, profile := range agentHostProfileEntries {\n"
        "\t\tout[index] = cloneAgentHostProfile(profile)\n"
        "\t}\n"
        "\treturn out\n"
        "}\n\n"
        "func AgentHostProfileByID(value string) (AgentHostProfileEntry, bool) {\n"
        "\tnormalized := normalizeIdentityToken(value)\n"
        "\tif normalized == \"\" {\n"
        "\t\treturn AgentHostProfileEntry{}, false\n"
        "\t}\n"
        "\tfor _, profile := range agentHostProfileEntries {\n"
        "\t\tif profile.ID == normalized {\n"
        "\t\t\treturn cloneAgentHostProfile(profile), true\n"
        "\t\t}\n"
        "\t}\n"
        "\treturn AgentHostProfileEntry{}, false\n"
        "}\n\n"
        "func AgentHostProfileForIdentity(values ...string) (AgentHostProfileEntry, bool) {\n"
        "\tfor _, value := range values {\n"
        "\t\tfor _, profile := range agentHostProfileEntries {\n"
        "\t\t\tif agentHostProfileMatchesIdentity(profile, value) {\n"
        "\t\t\t\treturn cloneAgentHostProfile(profile), true\n"
        "\t\t\t}\n"
        "\t\t}\n"
        "\t}\n"
        "\treturn AgentHostProfileEntry{}, false\n"
        "}\n\n"
        "func RuntimePlatformForHostIdentityToken(value string) string {\n"
        "\tprofile, ok := AgentHostProfileForIdentity(value)\n"
        "\tif !ok {\n"
        "\t\treturn \"\"\n"
        "\t}\n"
        "\treturn profile.RuntimePlatform\n"
        "}\n\n"
        "func NormalizeRuntimePlatformForAgentHostProfile(profileID, platform string) string {\n"
        "\treportedPlatform := strings.TrimSpace(platform)\n"
        "\tprofile, ok := AgentHostProfileByID(profileID)\n"
        "\tif !ok || strings.TrimSpace(profile.RuntimePlatform) == \"\" {\n"
        "\t\treturn reportedPlatform\n"
        "\t}\n"
        "\tif reportedPlatform == \"\" || agentHostProfileMatchesIdentity(profile, reportedPlatform) {\n"
        "\t\treturn profile.RuntimePlatform\n"
        "\t}\n"
        "\treturn reportedPlatform\n"
        "}\n\n"
        "func (profile AgentHostProfileEntry) MatchesIdentity(value string) bool {\n"
        "\treturn agentHostProfileMatchesIdentity(profile, value)\n"
        "}\n\n"
        "func agentHostProfileMatchesIdentity(profile AgentHostProfileEntry, value string) bool {\n"
        "\tnormalized := normalizeIdentityToken(value)\n"
        "\tif normalized == \"\" {\n"
        "\t\treturn false\n"
        "\t}\n"
        "\tif profile.ID == normalized {\n"
        "\t\treturn true\n"
        "\t}\n"
        "\tfor _, token := range profile.HostIdentityTokens {\n"
        "\t\tif normalizeIdentityToken(token) == normalized {\n"
        "\t\t\treturn true\n"
        "\t\t}\n"
        "\t}\n"
        "\treturn false\n"
        "}\n\n"
        "func cloneAgentHostProfile(profile AgentHostProfileEntry) AgentHostProfileEntry {\n"
        "\tprofile.HostIdentityTokens = append([]string(nil), profile.HostIdentityTokens...)\n"
        "\treturn profile\n"
        "}\n\n"
        "func normalizeIdentityToken(value string) string {\n"
        "\treturn strings.ToLower(strings.TrimSpace(value))\n"
        "}\n"
    )


def format_go(source: str) -> str:
    result = subprocess.run(
        ["gofmt"],
        input=source,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(
            "failed to format generated Go:\n"
            f"{result.stderr.strip() or result.stdout.strip()}"
        )
    return result.stdout


def generate(check: bool) -> int:
    manifest_bytes = MANIFEST_PATH.read_bytes()
    manifest_hash = hashlib.sha256(manifest_bytes).hexdigest()
    raw_manifest = require_dict(json.loads(manifest_bytes), "manifest root")
    normalized = normalize_manifest(raw_manifest)
    expected = format_go(render_module(normalized, manifest_hash))

    if check:
        current = OUTPUT_PATH.read_text(encoding="utf-8") if OUTPUT_PATH.exists() else ""
        if current != expected:
            print(
                "platform support backend projection is stale; run "
                "scripts/release_control/generate_platform_support_backend_module.py"
            )
            return 1
        print("platform support backend projection is up to date")
        return 0

    OUTPUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    OUTPUT_PATH.write_text(expected, encoding="utf-8")
    print(f"wrote {OUTPUT_PATH.relative_to(REPO_ROOT)}")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail if output would change")
    return parser.parse_args()


if __name__ == "__main__":
    raise SystemExit(generate(parse_args().check))
