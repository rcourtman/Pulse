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
        self.wfile.write(bytes([0x81, 5]) + b"hello")
        self.wfile.write(bytes([0x89, 4]) + b"ping")
        self.wfile.write(bytes([0x81, 126]) + struct.pack("!H", 300) + b"x" * 300)
        self.wfile.flush()
        first, second = self.rfile.read(2)
        mask = self.rfile.read(4)
        payload = bytes(b ^ mask[i % 4] for i, b in enumerate(self.rfile.read(second & 0x7F)))
        with self.lock:
            self.pongs.append(bytes([first]) + payload)
        self.rfile.read(1)  # hold the connection until the client closes it


class AgentLoadTests(unittest.TestCase):
    def setUp(self):
        FakePulse.tokens, FakePulse.reports, FakePulse.pongs = [], [], []
        FakePulse.reject_reports = False
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
        _, stats = self.drive()
        self.assertGreaterEqual(stats["dashboard"]["connects"], 1)
        self.assertGreaterEqual(stats["dashboard"]["messages"], 2)
        self.assertGreaterEqual(stats["dashboard"]["bytes"], 305)
        self.assertIn(bytes([0x8A]) + b"ping", FakePulse.pongs)

    def test_rejected_reports_fail_the_run(self):
        FakePulse.reject_reports = True
        code, stats = self.drive()
        self.assertEqual(code, 1)
        self.assertEqual(sum(stats["accepted"].values()), 0)
        self.assertEqual(stats["statuses"].keys(), {"400"})


if __name__ == "__main__":
    unittest.main()
