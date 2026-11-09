#!/usr/bin/env python3
"""Run the footprint agent load driver against a small fake Pulse."""

import base64
import gzip
import hashlib
import importlib.util
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import struct
import sys
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("agent_load", ROOT / "scripts/memory/agent_load.py")
agent_load = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = agent_load  # dataclasses resolve annotations through sys.modules
SPEC.loader.exec_module(agent_load)

WS_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC11B65"


class FakePulse(BaseHTTPRequestHandler):
    tokens: list[str] = []
    reports: list[tuple[str, str, dict]] = []
    pongs: list[bytes] = []
    reject_reports = False
    dashboard_mode = "state"
    lock = threading.Lock()

    def log_message(self, *_):
        pass

    def reply(self, status, body):
        data = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/api/health":
            self.reply(200, {"status": "healthy"})
        elif self.path == "/ws":
            if self.dashboard_mode == "reject":
                self.reply(403, {"error": "dashboard rejected"})
            else:
                self.websocket()
        else:
            self.reply(404, {})

    def do_POST(self):
        body = self.rfile.read(int(self.headers["Content-Length"]))
        if self.path == "/api/agent-install-command":
            token = f"token-{len(self.tokens):03d}"
            with self.lock:
                self.tokens.append(token)
            self.reply(200, {"command": "", "token": token})
            return
        if self.path in ("/api/agents/agent/report", "/api/agents/docker/report"):
            if self.headers.get("Content-Encoding") == "gzip":
                body = gzip.decompress(body)
            with self.lock:
                self.reports.append((self.path, self.headers.get("X-API-Token", ""), json.loads(body)))
            self.reply(400 if self.reject_reports else 200, {"success": not self.reject_reports})
            return
        self.reply(404, {})

    def websocket(self):
        accept = base64.b64encode(hashlib.sha1((self.headers["Sec-WebSocket-Key"] + WS_GUID).encode()).digest()).decode()
        self.send_response(101)
        self.send_header("Upgrade", "websocket")
        self.send_header("Connection", "Upgrade")
        self.send_header("Sec-WebSocket-Accept", accept)
        self.end_headers()
        mode = self.dashboard_mode
        if mode == "idle":
            self.rfile.read(1)
            return
        welcome = json.dumps({"type": "welcome", "data": {"message": "connected"}}).encode()
        initial = json.dumps({"type": "initialState", "data": {"hosts": []}}).encode()
        update = json.dumps({"type": "rawData", "data": {"hosts": [{"id": "host-0", "padding": "x" * 300}]}}).encode()
        if mode != "control-only":
            self.frame(0x1, welcome)
        if mode in ("state", "fragmented", "initial-only", "empty-update", "invalid-state"):
            self.frame(0x1, initial)
        if mode in ("state", "raw-only"):
            self.frame(0x1, update)
        elif mode == "empty-update":
            self.frame(0x1, b'{"type":"rawData","data":{}}')
        elif mode == "invalid-state":
            for data in (None, [], "not state", 1):
                self.frame(0x1, json.dumps({"type": "rawData", "data": data}).encode())
            self.frame(0x1, b'not JSON: rawData')
            self.frame(0x1, b'[]')
            self.frame(0x2, update)  # Binary bytes are not Pulse's JSON text protocol.
        elif mode == "fragmented":
            # Exercise continuation and both extended lengths, with a ping in
            # the middle. Count the assembled state once, not each frame.
            update = json.dumps({"type": "rawData", "data": {"padding": "x" * 70000}}).encode()
            self.frame(0x1, update[:10], final=False)
            self.frame(0x9, b"ping")
            self.frame(0x0, update[10:300], final=False)
            self.frame(0x0, update[300:])
        self.frame(0xA, b"pong")  # Unsolicited pong is not a state message.
        if mode != "fragmented":
            self.frame(0x9, b"ping")
        self.wfile.flush()
        first, second = self.rfile.read(2)
        mask = self.rfile.read(4)
        payload = bytes(b ^ mask[i % 4] for i, b in enumerate(self.rfile.read(second & 0x7F)))
        with self.lock:
            self.pongs.append(bytes([first]) + payload)
        self.rfile.read(1)  # hold the connection until the client closes it

    def frame(self, opcode, payload, final=True):
        first = (0x80 if final else 0) | opcode
        size = len(payload)
        if size < 126:
            header = bytes([first, size])
        elif size < 1 << 16:
            header = bytes([first, 126]) + struct.pack("!H", size)
        else:
            header = bytes([first, 127]) + struct.pack("!Q", size)
        self.wfile.write(header + payload)


class AgentLoadTests(unittest.TestCase):
    def setUp(self):
        FakePulse.tokens, FakePulse.reports, FakePulse.pongs = [], [], []
        FakePulse.reject_reports = False
        FakePulse.dashboard_mode = "state"
        agent_load.STOP.clear()
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), FakePulse)
        self.server.daemon_threads = True
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def drive(self, *extra):
        with tempfile.TemporaryDirectory() as tmp:
            stats_path = Path(tmp) / "stats.json"
            code = agent_load.main([
                "--port", str(self.server.server_address[1]), "--agents", "4", "--docker-agents", "2",
                "--containers", "3", "--interval", "1", "--duration", "2.5", "--stats", str(stats_path), *extra,
            ])
            return code, json.loads(stats_path.read_text())

    def test_each_agent_reports_with_its_own_token_and_identity(self):
        code, stats = self.drive()
        self.assertEqual(code, 0)
        self.assertEqual(len(FakePulse.tokens), 4)
        hosts = [(token, body) for path, token, body in FakePulse.reports if path.endswith("/agent/report")]
        docker = [(token, body) for path, token, body in FakePulse.reports if path.endswith("/docker/report")]
        self.assertGreaterEqual(stats["accepted"]["host"], 4)
        self.assertEqual(stats["accepted"]["docker"], len(docker))
        identity = {}
        for token, body in hosts:
            identity.setdefault(token, set()).add((body["host"]["hostname"], body["host"]["machineId"]))
        self.assertEqual(set(identity), set(FakePulse.tokens))
        self.assertTrue(all(len(pairs) == 1 for pairs in identity.values()))
        self.assertEqual(len({next(iter(pairs)) for pairs in identity.values()}), 4)
        self.assertEqual({token for token, _ in docker}, set(FakePulse.tokens[:2]))
        self.assertTrue(all(len(body["containers"]) == 3 for _, body in docker))

    def test_sequence_ids_increase_per_agent(self):
        self.drive()
        by_token = {}
        for path, token, body in FakePulse.reports:
            if path.endswith("/agent/report"):
                by_token.setdefault(token, []).append(int(body["sequenceId"].rsplit(":", 1)[1]))
        for sequence in by_token.values():
            self.assertEqual(sequence, sorted(sequence))
            self.assertEqual(len(set(sequence)), len(sequence))

    def test_dashboard_counts_frames_and_answers_pings(self):
        code, stats = self.drive()
        self.assertEqual(code, 0)
        self.assertGreaterEqual(stats["dashboard"]["connects"], 1)
        self.assertEqual(stats["dashboard"]["messages"], 3)
        self.assertEqual(stats["dashboard"]["initial_states"], 1)
        self.assertEqual(stats["dashboard"]["state_updates"], 1)
        self.assertEqual(stats["dashboard"]["state_messages"], 2)
        self.assertGreater(stats["dashboard"]["bytes"], stats["dashboard"]["state_bytes"])
        self.assertIn(bytes([0x8A]) + b"ping", FakePulse.pongs)
        self.assertFalse(agent_load.DASHBOARD_SOCKETS)

    def test_reports_without_live_state_traffic_fail_with_stats_preserved(self):
        for mode in ("reject", "idle", "control-only", "welcome", "initial-only", "raw-only", "empty-update", "invalid-state"):
            with self.subTest(mode=mode):
                FakePulse.dashboard_mode = mode
                agent_load.STOP.clear()
                code, stats = self.drive("--duration", "1.2")
                self.assertGreater(stats["accepted"]["host"], 0)
                self.assertEqual(sum(stats["failed"].values()), 0)
                # This is the false-success path: accepted reports alone
                # must not turn an unexercised broadcast path green.
                self.assertEqual(code, 1)
                self.assertEqual(stats["dashboard"]["state_updates"], 0)
                self.assertEqual(stats["dashboard"]["connects"], 0 if mode == "reject" else 1)

    def test_fragmented_state_with_interleaved_ping_is_one_update(self):
        FakePulse.dashboard_mode = "fragmented"
        code, stats = self.drive()
        self.assertEqual(code, 0)
        self.assertEqual(stats["dashboard"]["messages"], 3)
        self.assertEqual(stats["dashboard"]["state_messages"], 2)
        self.assertEqual(stats["dashboard"]["state_updates"], 1)
        self.assertGreater(stats["dashboard"]["state_bytes"], 70000)
        self.assertIn(bytes([0x8A]) + b"ping", FakePulse.pongs)

    def test_reconnect_does_not_reuse_another_connections_initial_state(self):
        stats = agent_load.Stats()
        initial = b'{"type":"initialState","data":{"hosts":[]}}'
        update = b'{"type":"rawData","data":{"timestamp":"later"}}'
        self.assertTrue(agent_load.record_ws_message(0x1, initial, stats, False))
        self.assertFalse(agent_load.record_ws_message(0x1, update, stats, False))
        self.assertEqual(stats.ws_state_updates, 0)
        self.assertTrue(agent_load.record_ws_message(0x1, update, stats, True))
        self.assertEqual(stats.ws_state_updates, 1)

    def test_rejected_reports_fail_the_run(self):
        FakePulse.reject_reports = True
        code, stats = self.drive()
        self.assertEqual(code, 1)
        self.assertEqual(sum(stats["accepted"].values()), 0)
        self.assertEqual(stats["statuses"].keys(), {"400"})


if __name__ == "__main__":
    unittest.main()
