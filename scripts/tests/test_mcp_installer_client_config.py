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
# Large enough for the real install(1) file-size-limit control to fail partway.
CANDIDATE = b"verified MCP fixture; never executed\n" * 2048


def checked(argv):
    return subprocess.run(argv, check=True, capture_output=True, text=True)


class MCPInstallerClientConfig(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.resources = contextlib.ExitStack()
        cls.addClassCleanup(cls.resources.close)
        cls.root = Path(cls.resources.enter_context(tempfile.TemporaryDirectory()))
        for name in ("curl", "openssl", "ssh-keygen", "install", "mv", "mktemp"):
            if not shutil.which(name):
                raise RuntimeError(name + " is required; do not skip installer proof")
        cls.curl = shutil.which("curl")
        cls.install = shutil.which("install")
        cls.mv = shutil.which("mv")
        cls.mktemp = shutil.which("mktemp")
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

    def exercise(self, *, trusted=True, hostile=False, changed=None, missing=None,
                 fault=None, fresh=False, target_kind="file"):
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
                # Observe the real publication tools. Only the chosen failure
                # point is injected; no installer control or signed bytes change.
                (tools / "install").write_text(
                    "#!/usr/bin/env python3\nimport json,os,resource,signal,subprocess,sys\n"
                    "from pathlib import Path\n"
                    "args=sys.argv[1:]\n"
                    "with open(os.environ['PUBLICATION_CALLS'],'a') as f:\n"
                    " f.write(json.dumps({'tool':'install','args':args})+'\\n')\n"
                    "if os.environ.get('FAULT')=='copy-limit':\n"
                    " resource.setrlimit(resource.RLIMIT_CORE,(0,0))\n"
                    " resource.setrlimit(resource.RLIMIT_FSIZE,(1024,1024))\n"
                    "if os.environ.get('FAULT')=='term-after-copy':\n"
                    f" p=subprocess.run([{self.install!r}]+args)\n"
                    " if p.returncode:sys.exit(p.returncode)\n"
                    " os.kill(os.getppid(),signal.SIGTERM)\n"
                    " sys.exit(0)\n"
                    f"os.execv({self.install!r},[{self.install!r}]+args)\n")
                (tools / "mv").write_text(
                    "#!/usr/bin/env python3\nimport hashlib,json,os,sys\n"
                    "from pathlib import Path\n"
                    "args=sys.argv[1:];target=Path(os.environ['PULSE_MCP_BIN_DIR'])/'pulse-mcp'\n"
                    "prepared=Path(args[-2])\n"
                    "entry={'tool':'mv','args':args,'stage_mode':prepared.parent.stat().st_mode&0o777,\n"
                    " 'prepared_sha256':hashlib.sha256(prepared.read_bytes()).hexdigest(),\n"
                    " 'live_sha256':hashlib.sha256(target.read_bytes()).hexdigest() if target.is_file() else None}\n"
                    "with open(os.environ['PUBLICATION_CALLS'],'a') as f:f.write(json.dumps(entry)+'\\n')\n"
                    "if os.environ.get('FAULT')=='rename-refused':sys.exit(74)\n"
                    "if os.environ.get('FAULT')=='directory-before-rename':\n"
                    " target.rename(target.parent.parent/'saved-previous');target.mkdir()\n"
                    f"os.execv({self.mv!r},[{self.mv!r}]+args)\n")
                (tools / "mktemp").write_text(
                    "#!/usr/bin/env python3\nimport os,sys\nfrom pathlib import Path\n"
                    "counter=Path(os.environ['MKTEMP_COUNTER'])\n"
                    "n=int(counter.read_text())+1 if counter.exists() else 1\n"
                    "counter.write_text(str(n))\n"
                    "if os.environ.get('FAULT')=='third-temp-refused' and n==3:sys.exit(73)\n"
                    f"os.execv({self.mktemp!r},[{self.mktemp!r}]+sys.argv[1:])\n")
                for tool in ("install", "mv", "mktemp"):
                    (tools / tool).chmod(0o700)
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
                victim = root / "victim"
                if target_kind.startswith("symlink"):
                    if target_kind == "symlink-directory":
                        victim.mkdir()
                    else:
                        victim.write_bytes(PREVIOUS)
                    target.symlink_to(victim)
                elif target_kind == "directory":
                    target.mkdir()
                elif not fresh:
                    target.write_bytes(PREVIOUS)
                    target.chmod(0o755)
                old_handle = target.open("rb") if target.is_file() else contextlib.nullcontext(None)
                env = {k: v for k, v in os.environ.items()
                       if k not in ("CURL_CA_BUNDLE", "SSL_CERT_FILE", "SSL_CERT_DIR")}
                env.update({"PATH": str(tools) + os.pathsep + os.environ["PATH"],
                            "HOME": str(root), "CURL_HOME": str(root), "XDG_CONFIG_HOME": str(root),
                            "TMPDIR": str(root), "CALLS": str(calls), "FIXTURE_URL": url,
                            "PULSE_MCP_VERSION": "v-fixture", "PULSE_MCP_REPO": "rcourtman/Pulse",
                            "PULSE_MCP_BIN_DIR": str(installed), "NO_PROXY": "*", "no_proxy": "*"})
                env.update({"FAULT": fault or "", "PUBLICATION_CALLS": str(root / "publication.jsonl"),
                            "MKTEMP_COUNTER": str(root / "mktemp-count")})
                if trusted:
                    env["CURL_CA_BUNDLE"] = str(self.root / "tls.crt")
                with old_handle as handle:
                    result = subprocess.run(["bash", str(script)], env=env, text=True,
                                            capture_output=True, timeout=25)
                    previous_handle_bytes = handle.read() if handle else None
                publication = root / "publication.jsonl"
                observed = {"exit": result.returncode, "output": result.stdout + result.stderr,
                        "requests": requests, "trace": trace.exists(),
                        "installed": target.read_bytes() if target.is_file() else None,
                        "target_is_symlink": target.is_symlink(),
                        "target_is_directory": target.is_dir(),
                        "target_children": sorted(p.name for p in target.iterdir()) if target.is_dir() else [],
                        "victim": victim.read_bytes() if victim.is_file() else None,
                        "victim_children": list(victim.iterdir()) if victim.is_dir() else [],
                        "previous_handle_bytes": previous_handle_bytes,
                        "saved_previous": (root / "saved-previous").read_bytes()
                            if (root / "saved-previous").is_file() else None,
                        "mode": target.stat().st_mode & 0o777 if target.is_file() else None,
                        "publication": [json.loads(l) for l in publication.read_text().splitlines()]
                            if publication.exists() else [],
                        "staging": sorted(p.name for p in installed.glob(".pulse-mcp.*")),
                        "calls": [json.loads(line) for line in calls.read_text().splitlines()],
                        "temporary_inputs": sorted(p.name for p in root.glob("pulse-mcp*"))}
                print("MCP_TRANSPORT_CASE " + json.dumps({
                    "trusted": trusted, "hostile": hostile, "changed": changed, "missing": missing,
                    "fault": fault, "fresh": fresh, "target_kind": target_kind,
                    **{key: observed[key] for key in ("exit", "requests", "trace", "calls", "temporary_inputs",
                        "publication", "staging", "target_is_symlink", "target_is_directory", "target_children")},
                    "installed": "candidate" if observed["installed"] == CANDIDATE else (
                        "previous" if observed["installed"] == PREVIOUS else "other-or-absent"),
                    "installed_bytes": len(observed["installed"]) if observed["installed"] is not None else None,
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
        self.assertEqual(result["staging"], [], "publication staging leaked")

    def test_publication_preserves_previous_until_atomic_rename(self):
        for fresh in (False, True):
            with self.subTest(fresh=fresh):
                r = self.exercise(fresh=fresh)
                self.assertEqual(r["exit"], 0, r["output"])
                self.assertEqual(r["installed"], CANDIDATE)
                self.assertEqual(r["mode"], 0o755)
                if not fresh:
                    self.assertEqual(r["previous_handle_bytes"], PREVIOUS)
                renames = [c for c in r["publication"] if c["tool"] == "mv"]
                self.assertEqual(len(renames), 1, "verified file was not atomically published")
                self.assertEqual(renames[0]["live_sha256"], None if fresh else hashlib.sha256(PREVIOUS).hexdigest())
                self.assertEqual(renames[0]["prepared_sha256"], hashlib.sha256(CANDIDATE).hexdigest())
                self.assertEqual(renames[0]["stage_mode"], 0o700)
                self.assert_clean_transport(r)

    def test_copy_failure_preserves_previous_and_cleans_staging(self):
        r = self.exercise(fault="copy-limit")
        self.assertNotEqual(r["exit"], 0, r["output"])
        self.assertIn("sha256 verified", r["output"], "failure happened before signed admission")
        self.assertEqual(r["installed"], PREVIOUS, "failed real install destroyed previous executable")
        self.assertNotIn("installed:", r["output"])
        self.assert_clean_transport(r)

    def test_termination_before_publication_preserves_previous(self):
        r = self.exercise(fault="term-after-copy")
        self.assertIn(r["exit"], (-15, 143), r["output"])
        self.assertIn("sha256 verified", r["output"])
        self.assertEqual(r["installed"], PREVIOUS, "interrupted install changed live executable")
        self.assertNotIn("installed:", r["output"])
        self.assert_clean_transport(r)

    def test_rename_refusal_preserves_previous(self):
        r = self.exercise(fault="rename-refused")
        self.assertNotEqual(r["exit"], 0, r["output"])
        self.assertEqual(r["installed"], PREVIOUS)
        self.assertNotIn("installed:", r["output"])
        self.assert_clean_transport(r)

    def test_directory_destination_is_not_reported_as_installed(self):
        for fault in (None, "directory-before-rename"):
            with self.subTest(fault=fault):
                r = self.exercise(fault=fault, target_kind="directory" if fault is None else "file")
                self.assertNotEqual(r["exit"], 0, r["output"])
                self.assertTrue(r["target_is_directory"])
                self.assertEqual(r["target_children"], [], "installer copied inside destination directory")
                if fault:
                    self.assertEqual(r["saved_previous"], PREVIOUS)
                self.assertNotIn("installed:", r["output"])
                self.assert_clean_transport(r)

    def test_symlink_destinations_are_replaced_not_followed(self):
        for kind in ("symlink-file", "symlink-directory"):
            with self.subTest(kind=kind):
                r = self.exercise(target_kind=kind)
                self.assertEqual(r["exit"], 0, r["output"])
                self.assertFalse(r["target_is_symlink"])
                self.assertEqual(r["installed"], CANDIDATE)
                self.assertEqual(r["victim_children"], [])
                if kind == "symlink-file":
                    self.assertEqual(r["victim"], PREVIOUS)
                self.assert_clean_transport(r)

    def test_temporary_allocation_failure_cleans_previous_inputs(self):
        r = self.exercise(fault="third-temp-refused")
        self.assertNotEqual(r["exit"], 0, r["output"])
        self.assertEqual(r["installed"], PREVIOUS)
        self.assert_clean_transport(r)

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
