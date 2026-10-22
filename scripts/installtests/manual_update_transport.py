#!/usr/bin/env python3
"""Exercise the generated update command with real local HTTPS and signatures.

Only inert, fixture-signed installers run. Curl's destination and CA are routed
to a private listener by a recording wrapper; its production timeout, protocol
and configuration options are unchanged. No service, provider or host install
is used. These checks are not installed upgrade or LXC recovery acceptance.
"""

import json
import os
from pathlib import Path
import shlex
import shutil
import signal
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_MANUAL_UPDATE_INSTALLER", ROOT / "install.sh"))
ASSET = "/rcourtman/Pulse/releases/latest/download/install.sh"
OPTIONS = ["--disable", "-fsSL", "--connect-timeout", "5", "--max-time", "60",
           "--proto", "=https", "--proto-redir", "=https"]


class LocalDelivery:
    def __init__(self, certificate, key, script, signature, fault, phase):
        self.script, self.signature = script, signature
        self.fault, self.phase = fault, phase
        self.requests, self.errors = [], []
        self.finished = threading.Event()
        self.listener = socket.socket()
        self.listener.bind(("127.0.0.1", 0))
        self.port = self.listener.getsockname()[1]
        self.listener.listen()
        self.listener.settimeout(0.1)
        self.context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        self.context.load_cert_chain(certificate, key)
        self.thread = threading.Thread(target=self.serve)
        self.thread.start()

    def serve(self):
        attempts = 0
        try:
            while not self.finished.is_set():
                try:
                    connection, _ = self.listener.accept()
                except socket.timeout:
                    continue
                attempts += 1
                with connection:
                    if self.fault == "handshake" and attempts == self.phase:
                        # A missing five-second connect budget cannot pass by
                        # waiting for the server to give it a favourable error.
                        self.finished.wait(30)
                        continue
                    connection.settimeout(3)
                    with self.context.wrap_socket(connection, server_side=True) as secure:
                        request = b""
                        while b"\r\n\r\n" not in request:
                            block = secure.recv(4096)
                            if not block:
                                raise ValueError("incomplete local HTTP request")
                            request += block
                            if len(request) > 16384:
                                raise ValueError("oversized local HTTP request")
                        method, path, _ = request.split(b"\r\n", 1)[0].decode("ascii").split()
                        self.requests.append((method, path))
                        phase = 2 if path == ASSET + ".sshsig" else 1
                        payload = self.signature if phase == 2 else self.script
                        status, headers = "200 OK", ""
                        if method != "GET" or path not in (ASSET, ASSET + ".sshsig"):
                            status, payload = "405 Method Not Allowed", b""
                        elif phase == self.phase:
                            if self.fault == "http":
                                status, payload = "503 Service Unavailable", b""
                            elif self.fault == "redirect":
                                status, payload = "302 Found", b""
                                headers = f"Location: http://127.0.0.1:{self.port}/not-https\r\n"
                            elif self.fault == "signature":
                                payload = b"not an SSH signature\n"
                        length = len(payload) + (64 if self.fault == "partial" and phase == self.phase else 0)
                        response = (f"HTTP/1.1 {status}\r\nContent-Length: {length}\r\n"
                                    + headers + "Connection: close\r\n\r\n").encode() + payload
                        secure.sendall(response)
        except Exception as error:
            self.errors.append(repr(error))

    def close(self):
        self.finished.set()
        self.thread.join(timeout=5)
        self.listener.close()
        if self.thread.is_alive():
            raise AssertionError("local HTTPS listener did not stop")
        if self.errors:
            raise AssertionError("local HTTPS listener failed: " + repr(self.errors))


class ManualUpdateTransport(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.assets = tempfile.TemporaryDirectory(prefix="pulse-manual-transport-assets-")
        cls.addClassCleanup(cls.assets.cleanup)
        cls.root = Path(cls.assets.name)
        for name in ("bash", "curl", "openssl", "ssh-keygen"):
            if not shutil.which(name):
                raise RuntimeError("transport proof requires " + name)
        cls.curl = shutil.which("curl")
        cls.key = cls.root / "signing-key"
        subprocess.run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(cls.key)],
                       check=True, capture_output=True, timeout=10)
        cls.script = cls.root / "install.sh"
        cls.script.write_text('#!/bin/bash\nset -eu\nprintf "%s\\n" "$@" > "$FIXTURE/executed"\n'
                              'printf "%s\\n" "$PULSE_SERVICE_NAME" "$PULSE_INSTALL_DIR" '
                              '"$PULSE_CONFIG_DIR" > "$FIXTURE/identity"\n')
        subprocess.run(["ssh-keygen", "-q", "-Y", "sign", "-f", str(cls.key),
                        "-n", "pulse-install", str(cls.script)],
                       check=True, capture_output=True, timeout=10)
        cls.certificate, cls.tls_key = cls.root / "cert.pem", cls.root / "key.pem"
        config = cls.root / "openssl.cnf"
        config.write_text("[req]\ndistinguished_name=dn\nx509_extensions=extensions\nprompt=no\n"
                          "[dn]\nCN=github.com\n[extensions]\nsubjectAltName=DNS:github.com\n"
                          "basicConstraints=critical,CA:TRUE\n"
                          "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign\n"
                          "extendedKeyUsage=serverAuth\n")
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-sha256",
                        "-days", "1", "-keyout", str(cls.tls_key), "-out", str(cls.certificate),
                        "-config", str(config)], check=True, capture_output=True, timeout=15)

    def exercise(self, fault="", phase=1, *, ambient=False, allocation=0):
        with tempfile.TemporaryDirectory(prefix="pulse-manual-transport-") as temporary:
            fixture = Path(temporary)
            config = fixture / "config"
            config.mkdir()
            (fixture / "tmp").mkdir()
            (fixture / "tools").mkdir()
            (config / "system.json").write_text('{"updateChannel":"rc","autoUpdateEnabled":false}\n')
            before = (config / "system.json").read_bytes()
            helper = fixture / "update"
            env = dict(os.environ, FIXTURE=str(fixture), INSTALLER=str(INSTALLER),
                       FIXTURE_PUBLIC_KEY=Path(str(self.key) + ".pub").read_text().strip(),
                       PULSE_INSTALL_DIR=str(fixture / "installed"), PULSE_CONFIG_DIR=str(config),
                       PULSE_SERVICE_NAME="pulse-preview", PULSE_UPDATE_HELPER_PATH=str(helper),
                       PULSE_PROFILE_PATH=str(fixture / "profile"), PULSE_BASHRC_PATH=str(fixture / "bashrc"))
            generated = subprocess.run(["bash", "-c", 'source "$INSTALLER"\n'
                                        'PINNED_RELEASE_SSH_PUBLIC_KEY="$FIXTURE_PUBLIC_KEY"\n'
                                        'setup_update_command\n'], env=env,
                                       text=True, capture_output=True, timeout=10)
            self.assertEqual(generated.returncode, 0, generated.stdout + generated.stderr)
            syntax = subprocess.run(["bash", "-n", str(helper)], capture_output=True, timeout=5)
            self.assertEqual(syntax.returncode, 0, syntax.stderr)
            if ambient:
                # --disable must be curl's first argument: a different request
                # method from the user's file must never reach signed delivery.
                (fixture / ".curlrc").write_text("request = DELETE\nmax-time = 0\n")
            delivery = LocalDelivery(self.certificate, self.tls_key, self.script.read_bytes(),
                                     Path(str(self.script) + ".sig").read_bytes(), fault, phase)
            try:
                wrapper = fixture / "curl-wrapper.py"
                wrapper.write_text('import json, os, sys\nfrom pathlib import Path\n'
                                   'root = Path(os.environ["FIXTURE"])\n'
                                   'with (root / "transport").open("a") as out:\n'
                                   '    out.write(json.dumps(sys.argv[1:]) + "\\n")\n'
                                   'os.execv(os.environ["REAL_CURL"], [os.environ["REAL_CURL"], *sys.argv[1:],\n'
                                   '    "--connect-to", "github.com:443:127.0.0.1:" + os.environ["TLS_PORT"],\n'
                                   '    "--cacert", os.environ["TLS_CERT"], "--noproxy", "*"])\n')
                curl = fixture / "tools/curl"
                curl.write_text("#!/bin/sh\nexec " + shlex.quote(sys.executable)
                                + " -I " + shlex.quote(str(wrapper)) + ' "$@"\n')
                curl.chmod(0o700)
                mktemp = fixture / "tools/mktemp"
                mktemp.write_text('''#!/bin/bash
set -eu
count=0
[[ ! -f "$FIXTURE/allocations" ]] || read -r count < "$FIXTURE/allocations"
((count += 1))
printf '%s\n' "$count" > "$FIXTURE/allocations"
[[ "$count" != "$FAIL_ALLOCATION" ]] || exit 73
template="${@: -1}"
exec "$REAL_MKTEMP" "$FIXTURE/tmp/${template##*/}"
''')
                mktemp.chmod(0o700)
                env.update(PATH=str(fixture / "tools") + os.pathsep + os.environ["PATH"],
                           REAL_CURL=self.curl, TLS_PORT=str(delivery.port), TLS_CERT=str(self.certificate),
                           REAL_MKTEMP=shutil.which("mktemp"), FAIL_ALLOCATION=str(allocation),
                           HOME=str(fixture), CURL_HOME=str(fixture), XDG_CONFIG_HOME=str(fixture),
                           HTTP_PROXY="", HTTPS_PROXY="", ALL_PROXY="", http_proxy="", https_proxy="", all_proxy="")
                started = time.monotonic()
                process = subprocess.Popen(["bash", str(helper)], env=env, text=True,
                                           stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
                try:
                    try:
                        stdout, stderr = process.communicate(timeout=15)
                    except subprocess.TimeoutExpired:
                        self.fail("generated helper exceeded transport deadline")
                    elapsed = time.monotonic() - started
                finally:
                    if process.poll() is None:
                        os.killpg(process.pid, signal.SIGKILL)
                    process.communicate(timeout=5)
                transports = ([json.loads(line) for line in (fixture / "transport").read_text().splitlines()]
                              if (fixture / "transport").exists() else [])
                for index, args in enumerate(transports):
                    self.assertEqual(args[:len(OPTIONS)], OPTIONS)
                    self.assertEqual(args[len(OPTIONS)], "https://github.com" + ASSET + (".sshsig" if index else ""))
                    self.assertEqual(args[len(OPTIONS) + 1], "-o")
                    self.assertEqual(len(args), len(OPTIONS) + 3)
                self.assertEqual(list((fixture / "tmp").iterdir()), [], "staged installer/signature leaked")
                self.assertEqual((config / "system.json").read_bytes(), before, "operator settings changed")
                executed = (fixture / "executed").read_text() if (fixture / "executed").exists() else None
                identity = (fixture / "identity").read_text() if (fixture / "identity").exists() else None
                print(json.dumps(dict(control=fault or "success", phase=phase, elapsed_seconds=round(elapsed, 3),
                                      command_exit=process.returncode, downloads=len(transports),
                                      installer_executed=executed is not None)), flush=True)
                return process.returncode, stdout + stderr, transports, list(delivery.requests), executed, identity, fixture
            finally:
                delivery.close()

    def assert_refused(self, fault, phase):
        code, output, transport, requests, executed, _, _ = self.exercise(fault, phase)
        self.assertNotEqual(code, 0, output)
        self.assertEqual(len(transport), phase, output)
        self.assertIsNone(executed, output)
        self.assertNotIn("Update complete!", output)
        self.assertTrue(all(method == "GET" and path in (ASSET, ASSET + ".sshsig")
                            for method, path in requests))

    def test_real_https_ignores_ambient_curl_config(self):
        code, output, transport, requests, executed, identity, fixture = self.exercise(ambient=True)
        self.assertEqual(code, 0, output)
        self.assertEqual(requests, [("GET", ASSET), ("GET", ASSET + ".sshsig")])
        self.assertEqual(len(transport), 2)
        self.assertEqual(executed, "--rc\n")
        self.assertEqual(identity, f"pulse-preview\n{fixture}/installed\n{fixture}/config\n")
        self.assertIn("Update complete!", output)

    def test_http_failures_never_execute_the_installer(self):
        for phase in (1, 2):
            with self.subTest(phase=phase):
                self.assert_refused("http", phase)

    def test_partial_transfers_never_execute_the_installer(self):
        for phase in (1, 2):
            with self.subTest(phase=phase):
                self.assert_refused("partial", phase)

    def test_http_downgrades_never_reach_the_destination(self):
        for phase in (1, 2):
            with self.subTest(phase=phase):
                self.assert_refused("redirect", phase)

    def test_stalled_tls_handshakes_obey_each_connect_budget(self):
        for phase in (1, 2):
            with self.subTest(phase=phase):
                self.assert_refused("handshake", phase)

    def test_real_signature_verification_still_refuses_unknown_content(self):
        self.assert_refused("signature", 2)

    def test_allocation_failures_leave_no_download_or_staged_file(self):
        for allocation in (1, 2):
            with self.subTest(allocation=allocation):
                code, output, transport, requests, executed, _, _ = self.exercise(allocation=allocation)
                self.assertNotEqual(code, 0, output)
                self.assertEqual(transport, [])
                self.assertEqual(requests, [])
                self.assertIsNone(executed)
                self.assertIn("Could not create installer verification files", output)


if __name__ == "__main__":
    unittest.main()
