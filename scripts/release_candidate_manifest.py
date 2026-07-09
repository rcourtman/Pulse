#!/usr/bin/env python3
"""Create and verify immutable Pulse release-candidate manifests."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any


SCHEMA_VERSION = 1
VERSION_PATTERN = re.compile(
    r"^[0-9]+\.[0-9]+\.[0-9]+(?:-(?:rc|alpha|beta)\.[0-9]+)?$"
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
        if not isinstance(name, str) or not name or Path(name).name != name:
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


def verify_release(manifest: dict[str, Any], release_assets: list[dict[str, Any]]) -> None:
    expected = manifest_assets_by_name(manifest)
    actual: dict[str, dict[str, Any]] = {}
    for index, asset in enumerate(release_assets):
        name = asset.get("name")
        if not isinstance(name, str) or not name:
            raise ValueError(f"release asset {index} has no valid name")
        if name in actual:
            raise ValueError(f"release contains duplicate asset: {name}")
        actual[name] = asset

    if set(actual) != set(expected):
        missing = sorted(set(expected) - set(actual))
        extra = sorted(set(actual) - set(expected))
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
            verify_release(manifest, release_assets)
            print(
                f"Published release matches candidate: assets={len(manifest['assets'])} "
                f"version={manifest['version']} source_sha={manifest['source_sha']}"
            )
    except (OSError, ValueError) as exc:
        print(f"release candidate verification failed: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
