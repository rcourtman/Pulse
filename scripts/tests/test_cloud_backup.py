#!/usr/bin/env python3
"""Run the hosted tarball helper against disposable, non-database fixtures.

Real tar/gzip and local retention operate only on those fixtures. age, rclone,
Docker and host identity are closed recording fakes for orchestration controls.
One round-trip uses real age/age-keygen when supplied or installed; no test
establishes application consistency, cloud delivery or native restore readiness.
"""

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
METADATA_MARKER = b"synthetic-private-command-argument"

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
if tool != "date":
    with calls_file.open("a") as output:
        output.write(json.dumps([tool, *args]) + "\n")

if tool == "tar":
    assert len(args) == 5 and args[0] == "-czf" and args[2] == "-C", args
    output = Path(args[1]).resolve()
    source = Path(args[3]) / args[4]
    assert output.is_relative_to(root / "backups"), output
    assert source.resolve().is_relative_to(root), source
    assert source.name in ("tenants", "control-plane"), source
    number = 1 + sum(call[0] == tool for call in previous)
    if number == scenario.get("tar_fail"):
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
    result = subprocess.run([os.environ["REAL_TAR"], *args], check=False)
    if number == 1 and scenario.get("source_disappears"):
        (root / "control-plane").rename(root / "moved-control-plane")
    if number == 1 and scenario.get("source_becomes_symlink"):
        (root / "control-plane").rename(root / "moved-control-plane")
        (root / "control-plane").symlink_to(root / "moved-control-plane", target_is_directory=True)
    sys.exit(result.returncode)
if tool == "find":
    assert Path(args[0]).resolve() == root / "backups", args
    assert args[1:4] == ["-name", "pulse-backup-*.tar.gz*", "-mtime"], args
    assert len(args) == 6 and args[-1] == "-delete", args
    if scenario.get("local_retention_fail"):
        sys.exit(9)
    sys.exit(subprocess.run([os.environ["REAL_FIND"], *args], check=False).returncode)
if tool == "age":
    assert len(args) == 5 and args[:3] == ["-r", "fixture-recipient", "-o"], args
    output, source = Path(args[3]), Path(args[4])
    assert output.resolve().is_relative_to(root / "backups"), output
    assert source.resolve().is_relative_to(root / "backups"), source
    output.write_bytes(b"FAKE-AGE\n" + source.read_bytes())
    number = 1 + sum(call[0] == tool for call in previous)
    sys.exit(8 if number == scenario.get("age_fail") else 0)
if tool == "rclone":
    if args[0] == "copyto":
        assert len(args) == 3, args
        assert Path(args[1]).resolve().is_relative_to(root / "backups"), args
        assert Path(args[1]).is_file(), args
        assert args[2] == "fixture-remote:bucket/pulse-cloud-tarballs/" + Path(args[1]).name, args
        number = 1 + sum(call[:2] == [tool, "copyto"] for call in previous)
        sys.exit(10 if number == scenario.get("upload_fail") else 0)
    if args[0] == "delete":
        assert len(args) == 4 and args[:3] == ["delete", "fixture-remote:bucket/pulse-cloud-tarballs", "--min-age"], args
        sys.exit(11 if scenario.get("remote_retention_fail") else 0)
if tool == "date":
    assert args[0] == "-u" and len(args) == 2, args
    assert args[1] in ("+%Y%m%d-%H%M%S", "+%Y-%m-%dT%H:%M:%SZ"), args
    print(os.environ["FIXTURE_TIMESTAMP"] if args[1] == "+%Y%m%d-%H%M%S" else "2026-10-22T01:00:00Z")
    sys.exit(0)
if tool == "hostname" and not args:
    print("fixture-host")
    sys.exit(0)
if tool == "docker" and args == ["ps", "-a", "--no-trunc"]:
    print("CONTAINER ID  IMAGE  COMMAND  NAMES")
    print('fixture-id  fixture-image  "fixture-worker synthetic-private-command-argument"  fixture-tenant')
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
        self.log = self.root / "log/backup.log"
        self.old = []
        for name in ("tenants", "control-plane"):
            directory = self.root / name
            directory.mkdir()
            (directory / "fixture.txt").write_text(f"synthetic {name} data\n")
        for name in ("tenants", "cp"):
            archive = self.backups / f"pulse-backup-{name}-20000101-000000.tar.gz"
            archive.write_bytes(b"retained previous fixture")
            os.utime(archive, (946684800, 946684800))
            self.old.append(archive)
        # PATH admits only these local utilities and closed fakes. No real
        # Docker, encryption client or cloud client can be reached.
        for name in ("mkdir", "touch", "chmod", "dirname", "basename", "du", "cut", "rm", "rmdir", "gzip"):
            executable = shutil.which(name)
            self.assertIsNotNone(executable, f"required fixture dependency missing: {name}")
            (self.bin / name).symlink_to(executable)
        for name in ("tar", "find", "age", "rclone", "docker", "hostname", "date"):
            executable = self.bin / name
            executable.write_text(f"#!{sys.executable}\n" + FAKE)
            executable.chmod(0o700)
        self.env = {
            "PATH": str(self.bin), "FIXTURE": str(self.root), "HOME": str(self.root),
            "TMPDIR": str(self.root), "LC_ALL": "C", "REAL_TAR": shutil.which("tar"),
            "REAL_FIND": shutil.which("find"), "PULSE_DATA_DIR": str(self.root),
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
        result = subprocess.run([BASH, str(SCRIPT)], cwd=self.root,
                                env=dict(invocation_env, **env), text=True, capture_output=True, timeout=20)
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

    def test_missing_tar_or_retention_tool_stops_before_archiving(self):
        for name in ("tar", "find", "rmdir"):
            with self.subTest(name=name):
                content = (self.bin / name).read_bytes()
                (self.bin / name).unlink()
                self.assert_stopped_before_retention(self.invoke(), before_archives=True)
                (self.bin / name).write_bytes(content)
                (self.bin / name).chmod(0o700)

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
        self.assertTrue(all(call[2].endswith(".age") for call in copies))
        self.assertTrue(copies[2][2].endswith(".txt.age"))
        metadata = list(self.backups.glob("pulse-backup-meta-*.txt.age"))
        self.assertEqual(len(metadata), 1)
        self.assertTrue(metadata[0].read_bytes().startswith(b"FAKE-AGE\n"))
        self.assertEqual(metadata[0].stat().st_mode & 0o777, 0o600)
        self.assertEqual(list(self.backups.glob("*.txt")), [])
        encryptions = [call for call in self.calls if call[0] == "age"]
        self.assertEqual(len(encryptions), 3)
        self.assertLess(max(self.calls.index(call) for call in encryptions), self.calls.index(copies[0]))

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
        # The failed run is not an encrypted success. Keep its partial files
        # private for diagnosis, just as for an archive encryption failure.
        self.assertEqual(len(list(self.backups.glob("*.txt"))), 1)
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
        result = self.invoke(PULSE_BACKUP_ENCRYPT="true", PULSE_BACKUP_AGE_RECIPIENT=public.stdout.decode().strip())
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


if __name__ == "__main__":
    unittest.main()
