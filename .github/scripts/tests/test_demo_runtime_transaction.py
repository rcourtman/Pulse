import hashlib
import importlib.util
import io
import tarfile
import json
import os
from pathlib import Path
import signal
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SCRIPTS = Path(__file__).resolve().parents[1]


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, SCRIPTS / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


engine = load("demo_transaction", "demo-runtime-transaction.py")
dispatcher = load("demo_dispatcher", "dispatch-demo-runtime.py")
native = load("demo_native", "tests/demo_runtime_native.py")


def request(mode="recover"):
    return {"mode": mode, "hostname": "fixture", "local_url": "http://127.0.0.1:7655",
            "public_url": "https://demo.example/api/health", "version": "v6.4.5",
            "control_sha": "a" * 40, "run_id": "123", "run_attempt": "1",
            "binary_sha256": hashlib.sha256(b"changed binary").hexdigest() if mode == "update" else "",
            "version_sha256": hashlib.sha256(b"6.4.6\n").hexdigest() if mode == "update" else "",
            "profile": {**{name: "2" for name in engine.COUNT_KEYS},
                        "seed_duration": "2h", "sample_interval": "5m", "update_interval": "15s"}}


class FixtureHost(engine.Host):
    """Real filesystem estate with a virtual *unchanged* 300-second clock.

    Commands/services/journal/HTTP are adapters, not native or installed proof.
    """
    def __init__(self):
        self.clock = 0
        self.pid = 100
        self.running = True
        self.runtime_version = "6.4.5"
        self.starts = 0
        self.stops = 0
        self.install_count = 0
        self.fail_at = None
        self.failed = False
        self.rollback_failure = False
        self.unhealthy = False
        self.journal_failure = False
        self.ready_until = 0
        self.cancel_at = None
        self.cancel_in_recovery = False
        self.co_host_change = False
        self.co_pid = 200
        self.transaction = None
        self.prior_exit = 0

    def now(self):
        return self.clock

    def sleep(self, seconds):
        self.clock += seconds

    def identity(self, hostname):
        if hostname != "fixture":
            raise engine.Failure("host-identity")

    def prior_closed(self, attempt, status):
        return self.prior_exit == engine.TERMINAL_EXIT[status]

    def state(self):
        state = {"ActiveState": "active", "SubState": "running", "MainPID": str(self.pid), "NRestarts": "0"}
        result = {"pulse": state, "pulse-relay": {**state, "MainPID": "200"}, "caddy": {**state, "MainPID": str(self.co_pid)}}
        if not self.running:
            result["pulse"] = {**state, "ActiveState": "inactive", "SubState": "dead", "MainPID": "0"}
        return result

    def status(self, url):
        if url == engine.RELAY_HEALTH:
            return "200"
        if self.clock < self.ready_until or self.unhealthy:
            return "503"
        if self.failed and self.rollback_failure:
            return "503"
        return "200" if self.running else "000"

    def version(self, url):
        return self.runtime_version

    def cursors(self):
        return {name: str(self.clock) for name in engine.SERVICES}

    def check_journal(self, cursors):
        phase = self.transaction.receipt["status"] if self.transaction else ""
        if self.journal_failure:
            raise engine.Failure("journal-observation")
        if self.co_host_change and self.clock >= 55:
            self.co_pid = 201
        if self.cancel_at is not None and self.clock >= self.cancel_at and not self.failed:
            self.failed = True
            handler = signal.getsignal(signal.SIGTERM)
            handler(signal.SIGTERM, None)
        if self.cancel_in_recovery and phase == "recovering" and self.clock >= 350:
            # Exercise the actual installed handler decision; do not signal the
            # source-proof executor or kill another process to prove it.
            assert signal.getsignal(signal.SIGTERM) == signal.SIG_IGN
            self.cancel_in_recovery = False
        if self.fail_at is not None and self.clock >= self.fail_at and not self.failed:
            self.failed = True
            raise engine.Failure("new-service-crash")

    def stop(self):
        self.stops += 1
        self.running = False

    def start(self):
        self.starts += 1
        self.pid += 1
        self.running = True
        if self.transaction.receipt["status"] == "recovering":
            self.runtime_version = "6.4.5"
            self.unhealthy = False

    def install(self, attempt, req):
        # Keep the clock/services virtual, but use the real narrow file swaps.
        super().install(attempt, req)
        self.install_count += 1
        self.runtime_version = req["version"].lstrip("v")

    def profile(self, req, unhealthy):
        super().profile(req, unhealthy)
        if self.unhealthy:
            self.unhealthy = False


class TransactionTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        root = Path(self.tmp.name)
        self.paths = {name: root / "estate" / name for name in engine.PATHS}
        for name, path in self.paths.items():
            path.parent.mkdir(parents=True, exist_ok=True)
            if name in {"dropins", "data"}:
                path.mkdir()
            else:
                path.write_bytes(b"old " + name.encode())
                path.chmod(0o755 if name == "binary" else 0o644)
        (self.paths["dropins"] / "limits.conf").write_bytes(b"old limits")
        (self.paths["data"] / ".env").write_text("PRIVATE_TOKEN=not-logged\n")
        (self.paths["data"] / "billing.json").write_text('{"capabilities":[],"integrity":"private"}')
        (self.paths["data"] / "persisted-history").write_bytes(b"original history")
        (self.paths["data"] / "alerts").mkdir()
        (self.paths["data"] / "alerts/events.db").write_bytes(b"synthetic opaque persistent file")
        self.root = root / "control"
        self.attempt = self.root / "attempts" / ("a" * 64)
        self.attempt.mkdir(parents=True)
        self.paths["version"].write_text("6.4.5\n")
        (self.attempt / "binary").write_bytes(b"changed binary")
        (self.attempt / "version").write_bytes(b"6.4.6\n")
        # Untouched distribution/service assets are outside the chosen operation.
        self.untouched = {name: root / "unowned" / name for name in
                          ("agent", "scripts", "helper", "auto-update", "timer", "symlink", "marker", "backup")}
        for name, path in self.untouched.items():
            path.parent.mkdir(parents=True, exist_ok=True); path.write_text("original " + name)
        self.untouched_before = engine.estate_hash(self.untouched)
        self.addCleanup(patch.stopall)
        patch.object(engine, "ROOT", self.root).start()
        patch.object(engine, "PATHS", self.paths).start()
        self.before = engine.estate_hash(self.paths)
        self.host = FixtureHost()

    def run_transaction(self, mode="recover", version=None):
        req = request(mode)
        if version:
            req["version"] = version
        tx = engine.Transaction(self.host, req, self.attempt)
        self.host.transaction = tx
        result = tx.run()
        self.assertEqual(tx.receipt, json.loads((self.attempt / "receipt.json").read_text()))
        return result, tx.receipt

    def test_healthy_profile_change_keeps_data_and_watches_complete_window(self):
        code, receipt = self.run_transaction()
        self.assertEqual(code, 0)
        self.assertEqual(receipt["status"], "committed")
        self.assertEqual(receipt["forward"]["elapsed_seconds"], 300)
        self.assertEqual(receipt["forward"]["samples"], 61)
        self.assertEqual(receipt["baseline_services"]["pulse"]["MainPID"], "100")
        self.assertEqual(receipt["forward"]["initial_services"], receipt["forward"]["last_services"])
        self.assertEqual(receipt["forward"]["observed_version"], "6.4.5")
        self.assertEqual(self.host.install_count, 0)
        for name in ("binary", "unit", "dropins"):
            self.assertEqual(engine.estate_hash({name: self.paths[name]}),
                             engine.estate_hash({name: self.attempt / "snapshot" / name}))
        self.assertEqual((self.paths["data"] / "alerts/events.db").read_bytes(), b"synthetic opaque persistent file")
        self.assertTrue((self.attempt / "snapshot/data/persisted-history").exists())
        self.assertNotIn("not-logged", json.dumps(receipt))

    def test_healthy_noop_requires_profile_and_full_watch_without_restart(self):
        self.host.profile(request(), False)
        code, receipt = self.run_transaction()
        self.assertEqual(code, 0)
        self.assertEqual(receipt["status"], "healthy_noop")
        self.assertEqual(receipt["forward"]["elapsed_seconds"], 300)
        self.assertFalse(receipt["mutated"])
        self.assertEqual(self.host.stops, 0)

    def test_noop_cancellation_retains_terminal_failure_without_stopping_service(self):
        self.host.profile(request(), False)
        self.host.cancel_at = 55
        code, receipt = self.run_transaction()
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "refused")
        self.assertEqual(receipt["failure"], "cancelled-forward")
        self.assertEqual(self.host.stops, 0)
        self.assertFalse(receipt["mutated"])

    def test_update_restores_complete_changed_executable_unit_and_data_after_late_failure(self):
        self.host.fail_at = 55
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "rolled_back")
        self.assertEqual(receipt["failure"], "new-service-crash")
        self.assertEqual(receipt["recovery"]["elapsed_seconds"], 300)
        self.assertEqual(engine.estate_hash(self.paths), self.before)
        self.assertEqual(self.host.runtime_version, "6.4.5")

    def test_success_changes_only_runtime_and_profile_leaving_distribution_assets(self):
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual((code, receipt["status"]), (0, "committed"))
        self.assertEqual(self.paths["binary"].read_bytes(), b"changed binary")
        self.assertEqual(self.paths["version"].read_text(), "6.4.6\n")
        for name in ("unit", "dropins"):
            self.assertEqual(engine.estate_hash({name: self.paths[name]}),
                             engine.estate_hash({name: self.attempt / "snapshot" / name}))
        self.assertEqual(engine.estate_hash(self.untouched), self.untouched_before)
        self.assertEqual(receipt["forward"]["elapsed_seconds"], 300)

    def test_late_failure_restores_VERSION_and_runtime_with_untouched_helpers(self):
        self.host.fail_at = 55
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual((code, receipt["status"]), (1, "rolled_back"))
        self.assertEqual(self.paths["version"].read_text(), "6.4.5\n")
        self.assertEqual(engine.estate_hash(self.paths), self.before)
        self.assertEqual(engine.estate_hash(self.untouched), self.untouched_before)
        self.assertEqual(receipt["recovery"]["runtime_sha256"], receipt["runtime_snapshot_sha256"])
        self.assertEqual(receipt["recovery"]["elapsed_seconds"], 300)

    def receipt_fault(self, status, *, persistent=False, after_replace=False):
        """Fault only receipt I/O, leaving the real capture/restore usable."""
        original = engine.atomic_json
        fired = []

        def write(path, value):
            if path == self.attempt / "receipt.json" and (
                    value["status"] == status or (persistent and fired)):
                fired.append(value["status"])
                if after_replace:
                    original(path, value)
                raise OSError("private fixture path must not appear in receipt")
            return original(path, value)

        return patch.object(engine, "atomic_json", side_effect=write), fired

    def assert_observation_failure_restored(self, tx, result, status):
        receipt = tx.receipt
        self.assertEqual((result, receipt["status"], receipt["rollback"]),
                         (2, "observation_failed", "unverified"))
        self.assertTrue(receipt["recovery_required"])
        self.assertIn(status, receipt["observation_failures"])
        self.assertEqual(receipt["recovery"]["elapsed_seconds"], 300)
        self.assertEqual(receipt["recovery"]["samples"], 61)
        self.assertEqual(engine.estate_hash(self.paths), self.before)
        self.assertEqual(engine.estate_hash(self.untouched), self.untouched_before)
        self.assertEqual(self.paths["version"].read_text(), "6.4.5\n")
        self.assertEqual(self.host.runtime_version, "6.4.5")
        self.assertEqual((self.host.stops, self.host.starts),
                         (2, 1 if status == "applying" else 2))
        self.assertTrue(receipt["snapshot_retained"])
        self.assertTrue((self.attempt / "snapshot/data/persisted-history").exists())
        self.assertNotIn("private fixture", json.dumps(receipt))

    def run_fault(self, status, **fault):
        req = request("update"); req["version"] = "v6.4.6"
        tx = engine.Transaction(self.host, req, self.attempt)
        self.host.transaction = tx
        self.host.fail_at = 55 if status in {"recovering", "rolled_back"} else None
        writer, fired = self.receipt_fault(status, **fault)
        with writer:
            result = tx.run()
        self.assertTrue(fired)
        self.assert_observation_failure_restored(tx, result, status)
        return tx

    def test_receipt_failure_after_stop_does_not_abandon_original_service(self):
        tx = self.run_fault("applying")
        self.assertEqual(self.host.install_count, 0)
        self.assertEqual(tx.receipt, json.loads((self.attempt / "receipt.json").read_text()))

    def test_terminal_forward_receipt_failure_restores_changed_runtime(self):
        tx = self.run_fault("committed")
        self.assertEqual(tx.receipt["forward"]["elapsed_seconds"], 300)
        self.assertEqual(self.host.install_count, 1)
        self.assertEqual(tx.receipt, json.loads((self.attempt / "receipt.json").read_text()))

    def test_recovering_receipt_failure_cannot_prevent_restoration(self):
        tx = self.run_fault("recovering")
        self.assertEqual(tx.receipt["failure"], "new-service-crash")
        self.assertEqual(tx.receipt["forward"]["elapsed_seconds"], 50)
        self.assertEqual(tx.receipt, json.loads((self.attempt / "receipt.json").read_text()))

    def test_recovery_terminal_receipt_failure_retains_full_restoration(self):
        tx = self.run_fault("rolled_back")
        self.assertEqual(tx.receipt["observed_outcome"], "rolled_back")
        self.assertEqual(tx.receipt["failure"], "new-service-crash")
        self.assertEqual(tx.receipt, json.loads((self.attempt / "receipt.json").read_text()))

    def test_persistent_receipt_failure_keeps_capture_and_full_cancel_safe_recovery(self):
        self.host.cancel_in_recovery = True
        previous = signal.getsignal(signal.SIGTERM)
        tx = self.run_fault("recovering", persistent=True)
        self.assertEqual(tx.receipt["failure"], "new-service-crash")
        self.assertEqual(tx.receipt["observation_failures"], ["recovering", "observation_failed"])
        retained = json.loads((self.attempt / "receipt.json").read_text())
        self.assertEqual(retained["status"], "applying")
        self.assertNotIn(retained["status"], engine.TERMINAL)
        self.assertFalse(self.host.cancel_in_recovery)
        self.assertEqual(signal.getsignal(signal.SIGTERM), previous)
        # Lost terminal evidence blocks a different attempt without touching it.
        self.attempt = self.root / "attempts" / ("b" * 64); self.attempt.mkdir()
        stops = self.host.stops
        code, receipt = self.run_transaction()
        self.assertEqual((code, receipt["failure"]), (1, "prior-recovery-required"))
        self.assertEqual(self.host.stops, stops)

    def test_visible_forward_receipt_does_not_make_failed_fsync_an_acceptance(self):
        tx = self.run_fault("committed", persistent=True, after_replace=True)
        self.assertEqual(tx.receipt["failure"], "receipt-observation")
        self.assertEqual(json.loads((self.attempt / "receipt.json").read_text())["status"], "observation_failed")

    def test_stale_visible_terminal_cannot_unblock_new_mutation_after_writer_failure(self):
        for status in ("committed", "rolled_back", "refused"):
            with self.subTest(status=status):
                (self.attempt / "receipt.json").write_text(json.dumps({"status": status, "child_result_required": True}))
                original = self.attempt
                self.attempt = self.root / "attempts" / ("b" * 64); self.attempt.mkdir()
                self.host.prior_exit = 2
                code, receipt = self.run_transaction()
                self.assertEqual((code, receipt["failure"]), (1, "prior-recovery-required"))
                self.assertEqual(self.host.stops, 0)
                self.assertEqual(engine.estate_hash(self.paths), self.before)
                (self.attempt / "receipt.json").unlink(); self.attempt.rmdir(); self.attempt = original

    def test_noop_terminal_failure_never_restarts_or_reports_healthy_acceptance(self):
        self.host.profile(request(), False)
        writer, _ = self.receipt_fault("healthy_noop")
        with writer:
            code, receipt = self.run_transaction()
        self.assertEqual((code, receipt["status"]), (2, "observation_failed"))
        self.assertEqual(receipt["failure"], "receipt-observation")
        self.assertEqual(receipt["forward"]["elapsed_seconds"], 300)
        self.assertEqual((self.host.stops, self.host.starts), (0, 0))

    def test_lock_refusal_receipt_error_stays_failed_without_touching_estate(self):
        lock = self.root / "host.lock"
        fd = os.open(lock, os.O_CREAT | os.O_RDWR, 0o600); self.addCleanup(os.close, fd)
        engine.fcntl.flock(fd, engine.fcntl.LOCK_EX | engine.fcntl.LOCK_NB)
        (self.attempt / "request.json").write_text(json.dumps(request()))
        writer, _ = self.receipt_fault("refused", persistent=True)
        with writer, patch.object(engine, "LOCK", lock), patch.object(engine.sys, "argv", ["transaction", str(self.attempt)]):
            self.assertEqual(engine.main(), 2)
        self.assertEqual(self.host.stops, 0)
        self.assertEqual(engine.estate_hash(self.paths), self.before)

    def test_payload_version_mismatch_restores_without_installing(self):
        (self.attempt / "version").write_text("6.4.7\n")
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual((code, receipt["status"]), (1, "rolled_back"))
        self.assertEqual(receipt["failure"], "runtime-payload-identity")
        self.assertEqual(engine.estate_hash(self.paths), self.before)

    def test_failure_at_299_seconds_still_collects_entire_recovery_and_cancellation(self):
        self.host.fail_at = 299
        self.host.cancel_in_recovery = True
        previous = signal.getsignal(signal.SIGTERM)
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["recovery"]["elapsed_seconds"], 300)
        self.assertGreaterEqual(self.host.clock, 600)
        self.assertFalse(self.host.cancel_in_recovery)
        self.assertEqual(signal.getsignal(signal.SIGTERM), previous)
        self.assertEqual(engine.estate_hash(self.paths), self.before)

    def test_forward_cancel_owns_full_recovery(self):
        self.host.cancel_at = 55
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["failure"], "cancelled-forward")
        self.assertEqual(receipt["status"], "rolled_back")
        self.assertEqual(receipt["recovery"]["elapsed_seconds"], 300)

    def test_failed_rollback_retains_estate_and_blocks_next_transaction(self):
        self.host.fail_at = 55
        self.host.rollback_failure = True
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "rollback_failed")
        self.assertTrue(receipt["snapshot_retained"])
        next_attempt = self.root / "attempts" / ("b" * 64)
        next_attempt.mkdir()
        self.attempt = next_attempt
        self.host.rollback_failure = False
        code, receipt = self.run_transaction()
        self.assertEqual(receipt["failure"], "prior-recovery-required")
        self.assertFalse(receipt["mutated"])

    def test_unhealthy_recovery_retains_original_generated_history(self):
        self.host.unhealthy = True
        code, receipt = self.run_transaction()
        self.assertEqual(code, 0)
        self.assertFalse(receipt["healthy_baseline"])
        self.assertFalse((self.paths["data"] / "alerts/events.db").exists())
        self.assertEqual((self.attempt / "snapshot/data/alerts/events.db").read_bytes(), b"synthetic opaque persistent file")
        self.assertEqual(receipt["forward"]["elapsed_seconds"], 300)

    def test_failed_unhealthy_recovery_is_not_a_healthy_rollback(self):
        self.host.unhealthy = True
        self.host.fail_at = 55
        code, receipt = self.run_transaction()
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "recovery_required")
        self.assertEqual(receipt["rollback"], "unavailable-unhealthy-baseline")
        self.assertEqual(engine.estate_hash(self.paths), self.before)
        self.assertFalse(self.host.running)

    def test_update_refuses_unhealthy_baseline_without_stopping_or_pruning(self):
        self.host.unhealthy = True
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["failure"], "update-baseline-unhealthy")
        self.assertEqual(self.host.stops, 0)
        self.assertEqual(engine.estate_hash(self.paths), self.before)

    def test_recovery_refuses_different_healthy_source(self):
        code, receipt = self.run_transaction(version="v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["failure"], "recovery-source-mismatch")
        self.assertEqual(self.host.stops, 0)

    def test_listener_readiness_does_not_spend_the_watch_window(self):
        self.host.ready_until = 30
        code, receipt = self.run_transaction()
        self.assertEqual(code, 0)
        self.assertEqual(receipt["forward"]["elapsed_seconds"], 300)
        self.assertGreaterEqual(self.host.clock, 330)
        self.assertEqual(self.host.starts, 1)

    def test_missing_journal_fails_closed_and_retains_failure(self):
        self.host.journal_failure = True
        code, receipt = self.run_transaction()
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "rollback_failed")
        self.assertEqual(receipt["recovery_failure"], "journal-observation")

    def test_cohost_restart_is_not_silently_accepted_as_recovery(self):
        self.host.co_host_change = True
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "rollback_failed")
        self.assertEqual(receipt["recovery_failure"], "cohost-identity-changed")

    def test_snapshot_headroom_failure_never_deletes_persistent_files(self):
        with patch.object(engine.shutil, "disk_usage", return_value=type("Usage", (), {"free": 0})()):
            code, receipt = self.run_transaction()
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "rolled_back")
        self.assertFalse(receipt["mutated"])
        self.assertEqual(receipt["failure"], "snapshot-headroom")
        self.assertEqual(engine.estate_hash(self.paths), self.before)

    def test_linked_history_cannot_remove_files_outside_estate(self):
        (self.paths["data"] / "alerts/events.db").unlink()
        outside = self.root / "private-file"
        outside.write_text("keep")
        (self.paths["data"] / "alerts/events.db").symlink_to(outside)
        self.host.unhealthy = True
        code, receipt = self.run_transaction()
        self.assertEqual(code, 1)
        self.assertEqual(receipt["failure"], "history-symlink")
        self.assertEqual(outside.read_text(), "keep")

    def test_shared_host_lock_refusal_is_terminal_and_has_no_mutation(self):
        lock = self.root / "host.lock"
        fd = os.open(lock, os.O_CREAT | os.O_RDWR, 0o600)
        self.addCleanup(os.close, fd)
        engine.fcntl.flock(fd, engine.fcntl.LOCK_EX | engine.fcntl.LOCK_NB)
        (self.attempt / "request.json").write_text(json.dumps(request()))
        with patch.object(engine, "LOCK", lock), patch.object(engine.sys, "argv", ["transaction", str(self.attempt)]):
            self.assertEqual(engine.main(), 1)
        receipt = json.loads((self.attempt / "receipt.json").read_text())
        self.assertEqual(receipt["status"], "refused")
        self.assertFalse(receipt["mutated"])
        self.assertEqual(engine.estate_hash(self.paths), self.before)


class InputAndCommandTest(unittest.TestCase):
    def test_prior_terminal_result_rejects_missing_and_failed_writer_evidence(self):
        host = engine.Host(); attempt = Path("/fixed/attempts") / ("a" * 64)
        for status, expected in engine.TERMINAL_EXIT.items():
            body = f"ActiveState={'active' if expected == 0 else 'failed'}\nSubState={'exited' if expected == 0 else 'failed'}\nMainPID=0\nExecMainCode=1\nExecMainStatus={expected}\n"
            with self.subTest(status=status), patch.object(host, "command", return_value=body) as command:
                self.assertTrue(host.prior_closed(attempt, status))
                self.assertEqual(command.call_args.args[0][2], "pulse-demo-" + "a" * 32)
            with patch.object(host, "command", return_value=body.replace(f"ExecMainStatus={expected}", "ExecMainStatus=2")):
                self.assertFalse(host.prior_closed(attempt, status))
        for body in ("", "MainPID=bad", "ActiveState=inactive\nSubState=dead"):
            with patch.object(host, "command", return_value=body):
                self.assertFalse(host.prior_closed(attempt, "committed"))
        with patch.object(host, "command", side_effect=engine.Failure("command-unavailable")):
            self.assertFalse(host.prior_closed(attempt, "committed"))

    def test_cohost_check_uses_the_existing_relay_health_route(self):
        self.assertEqual(engine.RELAY_HEALTH, "https://relay.pulserelay.pro/healthz")

    def test_valid_current_envelope(self):
        engine.validate(request())
        engine.validate(request("update"))

    def test_no_candidate_prerelease_arbitrary_service_url_or_profile(self):
        cases = [("mode", "shell"), ("version", "v6.4.6-rc.1"), ("control_sha", "main"),
                 ("local_url", "http://other-host:7655"), ("public_url", "http://demo.example/api/health"),
                 ("public_url", "https://user:secret@demo.example/api/health"),
                 ("public_url", "https://demo.example/api/health?token=secret"),
                 ("binary_sha256", "arbitrary"), ("version_sha256", "arbitrary")]
        for name, value in cases:
            with self.subTest(name=name, value=value):
                req = request(); req[name] = value
                with self.assertRaises(engine.Failure):
                    engine.validate(req)
        req = request(); req["service"] = "license"
        with self.assertRaises(engine.Failure):
            engine.validate(req)
        req = request(); req["profile"]["nodes"] = "2; restart caddy"
        with self.assertRaises(engine.Failure):
            engine.validate(req)

    def test_http_observation_rejects_redirect_and_transport_failure(self):
        host = engine.Host()
        with patch.object(host, "command", return_value="302") as command:
            self.assertEqual(host.status("https://demo.example/api/health"), "302")
            self.assertNotIn("--location", command.call_args.args[0])
            self.assertIn("--disable", command.call_args.args[0])
            self.assertIn("/dev/null", command.call_args.args[0])
        with patch.object(host, "command", side_effect=engine.Failure("command-failed")):
            self.assertEqual(host.status("https://demo.example/api/health"), "000")

    def test_malformed_or_unavailable_journal_is_not_no_crash(self):
        host = engine.Host()
        for text in ["not-json", '{"MESSAGE":"fine"}', '{"MESSAGE":[],"__CURSOR":"c"}']:
            with self.subTest(text=text), patch.object(host, "command", return_value=text):
                with self.assertRaises(engine.Failure):
                    host.check_journal({"pulse": "c"})
        with patch.object(host, "command", return_value='{"MESSAGE":"panic: fixture","__CURSOR":"d"}'):
            with self.assertRaisesRegex(engine.Failure, "new-service-crash"):
                host.check_journal({"pulse": "c"})

    def test_journal_cursor_advances_without_returning_private_message(self):
        host = engine.Host(); cursors = {"pulse": "c"}
        with patch.object(host, "command", return_value='{"MESSAGE":"private normal message","__CURSOR":"d"}'):
            host.check_journal(cursors)
        self.assertEqual(cursors, {"pulse": "d"})

    def test_available_empty_journal_window_has_no_non_json_header(self):
        host = engine.Host(); cursors = {"pulse": "c"}
        with patch.object(host, "command", return_value="") as command:
            host.check_journal(cursors)
        self.assertIn("--quiet", command.call_args.args[0])
        self.assertEqual(cursors, {"pulse": "c"})

    def test_no_journal_cursor_and_malformed_service_state_fail_closed(self):
        host = engine.Host()
        with patch.object(host, "command", return_value=""):
            with self.assertRaises(engine.Failure):
                host.cursors()
            with self.assertRaises(engine.Failure):
                host.state()


class DispatchLifecycleTest(unittest.TestCase):
    def test_observer_budget_covers_child_and_recovery_and_no_wait_owned_by_ssh(self):
        source = dispatcher.BOOTSTRAP
        self.assertIn("RuntimeMaxSec=45min", source)
        self.assertIn("TimeoutStopSec=20min", source)
        self.assertIn("KillMode=mixed", source)
        self.assertIn("+ 4000", source)
        self.assertGreater(4000, 45 * 60 + 20 * 60)
        self.assertNotIn("'--wait'", source)
        self.assertNotIn("systemctl', 'stop'", source)
        self.assertIn("RemainAfterExit=yes", source)
        self.assertNotIn("'--collect'", source)

    def test_bootstrap_retains_intent_and_repeated_or_uncertain_launch_does_not_replay(self):
        # Execute the real bootstrap with only its filesystem/root/systemd/clock
        # adapters rebound, including a lost submission response. No host tools.
        for uncertain in (False, True):
            with self.subTest(uncertain=uncertain), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "state"
                source = dispatcher.BOOTSTRAP.replace("'/var/lib/pulse-deploy/demo'", repr(str(root)))
                source = source.replace("root.stat().st_uid == 0", "root.stat().st_uid == os.getuid()")
                clock = [0]
                calls = []
                payload = {"request": request(), "source": "c291cmNl", "binary": "", "version_file": ""}

                def launch(argv, **kwargs):
                    if argv[0] == "systemctl":
                        return subprocess.CompletedProcess(argv, 0, b"ActiveState=active\nSubState=exited\nMainPID=0\nExecMainCode=1\nExecMainStatus=0\n")
                    calls.append(argv)
                    attempt = next((root / "attempts").iterdir())
                    if uncertain:
                        raise subprocess.TimeoutExpired("systemd-run", 30)
                    (attempt / "receipt.json").write_text(json.dumps({"status": "committed"}))
                    return subprocess.CompletedProcess(argv, 0)

                class Input:
                    def read(self, _limit):
                        return json.dumps(payload)

                with patch.object(os, "geteuid", return_value=0), patch.object(subprocess, "run", side_effect=launch), \
                     patch.object(dispatcher.sys, "stdin", Input()), patch("time.monotonic", side_effect=lambda: clock[0]), \
                     patch("time.sleep", side_effect=lambda seconds: clock.__setitem__(0, clock[0] + seconds)), patch("builtins.print"):
                    for _ in range(2):
                        with self.assertRaises(SystemExit):
                            exec(compile(source, "fixed-bootstrap", "exec"), {})
                self.assertEqual(len(calls), 1)
                attempt = next((root / "attempts").iterdir())
                self.assertTrue((attempt / "intent.json").exists())
                self.assertEqual((attempt / "request.json").stat().st_mode & 0o777, 0o600)
                self.assertEqual(attempt.stat().st_mode & 0o777, 0o700)
                if uncertain:
                    self.assertEqual(json.loads((attempt / "launch.json").read_text())["state"], "uncertain")

    def test_terminal_observer_requires_writer_closure_without_replay_or_stale_success(self):
        cases = [
            ("committed", "success", 0, "committed"),
            ("committed", "failure", 1, "uncertain"),
            ("healthy_noop", "signal", 1, "uncertain"),
            ("observation_failed", "evidence_failure", 1, "observation_failed"),
            ("rolled_back", "failure", 1, "rolled_back"),
            ("rolled_back", "evidence_failure", 1, "uncertain"),
            ("committed", "malformed", 1, "uncertain"),
            ("committed", "unavailable", 1, "uncertain"),
            ("committed", "missing", 1, "uncertain"),
            ("committed", "timeout", 1, "uncertain"),
            ("committed", "running", 1, "uncertain"),
        ]
        for receipt_status, child_status, expected_code, expected_status in cases:
            with self.subTest(receipt=receipt_status, child=child_status), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "state"
                source = dispatcher.BOOTSTRAP.replace("'/var/lib/pulse-deploy/demo'", repr(str(root)))
                source = source.replace("root.stat().st_uid == 0", "root.stat().st_uid == os.getuid()")
                clock, launches, reads = [0], [], []
                payload = {"request": request(), "source": "c291cmNl", "binary": "", "version_file": ""}

                class Input:
                    def read(self, _limit):
                        return json.dumps(payload)

                def command(argv, **kwargs):
                    if argv[0] == "systemd-run":
                        launches.append(argv)
                        attempt = next((root / "attempts").iterdir())
                        (attempt / "receipt.json").write_text(json.dumps({"status": receipt_status}))
                        return subprocess.CompletedProcess(argv, 0)
                    self.assertEqual(argv[:2], ["systemctl", "show"])
                    reads.append(argv)
                    # Even a visible terminal must wait for its writing child.
                    if len(reads) == 1 or child_status == "running":
                        body = b"ActiveState=active\nSubState=running\nMainPID=123\nExecMainCode=0\nExecMainStatus=0\n"
                    elif child_status == "timeout":
                        raise subprocess.TimeoutExpired("systemctl", 20)
                    else:
                        body = {
                            "success": b"ActiveState=active\nSubState=exited\nMainPID=0\nExecMainCode=1\nExecMainStatus=0\n",
                            "failure": b"ActiveState=failed\nSubState=failed\nMainPID=0\nExecMainCode=1\nExecMainStatus=1\n",
                            "evidence_failure": b"ActiveState=failed\nSubState=failed\nMainPID=0\nExecMainCode=1\nExecMainStatus=2\n",
                            "signal": b"ActiveState=failed\nSubState=failed\nMainPID=0\nExecMainCode=2\nExecMainStatus=9\n",
                            "malformed": b"MainPID=not-a-pid\n",
                            "unavailable": b"",
                            "missing": b"ActiveState=inactive\nSubState=dead\nMainPID=0\nExecMainCode=0\nExecMainStatus=0\n",
                        }[child_status]
                    return subprocess.CompletedProcess(argv, 1 if child_status == "unavailable" and len(reads) > 1 else 0, body)

                with patch.object(os, "geteuid", return_value=0), patch.object(subprocess, "run", side_effect=command), \
                     patch.object(dispatcher.sys, "stdin", Input()), patch("time.monotonic", side_effect=lambda: clock[0]), \
                     patch("time.sleep", side_effect=lambda seconds: clock.__setitem__(0, clock[0] + seconds)), patch("builtins.print") as output:
                    for _ in range(2):
                        reads.clear(); clock[0] = 0
                        with self.assertRaises(SystemExit) as exit_result:
                            exec(compile(source, "fixed-bootstrap", "exec"), {})
                        self.assertEqual(exit_result.exception.code, expected_code)
                        result = json.loads(output.call_args.args[0])
                        self.assertEqual(result["status"], expected_status)
                        self.assertGreaterEqual(len(reads), 2)
                        if expected_status == "uncertain":
                            self.assertTrue(result["recovery_required"])
                self.assertEqual(len(launches), 1)
                # Collection must not edit or fabricate the host receipt.
                attempt = next((root / "attempts").iterdir())
                self.assertEqual(json.loads((attempt / "receipt.json").read_text())["status"], receipt_status)


class NativeAdmissionTest(unittest.TestCase):
    def test_native_driver_refuses_wrong_context_before_any_service_command(self):
        with tempfile.TemporaryDirectory() as directory:
            result_path = Path(directory) / "result.json"
            existing = Path(directory) / "existing-estate"
            existing.mkdir()
            cases = [(1000, "true", "github-hosted", []),
                     (0, "false", "github-hosted", []),
                     (0, "true", "self-hosted", []),
                     (0, "true", "github-hosted", [existing])]
            for uid, actions, environment, estate in cases:
                with self.subTest(uid=uid, actions=actions, environment=environment, occupied=bool(estate)), \
                     patch.object(native, "RESULT", result_path), patch.object(native, "OWNED", estate), \
                     patch.object(os, "geteuid", return_value=uid), \
                     patch.dict(os.environ, {"GITHUB_ACTIONS": actions, "RUNNER_ENVIRONMENT": environment}), \
                     patch.object(native.socket, "gethostname", return_value="disposable-fixture"), \
                     patch.object(native, "command", side_effect=AssertionError("must not execute a service command")) as command:
                    self.assertEqual(native.main(), 1)
                    command.assert_not_called()
                    result = json.loads(result_path.read_text())
                    self.assertFalse(result["passed"])
                    self.assertFalse(result["cleanup_complete"])
                    self.assertFalse(result["signed_published_installer_acceptance"])
                    self.assertEqual(result["failure_code"], "not-an-empty-disposable-public-ci-runner")
            self.assertTrue(existing.is_dir())

    def test_synthetic_changed_executables_have_actual_persistent_effects(self):
        compile(native.binary("1.0.1"), "fixture-runtime", "exec")
        self.assertNotEqual(native.binary("1.0.1"), native.binary("1.0.2", 55))
        self.assertIn(b"persistent-marker", native.binary("1.0.1"))
        self.assertNotIn(b"curl", native.binary("1.0.1"))



class RuntimeSelectionTest(unittest.TestCase):
    def archive(self, directory, entries):
        path = Path(directory) / 'release.tgz'
        with tarfile.open(path, 'w:gz') as stream:
            for name, content, kind in entries:
                member = tarfile.TarInfo(name); member.size = len(content); member.type = kind
                if kind == tarfile.SYMTYPE:
                    member.linkname = '/outside'; member.size = 0
                    stream.addfile(member)
                else:
                    stream.addfile(member, io.BytesIO(content))
        return path

    def elf(self):
        body = bytearray(64); body[:6] = b'\x7fELF\x02\x01'; body[18:20] = b'\x3e\x00'
        return bytes(body)

    def test_selects_only_signed_archive_runtime_without_extracting_paths(self):
        with tempfile.TemporaryDirectory() as directory:
            entries = [('bin/pulse', self.elf(), tarfile.REGTYPE), ('./VERSION', b'6.4.6\n', tarfile.REGTYPE),
                       ('../outside', b'untrusted path', tarfile.REGTYPE),
                       ('scripts/install.sh', b'never executed', tarfile.REGTYPE),
                       ('bin/pulse-agent', b'not installed', tarfile.REGTYPE)]
            files = engine.runtime_members(self.archive(directory, entries), 'v6.4.6')
            self.assertEqual(set(files), {'binary', 'version'})
            self.assertEqual(files['binary'], self.elf())
            self.assertEqual([p.name for p in Path(directory).iterdir()], ['release.tgz'])

    def test_duplicate_link_wrong_version_or_architecture_is_rejected(self):
        valid = [('bin/pulse', self.elf(), tarfile.REGTYPE), ('VERSION', b'6.4.6\n', tarfile.REGTYPE)]
        cases = [valid + [('pulse', self.elf(), tarfile.REGTYPE)],
                 [valid[0], ('VERSION', b'', tarfile.SYMTYPE)],
                 [valid[0], ('VERSION', b'6.4.7\n', tarfile.REGTYPE)],
                 [('bin/pulse', b'#!/bin/sh\n', tarfile.REGTYPE), valid[1]], [valid[0]]]
        for entries in cases:
            with self.subTest(entries=[x[0] for x in entries]), tempfile.TemporaryDirectory() as directory:
                with self.assertRaises(engine.Failure):
                    engine.runtime_members(self.archive(directory, entries), 'v6.4.6')


class NativeGateTest(unittest.TestCase):
    def row(self, run_id=10, **fields):
        return {'id': run_id, 'head_sha': 'a' * 40, 'run_attempt': 1,
                'created_at': '2026-10-03T01:00:00Z', 'updated_at': '2026-10-03T01:10:00Z',
                'path': '.github/workflows/demo-runtime-native.yml', 'event': 'push', 'head_branch': 'main',
                'status': 'completed', 'conclusion': 'success', **fields}

    def check(self, rows, history=None, read_exit=0, same=True):
        calls = []
        def command(argv, **kwargs):
            calls.append(argv)
            if argv[0] == 'gh':
                payload = rows if 'created=' in argv[-1] else (history if history is not None else rows)
                return subprocess.CompletedProcess(argv, read_exit, json.dumps({'workflow_runs': payload}).encode())
            return subprocess.CompletedProcess(argv, 0 if argv[1] == 'merge-base' or same else 1)
        with patch.object(dispatcher.subprocess, 'run', side_effect=command):
            result = dispatcher.require_native({'GITHUB_SHA': 'b' * 40, 'GITHUB_REPOSITORY': 'rcourtman/Pulse'})
        return result, calls

    def test_matching_reviewed_native_source_passes_with_current_first_inventory(self):
        result, calls = self.check([self.row()])
        self.assertEqual(result['run_id'], 10)
        reads = [c for c in calls if c[0] == 'gh']
        self.assertEqual(len(reads), 2)
        self.assertIn('created=', reads[0][-1]); self.assertNotIn('created=', reads[1][-1])
        self.assertEqual(set(calls[-1][6:]), set(dispatcher.NATIVE_PATHS))

    def test_newer_matching_failure_or_pending_run_cannot_use_older_green(self):
        for fields in ({'conclusion': 'failure'}, {'status': 'in_progress', 'conclusion': None}):
            with self.subTest(fields=fields), self.assertRaisesRegex(ValueError, 'not-passed'):
                self.check([self.row(11, **fields), self.row(10)])

    def test_latest_duplicate_attempt_retains_failure(self):
        with self.assertRaisesRegex(ValueError, 'not-passed'):
            self.check([self.row()], [self.row(run_attempt=2, conclusion='failure')])

    def test_missing_changed_or_misidentified_proof_cannot_enable_mutation(self):
        for rows, same in (([], True), ([self.row()], False), ([self.row(event='workflow_dispatch')], True)):
            with self.subTest(rows=rows, same=same), self.assertRaises(ValueError):
                self.check(rows, same=same)

    def test_refused_read_has_no_history_fallback_or_ssh(self):
        calls = []
        def denied(argv, **kwargs):
            calls.append(argv); return subprocess.CompletedProcess(argv, 1, b'')
        with patch.object(dispatcher.subprocess, 'run', side_effect=denied), self.assertRaises(ValueError):
            dispatcher.require_native({'GITHUB_SHA': 'b' * 40, 'GITHUB_REPOSITORY': 'rcourtman/Pulse'})
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0][0:2], ['gh', 'api'])


if __name__ == "__main__":
    unittest.main()
