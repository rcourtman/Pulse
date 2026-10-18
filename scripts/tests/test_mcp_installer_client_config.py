#!/usr/bin/env python3
"""Run the complete MCP installer with real local TLS/curl/SSH/install tools.

Only the fixed GitHub destination and release signing key are substituted with
an isolated loopback peer and a fresh test key. No downloaded binary is run.
"""

import contextlib
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import platform
import shutil
import ssl
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_MCP_INSTALLER", ROOT / "scripts/install-mcp.sh"))
PUBLIC_KEY = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer"
PREVIOUS = b"previous installed MCP fixture; never executed\n"
CANDIDATE = b"verified MCP fixture; never executed\n"


def checked(argv):
    return subprocess.run(argv, check=True, capture_output=True, text=True)


class MCPInstallerClientConfig(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.resources = contextlib.ExitStack()
        cls.addClassCleanup(cls.resources.close)
        cls.root = Path(cls.resources.enter_context(tempfile.TemporaryDirectory()))
        for name in ("curl", "openssl", "ssh-keygen", "install"):
            if not shutil.which(name):
                raise RuntimeError(name + " is required; do not skip installer proof")
        cls.curl = shutil.which("curl")
        os_name = platform.system().lower()
        arch = {"x86_64": "amd64", "aarch64": "arm64", "i386": "386", "i686": "386"}.get(
            platform.machine(), platform.machine())
        cls.binary = "pulse-mcp-" + os_name + "-" + arch
        (cls.root / "tls.conf").write_text("[req]\ndistinguished_name=dn\nx509_extensions=san\n"
                                           "[dn]\n[san]\nsubjectAltName=IP:127.0.0.1\n")
        checked(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                 "-subj", "/CN=localhost", "-config", str(cls.root / "tls.conf"),
                 "-keyout", str(cls.root / "tls.key"), "-out", str(cls.root / "tls.crt")])
        checked(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(cls.root / "signer")])
        cls.manifest = (hashlib.sha256(CANDIDATE).hexdigest() + "  " + cls.binary + "\n").encode()
        (cls.root / "checksums.txt").write_bytes(cls.manifest)
        checked(["ssh-keygen", "-q", "-Y", "sign", "-f", str(cls.root / "signer"),
                 "-n", "pulse-install", str(cls.root / "checksums.txt")])
        cls.signature = (cls.root / "checksums.txt.sig").read_bytes()
        source = INSTALLER.read_text()
        if source.count(PUBLIC_KEY) != 1:
            raise RuntimeError("installer must have exactly one production release key")
        cls.source = source.replace(PUBLIC_KEY, (cls.root / "signer.pub").read_text().strip(), 1)
        print("real-client versions:", checked([cls.curl, "--version"]).stdout.splitlines()[0],
              "OpenSSL:", checked(["openssl", "version"]).stdout.strip(), flush=True)

    def exercise(self, *, trusted=True, hostile=False, changed=None, missing=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            requests = []
            binary = self.binary
            payloads = {binary: CANDIDATE, "checksums.txt": self.manifest,
                        "checksums.txt.sshsig": self.signature}

            class Handler(BaseHTTPRequestHandler):
                def log_message(self, *_args):
                    pass

                def do_GET(self):
                    requests.append([self.command, self.path, self.headers.get("X-Ambient")])
                    name = self.path.rsplit("/", 1)[-1]
                    if name == missing:
                        self.send_response(404)
                        self.end_headers()
                        return
                    body = payloads.get(name, b"unexpected fixture request\n")
                    if name == changed:
                        body = b"changed unsigned input\n"
                    self.send_response(200)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)

                do_DELETE = do_GET
                do_POST = do_GET

            server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(self.root / "tls.crt", self.root / "tls.key")
            server.socket = context.wrap_socket(server.socket, server_side=True)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                url = "https://127.0.0.1:" + str(server.server_port)
                tools = root / "tools"
                tools.mkdir()
                calls = root / "calls.jsonl"
                # Keep each production curl option exactly as written. Map only
                # the public GitHub URL to loopback; no external requests occur.
                # CA trust uses the supported environment, not a product bypass.
                (tools / "curl").write_text(
                    "#!/usr/bin/env python3\nimport json,os,sys\n"
                    "from urllib.parse import urlsplit\n"
                    "args=sys.argv[1:]\n"
                    "with open(os.environ['CALLS'],'a') as f:f.write(json.dumps(args)+'\\n')\n"
                    "for i,arg in enumerate(args):\n"
                    " if arg.startswith('https://github.com/'):\n"
                    "  p=urlsplit(arg);args[i]=os.environ['FIXTURE_URL']+p.path\n"
                    f"os.execv({self.curl!r},[{self.curl!r}]+args)\n")
                (tools / "curl").chmod(0o700)
                trace = root / "ambient-trace"
                if hostile:
                    (root / ".curlrc").write_text("insecure\nrequest = DELETE\n"
                                                 'header = "X-Ambient: fixture-only"\n'
                                                 "trace-ascii = " + str(trace) + "\nurl = " +
                                                 url + "/ambient-extra\n")
                script = root / "install-mcp.sh"
                script.write_text(self.source)
                installed = root / "bin"
                installed.mkdir()
                target = installed / "pulse-mcp"
                target.write_bytes(PREVIOUS)
                target.chmod(0o755)
                env = {k: v for k, v in os.environ.items()
                       if k not in ("CURL_CA_BUNDLE", "SSL_CERT_FILE", "SSL_CERT_DIR")}
                env.update({"PATH": str(tools) + os.pathsep + os.environ["PATH"],
                            "HOME": str(root), "CURL_HOME": str(root), "XDG_CONFIG_HOME": str(root),
                            "TMPDIR": str(root), "CALLS": str(calls), "FIXTURE_URL": url,
                            "PULSE_MCP_VERSION": "v-fixture", "PULSE_MCP_REPO": "rcourtman/Pulse",
                            "PULSE_MCP_BIN_DIR": str(installed), "NO_PROXY": "*", "no_proxy": "*"})
                if trusted:
                    env["CURL_CA_BUNDLE"] = str(self.root / "tls.crt")
                result = subprocess.run(["bash", str(script)], env=env, text=True,
                                        capture_output=True, timeout=25)
                observed = {"exit": result.returncode, "output": result.stdout + result.stderr,
                        "requests": requests, "trace": trace.exists(), "installed": target.read_bytes(),
                        "calls": [json.loads(line) for line in calls.read_text().splitlines()],
                        "temporary_inputs": sorted(p.name for p in root.glob("pulse-mcp*"))}
                print("MCP_TRANSPORT_CASE " + json.dumps({
                    "trusted": trusted, "hostile": hostile, "changed": changed, "missing": missing,
                    **{key: observed[key] for key in ("exit", "requests", "trace", "calls", "temporary_inputs")},
                    "installed": "candidate" if observed["installed"] == CANDIDATE else "previous",
                }, sort_keys=True), flush=True)
                return observed
            finally:
                server.shutdown()
                server.server_close()
                thread.join(timeout=5)

    def assert_clean_transport(self, result):
        self.assertFalse(result["trace"], "saved curl settings enabled tracing")
        for method, path, header in result["requests"]:
            self.assertEqual(method, "GET", "saved curl settings changed request method: " + path)
            self.assertIsNone(header, "saved curl settings changed headers")
            self.assertNotEqual(path, "/ambient-extra", "saved curl settings added a transfer")
        for args in result["calls"]:
            self.assertEqual(args[0], "--disable", "curl must disable saved configuration first")
        self.assertEqual(result["temporary_inputs"], [], "download/signature inputs leaked")

    def test_verified_install_ignores_saved_configuration(self):
        for hostile in (False, True):
            with self.subTest(hostile=hostile):
                r = self.exercise(hostile=hostile)
                self.assertEqual(r["exit"], 0, r["output"])
                self.assertEqual(r["installed"], CANDIDATE)
                self.assertEqual(len(r["requests"]), 3)
                self.assertEqual([p.rsplit("/", 1)[-1] for _, p, _ in r["requests"]],
                                 [self.binary, "checksums.txt", "checksums.txt.sshsig"])
                self.assert_clean_transport(r)

    def test_untrusted_tls_cannot_be_enabled_by_saved_configuration(self):
        for hostile in (False, True):
            with self.subTest(hostile=hostile):
                r = self.exercise(trusted=False, hostile=hostile)
                self.assertNotEqual(r["exit"], 0, "untrusted TLS allowed installation")
                self.assertEqual(r["installed"], PREVIOUS, "refused download replaced installed binary")
                self.assertEqual(r["requests"], [], "untrusted peer received an HTTP request")
                self.assert_clean_transport(r)

    def test_bad_integrity_preserves_existing_install(self):
        for changed in (self.binary, "checksums.txt", "checksums.txt.sshsig"):
            for hostile in (False, True):
                with self.subTest(changed=changed, hostile=hostile):
                    r = self.exercise(changed=changed, hostile=hostile)
                    self.assertNotEqual(r["exit"], 0, "changed unsigned input installed")
                    self.assertEqual(r["installed"], PREVIOUS)
                    self.assertEqual(len(r["requests"]), 3)
                    self.assert_clean_transport(r)

    def test_missing_integrity_preserves_existing_install(self):
        for missing in ("checksums.txt", "checksums.txt.sshsig"):
            for hostile in (False, True):
                with self.subTest(missing=missing, hostile=hostile):
                    r = self.exercise(missing=missing, hostile=hostile)
                    self.assertNotEqual(r["exit"], 0, "missing integrity input installed")
                    self.assertEqual(r["installed"], PREVIOUS)
                    self.assertEqual(len(r["requests"]), 2 if missing == "checksums.txt" else 3)
                    self.assert_clean_transport(r)


if __name__ == "__main__":
    unittest.main()
