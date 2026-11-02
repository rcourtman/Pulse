#!/usr/bin/env python3
"""Execute the copied secret-body recipe, never real credentials or services."""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest

from test_api_auth_docs import (
    DOC, TEST_TOKEN, exercise_curl, private_recording_server, recording_server,
    request_helper,
)


BODY_TOKEN = "synthetic-body-secret-not-the-header"
BODY = json.dumps({"token": BODY_TOKEN}) + "\n"


def body_recipe() -> str:
    blocks = re.findall(r"```bash\n(.*?)```", DOC.read_text(), re.DOTALL)
    matches = [block for block in blocks if 'body_file=$(mktemp ' in block]
    if len(matches) != 1:
        raise AssertionError("expected one executable private JSON recipe")
    return matches[0]


def editor(home: Path, source: str | None = None) -> None:
    tools = home / "tools"
    tools.mkdir(exist_ok=True)
    vi = tools / "vi"
    vi.write_text(source or (
        "#!/usr/bin/env python3\nimport json, os, stat, sys\nfrom pathlib import Path\n"
        "p = Path(sys.argv[1])\n"
        "assert len(sys.argv) == 2\n"
        "assert p.parent == Path(os.environ['HOME']) / '.config/pulse'\n"
        "assert p.name.startswith('api-body.')\n"
        "assert stat.S_IMODE(p.stat().st_mode) == 0o600\n"
        "assert stat.S_IMODE(p.parent.stat().st_mode) == 0o700\n"
        "assert p.read_bytes() == b''\n"
        f"p.write_text({BODY!r})\n"
        "(Path(os.environ['HOME']) / 'editor-argv.json').write_text(json.dumps(sys.argv[1:]))\n"
    ))
    vi.chmod(0o700)


class APISecretBodyDocsTest(unittest.TestCase):
    def test_copied_body_uses_stdin_not_shell_arguments_or_history_literals(self):
        recipe = body_recipe()
        self.assertNotIn(BODY_TOKEN, recipe)
        self.assertIn('pulse_api POST /api/security/validate-token < "$body_file"', recipe)
        self.assertNotIn("<<", recipe)
        self.assertNotIn("--data", recipe)
        guide = DOC.read_text()
        for boundary in ("separate, already", "settings:write", "valid: false",
                         "does not prove permission", "shell arguments", "swap/backup",
                         "Do not widen a monitoring token"):
            self.assertIn(boundary, guide)

    def test_real_transport_keeps_both_secrets_private_and_preserves_previous_files(self):
        for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
            for status, partial, expected_exit in ((200, False, 0), (400, False, 22),
                                                   (403, False, 22), (302, False, 1),
                                                   (200, True, 18)):
                with self.subTest(header=key, status=status, partial=partial), \
                        tempfile.TemporaryDirectory() as temporary, \
                        private_recording_server(status, partial) as (port, requests):
                    home = Path(temporary)
                    editor(home)
                    directory = home / ".config/pulse"
                    directory.mkdir(parents=True)
                    previous = directory / "api-body.previous"
                    previous.write_text("retain earlier secret body\n")
                    previous.chmod(0o600)
                    for _ in range(2):
                        before = {p: p.read_bytes() for p in directory.glob("api-body.*")}
                        result = exercise_curl(self, home, f"{key}: {value}", port,
                                               request_helper() + "\n" + body_recipe(),
                                               private_response=True)
                        self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                        added = set(directory.glob("api-body.*")) - before.keys()
                        self.assertEqual(len(added), 1)
                        body = added.pop()
                        self.assertEqual(body.read_text(), BODY)
                        self.assertEqual(stat.S_IMODE(body.stat().st_mode), 0o600)
                        self.assertIn(str(body).encode(), result.stdout)
                        self.assertNotIn(BODY_TOKEN.encode(), result.stdout + result.stderr)
                        self.assertNotIn(BODY_TOKEN, (home / "argv.json").read_text())
                        self.assertEqual(json.loads((home / "editor-argv.json").read_text()), [str(body)])
                        for p, content in before.items():
                            self.assertEqual(p.read_bytes(), content)
                        path, headers, method, payload = requests[-1]
                        self.assertEqual((method, path), ("POST", "/api/security/validate-token"))
                        self.assertEqual(headers[key], value)
                        self.assertEqual(json.loads(payload), {"token": BODY_TOKEN})
                    self.assertEqual(len(requests), 2, "one deliberate call each, never a redirect or retry")

    def test_http_success_does_not_turn_invalid_token_into_valid_result(self):
        response = b'{"valid":false,"message":"Token is invalid"}\n'
        with tempfile.TemporaryDirectory() as temporary, \
                recording_server(response=response) as (port, requests):
            home = Path(temporary)
            editor(home)
            result = exercise_curl(self, home, "X-API-Token: " + TEST_TOKEN, port,
                                   request_helper() + "\n" + body_recipe())
            self.assertEqual(result.returncode, 0, "helper reports HTTP success, not token validity")
            files = list((home / ".config/pulse").glob("api-response.*"))
            self.assertEqual(len(files), 1)
            self.assertIs(json.loads(files[0].read_bytes())["valid"], False)
            self.assertEqual(stat.S_IMODE(files[0].stat().st_mode), 0o600)
            self.assertEqual(len(requests), 1)

    def run_stopped_recipe(self, home: Path) -> subprocess.CompletedProcess:
        mock = 'pulse_api() { : > "$HOME/request-called"; }\n'
        result = subprocess.run(["bash", "-c", mock + body_recipe()], capture_output=True,
                                env=dict(os.environ, HOME=str(home),
                                         PATH=f"{home / 'tools'}:{os.environ['PATH']}"), timeout=10)
        self.assertFalse((home / "request-called").exists())
        return result

    def test_preparation_and_editor_fail_closed_in_an_ordinary_shell(self):
        for command in ("mkdir", "chmod", "mktemp", "vi"):
            with self.subTest(command=command), tempfile.TemporaryDirectory() as temporary:
                home = Path(temporary)
                editor(home)
                failure = home / "tools" / command
                failure.write_text("#!/bin/sh\nexit 73\n")
                failure.chmod(0o700)
                self.assertEqual(self.run_stopped_recipe(home).returncode, 73)

    def test_symlinked_directories_are_rejected_without_changing_the_target(self):
        for component in (".config", ".config/pulse"):
            with self.subTest(component=component), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                home, target = root / "home", root / "target"
                home.mkdir()
                target.mkdir(mode=0o755)
                editor(home)
                link = home / component
                link.parent.mkdir(parents=True, exist_ok=True)
                link.symlink_to(target)
                self.assertEqual(self.run_stopped_recipe(home).returncode, 1)
                self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o755)
                self.assertEqual(list(target.iterdir()), [])

    def test_editor_replaced_symlink_is_rejected_before_read_or_chmod(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            target = home / "keep-private-original"
            target.write_text("unchanged\n")
            target.chmod(0o644)
            editor(home, "#!/usr/bin/env python3\nimport os, sys\nfrom pathlib import Path\n"
                   "p=Path(sys.argv[1]); p.unlink(); p.symlink_to(Path(os.environ['HOME'])/'keep-private-original')\n")
            self.assertEqual(self.run_stopped_recipe(home).returncode, 1)
            self.assertEqual(target.read_text(), "unchanged\n")
            self.assertEqual(stat.S_IMODE(target.stat().st_mode), 0o644)


if __name__ == "__main__":
    unittest.main()
