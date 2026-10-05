#!/usr/bin/env python3
"""Execute the PBS guide's recipes with synthetic tokens and guest-local TLS.

No real PBS/Pulse instance, installer, service or credential is used. Run the
loopback checks with pulse-worker-source-proof, not on the worker host.
"""

from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import ssl
import stat
import subprocess
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[2]
DOC = ROOT / "docs/PBS.md"
PBS_TOKEN = "synthetic-pbs-secret"
AGENT_TOKEN = "synthetic-agent-secret"
AUTH_HEADER = f"PBSAPIToken=pulse-monitor@pbs!pulse-token:{PBS_TOKEN}"
PRIVATE_RESPONSE = json.dumps({
    "data": [], "token": PBS_TOKEN, "resource": "private-fixture-datastore",
}).encode() + b"\n"
DOWNLOAD_SELECTOR = 'installer_file="$config_dir/pbs-agent-install.sh"'


def blocks():
    # Nested list-item fences have two leading spaces, which are not shell
    # input when copied from the rendered guide.
    text = re.sub(r"^  ", "", DOC.read_text(), flags=re.MULTILINE)
    return re.findall(r"```bash\n(.*?)```", text, re.DOTALL)


def recipe(needle, *, also=None):
    matches = [block for block in blocks() if needle in block and (also is None or also in block)]
    if len(matches) != 1:
        raise AssertionError(f"expected one recipe containing {needle!r}")
    return matches[0]


def fixture_environment(home):
    env = dict(os.environ, HOME=str(home))
    # Keep all requests guest-local even if the proof runtime has proxy vars.
    for key in list(env):
        if key.lower().endswith("_proxy"):
            del env[key]
    return env


def certificate(directory, name):
    certificate = directory / f"{name}.pem"
    key = directory / f"{name}.key"
    subprocess.run([
        "openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
        "-keyout", str(key), "-out", str(certificate), "-days", "1",
        "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost",
    ], check=True, capture_output=True)
    key.chmod(0o600)
    return certificate, key


def server_context(cert, key):
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    context.load_cert_chain(str(cert), str(key))
    return context


@contextmanager
def server(cert, key, status=200, installer=None, installer_path="/install.sh", response=b'{"data":[]}\n'):
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.path, dict(self.headers)))
            self.send_response(status)
            if status == 302:
                self.send_header("Location", "/should-not-receive-credentials")
            self.end_headers()
            body = installer if self.path == installer_path else response
            self.wfile.write(body or b"fixture error\n")

        def log_message(self, *_args):
            pass

    http = HTTPServer(("127.0.0.1", 0), Handler)
    context = server_context(cert, key)
    http.socket = context.wrap_socket(http.socket, server_side=True)
    thread = threading.Thread(target=http.serve_forever, daemon=True)
    thread.start()
    try:
        yield http.server_address[1], requests
    finally:
        http.shutdown()
        http.server_close()
        thread.join(timeout=5)


def private_server(cert, key, status=200, installer=None, installer_path="/install.sh"):
    # The agent guide imports the shared TLS fixture. Only PBS diagnostic
    # checks use the credential-bearing synthetic response sensitivity marker.
    return server(cert, key, status, installer, installer_path, response=PRIVATE_RESPONSE)


class PBSDocsTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory()
        cls.cert, cls.key = certificate(Path(cls.temporary.name), "server")
        cls.other_cert, _ = certificate(Path(cls.temporary.name), "other")

    @classmethod
    def tearDownClass(cls):
        cls.temporary.cleanup()

    def test_tls_fixture_explicitly_rejects_legacy_protocols(self):
        self.assertEqual(server_context(self.cert, self.key).minimum_version,
                         ssl.TLSVersion.TLSv1_2)

    def test_shipped_copy_and_safe_scope(self):
        self.assertEqual(DOC.read_bytes(), (ROOT / "frontend-modern/public/docs/PBS.md").read_bytes())
        text = DOC.read_text()
        shell = "\n".join(blocks())
        self.assertNotRegex(shell, r"Authorization:|PBSAPIToken=|PULSE_SETUP_TOKEN=|--token(?:\s|=)|--insecure|curl\s+-[^\s]*k")
        self.assertNotIn("/api2/json/version", shell)
        self.assertNotIn("disable SSL verification", text)
        for phrase in ("both the user and the token", "Manual Token Setup",
                       "not the **PBS** token", "independently verified",
                       "An agent is not", "empty list is not proof",
                       "Do not delete backups", "Do not clear History"):
            self.assertIn(phrase, text)

    def test_history_checks_cover_batched_and_single_metric_requests(self):
        history = DOC.read_text().split("### PBS is connected but History stays empty\n")[1].split("\n### ")[0]
        for phrase in ("without\n   `metric`", "do not wait for a separate `metric=cpu` request",
                       "`metrics: {}`", "`metrics.cpu` array", "CPU is absent",
                       "`points` array for that single metric", "A measured zero is a\n   sample",
                       "Do not edit or replay", "Do not share a HAR", "enable\nDebug logging"):
            with self.subTest(phrase=phrase):
                self.assertIn(phrase, history)
        self.assertNotIn("completed request with `metric=cpu`", history)
        self.assertIn("does\n   not prove there is no data under another target", history)

    def test_every_documented_shell_recipe_parses(self):
        for command in blocks():
            with self.subTest(command=command):
                subprocess.run(["bash", "-n", "-c", command], check=True, capture_output=True)

    def test_new_setup_grants_audit_to_user_and_token(self):
        command = recipe("proxmox-backup-manager user create")
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            tool = home / "proxmox-backup-manager"
            tool.write_text(
                "#!/usr/bin/env python3\nimport json, os, sys\n"
                "with open(os.environ['CALLS'], 'a') as f: f.write(json.dumps(sys.argv[1:]) + '\\n')\n"
            )
            tool.chmod(0o700)
            env = fixture_environment(home)
            env.update(PATH=f"{home}:{os.environ['PATH']}", CALLS=str(home / "calls.jsonl"))
            subprocess.run(["bash", "-eu", "-c", command], env=env, check=True)
            calls = [json.loads(line) for line in (home / "calls.jsonl").read_text().splitlines()]
            self.assertEqual(calls, [
                ["user", "create", "pulse-monitor@pbs", "--comment", "Pulse monitoring"],
                ["user", "generate-token", "pulse-monitor@pbs", "pulse-token"],
                ["acl", "update", "/", "Audit", "--auth-id", "pulse-monitor@pbs"],
                ["acl", "update", "/", "Audit", "--auth-id", "pulse-monitor@pbs!pulse-token"],
            ])
        # Same permission shapes as the owning implementation, not a claim of
        # running proxmox-backup-manager on a real appliance.
        source = (ROOT / "internal/api/configapi/setup_script_render.go").read_text()
        self.assertIn("proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs", source)
        self.assertIn('proxmox-backup-manager acl update / Audit --auth-id "$PULSE_TOKEN_ID"', source)

    def test_private_preparation_preserves_existing_content(self):
        for name in ("pbs-agent-token", "pbs-header"):
            command = recipe(f'credential_file="$config_dir/{name}"', also='vi "$credential_file"')
            self.assertIn('vi "$credential_file"', command)
            with self.subTest(name=name), tempfile.TemporaryDirectory() as temporary:
                home = Path(temporary)
                tools = home / "tools"
                tools.mkdir()
                editor = tools / "vi"
                editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ -f "$1" ]\n')
                editor.chmod(0o700)
                env = fixture_environment(home)
                env["PATH"] = f"{tools}:{os.environ['PATH']}"
                path = home / ".config/pulse" / name
                for existing in (False, True):
                    if existing:
                        path.write_text("synthetic-existing-content\n")
                        path.chmod(0o644)
                        path.parent.chmod(0o755)
                    subprocess.run(["bash", "-eu", "-c", command], env=env, check=True)
                    self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
                    self.assertEqual(stat.S_IMODE(path.parent.stat().st_mode), 0o700)
                    self.assertEqual(path.read_text(), "synthetic-existing-content\n" if existing else "")

    def run_curl(self, home, command, port, ca=None, hostname="localhost"):
        private = home / ".config/pulse"
        private.mkdir(parents=True, exist_ok=True, mode=0o755)
        header = private / "pbs-header"
        header.write_text(f"Authorization: {AUTH_HEADER}\n")
        header.chmod(0o644)
        earlier = private / "earlier-pbs-response"
        earlier.write_bytes(b"earlier private result\n")
        earlier.chmod(0o600)
        real_curl = shutil.which("curl")
        self.assertIsNotNone(real_curl)
        tools = home / "tools"
        tools.mkdir(exist_ok=True)
        recorder = tools / "curl"
        recorder.write_text(
            "#!/usr/bin/env python3\nimport json, os, sys\nfrom pathlib import Path\n"
            "with Path(os.environ['ARGV_RECEIPT']).open('a') as f: f.write(json.dumps(sys.argv[1:]) + '\\n')\n"
            "os.execv(os.environ['REAL_CURL'], [os.environ['REAL_CURL'], *sys.argv[1:]])\n"
        )
        recorder.chmod(0o700)
        receipt = home / "argv.json"
        receipt.write_text("")
        trace = home / "curl-trace.txt"
        (home / ".curlrc").write_text(
            f'header = "X-Curlrc-Injected: yes"\nverbose\ninsecure\nlocation\ntrace-ascii = "{trace}"\n'
        )
        env = fixture_environment(home)
        env.update(PATH=f"{tools}:{os.environ['PATH']}", REAL_CURL=real_curl, ARGV_RECEIPT=str(receipt),
                   CURL_HOME=str(home), XDG_CONFIG_HOME=str(home / ".config"))
        command = command.replace("https://pbs.example.com:8007", f"https://{hostname}:{port}")
        command = command.replace("https://pulse.example.com", f"https://{hostname}:{port}")
        if ca is not None:
            shutil.copyfile(ca, private / "pbs-ca.pem")
            # This is the guide's optional, independently verified CA-file form.
            self.assertEqual(command.count("curl --disable "), 1)
            command = command.replace("curl --disable ", 'curl --disable --cacert "$HOME/.config/pulse/pbs-ca.pem" ', 1)
        result = subprocess.run(["bash", "-eu", "-c", command], env=env, capture_output=True, timeout=20)
        calls = [json.loads(line) for line in receipt.read_text().splitlines()]
        self.assertEqual(len(calls), 1, "a copied recipe must invoke curl only once")
        argv = calls[0]
        self.assertEqual(argv[0], "--disable", "ignore local curl settings before other options")
        for secret in (PBS_TOKEN, AGENT_TOKEN, "private-fixture-datastore"):
            self.assertNotIn(secret, " ".join(argv))
            self.assertNotIn(secret.encode(), result.stdout + result.stderr, "private response leaked")
        self.assertNotIn("--insecure", argv)
        self.assertNotIn("-k", argv)
        self.assertFalse(trace.exists(), "curl defaults must not enable credential-bearing traces")
        self.assertEqual(earlier.read_bytes(), b"earlier private result\n")
        return result, argv

    def assert_private_response(self, home, result, expected=PRIVATE_RESPONSE):
        private = home / ".config/pulse"
        responses = list(private.glob("pbs-response.*"))
        self.assertEqual(len(responses), 1)
        response = responses[0]
        self.assertIn(str(response).encode(), result.stdout)
        self.assertEqual(response.read_bytes(), expected)
        for path, mode in ((private, 0o700), (private / "pbs-header", 0o600), (response, 0o600)):
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), mode)

    def test_datastore_request_transmits_auth_only_in_header(self):
        with tempfile.TemporaryDirectory() as temporary, private_server(self.cert, self.key) as (port, requests):
            result, argv = self.run_curl(Path(temporary), recipe("--fail-with-body"), port, self.cert)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertIn("--max-time", argv)
            self.assertIn("--connect-timeout", argv)
            self.assertIn("@" + str(Path(temporary) / ".config/pulse/pbs-header"), argv)
            self.assertEqual(requests, [("/api2/json/admin/datastore", {
                **requests[0][1], "Authorization": AUTH_HEADER,
            })])
            self.assertNotIn("X-Curlrc-Injected", requests[0][1])
            self.assert_private_response(Path(temporary), result)

    def test_auth_errors_retain_body_and_fail(self):
        for status in (401, 403):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as temporary:
                with private_server(self.cert, self.key, status) as (port, requests):
                    result, _ = self.run_curl(Path(temporary), recipe("--fail-with-body"), port, self.cert)
                    self.assertEqual(result.returncode, 22, result.stderr.decode())
                    self.assertEqual(len(requests), 1)
                    self.assert_private_response(Path(temporary), result)

    def test_redirect_is_not_followed_or_successful(self):
        with tempfile.TemporaryDirectory() as temporary, private_server(self.cert, self.key, 302) as (port, requests):
            result, _ = self.run_curl(Path(temporary), recipe("--fail-with-body"), port, self.cert)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(len(requests), 1)
            self.assert_private_response(Path(temporary), result)

    def test_tls_rejects_untrusted_wrong_certificate_and_wrong_hostname(self):
        for ca, hostname in ((None, "localhost"), (self.other_cert, "localhost"), (self.cert, "127.0.0.1")):
            with self.subTest(ca=ca, hostname=hostname), tempfile.TemporaryDirectory() as temporary:
                with private_server(self.cert, self.key) as (port, requests):
                    result, _ = self.run_curl(Path(temporary), recipe("--fail-with-body"), port, ca, hostname)
                    self.assertEqual(result.returncode, 60, result.stderr.decode())
                    # Authentication is never sent over an unverified channel.
                    self.assertEqual(requests, [])
                    self.assert_private_response(Path(temporary), result, b"")

    def test_download_and_agent_file_handoff(self):
        # The downloaded fixture deliberately cannot install or change a
        # service. It verifies only the documented argument/file handoff.
        installer = b'''#!/bin/bash
python3 - "$@" <<'PY'
import json, os, stat, sys
from pathlib import Path
args = sys.argv[1:]
path = Path(args[args.index('--token-file') + 1])
assert stat.S_IMODE(path.stat().st_mode) == 0o600
assert path.read_text().strip() == os.environ['EXPECTED_AGENT_TOKEN']
assert os.environ['EXPECTED_AGENT_TOKEN'] not in ' '.join(args)
Path(os.environ['AGENT_RECEIPT']).write_text(json.dumps(args))
PY
'''
        with tempfile.TemporaryDirectory() as temporary, private_server(self.cert, self.key, installer=installer) as (port, requests):
            home = Path(temporary)
            result, argv = self.run_curl(home, recipe(DOWNLOAD_SELECTOR), port, self.cert)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertEqual(len(requests), 1)
            self.assertEqual(requests[0][0], "/install.sh")
            self.assertNotIn("Authorization", requests[0][1])
            self.assertNotIn("X-Curlrc-Injected", requests[0][1])
            self.assertNotIn("--header", argv)
            saved = home / ".config/pulse/pbs-agent-install.sh"
            self.assertEqual(saved.read_bytes(), installer)
            self.assertEqual(stat.S_IMODE(saved.stat().st_mode), 0o600)
            self.assertEqual(list(saved.parent.glob("pbs-agent-download.*")), [])
            token = home / ".config/pulse/pbs-agent-token"
            token.write_text(AGENT_TOKEN + "\n")
            token.chmod(0o600)
            command = recipe('bash "$HOME/.config/pulse/pbs-agent-install.sh"')
            env = fixture_environment(home)
            env.update(EXPECTED_AGENT_TOKEN=AGENT_TOKEN, AGENT_RECEIPT=str(home / "agent-argv.json"))
            result = subprocess.run(["bash", "-eu", "-c", command], env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertEqual(json.loads((home / "agent-argv.json").read_text()), [
                "--url", "https://pulse.example.com", "--token-file", str(token),
                "--enable-proxmox", "--proxmox-type", "pbs", "--enable-docker=false",
            ])
            self.assertNotIn(AGENT_TOKEN.encode(), result.stdout + result.stderr)
        actual = (ROOT / "scripts/install.sh").read_text()
        self.assertIn('--token-file) TOKEN_FILE_PATH="$2"; shift 2 ;;', actual)
        self.assertIn('read_collector_token_file_safely "$TOKEN_FILE_PATH" true', actual)

    def test_failed_installer_download_is_nonzero(self):
        for status, exitcode in ((403, 22), (302, 1), (204, 1)):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as temporary:
                with private_server(self.cert, self.key, status) as (port, requests):
                    home = Path(temporary)
                    result, _ = self.run_curl(home, recipe(DOWNLOAD_SELECTOR), port, self.cert)
                    self.assertEqual(result.returncode, exitcode, result.stderr.decode())
                    self.assertEqual(len(requests), 1)
                    self.assertFalse((home / ".config/pulse/pbs-agent-install.sh").exists())

    def test_private_response_assertion_rejects_console_output_regression(self):
        command = recipe("--fail-with-body")
        self.assertEqual(command.count('--output "$result_file" '), 1)
        unsafe = command.replace('--output "$result_file" ', "")
        with tempfile.TemporaryDirectory() as temporary, private_server(self.cert, self.key) as (port, requests):
            with self.assertRaisesRegex(AssertionError, "private response leaked"):
                self.run_curl(Path(temporary), unsafe, port, self.cert)
            self.assertEqual(len(requests), 1)


if __name__ == "__main__":
    unittest.main()
