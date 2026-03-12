#!/usr/bin/env python3
"""Validate the live unified-agent RC crossover against expected release assets."""

from __future__ import annotations

import argparse
import hashlib
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterable
from urllib import error, parse, request


DEFAULT_RELEASE_BASE_URL = "https://github.com/rcourtman/Pulse/releases/download"


@dataclass
class CheckResult:
    name: str
    ok: bool
    detail: str


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Verify that a live Pulse RC instance serves the expected unified-agent "
            "version and release assets for the v5-to-v6 crossover rehearsal."
        )
    )
    parser.add_argument("--base-url", required=True, help="Pulse server base URL")
    parser.add_argument(
        "--expected-version",
        required=True,
        help="Expected Pulse version from /api/agent/version, e.g. 6.0.0-rc.1",
    )
    parser.add_argument(
        "--release-base-url",
        default=DEFAULT_RELEASE_BASE_URL,
        help="Release asset base URL to compare against",
    )
    parser.add_argument(
        "--arch",
        action="append",
        default=[],
        help="Optional unified-agent architecture to verify via /download/pulse-agent?arch=...",
    )
    parser.add_argument(
        "--update-info-dir",
        help="Optional directory containing .pulse-update-info on the upgraded machine",
    )
    parser.add_argument(
        "--expected-updated-from",
        help="Optional expected previous agent version inside .pulse-update-info",
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=30.0,
        help="HTTP timeout in seconds",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit JSON instead of human-readable output",
    )
    return parser.parse_args(argv)


def normalize_base_url(url: str) -> str:
    return url.rstrip("/")


def fetch_bytes(url: str, *, timeout: float) -> tuple[bytes, dict[str, str]]:
    req = request.Request(url, method="GET")
    try:
        with request.urlopen(req, timeout=timeout) as resp:
            body = resp.read()
            headers = {key.lower(): value for key, value in resp.headers.items()}
            return body, headers
    except error.HTTPError as exc:
        body = exc.read()
        message = body.decode("utf-8", errors="replace").strip()
        raise RuntimeError(f"{url} returned HTTP {exc.code}: {message}") from exc
    except error.URLError as exc:
        raise RuntimeError(f"failed to fetch {url}: {exc.reason}") from exc


def fetch_json(url: str, *, timeout: float) -> dict[str, object]:
    body, _headers = fetch_bytes(url, timeout=timeout)
    try:
        payload = json.loads(body.decode("utf-8"))
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{url} did not return valid JSON") from exc
    if not isinstance(payload, dict):
        raise RuntimeError(f"{url} did not return a JSON object")
    return payload


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def release_asset_url(release_base_url: str, version: str, asset_name: str) -> str:
    return f"{normalize_base_url(release_base_url)}/v{version}/{asset_name}"


def agent_binary_asset_name(arch: str) -> str:
    asset = f"pulse-agent-{arch}"
    if arch.startswith("windows-"):
        asset += ".exe"
    return asset


def compare_asset(
    *,
    name: str,
    live_url: str,
    release_url: str,
    timeout: float,
    expect_checksum_header: bool = False,
) -> CheckResult:
    live_body, live_headers = fetch_bytes(live_url, timeout=timeout)
    release_body, _release_headers = fetch_bytes(release_url, timeout=timeout)

    live_hash = sha256_hex(live_body)
    release_hash = sha256_hex(release_body)
    if live_hash != release_hash:
        return CheckResult(
            name=name,
            ok=False,
            detail=(
                f"live asset hash {live_hash} does not match release asset hash "
                f"{release_hash} ({release_url})"
            ),
        )

    if expect_checksum_header:
        checksum_header = live_headers.get("x-checksum-sha256", "").strip()
        if checksum_header != live_hash:
            return CheckResult(
                name=name,
                ok=False,
                detail=(
                    f"X-Checksum-Sha256={checksum_header or '<missing>'} does not "
                    f"match live asset hash {live_hash}"
                ),
            )

    served_from = live_headers.get("x-served-from")
    served_note = f"; served_from={served_from}" if served_from else ""
    return CheckResult(
        name=name,
        ok=True,
        detail=f"matched release asset hash {live_hash}{served_note}",
    )


def check_version(base_url: str, expected_version: str, timeout: float) -> CheckResult:
    payload = fetch_json(f"{base_url}/api/agent/version", timeout=timeout)
    version = str(payload.get("version", "")).strip()
    if version != expected_version:
        return CheckResult(
            name="agent-version-endpoint",
            ok=False,
            detail=f"reported version {version!r}, expected {expected_version!r}",
        )
    return CheckResult(
        name="agent-version-endpoint",
        ok=True,
        detail=f"reported expected version {version}",
    )


def check_update_info(update_info_dir: str, expected_updated_from: str) -> CheckResult:
    info_path = Path(update_info_dir) / ".pulse-update-info"
    if not info_path.exists():
        return CheckResult(
            name="local-update-info",
            ok=False,
            detail=f"{info_path} is missing",
        )
    actual = info_path.read_text(encoding="utf-8").strip()
    if actual != expected_updated_from:
        return CheckResult(
            name="local-update-info",
            ok=False,
            detail=f"{info_path} contains {actual!r}, expected {expected_updated_from!r}",
        )
    return CheckResult(
        name="local-update-info",
        ok=True,
        detail=f"{info_path} contains expected previous version {actual}",
    )


def run_rehearsal(args: argparse.Namespace) -> list[CheckResult]:
    base_url = normalize_base_url(args.base_url)
    release_base_url = normalize_base_url(args.release_base_url)
    results = [
        check_version(base_url, args.expected_version, args.timeout),
        compare_asset(
            name="install-sh-asset",
            live_url=f"{base_url}/install.sh",
            release_url=release_asset_url(release_base_url, args.expected_version, "install.sh"),
            timeout=args.timeout,
        ),
        compare_asset(
            name="install-ps1-asset",
            live_url=f"{base_url}/install.ps1",
            release_url=release_asset_url(release_base_url, args.expected_version, "install.ps1"),
            timeout=args.timeout,
        ),
    ]

    for arch in args.arch:
        quoted_arch = parse.quote(arch, safe="")
        results.append(
            compare_asset(
                name=f"agent-binary-{arch}",
                live_url=f"{base_url}/download/pulse-agent?arch={quoted_arch}",
                release_url=release_asset_url(
                    release_base_url, args.expected_version, agent_binary_asset_name(arch)
                ),
                timeout=args.timeout,
                expect_checksum_header=True,
            )
        )

    if args.update_info_dir or args.expected_updated_from:
        if not args.update_info_dir or not args.expected_updated_from:
            results.append(
                CheckResult(
                    name="local-update-info",
                    ok=False,
                    detail="both --update-info-dir and --expected-updated-from are required together",
                )
            )
        else:
            results.append(check_update_info(args.update_info_dir, args.expected_updated_from))

    return results


def render_text(results: Iterable[CheckResult]) -> str:
    lines: list[str] = []
    for result in results:
        prefix = "PASS" if result.ok else "FAIL"
        lines.append(f"[{prefix}] {result.name}: {result.detail}")
    lines.append(
        "Manual follow-up still required: confirm the upgraded v5-installed agent "
        "reconnects as one canonical v6 identity, surfaces `updated_from` exactly once, "
        "and leaves user-visible active-agent counts aligned with runtime enforcement."
    )
    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_rehearsal(args)
    ok = all(result.ok for result in results)

    if args.json:
        print(json.dumps({"ok": ok, "results": [asdict(result) for result in results]}, indent=2))
    else:
        print(render_text(results))

    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
