#!/usr/bin/env python3
"""Drive a real (non-mock) Pulse with synthetic Unified Agents (#2199).

Mock mode drops pushed agent reports and skips state broadcasts without a
subscriber, so it never exercises the cost #2199 measured: every accepted
report refreshes the unified resource registry and broadcasts state to
connected dashboards. This driver reproduces that traffic against a Pulse
started with development admin bypass and authentication configured:

- mints one install token per agent through /api/agent-install-command,
  the way the install flow does, and sends every report with its token;
- posts a gzip-compressed host report from each agent every interval, with a
  random start offset, a stable machine identity and drifting metrics;
- posts a Docker report for the first --docker-agents agents with
  --containers containers each, under the same token, as a Unified Agent with
  Docker enabled does;
- keeps one WebSocket dashboard subscriber connected to /ws.

It runs until --duration elapses or SIGTERM arrives, then writes a JSON stats
file. It exits 1 when no report was accepted, more than 1% failed, or the
dashboard never received an initial state followed by a nonempty state update.
A handshake, welcome or heartbeat alone cannot pass as broadcast workload.
"""

from __future__ import annotations

import argparse
import base64
import gzip
import json
import os
import random
import signal
import socket
import struct
import sys
import threading
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone

GIB = 1024**3
# The Unified Agent's report client gives up after 15 seconds
# (internal/hostagent/agent.go); a slower answer is a failed report there too.
REPORT_TIMEOUT_S = 15
STOP = threading.Event()
DASHBOARD_SOCKETS: set[socket.socket] = set()
DASHBOARD_LOCK = threading.Lock()


@dataclass
class Stats:
    lock: threading.Lock = field(default_factory=threading.Lock)
    accepted: dict[str, int] = field(default_factory=lambda: {"host": 0, "docker": 0})
    failed: dict[str, int] = field(default_factory=lambda: {"host": 0, "docker": 0})
    statuses: dict[str, int] = field(default_factory=dict)
    errors: dict[str, int] = field(default_factory=dict)
    latencies: dict[str, list[float]] = field(default_factory=lambda: {"host": [], "docker": []})
    ws_connects: int = 0
    ws_messages: int = 0
    ws_bytes: int = 0
    ws_initial_states: int = 0
    ws_state_updates: int = 0
    ws_state_bytes: int = 0
    ws_errors: list[str] = field(default_factory=list)

    def record(self, kind: str, status: int, error: str = "", seconds: float | None = None) -> None:
        with self.lock:
            if seconds is not None:
                self.latencies[kind].append(seconds)
            bucket = self.accepted if 200 <= status < 300 else self.failed
            bucket[kind] += 1
            self.statuses[str(status)] = self.statuses.get(str(status), 0) + 1
            if error:
                self.errors[error] = self.errors.get(error, 0) + 1


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")


def request(url: str, body: bytes | None = None, headers: dict[str, str] | None = None, timeout: float = 10) -> tuple[int, bytes]:
    status, data, _ = request_detail(url, body, headers, timeout)
    return status, data


def request_detail(url: str, body: bytes | None = None, headers: dict[str, str] | None = None, timeout: float = 10) -> tuple[int, bytes, str]:
    """Status, body and, for a transport failure (status 0), the error's kind."""
    req = urllib.request.Request(url, data=body, headers=headers or {}, method="POST" if body is not None else "GET")
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, resp.read(), ""
    except urllib.error.HTTPError as err:
        with err:
            return err.code, err.read(), ""
    except (urllib.error.URLError, OSError) as err:
        reason = getattr(err, "reason", err)
        return 0, b"", f"{type(reason).__name__}: {reason}"[:120]


def mint_token(base: str, name: str) -> str:
    status, body = request(
        f"{base}/api/agent-install-command",
        json.dumps({"type": "host", "name": name}).encode(),
        {"Content-Type": "application/json"},
    )
    token = json.loads(body or b"{}").get("token", "") if status == 200 else ""
    if not token:
        raise SystemExit(f"could not mint an install token for {name}: HTTP {status} {body[:200]!r}")
    return token


class Agent:
    def __init__(self, index: int, token: str, containers: int, interval: int, rng: random.Random):
        self.index = index
        self.token = token
        self.interval = interval
        self.rng = rng
        self.hostname = f"footprint-host-{index:03d}"
        self.machine_id = f"f00d{index:028x}"
        self.agent_id = f"footprint-agent-{index:03d}"
        self.stream = f"fp{index:03d}{rng.randrange(1 << 32):08x}"
        self.sequence = 0
        self.cpu = rng.uniform(3, 40)
        self.mem = rng.uniform(0.3, 0.8)
        self.rx = rng.randrange(1 << 30)
        self.tx = rng.randrange(1 << 30)
        self.started = time.time() - rng.randrange(3600, 90 * 86400)
        self.containers = [
            {"id": f"{index:04x}{c:04x}" + "0" * 56, "name": f"app-{index:03d}-{c:02d}",
             "image": f"registry.example/app{c % 7}:1.{c % 5}", "cpu": rng.uniform(0, 8), "mem": rng.randrange(32, 1024) * 1024 * 1024}
            for c in range(containers)
        ]

    def drift(self) -> None:
        self.cpu = min(95.0, max(0.5, self.cpu + self.rng.uniform(-3, 3)))
        self.mem = min(0.95, max(0.1, self.mem + self.rng.uniform(-0.01, 0.01)))
        self.rx += self.rng.randrange(1 << 16, 1 << 22)
        self.tx += self.rng.randrange(1 << 16, 1 << 22)
        for c in self.containers:
            c["cpu"] = min(100.0, max(0.0, c["cpu"] + self.rng.uniform(-1, 1)))

    def host_report(self) -> dict:
        self.sequence += 1
        total = 16 * GIB
        used = int(total * self.mem)
        uptime = int(time.time() - self.started)
        disks = [
            {"device": f"/dev/sd{chr(97 + d)}1", "mountpoint": "/" if d == 0 else f"/data{d}", "filesystem": "ext4",
             "type": "disk", "totalBytes": 500 * GIB, "usedBytes": (120 + 40 * d) * GIB,
             "freeBytes": (380 - 40 * d) * GIB, "usage": (120 + 40 * d) / 5}
            for d in range(3)
        ]
        return {
            "agent": {"id": self.agent_id, "version": "6.7.0", "type": "unified",
                      "intervalSeconds": self.interval, "hostname": self.hostname},
            "host": {"hostname": self.hostname, "machineId": self.machine_id, "platform": "linux",
                     "osName": "Debian GNU/Linux", "osVersion": "12", "kernelVersion": "6.1.0-28-amd64",
                     "architecture": "amd64", "cpuModel": "Synthetic CPU", "cpuCount": 8, "uptimeSeconds": uptime,
                     "loadAverage": [round(self.cpu / 25, 2), round(self.cpu / 30, 2), round(self.cpu / 35, 2)]},
            "metrics": {"cpuUsagePercent": round(self.cpu, 2),
                        "memory": {"totalBytes": total, "usedBytes": used, "freeBytes": total - used,
                                   "usage": round(100 * self.mem, 2)}},
            "disks": disks,
            "network": [
                {"name": "eth0", "mac": f"02:00:00:00:{self.index // 256:02x}:{self.index % 256:02x}",
                 "addresses": [f"10.{self.index // 256}.{self.index % 256}.10/24"], "rxBytes": self.rx, "txBytes": self.tx},
                {"name": "lo", "addresses": ["127.0.0.1/8"], "rxBytes": 1000, "txBytes": 1000},
            ],
            "sensors": {"temperatureCelsius": {"cpu_package": round(40 + self.cpu / 4, 1), "nvme0": 38.0}},
            "timestamp": now_iso(),
            "sequenceId": f"v1:{self.stream}:{self.sequence}",
        }

    def docker_report(self) -> dict:
        created = datetime.fromtimestamp(self.started, timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        return {
            "agent": {"id": self.agent_id, "version": "6.7.0", "type": "unified", "intervalSeconds": self.interval},
            "host": {"hostname": self.hostname, "machineId": self.machine_id, "os": "Debian GNU/Linux 12",
                     "runtime": "docker", "runtimeVersion": "27.3.1", "dockerVersion": "27.3.1",
                     "totalCpu": 8, "totalMemoryBytes": 16 * GIB, "uptimeSeconds": int(time.time() - self.started)},
            "containers": [
                {"id": c["id"], "name": c["name"], "image": c["image"], "createdAt": created, "state": "running",
                 "status": "Up 3 days", "cpuPercent": round(c["cpu"], 2), "memoryUsageBytes": c["mem"],
                 "memoryLimitBytes": 2 * GIB, "memoryPercent": round(100 * c["mem"] / (2 * GIB), 2),
                 "uptimeSeconds": 259200, "restartCount": 0, "exitCode": 0}
                for c in self.containers
            ],
            "timestamp": now_iso(),
            "sequenceId": f"v1:{self.stream}d:{self.sequence}",
        }


def post_report(base: str, agent: Agent, path: str, payload: dict, kind: str, stats: Stats) -> None:
    body = gzip.compress(json.dumps(payload).encode())
    started = time.monotonic()
    status, _, error = request_detail(f"{base}{path}", body, {
        "Content-Type": "application/json", "Content-Encoding": "gzip", "X-API-Token": agent.token,
        "User-Agent": "pulse-footprint-agent-load"}, timeout=REPORT_TIMEOUT_S)
    stats.record(kind, status, error, time.monotonic() - started)


def run_agent(base: str, agent: Agent, deadline: float, stats: Stats) -> None:
    if STOP.wait(agent.rng.uniform(0, agent.interval)):
        return
    while not STOP.is_set() and time.time() < deadline:
        cycle = time.monotonic()
        agent.drift()
        post_report(base, agent, "/api/agents/agent/report", agent.host_report(), "host", stats)
        if agent.containers:
            post_report(base, agent, "/api/agents/docker/report", agent.docker_report(), "docker", stats)
        STOP.wait(max(0.0, agent.interval - (time.monotonic() - cycle)))


def ws_read_exact(sock: socket.socket, n: int) -> bytes:
    data = b""
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise ConnectionError("websocket closed")
        data += chunk
    return data


def ws_send(sock: socket.socket, opcode: int, payload: bytes) -> None:
    mask = os.urandom(4)
    header = bytes([0x80 | opcode])
    length = len(payload)
    if length < 126:
        header += bytes([0x80 | length])
    elif length < 1 << 16:
        header += bytes([0x80 | 126]) + struct.pack("!H", length)
    else:
        header += bytes([0x80 | 127]) + struct.pack("!Q", length)
    sock.sendall(header + mask + bytes(b ^ mask[i % 4] for i, b in enumerate(payload)))


def record_ws_message(opcode: int, payload: bytes, stats: Stats, has_initial_state: bool) -> bool:
    """Count complete application messages separately from usable state traffic."""
    with stats.lock:
        stats.ws_messages += 1
        stats.ws_bytes += len(payload)
    if opcode != 0x1:  # Pulse sends state as JSON text, not binary/control frames.
        return has_initial_state
    try:
        message = json.loads(payload)
    except (ValueError, UnicodeError):
        return has_initial_state
    if not isinstance(message, dict) or not isinstance(message.get("data"), dict) or not message["data"]:
        return has_initial_state
    with stats.lock:
        if message.get("type") == "initialState":
            stats.ws_initial_states += 1
            has_initial_state = True
        elif message.get("type") == "rawData" and has_initial_state:
            stats.ws_state_updates += 1
        else:
            return has_initial_state
        stats.ws_state_bytes += len(payload)
    return has_initial_state


def run_dashboard(host: str, port: int, deadline: float, stats: Stats) -> None:
    """One subscribed dashboard: read every frame, answer pings, reconnect."""
    while not STOP.is_set() and time.time() < deadline:
        sock = None
        try:
            with socket.create_connection((host, port), timeout=30) as sock:
                with DASHBOARD_LOCK:
                    DASHBOARD_SOCKETS.add(sock)
                if STOP.is_set():
                    break
                key = base64.b64encode(os.urandom(16)).decode()
                sock.sendall((
                    f"GET /ws HTTP/1.1\r\nHost: {host}:{port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
                    f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\nOrigin: http://{host}:{port}\r\n\r\n"
                ).encode())
                response = b""
                while b"\r\n\r\n" not in response:
                    response += ws_read_exact(sock, 1)
                if b" 101 " not in response.split(b"\r\n", 1)[0]:
                    raise ConnectionError(response.split(b"\r\n", 1)[0].decode(errors="replace"))
                with stats.lock:
                    stats.ws_connects += 1
                has_initial_state = False  # A reconnect needs its own baseline.
                message_opcode = None
                fragments = bytearray()
                while not STOP.is_set() and time.time() < deadline:
                    first, second = ws_read_exact(sock, 2)
                    final, opcode, length = bool(first & 0x80), first & 0x0F, second & 0x7F
                    if length == 126:
                        length = struct.unpack("!H", ws_read_exact(sock, 2))[0]
                    elif length == 127:
                        length = struct.unpack("!Q", ws_read_exact(sock, 8))[0]
                    payload = ws_read_exact(sock, length)
                    if opcode == 0x9:
                        ws_send(sock, 0xA, payload)
                    elif opcode == 0x8:
                        raise ConnectionError("server closed the websocket")
                    elif opcode == 0xA:
                        continue
                    elif opcode in (0x1, 0x2, 0x0):
                        if opcode != 0x0:
                            if message_opcode is not None:
                                raise ValueError("new websocket message before final continuation")
                            message_opcode = opcode
                        elif message_opcode is None:
                            raise ValueError("websocket continuation without a message")
                        fragments.extend(payload)
                        if final:
                            has_initial_state = record_ws_message(message_opcode, bytes(fragments), stats, has_initial_state)
                            message_opcode = None
                            fragments.clear()
        except (OSError, ConnectionError, ValueError) as err:
            if STOP.is_set():
                break
            with stats.lock:
                if len(stats.ws_errors) < 20:
                    stats.ws_errors.append(f"{time.strftime('%H:%M:%S')} {err}")
            STOP.wait(5)
        finally:
            with DASHBOARD_LOCK:
                DASHBOARD_SOCKETS.discard(sock)


def latency_summary(samples: list[float]) -> dict[str, float | None]:
    """Median, 95th percentile and maximum in milliseconds, failures included."""
    if not samples:
        return {"p50_ms": None, "p95_ms": None, "max_ms": None}
    ordered = sorted(samples)
    pick = lambda q: ordered[min(len(ordered) - 1, int(q * len(ordered)))]
    return {"p50_ms": round(1000 * pick(0.5), 1), "p95_ms": round(1000 * pick(0.95), 1),
            "max_ms": round(1000 * ordered[-1], 1)}


def wait_ready(base: str, timeout: float) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if request(f"{base}/api/health")[0] == 200:
            return
        time.sleep(1)
    raise SystemExit(f"Pulse at {base} was not healthy within {timeout:g}s")


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--agents", type=int, default=30)
    parser.add_argument("--docker-agents", type=int, default=6)
    parser.add_argument("--containers", type=int, default=40)
    parser.add_argument("--interval", type=int, default=30)
    parser.add_argument("--duration", type=float, required=True, help="seconds to drive load")
    parser.add_argument("--seed", type=int, default=2199)
    parser.add_argument("--stats", required=True, help="path of the JSON stats file to write")
    args = parser.parse_args(argv)

    signal.signal(signal.SIGTERM, lambda *_: STOP.set())
    base = f"http://{args.host}:{args.port}"
    wait_ready(base, 120)
    rng = random.Random(args.seed)
    agents = [
        Agent(i, mint_token(base, f"footprint-{i:03d}"), args.containers if i < args.docker_agents else 0,
              args.interval, random.Random(rng.randrange(1 << 32)))
        for i in range(args.agents)
    ]
    stats = Stats()
    started = time.time()
    deadline = started + args.duration
    threads = [threading.Thread(target=run_dashboard, args=(args.host, args.port, deadline, stats), daemon=True)]
    threads += [threading.Thread(target=run_agent, args=(base, a, deadline, stats), daemon=True) for a in agents]
    for thread in threads:
        thread.start()
    while not STOP.is_set() and time.time() < deadline:
        STOP.wait(1)
    STOP.set()
    with DASHBOARD_LOCK:
        for sock in DASHBOARD_SOCKETS:
            try:
                sock.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass
    for thread in threads:
        thread.join(timeout=15)

    accepted = sum(stats.accepted.values())
    failed = sum(stats.failed.values())
    result = {
        "agents": args.agents, "docker_agents": args.docker_agents, "containers_per_docker_agent": args.containers,
        "interval_s": args.interval, "elapsed_s": round(time.time() - started, 1),
        "accepted": stats.accepted, "failed": stats.failed, "statuses": stats.statuses, "errors": stats.errors,
        "latency": {kind: latency_summary(samples) for kind, samples in stats.latencies.items()},
        "dashboard": {"connects": stats.ws_connects, "messages": stats.ws_messages, "bytes": stats.ws_bytes,
                      "initial_states": stats.ws_initial_states, "state_updates": stats.ws_state_updates,
                      "state_messages": stats.ws_initial_states + stats.ws_state_updates, "state_bytes": stats.ws_state_bytes,
                      "errors": stats.ws_errors},
    }
    with open(args.stats, "w") as handle:
        json.dump(result, handle, indent=2)
        handle.write("\n")
    if accepted == 0 or failed > 0.01 * (accepted + failed):
        print(f"agent load did not exercise the report path: {accepted} accepted, {failed} failed", file=sys.stderr)
        return 1
    if stats.ws_connects == 0 or stats.ws_initial_states == 0 or stats.ws_state_updates == 0:
        print(f"agent load did not exercise the dashboard state path: {stats.ws_connects} connections, "
              f"{stats.ws_initial_states} initial states, {stats.ws_state_updates} state updates", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
