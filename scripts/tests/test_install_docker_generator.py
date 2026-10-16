#!/usr/bin/env python3
"""Run the actual Docker configuration generator without root or a daemon.

Only the root/container eligibility checks are disabled in a private copy.
Docker, timezone and hostname are closed fixtures; no containers are started.
"""

import os
from pathlib import Path
import shlex
import shutil
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get(
    "PULSE_DOCKER_INSTALLER_UNDER_TEST", ROOT / "scripts/install-docker.sh"))
COMPOSE_NAMES = (
    "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml",
    "docker-compose.override.yml", "docker-compose.override.yaml",
    "compose.override.yml", "compose.override.yaml",
)


class DockerGeneratorSafety(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="pulse-compose-fixture-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.work = self.root / "deployment"
        self.work.mkdir()
        tools = self.root / "tools"
        tools.mkdir()
        self.tools = tools
        source = INSTALLER.read_text()
        # Assert both confinement edits still target the actual guards. All
        # configuration generation, admission, diagnostics and exits stay real.
        for condition in (
            'if [ "$EUID" -ne 0 ]; then',
            'if [ -f /.dockerenv ] || [ -f /run/.containerenv ]; then',
        ):
            self.assertEqual(source.count(condition), 1)
            source = source.replace(condition, "if false; then", 1)
        self.script = self.root / "install-docker.sh"
        self.script.write_text(source)
        commands = {
            "docker": """#!/bin/sh
printf '%s\\n' "$*" >> "$FIXTURE_DOCKER_CALLS"
[ "$#" = 2 ] && [ "$1" = compose ] && [ "$2" = version ]
""",
            "timedatectl": "#!/bin/sh\nprintf '%s\\n' Europe/London\n",
            "hostname": "#!/bin/sh\nprintf '%s\\n' 192.0.2.10\n",
        }
        for name, content in commands.items():
            path = tools / name
            path.write_text(content)
            path.chmod(0o700)
        self.environment = {
            "PATH": str(tools) + os.pathsep + os.environ["PATH"],
            "HOME": str(self.root),
            "LC_ALL": "C",
            "PULSE_IMAGE_VERSION": "6.6.0",
            "FIXTURE_DOCKER_CALLS": str(self.root / "docker-calls"),
        }

    def run_generator(self, **environment):
        result = subprocess.run(
            ["bash", str(self.script)], cwd=self.work,
            env={**self.environment, **environment}, capture_output=True,
            text=True, timeout=10, umask=0o000,
        )
        # Only capability discovery is admitted. This fixture cannot accept a
        # claimed Docker install, pull, start or recovery result.
        calls = (self.root / "docker-calls").read_text().splitlines()
        self.assertTrue(calls)
        self.assertEqual(set(calls), {"compose version"})
        return result

    def assert_refused(self, result):
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("no files changed", result.stderr)
        self.assertNotIn("Configuration generated", result.stdout)
        self.assertNotIn("Installation Complete", result.stdout)

    def test_existing_compose_and_backup_survive_repeated_invocation(self):
        compose = self.work / "docker-compose.yml"
        backup = self.work / "docker-compose.yml.backup"
        compose.write_text("services:\n  pulse:\n    volumes: [operator-data:/data]\n")
        backup.write_text("original operator recovery copy\n")
        expected = (compose.read_bytes(), backup.read_bytes())
        for _ in range(2):
            self.assert_refused(self.run_generator())
            self.assertEqual((compose.read_bytes(), backup.read_bytes()), expected)
            self.assertFalse((self.work / ".env").exists())

    def test_all_default_compose_and_override_names_stop_generation(self):
        for name in COMPOSE_NAMES:
            with self.subTest(name=name):
                path = self.work / name
                path.write_text("existing operator configuration\n")
                try:
                    self.assert_refused(self.run_generator())
                    self.assertEqual(path.read_text(), "existing operator configuration\n")
                    self.assertEqual(set(p.name for p in self.work.iterdir()), {name})
                finally:
                    path.unlink()

    def test_existing_compose_links_are_not_followed(self):
        for dangling in (False, True):
            with self.subTest(dangling=dangling):
                target = self.root / ("missing" if dangling else "saved-compose")
                if not dangling:
                    target.write_text("original linked deployment\n")
                link = self.work / "docker-compose.yml"
                link.symlink_to(target)
                try:
                    self.assert_refused(self.run_generator())
                    self.assertTrue(link.is_symlink())
                    self.assertEqual(link.readlink(), target)
                    if dangling:
                        self.assertFalse(target.exists())
                    else:
                        self.assertEqual(target.read_text(), "original linked deployment\n")
                    self.assertFalse((self.work / ".env").exists())
                finally:
                    link.unlink()

    def test_unsafe_env_paths_stop_before_compose_is_created(self):
        path = self.work / ".env"
        for kind in ("directory", "fifo", "symlink", "dangling symlink"):
            with self.subTest(kind=kind):
                target = self.root / ("missing-env" if kind == "dangling symlink" else "saved-env")
                if kind == "directory":
                    path.mkdir()
                elif kind == "fifo":
                    os.mkfifo(path)
                else:
                    if kind == "symlink":
                        target.write_text("PULSE_IMAGE=operator/pulse:retained\n")
                    path.symlink_to(target)
                try:
                    self.assert_refused(self.run_generator())
                    self.assertFalse((self.work / "docker-compose.yml").exists())
                    if kind == "symlink":
                        self.assertEqual(target.read_text(), "PULSE_IMAGE=operator/pulse:retained\n")
                    elif kind == "dangling symlink":
                        self.assertFalse(target.exists())
                finally:
                    if kind == "directory":
                        path.rmdir()
                    else:
                        path.unlink()

    def test_existing_compose_directory_is_not_replaced(self):
        path = self.work / "docker-compose.yml"
        path.mkdir()
        (path / "operator-data").write_text("keep\n")
        self.assert_refused(self.run_generator())
        self.assertEqual((path / "operator-data").read_text(), "keep\n")
        self.assertFalse((self.work / ".env").exists())

    def test_fresh_generation_preserves_defaults_and_explicit_image(self):
        result = self.run_generator(PULSE_IMAGE="example/pulse:retained", PULSE_PORT="8765")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("Configuration generated", result.stdout)
        self.assertNotIn("Installation Complete", result.stdout)
        self.assertIn("image: ${PULSE_IMAGE:-rcourtman/pulse:6.6.0}",
                      (self.work / "docker-compose.yml").read_text())
        self.assertEqual((self.work / ".env").read_text(),
                         "PULSE_IMAGE=example/pulse:retained\nPULSE_PORT=8765\nTZ=Europe/London\n")
        self.assertIn("docker compose up -d", result.stdout)
        self.assertFalse((self.work / "docker-compose.yml.backup").exists())
        expected = {p.name: p.read_bytes() for p in self.work.iterdir()}
        self.assert_refused(self.run_generator())
        self.assertEqual({p.name: p.read_bytes() for p in self.work.iterdir()}, expected)

    def test_existing_regular_env_is_preserved_byte_for_byte(self):
        path = self.work / ".env"
        content = b"# Operator settings\nPULSE_IMAGE=private/pulse:retained\nPULSE_PORT=8765\n"
        path.write_bytes(content)
        path.chmod(0o600)
        before = path.stat()
        result = self.run_generator()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(path.read_bytes(), content)
        self.assertEqual((path.stat().st_ino, path.stat().st_mode, path.stat().st_mtime_ns),
                         (before.st_ino, before.st_mode, before.st_mtime_ns))
        self.assertIn(".env file already exists - not overwriting", result.stdout)

    def test_failed_render_publishes_no_partial_configuration(self):
        real_cat = shlex.quote(shutil.which("cat"))
        cat = self.tools / "cat"
        cat.write_text(f'''#!/bin/bash
count=0
if [ -f "$FIXTURE_RENDER_COUNT" ]; then read -r count < "$FIXTURE_RENDER_COUNT"; fi
count=$((count + 1))
printf '%s\\n' "$count" > "$FIXTURE_RENDER_COUNT"
if [ "$count" = "$FIXTURE_FAIL_RENDER" ]; then
    printf '%s\\n' 'incomplete configuration'
    exit 44
fi
exec {real_cat} "$@"
''')
        cat.chmod(0o700)
        for index, name in ((1, "compose"), (2, "environment")):
            with self.subTest(render=name):
                self.work = self.root / ("failed-" + name)
                self.work.mkdir()
                result = self.run_generator(
                    FIXTURE_RENDER_COUNT=str(self.root / ("render-count-" + name)),
                    FIXTURE_FAIL_RENDER=str(index),
                )
                self.assertEqual(result.returncode, 44, result.stdout + result.stderr)
                self.assertNotIn("Configuration generated", result.stdout)
                self.assertEqual(list(self.work.iterdir()), [],
                                 "failed rendering left a partial output or private staging input")

    def test_new_outputs_are_private_complete_files_without_staging_residue(self):
        result = self.run_generator()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual({p.name for p in self.work.iterdir()}, {"docker-compose.yml", ".env"})
        for name in ("docker-compose.yml", ".env"):
            with self.subTest(output=name):
                path = self.work / name
                self.assertTrue(stat.S_ISREG(path.lstat().st_mode))
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
                self.assertEqual(path.stat().st_nlink, 1)

    def test_refused_publication_never_copies_over_the_destination(self):
        real_link = shlex.quote(shutil.which("link"))
        link = self.tools / "link"
        link.write_text(f'''#!/bin/bash
if [ "$2" = "$FIXTURE_FAIL_LINK" ]; then exit 73; fi
exec {real_link} "$@"
''')
        link.chmod(0o700)
        for target, retained in ((".env", set()), ("./docker-compose.yml", {".env"})):
            with self.subTest(destination=target):
                self.work = self.root / ("link-" + str(len(retained)))
                self.work.mkdir()
                result = self.run_generator(FIXTURE_FAIL_LINK=target)
                self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                self.assertIn("Could not publish", result.stderr)
                self.assertNotIn("Configuration generated", result.stdout)
                self.assertEqual({p.name for p in self.work.iterdir()}, retained)
                if retained:
                    self.assertEqual((self.work / ".env").read_text(),
                                     "PULSE_IMAGE=rcourtman/pulse:6.6.0\nPULSE_PORT=7655\nTZ=Europe/London\n")
                    self.assertIn("complete .env already generated is retained", result.stderr)

    def test_output_appearing_after_preflight_is_not_truncated(self):
        hook = self.root / "before-output.sh"
        hook.write_text(r'''
fixture_before_output() {
    if [[ "$BASH_COMMAND" == "cat > \"\$$FIXTURE_OUTPUT_VARIABLE\""* ||
          "$BASH_COMMAND" == "link "*"\"\$$FIXTURE_OUTPUT_VARIABLE\"" ]]; then
        trap - DEBUG
        if [[ "$FIXTURE_RACE_KIND" == regular ]]; then
            printf '%s\n' 'concurrent operator configuration' > "$FIXTURE_RACE_PATH"
        elif [[ "$FIXTURE_RACE_KIND" == directory ]]; then
            mkdir "$FIXTURE_RACE_PATH"
        elif [[ "$FIXTURE_RACE_KIND" == fifo ]]; then
            mkfifo "$FIXTURE_RACE_PATH"
        else
            ln -s "$FIXTURE_RACE_TARGET" "$FIXTURE_RACE_PATH"
        fi
    fi
}
trap fixture_before_output DEBUG
''')
        for variable, name in (("COMPOSE_FILE", "docker-compose.yml"), ("ENV_FILE", ".env")):
            for kind in ("regular", "link", "dangling", "directory", "fifo"):
                with self.subTest(output=name, kind=kind):
                    self.work = self.root / (variable + "-" + kind)
                    self.work.mkdir()
                    path = self.work / name
                    target = self.root / (variable + "-" + kind + "-target")
                    if kind == "link":
                        target.write_text("saved linked operator configuration\n")
                    result = self.run_generator(
                        BASH_ENV=str(hook), FIXTURE_OUTPUT_VARIABLE=variable,
                        FIXTURE_RACE_PATH=str(path), FIXTURE_RACE_TARGET=str(target),
                        FIXTURE_RACE_KIND=kind,
                    )
                    self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
                    self.assertNotIn("Configuration generated", result.stdout)
                    if kind == "regular":
                        self.assertEqual(path.read_text(), "concurrent operator configuration\n")
                    elif kind == "directory":
                        self.assertTrue(path.is_dir())
                        self.assertEqual(list(path.iterdir()), [])
                    elif kind == "fifo":
                        self.assertTrue(stat.S_ISFIFO(path.lstat().st_mode))
                    else:
                        self.assertTrue(path.is_symlink())
                        if kind == "link":
                            self.assertEqual(target.read_text(), "saved linked operator configuration\n")
                        else:
                            self.assertFalse(target.exists())
                    self.assertFalse(list(self.work.glob(".pulse-docker.*")))


if __name__ == "__main__":
    unittest.main(verbosity=2)
