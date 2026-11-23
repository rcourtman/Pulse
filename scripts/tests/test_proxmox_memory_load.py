#!/usr/bin/env python3
"""Exercise PVE guest/archive and PBS snapshot coverage in the memory watch."""

from collections import Counter
import hashlib
import json
from pathlib import Path
import sys
import threading
import unittest
from unittest.mock import Mock, patch
from urllib.error import HTTPError
from urllib.request import urlopen

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "memory"))
import proxmox_load as load


END = 1_795_305_900  # Synthetic end of RSS sampling, Unix seconds.


class ProxmoxMemoryLoadTests(unittest.TestCase):
    def test_cluster_inventory_matches_estate_and_node_lists(self):
        counters = Counter()
        nodes = ["pve1", "pve2", "pve3"]
        handler, guest_refs, _ = load.make_pve_handler(
            nodes, 6, 5, counters, threading.Lock(),
            archive_as_of=END, steady_since=END - 300, sample_until=END,
        )
        with load.ThreadedHTTPServer(("127.0.0.1", 0), handler) as server:
            worker = threading.Thread(target=server.serve_forever, daemon=True)
            worker.start()
            try:
                def get(path):
                    with urlopen(f"http://127.0.0.1:{server.server_port}/api2/json{path}", timeout=5) as response:
                        self.assertEqual(response.status, 200)
                        return json.load(response)["data"]

                inventory = get("/cluster/resources?type=vm")
                self.assertEqual(Counter(row["type"] for row in inventory), {"qemu": 20, "lxc": 15})
                self.assertEqual(len({row["vmid"] for row in inventory}), 35)
                self.assertEqual(
                    {(row["node"], "vm" if row["type"] == "qemu" else "ct", row["vmid"], row["name"])
                     for row in inventory},
                    set(guest_refs),
                )
                for row in inventory:
                    self.assertEqual(row["id"], f"{row['type']}/{row['vmid']}")
                    self.assertEqual(row["status"], "running")
                    self.assertGreater(row["maxcpu"], 0)
                    self.assertGreater(row["mem"], 0)
                    self.assertGreater(row["diskread"], 0)
                for node in nodes:
                    for kind in ("qemu", "lxc"):
                        self.assertEqual(
                            {row["vmid"] for row in get(f"/nodes/{node}/{kind}")},
                            {r["vmid"] for r in inventory if r["node"] == node and r["type"] == kind},
                        )
                self.assertEqual(counters["pve_requests"], 7)
            finally:
                server.shutdown()
                worker.join(timeout=5)

    def test_shared_archive_returns_a_large_mixed_response_with_history_per_guest(self):
        nodes = ["pve1", "pve2", "pve3"]
        for vm_count, lxc_count, guest_count in ((6, 5, 35), (60, 50, 330)):
            with self.subTest(guests=guest_count):
                handler, guest_refs, archive = load.make_pve_handler(
                    nodes, vm_count, lxc_count, Counter(), threading.Lock(),
                    archive_as_of=END, steady_since=END - 300, sample_until=END,
                )
                handled = threading.Event()

                class ObservedHandler(handler):
                    def do_GET(self):
                        try:
                            super().do_GET()
                        finally:
                            handled.set()

                with load.ThreadedHTTPServer(("127.0.0.1", 0), ObservedHandler) as server:
                    worker = threading.Thread(target=server.serve_forever, daemon=True)
                    worker.start()
                    try:
                        def get(node, tail):
                            path = f"/api2/json/nodes/{node}/storage{tail}"
                            handled.clear()
                            try:
                                with urlopen(f"http://127.0.0.1:{server.server_port}{path}", timeout=5) as response:
                                    self.assertEqual(response.status, 200)
                                    body = response.read()
                                    self.assertEqual(int(response.headers["Content-Length"]), len(body))
                                    return body
                            finally:
                                self.assertTrue(handled.wait(5))

                        stores = json.loads(get("pve1", ""))["data"]
                        shared = next(store for store in stores if store["storage"] == "nfs-backups")
                        self.assertEqual(shared["shared"], 1)
                        self.assertEqual(set(shared["content"].split(",")), {"backup", "iso", "vztmpl"})
                        route = "/nfs-backups/content"
                        get("pve1", "/local/content")
                        with self.assertRaises(HTTPError) as missing:
                            get("not-a-node", route)
                        self.assertEqual(missing.exception.code, 404)
                        missing.exception.close()
                        with patch.object(load.time, "time_ns", return_value=(END - 301) * 1_000_000_000):
                            body = get("pve1", route)  # startup only
                        with patch.object(load.time, "time_ns", return_value=(END - 45) * 1_000_000_000):
                            self.assertEqual(get("pve2", route), body)
                            filtered = json.loads(get("pve3", route + "?content=backup"))["data"]
                        rows = json.loads(body)["data"]
                        self.assertEqual(len({row["volid"] for row in rows}), len(rows))
                        self.assertEqual(Counter(row["content"] for row in rows),
                                         {"backup": guest_count * 24, "iso": 24, "vztmpl": 24})
                        self.assertEqual(filtered, [row for row in rows if row["content"] == "backup"])
                        self.assertGreater(len(body), 128 * 1024)
                        ids = Counter(row["vmid"] for row in filtered)
                        self.assertEqual(ids, {vmid: 24 for _, _, vmid, _ in guest_refs})
                        self.assertEqual({row["format"] for row in filtered}, {"vma.zst", "tar.zst"})
                        self.assertEqual(len({row["ctime"] for row in filtered}), 24)
                        self.assertEqual(max(row["ctime"] for row in filtered), END - 3600)
                        observation = archive.report(nodes, END)
                        self.assertEqual(observation["body_bytes"], len(body))
                        self.assertEqual(observation["body_sha256"], hashlib.sha256(body).hexdigest())
                        self.assertEqual(observation["nodes"], {
                            "pve1": {"responses": 1, "steady_responses": 0},
                            "pve2": {"responses": 1, "steady_responses": 1},
                            "pve3": {"responses": 0, "steady_responses": 0},
                        })
                    finally:
                        server.shutdown()
                        worker.join(timeout=5)

    def test_failed_archive_writes_and_out_of_window_completions_do_not_count_as_steady(self):
        nodes = ["pve1"]
        handler_type, _, archive = load.make_pve_handler(
            nodes, 1, 1, Counter(), threading.Lock(),
            archive_as_of=END, steady_since=END - 300, sample_until=END,
        )
        handler = object.__new__(handler_type)
        handler.path = "/api2/json/nodes/pve1/storage/nfs-backups/content"
        handler.send_json_body = Mock(side_effect=BrokenPipeError)
        with patch.object(load.time, "time_ns", return_value=(END - 10) * 1_000_000_000):
            with self.assertRaises(BrokenPipeError):
                handler.do_GET()
        self.assertEqual(archive.report(nodes, END)["nodes"]["pve1"],
                         {"responses": 0, "steady_responses": 0})
        handler.send_json_body.side_effect = None
        for at in (END - 300, END, END + 1):
            with patch.object(load.time, "time_ns", return_value=at * 1_000_000_000):
                handler.do_GET()
        self.assertIsNone(archive.report(nodes, END - 1)["window_s"])
        self.assertIsNone(archive.report(nodes, END - 1)["nodes"]["pve1"]["steady_responses"])
        self.assertEqual(archive.report(nodes, END)["nodes"]["pve1"],
                         {"responses": 3, "steady_responses": 2})

    def test_pbs_status_empty_and_duplicate_reads_cannot_cover_snapshot_groups(self):
        guests = [("pve", "vm", 100, "vm-100"), ("pve", "ct", 101, "ct-101")]
        counters = Counter()
        lock = threading.Lock()
        handler_type, total, primary = load.make_pbs_handler("pbs1", guests, 3, counters, lock)
        handler = object.__new__(handler_type)
        handler.send_json = Mock()
        _, _, secondary = load.make_pbs_handler("pbs2", guests, 3, counters, lock)
        self.assertEqual(total, 9)

        def request(path):
            handler.path = f"/api2/json/admin/datastore/{path}"
            handler.do_GET()
            return handler.send_json.call_args.args[0]

        def gaps():
            return load.missing_snapshot_coverage({
                name: {store: row.counts() for store, row in coverage.items()}
                for name, coverage in {"pbs1": primary, "pbs2": secondary}.items()
            })

        request("store-archive/status")
        request("store-archive/groups")
        self.assertEqual(request("store-archive/snapshots?backup-type=ct&backup-id=101"), [])
        self.assertEqual(counters["pbs1_requests"], 3)
        self.assertIn("pbs1/store-archive 0/1", gaps())
        for _ in range(2):
            self.assertEqual(len(request("store-fast/snapshots?backup-type=vm&backup-id=100")), 3)
        self.assertIn("pbs1/store-fast 1/2", gaps())

        handler.send_json.side_effect = BrokenPipeError
        with self.assertRaises(BrokenPipeError):
            request("store-fast/snapshots?backup-type=ct&backup-id=101")
        self.assertIn("pbs1/store-fast 1/2", gaps())
        handler.send_json.side_effect = None

        request("store-fast/snapshots?backup-type=ct&backup-id=101")
        request("store-archive/snapshots?backup-type=vm&backup-id=100")
        self.assertEqual(gaps(), ["pbs2/store-archive 0/1", "pbs2/store-fast 0/2"])

    def test_pbs_status_window_does_not_credit_a_silent_store_with_its_siblings(self):
        start = END - 300
        window = load.PBSStatusWindow(start, END)
        expected = {}
        handlers = {}
        counters = Counter()
        lock = threading.Lock()
        for name in ("pbs1", "pbs2"):
            handler_type, _, expected[name] = load.make_pbs_handler(
                name, [("pve", "vm", 100, "vm-100")], 3, counters, lock, window,
            )
            handlers[name] = object.__new__(handler_type)
            handlers[name].send_json = Mock()

        def request(name, path, offset):
            handler = handlers[name]
            handler.path = f"/api2/json/admin/datastore/{path}"
            with patch.object(load.time, "time_ns", return_value=(start + offset) * 1_000_000_000):
                handler.do_GET()

        for offset in (-1, 0, 30, 60, 210, 270, 300, 301):
            request("pbs1", "store-fast/status", offset)
        request("pbs1", "store-archive/status", 20)
        request("pbs2", "store-fast/status", 290)
        request("pbs2", "store-archive/status", -1)
        request("pbs2", "store-archive/status/unknown", 40)
        request("pbs2", "unknown/status", 40)
        request("pbs2", "store-archive/groups", 40)
        request("pbs2", "store-archive/snapshots?backup-type=vm&backup-id=100", 40)
        handlers["pbs2"].send_json.side_effect = BrokenPipeError
        with self.assertRaises(BrokenPipeError):
            request("pbs2", "store-archive/status", 40)

        # Moving a snapshot response into the window establishes a response
        # for that group, not a per-store status response or a Pulse poll.
        self.assertEqual(expected["pbs2"]["store-archive"].counts(),
                         {"expected_groups": 1, "served_groups": 1})
        self.assertIsNone(window.report(expected, END - 1))
        self.assertIsNone(window.report({}, END))
        self.assertEqual(window.report(expected, END), {
            "window_s": 300,
            "datastores": {
                "pbs1": {
                    "store-archive": {"responses": 1, "longest_gap_ms": 280000},
                    "store-fast": {"responses": 6, "longest_gap_ms": 150000},
                },
                "pbs2": {
                    "store-archive": {"responses": 0, "longest_gap_ms": 300000},
                    "store-fast": {"responses": 1, "longest_gap_ms": 290000},
                },
            },
        })

    def setUp(self):
        self.expected = {("qemu", "vm-100"), ("lxc", "ct-101")}
        self.stats = load.DashboardStats(expected_guests=self.expected)
        self.initial = False

    def frame(self, message_type, **data):
        payload = json.dumps({"type": message_type, "data": data}).encode()
        self.initial = load.record_ws_message(1, payload, self.stats, self.initial)

    def frame_at(self, timestamp_s, message_type, **data):
        with patch.object(load.time, "time_ns", return_value=timestamp_s * 1_000_000_000):
            self.frame(message_type, **data)

    def test_state_traffic_without_guest_rows_does_not_establish_coverage(self):
        self.frame("initialState", resources=[])
        self.frame("rawData", lastUpdate=200)
        self.assertEqual(self.stats.ws_state_updates, 1)
        self.assertEqual(self.stats.missing_guests(), self.expected)

    def test_guest_coverage_follows_new_rows_partial_patches_and_removals(self):
        self.frame("initialState", resources=[{"id": "a", "technology": "qemu", "name": "vm-100"}])
        self.assertEqual(self.stats.missing_guests(), {("lxc", "ct-101")})
        self.frame("rawData", resourceDelta={"upserts": [
            {"id": "a", "cpu": {"current": 0.3}},
            {"id": "b", "technology": "lxc", "name": "ct-101"},
        ]})
        self.assertFalse(self.stats.missing_guests())
        self.frame("rawData", resourceDelta={"upserts": [{"id": "b", "technology": "docker"}], "removed": ["a"]})
        self.assertEqual(self.stats.missing_guests(), self.expected)

    def test_withheld_or_replaced_state_cannot_reuse_previous_guests(self):
        rows = [
            {"id": "a", "technology": "qemu", "name": "vm-100"},
            {"id": "b", "technology": "lxc", "name": "ct-101"},
        ]
        self.frame("initialState", resources=rows)
        self.frame("rawData", resources=[])
        self.assertEqual(self.stats.missing_guests(), self.expected)
        self.frame("rawData", resources=rows)
        self.assertFalse(self.stats.missing_guests())
        self.frame("stateTooLarge", supersedes="rawData")
        self.frame("rawData", resourceDelta={"upserts": rows})
        self.assertFalse(self.initial)
        self.assertEqual(self.stats.missing_guests(), self.expected)
        self.frame("initialState", resources=rows[:1])
        self.assertEqual(self.stats.missing_guests(), {("lxc", "ct-101")})

    def test_a_retained_roster_or_unrelated_update_does_not_prove_late_collection(self):
        old = (END - 270) * 1000
        self.frame("initialState", resources=[
            {"id": "a", "technology": "qemu", "name": "vm-100", "lastSeen": old},
            {"id": "b", "technology": "lxc", "name": "ct-101", "lastSeen": old},
        ])
        self.frame("rawData", resourceDelta={"upserts": [
            {"id": "a", "lastSeen": (END - 60) * 1000},
            {"id": "b", "updatedAt": (END - 5) * 1000},
        ]})
        self.assertFalse(self.stats.missing_guests())
        self.assertEqual(self.stats.missing_recent_guests(END - 300, END), {("lxc", "ct-101")})
        self.frame("rawData", resourceDelta={"upserts": [
            {"id": "b", "lastSeen": (END - 2) * 1000 + 321},
        ]})
        self.assertFalse(self.stats.missing_recent_guests(END - 300, END))

    def test_missing_or_invalid_times_and_previous_run_do_not_prove_a_poll(self):
        for timestamp in ((END - 31) * 1000, END * 1000 + 1, END - 1,
                          "2026-11-21T00:04:59Z", str(END * 1000), END * 1000.0, 0, True, None):
            with self.subTest(timestamp=timestamp):
                self.frame("initialState", resources=[
                    {"id": "a", "technology": "qemu", "name": "vm-100", "lastSeen": timestamp},
                    {"id": "b", "technology": "lxc", "name": "ct-101"},
                ])
                self.assertFalse(self.stats.missing_guests())
                self.assertEqual(self.stats.missing_recent_guests(END - 30, END), self.expected)

    def test_recent_sighting_stays_bound_to_the_guest_and_current_inventory(self):
        self.frame("initialState", resources=[
            {"id": "a", "technology": "qemu", "name": "vm-100", "lastSeen": (END - 9) * 1000},
            {"id": "b", "technology": "lxc", "name": "ct-101", "lastSeen": (END - 8) * 1000},
        ])
        self.frame("rawData", resourceDelta={"upserts": [{"id": "a", "cpu": {"current": 0.3}}]})
        self.assertFalse(self.stats.missing_recent_guests(END - 300, END))
        self.frame("rawData", resourceDelta={"upserts": [
            {"id": "c", "technology": "qemu", "name": "vm-100"},
        ], "removed": ["a"]})
        self.assertEqual(self.stats.missing_recent_guests(END - 300, END), {("qemu", "vm-100")})
        self.frame("rawData", resourceDelta={"upserts": [
            {"id": "b", "technology": "qemu", "name": "vm-100"},
        ]})
        self.assertEqual(self.stats.missing_recent_guests(END - 300, END), self.expected)

    def test_late_recovery_does_not_hide_a_mid_window_guest_sighting_gap(self):
        start = END - 300
        self.stats.poll_window = window = load.GuestPollWindow(start, END)
        self.frame_at(start + 10, "initialState", resources=[
            {"id": "a", "technology": "qemu", "name": "vm-100", "lastSeen": (start + 10) * 1000},
            {"id": "b", "technology": "lxc", "name": "ct-101", "lastSeen": (start + 10) * 1000},
        ])
        for offset in range(20, 300, 10):
            current_ms = (start + offset) * 1000
            lxc = {"id": "b", "updatedAt": current_ms, "cpu": {"current": 0.5}}
            if offset in (140, 290):
                lxc["lastSeen"] = current_ms
            self.frame_at(start + offset, "rawData", resourceDelta={"upserts": [
                {"id": "a", "lastSeen": current_ms}, lxc,
            ]})

        self.assertFalse(self.stats.missing_recent_guests(start, END))
        self.assertEqual(window.report(self.expected, END), {
            "window_s": 300, "limit_s": 60, "guests_with_sightings": 2, "longest_gap_ms": 150000,
            "guests_over_limit": ["lxc/ct-101"],
        })

    def test_window_boundaries_missing_guests_and_late_frames_stay_distinct(self):
        start = END - 180
        self.stats.poll_window = window = load.GuestPollWindow(start, END)
        self.frame_at(start + 30, "initialState", resources=[
            {"id": "a", "technology": "qemu", "name": "vm-100", "lastSeen": (start - 1) * 1000},
            {"id": "b", "technology": "lxc", "name": "ct-101", "lastSeen": (start + 60) * 1000},
        ])
        for offset in (60, 120):
            self.frame_at(start + offset, "rawData", resourceDelta={"upserts": [
                {"id": "a", "lastSeen": (start + offset) * 1000},
            ]})
        self.frame_at(start + 130, "rawData", resourceDelta={"upserts": [
            {"id": "a", "lastSeen": (start + 75) * 1000},
            {"id": "b", "lastSeen": (start - 1) * 1000},
        ]})
        self.frame_at(END + 1, "rawData", resourceDelta={"upserts": [
            {"id": "b", "lastSeen": END * 1000},
        ]})

        self.assertIsNone(window.report(self.expected, END - 1))
        self.assertEqual(window.report(self.expected, END), {
            "window_s": 180, "limit_s": 60, "guests_with_sightings": 1, "longest_gap_ms": 180000,
            "guests_over_limit": ["lxc/ct-101"],
        })


if __name__ == "__main__":
    unittest.main()
