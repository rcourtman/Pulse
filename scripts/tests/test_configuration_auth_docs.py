#!/usr/bin/env python3
"""Exercise private-file auth recipes with synthetic credentials, not Docker.

The recorders verify file permissions, argument boundaries and failure stops.
They do not establish native container startup or a password-change outcome.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDES = ("CONFIGURATION.md", "INSTALL.md", "DOCKER.md", "i18n/de/README.md", "i18n/es/README.md")
HASH = "$2a$12$" + "a" * 53  # Synthetic file/argv sentinel, not a real password.
CONTENT = f"PULSE_AUTH_USER=admin\nPULSE_AUTH_PASS={HASH}\n"


def recipe(needle: str) -> str:
    text = (ROOT / "docs/CONFIGURATION.md").read_text(encoding="utf-8")
    matches = [block for block in re.findall(r"```bash\n(.*?)```", text, re.S)
               if needle in block]
    if len(matches) != 1:
        raise AssertionError(f"expected one configuration recipe containing {needle!r}")
    return matches[0]


EDITOR = '''#!/usr/bin/env python3
import json, os, stat, sys
from pathlib import Path
p = Path(sys.argv[1])
assert p == Path.home() / '.config/pulse/docker-auth.env'
assert stat.S_IMODE(p.stat().st_mode) == 0o600
assert stat.S_IMODE(p.parent.stat().st_mode) == 0o700
(Path.home() / 'editor.json').write_text(json.dumps(sys.argv[1:]))
sys.exit(int(os.environ.get('EDITOR_EXIT', '0')))
'''

DOCKER = '''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
p = Path(args[args.index('--env-file') + 1])
assert p == Path.home() / '.config/pulse/docker-auth.env'
assert p.read_text() == os.environ['EXPECTED_FILE']
assert os.environ['FIXTURE_HASH'] not in ' '.join(args)
assert not any(a.startswith('PULSE_AUTH_PASS=') for a in args)
(Path.home() / 'docker.json').write_text(json.dumps(args))
sys.exit(int(os.environ.get('DOCKER_EXIT', '0')))
'''


def environment(home: Path, **overrides: str) -> dict[str, str]:
    tools = home / "tools"
    tools.mkdir(exist_ok=True)
    for name, body in (("vi", EDITOR), ("docker", DOCKER)):
        script = tools / name
        script.write_text(body, encoding="utf-8")
        script.chmod(0o700)
    return {**os.environ, "HOME": str(home), "PATH": f"{tools}:{os.environ['PATH']}",
            "FIXTURE_HASH": HASH, "EXPECTED_FILE": CONTENT, **overrides}


def private_file(home: Path, content: str = CONTENT) -> Path:
    path = home / ".config/pulse/docker-auth.env"
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_text(content, encoding="utf-8")
    path.chmod(0o600)
    return path


class ConfigurationAuthDocsTest(unittest.TestCase):
    def test_no_shared_password_or_secret_in_command_arguments(self):
        for name in GUIDES:
            with self.subTest(guide=name):
                text = (ROOT / "docs" / name).read_text(encoding="utf-8")
                self.assertNotIn("secret123", text)
                shell = "\n".join(re.findall(r"```bash\n(.*?)```", text, re.S))
                self.assertNotRegex(shell, r"(?:-e|--env)\s+PULSE_AUTH_(?:PASS|USER)=")
                self.assertNotRegex(shell, r"\b(?:export\s+)?PULSE_AUTH_PASS=")
                if name != "CONFIGURATION.md":
                    self.assertNotRegex(text, r"PULSE_AUTH_PASS\s*[:=]")

    def test_mirrors_and_sensitive_runtime_limit(self):
        for name in GUIDES:
            self.assertEqual((ROOT / "docs" / name).read_bytes(),
                             (ROOT / "frontend-modern/public/docs" / name).read_bytes())
        text = (ROOT / "docs/CONFIGURATION.md").read_text(encoding="utf-8")
        for expected in ("not out of Docker's", "does not scrub its original value",
                         "Do not source the file", "Deployment-supplied environment values take precedence",
                         "complete bcrypt hash", "unquoted", "no `$$` substitution",
                         "this new-container example is not an upgrade procedure"):
            self.assertIn(expected, text)

    def test_install_quick_start_preserves_data_and_bootstrap_route(self):
        text = (ROOT / "docs/INSTALL.md").read_text(encoding="utf-8")
        compose = next(block for block in re.findall(r"```yaml\n(.*?)```", text, re.S)
                       if "services:" in block)
        for expected in ("image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}",
                         "pulse_data:/data", "PULSE_DEPLOYMENT_METHOD=docker_compose"):
            self.assertIn(expected, compose)
        self.assertNotRegex(compose, r"PULSE_AUTH_(?:USER|PASS)")
        for expected in ("[bootstrap-token setup](#step-1-get-the-token)",
                         "[authentication guide](CONFIGURATION.md#private-docker-authentication-file)",
                         "Leave authentication overrides unset for a new install",
                         "not a Compose interpolation recipe", "do not reset authentication",
                         "deployment-supplied password takes precedence"):
            self.assertIn(expected, text)

    def test_preparation_preserves_existing_file_and_protects_both_paths(self):
        for existing in (False, True):
            with self.subTest(existing=existing), tempfile.TemporaryDirectory(prefix="auth docs ") as temp:
                home = Path(temp)
                path = home / ".config/pulse/docker-auth.env"
                if existing:
                    private_file(home)
                    path.chmod(0o644)
                    path.parent.chmod(0o755)
                result = subprocess.run(["bash", "-c", recipe('vi "$auth_file"')],
                                        env=environment(home), capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE(path.parent.stat().st_mode), 0o700)
                self.assertEqual(path.read_text(), CONTENT if existing else "")
                self.assertEqual(json.loads((home / "editor.json").read_text()), [str(path)])
                self.assertNotIn(HASH.encode(), result.stdout + result.stderr)

    def test_preparation_refuses_symlink_without_touching_target(self):
        for linked in ("file", "directory"):
            with self.subTest(linked=linked), tempfile.TemporaryDirectory(prefix="auth docs ") as temp:
                home = Path(temp)
                target = home / "retained"
                target.mkdir()
                credential = target / "docker-auth.env"
                credential.write_text(CONTENT)
                credential.chmod(0o644)
                directory = home / ".config/pulse"
                directory.parent.mkdir()
                if linked == "directory":
                    directory.symlink_to(target, target_is_directory=True)
                else:
                    directory.mkdir()
                    (directory / "docker-auth.env").symlink_to(credential)
                result = subprocess.run(["bash", "-c", recipe('vi "$auth_file"')],
                                        env=environment(home), capture_output=True, timeout=10)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(b"Refusing a symlinked credential path", result.stderr)
                self.assertEqual(credential.read_text(), CONTENT)
                self.assertEqual(stat.S_IMODE(credential.stat().st_mode), 0o644)
                self.assertFalse((home / "editor.json").exists())

    def test_editor_failure_is_not_reported_as_preparation_success(self):
        with tempfile.TemporaryDirectory(prefix="auth docs ") as temp:
            home = Path(temp)
            path = private_file(home)
            result = subprocess.run(["bash", "-c", recipe('vi "$auth_file"')],
                                    env=environment(home, EDITOR_EXIT="23"), capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 23)
            self.assertEqual(path.read_text(), CONTENT)

    def test_launch_passes_only_private_path_and_preserves_hash_bytes(self):
        with tempfile.TemporaryDirectory(prefix="auth docs ") as temp:
            home = Path(temp)
            path = private_file(home)
            result = subprocess.run(["bash", "-c", recipe("docker run -d")],
                                    env=environment(home), capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            argv = json.loads((home / "docker.json").read_text())
            self.assertEqual(argv, ["run", "-d", "--name", "pulse", "-p", "7655:7655",
                                   "-v", "pulse_data:/data", "--env-file", str(path), "-e",
                                   "PULSE_DEPLOYMENT_METHOD=docker_run", "--restart", "unless-stopped",
                                   "rcourtman/pulse:vX.Y.Z"])
            self.assertNotIn(HASH, json.dumps(argv))
            self.assertNotIn(HASH.encode(), result.stdout + result.stderr)
            self.assertEqual(path.read_text(), CONTENT)

    def test_absent_or_empty_file_prevents_docker_start(self):
        for empty in (False, True):
            with self.subTest(empty=empty), tempfile.TemporaryDirectory(prefix="auth docs ") as temp:
                home = Path(temp)
                if empty:
                    private_file(home, "")
                result = subprocess.run(["bash", "-c", recipe("docker run -d")],
                                        env=environment(home), capture_output=True, timeout=10)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(b"Prepare the private authentication file first", result.stderr)
                self.assertFalse((home / "docker.json").exists())

    def test_docker_failure_remains_failed(self):
        with tempfile.TemporaryDirectory(prefix="auth docs ") as temp:
            home = Path(temp)
            path = private_file(home)
            result = subprocess.run(["bash", "-c", recipe("docker run -d")],
                                    env=environment(home, DOCKER_EXIT="31"), capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 31)
            self.assertEqual(path.read_text(), CONTENT)


if __name__ == "__main__":
    unittest.main()
