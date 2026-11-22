#!/usr/bin/env python3
"""Exercise the inventory served and observed by the Proxmox memory watch."""

from collections import Counter
import json
from pathlib import Path
import sys
import threading
import unittest
from urllib.request import urlopen

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "memory"))
import proxmox_load as load


class ProxmoxMemoryLoadTests(unittest.TestCase):
    def test_cluster_inventory_matches_estate_and_node_lists(self):
        counters = Counter()
        nodes = ["pve1", "pve2", "pve3"]
        handler, guest_refs = load.make_pve_handler(nodes, 6, 5, counters, threading.Lock())
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

    def setUp(self):
        self.expected = {("qemu", "vm-100"), ("lxc", "ct-101")}
        self.stats = load.DashboardStats(expected_guests=self.expected)
        self.initial = False

    def frame(self, message_type, **data):
        payload = json.dumps({"type": message_type, "data": data}).encode()
        self.initial = load.record_ws_message(1, payload, self.stats, self.initial)

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


if __name__ == "__main__":
    unittest.main()
