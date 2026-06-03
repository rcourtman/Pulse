#!/usr/bin/env python3
"""Validate the live unified-agent RC crossover against expected release assets."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import threading
import time
from dataclasses import asdict, dataclass
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Iterable
from urllib import error, parse, request


DEFAULT_RELEASE_BASE_URL = "https://github.com/rcourtman/Pulse/releases/download"


@dataclass
class CheckResult:
    name: str
    ok: bool
    detail: str


class _RuntimeProofHandler(BaseHTTPRequestHandler):
    server: "_RuntimeProofServer"

    def do_GET(self) -> None:  # noqa: N802
        self.server.record_request("GET", self.path)
        parsed = parse.urlparse(self.path)
        path = parsed.path

        if path == "/api/agent/version":
            self._write_json({"version": self.server.expected_version})
            return

        if path == "/download/pulse-agent":
            self.send_response(200)
            self.send_header("Content-Type", "application/octet-stream")
            self.send_header("X-Checksum-Sha256", self.server.v6_checksum)
            self.send_header("Content-Length", str(len(self.server.v6_binary)))
            self.end_headers()
            self.wfile.write(self.server.v6_binary)
            return

        if path in {"/api/agents/host/lookup", "/api/agents/agent/lookup"}:
            if path.endswith("/host/lookup"):
                self._write_json({"success": True, "host": {"id": self.server.agent_id}})
            else:
                self._write_json({"success": True, "agent": {"id": self.server.agent_id}})
            return

        if path.startswith("/api/agents/host/") and path.endswith("/config"):
            self._write_json({"success": True, "hostId": self.server.agent_id, "config": {}})
            return

        if path.startswith("/api/agents/agent/") and path.endswith("/config"):
            self._write_json({"success": True, "agentId": self.server.agent_id, "config": {}})
            return

        self.send_response(404)
        self.end_headers()
        self.wfile.write(b"missing")

    def do_POST(self) -> None:  # noqa: N802
        self.server.record_request("POST", self.path)
        body = self.rfile.read(int(self.headers.get("Content-Length", "0") or "0"))
        if self.headers.get("Content-Encoding", "").lower() == "gzip":
            body = gzip.decompress(body)

        if self.path in {"/api/agents/host/report", "/api/agents/agent/report"}:
            try:
                payload = json.loads(body.decode("utf-8"))
            except json.JSONDecodeError:
                self.send_response(400)
                self.end_headers()
                self.wfile.write(b"bad report json")
                return
            self.server.record_report(self.path, payload)
            if self.path.endswith("/agent/report"):
                self._write_json({"success": True, "agentId": self.server.agent_id, "config": {}})
            else:
                self._write_json({"success": True, "hostId": self.server.agent_id, "config": {}})
            return

        self.send_response(404)
        self.end_headers()
        self.wfile.write(b"missing")

    def log_message(self, format: str, *args: object) -> None:  # noqa: A003
        return

    def _write_json(self, payload: dict[str, object]) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


class _RuntimeProofServer(ThreadingHTTPServer):
    def __init__(self, *, expected_version: str, v6_binary: bytes, agent_id: str) -> None:
        super().__init__(("127.0.0.1", 0), _RuntimeProofHandler)
        self.expected_version = expected_version
        self.v6_binary = v6_binary
        self.v6_checksum = sha256_hex(v6_binary)
        self.agent_id = agent_id
        self.requests: list[tuple[str, str]] = []
        self.reports: list[tuple[str, dict[str, object]]] = []

    @property
    def base_url(self) -> str:
        return f"http://127.0.0.1:{self.server_port}"

    def record_request(self, method: str, path: str) -> None:
        self.requests.append((method, path))

    def record_report(self, path: str, payload: dict[str, object]) -> None:
        self.reports.append((path, payload))


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
    parser.add_argument(
        "--skip-asset-checks",
        action="store_true",
        help=(
            "Skip live Pulse version/install/download asset checks; intended for isolated "
            "local runtime self-update proof runs"
        ),
    )
    parser.add_argument(
        "--runtime-v5-agent",
        help="Optional path to a real v5 pulse-agent binary to exercise in-place self-update",
    )
    parser.add_argument(
        "--runtime-v6-agent",
        help="Optional path to the v6 pulse-agent binary served by the local runtime proof server",
    )
    parser.add_argument(
        "--runtime-expected-from",
        default="v5.1.34",
        help="Expected version reported by --runtime-v5-agent before the swap",
    )
    parser.add_argument(
        "--runtime-expected-to",
        help="Expected version after the swap; defaults to --expected-version",
    )
    parser.add_argument(
        "--runtime-timeout",
        type=float,
        default=60.0,
        help="Seconds to wait for the real v5 process to self-update and report as v6",
    )
    parser.add_argument(
        "--runtime-work-dir",
        help="Optional existing directory for runtime proof artifacts; defaults to a temporary directory",
    )
    parser.add_argument(
        "--runtime-keep-work-dir",
        action="store_true",
        help="Keep runtime proof artifacts instead of deleting the temporary directory",
    )
    return parser.parse_args(argv)


def normalize_base_url(url: str) -> str:
    return url.rstrip("/")


def summarize_http_error_body(body: bytes) -> str:
    message = body.decode("utf-8", errors="replace").strip()
    if not message:
        return "<empty response>"
    first_line = message.splitlines()[0].strip()
    if first_line.startswith("<!DOCTYPE html") or first_line.startswith("<html"):
        return "<html error page omitted>"
    if len(first_line) > 240:
        return first_line[:237] + "..."
    return first_line


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
        message = summarize_http_error_body(body)
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


def normalize_version(value: str) -> str:
    value = value.strip()
    if value.startswith("v"):
        value = value[1:]
    return value


def report_agent_info(report: dict[str, object]) -> dict[str, object]:
    agent = report.get("agent")
    if isinstance(agent, dict):
        return agent
    return {}


def agent_report_version(report: dict[str, object]) -> str:
    agent = report_agent_info(report)
    return str(agent.get("version", "")).strip()


def agent_report_updated_from(report: dict[str, object]) -> str:
    agent = report_agent_info(report)
    return str(agent.get("updatedFrom") or agent.get("updated_from") or "").strip()


def agent_report_type(report: dict[str, object]) -> str:
    agent = report_agent_info(report)
    return str(agent.get("type", "")).strip()


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


def check_local_runtime_self_update(
    *,
    v5_agent: str,
    v6_agent: str,
    expected_from: str,
    expected_to: str,
    timeout: float,
    work_dir: str | None,
    keep_work_dir: bool,
) -> CheckResult:
    v5_path = Path(v5_agent)
    v6_path = Path(v6_agent)
    if not v5_path.is_file():
        return CheckResult("local-runtime-self-update", False, f"v5 agent not found: {v5_path}")
    if not v6_path.is_file():
        return CheckResult("local-runtime-self-update", False, f"v6 agent not found: {v6_path}")

    expected_from_output = subprocess.check_output([str(v5_path), "--version"], text=True).strip()
    if normalize_version(expected_from_output) != normalize_version(expected_from):
        return CheckResult(
            "local-runtime-self-update",
            False,
            f"v5 binary reports {expected_from_output!r}, expected {expected_from!r}",
        )

    expected_to_output = subprocess.check_output([str(v6_path), "--version"], text=True).strip()
    if normalize_version(expected_to_output) != normalize_version(expected_to):
        return CheckResult(
            "local-runtime-self-update",
            False,
            f"v6 binary reports {expected_to_output!r}, expected {expected_to!r}",
        )

    v6_binary = v6_path.read_bytes()
    agent_id = "agent-v5-to-v6-runtime-proof"
    server = _RuntimeProofServer(
        expected_version=normalize_version(expected_to),
        v6_binary=v6_binary,
        agent_id=agent_id,
    )
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()

    temp_root_created = False
    if work_dir:
        root = Path(work_dir)
        root.mkdir(parents=True, exist_ok=True)
    else:
        root = Path(tempfile.mkdtemp(prefix="pulse-agent-v5-v6-proof-"))
        temp_root_created = True

    proc: subprocess.Popen[str] | None = None
    try:
        runtime_agent = root / "pulse-agent"
        shutil.copy2(v5_path, runtime_agent)
        runtime_agent.chmod(0o755)
        token_file = root / "token"
        token_file.write_text("runtime-proof-token\n", encoding="utf-8")
        token_file.chmod(0o600)
        agent_id_file = root / "agent-id"
        state_dir = root / "state"
        state_dir.mkdir(exist_ok=True)
        log_path = root / "agent.log"

        env = os.environ.copy()
        env.update(
            {
                "PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED": "false",
                "PULSE_STATE_DIR": str(state_dir),
            }
        )
        with log_path.open("w", encoding="utf-8") as log_file:
            proc = subprocess.Popen(
                [
                    str(runtime_agent),
                    "--url",
                    server.base_url,
                    "--token-file",
                    str(token_file),
                    "--agent-id",
                    agent_id,
                    "--agent-id-file",
                    str(agent_id_file),
                    "--hostname",
                    "pulse-v5-v6-runtime-proof",
                    "--interval",
                    "2s",
                    "--health-addr",
                    "127.0.0.1:0",
                    "--insecure",
                    "--log-level",
                    "debug",
                ],
                cwd=str(root),
                env=env,
                stdout=log_file,
                stderr=subprocess.STDOUT,
                text=True,
            )

            deadline = time.monotonic() + timeout
            saw_v5 = False
            saw_v6 = False
            saw_updated_from = False
            while time.monotonic() < deadline:
                if proc.poll() is not None:
                    break
                for _path, report in server.reports:
                    version = normalize_version(agent_report_version(report))
                    if version == normalize_version(expected_from):
                        saw_v5 = True
                    if version == normalize_version(expected_to):
                        saw_v6 = True
                        if normalize_version(agent_report_updated_from(report)) == normalize_version(
                            expected_from
                        ):
                            saw_updated_from = True
                        if agent_report_type(report) != "unified":
                            return CheckResult(
                                "local-runtime-self-update",
                                False,
                                f"v6 report type={agent_report_type(report)!r}, expected 'unified'",
                            )
                if saw_v5 and saw_v6 and saw_updated_from:
                    replaced_version = subprocess.check_output(
                        [str(runtime_agent), "--version"], text=True
                    ).strip()
                    if normalize_version(replaced_version) != normalize_version(expected_to):
                        return CheckResult(
                            "local-runtime-self-update",
                            False,
                            f"replaced binary reports {replaced_version!r}, expected {expected_to!r}",
                        )
                    return CheckResult(
                        "local-runtime-self-update",
                        True,
                        (
                            f"v5 process reported {expected_from_output}, downloaded checksum "
                            f"{server.v6_checksum}, exec'd v6 report {expected_to_output} with "
                            f"updated_from={expected_from_output}; work_dir={root}"
                        ),
                    )
                time.sleep(0.25)

            log_excerpt = ""
            if log_path.exists():
                log_lines = log_path.read_text(encoding="utf-8", errors="replace").splitlines()
                log_excerpt = "; log_tail=" + " | ".join(log_lines[-8:])
            versions = [agent_report_version(report) for _path, report in server.reports]
            return CheckResult(
                "local-runtime-self-update",
                False,
                (
                    f"timed out waiting for v5->v6 report sequence "
                    f"(saw_v5={saw_v5}, saw_v6={saw_v6}, "
                    f"saw_updated_from={saw_updated_from}, versions={versions!r}, "
                    f"work_dir={root}){log_excerpt}"
                ),
            )
    finally:
        if proc is not None and proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait(timeout=5)
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        if temp_root_created and not keep_work_dir:
            shutil.rmtree(root, ignore_errors=True)


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
    results: list[CheckResult] = []

    if not args.skip_asset_checks:
        results.extend(
            [
                safe_check(
                    "agent-version-endpoint",
                    lambda: check_version(base_url, args.expected_version, args.timeout),
                ),
                safe_check(
                    "install-sh-asset",
                    lambda: compare_asset(
                        name="install-sh-asset",
                        live_url=f"{base_url}/install.sh",
                        release_url=release_asset_url(
                            release_base_url, args.expected_version, "install.sh"
                        ),
                        timeout=args.timeout,
                    ),
                ),
                safe_check(
                    "install-ps1-asset",
                    lambda: compare_asset(
                        name="install-ps1-asset",
                        live_url=f"{base_url}/install.ps1",
                        release_url=release_asset_url(
                            release_base_url, args.expected_version, "install.ps1"
                        ),
                        timeout=args.timeout,
                    ),
                ),
            ]
        )

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
    elif args.arch:
        results.append(
            safe_check(
                "asset-check-selection",
                lambda: CheckResult(
                    name="asset-check-selection",
                    ok=False,
                    detail="--arch cannot be combined with --skip-asset-checks",
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

    if args.runtime_v5_agent or args.runtime_v6_agent:
        if not args.runtime_v5_agent or not args.runtime_v6_agent:
            results.append(
                CheckResult(
                    name="local-runtime-self-update",
                    ok=False,
                    detail="both --runtime-v5-agent and --runtime-v6-agent are required together",
                )
            )
        else:
            results.append(
                safe_check(
                    "local-runtime-self-update",
                    lambda: check_local_runtime_self_update(
                        v5_agent=args.runtime_v5_agent,
                        v6_agent=args.runtime_v6_agent,
                        expected_from=args.runtime_expected_from,
                        expected_to=args.runtime_expected_to or args.expected_version,
                        timeout=args.runtime_timeout,
                        work_dir=args.runtime_work_dir,
                        keep_work_dir=args.runtime_keep_work_dir,
                    ),
                )
            )

    if not results:
        results.append(
            CheckResult(
                name="check-selection",
                ok=False,
                detail="no rehearsal checks selected",
            )
        )

    return results


def render_text(results: Iterable[CheckResult]) -> str:
    results = list(results)
    lines: list[str] = []
    for result in results:
        prefix = "PASS" if result.ok else "FAIL"
        lines.append(f"[{prefix}] {result.name}: {result.detail}")
    if any(result.name == "local-runtime-self-update" and result.ok for result in results):
        lines.append(
            "Runtime follow-up covered: local v5 pulse-agent performed an in-place "
            "self-update, exec'd the v6 binary, and reported `updated_from` once."
        )
    else:
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
    results = list(results)
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
    runtime_covered = any(result.name == "local-runtime-self-update" and result.ok for result in results)
    if runtime_covered:
        lines.extend(
            [
                "",
                "## Runtime Proof",
                "",
                "- Local v5 `pulse-agent` performed the in-place self-update and exec'd the v6 binary.",
                "- The first v6 report carried `updated_from` for the v5 source version.",
                "- Active-agent accounting still requires a live Pulse API check when this proof is run outside a full server.",
            ]
        )
    else:
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
