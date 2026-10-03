import copy
import hashlib
import importlib.util
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


def request(mode="recover"):
    return {"mode": mode, "hostname": "fixture", "local_url": "http://127.0.0.1:7655",
            "public_url": "https://demo.example/api/health", "version": "v6.4.5",
            "control_sha": "a" * 40, "run_id": "123", "run_attempt": "1",
            "installer_sha256": hashlib.sha256(b"signed-installer-fixture").hexdigest() if mode == "update" else "",
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

    def now(self):
        return self.clock

    def sleep(self, seconds):
        self.clock += seconds

    def identity(self, hostname):
        if hostname != "fixture":
            raise engine.Failure("host-identity")

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
        if hashlib.sha256((attempt / "installer.sh").read_bytes()).hexdigest() != req["installer_sha256"]:
            raise engine.Failure("installer-identity")
        self.install_count += 1
        self.runtime_version = req["version"].lstrip("v")
        engine.PATHS["binary"].write_bytes(b"new executable")
        engine.PATHS["unit"].write_bytes(b"new unit")
        (engine.PATHS["data"] / "persisted-history").write_bytes(b"forward history")

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
        (self.attempt / "installer.sh").write_bytes(b"signed-installer-fixture")
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

    def test_update_restores_complete_changed_executable_unit_and_data_after_late_failure(self):
        self.host.fail_at = 55
        code, receipt = self.run_transaction("update", "v6.4.6")
        self.assertEqual(code, 1)
        self.assertEqual(receipt["status"], "rolled_back")
        self.assertEqual(receipt["failure"], "new-service-crash")
        self.assertEqual(receipt["recovery"]["elapsed_seconds"], 300)
        self.assertEqual(engine.estate_hash(self.paths), self.before)
        self.assertEqual(self.host.runtime_version, "6.4.5")

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
    def test_valid_current_envelope(self):
        engine.validate(request())
        engine.validate(request("update"))

    def test_no_candidate_prerelease_arbitrary_service_url_or_profile(self):
        cases = [("mode", "shell"), ("version", "v6.4.6-rc.1"), ("control_sha", "main"),
                 ("local_url", "http://other-host:7655"), ("public_url", "http://demo.example/api/health"),
                 ("public_url", "https://user:secret@demo.example/api/health"),
                 ("public_url", "https://demo.example/api/health?token=secret"),
                 ("installer_sha256", "arbitrary")]
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

    def test_bootstrap_retains_intent_and_repeated_or_uncertain_launch_does_not_replay(self):
        # Execute the real bootstrap with only its filesystem/root/systemd/clock
        # adapters rebound, including a lost submission response. No host tools.
        for uncertain in (False, True):
            with self.subTest(uncertain=uncertain), tempfile.TemporaryDirectory() as directory:
                root = Path(directory) / "state"
                source = dispatcher.BOOTSTRAP.replace("pathlib.Path('/var/lib/pulse-deploy/demo')", repr(root))
                # repr(Path) is not a Python literal: bind just the fixed path.
                source = dispatcher.BOOTSTRAP.replace("'/var/lib/pulse-deploy/demo'", repr(str(root)))
                source = source.replace("root.stat().st_uid == 0", "root.stat().st_uid == os.getuid()")
                clock = [0]
                calls = []
                payload = {"request": request(), "source": "c291cmNl", "installer": ""}

                def launch(argv, **kwargs):
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


if __name__ == "__main__":
    unittest.main()
