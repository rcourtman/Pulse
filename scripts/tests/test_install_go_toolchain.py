#!/usr/bin/env python3
"""Actual curl/private-file controls; sudo is an inert, recorded boundary.

The complete installer is copied with only its fixed download origin replaced
by a disposable HTTPS peer. No installed compiler, host path or root command
is changed. The timeout control calls the unchanged download helper with a
short fixture budget; complete-script cases separately pin production budgets.
"""

from concurrent.futures import ThreadPoolExecutor
import contextlib
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import signal
import ssl
import subprocess
import sys
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
INSTALLER = ROOT / "scripts/install-go-toolchain.sh"
PAYLOAD = b"inert authenticated archive fixture; never extracted\n"
DIGEST = hashlib.sha256(PAYLOAD).hexdigest()
ORIGIN = 'DOWNLOAD_ROOT="https://dl.google.com/go"'

COMMAND_PEER = r'''
import hashlib, json, os, pathlib, stat, sys
name = pathlib.Path(sys.argv[0]).name
args = sys.argv[1:]
record = {"command": name, "args": args}
if name == "curl":
    flag = "--output" if "--output" in args else "-o"
    target = pathlib.Path(args[args.index(flag) + 1])
    record["parent_mode"] = stat.S_IMODE(target.parent.stat().st_mode)
elif name == "sudo" and args[0] == "tar":
    target = pathlib.Path(args[args.index("-xzf") + 1])
    record["archive_sha256"] = hashlib.sha256(target.read_bytes()).hexdigest()
    record["archive_mode"] = stat.S_IMODE(target.stat().st_mode)
fd = os.open(os.environ["FIXTURE_LOG"], os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
try:
    os.write(fd, (json.dumps(record) + "\n").encode())
finally:
    os.close(fd)
if name == "curl":
    os.execv(os.environ["FIXTURE_REAL_CURL"], ["curl", *args])
# All privileged commands, including tar/chown/ln, are deliberately inert.
sys.exit(int(os.environ.get("FIXTURE_SUDO_EXIT", "0")))
'''


class Fixture:
    def __init__(self, root, cert, key, *, mode="ok", checksum=None, trusted=True,
                 pin="default", sudo_exit=0):
        self.root = root
        self.mode = mode
        self.checksum = (DIGEST + "\n").encode() if checksum is None else checksum
        self.requests = []
        self.started = threading.Event()
        self.release = threading.Event()
        self.tmp = root / "tmp"
        self.tmp.mkdir()
        self.log = root / "commands.jsonl"
        self.home = root / "home"
        self.home.mkdir()
        # This must not turn the untrusted-certificate test into a success.
        (self.home / ".curlrc").write_text("insecure\n")
        self.version = re.search(r'^DEFAULT_VERSION="([^"]+)"$',
                                 INSTALLER.read_text(), re.M)[1]
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                owner.requests.append(self.path)
                digest = self.path.endswith(".sha256")
                if owner.mode == "redirect":
                    self.send_response(302)
                    self.send_header("Location", owner.origin.replace("https:", "http:") + "/downgrade")
                    self.end_headers()
                    return
                if owner.mode == "archive-error" or (owner.mode == "digest-error" and digest):
                    self.send_response(503)
                    self.end_headers()
                    return
                body = owner.checksum if digest else PAYLOAD
                self.send_response(200)
                self.send_header("Content-Length", str(len(body) + (10 if owner.mode == "partial" else 0)))
                self.send_header("Connection", "close")
                self.end_headers()
                owner.started.set()
                if owner.mode == "stall":
                    owner.release.wait(15)
                    return
                try:
                    self.wfile.write(body)
                except (BrokenPipeError, ConnectionResetError):
                    pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.server.daemon_threads = True
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.minimum_version = ssl.TLSVersion.TLSv1_2
        tls.load_cert_chain(cert, key)
        self.server.socket = tls.wrap_socket(self.server.socket, server_side=True)
        self.origin = "https://localhost:" + str(self.server.server_port)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.script = root / "scripts/install-go-toolchain.sh"
        self.script.parent.mkdir()
        source = INSTALLER.read_text()
        if source.count(ORIGIN) != 1:
            raise AssertionError("fixed Go download origin moved; reassess fixture")
        self.script.write_text(source.replace(ORIGIN, 'DOWNLOAD_ROOT="' + self.origin + '"'))
        if pin is not None:
            (self.script.parent / ".go-version").write_bytes(
                (self.version + "\n").encode() if pin == "default" else pin)
        commands = root / "bin"
        commands.mkdir()
        for name in ("curl", "sudo"):
            target = commands / name
            target.write_text("#!" + sys.executable + "\n" + COMMAND_PEER)
            target.chmod(0o755)
        self.env = {
            "PATH": str(commands) + os.pathsep + os.environ["PATH"],
            "HOME": str(self.home), "CURL_HOME": str(self.home),
            "TMPDIR": str(self.tmp), "LC_ALL": "C",
            "HTTPS_PROXY": "http://127.0.0.1:1", "https_proxy": "http://127.0.0.1:1",
            "FIXTURE_LOG": str(self.log), "FIXTURE_REAL_CURL": shutil.which("curl"),
            "FIXTURE_SUDO_EXIT": str(sudo_exit),
        }
        if trusted:
            self.env["CURL_CA_BUNDLE"] = str(cert)

    def close(self):
        self.release.set()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)

    def run(self):
        return subprocess.run(["bash", str(self.script)], env=self.env,
                              capture_output=True, text=True, timeout=15)

    def records(self, command):
        records = [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []
        return [record for record in records if record["command"] == command]

    def downloads(self):
        return [Path(item["args"][item["args"].index("--output") + 1])
                for item in self.records("curl")]


class ToolchainInstallerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        for executable in ("curl", "openssl", "bash", "sha256sum", "mktemp"):
            if shutil.which(executable) is None:
                raise RuntimeError("missing required fixture command: " + executable)
        cls.cert_dir = tempfile.TemporaryDirectory()
        cls.cert = Path(cls.cert_dir.name) / "cert.pem"
        cls.key = Path(cls.cert_dir.name) / "key.pem"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", str(cls.key), "-out", str(cls.cert), "-days", "1",
                        "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost"],
                       check=True, capture_output=True, timeout=15)

    @classmethod
    def tearDownClass(cls):
        cls.cert_dir.cleanup()

    @contextlib.contextmanager
    def fixture(self, **options):
        with tempfile.TemporaryDirectory(prefix="go-download-test-") as directory:
            peer = Fixture(Path(directory), self.cert, self.key, **options)
            try:
                yield peer
            finally:
                peer.close()

    def assert_cleaned(self, fixture):
        self.assertEqual(list(fixture.tmp.glob("pulse-go-install.*")), [])

    def test_fresh_private_download_ignores_shared_cache_and_saved_curl_settings(self):
        with self.fixture() as f:
            legacy = f.tmp / "go-install"
            legacy.mkdir(mode=0o777)
            cached = legacy / (f.version + ".linux-amd64.tar.gz")
            cached.write_bytes(PAYLOAD)
            result = f.run()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(len(f.requests), 2)
            downloads = f.records("curl")
            self.assertEqual(len(downloads), 2, "must fetch archive as well as digest")
            for download, budget in zip(downloads, ("300", "60")):
                args = download["args"]
                self.assertEqual(args[0], "--disable")
                for flag, value in (("--proto", "=https"), ("--proto-redir", "=https"),
                                    ("--noproxy", "*"), ("--connect-timeout", "5"),
                                    ("--max-time", budget)):
                    self.assertEqual(args[args.index(flag) + 1], value)
                for flag in ("--fail", "--silent", "--show-error", "--location"):
                    self.assertIn(flag, args)
                self.assertNotIn("--retry", args)
                self.assertEqual(download["parent_mode"], 0o700)
            for path in f.downloads():
                self.assertNotEqual(path.parent, legacy)
                self.assertFalse(path.exists())
            extraction = next(r for r in f.records("sudo") if r["args"][0] == "tar")
            self.assertEqual(extraction["archive_sha256"], DIGEST)
            self.assertEqual(extraction["archive_mode"], 0o600)
            self.assertEqual(cached.read_bytes(), PAYLOAD)
            self.assert_cleaned(f)

    def test_invalid_pins_do_no_download_or_privileged_work(self):
        for pin in (b"", b"../go", b"go1.26.9/../../etc", b" go1.26.9\n",
                    b"go1.26.9\ngo1.26.10\n", b"go1.26.9 -other", b"go1.26.9-rc.1"):
            with self.subTest(pin=pin), self.fixture(pin=pin) as f:
                result = f.run()
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(f.records("curl"), [])
                self.assertEqual(f.records("sudo"), [])
                self.assertEqual(f.requests, [])
                self.assert_cleaned(f)

    def test_missing_pin_uses_governed_default_without_writing_source(self):
        with self.fixture(pin=None) as f:
            result = f.run()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((f.script.parent / ".go-version").exists())
            self.assertEqual(f.requests, ["/" + f.version + ".linux-amd64.tar.gz",
                                          "/" + f.version + ".linux-amd64.tar.gz.sha256"])
            self.assertFalse(any(r["args"][0] == "tee" for r in f.records("sudo")))
            self.assert_cleaned(f)

    def test_crlf_pin_and_uppercase_digest_work_in_tmpdir_with_newline(self):
        version = re.search(r'^DEFAULT_VERSION="([^"]+)"$', INSTALLER.read_text(), re.M)[1]
        with self.fixture(pin=(version + "\r\n").encode(),
                          checksum=(DIGEST.upper() + "\r\n").encode()) as f:
            f.tmp = f.root / "temporary files\nline two"
            f.tmp.mkdir()
            f.env["TMPDIR"] = str(f.tmp)
            result = f.run()
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assert_cleaned(f)

    def test_untrusted_certificate_cannot_be_accepted_by_saved_insecure_option(self):
        with self.fixture(trusted=False) as f:
            result = f.run()
            self.assertEqual(result.returncode, 60, result.stderr)
            self.assertEqual(len(f.records("curl")), 1)
            self.assertEqual(f.requests, [])
            self.assertEqual(f.records("sudo"), [])
            self.assert_cleaned(f)

    def test_downgrade_partial_response_and_http_errors_stop_without_replay(self):
        for mode, exit_code, requests in (("redirect", 1, 1), ("partial", 18, 1),
                                          ("archive-error", 22, 1), ("digest-error", 22, 2)):
            with self.subTest(mode=mode), self.fixture(mode=mode) as f:
                result = f.run()
                self.assertEqual(result.returncode, exit_code, result.stderr)
                self.assertEqual(len(f.requests), requests)
                self.assertEqual(len(f.records("curl")), requests)
                self.assertEqual(f.records("sudo"), [])
                self.assert_cleaned(f)

    def test_checksum_mismatch_and_malformed_responses_stop_before_sudo(self):
        for checksum in (b"0" * 64, b"", b"x" * 64, b"1" * 63, b"1" * 65,
                         (DIGEST + " archive\n").encode(),
                         (DIGEST[:32] + "\n" + DIGEST[32:]).encode(),
                         (DIGEST + "\n" + DIGEST + "\n").encode()):
            with self.subTest(checksum=checksum), self.fixture(checksum=checksum) as f:
                result = f.run()
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(len(f.requests), 2)
                self.assertEqual(f.records("sudo"), [])
                self.assert_cleaned(f)

    def test_allocation_failure_does_not_create_parent_or_reach_network(self):
        with self.fixture() as f:
            missing = f.root / "absent/parent"
            f.env["TMPDIR"] = str(missing)
            result = f.run()
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(missing.exists())
            self.assertEqual(f.requests, [])
            self.assertEqual(f.records("sudo"), [])

    def test_privileged_command_failure_preserves_exit_and_removes_downloads(self):
        with self.fixture(sudo_exit=42) as f:
            result = f.run()
            self.assertEqual(result.returncode, 42)
            self.assertEqual(len(f.requests), 2)
            self.assertEqual(len(f.records("sudo")), 1)
            self.assert_cleaned(f)

    def test_concurrent_downloads_do_not_share_archive_or_checksum_paths(self):
        with self.fixture() as f:
            with ThreadPoolExecutor(max_workers=2) as pool:
                results = list(pool.map(lambda _: f.run(), range(2)))
            for result in results:
                self.assertEqual(result.returncode, 0, result.stderr)
            downloads = f.downloads()
            self.assertEqual(len(downloads), 4)
            self.assertEqual(len(set(downloads)), 4)
            self.assertEqual(len({path.parent for path in downloads}), 2)
            self.assertEqual(len(f.requests), 4)
            self.assert_cleaned(f)

    def test_term_removes_private_partial_download_without_privileged_work(self):
        with self.fixture(mode="stall") as f:
            process = subprocess.Popen(["bash", str(f.script)], env=f.env,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                       text=True, start_new_session=True)
            try:
                self.assertTrue(f.started.wait(5), "download did not start")
                os.killpg(process.pid, signal.SIGTERM)
                _, stderr = process.communicate(timeout=10)
                self.assertEqual(process.returncode, 143, stderr)
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.communicate(timeout=5)
            self.assertEqual(f.records("sudo"), [])
            self.assert_cleaned(f)

    def test_real_curl_helper_enforces_its_total_budget(self):
        with self.fixture(mode="stall") as f:
            helper = re.search(r'(?ms)^download_go_asset\(\) \(\n.*?^\)\n', INSTALLER.read_text())[0]
            script = ('DOWNLOAD_ROOT="$1"; ' + helper +
                      '\ndownload_go_asset archive "$2" 0.2\n')
            result = subprocess.run(["bash", "-c", script, "fixture", f.origin,
                                     str(f.tmp / "partial")], env=f.env,
                                    capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 28, result.stderr)
            self.assertEqual(len(f.records("curl")), 1)
            self.assertEqual(len(f.requests), 1)


if __name__ == "__main__":
    unittest.main()
