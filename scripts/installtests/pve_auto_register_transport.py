#!/usr/bin/env python3
"""Real-curl controls for the installer's PVE token probe, using private peers.

Only extracted installer functions run, against synthetic certificates/tokens.
Fixed CA paths are relocated into the fixture; curl's security/timeout options
are not rewritten. No PVE command, installed service or provider is used.
"""

import http.server
import json
import os
from pathlib import Path
import re
import shutil
import socket
import ssl
import subprocess
import tempfile
import threading
import time
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = (ROOT / "install.sh").read_text()
TOKEN_ID, TOKEN_VALUE = "fixture@pve!monitor", "fixture-only-secret"
PROVIDER_TEXT = "provider-body-must-not-be-logged " + TOKEN_VALUE


def function(name):
    match = re.search(r"(?m)^" + re.escape(name) + r"\(\) \{\n", SOURCE)
    if not match:
        raise AssertionError("missing installer function: " + name)
    # A Python dictionary inside a shell heredoc can also have a bare '}'.
    # Stop at the next shell function definition, then its last closing brace,
    # rather than truncating the real caller at a heredoc dictionary.
    following = re.search(r"(?m)^[a-zA-Z_][a-zA-Z0-9_]*\(\) \{", SOURCE[match.end():])
    end = match.end() + following.start() if following else len(SOURCE)
    block = SOURCE[match.start():end]
    return block[:block.rindex("\n}") + 2]


class Peer:
    def __init__(self, certificate=None, key=None, *, status=200, redirect="", stall=False):
        self.requests = []
        peer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_GET(self):
                peer.requests.append((self.command, self.path, self.headers.get("Authorization")))
                if stall:
                    peer.done.wait(25)
                    return
                self.send_response(status)
                self.send_header("Content-Length", str(len(PROVIDER_TEXT.encode())))
                self.send_header("Connection", "close")
                if redirect:
                    self.send_header("Location", redirect)
                self.end_headers()
                try:
                    self.wfile.write(PROVIDER_TEXT.encode())
                except (BrokenPipeError, ConnectionResetError, ssl.SSLError):
                    pass

        self.done = threading.Event()
        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.server.daemon_threads = True
        scheme = "http"
        if certificate:
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(certificate, key)
            self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
            scheme = "https"
        self.url = f"{scheme}://127.0.0.1:{self.server.server_port}"
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={"poll_interval": 0.05})
        self.thread.start()

    def close(self):
        self.done.set()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        if self.thread.is_alive():
            raise AssertionError("fixture listener did not stop")


class PVEAutoRegisterTransport(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        for tool in ("bash", "curl", "openssl", "python3"):
            if not shutil.which(tool):
                raise RuntimeError("PVE transport proof requires " + tool)
        cls.curl = shutil.which("curl")
        cls.assets = tempfile.TemporaryDirectory(prefix="pulse-pve-trust-")
        cls.addClassCleanup(cls.assets.cleanup)
        cls.root = Path(cls.assets.name)
        cls.certificates = {}
        for name, san in (("cluster", "IP:127.0.0.1"), ("public", "IP:127.0.0.1"),
                          ("unknown", "IP:127.0.0.1"), ("wrong-name", "DNS:not-this-host.invalid")):
            cert, key = cls.root / (name + ".pem"), cls.root / (name + ".key")
            config = cls.root / (name + ".cnf")
            config.write_text("[req]\ndistinguished_name=dn\nx509_extensions=extensions\nprompt=no\n"
                              f"[dn]\nCN={name}\n[extensions]\nsubjectAltName={san}\n"
                              "basicConstraints=critical,CA:TRUE\n"
                              "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign\n"
                              "extendedKeyUsage=serverAuth\n")
            subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-sha256",
                            "-days", "1", "-keyout", str(key), "-out", str(cert), "-config", str(config)],
                           check=True, capture_output=True, timeout=15)
            cls.certificates[name] = cert, key
        cls.system_ca = cls.root / "system-ca"
        cls.system_ca.mkdir()
        public_ca = cls.certificates["public"][0]
        ca_hash = subprocess.run(["openssl", "x509", "-hash", "-noout", "-in", str(public_ca)],
                                 check=True, capture_output=True, text=True, timeout=5).stdout.strip()
        shutil.copyfile(public_ca, cls.system_ca / (ca_hash + ".0"))

    def peer(self, name="cluster", **kwargs):
        peer = Peer(*self.certificates[name], **kwargs) if name else Peer(**kwargs)
        self.addCleanup(peer.close)
        return peer

    def exercise(self, url, *, mode="smoke", cluster_ca="cluster", ambient=False, port="8006"):
        with tempfile.TemporaryDirectory(prefix="pulse-pve-probe-") as temporary:
            fixture = Path(temporary)
            tools = fixture / "tools"
            tools.mkdir()
            ca = self.certificates[cluster_ca][0] if cluster_ca else fixture / "absent.pem"
            source = "\n".join(function(name) for name in (
                "pve_auto_register_api_status", "preflight_pve_auto_register_api",
                "smoke_test_pve_auto_register_token", "auto_register_pve_node"))
            source = source.replace("/etc/pve/pve-root-ca.pem", str(ca))
            source = source.replace("/etc/ssl/certs", str(self.system_ca))
            # Observe argv without changing stdin, destination or curl options.
            (tools / "curl").write_text("#!/usr/bin/env python3\nimport json, os, sys\n"
                                        "with open(os.environ['ARGV_LOG'], 'a') as f:\n"
                                        "    f.write(json.dumps(sys.argv[1:]) + '\\n')\n"
                                        "os.execv(os.environ['REAL_CURL'], ['curl'] + sys.argv[1:])\n")
            (tools / "curl").chmod(0o755)
            # Any PVE operation or setup-token call in a refused preflight is
            # a failure, not permission to emulate successful registration.
            (tools / "pveum").write_text('#!/bin/sh\nprintf "called\\n" >> "$PVE_LOG"\nexit 99\n')
            (tools / "pveum").chmod(0o755)
            proxy = self.peer(None) if ambient else None
            if ambient:
                (fixture / ".curlrc").write_text("insecure\nlocation\ntrace-ascii = " + str(fixture / "trace") + "\n")
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       HOME=str(fixture), CURL_HOME=str(fixture), REAL_CURL=self.curl,
                       ARGV_LOG=str(fixture / "argv"), PVE_LOG=str(fixture / "pve"),
                       URL=url, TOKEN_ID=TOKEN_ID, TOKEN_VALUE=TOKEN_VALUE,
                       PULSE_PVE_HOST_URL=url, PULSE_PVE_API_PORT=port,
                       NO_PROXY="", no_proxy="", HTTPS_PROXY=proxy.url if proxy else "",
                       https_proxy=proxy.url if proxy else "", ALL_PROXY="", all_proxy="")
            setup = ('set -euo pipefail\nprint_warn() { printf "%s\\n" "$*"; }\n'
                     'IN_CONTAINER=false\nAUTO_NODE_REGISTER_ERROR=""\n') + source + "\n"
            if mode == "auto":
                command = 'auto_register_pve_node 999 127.0.0.1 7655\nprintf "ERROR=%s\\n" "$AUTO_NODE_REGISTER_ERROR"\n'
            elif mode == "preflight":
                command = 'preflight_pve_auto_register_api "$URL"\n'
            else:
                command = 'smoke_test_pve_auto_register_token "$URL" "$TOKEN_ID" "$TOKEN_VALUE"\n'
            start = time.monotonic()
            result = subprocess.run(["bash", "-c", setup + command], env=env,
                                    capture_output=True, text=True, timeout=23)
            elapsed = time.monotonic() - start
            output = result.stdout + result.stderr
            self.assertNotIn(TOKEN_VALUE, output)
            self.assertNotIn(PROVIDER_TEXT, output)
            self.assertFalse((fixture / "trace").exists(), "ambient curl tracing was used")
            self.assertFalse((fixture / "pve").exists(), "a refused preflight changed PVE credentials")
            argv = [json.loads(line) for line in (fixture / "argv").read_text().splitlines()] if (fixture / "argv").exists() else []
            for args in argv:
                self.assertEqual(args[0], "--disable")
                self.assertNotIn(TOKEN_VALUE, " ".join(args))
                self.assertNotIn(TOKEN_ID, " ".join(args))
                self.assertNotIn("Authorization:", " ".join(args))
                for value in ("--insecure", "-k", "--location", "--retry", "--verbose", "--trace"):
                    self.assertNotIn(value, args)
                self.assertEqual(args[args.index("--proto") + 1], "=https")
                self.assertEqual(args[args.index("--connect-timeout") + 1], "5")
                self.assertEqual(args[args.index("--max-time") + 1], "15")
                self.assertEqual(args[args.index("--output") + 1], "/dev/null")
            if proxy:
                self.assertEqual(proxy.requests, [], "ambient proxy received a request")
            return result.returncode, output, argv, elapsed

    def test_cluster_and_system_trust_both_authenticate_the_api(self):
        for name in ("cluster", "public"):
            with self.subTest(certificate=name):
                peer = self.peer(name)
                code, output, argv, _ = self.exercise(peer.url, ambient=True)
                self.assertEqual(code, 0, output)
                self.assertEqual(len(argv), 1)
                self.assertEqual(peer.requests, [("GET", "/api2/json/nodes",
                                                 f"PVEAPIToken={TOKEN_ID}={TOKEN_VALUE}")])

    def test_missing_cluster_ca_keeps_system_trust(self):
        peer = self.peer("public")
        code, output, argv, _ = self.exercise(peer.url, cluster_ca=None)
        self.assertEqual(code, 0, output)
        self.assertNotIn("--cacert", argv[0])

    def test_untrusted_or_wrong_name_certificates_never_receive_a_token(self):
        for name in ("unknown", "wrong-name"):
            with self.subTest(certificate=name):
                peer = self.peer(name)
                # Trust the wrong-name certificate to isolate hostname checks.
                ca = "wrong-name" if name == "wrong-name" else "cluster"
                code, output, argv, _ = self.exercise(peer.url, cluster_ca=ca, ambient=True)
                self.assertNotEqual(code, 0, output)
                self.assertEqual(len(argv), 1)
                self.assertEqual(peer.requests, [])

    def test_http_never_receives_the_token(self):
        peer = self.peer(None)
        code, output, argv, _ = self.exercise(peer.url, ambient=True)
        self.assertNotEqual(code, 0, output)
        self.assertEqual(len(argv), 1)
        self.assertEqual(peer.requests, [])

    def test_redirects_and_non_200_responses_fail_without_disclosing_bodies(self):
        destination = self.peer()
        for status in (204, 301, 302, 307, 401, 403, 500):
            with self.subTest(status=status):
                peer = self.peer(status=status, redirect=destination.url + "/redirected")
                code, output, argv, _ = self.exercise(peer.url, ambient=True)
                self.assertNotEqual(code, 0, output)
                self.assertEqual(len(argv), 1)
                self.assertEqual(len(peer.requests), 1)
                self.assertEqual(destination.requests, [])

    def test_credential_free_preflight_accepts_expected_auth_challenges(self):
        for status in (200, 401, 403):
            with self.subTest(status=status):
                peer = self.peer(status=status)
                code, output, _, _ = self.exercise(peer.url, mode="preflight")
                self.assertEqual(code, 0, output)
                self.assertEqual(peer.requests, [("GET", "/api2/json/nodes", None)])

    def test_failed_preflight_cannot_create_or_rotate_credentials(self):
        for name, status in (("unknown", 200), ("cluster", 302), ("cluster", 500)):
            with self.subTest(certificate=name, status=status):
                peer = self.peer(name, status=status)
                code, output, argv, _ = self.exercise(peer.url, mode="auto")
                self.assertEqual(code, 0, output)  # Optional setup must not fail the installation.
                self.assertIn("ERROR=Proxmox API trust or availability check failed", output)
                self.assertEqual(len(argv), 1)  # No request for a setup token.
                self.assertTrue(all(request[2] is None for request in peer.requests))

    def test_unsafe_origins_stop_before_any_network_or_token_operation(self):
        for url in ("http://127.0.0.1", "https://user:password@127.0.0.1", "https://127.0.0.1/path",
                    "https://127.0.0.1?token=fixture", "https://127.0.0.1#fragment",
                    "https://127.0.0.1:0", "https://127.0.0.1:65536", "https://[::1",
                    "https://bad\\host", "https://host\nname"):
            with self.subTest(url=url):
                code, output, argv, _ = self.exercise(url, mode="auto")
                self.assertEqual(code, 0, output)
                self.assertIn("ERROR=invalid host URL", output)
                self.assertEqual(argv, [])

    def test_origin_normalization_preserves_dns_ports_and_ipv6(self):
        # A deterministic curl double stops after normalization, before any
        # PVE or Pulse operation. Real TLS admission is exercised above.
        source = function("auto_register_pve_node")
        for host, port, expected in (("pve.example.test", "8006", "https://pve.example.test:8006"),
                                     ("https://pve.example.test:8443/", "8006", "https://pve.example.test:8443"),
                                     ("[2001:db8::1]", "8006", "https://[2001:db8::1]:8006")):
            with self.subTest(host=host):
                script = ('set -eu\nIN_CONTAINER=false\nprint_warn() { :; }\n'
                          'pveum() { return 99; }\ncurl() { return 99; }\n'
                          'preflight_pve_auto_register_api() { printf "%s\\n" "$1"; return 1; }\n'
                          + source + '\nauto_register_pve_node 999 127.0.0.1\n')
                result = subprocess.run(["bash", "-c", script], capture_output=True, text=True, timeout=5,
                                        env=dict(os.environ, PULSE_PVE_HOST_URL=host, PULSE_PVE_API_PORT=port))
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.strip(), expected)

    def test_stalled_response_obeys_the_request_budget(self):
        peer = self.peer(stall=True)
        code, output, argv, elapsed = self.exercise(peer.url)
        self.assertNotEqual(code, 0, output)
        self.assertEqual(len(argv), 1)
        self.assertGreater(elapsed, 13)
        self.assertLess(elapsed, 19)

    def test_stalled_handshake_obeys_the_connect_budget(self):
        listener = socket.socket()
        listener.bind(("127.0.0.1", 0))
        listener.listen()
        listener.settimeout(1)
        done = threading.Event()

        def accept():
            while not done.is_set():
                try:
                    connection, _ = listener.accept()
                except socket.timeout:
                    continue
                with connection:
                    done.wait(20)
                return

        thread = threading.Thread(target=accept)
        thread.start()
        try:
            code, output, argv, elapsed = self.exercise(f"https://127.0.0.1:{listener.getsockname()[1]}")
            self.assertNotEqual(code, 0, output)
            self.assertEqual(len(argv), 1)
            self.assertGreater(elapsed, 4)
            self.assertLess(elapsed, 9)
        finally:
            done.set()
            thread.join(timeout=5)
            listener.close()
            self.assertFalse(thread.is_alive(), "fixture handshake listener did not stop")

    def test_trust_preflight_precedes_all_setup_and_token_mutations(self):
        source = function("auto_register_pve_node")
        preflight = source.index('preflight_pve_auto_register_api "$normalized_host_url"')
        for action in ('"$pulse_url/api/setup-script-url"', 'pveum user add',
                       'create_pve_auto_register_token "$token_name"',
                       'pveum aclmod / -token "$token_id"'):
            self.assertLess(preflight, source.index(action))
        self.assertNotIn('pveum user token remove', source,
                         "optional registration must preserve an existing monitoring token")

    def test_extracted_functions_are_complete_bash(self):
        for name in ("pve_auto_register_api_status", "preflight_pve_auto_register_api",
                     "smoke_test_pve_auto_register_token", "auto_register_pve_node"):
            with self.subTest(function=name):
                result = subprocess.run(["bash", "-n"], input=function(name), text=True,
                                        capture_output=True, timeout=5)
                self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
