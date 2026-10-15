#!/usr/bin/env python3
"""The unattended updater must finish without a retired HTTP notification route.

Prerequisite/update callbacks are confined doubles, not installed recovery proof.
The success control leaves a real loopback listener unresponsive and uses the
actual shell main; no host manager, remote service or notification destination
is contacted. Only each fixture's own process group is stopped on failure.
"""
from __future__ import annotations

import os
from pathlib import Path
import shlex
import shutil
import signal
import socket
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
UPDATER = Path(os.environ.get("PULSE_AUTO_UPDATER_UNDER_TEST", ROOT / "scripts/pulse-auto-update.sh")).resolve()

HARNESS = r'''
source "$UPDATER"
log() { printf '[%s] %s\n' "$1" "${*:2}"; }
check_auto_updates_enabled() { :; }
installed_binary_is_pulse_pro() { return 1; }
get_current_version() { printf '%s\n' v6.5.0; }
get_latest_stable_version() { printf '%s\n' "$LATEST_VERSION"; }
perform_update() {
    printf '%s\n' "$1" > "$FIXTURE/update-result"
    return "$UPDATE_EXIT"
}
main
'''


class UpdateCompletion(unittest.TestCase):
    def run_main(self, *, webhook="webhooks.json", update_exit=0, latest="v6.6.0", listener=False):
        with tempfile.TemporaryDirectory(prefix="pulse-update-completion-") as temporary:
            root = Path(temporary)
            if webhook:
                (root / webhook).write_text("synthetic presence only\n")
            before = {path.name: path.read_bytes() for path in root.iterdir()}
            tools = root / "tools"
            tools.mkdir()
            # If the success path tries curl, run the real client against the
            # private stalled listener. In the remaining cases, record/refuse
            # every unexpected call without any network access.
            trace = root / "curl-calls"
            curl = tools / "curl"
            real_curl = shutil.which("curl")
            self.assertIsNotNone(real_curl)
            curl.write_text('#!/bin/sh\nprintf "called\\n" >> "$FIXTURE/curl-calls"\n'
                            + ('exec ' + shlex.quote(real_curl) + ' "$@"\n' if listener else 'exit 97\n'))
            curl.chmod(0o700)
            env = dict(os.environ, UPDATER=str(UPDATER), FIXTURE=str(root),
                       PULSE_CONFIG_DIR=str(root), LATEST_VERSION=latest,
                       UPDATE_EXIT=str(update_exit),
                       PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       # Keep the retired endpoint's test strictly loopback,
                       # independent of ambient proxy/client configuration.
                       HOME=str(root), CURL_HOME=str(root),
                       http_proxy="", https_proxy="", ALL_PROXY="", all_proxy="",
                       HTTP_PROXY="", HTTPS_PROXY="", NO_PROXY="*", no_proxy="*")
            connected = threading.Event()
            finished = threading.Event()
            server = None
            thread = None
            if listener:
                server = socket.socket()
                self.addCleanup(server.close)
                server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                server.bind(("127.0.0.1", 7655))
                server.listen()
                server.settimeout(0.1)

                def stall():
                    while not finished.is_set():
                        try:
                            connection, _ = server.accept()
                        except socket.timeout:
                            continue
                        with connection:
                            connected.set()
                            finished.wait(5)
                        return

                thread = threading.Thread(target=stall, daemon=True)
                thread.start()
            process = subprocess.Popen(["bash", "-c", HARNESS], cwd=root,
                                       env=env, text=True, stdout=subprocess.PIPE,
                                       stderr=subprocess.PIPE, start_new_session=True)
            try:
                try:
                    stdout, stderr = process.communicate(timeout=3)
                except subprocess.TimeoutExpired:
                    self.fail("verified update did not finish; retired HTTP request connected="
                              + str(connected.is_set()))
                result = subprocess.CompletedProcess(process.args, process.returncode, stdout, stderr)
                calls = trace.read_text() if trace.exists() else ""
                update = (root / "update-result").read_text() if (root / "update-result").exists() else None
                after = {name: (root / name).read_bytes() for name in before}
                self.assertEqual(before, after, "notification configuration was changed")
                self.assertEqual(calls, "", "retired notification endpoint was still contacted")
                self.assertFalse(connected.is_set(), "retired HTTP request reached another local service")
                return result, update
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGTERM)
                process.communicate(timeout=3)
                finished.set()
                if thread:
                    thread.join(timeout=2)
                    self.assertFalse(thread.is_alive(), "stalled listener was not stopped")

    def test_verified_update_finishes_with_a_stalled_local_listener(self):
        result, update = self.run_main(listener=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(update, "v6.6.0\n")
        self.assertIn("Update completed successfully to v6.6.0", result.stdout)
        self.assertIn("Auto-update check completed", result.stdout)

    def test_each_legacy_webhook_file_and_absence_leave_completion_local(self):
        for webhook in ("webhooks.json", "webhooks.enc", ""):
            with self.subTest(webhook=webhook):
                result, update = self.run_main(webhook=webhook)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(update, "v6.6.0\n")
                self.assertIn("Auto-update check completed", result.stdout)

    def test_failed_update_is_not_relabelled_as_success(self):
        result, update = self.run_main(update_exit=23)
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(update, "v6.6.0\n")
        self.assertIn("Update failed", result.stdout)
        self.assertNotIn("Update completed successfully", result.stdout)
        self.assertNotIn("Auto-update check completed", result.stdout)

    def test_no_update_keeps_the_existing_no_work_completion(self):
        result, update = self.run_main(latest="v6.5.0")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIsNone(update)
        self.assertIn("Already running latest version", result.stdout)
        self.assertIn("Auto-update check completed", result.stdout)


if __name__ == "__main__":
    unittest.main()
