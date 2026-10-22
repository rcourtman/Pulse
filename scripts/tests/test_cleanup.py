#!/usr/bin/env python3
"""Run the real cleanup and Make targets only in disposable synthetic checkouts.

Native Git supplies the tracked-file decisions and native rm performs deletion.
Closed adapters cover Git inspection/removal failures and forbid sudo. Nothing
executes cleanup in the source checkout, reads real state or contacts a service.
The ordinary script-smoke inventory discovers this suite.
"""

import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = Path(os.environ.get("PULSE_CLEANUP_TEST_SCRIPT", ROOT / "scripts/cleanup.sh"))
MAKEFILE = Path(os.environ.get("PULSE_CLEANUP_TEST_MAKEFILE", ROOT / "Makefile"))
TOOLS = {name: shutil.which(name) for name in ("bash", "dirname", "find", "git", "make", "rm")}

GIT_ADAPTER = r'''import os
from pathlib import Path
import sys

args = sys.argv[1:]
if "--show-toplevel" in args and os.environ.get("FAIL_GIT_ROOT"):
    print(os.environ["FAIL_GIT_ROOT"])
    sys.exit(0)
if "ls-files" in args:
    marker = Path(os.environ["GIT_CALLS"])
    calls = int(marker.read_text()) + 1 if marker.exists() else 1
    marker.write_text(str(calls))
    if calls == int(os.environ.get("FAIL_GIT_QUERY", "0")):
        print("fixture: index inspection unavailable", file=sys.stderr)
        sys.exit(128)
os.execv(os.environ["REAL_GIT"], [os.environ["REAL_GIT"], *args])
'''

RM_ADAPTER = r'''import os
from pathlib import Path
import sys

if Path(sys.argv[-1]).name == "pulse":
    print("fixture: permission denied", file=sys.stderr)
    sys.exit(1)
os.execv(os.environ["REAL_RM"], [os.environ["REAL_RM"], *sys.argv[1:]])
'''

SUDO_ADAPTER = r'''import os
from pathlib import Path
import sys

Path(os.environ["SUDO_CALLED"]).write_text("unexpected privilege escalation")
sys.exit(90)
'''


class CleanupTest(unittest.TestCase):
    def setUp(self):
        for name, path in TOOLS.items():
            self.assertIsNotNone(path, f"native fixture dependency missing: {name}")
        temporary = tempfile.TemporaryDirectory(prefix="pulse-cleanup-")
        self.addCleanup(temporary.cleanup)
        self.fixture = Path(temporary.name)
        self.root = self.fixture / "checkout with spaces"
        self.root.mkdir()
        self.bin = self.fixture / "tools"
        self.bin.mkdir()
        for name, path in TOOLS.items():
            if name != "git":
                (self.bin / name).symlink_to(path)
        self.adapter("git", GIT_ADAPTER)
        self.adapter("sudo", SUDO_ADAPTER)
        self.env = {
            "PATH": str(self.bin), "HOME": str(self.fixture), "LC_ALL": "C",
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
            "REAL_GIT": TOOLS["git"], "REAL_RM": TOOLS["rm"],
            "GIT_CALLS": str(self.fixture / "git-calls"),
            "SUDO_CALLED": str(self.fixture / "sudo-called"),
        }
        self.git("init", "-q")
        self.write("scripts/cleanup.sh", SCRIPT.read_bytes(), 0o755)
        self.write("Makefile", MAKEFILE.read_bytes())
        self.git("add", "--", "scripts/cleanup.sh", "Makefile")

    def adapter(self, name, body):
        path = self.bin / name
        if path.is_symlink():
            path.unlink()
        path.write_text(f"#!{sys.executable}\n" + body)
        path.chmod(0o700)

    def git(self, *args):
        result = subprocess.run([TOOLS["git"], "-C", str(self.root), *args],
                                env=self.env, capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def write(self, name, value=b"synthetic fixture\n", mode=0o600):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(value)
        path.chmod(mode)
        return path

    def snapshot(self, paths):
        return {path: (path.read_bytes(), path.stat().st_mode & 0o777) for path in paths}

    def assert_preserved(self, snapshot):
        for path, expected in snapshot.items():
            with self.subTest(preserved=str(path.relative_to(self.fixture))):
                self.assertTrue(path.is_file(), f"cleanup removed {path.name}")
                self.assertEqual((path.read_bytes(), path.stat().st_mode & 0o777), expected)

    def invoke(self, *args, make=None, **env):
        command = [TOOLS["make"], "-C", str(self.root), make] if make else [
            TOOLS["bash"], str(self.root / "scripts/cleanup.sh"), *args]
        result = subprocess.run(command, cwd=self.fixture,
                                env=dict(self.env, **env), capture_output=True, text=True, timeout=15)
        self.assertFalse((self.fixture / "sudo-called").exists(), result.stdout + result.stderr)
        return result

    def assert_ok(self, result):
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Cleanup complete.", result.stdout)

    def seed_outputs(self):
        names = ("pulse", "frontend-modern/dist/index.html", "internal/api/frontend-modern/dist/index.html",
                 "bin/pulse-agent", "build/generated", "dist/archive", "frontend-modern/node_modules/package/index.js",
                 "frontend-modern/.vite/cache", "node_modules/package/index.js")
        return [self.write(name) for name in names]

    def test_distclean_preserves_local_state_settings_and_recovery_evidence(self):
        outputs = self.seed_outputs()
        names = ("data/settings.json", "data/.encryption.key", "data/nested/synthetic-state",
                 ".env", ".env.local", ".env.dev", ".encryption.key", ".codex/config.toml",
                 ".claude/settings.json", ".claudecode-settings.json", ".claudecode-hooks/hook",
                 ".mcp-servers/config.json", "pulse.log", "debug.log", "frontend-modern/debug.log",
                 "notes/operator.bak", "notes/operator.backup", "notes/config.orig", "notes/config~",
                 "notes/request.tmp", "notes/__pycache__/unknown.pyc", "notes/coverage/preserved",
                 "tmp/recovery/capture", ".tmp/readback", ".cache/receipt", "downloads/published-artifact",
                 "backups/archive.tar.gz", "backups/key.backup")
        snapshot = self.snapshot([self.write(name) for name in names])
        empty = self.root / "notes/empty-recovery-directory"
        empty.mkdir()
        self.assert_ok(self.invoke(make="distclean"))
        self.assert_preserved(snapshot)
        self.assertTrue(empty.is_dir())
        self.assertTrue(all(not path.exists() for path in outputs))

    def test_make_clean_removes_only_build_targets(self):
        outputs = self.seed_outputs()
        snapshot = self.snapshot(outputs[3:])
        self.assert_ok(self.invoke(make="clean"))
        self.assertTrue(all(not path.exists() for path in outputs[:3]))
        self.assert_preserved(snapshot)

    def test_tracked_file_and_directory_are_preserved_even_with_untracked_children(self):
        pulse = self.write("pulse")
        source = self.write("build/checked-in-source.c")
        sibling = self.write("build/untracked-note.bak")
        generated = self.write("dist/generated")
        self.git("add", "--", "pulse", "build/checked-in-source.c")
        snapshot = self.snapshot([pulse, source, sibling])
        self.assert_ok(self.invoke())
        self.assert_preserved(snapshot)
        self.assertFalse(generated.exists())

    def test_make_clean_also_preserves_tracked_output(self):
        pulse = self.write("pulse")
        self.git("add", "--", "pulse")
        snapshot = self.snapshot([pulse])
        self.assert_ok(self.invoke(make="clean"))
        self.assert_preserved(snapshot)

    def test_linked_ancestors_cannot_remove_another_checkouts_builds(self):
        external = self.fixture / "other-checkout"
        external.mkdir()
        payload = external / "dist/important"
        payload.parent.mkdir()
        payload.write_bytes(b"other checkout fixture\n")
        (self.root / "frontend-modern").symlink_to(external, target_is_directory=True)
        snapshot = self.snapshot([payload])
        self.assert_ok(self.invoke(make="distclean"))
        self.assert_preserved(snapshot)
        self.assertTrue((self.root / "frontend-modern").is_symlink())

    def test_linked_internal_parent_cannot_remove_an_external_embed(self):
        external = self.fixture / "other-internal"
        payload = external / "api/frontend-modern/dist/index.html"
        payload.parent.mkdir(parents=True)
        payload.write_bytes(b"external embed fixture\n")
        (self.root / "internal").symlink_to(external, target_is_directory=True)
        snapshot = self.snapshot([payload])
        self.assert_ok(self.invoke(make="clean"))
        self.assert_preserved(snapshot)

    def test_final_and_dangling_links_are_preserved(self):
        external = self.fixture / "external"
        external.write_bytes(b"external fixture\n")
        (self.root / "pulse").symlink_to(external)
        (self.root / "build").symlink_to(self.fixture / "missing", target_is_directory=True)
        self.assert_ok(self.invoke())
        self.assertTrue((self.root / "pulse").is_symlink())
        self.assertTrue((self.root / "build").is_symlink())
        self.assertEqual(external.read_bytes(), b"external fixture\n")

    def test_missing_git_refuses_before_removing_builds(self):
        snapshot = self.snapshot(self.seed_outputs())
        (self.bin / "git").unlink()
        result = self.invoke(make="distclean")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("no files removed", result.stderr)
        self.assert_preserved(snapshot)

    def test_wrong_git_root_refuses_before_removing_builds(self):
        snapshot = self.snapshot(self.seed_outputs())
        result = self.invoke(FAIL_GIT_ROOT=str(self.fixture))
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("no files removed", result.stderr)
        self.assert_preserved(snapshot)

    def test_late_index_failure_refuses_before_any_removal(self):
        snapshot = self.snapshot(self.seed_outputs())
        result = self.invoke(FAIL_GIT_QUERY="2")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("no files removed", result.stderr)
        self.assert_preserved(snapshot)

    def test_missing_repository_refuses_before_removing_builds(self):
        snapshot = self.snapshot(self.seed_outputs())
        # Hide only this synthetic Git directory; no source metadata is changed.
        (self.root / ".git").rename(self.fixture / "saved-git")
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("no files removed", result.stderr)
        self.assert_preserved(snapshot)

    def test_removal_error_is_reported_without_sudo_or_false_completion(self):
        pulse = self.write("pulse")
        snapshot = self.snapshot([pulse])
        self.adapter("rm", RM_ADAPTER)
        result = self.invoke()
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn("Cleanup complete.", result.stdout)
        self.assertIn("no privilege escalation attempted", result.stderr)
        self.assert_preserved(snapshot)

    def test_empty_checkout_is_idempotent(self):
        self.assert_ok(self.invoke())
        self.assert_ok(self.invoke())

    def test_arguments_fail_closed(self):
        snapshot = self.snapshot(self.seed_outputs())
        for args in (("--unknown",), ("--build-only", "extra"), ("", "extra")):
            with self.subTest(args=args):
                result = self.invoke(*args)
                self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
                self.assertIn("Usage:", result.stderr)
                self.assert_preserved(snapshot)


if __name__ == "__main__":
    unittest.main()
