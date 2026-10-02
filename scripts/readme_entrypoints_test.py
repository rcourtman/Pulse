#!/usr/bin/env python3
"""Exercise the public landing pages' signed-install gate without network access."""

from __future__ import annotations

import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read_repo_text(relative_path: str) -> str:
    return (ROOT / relative_path).read_text(encoding="utf-8")


ENTRYPOINTS = (
    "README.md",
    "docs/i18n/de/README.md",
    "docs/i18n/es/README.md",
)
SIGNER = (
    'pulse-installer namespaces="pulse-install" ssh-ed25519 '
    "AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
)


def installer_commands(text: str) -> str:
    for block in re.findall(r"```bash\n(.*?)\n```", text, flags=re.DOTALL):
        if "ssh-keygen -Y verify" not in block:
            continue
        lines = block.splitlines()
        for index, line in enumerate(lines):
            if line.startswith("bash install.sh --version "):
                # Only exercise the download -> verification -> execution gate,
                # never any subsequent file-cleanup recipe.
                return "\n".join(lines[: index + 1]) + "\n"
    raise AssertionError("signed server-installer command block is missing")


STUB = """#!/usr/bin/python3
import json
import os
import sys
from pathlib import Path

name = Path(sys.argv[0]).name
trace = Path(os.environ['TRACE'])
events = [json.loads(line) for line in trace.read_text().splitlines()] if trace.exists() else []
event = {'command': name, 'argv': sys.argv[1:]}
if name == 'ssh-keygen':
    event['allowed_signers'] = Path(sys.argv[sys.argv.index('-f') + 1]).read_text()
with trace.open('a') as out:
    out.write(json.dumps(event) + '\\n')
if name == 'curl':
    attempt = 1 + sum(event['command'] == 'curl' for event in events)
    code = int(os.environ.get('CURL_' + str(attempt), '0'))
    if code == 0:
        # Public dummy bytes only. The installer itself is never executed.
        Path(sys.argv[-1].rsplit('/', 1)[-1]).write_text('downloaded fixture\\n')
elif name == 'ssh-keygen':
    sys.stdin.read()
    code = int(os.environ.get('VERIFY', '0'))
elif name == 'bash':
    code = int(os.environ.get('INSTALL', '0'))
else:
    raise AssertionError('unexpected stub command')
sys.exit(code)
"""


def exercise_install(commands: str, exits: dict[str, str]) -> tuple[int, list[dict]]:
    with tempfile.TemporaryDirectory(prefix="pulse-readme-gate-") as directory:
        root = Path(directory)
        stubs = root / "bin"
        stubs.mkdir()
        for name in ("curl", "ssh-keygen", "bash"):
            stub = stubs / name
            stub.write_text(STUB, encoding="utf-8")
            stub.chmod(0o700)
        # A failed download must not execute files left by an earlier attempt.
        (root / "install.sh").write_text("stale installer fixture\n", encoding="utf-8")
        (root / "install.sh.sshsig").write_text("stale signature fixture\n", encoding="utf-8")
        trace = root / "trace.jsonl"
        completed = subprocess.run(
            ["/bin/bash", "-c", commands],
            cwd=root,
            env={"PATH": f"{stubs}:/usr/bin:/bin", "TRACE": str(trace), **exits},
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
        if completed.stderr:
            raise AssertionError(completed.stderr)
        events = [json.loads(line) for line in trace.read_text().splitlines()]
        return completed.returncode, events


class ReadmeEntrypointsTest(unittest.TestCase):
    def test_signed_install_stops_at_each_failed_trust_step(self) -> None:
        cases = (
            ({"CURL_1": "22"}, 22, ["curl"]),
            ({"CURL_2": "23"}, 23, ["curl", "curl"]),
            ({"VERIFY": "1"}, 1, ["curl", "curl", "ssh-keygen"]),
        )
        for relative_path in ENTRYPOINTS:
            commands = installer_commands(read_repo_text(relative_path))
            for exits, expected_exit, expected_steps in cases:
                with self.subTest(page=relative_path, exits=exits):
                    code, events = exercise_install(commands, exits)
                    self.assertEqual([event["command"] for event in events], expected_steps)
                    self.assertEqual(code, expected_exit)

    def test_verified_install_keeps_pinned_version_and_signer(self) -> None:
        for relative_path in ENTRYPOINTS:
            with self.subTest(page=relative_path):
                commands = installer_commands(read_repo_text(relative_path))
                self.assertIn(SIGNER, commands)
                code, events = exercise_install(commands, {})
                self.assertEqual(code, 0)
                self.assertEqual(
                    [event["command"] for event in events],
                    ["curl", "curl", "ssh-keygen", "bash"],
                )
                self.assertEqual(
                    [event["argv"][-1] for event in events[:2]],
                    [
                        "https://github.com/rcourtman/Pulse/releases/download/vX.Y.Z/install.sh",
                        "https://github.com/rcourtman/Pulse/releases/download/vX.Y.Z/install.sh.sshsig",
                    ],
                )
                self.assertTrue(all(event["argv"][0] == "-fsSLO" for event in events[:2]))
                self.assertEqual(events[2]["allowed_signers"], SIGNER + "\n")
                verify = events[2]["argv"]
                for option, value in (
                    ("-I", "pulse-installer"),
                    ("-n", "pulse-install"),
                    ("-s", "install.sh.sshsig"),
                ):
                    self.assertEqual(verify[verify.index(option) + 1], value)
                self.assertEqual(events[3]["argv"], ["install.sh", "--version", "vX.Y.Z"])

    def test_installer_failure_remains_nonzero(self) -> None:
        for relative_path in ENTRYPOINTS:
            with self.subTest(page=relative_path):
                code, events = exercise_install(
                    installer_commands(read_repo_text(relative_path)), {"INSTALL": "17"}
                )
                self.assertEqual(code, 17)
                self.assertEqual(events[-1]["command"], "bash")

    def test_entrypoint_plans_do_not_offer_retired_relay(self) -> None:
        expected = {
            "README.md": (
                "31 March 2027", "Relay is no longer sold", "current price",
                "for as long as their subscription continues", "own VPN or tunnel",
                "ntfy, Gotify or Pushover", "not the web UI",
            ),
            "docs/i18n/de/README.md": (
                "31. März 2027", "Relay wird nicht mehr verkauft", "bisherigen Preis",
                "solange ihr Abonnement weiterläuft", "eigenes VPN oder einen Tunnel",
                "ntfy, Gotify oder Pushover", "nicht mit der Weboberfläche",
            ),
            "docs/i18n/es/README.md": (
                "31 de marzo de 2027", "Relay ya no se vende", "precio actual",
                "mientras continúe su suscripción", "tu propia VPN o un túnel",
                "ntfy, Gotify o Pushover", "no la interfaz web",
            ),
        }
        stale_claims = (
            "**Relay** — Community plus", "**Pro** — Relay plus",
            "Relay ergänzt sicheren Remote-Zugriff", "Relay añade acceso remoto seguro",
        )
        for relative_path, fragments in expected.items():
            with self.subTest(page=relative_path):
                prose = " ".join(read_repo_text(relative_path).split())
                for fragment in fragments:
                    self.assertIn(fragment, prose)
                for claim in stale_claims:
                    self.assertNotIn(claim, prose)


if __name__ == "__main__":
    unittest.main()
