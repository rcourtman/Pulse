#!/usr/bin/env python3
"""Drive a real (non-mock) Pulse with synthetic Proxmox VE + PBS servers (#1803).

Mock mode and the Unified Agent estate never exercise Proxmox VE polling,
Proxmox Backup Server snapshot inventory, recovery.db ingestion, protection
posture refreshes, or PBS backup cloning. This driver reproduces a realistic
Proxmox homelab estate against a Pulse started with development admin bypass
and authentication configured:

- serves 1 synthetic Proxmox VE cluster (3 nodes, 20 VMs, 15 LXCs, storage,
  guest snapshots, tasks, physical disks) over local HTTP;
- serves 2 synthetic Proxmox Backup Server instances (2 datastores each,
  50 snapshots per backup group by default -> 5,200 PBS snapshots total);
- registers the PVE cluster and PBS instances via /api/config/nodes;
- keeps one WebSocket dashboard subscriber connected to /ws.

It runs until --duration elapses or SIGTERM arrives, then writes a JSON stats
file. It exits 1 when registration failed, PVE or PBS polling did not occur,
the fixture did not serve snapshots for every configured PBS group, or the
dashboard did not receive state updates and the full synthetic PVE inventory.
"""

from __future__ import annotations

import argparse
import base64
import http.server
import json
import math
import os
import signal
import socket
import socketserver
import struct
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter
from dataclasses import dataclass, field

STOP = threading.Event()
DASHBOARD_SOCKETS: set[socket.socket] = set()
DASHBOARD_LOCK = threading.Lock()


class ThreadedHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


@dataclass
class DashboardStats:
    expected_guests: set[tuple[str, str]] = field(default_factory=set)
    lock: threading.Lock = field(default_factory=threading.Lock)
    ws_connects: int = 0
    ws_messages: int = 0
    ws_bytes: int = 0
    ws_initial_states: int = 0
    ws_state_updates: int = 0
    ws_state_bytes: int = 0
    ws_errors: list[str] = field(default_factory=list)
    resource_identities: dict[str, tuple[str | None, str | None]] = field(default_factory=dict)

    def update_guests(self, data: dict) -> None:
        # Keep just the identity fields. Existing resource upserts are merge
        # patches, so a metrics-only update must not erase a guest's identity.
        if "resources" in data:
            self.resource_identities.clear()
            rows = data["resources"]
        else:
            delta = data.get("resourceDelta")
            if not isinstance(delta, dict):
                return
            removed = delta.get("removed", [])
            if not isinstance(removed, list) or not all(isinstance(key, str) for key in removed):
                self.resource_identities.clear()
                return
            for resource_id in removed:
                self.resource_identities.pop(resource_id, None)
            rows = delta.get("upserts", [])
        if not isinstance(rows, list):
            self.resource_identities.clear()
            return
        for row in rows:
            if not isinstance(row, dict) or not isinstance(row.get("id"), str) or not row["id"]:
                continue
            resource_id = row["id"]
            technology, name = self.resource_identities.get(resource_id, (None, None))
            technology = row.get("technology", technology)
            name = row.get("name", name)
            self.resource_identities[resource_id] = (
                technology if isinstance(technology, str) else None,
                name if isinstance(name, str) else None,
            )

    def missing_guests(self) -> set[tuple[str, str]]:
        return self.expected_guests.difference(self.resource_identities.values())


@dataclass
class SnapshotCoverage:
    expected_groups: int
    served_groups: set[tuple[str, str]] = field(default_factory=set)

    def counts(self) -> dict[str, int]:
        return {"expected_groups": self.expected_groups, "served_groups": len(self.served_groups)}


def missing_snapshot_coverage(coverage: dict[str, dict[str, dict[str, int]]]) -> list[str]:
    return [
        f"{instance}/{store} {row['served_groups']}/{row['expected_groups']}"
        for instance, stores in sorted(coverage.items())
        for store, row in sorted(stores.items())
        if row["served_groups"] < row["expected_groups"]
    ]


def make_pve_handler(
    nodes: list[str],
    vms_per_node: int,
    lxcs_per_node: int,
    counters: Counter,
    lock: threading.Lock,
):
    guests_by_node: dict[str, tuple[list[dict], list[dict]]] = {}
    vmid = 100
    for idx, node in enumerate(nodes):
        vms = []
        lxcs = []
        # Default 3 nodes with vms_per_node=6, lxcs_per_node=5 -> 7+7+6=20 VMs, 15 LXCs (35 guests).
        v_count = vms_per_node + (1 if idx < 2 and len(nodes) == 3 and vms_per_node == 6 else 0)
        for _ in range(v_count):
            vms.append({
                "vmid": vmid,
                "name": f"vm-{vmid}",
                "status": "running",
                "cpus": 2,
                "maxmem": 4 * 1024**3,
                "maxdisk": 64 * 1024**3,
                "uptime": 864000,
                "template": 0,
                "tags": "prod;web",
            })
            vmid += 1
        for _ in range(lxcs_per_node):
            lxcs.append({
                "vmid": vmid,
                "name": f"ct-{vmid}",
                "status": "running",
                "cpus": 1,
                "maxmem": 1 * 1024**3,
                "maxdisk": 16 * 1024**3,
                "uptime": 432000,
                "template": 0,
                "tags": "infra",
            })
            vmid += 1
        guests_by_node[node] = (vms, lxcs)

    all_guest_refs: list[tuple[str, str, int, str]] = []
    for node, (vms, lxcs) in guests_by_node.items():
        for guest in vms:
            all_guest_refs.append((node, "vm", guest["vmid"], guest["name"]))
        for guest in lxcs:
            all_guest_refs.append((node, "ct", guest["vmid"], guest["name"]))

    def guest_payload(guest: dict, kind: str, wave: float, now: int) -> dict:
        is_vm = kind == "qemu"
        io_scale = 4 if is_vm else 1
        return {
            **guest,
            "cpu": wave * (0.5 if is_vm else 0.3),
            "mem": int(guest["maxmem"] * ((0.4 if is_vm else 0.3) + 0.1 * wave)),
            "disk": int(guest["maxdisk"] * (0.35 if is_vm else 0.25)),
            "diskread": now * 1024 * io_scale,
            "diskwrite": now * 512 * io_scale,
            "netin": now * 2048 * io_scale,
            "netout": now * 1024 * io_scale,
        }

    class PVEHandler(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, format: str, *args) -> None:  # noqa: A002
            return

        def send_json(self, payload: object, status: int = 200) -> None:
            body = json.dumps({"data": payload}).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self) -> None:  # noqa: N802
            parsed = urllib.parse.urlparse(self.path)
            path = parsed.path
            if path.startswith("/api2/json"):
                path = path[len("/api2/json"):]
            with lock:
                counters["pve_requests"] += 1

            now = int(time.time())
            wave = 0.25 + 0.15 * math.sin(now / 30.0)

            if path == "/version":
                return self.send_json({"version": "8.3.1", "release": "8.3", "repoid": "synthetic"})
            if path == "/cluster/status":
                data = [
                    {
                        "type": "cluster",
                        "name": "footprint-cluster",
                        "id": "cluster",
                        "nodes": len(nodes),
                        "quorate": 1,
                    }
                ]
                for i, n in enumerate(nodes, 1):
                    data.append({
                        "type": "node",
                        "name": n,
                        "id": f"node/{n}",
                        "nodeid": i,
                        "online": 1,
                        "local": 1 if i == 1 else 0,
                        "ip": "127.0.0.1",
                    })
                return self.send_json(data)
            if path == "/cluster/options":
                return self.send_json({})
            if path == "/cluster/resources" and urllib.parse.parse_qs(parsed.query).get("type") == ["vm"]:
                # Pulse treats a successful empty response as authoritative,
                # not as a request to fall back to the per-node guest lists.
                return self.send_json([
                    {
                        **guest_payload(guest, kind, wave, now),
                        "type": kind,
                        "id": f"{kind}/{guest['vmid']}",
                        "node": node,
                        "maxcpu": guest["cpus"],
                    }
                    for node, (vms, lxcs) in guests_by_node.items()
                    for kind, guests in (("qemu", vms), ("lxc", lxcs))
                    for guest in guests
                ])
            if path == "/nodes":
                return self.send_json([
                    {
                        "node": n,
                        "status": "online",
                        "cpu": wave,
                        "maxcpu": 16,
                        "mem": int(24 * 1024**3 * (0.8 + 0.1 * wave)),
                        "maxmem": 64 * 1024**3,
                        "disk": 120 * 1024**3,
                        "maxdisk": 500 * 1024**3,
                        "uptime": 1200000 + now % 10000,
                        "level": "",
                    }
                    for n in nodes
                ])
            if path == "/storage":
                return self.send_json([
                    {"storage": "local", "type": "dir", "content": "backup,iso,vztmpl", "shared": 0},
                    {"storage": "local-zfs", "type": "zfspool", "content": "images,rootdir", "shared": 0},
                    {"storage": "nfs-backups", "type": "nfs", "content": "backup", "shared": 1},
                    {"storage": "pbs-primary", "type": "pbs", "content": "backup", "shared": 1},
                ])

            parts = [p for p in path.split("/") if p]
            if len(parts) >= 3 and parts[0] == "nodes" and parts[1] in guests_by_node:
                node = parts[1]
                vms, lxcs = guests_by_node[node]
                sub = parts[2]
                if sub == "status":
                    return self.send_json({
                        "cpu": wave,
                        "uptime": 1200000,
                        "loadavg": ["0.85", "0.72", "0.68"],
                        "memory": {"total": 64 * 1024**3, "used": 24 * 1024**3, "free": 40 * 1024**3},
                        "rootfs": {"total": 500 * 1024**3, "used": 120 * 1024**3, "avail": 380 * 1024**3},
                        "cpuinfo": {"cpus": 16, "cores": 8, "sockets": 1, "model": "AMD Ryzen 7"},
                        "pveversion": "pve-manager/8.3.1",
                        "kversion": "Linux 6.8.12-pve",
                    })
                if sub in ("qemu", "lxc") and len(parts) == 3:
                    return self.send_json([
                        guest_payload(guest, sub, wave, now)
                        for guest in (vms if sub == "qemu" else lxcs)
                    ])
                if sub == "storage" and len(parts) == 3:
                    return self.send_json([
                        {
                            "storage": "local",
                            "type": "dir",
                            "content": "backup,iso",
                            "active": 1,
                            "enabled": 1,
                            "shared": 0,
                            "total": 500 * 1024**3,
                            "used": 120 * 1024**3,
                            "avail": 380 * 1024**3,
                        },
                        {
                            "storage": "local-zfs",
                            "type": "zfspool",
                            "content": "images,rootdir",
                            "active": 1,
                            "enabled": 1,
                            "shared": 0,
                            "total": 2000 * 1024**3,
                            "used": 800 * 1024**3,
                            "avail": 1200 * 1024**3,
                        },
                        {
                            "storage": "nfs-backups",
                            "type": "nfs",
                            "content": "backup",
                            "active": 1,
                            "enabled": 1,
                            "shared": 1,
                            "total": 4000 * 1024**3,
                            "used": 1500 * 1024**3,
                            "avail": 2500 * 1024**3,
                        },
                    ])
                if sub == "storage" and len(parts) == 5 and parts[4] == "content":
                    if parts[3] == "nfs-backups":
                        items = [
                            {
                                "volid": f"nfs-backups:backup/vzdump-{kind}-{gid}-2026_11_20-02_00_00.vma.zst",
                                "content": "backup",
                                "format": "vma.zst",
                                "size": 512 * 1024**2,
                                "ctime": now - 3600,
                                "vmid": gid,
                            }
                            for _, kind, gid, _ in all_guest_refs[:10]
                        ]
                        return self.send_json(items)
                    return self.send_json([])
                if sub in ("qemu", "lxc") and len(parts) == 5 and parts[4] == "snapshot":
                    gid = int(parts[3])
                    return self.send_json(
                        [
                            {"name": "pre-upgrade", "description": "before update", "snaptime": now - 86400, "vmstate": 0},
                            {"name": "current", "description": "You are here!", "parent": "pre-upgrade"},
                        ]
                        if gid % 2 == 0
                        else [{"name": "current", "description": "You are here!"}]
                    )
                if sub in ("qemu", "lxc") and len(parts) == 5 and parts[4] == "config":
                    return self.send_json({"cores": 2, "memory": 4096, "net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"})
                if sub in ("qemu", "lxc") and len(parts) == 6 and parts[4] == "status" and parts[5] == "current":
                    return self.send_json({"status": "running", "cpu": wave * 0.4, "mem": 1024**3, "maxmem": 4 * 1024**3})
                if sub == "tasks" and len(parts) == 3:
                    return self.send_json([
                        {
                            "upid": f"UPID:{node}:00001234:0000ABCD:{now - 7200:08X}:vzdump:100:root@pam:",
                            "node": node,
                            "type": "vzdump",
                            "id": "100",
                            "user": "root@pam",
                            "status": "OK",
                            "starttime": now - 7200,
                            "endtime": now - 7100,
                        }
                    ])
                if sub == "disks" and len(parts) == 4 and parts[3] == "list":
                    return self.send_json([
                        {
                            "devpath": "/dev/nvme0n1",
                            "model": "Samsung 990 PRO",
                            "serial": f"SN-{node}-0",
                            "size": 2000 * 1024**3,
                            "type": "nvme",
                            "health": "PASSED",
                            "wearout": 98,
                        }
                    ])
                if sub == "disks" and len(parts) == 4 and parts[3] == "zfs":
                    return self.send_json([])
                if sub == "network":
                    return self.send_json([{"iface": "vmbr0", "type": "bridge", "active": 1}])
                if sub in ("apt", "rrddata"):
                    return self.send_json([])

            return self.send_json([])

    return PVEHandler, all_guest_refs


def make_pbs_handler(
    name: str,
    guest_refs: list[tuple[str, str, int, str]],
    snapshots_per_group: int,
    counters: Counter,
    lock: threading.Lock,
):
    datastores = ["store-fast", "store-archive"]
    base_time = int(time.time()) - snapshots_per_group * 86400

    groups_by_store: dict[str, list[dict]] = {}
    snaps_by_key: dict[tuple[str, str, str], list[dict]] = {}
    coverage: dict[str, SnapshotCoverage] = {}
    total_snapshots = 0
    for ds_idx, ds in enumerate(datastores):
        groups = []
        subset = guest_refs if ds_idx == 0 else guest_refs[: max(1, len(guest_refs) // 2)]
        for _, kind, gid, gname in subset:
            gid_str = str(gid)
            latest_ts = base_time + (snapshots_per_group - 1) * 86400
            groups.append({
                "backup-type": kind,
                "backup-id": gid_str,
                "backup-count": snapshots_per_group,
                "last-backup": latest_ts,
                "owner": "backup@pbs",
                "comment": gname,
            })
            snaps = []
            for s in range(snapshots_per_group):
                ts = base_time + s * 86400
                snaps.append({
                    "backup-type": kind,
                    "backup-id": gid_str,
                    "backup-time": ts,
                    "comment": gname,
                    "size": 2 * 1024**3 + s * 1024**2,
                    "owner": "backup@pbs",
                    "protected": s == snapshots_per_group - 1,
                    "verification": {"state": "ok", "upid": f"UPID:{name}:verify:{gid}:{s}"},
                    "files": [
                        {"filename": "index.json.blob", "size": 1024, "crypt-mode": "none"},
                        {"filename": "qemu-server.conf.blob", "size": 512, "crypt-mode": "none"},
                        {"filename": "drive-scsi0.img.fidx", "size": 2 * 1024**3, "crypt-mode": "none"},
                        {"filename": "client.log.blob", "size": 4096, "crypt-mode": "none"},
                    ],
                })
            snaps_by_key[(ds, kind, gid_str)] = snaps
            total_snapshots += len(snaps)
        groups_by_store[ds] = groups
        coverage[ds] = SnapshotCoverage(len(groups) if snapshots_per_group > 0 else 0)

    class PBSHandler(http.server.BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, format: str, *args) -> None:  # noqa: A002
            return

        def send_json(self, payload: object, status: int = 200) -> None:
            body = json.dumps({"data": payload}).encode("utf-8")
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self) -> None:  # noqa: N802
            parsed = urllib.parse.urlparse(self.path)
            path = parsed.path
            qs = urllib.parse.parse_qs(parsed.query)
            if path.startswith("/api2/json"):
                path = path[len("/api2/json"):]
            with lock:
                counters[f"{name}_requests"] += 1

            now = int(time.time())
            if path == "/version":
                return self.send_json({"version": "3.3.1", "release": "1", "repoid": "synthetic"})
            if path == "/nodes":
                return self.send_json([{"node": name}])
            if path == "/nodes/localhost/status":
                return self.send_json({
                    "cpu": 0.08,
                    "memory": {"total": 16 * 1024**3, "used": 4 * 1024**3, "free": 12 * 1024**3},
                    "uptime": 900000,
                    "loadavg": [0.2, 0.15, 0.1],
                })
            if path == "/nodes/localhost/tasks":
                return self.send_json([
                    {
                        "upid": f"UPID:{name}:00000100:00000200:00000001:{now - 3600:08X}:verifysnap:store-fast:backup@pbs:",
                        "node": name,
                        "worker_type": "verificationjob",
                        "worker_id": "store-fast:v-daily",
                        "status": "OK",
                        "starttime": now - 3600,
                        "endtime": now - 3500,
                    }
                ])
            if path == "/admin/datastore":
                return self.send_json([{"store": ds, "path": f"/mnt/{ds}"} for ds in datastores])
            if path.startswith("/admin/datastore/"):
                parts = path.split("/")
                if len(parts) >= 5:
                    ds = parts[3]
                    ep = parts[4]
                    if ep == "status":
                        return self.send_json({
                            "total": 8000 * 1024**3,
                            "used": 3200 * 1024**3,
                            "avail": 4800 * 1024**3,
                            "gc-status": {"upid": ""},
                        })
                    if ep == "gc":
                        return self.send_json({"upid": "", "last-run-state": "OK", "last-run-endtime": now - 86400})
                    if ep == "namespace":
                        return self.send_json([{"ns": "", "comment": "root"}])
                    if ep == "groups":
                        return self.send_json(groups_by_store.get(ds, []))
                    if ep == "snapshots":
                        btype = qs.get("backup-type", [""])[0]
                        bid = qs.get("backup-id", [""])[0]
                        snapshots = snaps_by_key.get((ds, btype, bid), [])
                        self.send_json(snapshots)
                        if snapshots:
                            # Count only a completed, nonempty response. Retries
                            # for the same group do not cover another group.
                            with lock:
                                coverage[ds].served_groups.add((btype, bid))
                        return
                    if ep == "rrd":
                        return self.send_json([])
            return self.send_json([])

    return PBSHandler, total_snapshots, coverage


def ws_read_exact(sock: socket.socket, count: int) -> bytes:
    buf = bytearray()
    while len(buf) < count:
        chunk = sock.recv(count - len(buf))
        if not chunk:
            raise ConnectionError("websocket closed")
        buf.extend(chunk)
    return bytes(buf)


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


def record_ws_message(opcode: int, payload: bytes, stats: DashboardStats, has_initial_state: bool) -> bool:
    with stats.lock:
        stats.ws_messages += 1
        stats.ws_bytes += len(payload)
    if opcode != 0x1:
        return has_initial_state
    try:
        message = json.loads(payload)
    except (ValueError, UnicodeError):
        return has_initial_state
    if not isinstance(message, dict):
        return has_initial_state
    if message.get("type") == "stateTooLarge":
        # Pulse invalidates the delta baseline when it withholds a state.
        with stats.lock:
            stats.resource_identities.clear()
        return False
    data = message.get("data")
    if not isinstance(data, dict) or not data:
        return has_initial_state
    with stats.lock:
        if message.get("type") == "initialState":
            stats.ws_initial_states += 1
            has_initial_state = True
            stats.resource_identities.clear()
        elif message.get("type") == "rawData" and has_initial_state:
            stats.ws_state_updates += 1
        else:
            return has_initial_state
        stats.update_guests(data)
        stats.ws_state_bytes += len(payload)
    return has_initial_state


def run_dashboard(host: str, port: int, deadline: float, stats: DashboardStats) -> None:
    while not STOP.is_set() and time.time() < deadline:
        sock = None
        try:
            with socket.create_connection((host, port), timeout=30) as sock:
                with DASHBOARD_LOCK:
                    DASHBOARD_SOCKETS.add(sock)
                if STOP.is_set():
                    break
                key = base64.b64encode(os.urandom(16)).decode()
                sock.sendall(
                    (
                        f"GET /ws HTTP/1.1\r\nHost: {host}:{port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"
                        f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\nOrigin: http://{host}:{port}\r\n\r\n"
                    ).encode()
                )
                response = b""
                while b"\r\n\r\n" not in response:
                    response += ws_read_exact(sock, 1)
                if b" 101 " not in response.split(b"\r\n", 1)[0]:
                    raise ConnectionError(response.split(b"\r\n", 1)[0].decode(errors="replace"))
                with stats.lock:
                    stats.ws_connects += 1
                    stats.resource_identities.clear()
                has_initial_state = False
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


def post_json(url: str, payload: dict) -> tuple[int, bytes]:
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=body, headers={"Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as err:
        with err:
            return err.code, err.read()


def wait_ready(base: str, timeout: float) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline and not STOP.is_set():
        try:
            with urllib.request.urlopen(f"{base}/api/health", timeout=5) as resp:
                if resp.status == 200:
                    return
        except (urllib.error.URLError, OSError):
            pass
        time.sleep(1)
    raise SystemExit(f"Pulse at {base} was not healthy within {timeout:g}s")


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, required=True)
    parser.add_argument("--nodes", type=int, default=3)
    parser.add_argument("--vms-per-node", type=int, default=6)
    parser.add_argument("--lxcs-per-node", type=int, default=5)
    parser.add_argument("--pbs-instances", type=int, default=2)
    parser.add_argument("--snapshots-per-group", type=int, default=50)
    parser.add_argument("--duration", type=float, required=True, help="seconds to drive load")
    parser.add_argument("--stats", required=True, help="path of the JSON stats file to write")
    args = parser.parse_args(argv)

    signal.signal(signal.SIGTERM, lambda *_: STOP.set())
    signal.signal(signal.SIGINT, lambda *_: STOP.set())

    counters: Counter = Counter()
    lock = threading.Lock()
    node_names = [f"pve{i + 1}" for i in range(args.nodes)]
    pve_handler, guest_refs = make_pve_handler(node_names, args.vms_per_node, args.lxcs_per_node, counters, lock)
    pve_srv = ThreadedHTTPServer(("127.0.0.1", 0), pve_handler)

    pbs_servers: list[tuple[str, ThreadedHTTPServer]] = []
    snapshot_coverage: dict[str, dict[str, SnapshotCoverage]] = {}
    total_pbs_snapshots = 0
    for idx in range(args.pbs_instances):
        name = f"pbs{idx + 1}"
        handler, snap_count, coverage = make_pbs_handler(name, guest_refs, args.snapshots_per_group, counters, lock)
        pbs_servers.append((name, ThreadedHTTPServer(("127.0.0.1", 0), handler)))
        total_pbs_snapshots += snap_count
        snapshot_coverage[name] = coverage

    servers = [pve_srv] + [srv for _, srv in pbs_servers]
    for srv in servers:
        threading.Thread(target=srv.serve_forever, daemon=True).start()

    base = f"http://{args.host}:{args.port}"
    wait_ready(base, 120)

    registrations: list[dict] = [
        {
            "type": "pve",
            "name": "footprint-pve",
            "host": f"http://127.0.0.1:{pve_srv.server_port}",
            "tokenName": "pulse@pam!monitor",
            "tokenValue": "00000000-0000-4000-8000-000000000001",
            "verifySSL": False,
            "monitorVMs": True,
            "monitorContainers": True,
            "monitorStorage": True,
            "monitorBackups": True,
            "monitorPhysicalDisks": True,
        }
    ]
    for idx, (name, srv) in enumerate(pbs_servers, start=2):
        registrations.append({
            "type": "pbs",
            "name": f"footprint-{name}",
            "host": f"http://127.0.0.1:{srv.server_port}",
            "tokenName": "pulse@pbs!monitor",
            "tokenValue": f"00000000-0000-4000-8000-{idx:012d}",
            "verifySSL": False,
            "monitorDatastores": True,
            "monitorBackups": True,
            "monitorSyncJobs": True,
            "monitorVerifyJobs": True,
            "monitorPruneJobs": True,
            "monitorGarbageJobs": True,
        })

    reg_results: dict[str, dict[str, object]] = {}
    reg_failed = False
    for payload in registrations:
        status, body = post_json(f"{base}/api/config/nodes", payload)
        reg_results[payload["name"]] = {"status": status, "body": body.decode("utf-8", "replace")[:200]}
        if not (200 <= status < 300):
            reg_failed = True

    dash_stats = DashboardStats(expected_guests={
        ("qemu" if kind == "vm" else "lxc", guest_name)
        for _, kind, _, guest_name in guest_refs
    })
    started = time.time()
    deadline = started + args.duration
    dash_thread = threading.Thread(
        target=run_dashboard, args=(args.host, args.port, deadline, dash_stats), daemon=True
    )
    dash_thread.start()

    while not STOP.is_set() and time.time() < deadline:
        STOP.wait(1)
    STOP.set()

    with DASHBOARD_LOCK:
        for sock in DASHBOARD_SOCKETS:
            try:
                sock.shutdown(socket.SHUT_RDWR)
            except OSError:
                pass
    dash_thread.join(timeout=15)

    for srv in servers:
        srv.shutdown()
        srv.server_close()

    with lock:
        req_counts = dict(counters)
        snapshot_counts = {
            name: {store: row.counts() for store, row in stores.items()}
            for name, stores in snapshot_coverage.items()
        }
    pbs_requests = sum(v for k, v in req_counts.items() if k != "pve_requests")
    pve_requests = req_counts.get("pve_requests", 0)
    with dash_stats.lock:
        missing_pve_guests = dash_stats.missing_guests()

    result = {
        "pve_nodes": args.nodes,
        "guests": len(guest_refs),
        "pve_guests_observed": len(guest_refs) - len(missing_pve_guests),
        "pbs_instances": args.pbs_instances,
        "snapshots_per_group": args.snapshots_per_group,
        "total_pbs_snapshots": total_pbs_snapshots,
        "elapsed_s": round(time.time() - started, 1),
        "registrations": reg_results,
        "requests": req_counts,
        "pve_requests": pve_requests,
        "pbs_requests": pbs_requests,
        "pbs_snapshot_group_coverage": snapshot_counts,
        "dashboard": {
            "connects": dash_stats.ws_connects,
            "messages": dash_stats.ws_messages,
            "bytes": dash_stats.ws_bytes,
            "initial_states": dash_stats.ws_initial_states,
            "state_updates": dash_stats.ws_state_updates,
            "state_messages": dash_stats.ws_initial_states + dash_stats.ws_state_updates,
            "state_bytes": dash_stats.ws_state_bytes,
            "errors": dash_stats.ws_errors,
        },
    }
    with open(args.stats, "w", encoding="utf-8") as handle:
        json.dump(result, handle, indent=2)
        handle.write("\n")

    if reg_failed:
        print(f"proxmox load registration failed: {reg_results}", file=sys.stderr)
        return 1
    if pve_requests == 0 or (args.pbs_instances > 0 and pbs_requests == 0):
        print(f"proxmox load did not exercise polling: pve={pve_requests}, pbs={pbs_requests}", file=sys.stderr)
        return 1
    gaps = missing_snapshot_coverage(snapshot_counts)
    if gaps:
        print(f"proxmox load did not serve every PBS snapshot group: {', '.join(gaps)}", file=sys.stderr)
        return 1
    if dash_stats.ws_connects == 0 or dash_stats.ws_initial_states == 0 or dash_stats.ws_state_updates == 0:
        print(
            f"proxmox load did not exercise the dashboard state path: {dash_stats.ws_connects} connections, "
            f"{dash_stats.ws_initial_states} initial states, {dash_stats.ws_state_updates} state updates",
            file=sys.stderr,
        )
        return 1
    if missing_pve_guests:
        print(
            f"proxmox load dashboard saw {result['pve_guests_observed']}/{len(guest_refs)} expected PVE guests; "
            f"missing: {', '.join(f'{kind}/{name}' for kind, name in sorted(missing_pve_guests))}",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
