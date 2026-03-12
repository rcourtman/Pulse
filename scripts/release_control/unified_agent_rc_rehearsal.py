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


def safe_check(name: str, fn) -> CheckResult:
    try:
        return fn()
    except Exception as exc:  # pragma: no cover - exercised via caller tests
        return CheckResult(name=name, ok=False, detail=str(exc))


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
        "--api-token",
        help="Optional API token sent as X-API-Token for authenticated rehearsal checks",
    )
    parser.add_argument(
        "--bearer-token",
        help="Optional API token sent as Authorization: Bearer for authenticated rehearsal checks",
    )
    parser.add_argument(
        "--cookie",
        help="Optional raw Cookie header for authenticated rehearsal checks",
    )
    parser.add_argument(
        "--expected-active-agents",
        type=int,
        help=(
            "Optional expected active Pulse Unified Agent count after upgrade; "
            "verifies both /api/license/entitlements and /api/license/agent-ledger"
        ),
    )
    parser.add_argument(
        "--expected-agent-name",
        action="append",
        default=[],
        help=(
            "Optional agent display name expected in /api/license/agent-ledger after "
            "the upgrade; may be passed multiple times"
        ),
    )
    parser.add_argument(
        "--expected-online-agents",
        type=int,
        help=(
            "Optional expected number of online agents in /api/license/agent-ledger "
            "after the upgrade"
        ),
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Emit JSON instead of human-readable output",
    )
    parser.add_argument(
        "--report-out",
        help="Optional path to write a markdown rehearsal report",
    )
    parser.add_argument(
        "--report-title",
        default="Unified Agent RC Rehearsal",
        help="Markdown title used when writing --report-out",
    )
    return parser.parse_args(argv)


def normalize_base_url(url: str) -> str:
    return url.rstrip("/")


def build_auth_headers(args: argparse.Namespace) -> dict[str, str]:
    headers: dict[str, str] = {}
    if args.api_token:
        headers["X-API-Token"] = args.api_token
    if args.bearer_token:
        headers["Authorization"] = f"Bearer {args.bearer_token}"
    if args.cookie:
        headers["Cookie"] = args.cookie
    return headers


def fetch_bytes(
    url: str, *, timeout: float, headers: dict[str, str] | None = None
) -> tuple[bytes, dict[str, str]]:
    req = request.Request(url, method="GET", headers=headers or {})
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


def fetch_json(
    url: str, *, timeout: float, headers: dict[str, str] | None = None
) -> dict[str, object]:
    body, _headers = fetch_bytes(url, timeout=timeout, headers=headers)
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


def check_active_agent_accounting(
    *,
    base_url: str,
    timeout: float,
    auth_headers: dict[str, str],
    expected_active_agents: int,
) -> CheckResult:
    if not auth_headers:
        return CheckResult(
            name="active-agent-accounting",
            ok=False,
            detail=(
                "expected active-agent verification requires --api-token, "
                "--bearer-token, or --cookie"
            ),
        )

    entitlements = fetch_json(
        f"{base_url}/api/license/entitlements",
        timeout=timeout,
        headers=auth_headers,
    )
    limits = entitlements.get("limits")
    if not isinstance(limits, list):
        return CheckResult(
            name="active-agent-accounting",
            ok=False,
            detail="/api/license/entitlements returned no limits array",
        )

    max_agents_current: int | None = None
    for item in limits:
        if not isinstance(item, dict):
            continue
        if str(item.get("key", "")).strip() != "max_agents":
            continue
        current = item.get("current")
        if not isinstance(current, int):
            return CheckResult(
                name="active-agent-accounting",
                ok=False,
                detail="max_agents current usage was missing or not an integer",
            )
        max_agents_current = current
        break

    if max_agents_current is None:
        return CheckResult(
            name="active-agent-accounting",
            ok=False,
            detail="/api/license/entitlements did not include max_agents",
        )

    ledger = fetch_json(
        f"{base_url}/api/license/agent-ledger",
        timeout=timeout,
        headers=auth_headers,
    )
    agents = ledger.get("agents")
    if not isinstance(agents, list):
        return CheckResult(
            name="active-agent-accounting",
            ok=False,
            detail="/api/license/agent-ledger returned no agents array",
        )
    ledger_total = ledger.get("total")
    if not isinstance(ledger_total, int):
        ledger_total = len(agents)

    ok = (
        max_agents_current == expected_active_agents
        and ledger_total == expected_active_agents
        and len(agents) == expected_active_agents
        and max_agents_current == ledger_total
    )
    detail = (
        f"entitlements max_agents.current={max_agents_current}, "
        f"agent-ledger total={ledger_total}, "
        f"agent-ledger agents={len(agents)}, "
        f"expected={expected_active_agents}"
    )
    return CheckResult(name="active-agent-accounting", ok=ok, detail=detail)


def check_agent_ledger_identity(
    *,
    base_url: str,
    timeout: float,
    auth_headers: dict[str, str],
    expected_agent_names: list[str],
    expected_online_agents: int | None,
) -> CheckResult:
    if not auth_headers:
        return CheckResult(
            name="agent-ledger-identity",
            ok=False,
            detail=(
                "agent ledger identity verification requires --api-token, "
                "--bearer-token, or --cookie"
            ),
        )

    ledger = fetch_json(
        f"{base_url}/api/license/agent-ledger",
        timeout=timeout,
        headers=auth_headers,
    )
    agents = ledger.get("agents")
    if not isinstance(agents, list):
        return CheckResult(
            name="agent-ledger-identity",
            ok=False,
            detail="/api/license/agent-ledger returned no agents array",
        )

    normalized_expected_names = [name.strip() for name in expected_agent_names if name.strip()]
    names = [str(agent.get("name", "")).strip() for agent in agents if isinstance(agent, dict)]
    online_agents = [
        agent
        for agent in agents
        if isinstance(agent, dict) and str(agent.get("status", "")).strip() == "online"
    ]

    failures: list[str] = []
    for expected_name in normalized_expected_names:
        matches = sum(1 for name in names if name == expected_name)
        if matches == 0:
            failures.append(f"missing expected agent name {expected_name!r}")
        elif matches > 1:
            failures.append(f"agent name {expected_name!r} appeared {matches} times")

    if expected_online_agents is not None and len(online_agents) != expected_online_agents:
        failures.append(
            f"online agent count {len(online_agents)} did not match expected {expected_online_agents}"
        )

    detail_parts = [
        f"ledger names={names!r}",
        f"online_agents={len(online_agents)}",
    ]
    if normalized_expected_names:
        detail_parts.append(f"expected_names={normalized_expected_names!r}")
    if expected_online_agents is not None:
        detail_parts.append(f"expected_online_agents={expected_online_agents}")
    if failures:
        detail_parts.append("failures=" + "; ".join(failures))

    return CheckResult(
        name="agent-ledger-identity",
        ok=not failures,
        detail=", ".join(detail_parts),
    )


def run_rehearsal(args: argparse.Namespace) -> list[CheckResult]:
    base_url = normalize_base_url(args.base_url)
    release_base_url = normalize_base_url(args.release_base_url)
    auth_headers = build_auth_headers(args)
    results = [
        safe_check(
            "agent-version-endpoint",
            lambda: check_version(base_url, args.expected_version, args.timeout),
        ),
        safe_check(
            "install-sh-asset",
            lambda: compare_asset(
                name="install-sh-asset",
                live_url=f"{base_url}/install.sh",
                release_url=release_asset_url(release_base_url, args.expected_version, "install.sh"),
                timeout=args.timeout,
            ),
        ),
        safe_check(
            "install-ps1-asset",
            lambda: compare_asset(
                name="install-ps1-asset",
                live_url=f"{base_url}/install.ps1",
                release_url=release_asset_url(release_base_url, args.expected_version, "install.ps1"),
                timeout=args.timeout,
            ),
        ),
    ]

    for arch in args.arch:
        quoted_arch = parse.quote(arch, safe="")
        results.append(
            safe_check(
                f"agent-binary-{arch}",
                lambda arch=arch, quoted_arch=quoted_arch: compare_asset(
                    name=f"agent-binary-{arch}",
                    live_url=f"{base_url}/download/pulse-agent?arch={quoted_arch}",
                    release_url=release_asset_url(
                        release_base_url, args.expected_version, agent_binary_asset_name(arch)
                    ),
                    timeout=args.timeout,
                    expect_checksum_header=True,
                ),
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
            results.append(
                safe_check(
                    "local-update-info",
                    lambda: check_update_info(args.update_info_dir, args.expected_updated_from),
                )
            )

    if args.expected_active_agents is not None:
        results.append(
            safe_check(
                "active-agent-accounting",
                lambda: check_active_agent_accounting(
                    base_url=base_url,
                    timeout=args.timeout,
                    auth_headers=auth_headers,
                    expected_active_agents=args.expected_active_agents,
                ),
            )
        )

    if args.expected_agent_name or args.expected_online_agents is not None:
        results.append(
            safe_check(
                "agent-ledger-identity",
                lambda: check_agent_ledger_identity(
                    base_url=base_url,
                    timeout=args.timeout,
                    auth_headers=auth_headers,
                    expected_agent_names=args.expected_agent_name,
                    expected_online_agents=args.expected_online_agents,
                ),
            )
        )

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


def render_markdown_report(
    *,
    title: str,
    base_url: str,
    expected_version: str,
    release_base_url: str,
    results: Iterable[CheckResult],
) -> str:
    lines = [
        f"# {title}",
        "",
        f"- Base URL: `{base_url}`",
        f"- Expected version: `{expected_version}`",
        f"- Release asset base: `{release_base_url}`",
        "",
        "## Automated Checks",
        "",
    ]
    for result in results:
        status = "PASS" if result.ok else "FAIL"
        lines.append(f"- `{status}` `{result.name}`: {result.detail}")
    lines.extend(
        [
            "",
            "## Manual Follow-up",
            "",
            "- Confirm the upgraded v5-installed agent reconnects as one canonical v6 identity.",
            "- Confirm `updated_from` appears exactly once on the first canonical v6 report and clears on the next report.",
            "- Confirm settings/billing active-agent counts still match runtime enforcement after the upgrade.",
        ]
    )
    return "\n".join(lines)


def write_report(path: str, content: str) -> None:
    report_path = Path(path)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(content + "\n", encoding="utf-8")


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    results = run_rehearsal(args)
    ok = all(result.ok for result in results)

    if args.report_out:
        write_report(
            args.report_out,
            render_markdown_report(
                title=args.report_title,
                base_url=normalize_base_url(args.base_url),
                expected_version=args.expected_version,
                release_base_url=normalize_base_url(args.release_base_url),
                results=results,
            ),
        )

    if args.json:
        print(json.dumps({"ok": ok, "results": [asdict(result) for result in results]}, indent=2))
    else:
        print(render_text(results))

    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
