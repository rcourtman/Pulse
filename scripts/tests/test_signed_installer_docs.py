#!/usr/bin/env python3
"""Execute the published installer recipes with controlled transport failures."""

import os
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import re
import shutil
import ssl
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]


def recipes():
    return [block for block in re.findall(
        r"```bash\n(.*?)\n```", (ROOT / "docs/INSTALL.md").read_text(), re.S
    ) if "ssh-keygen -Y verify" in block]


def with_fixture_signer(block, public_key):
    """Replace the recipe's one public verification key, not its trust policy."""
    command, count = re.subn(r"(?<=ssh-ed25519 )\S+", lambda _match: public_key, block)
    if count != 1:
        raise ValueError(f"expected one installer verification key, found {count}")
    return command


@contextmanager
def download_servers(root, mode):
    """Real local HTTPS and a separate plaintext downgrade witness."""
    requests = []
    installer = b'#!/bin/bash\nprintf "%s\\n" "$@" > "$INSTALL_RECEIPT"\n'
    asset = root / "install.sh"
    asset.write_bytes(installer)
    subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(root / "signer")],
                   check=True, capture_output=True)
    subprocess.run(["ssh-keygen", "-Y", "sign", "-f", str(root / "signer"),
                    "-n", "pulse-install", str(asset)], check=True, capture_output=True)
    signature = (root / "install.sh.sig").read_bytes()
    subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                    "-keyout", str(root / "tls.key"), "-out", str(root / "tls.crt"),
                    "-days", "1", "-subj", "/CN=localhost",
                    "-addext", "subjectAltName=IP:127.0.0.1"], check=True, capture_output=True)

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            encrypted = isinstance(self.connection, ssl.SSLSocket)
            requests.append((encrypted, self.path, dict(self.headers)))
            if mode in ("downgrade", "redirect") and not self.path.startswith("/final/"):
                self.send_response(302)
                origin = (f"http://127.0.0.1:{plain.server_port}" if mode == "downgrade"
                          else f"https://127.0.0.1:{secure.server_port}")
                self.send_header("Location", origin + "/final/" + self.path.rsplit("/", 1)[-1])
                self.end_headers()
                return
            is_signature = self.path.endswith(".sshsig")
            body = signature if is_signature else installer
            status = 500 if mode == ("signature-error" if is_signature else "installer-error") else 200
            if mode == "bad-signature" and is_signature:
                body = b"invalid fixture signature\n"
            self.send_response(status)
            self.send_header("Content-Length", str(len(body) + (100 if mode == "partial" else 0)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_args):
            pass

    plain = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    secure = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(root / "tls.crt", root / "tls.key")
    secure.socket = context.wrap_socket(secure.socket, server_side=True)
    threads = [threading.Thread(target=server.serve_forever, daemon=True)
               for server in (plain, secure)]
    for thread in threads:
        thread.start()
    try:
        yield secure.server_port, plain.server_port, requests
    finally:
        for server in (plain, secure):
            server.shutdown()
            server.server_close()
        for thread in threads:
            thread.join(timeout=5)


def run_local_recipe(root, block, secure_port, plain_port, *, trust=True):
    """Replace only the download origin, release and signing trust fixture."""
    tools = root / "bin"
    tools.mkdir()
    sudo = tools / "sudo"
    sudo.write_text('#!/bin/bash\nexec "$@"\n')
    sudo.chmod(0o700)
    # These defaults would add an unrelated request, bypass TLS and write a
    # trace if a copied recipe forgot to disable curl configuration first.
    (root / ".curlrc").write_text(
        f'insecure\nheader = "X-Curlrc-Injected: yes"\n'
        f'trace-ascii = "{root / "unexpected-trace"}"\n'
        f'url = "http://127.0.0.1:{plain_port}/from-config"\n'
        f'output = "{root / "unexpected-download"}"\n'
    )
    public_key = (root / "signer.pub").read_text().split()[1]
    command = block.replace("vX.Y.Z", "v6.4.5-rc.2").replace(
        "https://github.com/rcourtman/Pulse/releases/download",
        f"https://127.0.0.1:{secure_port}"
    )
    command = with_fixture_signer(command, public_key)
    env = {k: v for k, v in os.environ.items()
           if not k.lower().endswith("_proxy") and k not in ("CURL_CA_BUNDLE", "SSL_CERT_FILE", "SSL_CERT_DIR")}
    env.update(HOME=str(root), CURL_HOME=str(root), XDG_CONFIG_HOME=str(root / ".config"),
               PATH=f"{tools}:{os.environ['PATH']}", TMPDIR=str(root),
               INSTALL_RECEIPT=str(root / "executed"))
    if trust:
        env["CURL_CA_BUNDLE"] = str(root / "tls.crt")
    return subprocess.run(["bash", "-c", command], cwd=root, env=env,
                          capture_output=True, text=True, timeout=15)


class SignedInstallerDocsTest(unittest.TestCase):
    def test_fixture_signer_replacement_preserves_policy_and_requires_one_key(self):
        for block in recipes():
            with self.subTest(recipe=block.splitlines()[0]):
                replaced = with_fixture_signer(block, "fixture-public")
                self.assertEqual(replaced.count("ssh-ed25519 fixture-public "), 1)
                self.assertEqual(
                    re.sub(r"(?<=ssh-ed25519 )\S+", "KEY", block),
                    re.sub(r"(?<=ssh-ed25519 )\S+", "KEY", replaced),
                )
                for malformed in ("no verification key", block + "\n" + block):
                    with self.assertRaisesRegex(ValueError, "expected one installer verification key"):
                        with_fixture_signer(malformed, "fixture-public")

    def test_mirror_and_bounded_https_only_transfers(self):
        self.assertEqual((ROOT / "docs/INSTALL.md").read_bytes(),
                         (ROOT / "frontend-modern/public/docs/INSTALL.md").read_bytes())
        self.assertEqual(len(recipes()), 2)
        for block in recipes():
            calls = re.findall(r"^curl .*?(?=\n(?:curl|ssh-keygen))", block, re.M | re.S)
            self.assertEqual(len(calls), 2)
            for call, filename in zip(calls, ("install.sh", "install.sh.sshsig")):
                self.assertTrue(call.startswith("curl --disable "))
                for option in ("--fail", "--silent", "--show-error", "--location",
                               "--proto '=https'", "--proto-redir '=https'",
                               "--connect-timeout 5", "--max-time 60", f"--output {filename}"):
                    self.assertIn(option, call)
                self.assertNotRegex(call, r"--(?:insecure|retry|verbose|trace)\b|\s-k\b")

    def test_real_downloads_signature_gate_and_hostile_defaults(self):
        for executable in ("curl", "openssl", "ssh-keygen"):
            self.assertIsNotNone(shutil.which(executable), f"{executable} required for real download proof")
        for recipe_index, block in enumerate(recipes()):
            for mode in ("success", "redirect", "downgrade", "installer-error", "signature-error",
                         "partial", "bad-signature", "untrusted"):
                with self.subTest(recipe=recipe_index, mode=mode), tempfile.TemporaryDirectory(
                    prefix="signed installer docs "
                ) as directory:
                    root = Path(directory)
                    with download_servers(root, mode) as (secure, plain, requests):
                        result = run_local_recipe(root, block, secure, plain, trust=mode != "untrusted")
                    self.assertFalse((root / "unexpected-trace").exists(), "curl config was loaded")
                    self.assertFalse((root / "unexpected-download").exists(), "curl config added a URL")
                    self.assertTrue(all(encrypted for encrypted, _path, _headers in requests),
                                    "an installer or config request reached plaintext transport")
                    self.assertFalse(any("X-Curlrc-Injected" in headers for _, _, headers in requests))
                    receipt = root / "executed"
                    if mode in ("success", "redirect"):
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual(receipt.read_text(), "--version\nv6.4.5-rc.2\n")
                        self.assertEqual(len(requests), 4 if mode == "redirect" else 2)
                    else:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertFalse(receipt.exists(), "failed download or verification executed installer")
                        self.assertEqual(len(requests), 0 if mode == "untrusted" else
                                         2 if mode in ("signature-error", "bad-signature") else 1)

    def test_download_and_signature_failures_never_execute_installer(self):
        blocks = recipes()
        self.assertEqual(len(blocks), 2, "Exercise both LXC and systemd recipes")
        for recipe, block in enumerate(blocks):
            for failure in ("installer-download", "signature-download", "timeout", "signature", "none"):
                with self.subTest(recipe=recipe, failure=failure), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    commands = root / "bin"
                    commands.mkdir()
                    scripts = {
                        "curl": '''#!/bin/bash
set -e
url="${!#}"
case "$url" in
  */install.sh)
    [[ "$FAILURE" != installer-download ]] || exit 22
    [[ "$FAILURE" != timeout ]] || exit 28
    printf '%s\\n' '#!/bin/bash' 'printf "%s\\n" "$@" > "$INSTALL_RECEIPT"' > install.sh
    ;;
  */install.sh.sshsig)
    [[ "$FAILURE" != signature-download ]] || exit 22
    printf '%s\\n' fixture-signature > install.sh.sshsig
    ;;
  *) exit 23 ;;
esac
''',
                        "ssh-keygen": '#!/bin/bash\n[[ "$FAILURE" != signature ]]\n',
                        "sudo": '#!/bin/bash\nexec "$@"\n',
                    }
                    for name, source in scripts.items():
                        executable = commands / name
                        executable.write_text(source)
                        executable.chmod(0o755)
                    receipt = root / "executed"
                    env = {**os.environ, "PATH": f"{commands}:{os.environ['PATH']}",
                           "FAILURE": failure, "INSTALL_RECEIPT": str(receipt)}
                    result = subprocess.run(["bash", "-c", block.replace("vX.Y.Z", "v6.4.5-rc.2")],
                                            cwd=root, env=env, capture_output=True, text=True)
                    if failure == "none":
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual(receipt.read_text(), "--version\nv6.4.5-rc.2\n")
                    else:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertFalse(receipt.exists(), "Unverified installer was executed")


if __name__ == "__main__":
    unittest.main()
