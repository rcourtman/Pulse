#!/usr/bin/env python3
"""Prove the retired cleaner cannot delete incidents or interrupt monitoring.

All paths and command traps are synthetic. No installed history, real service,
runtime or guest is read or changed.
"""

from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts/clean-mock-alerts.sh"
BASH = shutil.which("bash")
PRIVATE = "private-fixture-marker"


class RetiredMockCleanupTest(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.calls = self.root / "calls"
        self.history = self.root / f"{PRIVATE}-history.json"
        # Both interior and prefix matches can be genuine names. None has
        # authenticated mock provenance; even synthetic-looking rows survive.
        self.original = json.dumps([
            {"alert": {"resourceId": name, "message": PRIVATE}}
            for name in ("hammock-storage", "mock-production-db", "mock-resource-1", "real-node")
        ]).encode()
        self.history.write_bytes(self.original)
        self.history.chmod(0o640)
        os.utime(self.history, ns=(1_000_000_000, 2_000_000_000))
        trap = f'#!{BASH}\nprintf "%s\\n" "$0 $*" >> "$CALL_LOG"\nexit 91\n'
        for name in ("jq", "id", "dirname", "date", "cp", "mktemp", "chown",
                     "pkill", "kill", "systemctl", "sudo", "sleep", "rm", "hot-dev-bg"):
            tool = self.bin / name
            tool.write_text(trap)
            tool.chmod(0o700)
        self.env = {
            "PATH": str(self.bin), "HOME": str(self.root), "LC_ALL": "C",
            "CALL_LOG": str(self.calls), "ALERT_HISTORY": str(self.history),
            "HOT_DEV_BG_PATH": str(self.bin / "hot-dev-bg"), "TMPDIR": str(self.root),
        }

    def invoke(self, *args, env=None):
        return subprocess.run([BASH, str(SCRIPT), *args], cwd=self.root,
                              env=self.env if env is None else env,
                              capture_output=True, timeout=3)

    def assert_refusal(self, result):
        self.assertEqual(result.returncode, 2, result)
        self.assertEqual(result.stdout, b"")
        self.assertIn(b"cleanup has been retired", result.stderr)
        self.assertIn(b"No cleanup was performed", result.stderr)
        self.assertIn(b"Keep existing history intact", result.stderr)
        self.assertIn(b"separate disposable data directory", result.stderr)
        self.assertNotIn(PRIVATE.encode(), result.stdout + result.stderr)
        self.assertFalse(self.calls.exists(), "helper invoked an external command")

    def test_default_invocation_refuses_without_runtime_commands(self):
        self.assert_refusal(self.invoke())

    def test_real_and_synthetic_looking_incidents_are_untouched(self):
        before = self.history.stat()
        self.assert_refusal(self.invoke())
        after = self.history.stat()
        self.assertEqual((after.st_ino, after.st_mode, after.st_uid, after.st_gid,
                          after.st_size, after.st_mtime_ns, after.st_atime_ns),
                         (before.st_ino, before.st_mode, before.st_uid, before.st_gid,
                          before.st_size, before.st_mtime_ns, before.st_atime_ns))
        self.assertEqual(self.history.read_bytes(), self.original)

    def test_no_backup_temporary_or_other_file_is_created(self):
        before = set(self.root.rglob("*"))
        self.assert_refusal(self.invoke())
        self.assertEqual(set(self.root.rglob("*")), before)

    def test_missing_history_is_not_inspected_or_created(self):
        self.history.unlink()
        self.assert_refusal(self.invoke())
        self.assertFalse(self.history.exists())

    def test_malformed_history_is_not_parsed_or_repaired(self):
        corrupt = b"not JSON\x00\xff" + PRIVATE.encode()
        self.history.write_bytes(corrupt)
        self.assert_refusal(self.invoke())
        self.assertEqual(self.history.read_bytes(), corrupt)

    def test_empty_and_unreadable_history_are_not_needed(self):
        self.history.write_bytes(b"")
        self.history.chmod(0)
        before = self.history.stat()
        self.assert_refusal(self.invoke())
        self.assertEqual(self.history.stat(), before)

    def test_symlink_target_and_link_are_preserved(self):
        link = self.root / "history-link"
        link.symlink_to(self.history)
        self.assert_refusal(self.invoke(env=dict(self.env, ALERT_HISTORY=str(link))))
        self.assertEqual(link.readlink(), self.history)
        self.assertEqual(self.history.read_bytes(), self.original)

    def test_fifo_is_never_opened(self):
        fifo = self.root / "history-fifo"
        os.mkfifo(fifo)
        self.assert_refusal(self.invoke(env=dict(self.env, ALERT_HISTORY=str(fifo))))

    def test_history_directory_is_untouched(self):
        self.assert_refusal(self.invoke(env=dict(self.env, ALERT_HISTORY=str(self.root))))
        self.assertEqual(self.history.read_bytes(), self.original)

    def test_no_dependency_or_privilege_tool_is_required(self):
        self.assert_refusal(self.invoke(env={"PATH": "", "LC_ALL": "C"}))

    def test_options_cannot_reenable_cleanup_or_leak_arguments(self):
        for args in (("--force",), ("--apply",), ("--dry-run",),
                     (str(self.history),), ("--help", "--force"),
                     ("--force", PRIVATE), ("--",), ("--help\n" + PRIVATE,)):
            with self.subTest(args=args):
                self.assert_refusal(self.invoke(*args))

    def test_help_is_successful_information_not_cleanup(self):
        for option in ("--help", "-h"):
            with self.subTest(option=option):
                result = self.invoke(option)
                self.assertEqual(result.returncode, 0, result)
                self.assertEqual(result.stderr, b"")
                self.assertIn(b"cleanup has been retired", result.stdout)
                self.assertIn(b"No cleanup was performed", result.stdout)
                self.assertIn(b"no force or apply option", result.stdout)
                self.assertFalse(self.calls.exists())
                self.assertEqual(self.history.read_bytes(), self.original)

    def test_concurrent_callers_do_not_need_locks_or_mutate_history(self):
        with ThreadPoolExecutor(max_workers=8) as pool:
            results = list(pool.map(lambda _: self.invoke(), range(8)))
        for result in results:
            self.assert_refusal(result)
        self.assertEqual(self.history.read_bytes(), self.original)


if __name__ == "__main__":
    unittest.main(verbosity=2)
