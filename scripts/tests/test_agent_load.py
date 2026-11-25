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
import time
import unittest
from unittest.mock import patch


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

    def test_warmup_dashboard_cannot_validate_steady_memory_comparison(self):
        since = int(time.time()) - 1
        until = since + 5
        record = agent_load.record_ws_message
        with patch.object(agent_load, "record_ws_message", side_effect=lambda *args: record(*args, observed_at=since - 1)):
            code, stats = self.drive(
                "--duration", "5", "--steady-since", str(since), "--sample-until", str(until),
            )
        self.assertEqual(code, 1)
        self.assertEqual(stats["dashboard"]["state_updates"], 1)
        observed = stats["steady_traffic"]
        self.assertTrue(observed["end_reached"])
        self.assertEqual(observed["dashboard_state_updates"], 0)
        self.assertEqual(observed["reports"]["host"]["agents_with_repeated_acceptance"], 4)
        self.assertEqual(observed["reports"]["docker"]["agents_with_repeated_acceptance"], 2)
        self.assertFalse(observed["complete"])

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


class AgentSteadyTrafficTests(unittest.TestCase):
    @staticmethod
    def window():
        return agent_load.SteadyTraffic(100, 200, {"host": {"machine-0": 0, "machine-1": 1}, "docker": {"ctr-0": 0}})

    @staticmethod
    def receive(stats, data, at, *, kind="rawData", connected=True):
        return agent_load.record_ws_message(
            0x1, json.dumps({"type": kind, "data": data}).encode(), stats, connected, observed_at=at,
        )

    def test_checks_each_report_stream_without_counting_warmup_or_late_responses(self):
        window = self.window()
        stats = agent_load.Stats(steady=window)
        for kind, index, status, at in (
            ("host", 0, 200, 100), ("host", 0, 202, 130), ("host", 0, 200, 160),
            ("host", 1, 200, 99.9), ("host", 1, 200, 101), ("host", 1, 200, 200),
            ("docker", 0, 200, 110), ("docker", 0, 503, 125),
        ):
            stats.record(kind, status, agent_index=index, observed_at=at)

        initial = b'{"type":"initialState","data":{"hosts":[]}}'
        update = b'{"type":"rawData","data":{"hosts":[{"id":"fixture"}]}}'

        def observe(message, initialized, at):
            return agent_load.record_ws_message(0x1, message, stats, initialized, observed_at=at)

        initialized = observe(initial, False, 90)
        observe(update, initialized, 99)
        observe(update, False, 150)  # A different connection has not initialized.
        observe(update, initialized, 200)
        partial = window.report(200)
        self.assertEqual(partial["dashboard_state_updates"], 0)
        self.assertEqual(partial["reports"]["host"], {
            "expected": 2, "accepted": 4, "agents_with_repeated_acceptance": 1,
            "agents_with_repeated_state_advances": 0,
        })
        self.assertEqual(partial["reports"]["docker"], {
            "expected": 1, "accepted": 1, "agents_with_repeated_acceptance": 0,
            "agents_with_repeated_state_advances": 0,
        })
        self.assertFalse(partial["complete"])

        stats.record("host", 200, agent_index=1, observed_at=190)
        stats.record("docker", 200, agent_index=0, observed_at=190)
        observe(update, initialized, 199.9)
        # The old warrant accepted this: both streams have repeated HTTP
        # success and rawData advanced, but not one matching row was applied.
        unrelated = window.report(200)
        self.assertEqual(unrelated["dashboard_state_updates"], 1)
        self.assertEqual(unrelated["reports"]["host"]["agents_with_repeated_acceptance"], 2)
        self.assertEqual(unrelated["reports"]["docker"]["agents_with_repeated_acceptance"], 1)
        self.assertFalse(unrelated["complete"])

    def test_requires_separate_advances_for_each_configured_stream(self):
        window = self.window()
        stats = agent_load.Stats(steady=window)
        for at in (112, 152):
            stats.record("host", 200, agent_index=0, observed_at=at)
            stats.record("host", 202, agent_index=1, observed_at=at)
            stats.record("docker", 200, agent_index=0, observed_at=at)

        self.receive(stats, {"resources": [
            {"id": "live-0", "agent": {"machineId": "machine-0"}},
            {"id": "live-1", "agent": {"machineId": "machine-1"}},
            {"id": "container", "docker": {"containerId": "ctr-0"}},
        ]}, 99, kind="initialState", connected=False)

        def update(report_time, at):
            self.receive(stats, {"resourceDelta": {"upserts": [
                {"id": "live-0", "agent": {"lastReportAt": report_time}},
                # An unrelated source can update a shared host row's sighting.
                {"id": "live-1", "lastSeen": at * 1000},
                {"id": "container", "lastSeen": at * 1000 - 1},
                {"id": "unrelated", "agent": {"machineId": "another", "lastReportAt": report_time}},
            ]}}, at)

        update("1970-01-01T00:01:39Z", 110)  # A warmup sighting replayed in a live frame.
        update("1970-01-01T00:02:00.123456789Z", 121)
        update("1970-01-01T00:02:00.123456789Z", 130)  # Same applied state in another frame.
        update("1970-01-01T00:02:20Z", 135)  # A time ahead of this observation.
        partial = window.report(200)
        self.assertEqual(partial["reports"]["host"]["agents_with_repeated_state_advances"], 0)
        self.assertEqual(partial["reports"]["docker"]["agents_with_repeated_state_advances"], 1)
        self.assertFalse(partial["complete"])
        update("1970-01-01T00:02:20Z", 145)
        self.assertEqual(window.report(200)["reports"]["host"]["agents_with_repeated_state_advances"], 1)
        self.assertFalse(window.report(200)["complete"])

        for report_time, at in (("00:02:36", 158), ("00:03:00", 181)):
            self.receive(stats, {"resourceDelta": {"upserts": [
                {"id": "live-1", "agent": {"lastReportAt": f"1970-01-01T{report_time}Z"}},
            ]}}, at)
        self.assertFalse(window.report(199.9)["complete"])
        self.assertTrue(window.report(200)["complete"])

    def test_discards_removed_or_invalidated_dashboard_identities(self):
        window = self.window()
        stats = agent_load.Stats(steady=window)
        host = {"id": "live", "agent": {"machineId": "machine-0", "lastReportAt": "1970-01-01T00:02:00Z"}}
        patch = {"id": "live", "agent": {"lastReportAt": "1970-01-01T00:02:10Z"}}
        self.receive(stats, {"resources": [host]}, 121, kind="initialState", connected=False)
        self.receive(stats, {"resourceDelta": {"removed": ["live"], "upserts": [patch]}}, 131)
        self.assertFalse(window.applied["host"])
        self.receive(stats, {"resources": [host]}, 132)
        self.assertEqual(window.applied["host"], {0: 1})
        connected = self.receive(stats, {}, 133, kind="stateTooLarge")
        self.assertFalse(connected)
        self.assertFalse(self.receive(stats, {"resourceDelta": {"upserts": [patch]}}, 134, connected=connected))
        self.assertEqual(window.applied["host"], {0: 1})
        connected = self.receive(stats, {"resources": [host]}, 135, connected=connected)
        self.assertTrue(connected)
        self.assertEqual(window.applied["host"], {0: 1})
        self.receive(stats, {"resourceDelta": {"upserts": [patch]}}, 136, connected=connected)
        self.assertEqual(window.applied["host"], {0: 2})

    def test_no_docker_stream_is_not_a_missing_docker_stream(self):
        window = agent_load.SteadyTraffic(
            100, 200, {"host": {"machine-0": 0}, "docker": {}}, accepted={"host": {0: 2}, "docker": {}},
            dashboard_updates=2, applied={"host": {0: 2}, "docker": {}},
        )
        result = window.report(200)
        self.assertTrue(result["complete"])
        self.assertEqual(result["reports"]["docker"]["expected"], 0)


if __name__ == "__main__":
    unittest.main()
