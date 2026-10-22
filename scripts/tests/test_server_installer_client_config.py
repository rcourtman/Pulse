#!/usr/bin/env python3
"""Real local curl/wget and SSH verification; no installer payload is executed.

Only the public transport destination and fixture CA are substituted. The
production readiness, discovery and signed-input helpers run unchanged.
"""

import contextlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os
from pathlib import Path
import re
import shutil
import ssl
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = Path(os.environ.get("PULSE_INSTALLER_UNDER_TEST", ROOT / "install.sh"))

HARNESS = r'''
source "$INSTALLER"
print_info() { :; }; print_success() { :; }
print_warn() { printf '%s\n' "$*" >&2; }
print_error() { printf '%s\n' "$*" >&2; }
sleep() { :; }
# Skip only the outer redundant watchdog to exercise both actual branches.
if [[ "$NO_TIMEOUT" == 1 ]]; then
    command() {
        [[ "$*" != '-v timeout' ]] || return 1
        builtin command "$@"
    }
fi
PINNED_RELEASE_SSH_PUBLIC_KEY=$(cat "$FIXTURE/signer.pub")
AUTO_UPDATE_DEST="$FIXTURE/helper"
FORCE_VERSION=v6.6.0
resolve_install_script_download_url() { printf '%s/install.sh\n' "$FIXTURE_URL"; }
resolve_release_asset_base_url() { printf '%s\n' "$FIXTURE_URL"; }
script_source="" script_directory=""
trap 'cleanup_container_install_inputs "$script_directory" "" false' EXIT
rc=0
case "$FLOW" in
    health) wait_for_pulse_ready "$FIXTURE_URL" 1 0 || rc=$? ;;
    redirect) get_latest_release_from_redirect "$FIXTURE_URL/latest" > "$FIXTURE/tag" || rc=$? ;;
    metadata) resolve_latest_release_tag_for_channel stable > "$FIXTURE/tag" || rc=$? ;;
    container) download_container_installer script_source script_directory || rc=$? ;;
    helper) download_auto_update_script || rc=$? ;;
    archive) download_release_archive v6.6.0 amd64 "$FIXTURE/archive" || rc=$? ;;
    *) exit 97 ;;
esac
exit "$rc"
'''


def checked(args, **kwargs):
    return subprocess.run(args, check=True, text=True, capture_output=True, **kwargs)


class ServerInstallerClientConfigTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.resources = contextlib.ExitStack()
        cls.addClassCleanup(cls.resources.close)
        cls.root = Path(cls.resources.enter_context(tempfile.TemporaryDirectory()))
        cls.real_clients = {name: shutil.which(name) for name in ("curl", "wget")}
        for name, client in cls.real_clients.items():
            if not client:
                raise RuntimeError(name + " is required; do not skip transport proof")
        # A config works with the native macOS LibreSSL as well as OpenSSL.
        (cls.root / "tls.conf").write_text("[req]\ndistinguished_name=dn\nx509_extensions=san\n"
                                           "[dn]\n[san]\nsubjectAltName=IP:127.0.0.1\n")
        checked(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                 "-subj", "/CN=localhost", "-config", str(cls.root / "tls.conf"),
                 "-keyout", str(cls.root / "tls.key"), "-out", str(cls.root / "tls.crt")])
        checked(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(cls.root / "signer")])
        cls.payloads = {}
        for name in ("install.sh", "pulse-auto-update.sh", "pulse-v6.6.0-linux-amd64.tar.gz"):
            payload = cls.root / name
            payload.write_bytes(b"inert signed input; never execute or extract\n")
            checked(["ssh-keygen", "-q", "-Y", "sign", "-f", str(cls.root / "signer"),
                     "-n", "pulse-install", str(payload)])
            cls.payloads["/" + name] = payload.read_bytes()
            cls.payloads["/" + name + ".sshsig"] = (cls.root / (name + ".sig")).read_bytes()

    def exercise(self, flow, *, trusted=True, hostile=False, tls=True, no_timeout=False,
                 tamper=False):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            requests = []
            payloads = self.payloads

            class Handler(BaseHTTPRequestHandler):
                def log_message(self, *_args):
                    pass

                def do_GET(self):
                    requests.append([self.command, self.path, self.headers.get("X-Ambient")])
                    if self.path == "/latest":
                        self.send_response(302)
                        self.send_header("Location", "/tag/v6.6.0")
                        self.end_headers()
                        return
                    payload_key = "/" + self.path.rsplit("/", 1)[-1]
                    body = payloads.get(payload_key, b"{}\n")
                    if self.path.endswith("/releases"):
                        body = b'[{"tag_name":"v6.6.0","draft":false,"prerelease":false}]'
                    if tamper and payload_key in payloads and not self.path.endswith(".sshsig"):
                        body = b"changed unsigned input\n"
                    self.send_response(200)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)

                do_DELETE = do_GET
                do_POST = do_GET

            server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            if tls:
                context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
                context.minimum_version = ssl.TLSVersion.TLSv1_2
                context.load_cert_chain(self.root / "tls.crt", self.root / "tls.key")
                server.socket = context.wrap_socket(server.socket, server_side=True)
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                url = ("https" if tls else "http") + "://127.0.0.1:" + str(server.server_port)
                tools = root / "tools"
                tools.mkdir()
                # This wrapper cannot reach GitHub: every production URL is
                # mapped to the same loopback TLS peer. All other args survive,
                # including --disable/--no-config as the first client option.
                for name, real in self.real_clients.items():
                    ca = (["--cacert", str(self.root / "tls.crt")] if name == "curl" else
                          ["--ca-certificate=" + str(self.root / "tls.crt")]) if trusted else []
                    code = ("#!/usr/bin/env python3\nimport os,sys\n"
                            "from urllib.parse import urlsplit\n"
                            "args=sys.argv[1:]\n"
                            "for i,arg in enumerate(args):\n"
                            " if arg.startswith(('https://','http://')):\n"
                            "  p=urlsplit(arg)\n"
                            "  args[i]=os.environ['FIXTURE_URL']+p.path+('?' + p.query if p.query else '')\n"
                            f"args += {ca!r}\n"
                            f"os.execv({real!r},[{real!r}]+args)\n")
                    (tools / name).write_text(code)
                    (tools / name).chmod(0o700)
                trace = root / "ambient-trace"
                if hostile:
                    (root / ".curlrc").write_text("insecure\nrequest = DELETE\ntrace-ascii = " +
                                                 str(trace) + "\nurl = " + url + "/ambient-extra\n")
                    (root / ".wgetrc").write_text("check_certificate = off\nmethod = DELETE\n"
                                                 "header = X-Ambient: fixture-only\n")
                (root / "signer.pub").write_bytes((self.root / "signer.pub").read_bytes())
                env = {**os.environ, "PATH": str(tools) + os.pathsep + os.environ["PATH"],
                       "HOME": str(root), "CURL_HOME": str(root), "XDG_CONFIG_HOME": str(root),
                       "WGETRC": str(root / ".wgetrc"), "TMPDIR": str(root),
                       "INSTALLER": str(INSTALLER), "FIXTURE": str(root), "FLOW": flow,
                       "FIXTURE_URL": url, "NO_TIMEOUT": "1" if no_timeout else "0",
                       "NO_PROXY": "*", "no_proxy": "*"}
                # wget requires WGETRC to exist when selected explicitly.
                (root / ".wgetrc").touch(exist_ok=True)
                result = subprocess.run(["bash", "-c", HARNESS], env=env, text=True,
                                        capture_output=True, timeout=20)
                return result, requests, trace.exists(), (root / "tag").read_text() if (root / "tag").exists() else None
            finally:
                server.shutdown()
                server.server_close()
                thread.join(timeout=5)

    def assert_read_only_success(self, flow, result):
        process, requests, trace, tag = result
        self.assertEqual(process.returncode, 0, process.stderr)
        expected = {
            "health": ["/api/health"],
            "redirect": ["/latest", "/tag/v6.6.0"],
            "metadata": ["/repos/rcourtman/Pulse/releases"],
            "container": ["/install.sh", "/install.sh.sshsig"],
            "helper": ["/pulse-auto-update.sh", "/pulse-auto-update.sh.sshsig"],
            "archive": ["/rcourtman/Pulse/releases/download/v6.6.0/pulse-v6.6.0-linux-amd64.tar.gz",
                        "/rcourtman/Pulse/releases/download/v6.6.0/pulse-v6.6.0-linux-amd64.tar.gz.sshsig"],
        }[flow]
        self.assertEqual(requests, [["GET", path, None] for path in expected])
        self.assertFalse(trace, "ambient curl trace was written")
        if flow in ("metadata", "redirect"):
            self.assertEqual(tag, "v6.6.0\n")

    def test_trusted_peer_preserves_discovery_and_signed_read_only_inputs(self):
        for flow in ("health", "redirect", "metadata", "container", "helper", "archive"):
            for no_timeout in (False, True):
                for hostile in (False, True):
                    with self.subTest(flow=flow, no_timeout=no_timeout, hostile=hostile):
                        self.assert_read_only_success(flow, self.exercise(flow, hostile=hostile, no_timeout=no_timeout))

    def test_ambient_insecure_never_admits_an_untrusted_peer(self):
        for flow in ("health", "redirect", "metadata", "container", "helper", "archive"):
            for hostile in (False, True):
                with self.subTest(flow=flow, hostile=hostile):
                    process, requests, trace, _ = self.exercise(flow, trusted=False, hostile=hostile)
                    self.assertNotEqual(process.returncode, 0, process.stdout + process.stderr)
                    self.assertEqual(requests, [])
                    self.assertFalse(trace)

    def test_local_plain_http_readiness_remains_supported(self):
        self.assert_read_only_success("health", self.exercise("health", hostile=True, tls=False))

    def test_changed_signed_inputs_are_still_refused(self):
        for flow in ("container", "helper", "archive"):
            with self.subTest(flow=flow):
                process, requests, trace, _ = self.exercise(flow, hostile=True, tamper=True)
                self.assertNotEqual(process.returncode, 0)
                self.assertIn("signature", process.stderr.lower())
                self.assertTrue(requests)
                self.assertFalse(trace)

    def test_every_direct_client_disables_ambient_configuration_first(self):
        source = INSTALLER.read_text()
        calls = {"curl": 0, "wget": 0}
        for line in source.splitlines():
            if line.lstrip().startswith("#"):
                continue
            if re.search(r'\bcurl\s+(?:["\'-]|https?://)', line):
                calls["curl"] += 1
                if 'curl "${curl_args[@]}"' in line:
                    self.assertIn("local curl_args=(--disable ", source)
                else:
                    self.assertIn("curl --disable ", line)
            if re.search(r"\bwget\s+-", line):
                calls["wget"] += 1
                self.assertIn("wget --no-config ", line)
        self.assertEqual(calls, {"curl": 19, "wget": 3})


if __name__ == "__main__":
    unittest.main()
