#!/usr/bin/env python3
"""Create and verify immutable Pulse release-candidate manifests."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path, PurePosixPath
from typing import Any


SCHEMA_VERSION = 1
VERSION_PATTERN = re.compile(
    r"^[0-9]+\.[0-9]+\.[0-9]+(?:-(?:rc|alpha|beta)\.[0-9]+)?$"
)
RELEASE_NOTE_VISUAL_PATTERN = re.compile(
    r"^release-note-[a-z0-9]+(?:-[a-z0-9]+)*-(?:before|now)\.png$"
)
MAX_RELEASE_NOTE_VISUAL_ASSETS = 20
RELEASE_NOTE_VISUAL_URL_PATTERN = re.compile(
    r"https://github\.com/[^/\s)]+/[^/\s)]+/releases/download/"
    r"(?P<tag>v[0-9]+\.[0-9]+\.[0-9]+(?:-(?:rc|alpha|beta)\.[0-9]+)?)/"
    r"(?P<name>release-note-[a-z0-9]+(?:-[a-z0-9]+)*-(?:before|now)\.png)"
)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def collect_assets(release_dir: Path) -> list[dict[str, Any]]:
    if not release_dir.is_dir():
        raise ValueError(f"release directory does not exist: {release_dir}")

    assets: list[dict[str, Any]] = []
    for path in sorted(release_dir.rglob("*")):
        if path.is_symlink():
            raise ValueError(f"release candidate must not contain symlinks: {path}")
        if not path.is_file():
            continue
        relative = path.relative_to(release_dir).as_posix()
        assets.append(
            {
                "name": relative,
                "size": path.stat().st_size,
                "sha256": sha256_file(path),
            }
        )

    if not assets:
        raise ValueError(f"release candidate is empty: {release_dir}")
    return assets


def validate_version(version: str) -> None:
    if not VERSION_PATTERN.fullmatch(version):
        raise ValueError(f"invalid release version: {version!r}")


def create_manifest(release_dir: Path, version: str, source_sha: str) -> dict[str, Any]:
    validate_version(version)
    if not re.fullmatch(r"[0-9a-f]{40}", source_sha):
        raise ValueError("source SHA must be a full lowercase Git commit SHA")
    return {
        "schema_version": SCHEMA_VERSION,
        "version": version,
        "tag": f"v{version}",
        "source_sha": source_sha,
        "assets": collect_assets(release_dir),
    }


def load_manifest(path: Path) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read release candidate manifest {path}: {exc}") from exc
    if not isinstance(payload, dict):
        raise ValueError("release candidate manifest must be a JSON object")
    if payload.get("schema_version") != SCHEMA_VERSION:
        raise ValueError(
            f"unsupported release candidate manifest schema: {payload.get('schema_version')!r}"
        )
    if not isinstance(payload.get("assets"), list) or not payload["assets"]:
        raise ValueError("release candidate manifest must contain assets")
    return payload


def manifest_assets_by_name(manifest: dict[str, Any]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for index, asset in enumerate(manifest["assets"]):
        if not isinstance(asset, dict):
            raise ValueError(f"manifest asset {index} must be an object")
        name = asset.get("name")
        size = asset.get("size")
        digest = asset.get("sha256")
        if not isinstance(name, str) or not name:
            raise ValueError(f"manifest asset {index} has invalid name: {name!r}")
        relative_name = PurePosixPath(name)
        if (
            relative_name.is_absolute()
            or relative_name.as_posix() != name
            or "\\" in name
            or any(part in {"", ".", ".."} for part in relative_name.parts)
        ):
            raise ValueError(f"manifest asset {index} has invalid name: {name!r}")
        if not isinstance(size, int) or size < 0:
            raise ValueError(f"manifest asset {name!r} has invalid size: {size!r}")
        if not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise ValueError(f"manifest asset {name!r} has invalid SHA-256 digest")
        if name in result:
            raise ValueError(f"manifest contains duplicate asset: {name}")
        result[name] = asset
    return result


def verify_manifest_identity(
    manifest: dict[str, Any], expected_version: str, expected_source_sha: str
) -> None:
    if manifest.get("version") != expected_version:
        raise ValueError(
            f"candidate version {manifest.get('version')!r} does not match {expected_version!r}"
        )
    if manifest.get("tag") != f"v{expected_version}":
        raise ValueError(f"candidate tag does not match v{expected_version}")
    if manifest.get("source_sha") != expected_source_sha:
        raise ValueError(
            f"candidate source SHA {manifest.get('source_sha')!r} does not match "
            f"{expected_source_sha!r}"
        )


def verify_local(
    release_dir: Path,
    manifest: dict[str, Any],
    expected_version: str,
    expected_source_sha: str,
) -> None:
    verify_manifest_identity(manifest, expected_version, expected_source_sha)

    expected = manifest_assets_by_name(manifest)
    actual = {asset["name"]: asset for asset in collect_assets(release_dir)}
    if set(actual) != set(expected):
        missing = sorted(set(expected) - set(actual))
        extra = sorted(set(actual) - set(expected))
        raise ValueError(f"candidate asset set mismatch: missing={missing}, extra={extra}")
    for name, expected_asset in expected.items():
        actual_asset = actual[name]
        if actual_asset["size"] != expected_asset["size"]:
            raise ValueError(f"candidate asset size mismatch: {name}")
        if actual_asset["sha256"] != expected_asset["sha256"]:
            raise ValueError(f"candidate asset digest mismatch: {name}")


def load_release_assets(path: Path) -> list[dict[str, Any]]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError(f"cannot read release asset metadata {path}: {exc}") from exc
    if not isinstance(payload, list):
        raise ValueError("release asset metadata must be a JSON array")
    if payload and all(isinstance(item, list) for item in payload):
        payload = [asset for page in payload for asset in page]
    if not all(isinstance(item, dict) for item in payload):
        raise ValueError("release asset metadata contains a non-object entry")
    return payload


def release_note_visual_assets(body: str, expected_tag: str) -> set[str]:
    assets: set[str] = set()
    for match in RELEASE_NOTE_VISUAL_URL_PATTERN.finditer(body):
        if match.group("tag") == expected_tag:
            assets.add(match.group("name"))
    if len(assets) > MAX_RELEASE_NOTE_VISUAL_ASSETS:
        raise ValueError(
            "release body references too many release-note visual sidecars: "
            f"{len(assets)} > {MAX_RELEASE_NOTE_VISUAL_ASSETS}"
        )
    return assets


def verify_release(
    manifest: dict[str, Any],
    release_assets: list[dict[str, Any]],
    auxiliary_assets: set[str] | None = None,
) -> None:
    expected = manifest_assets_by_name(manifest)
    expected_auxiliary = set(auxiliary_assets or ())
    overlap = sorted(set(expected) & expected_auxiliary)
    if overlap:
        raise ValueError(f"auxiliary assets overlap candidate manifest: {overlap}")
    invalid_auxiliary = sorted(
        name
        for name in expected_auxiliary
        if not RELEASE_NOTE_VISUAL_PATTERN.fullmatch(name)
    )
    if invalid_auxiliary:
        raise ValueError(
            f"invalid release-note visual sidecar name(s): {invalid_auxiliary}"
        )
    if len(expected_auxiliary) > MAX_RELEASE_NOTE_VISUAL_ASSETS:
        raise ValueError(
            "published release contains too many release-note visual sidecars: "
            f"{len(expected_auxiliary)} > {MAX_RELEASE_NOTE_VISUAL_ASSETS}"
        )
    actual: dict[str, dict[str, Any]] = {}
    for index, asset in enumerate(release_assets):
        name = asset.get("name")
        if not isinstance(name, str) or not name:
            raise ValueError(f"release asset {index} has no valid name")
        if name in actual:
            raise ValueError(f"release contains duplicate asset: {name}")
        actual[name] = asset

    expected_names = set(expected) | expected_auxiliary
    if set(actual) != expected_names:
        missing = sorted(expected_names - set(actual))
        extra = sorted(set(actual) - expected_names)
        raise ValueError(f"published asset set mismatch: missing={missing}, extra={extra}")

    for name, expected_asset in expected.items():
        actual_asset = actual[name]
        if actual_asset.get("size") != expected_asset["size"]:
            raise ValueError(f"published asset size mismatch: {name}")
        expected_digest = f"sha256:{expected_asset['sha256']}"
        if actual_asset.get("digest") != expected_digest:
            raise ValueError(
                f"published asset digest mismatch: {name}; "
                f"expected {expected_digest}, got {actual_asset.get('digest')!r}"
            )

    for name in sorted(expected_auxiliary):
        asset = actual[name]
        if (
            not isinstance(asset.get("size"), int)
            or isinstance(asset["size"], bool)
            or asset["size"] <= 0
        ):
            raise ValueError(f"published auxiliary asset is empty: {name}")
        if not isinstance(asset.get("digest"), str) or not re.fullmatch(
            r"sha256:[0-9a-f]{64}", asset["digest"]
        ):
            raise ValueError(f"published auxiliary asset has no SHA-256 digest: {name}")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    create = subparsers.add_parser("create")
    create.add_argument("--release-dir", type=Path, required=True)
    create.add_argument("--version", required=True)
    create.add_argument("--source-sha", required=True)
    create.add_argument("--output", type=Path, required=True)

    local = subparsers.add_parser("verify-local")
    local.add_argument("--release-dir", type=Path, required=True)
    local.add_argument("--manifest", type=Path, required=True)
    local.add_argument("--version", required=True)
    local.add_argument("--source-sha", required=True)

    release = subparsers.add_parser("verify-release")
    release.add_argument("--manifest", type=Path, required=True)
    release.add_argument("--assets-json", type=Path, required=True)
    release.add_argument("--version", required=True)
    release.add_argument("--source-sha", required=True)
    release.add_argument("--release-body-file", type=Path)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        if args.command == "create":
            manifest = create_manifest(args.release_dir, args.version, args.source_sha)
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(
                json.dumps(manifest, indent=2, sort_keys=True) + "\n",
                encoding="utf-8",
            )
            print(
                f"Release candidate manifest created: assets={len(manifest['assets'])} "
                f"version={args.version} source_sha={args.source_sha}"
            )
        elif args.command == "verify-local":
            manifest = load_manifest(args.manifest)
            verify_local(args.release_dir, manifest, args.version, args.source_sha)
            print(
                f"Release candidate verified locally: assets={len(manifest['assets'])} "
                f"version={args.version} source_sha={args.source_sha}"
            )
        else:
            manifest = load_manifest(args.manifest)
            verify_manifest_identity(manifest, args.version, args.source_sha)
            release_assets = load_release_assets(args.assets_json)
            auxiliary_assets: set[str] = set()
            if args.release_body_file is not None:
                body = args.release_body_file.read_text(encoding="utf-8")
                auxiliary_assets = release_note_visual_assets(
                    body, f"v{args.version}"
                )
            verify_release(manifest, release_assets, auxiliary_assets)
            print(
                f"Published release matches candidate: assets={len(manifest['assets'])} "
                f"auxiliary_assets={len(auxiliary_assets)} "
                f"version={manifest['version']} source_sha={manifest['source_sha']}"
            )
    except (OSError, ValueError) as exc:
        print(f"release candidate verification failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
