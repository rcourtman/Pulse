#!/usr/bin/env python3
"""Run the hosted tarball helper against disposable, non-database fixtures.

Real tar/gzip and local retention operate only on those fixtures. age, rclone,
Docker and host identity are closed fakes for orchestration controls. Optional
real age and local-backend rclone cases run when supplied or installed; no test
establishes application consistency, cloud delivery or native restore readiness.
"""

import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = Path(os.environ.get("PULSE_CLOUD_BACKUP_TEST_SCRIPT", ROOT / "scripts/cloud-backup.sh"))
BASH = shutil.which("bash")

REAL_AGE = os.environ.get("PULSE_CLOUD_BACKUP_TEST_REAL_AGE") or shutil.which("age")
REAL_AGE_KEYGEN = os.environ.get("PULSE_CLOUD_BACKUP_TEST_REAL_AGE_KEYGEN") or shutil.which("age-keygen")
REAL_RCLONE = os.environ.get("PULSE_CLOUD_BACKUP_TEST_REAL_RCLONE") or shutil.which("rclone")
METADATA_MARKER = b"fixture-container"
STAMP_PATTERN = "[0-9]" * 8 + "-" + "[0-9]" * 6
OWNED_TYPES = (("tenants", ".tar.gz"), ("cp", ".tar.gz"), ("meta", ".txt"))
OWNED_PATTERNS = {f"pulse-backup-{label}-{STAMP_PATTERN}{suffix}{encrypted}"
                  for label, suffix in OWNED_TYPES for encrypted in ("", ".age")}

FAKE = r'''import json
import os
from pathlib import Path
import subprocess
import sys

root = Path(os.environ["FIXTURE"])
tool, args = Path(sys.argv[0]).name, sys.argv[1:]
scenario = json.loads((root / "scenario.json").read_text())
calls_file = root / "calls.jsonl"
previous = [json.loads(line) for line in calls_file.read_text().splitlines()] if calls_file.exists() else []
backups = Path(os.environ["PULSE_BACKUP_DIR"]).resolve()
assert backups.is_relative_to(root), backups
if tool != "date":
    with calls_file.open("a") as output:
        output.write(json.dumps([tool, *args]) + "\n")

if tool in ("tar", "age"):
    completed = sorted(p.name for p in backups.iterdir()
                       if os.environ["FIXTURE_TIMESTAMP"] in p.name
                       and p.name.endswith((".tar.gz", ".age", ".txt")))
    with (root / "producer-observations.jsonl").open("a") as observations:
        observations.write(json.dumps({"tool": tool, "completed": completed}) + "\n")

if tool == "mv":
    assert len(args) == 5 and args[:3] == ["-T", "-n", "--"], args
    source, destination = map(Path, args[3:])
    assert source == Path(str(destination) + ".partial"), args
    assert source.resolve().is_relative_to(backups), args
    assert destination.parent.resolve() == backups, args
    assert source.is_file(), args
    number = 1 + sum(call[0] == tool for call in previous)
    if number == scenario.get("publish_fail"):
        sys.exit(13)
    if number == scenario.get("publish_collision"):
        kind = scenario.get("collision_kind", "file")
        if kind == "file":
            destination.write_bytes(b"late unrelated fixture")
        elif kind == "directory":
            destination.mkdir()
            (destination / "preserved.txt").write_bytes(b"late unrelated fixture")
        else:
            target = root / ("late-link-target-" + os.environ["FIXTURE_TIMESTAMP"])
            if kind == "symlink":
                target.write_bytes(b"late unrelated fixture")
            destination.symlink_to(target)
    sys.exit(subprocess.run([os.environ["REAL_MV"], *args], check=False).returncode)

if tool == "tar":
    # Admit both literal-member argv and the exact base for regression controls.
    assert len(args) in (5, 6) and args[0] == "-czf" and args[2] == "-C", args
    assert len(args) == 5 or args[4] == "--", args
    output = Path(args[1]).resolve() if args[1] != "-" else None
    source = Path(args[3]) / args[-1]
    assert output is None or output.is_relative_to(backups), output
    assert source.resolve().is_relative_to(root), source
    number = 1 + sum(call[0] == tool for call in previous)
    if number == scenario.get("tar_fail"):
        if output is None:
            sys.stdout.buffer.write(b"incomplete fixture archive")
        else:
            output.write_bytes(b"incomplete fixture archive")
        sys.exit(7)
    if number == 1 and scenario.get("block_first_tar"):
        import time
        (root / "tar-entered").touch()
        deadline = time.monotonic() + 10
        while not (root / "release-tar").exists():
            if time.monotonic() >= deadline:
                sys.exit(91)
            time.sleep(0.02)
    if number == scenario.get("native_tar_missing"):
        source.rename(source.with_name(source.name + "-moved-before-tar"))
    result = subprocess.run([os.environ["REAL_TAR"], *args], check=False)
    if number == 1 and scenario.get("source_disappears"):
        (root / "control-plane").rename(root / "moved-control-plane")
    if number == 1 and scenario.get("source_becomes_symlink"):
        (root / "control-plane").rename(root / "moved-control-plane")
        (root / "control-plane").symlink_to(root / "moved-control-plane", target_is_directory=True)
    sys.exit(result.returncode)
if tool == "find":
    assert Path(args[0]).resolve() == backups, args
    # Admit the unchanged parent as well for explicit regression controls.
    if args[1] == "-name":
        assert args[1:4] == ["-name", "pulse-backup-*.tar.gz*", "-mtime"], args
        assert len(args) == 6 and args[-1] == "-delete", args
    else:
        assert args[1:6] == ["-maxdepth", "1", "-type", "f", "("], args
        assert args[-4] == ")" and args[-3] == "-mtime" and args[-1] == "-delete", args
    if scenario.get("local_retention_fail"):
        sys.exit(9)
    sys.exit(subprocess.run([os.environ["REAL_FIND"], *args], check=False).returncode)
if tool == "age":
    # Admit the old file-input path for explicit exact-parent controls too.
    assert len(args) in (4, 5) and args[:3] == ["-r", "fixture-recipient", "-o"], args
    output = Path(args[3])
    assert output.resolve().is_relative_to(backups), output
    number = 1 + sum(call[0] == tool for call in previous)
    def observe_plaintext(phase):
        names = sorted(p.name for p in backups.glob("*" + os.environ["FIXTURE_TIMESTAMP"] + "*")
                       if p.name.endswith((".txt", ".tar.gz", ".txt.partial", ".tar.gz.partial")))
        with (root / "plaintext-observations.jsonl").open("a") as observations:
            observations.write(json.dumps({"number": number, "phase": phase, "names": names}) + "\n")
    observe_plaintext("before-input")
    if number == scenario.get("age_reject"):
        sys.exit(8)
    if len(args) == 5:
        source = Path(args[4])
        assert source.resolve().is_relative_to(backups), source
        content = source.read_bytes()
    else:
        content = sys.stdin.buffer.read()
    observe_plaintext("after-input")
    output.write_bytes(b"FAKE-AGE\n" + content)
    sys.exit(8 if number == scenario.get("age_fail") else 0)
if tool == "rclone":
    remote = root / "remote"
    if scenario.get("private_client_output"):
        # Clients can repeat connection-string credentials in either stream,
        # on success as well as failure. Never contact a real remote here.
        print("SYNTHETIC-CLIENT-STDOUT " + os.environ["PULSE_RCLONE_REMOTE"])
        print("SYNTHETIC-CLIENT-STDERR " + os.environ["PULSE_RCLONE_REMOTE"], file=sys.stderr)
    if args[0] == "copyto":
        # Accept the unchanged helper for explicit exact-parent controls too.
        assert len(args) in (3, 4) and args[3:] in ([], ["--error-on-no-transfer"]), args
        assert Path(args[1]).resolve().is_relative_to(backups), args
        assert Path(args[1]).is_file(), args
        assert args[2] == os.environ["PULSE_RCLONE_REMOTE"] + "/pulse-cloud-tarballs/" + Path(args[1]).name, args
        number = 1 + sum(call[:2] == [tool, "copyto"] for call in previous)
        if number == scenario.get("upload_fail"):
            sys.exit(10)
        if scenario.get("unsupported_transfer_flag") and args[3:]:
            sys.exit(2)
        if number == scenario.get("upload_skip"):
            # Closed no-transfer model, not a native rclone behaviour claim.
            sys.exit(9 if args[3:] else 0)
        if os.environ.get("REAL_RCLONE"):
            sys.exit(subprocess.run([os.environ["REAL_RCLONE"], "--config", os.devnull, "copyto",
                                    args[1], str(remote / Path(args[1]).name), *args[3:]],
                                   check=False).returncode)
        (remote / Path(args[1]).name).write_bytes(Path(args[1]).read_bytes())
        sys.exit(0)
    if args[0] == "delete":
        assert args[:3] == ["delete", os.environ["PULSE_RCLONE_REMOTE"] + "/pulse-cloud-tarballs", "--min-age"], args
        assert args[3].endswith("d") and args[3][:-1].isdigit(), args
        if scenario.get("remote_retention_fail"):
            sys.exit(11)
        if os.environ.get("REAL_RCLONE"):
            sys.exit(subprocess.run([os.environ["REAL_RCLONE"], "--config", os.devnull, "delete",
                                    str(remote), *args[2:]], check=False).returncode)
        # Closed local model of rclone's include/exclude/age contract, not proof
        # of the real client's filter engine or any provider's behaviour.
        import fnmatch
        import time
        includes, excludes, max_depth = [], [], None
        for option, value in zip(args[4::2], args[5::2], strict=True):
            if option == "--include":
                includes.append(value)
            elif option == "--exclude":
                excludes.append(value)
            elif option == "--max-depth":
                max_depth = int(value)
            else:
                sys.exit(90)
        cutoff = time.time() - int(args[3][:-1]) * 86400
        for path in remote.rglob("*"):
            relative = path.relative_to(remote)
            if path.is_symlink() or not path.is_file() or path.stat().st_mtime >= cutoff:
                continue
            if max_depth is not None and len(relative.parts) > max_depth:
                continue
            matches = lambda pattern: fnmatch.fnmatchcase("/" + relative.as_posix(), pattern)
            if includes:
                selected = any(matches(pattern) for pattern in includes)
            else:
                selected = not any(matches(pattern) or fnmatch.fnmatchcase(relative.as_posix(), pattern)
                                   for pattern in excludes)
            if selected:
                path.unlink()
        sys.exit(0)
if tool == "date":
    assert args[0] == "-u" and len(args) == 2, args
    assert args[1] in ("+%Y%m%d-%H%M%S", "+%Y-%m-%dT%H:%M:%SZ"), args
    print(os.environ["FIXTURE_TIMESTAMP"] if args[1] == "+%Y%m%d-%H%M%S" else "2026-10-22T01:00:00Z")
    sys.exit(0)
if tool == "hostname" and not args:
    print("fixture-host")
    sys.exit(0)
if tool == "docker":
    assert args[:3] == ["ps", "-a", "--no-trunc"], args
    if len(args) == 3:
        # Default Docker output includes command arguments. This synthetic
        # canary makes an unrestricted inventory observable without a daemon.
        print("CONTAINER ID IMAGE COMMAND NAMES")
        print("fixture-id fixture-image --password=SYNTHETIC-COMMAND-CANARY fixture-container")
        sys.exit(0)
    assert args[3:] == ["--format", 'table {{.ID}}\\t{{.Image}}\\t{{.Names}}\\t{{.Status}}'], args
    if scenario.get("docker_fail"):
        sys.exit(12)
    print("CONTAINER ID IMAGE NAMES STATUS")
    print("fixture-id fixture-image fixture-container Up 1 minute")
    sys.exit(0)
sys.exit(90)
'''


class CloudBackupTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.backups = self.root / "backups"
        self.backups.mkdir()
        self.remote = self.root / "remote"
        self.remote.mkdir()
        self.log = self.root / "log/backup.log"
        self.old = []
        for name in ("tenants", "control-plane"):
            directory = self.root / name
            directory.mkdir()
            (directory / "fixture.txt").write_text(f"synthetic {name} data\n")
        for label, suffix in OWNED_TYPES:
            for encrypted in ("", ".age"):
                name = f"pulse-backup-{label}-20000101-000000{suffix}{encrypted}"
                archive = self.backups / name
                for parent in (self.backups, self.remote):
                    old = parent / name
                    old.write_bytes(b"retained previous fixture")
                    os.utime(old, (946684800, 946684800))
                self.old.append(archive)
        # PATH admits only these local utilities and closed fakes. No real
        # Docker, encryption client or cloud client can be reached.
        for name in ("mkdir", "touch", "chmod", "dirname", "basename", "du", "cut", "rm", "rmdir", "gzip", "realpath"):
            executable = shutil.which(name)
            self.assertIsNotNone(executable, f"required fixture dependency missing: {name}")
            (self.bin / name).symlink_to(executable)
        for name in ("tar", "find", "age", "rclone", "docker", "hostname", "date", "mv"):
            executable = self.bin / name
            executable.write_text(f"#!{sys.executable}\n" + FAKE)
            executable.chmod(0o700)
        self.env = {
            "PATH": str(self.bin), "FIXTURE": str(self.root), "HOME": str(self.root),
            "TMPDIR": str(self.root), "LC_ALL": "C", "REAL_TAR": shutil.which("tar"),
            "REAL_FIND": shutil.which("find"), "REAL_MV": shutil.which("mv"),
            "PULSE_DATA_DIR": str(self.root),
            "PULSE_BACKUP_DIR": str(self.backups), "PULSE_BACKUP_LOG_FILE": str(self.log),
            "PULSE_RCLONE_REMOTE": "fixture-remote:bucket",
            "PULSE_BACKUP_AGE_RECIPIENT": "fixture-recipient",
        }
        self.calls = []
        self.invocations = 0

    def invoke(self, scenario=None, **env):
        (self.root / "scenario.json").write_text(json.dumps(scenario or {}))
        (self.root / "calls.jsonl").unlink(missing_ok=True)
        self.log.unlink(missing_ok=True)
        self.invocations += 1
        invocation_env = dict(self.env, FIXTURE_TIMESTAMP=f"20261022-01{self.invocations:04d}")
        invocation_env.update(env)
        self.timestamp = invocation_env["FIXTURE_TIMESTAMP"]
        result = subprocess.run([BASH, str(SCRIPT)], cwd=self.root,
                                env=invocation_env, text=True, capture_output=True, timeout=20)
        self.output = result.stdout + result.stderr + (self.log.read_text() if self.log.exists() else "")
        calls = self.root / "calls.jsonl"
        self.calls = [json.loads(line) for line in calls.read_text().splitlines()] if calls.exists() else []
        return result

    def remove_source(self, name):
        (self.root / name / "fixture.txt").unlink()
        (self.root / name).rmdir()

    def assert_stopped_before_retention(self, result, before_archives=False):
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertNotIn("=== tarball backup finished ===", self.output)
        self.assertFalse(any(call[0] == "find" or call[:2] == ["rclone", "delete"] for call in self.calls),
                         self.calls)
        self.assertEqual(list(self.backups.glob(".pulse-backup-*.lock")), [])
        for archive in self.old:
            self.assertTrue(archive.exists(), self.output)
            self.assertEqual(archive.read_bytes(), b"retained previous fixture")
        if before_archives:
            self.assertEqual(self.calls, [], self.calls)
            self.assertEqual(set(self.backups.iterdir()), set(self.old))

    def assert_no_uploads(self):
        self.assertFalse(any(call[0] == "rclone" for call in self.calls), self.calls)

    def assert_no_new_completed_files(self):
        self.assertEqual([p.name for p in self.backups.iterdir() if p not in self.old
                          and self.timestamp in p.name
                          and p.name.endswith((".tar.gz", ".age", ".txt"))], [])

    def assert_remote_retention_scope(self):
        deletes = [call for call in self.calls if call[:2] == ["rclone", "delete"]]
        self.assertEqual(len(deletes), 1, self.calls)
        options = list(zip(deletes[0][5::2], deletes[0][6::2], strict=True))
        self.assertEqual(len(options), 8, options)
        self.assertEqual({value for option, value in options if option == "--include"},
                         {"/" + pattern for pattern in OWNED_PATTERNS})
        self.assertEqual([(option, value) for option, value in options if option != "--include"],
                         [("--max-depth", "1"), ("--exclude", "*")])

    def retention_fixtures(self, parent):
        """Old unrelated files and young owned files must survive byte-for-byte."""
        preserved = {}
        unrelated = ("operator-notes.txt", "pulse-backup-other-20000101-000000.tar.gz",
                     "pulse-backup-tenants-latest.tar.gz", "pulse-backup-cp-20000101-000000.tar.gz.sha256",
                     "pulse-backup-meta-20000101-000000.txt.age.extra",
                     "pulse-backup-tenants-2000010X-000000.tar.gz",
                     "pulse-backup-cp-20000101-00000X.tar.gz.age",
                     "pulse-backup-meta-20000101-000000.tar.gz",
                     "pulse-backup-tenants-not-a-timestamp.tar.gz.partial",
                     "pulse-backup-meta-20000101-000000.txt.age.partial.extra",
                     "nested/pulse-backup-cp-20000101-000000.tar.gz.age.partial",
                     "nested/pulse-backup-tenants-20000101-000000.tar.gz",
                     "nested/pulse-backup-meta-20000101-000000.txt.age")
        for name in unrelated:
            path = parent / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"synthetic unrelated preserved data")
            os.utime(path, (946684800, 946684800))
            preserved[path] = path.read_bytes()
        for label, suffix in OWNED_TYPES:
            for encrypted in ("", ".age"):
                path = parent / f"pulse-backup-{label}-20990101-000000{suffix}{encrypted}"
                path.write_bytes(b"synthetic young preserved backup")
                os.utime(path, (4102444800, 4102444800))
                preserved[path] = path.read_bytes()
                if parent == self.remote:
                    partial = parent / f"pulse-backup-{label}-20000101-000000{suffix}{encrypted}.partial"
                    partial.write_bytes(b"synthetic unrelated remote partial")
                    os.utime(partial, (946684800, 946684800))
                    preserved[partial] = partial.read_bytes()
        return preserved

    def check_retention_scope(self, **env):
        preserved = {}
        for parent in (self.backups, self.remote):
            preserved.update(self.retention_fixtures(parent))
        result = self.invoke(**env)
        self.assertEqual(result.returncode, 0, self.output)
        for path, content in preserved.items():
            self.assertTrue(path.is_file(), f"unowned or young file removed: {path.relative_to(self.root)}")
            self.assertEqual(path.read_bytes(), content)
        for path in self.old:
            self.assertFalse(path.exists(), f"expired owned file survived: {path.name}")
            self.assertFalse((self.remote / path.name).exists(), f"expired remote fixture survived: {path.name}")
        self.assert_remote_retention_scope()

    def test_retention_expires_only_owned_top_level_files_in_both_formats(self):
        self.check_retention_scope()

    def test_encrypted_job_also_expires_earlier_plaintext_metadata(self):
        self.check_retention_scope(PULSE_BACKUP_ENCRYPT="true")

    def test_remote_retention_uses_rooted_generated_name_filters(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_remote_retention_scope()

    def test_local_retention_preserves_links_and_directories_with_backup_names(self):
        preserved = self.retention_fixtures(self.backups)
        directory = self.backups / "pulse-backup-cp-20000202-000000.tar.gz"
        directory.mkdir()
        marker = directory / "operator-owned.txt"
        marker.write_bytes(b"synthetic directory content")
        os.utime(directory, (946684800, 946684800))
        target = self.root / "unrelated-target.txt"
        target.write_bytes(b"synthetic link target")
        linked = self.backups / "pulse-backup-meta-20000202-000000.txt"
        linked.symlink_to(target)
        os.utime(linked, (946684800, 946684800), follow_symlinks=False)
        result = self.invoke(PULSE_RCLONE_REMOTE="")
        self.assertEqual(result.returncode, 0, self.output)
        self.assertTrue(linked.is_symlink())
        self.assertEqual(target.read_bytes(), b"synthetic link target")
        self.assertEqual(marker.read_bytes(), b"synthetic directory content")
        self.assertEqual({path: path.read_bytes() for path in preserved}, preserved)
        self.assert_no_uploads()

    def check_remote_configuration_not_logged(self, mode, failed_copy=None):
        # Exercise the real helper's own log formatting, not native rclone
        # parsing. This opaque synthetic destination reaches only the closed
        # fixture adapter, which verifies the exact unchanged argument.
        remote = "fixture-remote,SYNTHETIC-REMOTE-CONFIGURATION-CANARY:private-bucket"
        scenario = {"upload_fail": failed_copy} if failed_copy else {}
        result = self.invoke(scenario, PULSE_BACKUP_ENCRYPT=mode, PULSE_RCLONE_REMOTE=remote)
        if failed_copy:
            self.assert_stopped_before_retention(result)
        else:
            self.assertEqual(result.returncode, 0, self.output)
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertEqual(len(copies), failed_copy or 3, self.calls)
        for call in copies:
            self.assertEqual(call[3], remote + "/pulse-cloud-tarballs/" + Path(call[2]).name)
        self.assertNotIn("SYNTHETIC-REMOTE-CONFIGURATION-CANARY", self.output)
        self.assertNotIn(remote, self.output)
        for call in copies:
            self.assertIn("uploading backup file: " + Path(call[2]).name, self.output)
        self.assertEqual(self.output.count("upload complete:"), (failed_copy - 1) if failed_copy else 3)

    def test_success_logs_identify_files_without_remote_configuration(self):
        for mode in ("false", "true"):
            with self.subTest(encrypted=mode):
                self.check_remote_configuration_not_logged(mode)

    def test_failed_upload_logs_do_not_echo_remote_configuration(self):
        for mode in ("false", "true"):
            for failed_copy in (1, 2, 3):
                with self.subTest(encrypted=mode, failed_copy=failed_copy):
                    self.check_remote_configuration_not_logged(mode, failed_copy)

    def test_failed_upload_cannot_expire_archives_or_metadata_in_either_store(self):
        self.assert_stopped_before_retention(self.invoke({"upload_fail": 3}))
        for path in self.old:
            self.assertEqual((self.remote / path.name).read_bytes(), b"retained previous fixture")

    def assert_private_remote_log(self):
        self.assertNotIn("SYNTHETIC-PRIVATE-REMOTE", self.output)
        self.assertNotIn("SYNTHETIC-CLIENT-STDOUT", self.output)
        self.assertNotIn("SYNTHETIC-CLIENT-STDERR", self.output)
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertTrue(copies, self.calls)
        self.assertTrue(all("SYNTHETIC-PRIVATE-REMOTE" in call[3] for call in copies), copies)
        self.assertTrue(all(call[4:] == ["--error-on-no-transfer"] for call in copies), copies)

    def private_remote(self):
        # A valid rclone connection-string shape, with synthetic values only.
        return ":s3,access_key_id=SYNTHETIC-PRIVATE-REMOTE,secret_access_key=SYNTHETIC-PRIVATE-REMOTE:bucket"

    def assert_current_recovery_archives(self, encrypted):
        for label, directory in (("tenants", "tenants"), ("cp", "control-plane")):
            suffix = ".tar.gz.age" if encrypted else ".tar.gz"
            path = self.backups / f"pulse-backup-{label}-{self.timestamp}{suffix}"
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            content = path.read_bytes()
            if encrypted:
                self.assertTrue(content.startswith(b"FAKE-AGE\n"))
                content = content.removeprefix(b"FAKE-AGE\n")
            with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
                self.assertEqual(archive.extractfile(f"{directory}/fixture.txt").read(),
                                 f"synthetic {directory} data\n".encode())

    def test_remote_success_logs_keep_destination_and_client_output_private(self):
        for mode in ("false", "true"):
            with self.subTest(mode=mode):
                result = self.invoke({"private_client_output": True},
                                     PULSE_BACKUP_ENCRYPT=mode, PULSE_RCLONE_REMOTE=self.private_remote())
                self.assertEqual(result.returncode, 0, self.output)
                self.assert_private_remote_log()
                self.assertEqual(self.output.count("upload complete:"), 3, self.output)
                self.assertIn("=== tarball backup finished ===", self.output)
                self.assert_remote_retention_scope()
                self.assert_current_recovery_archives(encrypted=mode == "true")

    def test_remote_upload_error_logs_keep_private_data_and_preserve_recovery(self):
        for mode in ("false", "true"):
            for number in (1, 2, 3):
                with self.subTest(mode=mode, number=number):
                    result = self.invoke({"upload_fail": number, "private_client_output": True},
                                         PULSE_BACKUP_ENCRYPT=mode, PULSE_RCLONE_REMOTE=self.private_remote())
                    self.assert_skipped_upload_stops_retention(result, number)
                    self.assert_private_remote_log()
                    self.assertIn("rclone exit 10", self.output)
                    self.assert_current_recovery_archives(encrypted=mode == "true")

    def test_remote_retention_error_logs_keep_private_data_and_completed_archives(self):
        for mode in ("false", "true"):
            with self.subTest(mode=mode):
                result = self.invoke({"remote_retention_fail": True, "private_client_output": True},
                                     PULSE_BACKUP_ENCRYPT=mode, PULSE_RCLONE_REMOTE=self.private_remote())
                self.assertNotEqual(result.returncode, 0, self.output)
                self.assert_private_remote_log()
                self.assertIn("Remote retention failed", self.output)
                self.assertIn("rclone exit 11", self.output)
                self.assertNotIn("=== tarball backup finished ===", self.output)
                self.assertEqual(self.output.count("upload complete:"), 3, self.output)
                self.assert_remote_retention_scope()
                self.assert_current_recovery_archives(encrypted=mode == "true")
                for path in self.old:
                    self.assertEqual((self.remote / path.name).read_bytes(), b"retained previous fixture")

    def assert_skipped_upload_stops_retention(self, result, number):
        self.assert_stopped_before_retention(result)
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertEqual(len(copies), number, self.calls)
        self.assertTrue(all(call[4:] == ["--error-on-no-transfer"] for call in copies), copies)
        self.assertEqual(self.output.count("upload complete:"), number - 1, self.output)
        self.assertIn("Remote upload failed or transferred no file; no retention performed", self.output)
        for path in self.old:
            self.assertEqual((self.remote / path.name).read_bytes(), b"retained previous fixture")
        current = [path for path in self.backups.iterdir() if self.timestamp in path.name]
        self.assertEqual(len(current), 3, current)
        self.assertFalse(any(path.name.endswith(".partial") for path in current), current)
        self.assertTrue(all(path.stat().st_mode & 0o777 == 0o600 for path in current))
        for call in copies[:-1]:
            local = Path(call[2])
            self.assertEqual((self.remote / local.name).read_bytes(), local.read_bytes())

    def test_each_skipped_upload_preserves_prior_local_and_remote_recovery_files(self):
        for mode in ("false", "true"):
            for number in (1, 2, 3):
                with self.subTest(mode=mode, number=number):
                    result = self.invoke({"upload_skip": number}, PULSE_BACKUP_ENCRYPT=mode)
                    self.assert_skipped_upload_stops_retention(result, number)
                    skipped = [call for call in self.calls if call[:2] == ["rclone", "copyto"]][-1]
                    self.assertFalse((self.remote / Path(skipped[2]).name).exists())

    def test_unsupported_transfer_flag_fails_closed_without_retention(self):
        for mode in ("false", "true"):
            with self.subTest(mode=mode):
                result = self.invoke({"unsupported_transfer_flag": True}, PULSE_BACKUP_ENCRYPT=mode)
                self.assert_skipped_upload_stops_retention(result, 1)
                self.assertEqual([p for p in self.remote.iterdir() if self.timestamp in p.name], [])

    @unittest.skipUnless(REAL_RCLONE, "real rclone is not supplied or installed; native filter engine unverified")
    def test_real_rclone_local_backend_preserves_unowned_objects(self):
        # The adapter replaces only the fixed fixture destination with a local
        # path. Real rclone sees no remote config, network or provider identity.
        self.check_retention_scope(REAL_RCLONE=REAL_RCLONE)

    @unittest.skipUnless(REAL_RCLONE, "real rclone is not supplied or installed; native skipped-destination gate unverified")
    def test_real_rclone_skipped_existing_destination_cannot_authorize_retention(self):
        # Establish native bare-copy exit-zero skips, then run the entire helper
        # with the same settings. The explicit false environment default must
        # not defeat the command's required no-transfer failure. Only disposable
        # local paths are used, with no remote configuration or network identity.
        for mode in ("false", "true"):
            for setting in ("RCLONE_IGNORE_EXISTING", "RCLONE_UPDATE"):
                for number, (label, suffix) in enumerate(OWNED_TYPES, 1):
                    with self.subTest(mode=mode, setting=setting, number=number):
                        timestamp = f"20261022-01{self.invocations + 1:04d}"
                        filename = f"pulse-backup-{label}-{timestamp}{suffix}" + (".age" if mode == "true" else "")
                        existing = self.remote / filename
                        original = b"synthetic existing remote object, never replaced"
                        existing.write_bytes(original)
                        os.utime(existing, (4102444800, 4102444800))
                        probe = self.root / "current-archive-probe"
                        probe.write_bytes(b"different synthetic current archive")
                        env = {setting: "true", "RCLONE_ERROR_ON_NO_TRANSFER": "false"}
                        bare = subprocess.run([REAL_RCLONE, "--config", os.devnull, "copyto",
                                               str(probe), str(existing)], env=dict(self.env, **env),
                                              capture_output=True, timeout=10)
                        self.assertEqual(bare.returncode, 0, "bare native existing-destination copy did not succeed")
                        self.assertEqual(existing.read_bytes(), original)
                        result = self.invoke(REAL_RCLONE=REAL_RCLONE, PULSE_BACKUP_ENCRYPT=mode, **env)
                        self.assert_skipped_upload_stops_retention(result, number)
                        self.assertEqual(existing.read_bytes(), original)
        # Removing the skip settings restores the ordinary encrypted transfer
        # path. The existing separate native case covers plain transfers/expiry.
        result = self.invoke(REAL_RCLONE=REAL_RCLONE, PULSE_BACKUP_ENCRYPT="true")
        self.assertEqual(result.returncode, 0, self.output)
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertEqual(len(copies), 3)
        for call in copies:
            local = Path(call[2])
            self.assertEqual((self.remote / local.name).read_bytes(), local.read_bytes())
        self.assertTrue(all(not path.exists() and not (self.remote / path.name).exists() for path in self.old))

    def assert_archive_contents(self):
        for label, directory in (("tenants", "tenants"), ("cp", "control-plane")):
            archives = [path for path in self.backups.glob(f"pulse-backup-{label}-*.tar.gz") if path not in self.old]
            self.assertEqual(len(archives), 1, self.output)
            self.assertEqual(archives[0].stat().st_mode & 0o777, 0o600)
            with tarfile.open(archives[0], "r:gz") as archive:
                self.assertEqual(archive.extractfile(f"{directory}/fixture.txt").read(),
                                 f"synthetic {directory} data\n".encode())

    def symlink_source(self, name):
        target = self.root / ("physical-" + name)
        (self.root / name).rename(target)
        (self.root / name).symlink_to(target, target_is_directory=True)

    def assert_layout_rejected(self, staging, **env):
        # Snapshot source bytes and permissions, not just the exit code. Layout
        # admission must precede staging mkdir/chmod, archives and all expiry.
        sources = [self.root / name for name in ("tenants", "control-plane")]
        before = {source: (source.stat().st_mode, sorted(p.relative_to(source) for p in source.rglob("*")),
                           (source / "fixture.txt").read_bytes()) for source in sources}
        result = self.invoke(PULSE_BACKUP_DIR=staging, **env)
        self.assert_stopped_before_retention(result, before_archives=True)
        self.assert_no_uploads()
        self.assertIn("PULSE_BACKUP_DIR must be outside both source directories", self.output)
        after = {source: (source.stat().st_mode, sorted(p.relative_to(source) for p in source.rglob("*")),
                          (source / "fixture.txt").read_bytes()) for source in sources}
        self.assertEqual(after, before)

    def test_staging_equal_to_source_is_rejected_before_chmod(self):
        source = self.root / "tenants"
        source.chmod(0o750)
        self.assert_layout_rejected(str(source))

    def test_staging_inside_control_plane_is_rejected_before_creation(self):
        staging = self.root / "control-plane/not-created/backups"
        self.assert_layout_rejected(str(staging))
        self.assertFalse(staging.parent.exists())

    def test_staging_symlink_into_source_is_rejected(self):
        alias = self.root / "backup-alias"
        alias.symlink_to(self.root / "tenants", target_is_directory=True)
        self.assert_layout_rejected(str(alias))
        self.assertTrue(alias.is_symlink())

    def test_staging_layout_is_checked_for_both_sources_and_encryption_modes(self):
        for name in ("tenants", "control-plane"):
            for encrypted in ("true", "false"):
                for suffix in ("", "/not-created/nested"):
                    with self.subTest(source=name, encrypted=encrypted, suffix=suffix):
                        self.assert_layout_rejected(str(self.root / name) + suffix,
                                                    PULSE_BACKUP_ENCRYPT=encrypted)

    def test_relative_dot_and_parent_components_cannot_hide_staging_overlap(self):
        self.assert_layout_rejected("tenants/./not-created/../backups///", PULSE_RCLONE_REMOTE="")
        self.assertFalse((self.root / "tenants/not-created").exists())

    def test_symlinked_source_parent_cannot_hide_staging_overlap(self):
        (self.root / "data-alias").symlink_to(self.root, target_is_directory=True)
        self.assert_layout_rejected(str(self.root / "tenants/backups"),
                                    PULSE_TENANTS_DIR=str(self.root / "data-alias/tenants"))

    def test_symlink_and_parent_component_use_physical_not_lexical_location(self):
        (self.root / "tenants/subdir").mkdir()
        alias = self.root / "backup-alias"
        alias.symlink_to(self.root / "tenants/subdir", target_is_directory=True)
        self.assert_layout_rejected(str(alias) + "/../not-created/backups")

    def test_newlines_in_paths_do_not_weaken_staging_admission(self):
        self.assert_layout_rejected(str(self.root / "tenants/not-created\n"))
        # A physical source ending in a newline must not be truncated by shell
        # command substitution before comparison. Keep the original fixture
        # path as an alias solely for the unchanged-data snapshot assertion.
        physical = self.root / "tenants\n"
        (self.root / "tenants").rename(physical)
        (self.root / "tenants").symlink_to(physical, target_is_directory=True)
        self.assert_layout_rejected(str(physical / "not-created\n"),
                                    PULSE_TENANTS_DIR=str(physical))

    def test_missing_realpath_stops_before_archiving(self):
        (self.bin / "realpath").unlink()
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)
        self.assertIn("realpath (GNU coreutils) is required", self.output)

    def test_failed_staging_resolution_stops_before_chmod(self):
        # A resolver failure must not silently become an empty, admitted path.
        (self.bin / "realpath").unlink()
        (self.bin / "realpath").write_text("#!/bin/sh\nexit 1\n")
        (self.bin / "realpath").chmod(0o700)
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)
        self.assertIn("Cannot resolve PULSE_BACKUP_DIR", self.output)

    def test_failed_source_resolution_stops_before_archiving(self):
        (self.bin / "realpath").unlink()
        (self.bin / "realpath").write_text(
            '#!/bin/sh\nif [ "$1" = "-m" ]; then printf "%s\\0" "$PULSE_BACKUP_DIR"; else exit 1; fi\n')
        (self.bin / "realpath").chmod(0o700)
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)
        self.assertIn("Cannot resolve required tenants directory", self.output)

    def test_similarly_prefixed_sibling_staging_is_valid(self):
        sibling = self.root / "tenants-backups"
        self.backups.rename(sibling)
        self.backups = sibling
        self.old = [sibling / path.name for path in self.old]
        result = self.invoke(PULSE_BACKUP_DIR=str(sibling))
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_archive_contents()

    def test_safe_staging_alias_and_relative_paths_preserve_archive_layout(self):
        (self.root / "data-alias").symlink_to(self.root, target_is_directory=True)
        result = self.invoke(PULSE_BACKUP_DIR="./data-alias/backups/../backups///",
                             PULSE_TENANTS_DIR="./data-alias/tenants")
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_archive_contents()

    def check_literal_source(self, source_name, parent_name=None, relative=False):
        # A real file list makes GNU tar's old option interpretation succeed
        # with the wrong data. The adapter does not model tar option parsing.
        (self.root / "members.txt").write_text("unrelated.txt\n")
        (self.root / "unrelated.txt").write_bytes(b"synthetic unrelated canary")
        for directory, label, variable in (("tenants", "tenants", "PULSE_TENANTS_DIR"),
                                            ("control-plane", "cp", "PULSE_CONTROL_PLANE_DIR")):
            original = self.root / directory
            parent = self.root / parent_name if parent_name else self.root
            parent.mkdir(exist_ok=True)
            configured = parent / source_name
            original.rename(configured)
            try:
                for encrypted in ("false", "true"):
                    with self.subTest(directory=directory, encrypted=encrypted, name=source_name):
                        path = configured.relative_to(self.root) if relative else configured
                        result = self.invoke(**{variable: str(path), "PULSE_BACKUP_ENCRYPT": encrypted})
                        self.assertEqual(result.returncode, 0, self.output)
                        suffix = ".tar.gz.age" if encrypted == "true" else ".tar.gz"
                        files = [p for p in self.backups.glob(f"pulse-backup-{label}-*{suffix}")
                                 if p not in self.old]
                        # Each invocation uses a distinct timestamp; inspect the
                        # latest archive, including the actual gzip/tar payload.
                        archive_path = max(files, key=lambda p: p.name)
                        content = archive_path.read_bytes()
                        if encrypted == "true":
                            self.assertTrue(content.startswith(b"FAKE-AGE\n"))
                            content = content.removeprefix(b"FAKE-AGE\n")
                        with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
                            self.assertEqual(set(archive.getnames()),
                                             {source_name, f"{source_name}/fixture.txt"})
                            self.assertEqual(archive.extractfile(f"{source_name}/fixture.txt").read(),
                                             f"synthetic {directory} data\n".encode())
                        self.assertIn("=== tarball backup finished ===", self.output)
                        self.assertEqual(len([call for call in self.calls
                                              if call[:2] == ["rclone", "copyto"]]), 3)
                        self.assertEqual(len([call for call in self.calls if call[0] == "find"]), 1)
                        self.assert_remote_retention_scope()
                        self.assertEqual((configured / "fixture.txt").read_bytes(),
                                         f"synthetic {directory} data\n".encode())
            finally:
                configured.rename(original)

    def test_long_option_source_name_is_archived_literally(self):
        self.check_literal_source("--files-from=members.txt")

    def test_short_option_source_name_is_archived_literally(self):
        self.check_literal_source("-Tmembers.txt")

    def test_relative_option_source_name_is_archived_literally(self):
        self.check_literal_source("-Tmembers.txt", relative=True)

    def test_source_trailing_newline_is_preserved(self):
        self.check_literal_source("literal-source\n")

    def test_source_parent_trailing_newline_is_preserved(self):
        self.check_literal_source("literal-source", parent_name="literal-parent\n")

    def test_whitespace_and_embedded_newline_source_names_remain_literal(self):
        self.check_literal_source("literal source\nwith spaces", relative=True)

    def check_fixed_tar_policy(self, options):
        # The adapter passes the environment to real GNU tar. Do not emulate
        # exclusions, transformations or dereferencing in the closed fake.
        outside = self.root / "outside-data"
        outside.mkdir()
        canary = outside / "not-part-of-source.txt"
        canary.write_bytes(b"synthetic out-of-tree data\n")
        for directory in ("tenants", "control-plane"):
            (self.root / directory / "outside-link").symlink_to(outside, target_is_directory=True)
        for encrypted in ("false", "true"):
            with self.subTest(encrypted=encrypted):
                result = self.invoke(TAR_OPTIONS=options, PULSE_BACKUP_ENCRYPT=encrypted)
                self.assertEqual(result.returncode, 0, self.output)
                suffix = ".tar.gz.age" if encrypted == "true" else ".tar.gz"
                for label, directory in (("tenants", "tenants"), ("cp", "control-plane")):
                    with self.subTest(source=directory):
                        files = [p for p in self.backups.glob(f"pulse-backup-{label}-*{suffix}")
                                 if p not in self.old]
                        content = max(files, key=lambda p: p.name).read_bytes()
                        if encrypted == "true":
                            self.assertTrue(content.startswith(b"FAKE-AGE\n"))
                            content = content.removeprefix(b"FAKE-AGE\n")
                        with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
                            self.assertEqual(set(archive.getnames()),
                                             {directory, f"{directory}/fixture.txt", f"{directory}/outside-link"})
                            self.assertEqual(archive.extractfile(f"{directory}/fixture.txt").read(),
                                             f"synthetic {directory} data\n".encode())
                            link = archive.getmember(f"{directory}/outside-link")
                            self.assertTrue(link.issym())
                            self.assertEqual(link.linkname, str(outside))
                self.assertIn("=== tarball backup finished ===", self.output)
                self.assertEqual(len([call for call in self.calls if call[:2] == ["rclone", "copyto"]]), 3)
                self.assertEqual(len([call for call in self.calls if call[0] == "find"]), 1)
                self.assert_remote_retention_scope()
                self.assertTrue(all(not p.exists() for p in self.old))
                self.assertEqual(canary.read_bytes(), b"synthetic out-of-tree data\n")
                for directory in ("tenants", "control-plane"):
                    self.assertEqual((self.root / directory / "fixture.txt").read_bytes(),
                                     f"synthetic {directory} data\n".encode())
                    self.assertTrue((self.root / directory / "outside-link").is_symlink())

    def test_inherited_tar_exclusions_cannot_make_empty_successful_backups(self):
        self.check_fixed_tar_policy("--exclude=*")

    def test_inherited_tar_transforms_cannot_change_recovery_paths(self):
        self.check_fixed_tar_policy("--transform=s/fixture.txt/renamed.txt/")

    def test_inherited_tar_dereferencing_cannot_copy_out_of_tree_data(self):
        self.check_fixed_tar_policy("--dereference")

    def test_invalid_inherited_tar_options_do_not_break_backup_jobs(self):
        self.check_fixed_tar_policy("--pulse-invalid-option")

    def test_failed_member_or_parent_resolution_stops_without_upload_or_retention(self):
        for name, message in (("dirname", "parent directory"), ("basename", "archive member")):
            with self.subTest(tool=name):
                executable = self.bin / name
                real_tool = os.readlink(executable)
                executable.unlink()
                # Logging still uses ordinary dirname before preflight. Refuse
                # only the archive-path resolver, not log setup.
                executable.write_text(f'#!/bin/sh\n[ "$1" = "-z" ] && exit 1\nexec "{real_tool}" "$@"\n')
                executable.chmod(0o700)
                try:
                    for encrypted in ("false", "true"):
                        result = self.invoke(PULSE_BACKUP_ENCRYPT=encrypted)
                        self.assert_stopped_before_retention(result, before_archives=True)
                        self.assert_no_uploads()
                        self.assertIn(f"Cannot determine required tenants {message}", self.output)
                finally:
                    executable.unlink()
                    executable.symlink_to(real_tool)

    def test_same_second_failure_does_not_destroy_a_completed_backup(self):
        timestamp = "20261022-010001"
        result = self.invoke(FIXTURE_TIMESTAMP=timestamp)
        self.assertEqual(result.returncode, 0, self.output)
        completed = {p: p.read_bytes() for p in self.backups.iterdir() if p.is_file()}
        (self.root / "tenants/fixture.txt").write_text("new synthetic data\n")
        result = self.invoke({"tar_fail": 1}, FIXTURE_TIMESTAMP=timestamp)
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertEqual({p: p.read_bytes() for p in completed}, completed)
        self.assertIn("already exists", self.output)
        self.assertEqual(self.calls, [])

    def test_each_existing_output_is_preserved_before_archives_or_retention(self):
        for index, label in enumerate(("tenants", "cp", "meta", "tenants-encrypted", "cp-encrypted", "meta-encrypted"), 1):
            with self.subTest(label=label):
                timestamp = f"20261022-02{index:04d}"
                encrypted = label.endswith("-encrypted")
                name = label.removesuffix("-encrypted")
                suffix = (".txt" if name == "meta" else ".tar.gz") + (".age" if encrypted else "")
                existing = self.backups / f"pulse-backup-{name}-{timestamp}{suffix}"
                existing.write_bytes(b"previous completed fixture")
                result = self.invoke(FIXTURE_TIMESTAMP=timestamp,
                                     PULSE_BACKUP_ENCRYPT="true" if encrypted else "false")
                self.assert_stopped_before_retention(result)
                self.assertEqual(existing.read_bytes(), b"previous completed fixture")
                self.assertEqual(self.calls, [])
                self.assertEqual(list(self.backups.glob(".pulse-backup-*.lock")), [])

    def test_output_symlinks_cannot_overwrite_an_existing_file(self):
        for index, label in enumerate(("tenants", "cp", "meta", "tenants-encrypted", "cp-encrypted", "meta-encrypted"), 1):
            with self.subTest(label=label):
                timestamp = f"20261022-03{index:04d}"
                encrypted = label.endswith("-encrypted")
                name = label.removesuffix("-encrypted")
                suffix = (".txt" if name == "meta" else ".tar.gz") + (".age" if encrypted else "")
                target = self.backups / f"preserved-{index}.txt"
                target.write_bytes(b"synthetic unrelated file")
                existing = self.backups / f"pulse-backup-{name}-{timestamp}{suffix}"
                existing.symlink_to(target)
                result = self.invoke(FIXTURE_TIMESTAMP=timestamp,
                                     PULSE_BACKUP_ENCRYPT="true" if encrypted else "false")
                self.assert_stopped_before_retention(result)
                self.assertEqual(target.read_bytes(), b"synthetic unrelated file")
                self.assertTrue(existing.is_symlink())
                self.assertEqual(self.calls, [])

    def test_dangling_output_link_is_preserved(self):
        timestamp = "20261022-050001"
        existing = self.backups / f"pulse-backup-meta-{timestamp}.txt"
        target = self.backups / "must-not-be-created.txt"
        existing.symlink_to(target)
        result = self.invoke(FIXTURE_TIMESTAMP=timestamp)
        self.assert_stopped_before_retention(result)
        self.assertTrue(existing.is_symlink())
        self.assertFalse(target.exists())
        self.assertEqual(self.calls, [])

    def test_another_jobs_name_reservation_is_not_removed(self):
        timestamp = "20261022-060001"
        reservation = self.backups / f".pulse-backup-{timestamp}.lock"
        reservation.mkdir()
        marker = reservation / "belongs-to-another-job.txt"
        marker.write_bytes(b"synthetic owner marker")
        result = self.invoke(FIXTURE_TIMESTAMP=timestamp)
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertIn("timestamp already reserved", self.output)
        self.assertEqual(marker.read_bytes(), b"synthetic owner marker")
        self.assertEqual(self.calls, [])
        self.assertTrue(all(p.exists() for p in self.old))

    def test_concurrent_same_timestamp_job_stops_before_any_mutation(self):
        (self.root / "scenario.json").write_text(json.dumps({"block_first_tar": True}))
        env = dict(self.env, FIXTURE_TIMESTAMP="20261022-040001")
        first = subprocess.Popen([BASH, str(SCRIPT)], cwd=self.root, env=env,
                                 text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            deadline = time.monotonic() + 7
            while not (self.root / "tar-entered").exists():
                if first.poll() is not None or time.monotonic() >= deadline:
                    self.fail("first fixture job did not reach the bounded archive barrier")
                time.sleep(0.02)
            second = subprocess.run([BASH, str(SCRIPT)], cwd=self.root, env=env,
                                    text=True, capture_output=True, timeout=7)
            calls_before_release = [json.loads(line) for line in (self.root / "calls.jsonl").read_text().splitlines()]
        finally:
            (self.root / "release-tar").touch()
            first_output = first.communicate(timeout=12)
        self.output = "".join(first_output) + second.stdout + second.stderr + self.log.read_text()
        self.assertEqual(first.returncode, 0, self.output)
        self.assertNotEqual(second.returncode, 0, second.stdout + second.stderr)
        self.assertEqual([call[0] for call in calls_before_release if call[0] != "date"], ["tar"])
        self.assertEqual(list(self.backups.glob(".pulse-backup-*.lock")), [])
        self.assert_archive_contents()

    def test_symlinked_tenants_root_preserves_last_archives(self):
        self.symlink_source("tenants")
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)
        self.assertIn("Required tenants directory is a symbolic link", self.output)
        self.assertIn("physical directory", self.output)

    def test_symlinked_control_plane_root_preserves_last_archives(self):
        self.symlink_source("control-plane")
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)
        self.assertIn("Required control-plane directory is a symbolic link", self.output)

    def test_symlinked_root_with_trailing_slashes_is_not_admitted(self):
        self.symlink_source("tenants")
        self.assert_stopped_before_retention(
            self.invoke(PULSE_TENANTS_DIR=str(self.root / "tenants") + "///"), before_archives=True)
        self.assertIn("Required tenants directory is a symbolic link", self.output)

    def test_local_only_symlinked_source_also_preserves_last_archives(self):
        self.symlink_source("control-plane")
        self.assert_stopped_before_retention(self.invoke(PULSE_RCLONE_REMOTE=""), before_archives=True)

    def test_source_becoming_symlink_after_preflight_stops_before_retention(self):
        result = self.invoke({"source_becomes_symlink": True})
        self.assert_stopped_before_retention(result)
        self.assertIn("Required control-plane directory is a symbolic link", self.output)
        self.assertEqual(len([call for call in self.calls if call[0] == "tar"]), 1)
        self.assert_no_uploads()

    def test_parent_symlink_still_archives_the_actual_data(self):
        (self.root / "data-alias").symlink_to(self.root, target_is_directory=True)
        result = self.invoke(PULSE_TENANTS_DIR=str(self.root / "data-alias/tenants"), PULSE_RCLONE_REMOTE="")
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_archive_contents()

    def test_nested_symlinks_remain_links_not_out_of_tree_copies(self):
        outside = self.root / "outside-data"
        outside.mkdir()
        (outside / "not-part-of-source.txt").write_text("synthetic out-of-tree data\n")
        (self.root / "tenants/outside-link").symlink_to(outside, target_is_directory=True)
        result = self.invoke(PULSE_RCLONE_REMOTE="")
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_archive_contents()
        archive_path = next(self.backups.glob("pulse-backup-tenants-*.tar.gz"))
        with tarfile.open(archive_path, "r:gz") as archive:
            self.assertTrue(archive.getmember("tenants/outside-link").issym())
            self.assertFalse(any(name.endswith("not-part-of-source.txt") for name in archive.getnames()))

    def test_missing_both_sources_preserves_last_archives(self):
        self.remove_source("tenants")
        self.remove_source("control-plane")
        result = self.invoke(PULSE_RCLONE_REMOTE="")
        self.assert_stopped_before_retention(result, before_archives=True)
        self.assertIn("Required tenants directory not found", self.output)

    def test_missing_tenants_stops_whole_job(self):
        self.remove_source("tenants")
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)

    def test_missing_control_plane_stops_whole_job(self):
        self.remove_source("control-plane")
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)

    def test_source_must_be_a_directory(self):
        self.remove_source("tenants")
        (self.root / "tenants").write_text("not a directory")
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)

    def test_source_disappearing_after_preflight_cannot_enable_retention(self):
        result = self.invoke({"source_disappears": True}, PULSE_RCLONE_REMOTE="")
        self.assert_stopped_before_retention(result)
        self.assertIn("Required control-plane directory disappeared", self.output)
        self.assertEqual(len([call for call in self.calls if call[0] == "tar"]), 1)
        self.assert_no_uploads()

    def test_invalid_encryption_setting_does_not_silently_make_plaintext(self):
        for setting in ("TRUE", "1", "yes", " true", "false "):
            with self.subTest(setting=setting):
                result = self.invoke(PULSE_BACKUP_ENCRYPT=setting)
                self.assert_stopped_before_retention(result, before_archives=True)
                self.assertIn("PULSE_BACKUP_ENCRYPT must be true or false", self.output)

    def test_invalid_retention_cannot_be_ignored(self):
        for setting in ("0", "000", "-1", "1.5", "days", "30d", " 30", "1\n"):
            with self.subTest(setting=setting):
                result = self.invoke(PULSE_BACKUP_RETENTION=setting)
                self.assert_stopped_before_retention(result, before_archives=True)
                self.assertIn("PULSE_BACKUP_RETENTION must be a positive whole number", self.output)

    def test_missing_age_is_admitted_before_archiving(self):
        (self.bin / "age").unlink()
        self.assert_stopped_before_retention(self.invoke(PULSE_BACKUP_ENCRYPT="true"), before_archives=True)

    def test_missing_recipient_is_admitted_before_archiving(self):
        self.assert_stopped_before_retention(
            self.invoke(PULSE_BACKUP_ENCRYPT="true", PULSE_BACKUP_AGE_RECIPIENT=""), before_archives=True)

    def test_missing_remote_client_is_admitted_before_archiving(self):
        (self.bin / "rclone").unlink()
        self.assert_stopped_before_retention(self.invoke(), before_archives=True)

    def test_remote_dry_run_stops_before_archives_uploads_or_retention(self):
        self.check_remote_execution_setting_rejected("RCLONE_DRY_RUN")

    def test_interactive_remote_stops_before_archives_uploads_or_retention(self):
        self.check_remote_execution_setting_rejected("RCLONE_INTERACTIVE")

    def check_remote_execution_setting_rejected(self, setting):
        for mode in ("false", "true"):
            for value in ("1", "t", "T", "TRUE", "true", "True"):
                with self.subTest(setting=setting, encrypted=mode, value=value):
                    self.backups.chmod(0o750)
                    before_mode = self.backups.stat().st_mode
                    result = self.invoke(PULSE_BACKUP_ENCRYPT=mode, **{setting: value})
                    self.assert_stopped_before_retention(result, before_archives=True)
                    self.assertEqual(self.backups.stat().st_mode, before_mode)
                    self.assertIn(f"{setting} must be unset or false", self.output)
                    for archive in self.old:
                        self.assertEqual((self.remote / archive.name).read_bytes(), b"retained previous fixture")

    def test_invalid_remote_execution_settings_fail_closed_without_echoing_values(self):
        for setting in ("RCLONE_DRY_RUN", "RCLONE_INTERACTIVE"):
            for value in ("yes", "false ", "SYNTHETIC-PRIVATE-SETTING-CANARY"):
                with self.subTest(setting=setting, value=value):
                    result = self.invoke(**{setting: value})
                    self.assert_stopped_before_retention(result, before_archives=True)
                    self.assertIn(f"{setting} must be unset or false", self.output)
                    self.assertNotIn(f"{setting}={value}", self.output)
                    self.assertNotIn("SYNTHETIC-PRIVATE-SETTING-CANARY", self.output)

    def test_false_remote_execution_settings_keep_normal_uploads_and_retention(self):
        for mode in ("false", "true"):
            for value in ("", "false", "False", "FALSE", "0", "f", "F"):
                with self.subTest(encrypted=mode, value=value):
                    self.check_retention_scope(PULSE_BACKUP_ENCRYPT=mode,
                                               RCLONE_DRY_RUN=value, RCLONE_INTERACTIVE=value)
                    self.assertEqual(sum(call[:2] == ["rclone", "copyto"] for call in self.calls), 3)

    def test_rclone_execution_settings_do_not_change_local_only_jobs(self):
        for mode in ("false", "true"):
            with self.subTest(encrypted=mode):
                result = self.invoke(PULSE_RCLONE_REMOTE="", PULSE_BACKUP_ENCRYPT=mode,
                                     RCLONE_DRY_RUN="true", RCLONE_INTERACTIVE="true")
                self.assertEqual(result.returncode, 0, self.output)
                self.assert_no_uploads()
                self.assertTrue(all(not archive.exists() for archive in self.old))

    @unittest.skipUnless(REAL_RCLONE, "real rclone is not installed")
    def test_real_rclone_dry_run_success_is_not_an_upload(self):
        source = self.root / "dry-run-source.txt"
        source.write_bytes(b"synthetic dry-run copy fixture")
        target = self.remote / "dry-run-target.txt"
        env = dict(self.env, RCLONE_DRY_RUN="true")
        command = [REAL_RCLONE, "--config", os.devnull, "copyto", str(source), str(target)]
        dry = subprocess.run(command, env=env, capture_output=True, text=True, timeout=10)
        self.assertEqual(dry.returncode, 0, dry.stdout + dry.stderr)
        self.assertFalse(target.exists(), "dry run unexpectedly uploaded fixture bytes")
        actual = subprocess.run(command, env=dict(env, RCLONE_DRY_RUN="false"),
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(actual.returncode, 0, actual.stdout + actual.stderr)
        self.assertEqual(target.read_bytes(), source.read_bytes())

    def test_missing_tar_or_retention_tool_stops_before_archiving(self):
        for name in ("tar", "find", "rmdir", "mv"):
            with self.subTest(name=name):
                content = (self.bin / name).read_bytes()
                (self.bin / name).unlink()
                self.assert_stopped_before_retention(self.invoke(), before_archives=True)
                (self.bin / name).write_bytes(content)
                (self.bin / name).chmod(0o700)

    def test_failed_producers_never_publish_ordinary_backup_names(self):
        for mode in ("false", "true"):
            scenarios = [{"tar_fail": number} for number in (1, 2)]
            if mode == "true":
                scenarios += [{"age_fail": number} for number in (1, 2, 3)]
                scenarios += [{"age_reject": number} for number in (1, 2, 3)]
            for scenario in scenarios:
                with self.subTest(mode=mode, scenario=scenario):
                    result = self.invoke(scenario, PULSE_BACKUP_ENCRYPT=mode)
                    self.assert_stopped_before_retention(result)
                    self.assert_no_uploads()
                    self.assert_no_new_completed_files()
                    self.assertFalse(any(call[0] == "mv" for call in self.calls), self.calls)
                    partial = list(self.backups.glob(f"*{self.timestamp}*.partial"))
                    for path in partial:
                        self.assertEqual(path.stat().st_mode & 0o777, 0o600)
                    if "tar_fail" in scenario or "age_fail" in scenario:
                        self.assertTrue(partial, self.output)

    def test_native_tar_failure_retains_no_ordinary_backup_names(self):
        for mode in ("false", "true"):
            for number in (1, 2):
                with self.subTest(mode=mode, number=number):
                    # Run native tar after its admitted source disappears, not
                    # a fake exit code: gzip may still finish a readable archive.
                    source = self.root / ("tenants" if number == 1 else "control-plane")
                    try:
                        result = self.invoke({"native_tar_missing": number}, PULSE_BACKUP_ENCRYPT=mode)
                        self.assert_stopped_before_retention(result)
                        self.assert_no_uploads()
                        self.assert_no_new_completed_files()
                        partials = list(self.backups.glob(f"*{self.timestamp}*.partial"))
                        self.assertEqual(len(partials), number)
                        self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in partials))
                        self.assertTrue(any(p.stat().st_size > 0 for p in partials))
                    finally:
                        source.with_name(source.name + "-moved-before-tar").rename(source)

    def test_partial_evidence_expires_only_locally_after_a_successful_job(self):
        old_partials, preserved = [], {}
        for label, suffix in OWNED_TYPES:
            for encrypted in ("", ".age"):
                filename = f"pulse-backup-{label}-20000101-000000{suffix}{encrypted}.partial"
                local = self.backups / filename
                local.write_bytes(b"old incomplete fixture")
                os.utime(local, (946684800, 946684800))
                old_partials.append(local)
                # Partial names are never uploaded: they are not remotely owned.
                remote = self.remote / filename
                remote.write_bytes(b"unrelated remote fixture")
                os.utime(remote, (946684800, 946684800))
                preserved[remote] = remote.read_bytes()
                young = self.backups / filename.replace("20000101", "20990101")
                young.write_bytes(b"young incomplete fixture")
                os.utime(young, (4102444800, 4102444800))
                preserved[young] = young.read_bytes()
        link = self.backups / "pulse-backup-meta-19990101-000000.txt.partial"
        link.symlink_to(self.root / "absent-partial-target")
        os.utime(link, (946684800, 946684800), follow_symlinks=False)
        directory = self.backups / "pulse-backup-cp-19990101-000000.tar.gz.age.partial"
        directory.mkdir()
        (directory / "preserved.txt").write_bytes(b"preserved directory fixture")
        os.utime(directory, (946684800, 946684800))
        preserved[directory / "preserved.txt"] = b"preserved directory fixture"

        self.assert_stopped_before_retention(self.invoke({"tar_fail": 1}))
        self.assertTrue(all(p.exists() for p in old_partials))
        result = self.invoke()
        self.assertEqual(result.returncode, 0, self.output)
        self.assertTrue(all(not p.exists() for p in old_partials))
        self.assertEqual({p: p.read_bytes() for p in preserved}, preserved)
        self.assertTrue(link.is_symlink())
        self.assertEqual(list(directory.iterdir()), [directory / "preserved.txt"])

    def test_all_producers_finish_before_first_publication(self):
        for mode in ("false", "true"):
            with self.subTest(mode=mode):
                observations_file = self.root / "producer-observations.jsonl"
                observations_file.unlink(missing_ok=True)
                result = self.invoke(PULSE_BACKUP_ENCRYPT=mode)
                self.assertEqual(result.returncode, 0, self.output)
                observations = [json.loads(line) for line in observations_file.read_text().splitlines()]
                self.assertEqual(len(observations), 5 if mode == "true" else 2)
                self.assertTrue(all(not row["completed"] for row in observations), observations)
                publications = [index for index, call in enumerate(self.calls) if call[0] == "mv"]
                self.assertEqual(len(publications), 3, self.calls)
                last_producer = max(index for index, call in enumerate(self.calls)
                                    if call[0] in ("tar", "age", "docker"))
                first_upload = next(index for index, call in enumerate(self.calls)
                                    if call[:2] == ["rclone", "copyto"])
                self.assertLess(last_producer, publications[0])
                self.assertLess(publications[-1], first_upload)
                self.assertIn("pulse-backup-meta-", self.calls[publications[-1]][-1])
                self.assertEqual(list(self.backups.glob(f"*{self.timestamp}*.partial")), [])
                files = [p for p in self.backups.iterdir() if self.timestamp in p.name]
                self.assertEqual(len(files), 3)
                self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in files))

    def test_publication_errors_preserve_evidence_without_upload_or_retention(self):
        for mode in ("false", "true"):
            for number in (1, 2, 3):
                with self.subTest(mode=mode, number=number):
                    result = self.invoke({"publish_fail": number}, PULSE_BACKUP_ENCRYPT=mode)
                    self.assert_stopped_before_retention(result)
                    self.assert_no_uploads()
                    self.assertIn("Backup file publication failed", self.output)
                    files = [p for p in self.backups.iterdir() if self.timestamp in p.name]
                    self.assertEqual(len([p for p in files if p.name.endswith(".partial")]), 4 - number)
                    self.assertEqual(len([p for p in files if not p.name.endswith(".partial")]), number - 1)
                    self.assertTrue(all(p.stat().st_mode & 0o777 == 0o600 for p in files))
                    for path in files:
                        if path.name.endswith(".partial"):
                            continue
                        content = path.read_bytes()
                        if mode == "true":
                            self.assertTrue(content.startswith(b"FAKE-AGE\n"))
                            content = content.removeprefix(b"FAKE-AGE\n")
                        directory = "tenants" if "-tenants-" in path.name else "control-plane"
                        with tarfile.open(fileobj=io.BytesIO(content), mode="r:gz") as archive:
                            self.assertEqual(archive.extractfile(f"{directory}/fixture.txt").read(),
                                             f"synthetic {directory} data\n".encode())

    def test_late_publication_collisions_fail_closed_even_when_mv_exits_zero(self):
        for mode in ("false", "true"):
            for kind in ("file", "directory", "symlink", "dangling"):
                with self.subTest(mode=mode, kind=kind):
                    result = self.invoke({"publish_collision": 1, "collision_kind": kind},
                                         PULSE_BACKUP_ENCRYPT=mode)
                    self.assert_stopped_before_retention(result)
                    self.assert_no_uploads()
                    self.assertIn("Backup file publication failed", self.output)
                    partials = list(self.backups.glob(f"*{self.timestamp}*.partial"))
                    self.assertEqual(len(partials), 3)
                    destination = Path(next(call for call in self.calls if call[0] == "mv")[-1])
                    if kind == "directory":
                        self.assertEqual(list(destination.iterdir()), [destination / "preserved.txt"])
                        destination = destination / "preserved.txt"
                    elif kind in ("symlink", "dangling"):
                        self.assertTrue(destination.is_symlink())
                    if kind == "dangling":
                        self.assertFalse(destination.exists())
                    else:
                        self.assertEqual(destination.read_bytes(), b"late unrelated fixture")

    def test_existing_partial_files_directories_and_links_are_never_overwritten(self):
        for index, (label, suffix) in enumerate(OWNED_TYPES):
            for mode, encrypted in (("false", ""), ("false", ".age"), ("true", ""), ("true", ".age")):
                for kind in ("file", "directory", "symlink", "dangling"):
                    with self.subTest(label=label, mode=mode, encrypted=encrypted, kind=kind):
                        timestamp = f"20261022-09{self.invocations + 1:04d}"
                        existing = self.backups / f"pulse-backup-{label}-{timestamp}{suffix}{encrypted}.partial"
                        target = self.root / f"partial-target-{index}-{mode}-{kind}"
                        if kind == "directory":
                            existing.mkdir()
                            (existing / "preserved.txt").write_bytes(b"preserved partial fixture")
                        elif kind in ("symlink", "dangling"):
                            if kind == "symlink":
                                target.write_bytes(b"preserved partial fixture")
                            existing.symlink_to(target)
                        else:
                            existing.write_bytes(b"preserved partial fixture")
                        result = self.invoke(PULSE_BACKUP_ENCRYPT=mode, FIXTURE_TIMESTAMP=timestamp)
                        self.assert_stopped_before_retention(result)
                        self.assertEqual(self.calls, [])
                        self.assertIn("already exists", self.output)
                        if kind == "directory":
                            self.assertEqual(list(existing.iterdir()), [existing / "preserved.txt"])
                            self.assertEqual((existing / "preserved.txt").read_bytes(), b"preserved partial fixture")
                        elif kind == "dangling":
                            self.assertTrue(existing.is_symlink())
                            self.assertFalse(target.exists())
                        else:
                            self.assertEqual(existing.read_bytes(), b"preserved partial fixture")

    def test_archive_failures_preserve_last_archives_and_skip_uploads(self):
        for number in (1, 2):
            with self.subTest(number=number):
                self.assert_stopped_before_retention(self.invoke({"tar_fail": number}))
                self.assert_no_uploads()

    def test_encryption_failures_preserve_last_archives_and_skip_uploads(self):
        for number in (1, 2, 3):
            with self.subTest(number=number):
                self.assert_stopped_before_retention(self.invoke({"age_fail": number}, PULSE_BACKUP_ENCRYPT="true"))
                self.assert_no_uploads()

    def assert_no_new_plaintext(self):
        self.assertEqual([path.name for path in self.backups.iterdir()
                          if path not in self.old and path.name.endswith(
                              (".tar.gz", ".txt", ".tar.gz.partial", ".txt.partial"))], [])

    def test_encryption_never_stages_plaintext_even_while_age_reads(self):
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true")
        self.assertEqual(result.returncode, 0, self.output)
        observations = [json.loads(line) for line in
                        (self.root / "plaintext-observations.jsonl").read_text().splitlines()]
        self.assertEqual({(item["number"], item["phase"]) for item in observations},
                         {(number, phase) for number in (1, 2, 3) for phase in ("before-input", "after-input")})
        self.assertTrue(all(not item["names"] for item in observations), observations)
        self.assert_no_new_plaintext()
        self.assertTrue(all(len(call) == 5 for call in self.calls if call[0] == "age"), self.calls)

    def test_encryption_failures_never_leave_new_plaintext(self):
        for number in (1, 2, 3):
            with self.subTest(number=number):
                result = self.invoke({"age_fail": number}, PULSE_BACKUP_ENCRYPT="true")
                self.assert_stopped_before_retention(result)
                self.assert_no_uploads()
                self.assert_no_new_plaintext()

    def test_encryption_rejects_input_before_reading_without_plaintext_fallback(self):
        for number in (1, 2, 3):
            with self.subTest(number=number):
                result = self.invoke({"age_reject": number}, PULSE_BACKUP_ENCRYPT="true")
                self.assert_stopped_before_retention(result)
                self.assert_no_uploads()
                self.assert_no_new_plaintext()

    def test_failed_tar_stream_is_not_an_encrypted_success(self):
        for number in (1, 2):
            with self.subTest(number=number):
                result = self.invoke({"tar_fail": number}, PULSE_BACKUP_ENCRYPT="true")
                self.assert_stopped_before_retention(result)
                self.assert_no_uploads()
                self.assert_no_new_plaintext()
                # The fake encryptor succeeds with the partial producer output.
                # The job must still fail through Bash's pipefail.
                self.assertIn(b"FAKE-AGE\nincomplete fixture archive",
                              [path.read_bytes() for path in self.backups.glob("*.age.partial")])

    def test_each_upload_must_succeed_before_either_retention(self):
        for mode in ("false", "true"):
            for number in (1, 2, 3):
                with self.subTest(mode=mode, number=number):
                    self.assert_stopped_before_retention(
                        self.invoke({"upload_fail": number}, PULSE_BACKUP_ENCRYPT=mode))
                    copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
                    self.assertEqual(len(copies), number)
                    if mode == "true":
                        self.assertTrue(all(call[2].endswith(".age") for call in copies))

    def test_local_retention_error_is_not_a_success_and_stops_remote_retention(self):
        result = self.invoke({"local_retention_fail": True})
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertIn("Local retention failed", self.output)
        self.assertNotIn("=== tarball backup finished ===", self.output)
        self.assertFalse(any(call[:2] == ["rclone", "delete"] for call in self.calls))
        self.assert_archive_contents()

    def test_remote_retention_error_is_not_a_success(self):
        result = self.invoke({"remote_retention_fail": True})
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertIn("Remote retention failed", self.output)
        self.assertNotIn("=== tarball backup finished ===", self.output)
        self.assertEqual(len([call for call in self.calls if call[:2] == ["rclone", "copyto"]]), 3)
        self.assert_archive_contents()

    def test_local_only_success_requires_both_real_archives(self):
        result = self.invoke(PULSE_RCLONE_REMOTE="")
        self.assertEqual(result.returncode, 0, self.output)
        self.assertIn("=== tarball backup finished ===", self.output)
        self.assert_archive_contents()
        self.assert_no_uploads()
        self.assertFalse(any(path.exists() for path in self.old))
        metadata = list(self.backups.glob("pulse-backup-meta-*.txt"))
        self.assertEqual(len(metadata), 1)
        self.assertIn(METADATA_MARKER, metadata[0].read_bytes())
        self.assertEqual(metadata[0].stat().st_mode & 0o777, 0o600)
        self.assertEqual(list(self.backups.glob("*.age")), [])

    def test_upload_and_rotation_follow_both_archives(self):
        result = self.invoke(PULSE_BACKUP_RETENTION="0030")
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_archive_contents()
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertEqual(len(copies), 3)
        self.assertTrue(all(call[4:] == ["--error-on-no-transfer"] for call in copies), copies)
        last_upload = max(self.calls.index(call) for call in copies)
        local_retention = next(index for index, call in enumerate(self.calls) if call[0] == "find")
        remote_retention = next(index for index, call in enumerate(self.calls) if call[:2] == ["rclone", "delete"])
        self.assertLess(last_upload, local_retention)
        self.assertLess(local_retention, remote_retention)
        self.assertFalse(any(path.exists() for path in self.old))
        self.assertEqual(self.backups.stat().st_mode & 0o777, 0o700)

    def test_encrypted_mode_uploads_encrypted_names_not_plaintext(self):
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true")
        self.assertEqual(result.returncode, 0, self.output)
        self.assertEqual(len(list(self.backups.glob("*.tar.gz.age"))), 2)
        self.assertEqual(list(self.backups.glob("*.tar.gz")), [])
        for archive in self.backups.glob("*.tar.gz.age"):
            self.assertTrue(archive.read_bytes().startswith(b"FAKE-AGE\n"))
            self.assertEqual(archive.stat().st_mode & 0o777, 0o600)
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertEqual(len(copies), 3)
        self.assertTrue(all(call[4:] == ["--error-on-no-transfer"] for call in copies), copies)
        self.assertTrue(all(call[1] == "copyto" and call[2].endswith(".age") for call in copies[:2]))
        self.assertTrue(all(call[2].endswith(".age") for call in copies), copies)
        metadata = list(self.backups.glob("pulse-backup-meta-*.txt.age"))
        self.assertEqual(len(metadata), 1)
        self.assertEqual(list(self.backups.glob("*.txt")), [])
        self.assertEqual(metadata[0].stat().st_mode & 0o777, 0o600)
        self.assertTrue(metadata[0].read_bytes().startswith(b"FAKE-AGE\n"))
        self.assertIn(b"hostname: fixture-host", metadata[0].read_bytes())
        self.assertNotIn(b"SYNTHETIC-COMMAND-CANARY", metadata[0].read_bytes())
        self.assertEqual(len([call for call in self.calls if call[0] == "age"]), 3)
        encryptions = [call for call in self.calls if call[0] == "age"]
        self.assertLess(max(self.calls.index(call) for call in encryptions), self.calls.index(copies[0]))

    def test_metadata_contains_only_selected_inventory_not_container_commands(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, self.output)
        metadata = next(self.backups.glob("pulse-backup-meta-*.txt"))
        text = metadata.read_text()
        self.assertIn("hostname: fixture-host", text)
        self.assertIn("encrypted: false", text)
        self.assertIn("fixture-id fixture-image fixture-container Up 1 minute", text)
        self.assertNotIn("SYNTHETIC-COMMAND-CANARY", text)
        self.assertNotIn("COMMAND", text)
        self.assertEqual(metadata.stat().st_mode & 0o777, 0o600)

    def test_local_only_encryption_also_protects_metadata(self):
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", PULSE_RCLONE_REMOTE="")
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_no_uploads()
        metadata = list(self.backups.glob("pulse-backup-meta-*.txt.age"))
        self.assertEqual(len(metadata), 1)
        self.assertTrue(metadata[0].read_bytes().startswith(b"FAKE-AGE\n"))
        self.assertEqual(list(self.backups.glob("*.txt")), [])

    def test_missing_optional_docker_inventory_does_not_skip_metadata_encryption(self):
        result = self.invoke({"docker_fail": True}, PULSE_BACKUP_ENCRYPT="true")
        self.assertEqual(result.returncode, 0, self.output)
        metadata = next(self.backups.glob("pulse-backup-meta-*.txt.age"))
        self.assertIn(b"docker ps: unavailable", metadata.read_bytes())
        self.assertEqual(list(self.backups.glob("*.txt")), [])

    def test_metadata_encryption_failure_stops_before_every_upload_and_retention(self):
        result = self.invoke({"age_fail": 3}, PULSE_BACKUP_ENCRYPT="true")
        self.assert_stopped_before_retention(result)
        self.assert_no_uploads()
        self.assertEqual(len([call for call in self.calls if call[0] == "age"]), 3)
        # Only the partial encrypted stream is retained, never a new plaintext
        # inventory. A failed job does not claim that stream is a usable backup.
        self.assert_no_new_plaintext()
        metadata = next(self.backups.glob("pulse-backup-meta-*.txt.age.partial"))
        self.assertEqual(metadata.stat().st_mode & 0o777, 0o600)

    def test_each_encrypted_upload_preserves_prior_backups_on_failure(self):
        for number in (1, 2, 3):
            with self.subTest(number=number):
                result = self.invoke({"upload_fail": number}, PULSE_BACKUP_ENCRYPT="true")
                self.assert_stopped_before_retention(result)
                copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
                self.assertEqual(len(copies), number)
                self.assertTrue(all(call[2].endswith(".age") for call in copies), copies)
                self.assertEqual([path for path in self.backups.glob("*.txt") if path not in self.old], [])

    def test_existing_encrypted_metadata_directory_is_preserved_before_work(self):
        timestamp = "20261022-070001"
        existing = self.backups / f"pulse-backup-meta-{timestamp}.txt.age"
        existing.mkdir()
        marker = existing / "preserved.txt"
        marker.write_bytes(b"synthetic retained directory")
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", FIXTURE_TIMESTAMP=timestamp)
        self.assert_stopped_before_retention(result)
        self.assertEqual(marker.read_bytes(), b"synthetic retained directory")
        self.assertEqual(self.calls, [])

    def test_encrypted_metadata_dangling_link_is_preserved_before_work(self):
        timestamp = "20261022-080001"
        target = self.backups / "must-not-be-created.txt"
        existing = self.backups / f"pulse-backup-meta-{timestamp}.txt.age"
        existing.symlink_to(target)
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", FIXTURE_TIMESTAMP=timestamp)
        self.assert_stopped_before_retention(result)
        self.assertTrue(existing.is_symlink())
        self.assertFalse(target.exists())
        self.assertEqual(self.calls, [])


    def test_local_only_encryption_also_covers_metadata(self):
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", PULSE_RCLONE_REMOTE="")
        self.assertEqual(result.returncode, 0, self.output)
        self.assert_no_uploads()
        self.assertEqual(len(list(self.backups.glob("*.age"))), 3)
        self.assertEqual(list(self.backups.glob("*.txt")), [])
        self.assertEqual(list(self.backups.glob("*.tar.gz")), [])


    def test_metadata_encryption_failure_preserves_private_evidence_without_upload(self):
        result = self.invoke({"age_fail": 3}, PULSE_BACKUP_ENCRYPT="true")
        self.assert_stopped_before_retention(result)
        self.assert_no_uploads()
        # The failed run is not an encrypted success. Keep partial encrypted
        # files private for diagnosis; do not retain new plaintext inputs.
        # The expanded fixture includes an older plaintext inventory, which
        # assert_stopped_before_retention already checks is preserved intact.
        metadata = [path for path in self.backups.glob("*.txt") if path not in self.old]
        self.assertEqual(len(metadata), 0)
        self.assert_no_new_plaintext()
        for path in self.backups.glob("pulse-backup-*-20261022-*"):
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.backups.stat().st_mode & 0o777, 0o700)


    def test_existing_encrypted_metadata_is_reserved_in_both_modes(self):
        for mode, timestamp in (("false", "20261022-070001"), ("true", "20261022-070002")):
            with self.subTest(mode=mode):
                existing = self.backups / f"pulse-backup-meta-{timestamp}.txt.age"
                existing.write_bytes(b"retained encrypted inventory fixture")
                result = self.invoke(PULSE_BACKUP_ENCRYPT=mode, FIXTURE_TIMESTAMP=timestamp)
                self.assert_stopped_before_retention(result)
                self.assertEqual(self.calls, [])
                self.assertEqual(set(self.backups.iterdir()), {*self.old, existing})
                self.assertEqual(existing.read_bytes(), b"retained encrypted inventory fixture")
                existing.unlink()


    def test_same_second_retry_preserves_all_encrypted_outputs(self):
        timestamp = "20261022-080001"
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", FIXTURE_TIMESTAMP=timestamp)
        self.assertEqual(result.returncode, 0, self.output)
        completed = {p: p.read_bytes() for p in self.backups.iterdir()}
        self.assertEqual(len(completed), 3)
        self.assertTrue(all(p.name.endswith(".age") for p in completed))
        result = self.invoke({"age_fail": 3}, PULSE_BACKUP_ENCRYPT="true", FIXTURE_TIMESTAMP=timestamp)
        self.assertNotEqual(result.returncode, 0, self.output)
        self.assertEqual(self.calls, [])
        self.assertEqual({p: p.read_bytes() for p in self.backups.iterdir()}, completed)


    @unittest.skipUnless(REAL_AGE and REAL_AGE_KEYGEN, "real age and age-keygen are not supplied or installed")
    def test_real_age_recovers_archives_and_private_metadata(self):
        identity = self.root / "fixture-age-identity"
        # Keys exist only in this disposable fixture. Never print keygen's
        # private output or include identity bytes in assertion diagnostics.
        keygen = subprocess.run([REAL_AGE_KEYGEN, "-o", str(identity)], capture_output=True, timeout=5)
        self.assertEqual(keygen.returncode, 0, "fixture age identity creation failed")
        identity.chmod(0o600)
        public = subprocess.run([REAL_AGE_KEYGEN, "-y", str(identity)], capture_output=True, timeout=5)
        self.assertEqual(public.returncode, 0, "fixture age recipient derivation failed")
        (self.bin / "age").unlink()
        (self.bin / "age").symlink_to(REAL_AGE)
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", PULSE_BACKUP_AGE_RECIPIENT=public.stdout.decode().strip(),
                             TAR_OPTIONS="--exclude=*")
        self.assertEqual(result.returncode, 0, self.output)
        self.assertEqual(list(self.backups.glob("*.txt")), [])
        self.assertEqual(list(self.backups.glob("*.tar.gz")), [])
        encrypted = list(self.backups.glob("*.age"))
        self.assertEqual(len(encrypted), 3)
        for path in encrypted:
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertNotIn(METADATA_MARKER, path.read_bytes())
            decrypted = subprocess.run([REAL_AGE, "--decrypt", "-i", str(identity), str(path)],
                                       capture_output=True, timeout=5)
            self.assertEqual(decrypted.returncode, 0, "fixture age decryption failed")
            if path.name.endswith(".txt.age"):
                self.assertIn(METADATA_MARKER, decrypted.stdout)
                self.assertIn(b"encrypted: true\n", decrypted.stdout)
            else:
                directory = "tenants" if "-tenants-" in path.name else "control-plane"
                with tarfile.open(fileobj=io.BytesIO(decrypted.stdout), mode="r:gz") as archive:
                    self.assertEqual(archive.extractfile(f"{directory}/fixture.txt").read(),
                                     f"synthetic {directory} data\n".encode())
        copies = [call for call in self.calls if call[:2] == ["rclone", "copyto"]]
        self.assertEqual(len(copies), 3)
        self.assertTrue(all(call[2].endswith(".age") for call in copies))

    @unittest.skipUnless(REAL_AGE, "real age is not supplied or installed")
    def test_real_age_invalid_recipient_leaves_no_plaintext(self):
        (self.bin / "age").unlink()
        (self.bin / "age").symlink_to(REAL_AGE)
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", PULSE_BACKUP_AGE_RECIPIENT="invalid-fixture-recipient")
        self.assert_stopped_before_retention(result)
        self.assert_no_uploads()
        self.assert_no_new_plaintext()

    @unittest.skipUnless(REAL_AGE and REAL_AGE_KEYGEN, "real age and age-keygen are not supplied or installed")
    def test_real_age_keeps_failed_native_tar_output_partial(self):
        identity = self.root / "fixture-failed-tar-age-identity"
        keygen = subprocess.run([REAL_AGE_KEYGEN, "-o", str(identity)], capture_output=True, timeout=5)
        self.assertEqual(keygen.returncode, 0, "fixture age identity creation failed")
        identity.chmod(0o600)
        public = subprocess.run([REAL_AGE_KEYGEN, "-y", str(identity)], capture_output=True, timeout=5)
        self.assertEqual(public.returncode, 0, "fixture age recipient derivation failed")
        (self.bin / "age").unlink()
        (self.bin / "age").symlink_to(REAL_AGE)
        result = self.invoke({"native_tar_missing": 1}, PULSE_BACKUP_ENCRYPT="true",
                             PULSE_BACKUP_AGE_RECIPIENT=public.stdout.decode().strip())
        self.assert_stopped_before_retention(result)
        self.assert_no_uploads()
        self.assert_no_new_plaintext()
        self.assert_no_new_completed_files()
        partials = list(self.backups.glob(f"*{self.timestamp}*.age.partial"))
        self.assertEqual(len(partials), 1)
        self.assertEqual(partials[0].stat().st_mode & 0o777, 0o600)
        decrypted = subprocess.run([REAL_AGE, "--decrypt", "-i", str(identity), str(partials[0])],
                                   capture_output=True, timeout=5)
        self.assertEqual(decrypted.returncode, 0, "fixture partial age decryption failed")
        with tarfile.open(fileobj=io.BytesIO(decrypted.stdout), mode="r:gz") as archive:
            self.assertEqual(archive.getnames(), [])


def run_native_suite():
    """CI must execute the full suite, including the required native clients."""
    tools = (("age", REAL_AGE, "--version"),
             ("age-keygen", REAL_AGE_KEYGEN, "--version"),
             ("rclone", REAL_RCLONE, "version"))
    for name, executable, option in tools:
        if not executable or not Path(executable).is_file() or not os.access(executable, os.X_OK):
            print(f"Native backup proof requires executable {name}; no tests started", file=sys.stderr)
            return 2
    for name, executable, option in tools:
        try:
            version = subprocess.run([executable, option], capture_output=True, text=True, timeout=10)
            version_text = (version.stdout + version.stderr).strip()
            if version.returncode != 0 or not version_text or len(version_text) > 4096:
                raise ValueError("version command failed")
            print(json.dumps({"native_tool": name, "version": version_text,
                              "sha256": hashlib.sha256(Path(executable).read_bytes()).hexdigest()}), flush=True)
        except (OSError, ValueError, subprocess.TimeoutExpired):
            print(f"Native backup proof cannot identify {name}; no tests started", file=sys.stderr)
            return 2
    for path in (SCRIPT, Path(__file__)):
        print(json.dumps({"source_file": path.name,
                          "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}), flush=True)
    loader = unittest.TestLoader()
    required = {"test_real_age_recovers_archives_and_private_metadata",
                "test_real_rclone_local_backend_preserves_unowned_objects",
                "test_real_rclone_dry_run_success_is_not_an_upload",
                "test_real_rclone_skipped_existing_destination_cannot_authorize_retention"}
    if not required.issubset(loader.getTestCaseNames(CloudBackupTest)):
        print("Native backup cases are missing; no tests started", file=sys.stderr)
        return 2
    suite = loader.loadTestsFromTestCase(CloudBackupTest)
    expected = suite.countTestCases()
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    complete = result.wasSuccessful() and not result.skipped and result.testsRun == expected
    print(json.dumps({"native_backup_summary": {"expected": expected, "ran": result.testsRun,
                      "failures": len(result.failures), "errors": len(result.errors),
                      "skipped": len(result.skipped), "complete": complete}}), flush=True)
    return 0 if complete else 1


if __name__ == "__main__":
    if sys.argv[1:] == ["--require-native"]:
        sys.exit(run_native_suite())
    unittest.main()
