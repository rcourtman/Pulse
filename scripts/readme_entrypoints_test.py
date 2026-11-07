#!/usr/bin/env python3
"""Exercise the public landing pages' signed-install gate without network access."""

from __future__ import annotations

import importlib.util
import json
import re
import shutil
import sys
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read_repo_text(relative_path: str) -> str:
    return (ROOT / relative_path).read_text(encoding="utf-8")


SPEC = importlib.util.spec_from_file_location(
    "stamp_install_version", ROOT / "scripts" / "stamp_install_version.py"
)
assert SPEC is not None and SPEC.loader is not None
stamp_install_version = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = stamp_install_version
SPEC.loader.exec_module(stamp_install_version)

ENTRYPOINTS = (
    "README.md",
    "docs/i18n/de/README.md",
    "docs/i18n/es/README.md",
    "docs/i18n/fr/README.md",
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
        version = stamp_install_version.current_version()
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
                        f"https://github.com/rcourtman/Pulse/releases/download/{version}/install.sh",
                        f"https://github.com/rcourtman/Pulse/releases/download/{version}/install.sh.sshsig",
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
                self.assertEqual(events[3]["argv"], ["install.sh", "--version", version])

    def stamped_copy(self, directory: str) -> Path:
        root = Path(directory)
        for page in stamp_install_version.PAGES:
            for path in (page, f"{stamp_install_version.SHIPPED_DOCS}/{page.removeprefix('docs/')}"):
                if (ROOT / path).is_file():
                    (root / path).parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(ROOT / path, root / path)
        return root

    def test_every_install_command_uses_one_stable_version(self) -> None:
        version = stamp_install_version.current_version()
        stamp_install_version.version_key(version)
        for page in stamp_install_version.PAGES:
            with self.subTest(page=page):
                self.assertIn(version, read_repo_text(page))
        for relative_path in ENTRYPOINTS:
            with self.subTest(page=relative_path):
                self.assertNotIn("export PULSE_VERSION=vX.Y.Z", read_repo_text(relative_path))
                self.assertNotIn("rcourtman/pulse:vX.Y.Z", read_repo_text(relative_path))

    def test_stamp_moves_only_the_install_commands_and_their_shipped_copies(self) -> None:
        with tempfile.TemporaryDirectory(prefix="pulse-stamp-") as directory:
            root = self.stamped_copy(directory)
            before = stamp_install_version.current_version(root)
            placeholders = (root / "docs/DOCKER.md").read_text(encoding="utf-8").count("vX.Y.Z")
            changed = stamp_install_version.stamp("v99.0.0", root)
            self.assertEqual(stamp_install_version.current_version(root), "v99.0.0")
            self.assertIn("README.md", changed)
            self.assertIn("frontend-modern/public/docs/INSTALL.md", changed)
            docker = (root / "docs/DOCKER.md").read_text(encoding="utf-8")
            self.assertEqual(docker.count("vX.Y.Z"), placeholders)
            self.assertNotIn(before, docker)
            for page in stamp_install_version.PAGES:
                if page.startswith("docs/"):
                    shipped = f"{stamp_install_version.SHIPPED_DOCS}/{page.removeprefix('docs/')}"
                    self.assertEqual((root / page).read_bytes(), (root / shipped).read_bytes())
            self.assertEqual(stamp_install_version.stamp("v99.0.0", root), [])

    def test_stamp_refuses_prereleases_downgrades_and_disagreement(self) -> None:
        with tempfile.TemporaryDirectory(prefix="pulse-stamp-") as directory:
            root = self.stamped_copy(directory)
            stamp_install_version.stamp("v99.0.0", root)
            for refused in ("v99.1.0-rc.1", "v99.1.0-beta.1", "helm-chart-99.1.0", "v98.9.9"):
                with self.subTest(version=refused):
                    with self.assertRaises(stamp_install_version.StampError):
                        stamp_install_version.stamp(refused, root)
            readme = root / "README.md"
            readme.write_text(readme.read_text(encoding="utf-8").replace(
                "rcourtman/pulse:v99.0.0", "rcourtman/pulse:v98.0.0"), encoding="utf-8")
            with self.assertRaises(stamp_install_version.StampError):
                stamp_install_version.current_version(root)

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
            "docs/i18n/fr/README.md": (
                "31 mars 2027", "Relay n'est plus commercialisé", "tarif actuel",
                "tant que leur abonnement se poursuit", "votre propre VPN ou un tunnel",
                "ntfy, Gotify ou Pushover", "pas l'interface web",
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
