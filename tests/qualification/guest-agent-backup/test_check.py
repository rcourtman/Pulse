"""Synthetic checker controls; none is native PVE/QGA acceptance."""

import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location("backup_check", Path(__file__).with_name("check.py"))
CHECK = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECK)
SOURCE = "a" * 40
HASH = "b" * 64


def witness_events(filesystem="data"):
    events = []
    for kind, at, extra in [
        ("start", 100000, {}),
        ("write", 100020, {"sequence": 1, "started_unix_ms": 100010}),
        ("heartbeat", 101000, {}),
        ("write", 101020, {"sequence": 2, "started_unix_ms": 101010}),
        ("heartbeat", 105000, {}),
        ("write", 108020, {"sequence": 3, "started_unix_ms": 108010}),
        ("write", 109020, {"sequence": 4, "started_unix_ms": 109010}),
        ("stop", 120000, {"result": "complete"}),
    ]:
        event = {"schema_version": 1, "kind": kind, "run_id": "run-01", "filesystem_id": filesystem,
                 "unix_ms": at, "elapsed_ms": at - 100000, **extra}
        if kind == "write":
            payload = f"Pulse disposable backup witness\nrun-01\n{filesystem}\n{event['sequence']}\n".encode()
            payload += b"x" * (512 - len(payload))
            event["sha256"] = hashlib.sha256(payload).hexdigest()
        events.append(event)
    return events


def record():
    baseline = {"phase": "before", "at_ms": 102000, "guest_identity_sha256": HASH, "guest_status": "available",
                "metadata_sha256": HASH, "memory_sha256": HASH, "disk_sha256": HASH,
                "memory_source": "guest-agent-meminfo", "disk_reason": "", "disk_history_latest_ms": 100020,
                "memory_history_latest_ms": 100040, "cpu_latest_ms": 100010}
    locked = {**baseline, "phase": "locked", "at_ms": 104500, "cpu_latest_ms": 104000,
              "guest_status": "deferred", "memory_source": "previous-snapshot", "disk_reason": "prev-vm-locked"}
    resumed = {**baseline, "phase": "resumed", "at_ms": 110000, "disk_history_latest_ms": 109500,
               "memory_history_latest_ms": 109500, "cpu_latest_ms": 109500}
    return {"schema_version": 1, "run_id": "run-01", "evidence_class": "native-disposable",
            "identity": {"pulse_source": SOURCE, "pulse_binary_sha256": HASH, "pve_packages_sha256": HASH,
                         "qga_binary_sha256": HASH, "haos_image_sha256": HASH, "provenance_receipt_sha256": HASH},
            "preflight": {"pve_version": "9.2.21", "guest_type": "HAOS", "audit": True, "file_read": True,
                          "tls_verified": True, "qga_enabled": True, "freeze_enabled": True,
                          "single_pulse_process": True, "filesystem_map_sha256": HASH, "filesystem_ids": ["data"]},
            "window": {"start_ms": 100000, "end_ms": 120000, "clock_error_ms": 10},
            "backup": {"start_ms": 103000, "lock_start_ms": 103200, "freeze_ms": 104000, "thaw_request_ms": 106000,
                       "lock_end_ms": 106200, "completed_ms": 107000, "status": "OK", "trace_sha256": HASH},
            "coverage": {"start_ms": 100000, "end_ms": 120000, "dropped_events": 0, "trace_sha256": HASH},
            "commands": [{"id": "request-01", "method": "get-fsinfo", "start_ms": 103100,
                          "end_ms": 103500, "result": "ok"},
                         {"id": "resume-disk", "method": "get-fsinfo", "start_ms": 108100,
                          "end_ms": 108200, "result": "ok"},
                         {"id": "resume-memory", "method": "file-read", "start_ms": 108300,
                          "end_ms": 108400, "result": "ok"}],
            "snapshots": [baseline, locked, {**locked, "at_ms": 105500, "cpu_latest_ms": 105000}, resumed],
            "witnesses": []}


class BackupRecordTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.record = record()
        self.add_witness()

    def add_witness(self, filesystem="data", events=None):
        events = witness_events(filesystem) if events is None else events
        raw = b"".join(json.dumps(e).encode() + b"\n" for e in events)
        filename = filesystem + ".jsonl"
        (self.root / filename).write_bytes(raw)
        entry = {"filesystem_id": filesystem, "file": filename, "sha256": hashlib.sha256(raw).hexdigest(), "exit_code": 0}
        self.record["witnesses"] = [w for w in self.record["witnesses"] if w["filesystem_id"] != filesystem] + [entry]

    def check(self):
        return CHECK.check_record(self.record, self.root, SOURCE)

    def test_complete_record_checks_do_not_claim_native_acceptance(self):
        result = self.check()
        self.assertTrue(result["record_checks_passed"])
        self.assertFalse(result["native_acceptance_complete"])

    def test_every_covered_filesystem_needs_an_independent_witness(self):
        self.record["preflight"]["filesystem_ids"].append("root")
        with self.assertRaisesRegex(CHECK.InvalidRecord, "witness missing"):
            self.check()
        self.add_witness("root")
        self.assertEqual(self.check()["filesystem_witnesses"], 2)

    def test_ok_task_without_post_task_writes_is_not_thaw(self):
        self.add_witness(events=[e for e in witness_events() if e.get("sequence", 0) < 3])
        with self.assertRaisesRegex(CHECK.InvalidRecord, "post-task fsync"):
            self.check()

    def test_outstanding_prefreeze_write_is_not_two_fresh_post_task_writes(self):
        events = witness_events()
        for event in events:
            if event.get("sequence") == 3:
                event["started_unix_ms"] = 102000
        self.add_witness(events=events)
        with self.assertRaisesRegex(CHECK.InvalidRecord, "post-task fsync"):
            self.check()

    def test_resumed_history_and_writes_need_both_post_task_guest_reads(self):
        original = copy.deepcopy(self.record["commands"])
        for missing in (("resume-disk",), ("resume-memory",), ("resume-disk", "resume-memory")):
            with self.subTest(missing=missing):
                self.record["commands"] = [c for c in original if c["id"] not in missing]
                with self.assertRaisesRegex(CHECK.InvalidRecord, "read supporting resumption missing"):
                    self.check()

    def test_metadata_reads_cannot_support_resumed_memory_or_filesystems(self):
        original = copy.deepcopy(self.record["commands"])
        for index in (1, 2):
            for method in ("info", "get-osinfo", "network-get-interfaces"):
                with self.subTest(index=index, method=method):
                    self.record["commands"] = copy.deepcopy(original)
                    self.record["commands"][index]["method"] = method
                    with self.assertRaisesRegex(CHECK.InvalidRecord, "read supporting resumption missing"):
                        self.check()

    def test_old_or_task_uncertain_reads_cannot_support_resumed_history(self):
        original = copy.deepcopy(self.record["commands"])
        # These all start after the lock clears, so the lock-dispatch oracle
        # cannot reject them. Only a genuinely new post-task read may qualify.
        completed = self.record["backup"]["completed_ms"]
        skew = self.record["window"]["clock_error_ms"]
        for method in ("get-fsinfo", "file-read"):
            for began in (completed - 100, completed, completed + skew):
                with self.subTest(method=method, began=began):
                    self.record["commands"] = [copy.deepcopy(original[0])]
                    self.record["commands"].extend([
                        {"id": "old-read", "method": method, "start_ms": began,
                         "end_ms": completed + 100, "result": "ok"},
                        {"id": "new-other-read", "method": "file-read" if method == "get-fsinfo" else "get-fsinfo",
                         "start_ms": 108100, "end_ms": 108200, "result": "ok"},
                    ])
                    with self.assertRaisesRegex(CHECK.InvalidRecord, "read supporting resumption missing"):
                        self.check()

    def test_future_reads_cannot_support_an_earlier_resumed_snapshot(self):
        original = copy.deepcopy(self.record["commands"])
        at = self.record["snapshots"][-1]["at_ms"]
        skew = self.record["window"]["clock_error_ms"]
        for index in (1, 2):
            for began, finished in ((at - 100, at + skew + 1), (at + 100, at + 200)):
                with self.subTest(index=index, began=began, finished=finished):
                    self.record["commands"] = copy.deepcopy(original)
                    self.record["commands"][index].update(start_ms=began, end_ms=finished)
                    with self.assertRaisesRegex(CHECK.InvalidRecord, "read supporting resumption missing"):
                        self.check()

    def test_post_task_read_clock_boundaries_and_later_polling_remain_valid(self):
        original = copy.deepcopy(self.record)
        completed = self.record["backup"]["completed_ms"]
        at = self.record["snapshots"][-1]["at_ms"]
        for skew in (0, 10, 1000):
            with self.subTest(skew=skew):
                self.record = copy.deepcopy(original)
                self.record["window"]["clock_error_ms"] = skew
                # Give the maximum-skew control enough independent margins
                # for baseline, overlap, lock-time snapshots and liveness too.
                if skew == 1000:
                    self.record["backup"].update(lock_start_ms=103000, freeze_ms=103100,
                                                  thaw_request_ms=106500, lock_end_ms=106900)
                    self.record["commands"][0].update(start_ms=101100, end_ms=104100)
                    self.record["snapshots"][0]["at_ms"] = 101500
                    self.record["snapshots"][1]["at_ms"] = 104100
                    self.record["snapshots"][2]["at_ms"] = 105800
                self.record["commands"][1].update(start_ms=completed + skew + 1, end_ms=completed + skew + 2)
                self.record["commands"][2].update(start_ms=at - 10, end_ms=at + skew)
                self.record["commands"].append({"id": "later-poll", "method": "file-read", "start_ms": 115000,
                                                "end_ms": 115100, "result": "ok"})
                self.assertTrue(self.check()["record_checks_passed"])

    def test_cli_rejects_unsupported_resumption_without_native_acceptance_claim(self):
        self.record["commands"] = self.record["commands"][:1]
        path = self.root / "record.json"
        path.write_text(json.dumps(self.record))
        result = subprocess.run([sys.executable, str(Path(__file__).with_name("check.py")),
                                 "--record", str(path), "--expected-source", SOURCE], capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        failure = json.loads(result.stderr)
        self.assertFalse(failure["record_checks_passed"])
        self.assertFalse(failure["native_acceptance_complete"])
        self.assertEqual(failure["failure"], "post-task get-fsinfo read supporting resumption missing")

    def test_wrong_id_timeout_and_incomplete_native_commands_stay_adverse(self):
        for result in ("wrong-command-id", "timeout", "response-incomplete", "permission-denied"):
            with self.subTest(result=result):
                self.record["commands"][0]["result"] = result
                with self.assertRaisesRegex(CHECK.InvalidRecord, "adverse"):
                    self.check()

    def test_lock_dispatch_concurrency_replay_and_absent_overlap_fail(self):
        original = copy.deepcopy(self.record)
        changes = [
            lambda r: r["commands"][0].update(start_ms=104000, end_ms=104500),
            lambda r: r["commands"][0].update(start_ms=103200),
            lambda r: r["commands"][0].update(end_ms=103150),
            lambda r: r["commands"].append({**r["commands"][0], "id": "another"}),
            lambda r: r["commands"].append(copy.deepcopy(r["commands"][0])),
        ]
        for change in changes:
            self.record = copy.deepcopy(original)
            change(self.record)
            with self.subTest(commands=self.record["commands"]):
                with self.assertRaises(CHECK.InvalidRecord):
                    self.check()

    def test_cache_history_cpu_and_resume_controls(self):
        original = copy.deepcopy(self.record)
        changes = [
            lambda r: r["snapshots"][1].update(disk_history_latest_ms=104000),
            lambda r: r["snapshots"][1].update(memory_history_latest_ms=104000),
            lambda r: r["snapshots"][1].update(metadata_sha256="c" * 64),
            lambda r: r["snapshots"][1].update(memory_sha256="c" * 64),
            lambda r: r["snapshots"][1].update(disk_sha256="c" * 64),
            lambda r: r["snapshots"][1].update(cpu_latest_ms=100010),
            lambda r: r["snapshots"][1].update(guest_status="available"),
            lambda r: r["snapshots"][1].update(disk_reason=""),
            lambda r: r["snapshots"][1].update(memory_source="guest-agent-meminfo"),
            lambda r: r["snapshots"][-1].update(guest_identity_sha256="c" * 64),
            lambda r: r["snapshots"][-1].update(disk_history_latest_ms=100020),
            lambda r: r["snapshots"][-1].update(memory_history_latest_ms=100040),
            lambda r: r["snapshots"][-1].update(guest_status="deferred"),
        ]
        for change in changes:
            self.record = copy.deepcopy(original)
            change(self.record)
            with self.subTest(snapshots=self.record["snapshots"]):
                with self.assertRaises(CHECK.InvalidRecord):
                    self.check()

    def test_missing_permissions_identity_trace_and_clock_are_not_acceptance(self):
        original = copy.deepcopy(self.record)
        changes = [
            lambda r: r["identity"].update(pulse_source="c" * 40),
            lambda r: r["identity"].update(haos_image_sha256="missing"),
            lambda r: r.update(evidence_class="source-control"),
            lambda r: r["preflight"].update(pve_version="9.0"),
            lambda r: r["preflight"].update(guest_type="ordinary-qga-subprocess"),
            lambda r: r["preflight"].update(audit=False),
            lambda r: r["preflight"].update(file_read=False),
            lambda r: r["preflight"].update(tls_verified=False),
            lambda r: r["preflight"].update(freeze_enabled=False),
            lambda r: r["preflight"].update(single_pulse_process=False),
            lambda r: r["coverage"].update(dropped_events=1),
            lambda r: r["coverage"].update(start_ms=103500),
            lambda r: r["coverage"].update(end_ms=107000),
            lambda r: r["window"].update(end_ms=800000),
            lambda r: r["window"].update(clock_error_ms=2000),
            lambda r: r["backup"].update(status="failed"),
        ]
        for change in changes:
            self.record = copy.deepcopy(original)
            change(self.record)
            with self.subTest(record=self.record):
                with self.assertRaises(CHECK.InvalidRecord):
                    self.check()

    def test_witness_corruption_replay_missing_liveness_and_early_exit_fail(self):
        original = witness_events()
        changes = [
            lambda events: events[-1].update(result="write-pending"),
            lambda events: events[-1].update(result="interrupted"),
            lambda events: events[-1].update(unix_ms=110000, elapsed_ms=10000),
            lambda events: events[1].update(sha256=HASH),
            lambda events: events[5].update(sequence=1),
            lambda events: events[1].update(run_id="other-run"),
            lambda events: events[1].update(filesystem_id="other-fs"),
            lambda events: events[4].update(unix_ms=104000, elapsed_ms=4000),
            lambda events: events[4].update(elapsed_ms=4000),
            lambda events: events[1].update(kind=[]),
            lambda events: events[1].update(private_value="must not be accepted"),
        ]
        for change in changes:
            events = copy.deepcopy(original)
            change(events)
            self.add_witness(events=events)
            with self.subTest(events=events):
                with self.assertRaises(CHECK.InvalidRecord):
                    self.check()

    def test_missing_symlinked_oversized_truncated_and_changed_witness_fail(self):
        path = self.root / "data.jsonl"
        original = path.read_bytes()
        for raw in (original[:-1], original + b"corrupted\n", b"x" * (CHECK.MAX_BYTES + 1)):
            path.write_bytes(raw)
            self.record["witnesses"][0]["sha256"] = hashlib.sha256(raw).hexdigest()
            with self.assertRaises(CHECK.InvalidRecord):
                self.check()
        self.add_witness()
        self.record["witnesses"][0]["sha256"] = HASH
        with self.assertRaisesRegex(CHECK.InvalidRecord, "mismatch"):
            self.check()
        self.add_witness()
        target = self.root / "retained.jsonl"
        path.rename(target)
        path.symlink_to(target.name)
        with self.assertRaisesRegex(CHECK.InvalidRecord, "regular"):
            self.check()
        path.unlink()
        with self.assertRaisesRegex(CHECK.InvalidRecord, "regular"):
            self.check()

    def test_traversal_duplicate_filesystem_boolean_times_and_extra_fields_fail(self):
        original = copy.deepcopy(self.record)
        changes = [
            lambda r: r["witnesses"][0].update(file="../data.jsonl"),
            lambda r: r["witnesses"][0].update(exit_code=True),
            lambda r: r["witnesses"].append(copy.deepcopy(r["witnesses"][0])),
            lambda r: r["window"].update(start_ms=True),
            lambda r: r["snapshots"][0].update(at_ms=False),
            lambda r: r["commands"][0].update(method=[]),
            lambda r: r.update(private_names="do not accept private fields"),
        ]
        for change in changes:
            self.record = copy.deepcopy(original)
            change(self.record)
            with self.assertRaises(CHECK.InvalidRecord):
                self.check()

    def test_duplicate_json_keys_fail(self):
        with self.assertRaisesRegex(CHECK.InvalidRecord, "duplicate"):
            CHECK.decode(b'{"status":"OK","status":"failed"}')

    def test_cli_returns_sanitised_failures_without_echoing_private_input(self):
        path = self.root / "record.json"
        self.record["snapshots"][0]["at_ms"] = "private-value-must-not-escape"
        path.write_text(json.dumps(self.record))
        result = subprocess.run([sys.executable, str(Path(__file__).with_name("check.py")),
                                 "--record", str(path), "--expected-source", SOURCE], capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertNotIn("private-value-must-not-escape", result.stdout + result.stderr)
        self.assertEqual(json.loads(result.stderr)["failure"], "snapshot time")


if __name__ == "__main__":
    unittest.main()
