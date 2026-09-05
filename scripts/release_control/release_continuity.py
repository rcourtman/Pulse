#!/usr/bin/env python3
"""Validate and explain the public stable-release continuity identity."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any


STABLE_TAG = re.compile(r"^v[0-9]+\.[0-9]+\.[0-9]+$")
SOURCE_SHA = re.compile(r"^[0-9a-f]{40}$")
RUN_ID = re.compile(r"^[0-9]+$")
SHA256 = re.compile(r"^sha256:[0-9a-f]{64}$")
RELEASE_REFERENCE_VIOLATIONS = frozenset(
    {
        "release_payload_invalid",
        "release_id_invalid",
        "stable_tag_invalid",
        "source_identity_invalid",
    }
)
IMMUTABLE_REPLACEMENT_ACTION = (
    "Do not edit or repair the advertised release in place; restore the last known-good "
    "stable target if needed, then publish a corrected replacement through convergence."
)
ORPHAN_REPLACEMENT_ACTION = (
    "Do not advertise or repair the orphaned version in place; inspect the failed "
    "release run and publish a corrected replacement through convergence."
)


@dataclass(frozen=True)
class Violation:
    code: str
    field: str
    expected: str
    actual: Any
    action: str

    def as_dict(self) -> dict[str, Any]:
        return {
            "code": self.code,
            "field": self.field,
            "expected": self.expected,
            "actual": diagnostic_value(self.actual),
            "action": self.action,
        }


RELEASE_RULES = {
    "release_payload_invalid": (
        "GitHub did not return one release object.",
        "Inspect the releases/latest API response and API availability before retrying.",
    ),
    "release_id_invalid": (
        "The release id is absent or malformed.",
        "Do not activate the release; inspect how the release was created.",
    ),
    "stable_tag_invalid": (
        "The advertised release is not a stable vX.Y.Z tag.",
        "Restore the latest stable pointer to an exact stable release.",
    ),
    "source_identity_invalid": (
        "The release is not bound to a full lowercase source commit.",
        "Publish from an exact 40-character source commit.",
    ),
    "release_is_draft": (
        "The advertised release is still a draft.",
        "Keep drafts outside the stable channel until convergence completes.",
    ),
    "release_is_prerelease": (
        "The advertised release is marked as a prerelease.",
        "Keep prereleases outside the stable latest-release pointer.",
    ),
    "release_mutable": (
        "GitHub reports immutable=false for the advertised stable release.",
        "Publish a replacement through the immutable-release-gated pipeline; "
        "never repair the packet in place.",
    ),
    "publication_time_invalid": (
        "The advertised release has no publication timestamp.",
        "Do not treat the release as activated until GitHub reports publication.",
    ),
}


FRONTIER_RULES = {
    "frontier_payload_invalid": (
        "GitHub did not return complete stable-tag and release inventories.",
        "Inspect the matching-refs and releases API responses before retrying.",
    ),
    "stable_tag_without_release": (
        "A stable source tag newer than the advertised release has no GitHub Release packet.",
        ORPHAN_REPLACEMENT_ACTION,
    ),
    "newer_stable_release_not_advertised": (
        "A published stable release is newer than the advertised latest release.",
        "Inspect the latest-release selection and customer aliases; do not move them without "
        "an immutable activation packet and successful convergence.",
    ),
    "registry_stable_tag_beyond_latest": (
        "A public container registry exposes a stable version newer than the advertised release.",
        ORPHAN_REPLACEMENT_ACTION,
    ),
}


ACTIVATION_RULES = {
    "activation_asset_invalid": (
        "The release does not list one uploaded, digest-bound activation marker.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_asset_size_mismatch": (
        "The downloaded activation marker size differs from its release metadata.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_asset_digest_mismatch": (
        "The downloaded activation marker digest differs from its release metadata.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_payload_invalid": (
        "The activation marker is not one JSON object.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_schema_invalid": (
        "The activation marker schema is unsupported.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_tag_mismatch": (
        "The activation marker names a different release tag.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_release_mismatch": (
        "The activation marker names a different GitHub release id.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "activation_source_mismatch": (
        "The activation marker names a different source commit.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "source_run_invalid": (
        "The activation marker has no valid source release run id.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "convergence_run_invalid": (
        "The activation marker has no valid convergence run id.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "delivery_prefix_invalid": (
        "The activation marker has no customer-delivery prefix.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "server_digest_invalid": (
        "The activation marker has no valid server image digest.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "control_plane_digest_invalid": (
        "The activation marker has no valid control-plane image digest.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
    "helm_digest_invalid": (
        "The activation marker has no valid Helm chart digest.",
        IMMUTABLE_REPLACEMENT_ACTION,
    ),
}


def diagnostic_value(value: Any) -> Any:
    if value is None or isinstance(value, (bool, int, float)):
        return value
    if isinstance(value, str):
        return value if len(value) <= 160 else value[:157] + "..."
    return f"<{type(value).__name__}>"


def read_json(path: Path) -> Any:
    try:
        return parse_json(path.read_bytes(), path)
    except OSError as exc:
        raise ValueError(f"cannot read JSON from {path}: {exc}") from exc


def parse_json(content: bytes, path: Path) -> Any:
    try:
        return json.loads(content)
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read JSON from {path}: {exc}") from exc


def violation(
    code: str, field: str, expected: str, actual: Any, rules: dict[str, tuple[str, str]]
) -> Violation:
    _, action = rules[code]
    return Violation(code, field, expected, actual, action)


def release_violations(payload: Any) -> list[Violation]:
    if not isinstance(payload, dict):
        return [
            violation(
                "release_payload_invalid",
                "$",
                "object",
                payload,
                RELEASE_RULES,
            )
        ]

    failures: list[Violation] = []
    release_id = payload.get("id")
    if not isinstance(release_id, int) or isinstance(release_id, bool) or release_id <= 0:
        failures.append(
            violation("release_id_invalid", "id", "positive integer", release_id, RELEASE_RULES)
        )
    tag = payload.get("tag_name")
    if not isinstance(tag, str) or STABLE_TAG.fullmatch(tag) is None:
        failures.append(
            violation("stable_tag_invalid", "tag_name", "vX.Y.Z", tag, RELEASE_RULES)
        )
    source = payload.get("target_commitish")
    if not isinstance(source, str) or SOURCE_SHA.fullmatch(source) is None:
        failures.append(
            violation(
                "source_identity_invalid",
                "target_commitish",
                "40 lowercase hexadecimal characters",
                source,
                RELEASE_RULES,
            )
        )
    if payload.get("draft") is not False:
        failures.append(
            violation("release_is_draft", "draft", "false", payload.get("draft"), RELEASE_RULES)
        )
    if payload.get("prerelease") is not False:
        failures.append(
            violation(
                "release_is_prerelease",
                "prerelease",
                "false",
                payload.get("prerelease"),
                RELEASE_RULES,
            )
        )
    if payload.get("immutable") is not True:
        failures.append(
            violation(
                "release_mutable", "immutable", "true", payload.get("immutable"), RELEASE_RULES
            )
        )
    published_at = payload.get("published_at")
    if not isinstance(published_at, str) or not published_at:
        failures.append(
            violation(
                "publication_time_invalid",
                "published_at",
                "non-empty timestamp",
                published_at,
                RELEASE_RULES,
            )
        )
    return failures


def release_identity(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": payload["id"],
        "tag": payload["tag_name"],
        "source_sha": payload["target_commitish"],
        "draft": payload["draft"],
        "prerelease": payload["prerelease"],
        "immutable": payload["immutable"],
        "published_at": payload["published_at"],
    }


def release_is_referenceable(payload: Any, failures: list[Violation]) -> bool:
    """Return whether the release can be looked up without trusting it.

    A mutable, draft, prerelease, or incompletely published release must still
    fail continuity admission. Its validated tag, numeric release ID, and exact
    source SHA are nevertheless safe lookup data for collecting independent
    activation diagnostics. Keeping this distinction explicit prevents a
    first failure from hiding additional damage while preserving fail-closed
    delivery gates.
    """
    return isinstance(payload, dict) and not any(
        item.code in RELEASE_REFERENCE_VIOLATIONS for item in failures
    )


def stable_version(tag: str) -> tuple[int, int, int]:
    if STABLE_TAG.fullmatch(tag) is None:
        raise ValueError(f"invalid stable tag {tag!r}")
    parts = tuple(int(part) for part in tag.removeprefix("v").split("."))
    return (parts[0], parts[1], parts[2])


def inventory_list(payload: Any) -> list[Any] | None:
    """Flatten gh api's ordinary or --slurp pagination representation."""

    if not isinstance(payload, list):
        return None
    if payload and all(isinstance(page, list) for page in payload):
        return [item for page in payload for item in page]
    return payload


def frontier_violations(
    latest_release: Any,
    stable_refs_payload: Any,
    releases_payload: Any,
    registry_payloads: list[Any],
) -> tuple[list[Violation], dict[str, Any]]:
    refs = inventory_list(stable_refs_payload)
    releases = inventory_list(releases_payload)
    latest_tag = latest_release.get("tag_name") if isinstance(latest_release, dict) else None
    identity: dict[str, Any] = {
        "advertised_tag": latest_tag,
        "newer_stable_tags": "",
        "orphaned_stable_tags": "",
        "unadvertised_published_stable_tags": "",
        "registry_stable_tags_beyond_latest": "",
    }
    if (
        not isinstance(latest_tag, str)
        or STABLE_TAG.fullmatch(latest_tag) is None
        or refs is None
        or releases is None
    ):
        return (
            [
                violation(
                    "frontier_payload_invalid",
                    "$",
                    "stable latest release plus tag and release arrays",
                    {
                        "latest_release": type(latest_release).__name__,
                        "stable_refs": type(stable_refs_payload).__name__,
                        "releases": type(releases_payload).__name__,
                    },
                    FRONTIER_RULES,
                )
            ],
            identity,
        )

    stable_tags: set[str] = set()
    for ref in refs:
        if not isinstance(ref, dict):
            return (
                [
                    violation(
                        "frontier_payload_invalid",
                        "stable_refs",
                        "array of Git ref objects",
                        ref,
                        FRONTIER_RULES,
                    )
                ],
                identity,
            )
        ref_name = ref.get("ref")
        if isinstance(ref_name, str):
            tag = ref_name.removeprefix("refs/tags/")
            if STABLE_TAG.fullmatch(tag):
                stable_tags.add(tag)

    releases_by_tag: dict[str, dict[str, Any]] = {}
    for release in releases:
        if not isinstance(release, dict):
            return (
                [
                    violation(
                        "frontier_payload_invalid",
                        "releases",
                        "array of GitHub Release objects",
                        release,
                        FRONTIER_RULES,
                    )
                ],
                identity,
            )
        release_tag = release.get("tag_name")
        if isinstance(release_tag, str) and STABLE_TAG.fullmatch(release_tag):
            releases_by_tag[release_tag] = release

    latest_version = stable_version(latest_tag)
    newer_tags = sorted(
        (tag for tag in stable_tags if stable_version(tag) > latest_version),
        key=stable_version,
    )
    orphaned = [tag for tag in newer_tags if tag not in releases_by_tag]
    # Release inventory is an independent public surface. Do not make this
    # check conditional on the corresponding Git ref still appearing in the
    # matching-refs response: a published packet remains customer-visible
    # even if its source tag has subsequently been removed.
    unadvertised = sorted(
        (
            tag
            for tag, release in releases_by_tag.items()
            if stable_version(tag) > latest_version
            and release.get("draft") is False
            and isinstance(release.get("published_at"), str)
            and bool(release["published_at"])
        ),
        key=stable_version,
    )
    registry_tags: dict[str, set[str]] = {}
    for registry in registry_payloads:
        if (
            not isinstance(registry, dict)
            or not isinstance(registry.get("name"), str)
            or not isinstance(registry.get("tags"), list)
            or not all(isinstance(tag, str) for tag in registry["tags"])
        ):
            return (
                [
                    violation(
                        "frontier_payload_invalid",
                        "registries",
                        "array of registry tag-list objects",
                        registry,
                        FRONTIER_RULES,
                    )
                ],
                identity,
            )
        registry_name = registry["name"]
        for raw_tag in registry["tags"]:
            tag = raw_tag if raw_tag.startswith("v") else f"v{raw_tag}"
            if STABLE_TAG.fullmatch(tag) and stable_version(tag) > latest_version:
                registry_tags.setdefault(tag, set()).add(registry_name)
    identity.update(
        {
            "newer_stable_tags": ",".join(newer_tags),
            "orphaned_stable_tags": ",".join(orphaned),
            "unadvertised_published_stable_tags": ",".join(unadvertised),
            "registry_stable_tags_beyond_latest": ",".join(
                sorted(registry_tags, key=stable_version)
            ),
        }
    )

    failures = [
        violation(
            "stable_tag_without_release",
            f"refs/tags/{tag}",
            "one GitHub Release object",
            None,
            FRONTIER_RULES,
        )
        for tag in orphaned
    ]
    failures.extend(
        violation(
            "newer_stable_release_not_advertised",
            f"releases/{tag}",
            f"not newer than {latest_tag}",
            tag,
            FRONTIER_RULES,
        )
        for tag in unadvertised
    )
    failures.extend(
        violation(
            "registry_stable_tag_beyond_latest",
            f"registries/{tag}",
            f"no public stable container tag newer than {latest_tag}",
            ",".join(sorted(registry_tags[tag])),
            FRONTIER_RULES,
        )
        for tag in sorted(registry_tags, key=stable_version)
    )
    return failures, identity


def activation_violations(
    payload: Any, expected_release: dict[str, Any], activation_bytes: bytes
) -> list[Violation]:
    failures: list[Violation] = []
    assets = expected_release.get("assets")
    activation_assets = (
        [
            asset
            for asset in assets
            if isinstance(asset, dict) and asset.get("name") == "release-activation.json"
        ]
        if isinstance(assets, list)
        else []
    )
    if len(activation_assets) != 1:
        failures.append(
            violation(
                "activation_asset_invalid",
                "assets.release-activation.json",
                "one uploaded non-empty asset with a SHA-256 digest",
                assets,
                ACTIVATION_RULES,
            )
        )
    else:
        asset = activation_assets[0]
        asset_size = asset.get("size")
        asset_digest = asset.get("digest")
        if (
            asset.get("state") != "uploaded"
            or not isinstance(asset_size, int)
            or isinstance(asset_size, bool)
            or asset_size <= 0
            or not isinstance(asset_digest, str)
            or SHA256.fullmatch(asset_digest) is None
        ):
            failures.append(
                violation(
                    "activation_asset_invalid",
                    "assets.release-activation.json",
                    "one uploaded non-empty asset with a SHA-256 digest",
                    asset,
                    ACTIVATION_RULES,
                )
            )
        else:
            actual_size = len(activation_bytes)
            actual_digest = "sha256:" + hashlib.sha256(activation_bytes).hexdigest()
            if actual_size != asset_size:
                failures.append(
                    violation(
                        "activation_asset_size_mismatch",
                        "assets.release-activation.json.size",
                        str(asset_size),
                        actual_size,
                        ACTIVATION_RULES,
                    )
                )
            if actual_digest != asset_digest:
                failures.append(
                    violation(
                        "activation_asset_digest_mismatch",
                        "assets.release-activation.json.digest",
                        asset_digest,
                        actual_digest,
                        ACTIVATION_RULES,
                    )
                )

    if not isinstance(payload, dict):
        failures.append(
            violation(
                "activation_payload_invalid",
                "$",
                "object",
                payload,
                ACTIVATION_RULES,
            )
        )
        return failures

    checks = (
        ("activation_schema_invalid", "schema_version", 1, "integer 1"),
        (
            "activation_tag_mismatch",
            "tag",
            expected_release["tag_name"],
            expected_release["tag_name"],
        ),
        (
            "activation_release_mismatch",
            "release_id",
            str(expected_release["id"]),
            str(expected_release["id"]),
        ),
        (
            "activation_source_mismatch",
            "target_commitish",
            expected_release["target_commitish"],
            expected_release["target_commitish"],
        ),
    )
    for code, field, expected, expected_description in checks:
        actual = payload.get(field)
        if actual != expected or (field == "schema_version" and isinstance(actual, bool)):
            failures.append(
                violation(code, field, str(expected_description), actual, ACTIVATION_RULES)
            )

    for code, field in (
        ("source_run_invalid", "source_release_run_id"),
        ("convergence_run_invalid", "convergence_run_id"),
    ):
        actual = payload.get(field)
        if not isinstance(actual, str) or RUN_ID.fullmatch(actual) is None:
            failures.append(
                violation(code, field, "decimal run id string", actual, ACTIVATION_RULES)
            )

    prefix = payload.get("r2_prefix")
    if not isinstance(prefix, str) or not prefix:
        failures.append(
            violation(
                "delivery_prefix_invalid",
                "r2_prefix",
                "non-empty string",
                prefix,
                ACTIVATION_RULES,
            )
        )

    for code, field in (
        ("server_digest_invalid", "server_image_digest"),
        ("control_plane_digest_invalid", "control_plane_image_digest"),
        ("helm_digest_invalid", "helm_chart_digest"),
    ):
        actual = payload.get(field)
        if not isinstance(actual, str) or SHA256.fullmatch(actual) is None:
            failures.append(
                violation(code, field, "sha256:<64 lowercase hex>", actual, ACTIVATION_RULES)
            )
    return failures


def write_diagnostic(
    path: Path,
    check: str,
    identity: dict[str, Any],
    failures: list[Violation],
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    result = {
        "schema_version": 1,
        "check": check,
        "status": "failure" if failures else "success",
        "identity": {key: diagnostic_value(value) for key, value in identity.items()},
        "violations": [item.as_dict() for item in failures],
    }
    path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def append_outputs(path: Path, outputs: dict[str, str]) -> None:
    with path.open("a", encoding="utf-8") as handle:
        for key, value in outputs.items():
            if "\n" in value or "\r" in value:
                raise ValueError(f"output {key} contains a line break")
            handle.write(f"{key}={value}\n")


def report_failures(failures: list[Violation], rules: dict[str, tuple[str, str]]) -> None:
    for item in failures:
        message, action = rules[item.code]
        print(
            f"::error title=Stable release continuity [{item.code}]::{message} {action}",
            file=sys.stderr,
        )


def validate_release(args: argparse.Namespace) -> int:
    try:
        payload = read_json(args.release_json)
    except ValueError as exc:
        payload = None
        failures = [
            violation("release_payload_invalid", "$", "object", str(exc), RELEASE_RULES)
        ]
    else:
        failures = release_violations(payload)

    identity = release_identity(payload) if isinstance(payload, dict) and not failures else {
        key: payload.get(source) if isinstance(payload, dict) else None
        for key, source in (
            ("id", "id"),
            ("tag", "tag_name"),
            ("source_sha", "target_commitish"),
            ("draft", "draft"),
            ("prerelease", "prerelease"),
            ("immutable", "immutable"),
            ("published_at", "published_at"),
        )
    }
    write_diagnostic(args.diagnostic, "stable_release_identity", identity, failures)

    if release_is_referenceable(payload, failures):
        append_outputs(
            args.github_output,
            {
                "referenceable": "true",
                "tag": payload["tag_name"],
                "release_id": str(payload["id"]),
                "source_sha": payload["target_commitish"],
            },
        )
    if failures:
        report_failures(failures, RELEASE_RULES)
        return 1
    return 0


def validate_activation(args: argparse.Namespace) -> int:
    try:
        release = read_json(args.release_json)
        release_failures = release_violations(release)
        if not release_is_referenceable(release, release_failures):
            raise ValueError("release identity is not safe to reference")
        activation_bytes = args.activation_json.read_bytes()
        activation = parse_json(activation_bytes, args.activation_json)
    except (OSError, ValueError) as exc:
        activation = None
        failures = [
            violation("activation_payload_invalid", "$", "object", str(exc), ACTIVATION_RULES)
        ]
        identity: dict[str, Any] = {}
    else:
        failures = activation_violations(activation, release, activation_bytes)
        identity = (
            {
                key: activation.get(key)
                for key in (
                    "schema_version",
                    "tag",
                    "release_id",
                    "target_commitish",
                    "source_release_run_id",
                    "convergence_run_id",
                    "r2_prefix",
                    "server_image_digest",
                    "control_plane_image_digest",
                    "helm_chart_digest",
                )
            }
            if isinstance(activation, dict)
            else {}
        )
    write_diagnostic(args.diagnostic, "release_activation_binding", identity, failures)
    if failures:
        report_failures(failures, ACTIVATION_RULES)
        return 1

    append_outputs(
        args.github_output,
        {
            "activation_sha256": hashlib.sha256(activation_bytes).hexdigest(),
            "server_image_digest": activation["server_image_digest"],
            "control_plane_image_digest": activation["control_plane_image_digest"],
            "helm_chart_digest": activation["helm_chart_digest"],
        },
    )
    return 0


def validate_frontier(args: argparse.Namespace) -> int:
    try:
        latest_release = read_json(args.release_json)
        stable_refs = read_json(args.stable_refs_json)
        releases = read_json(args.releases_json)
        registries = [read_json(path) for path in args.registry_tags_json]
    except ValueError as exc:
        failures = [
            violation(
                "frontier_payload_invalid", "$", "valid JSON inventories", str(exc), FRONTIER_RULES
            )
        ]
        identity: dict[str, Any] = {}
    else:
        failures, identity = frontier_violations(latest_release, stable_refs, releases, registries)

    write_diagnostic(args.diagnostic, "stable_publication_frontier", identity, failures)
    if failures:
        report_failures(failures, FRONTIER_RULES)
        return 1
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)

    release = commands.add_parser("release")
    release.add_argument("--release-json", type=Path, required=True)
    release.add_argument("--diagnostic", type=Path, required=True)
    release.add_argument("--github-output", type=Path, required=True)

    activation = commands.add_parser("activation")
    activation.add_argument("--release-json", type=Path, required=True)
    activation.add_argument("--activation-json", type=Path, required=True)
    activation.add_argument("--diagnostic", type=Path, required=True)
    activation.add_argument("--github-output", type=Path, required=True)

    frontier = commands.add_parser("frontier")
    frontier.add_argument("--release-json", type=Path, required=True)
    frontier.add_argument("--stable-refs-json", type=Path, required=True)
    frontier.add_argument("--releases-json", type=Path, required=True)
    frontier.add_argument("--registry-tags-json", type=Path, action="append", required=True)
    frontier.add_argument("--diagnostic", type=Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.command == "release":
        return validate_release(args)
    if args.command == "activation":
        return validate_activation(args)
    return validate_frontier(args)


if __name__ == "__main__":
    raise SystemExit(main())
