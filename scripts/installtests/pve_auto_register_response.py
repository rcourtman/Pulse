#!/usr/bin/env python3
"""Execute the actual PVE installer caller against private registration replies.

curl and every Python invocation are real and observed; PVE commands, the API
trust preflight and token smoke check are closed doubles. No installed state,
external endpoint, real credential, root operation or returned command is used.
The separate transport suite exercises the actual PVE trust/smoke helpers.
"""

import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from urllib.parse import quote

import pve_auto_register_transport as installer

# Explicit exact-parent input supports the adverse comparison in the offline VM.
if os.environ.get("PULSE_PVE_REGISTRATION_SOURCE"):
    installer.SOURCE = Path(os.environ["PULSE_PVE_REGISTRATION_SOURCE"]).read_text()
function = installer.function


TOKEN = "fixture-only-pve-secret-4897"
SETUP_TOKEN = "fixture-only-setup-secret-3629"
PRIVATE_TEXT = "fixture-only-response-diagnostic-4172"
HOST = "https://pve-fixture.invalid:8006"
CANONICAL_NAME = "fixture-pve"


class RegistrationPeer:
    def __init__(self, response, *, legacy=False):
        self.requests = []
        self.errors = []
        peer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                try:
                    body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                    peer.requests.append((self.path, body))
                    if self.path == "/api/setup-script-url":
                        if body != {"type": "pve", "host": HOST, "backupPerms": True}:
                            raise ValueError("unexpected setup request")
                        url = (peer.url + "/api/setup-script?backup_perms=true&host="
                               + quote(HOST, safe="") + "&pulse_url="
                               + quote(peer.url, safe="") + "&type=pve")
                        command = ('if [ "$(id -u)" -eq 0 ]; then :; '
                                   'elif command -v sudo >/dev/null 2>&1; then :; fi; ' + url)
                        if legacy:
                            download = url.replace("&type=pve", "&setup_token=" + SETUP_TOKEN + "&type=pve")
                            with_token = "PULSE_SETUP_TOKEN=" + SETUP_TOKEN + " " + command
                            without_token = command
                        else:
                            download = url
                            with_token = without_token = "PULSE_SETUP_TOKEN_FILE=/private/fixture " + command
                        payload = json.dumps(dict(
                            setupToken=SETUP_TOKEN, tokenHint="fixture-hint", type="pve", host=HOST,
                            url=url, downloadURL=download, scriptFileName="pulse-setup-pve.sh",
                            command=with_token, commandWithEnv=with_token, commandWithoutEnv=without_token,
                            expires=int(time.time()) + 600))
                    elif self.path == "/api/auto-register":
                        expected = dict(type="pve", host=HOST, tokenId="pulse-monitor@pve!pulse-127-0-0-1",
                                        tokenValue=TOKEN, serverName=CANONICAL_NAME,
                                        authToken=SETUP_TOKEN, source="script")
                        if body != expected:
                            raise ValueError("unexpected registration request")
                        payload = response(body)
                    else:
                        raise ValueError("unexpected path")
                    encoded = payload.encode()
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(encoded)))
                    self.end_headers()
                    self.wfile.write(encoded)
                except Exception as error:
                    peer.errors.append(type(error).__name__)
                    self.send_error(500)

        self.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.server.daemon_threads = True
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={"poll_interval": 0.01})
        self.thread.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)
        if self.thread.is_alive():
            raise AssertionError("registration peer did not stop")


def confirmation(request, **changes):
    reply = dict(status="success", action="use_token", type="pve", source="script",
                 host=request["host"], tokenId=request["tokenId"], tokenValue=request["tokenValue"],
                 nodeId=CANONICAL_NAME, nodeName=CANONICAL_NAME,
                 message=PRIVATE_TEXT + " " + TOKEN + " " + SETUP_TOKEN)
    reply.update(changes)
    return json.dumps(reply)


class RegistrationResponse(unittest.TestCase):
    def exercise(self, response=confirmation, *, accepted=False, name=CANONICAL_NAME, legacy=False):
        peer = RegistrationPeer(response, legacy=legacy)
        try:
            with tempfile.TemporaryDirectory(prefix="pulse-private-confirmation-") as directory:
                root = Path(directory)
                tools = root / "tools"
                tools.mkdir()
                # The private observation contains only deliberately synthetic
                # values, and is never printed as a failure diagnostic.
                observer = ("#!" + sys.executable + "\nimport json, os, sys\n"
                            "with open(os.environ['ARGV_LOG'], 'a') as out:\n"
                            "    out.write(json.dumps([os.path.basename(sys.argv[0]), *sys.argv[1:]]) + '\\n')\n"
                            "tool = os.environ['REAL_PYTHON'] if sys.argv[0].endswith('python3') else os.environ['REAL_CURL']\n"
                            "os.execv(tool, [tool, *sys.argv[1:]])\n")
                for name_of_tool in ("python3", "curl"):
                    path = tools / name_of_tool
                    path.write_text(observer)
                    path.chmod(0o700)
                (root / "token.json").write_text(json.dumps({"value": TOKEN}))
                (tools / "pveum").write_text(
                    '#!/bin/sh\nprintf "%s\\n" "$*" >> "$FIXTURE/pve-calls"\n'
                    'if [ "$1 $2 $3" = "user token add" ]; then cat "$FIXTURE/token.json"; '
                    'elif [ "$1 $2" = "role list" ]; then printf "Sys.Audit VM.GuestAgent.Audit VM.GuestAgent.FileRead\\n"; fi\n')
                (tools / "pveum").chmod(0o700)
                (tools / "hostname").write_text('#!/bin/sh\nprintf "fixture-pve\\n"\n')
                (tools / "hostname").chmod(0o700)
                source = "\n".join(function(name) for name in (
                    "extract_pve_token_value", "pve_token_already_exists_error",
                    "create_pve_auto_register_token", "auto_register_pve_node"))
                script = """set -euo pipefail
print_warn() { printf 'WARN=%s\n' "$*"; }
print_info() { printf 'INFO=%s\n' "$*"; }
print_success() { printf 'SUCCESS=%s\n' "$*"; }
preflight_pve_auto_register_api() { [[ "$1" == "$PULSE_PVE_HOST_URL" ]]; }
smoke_test_pve_auto_register_token() {
    [[ "$1" == "$PULSE_PVE_HOST_URL" && "$2" == pulse-monitor@pve!pulse-127-0-0-1 && -n "$3" ]]
}
IN_CONTAINER=false
AUTO_NODE_REGISTERED=false
AUTO_NODE_REGISTERED_NAME=""
AUTO_NODE_REGISTER_ERROR=""
""" + source + """
auto_register_pve_node 999 127.0.0.1 "$PULSE_FIXTURE_PORT"
printf 'RESULT=%s\nNAME=%s\nERROR=%s\n' "$AUTO_NODE_REGISTERED" "$AUTO_NODE_REGISTERED_NAME" "$AUTO_NODE_REGISTER_ERROR"
"""
                # Syntax failure must not look like a confidentiality refusal.
                subprocess.run(["bash", "-n"], input=script, text=True,
                               capture_output=True, check=True, timeout=3)
                env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                           HOME=str(root), CURL_HOME=str(root), FIXTURE=str(root),
                           REAL_PYTHON=sys.executable, REAL_CURL=shutil.which("curl"),
                           ARGV_LOG=str(root / "argv"), PULSE_PVE_HOST_URL=HOST,
                           PULSE_FIXTURE_PORT=str(peer.server.server_port), PULSE_AUTO_BACKUP_PERMS="true",
                           PULSE_SKIP_AUTO_NODE="false", NO_PROXY="*", no_proxy="*",
                           HTTP_PROXY="", http_proxy="", HTTPS_PROXY="", https_proxy="", ALL_PROXY="", all_proxy="")
                result = subprocess.run(["bash", "-c", script], env=env,
                                        capture_output=True, text=True, timeout=12)
                output = result.stdout + result.stderr
                args = [json.loads(line) for line in (root / "argv").read_text().splitlines()]
                observed = json.dumps(args)
                for secret in (TOKEN, SETUP_TOKEN, PRIVATE_TEXT):
                    self.assertFalse(secret in output, "fixture secret/private response entered installer output")
                    self.assertFalse(secret in observed, "fixture secret/private response entered child argv")
                self.assertNotIn("Traceback", output)
                self.assertEqual(result.returncode, 0, "optional registration must not abort installation")
                self.assertEqual(peer.errors, [])
                self.assertEqual([path for path, _ in peer.requests], ["/api/setup-script-url", "/api/auto-register"])
                curl_args = [argv[1:] for argv in args if argv[0] == "curl"]
                self.assertEqual(len(curl_args), 2)
                self.assertEqual(curl_args[1][-2:], ["-d", "@-"])
                python_args = [argv[1:] for argv in args if argv[0] == "python3"]
                self.assertGreaterEqual(len(python_args), 7)
                self.assertTrue(all(argv[0] == "-" for argv in python_args))
                self.assertEqual(python_args[-1], ["-"], "confirmation parser accepted a process argument")
                calls = (root / "pve-calls").read_text().splitlines()
                self.assertEqual(sum(call.startswith("user token add ") for call in calls), 1)
                self.assertFalse(any(call.startswith("user token remove ") for call in calls))
                self.assertIn("RESULT=" + str(accepted).lower() + "\n", output)
                if accepted:
                    self.assertIn("NAME=" + name + "\nERROR=\n", output)
                    self.assertIn("SUCCESS=Registered " + name + " with Pulse automatically", output)
                    self.assertNotIn("WARN=", output)
                else:
                    self.assertIn("NAME=\nERROR=auto-register unsuccessful\n", output)
                    self.assertIn("WARN=Pulse auto-registration returned an invalid confirmation; skipping automatic node registration", output)
                    self.assertNotIn("SUCCESS=", output)
        finally:
            peer.close()

    def test_valid_modern_and_legacy_confirmation_retains_registration_without_private_output(self):
        for legacy in (False, True):
            with self.subTest(legacy=legacy):
                self.exercise(accepted=True, legacy=legacy)

    def test_stored_disambiguated_and_unicode_names_are_preserved(self):
        for name in ("fixture-pve (2)", "pve-équipe (2)"):
            with self.subTest(name=name):
                self.exercise(lambda request: confirmation(request, nodeId=name, nodeName=name),
                              accepted=True, name=name)

    def test_each_success_identity_and_exact_token_binding_is_required(self):
        for field in ("status", "action", "type", "source", "host", "tokenId", "tokenValue", "nodeId", "nodeName"):
            for value in ("", "mismatch", None, 1, True, [], {}):
                with self.subTest(field=field, kind=type(value).__name__):
                    self.exercise(lambda request: confirmation(request, **{field: value}))

    def test_each_required_field_must_be_present(self):
        for field in ("status", "action", "type", "source", "host", "tokenId", "tokenValue", "nodeId", "nodeName"):
            def missing(request):
                reply = json.loads(confirmation(request))
                del reply[field]
                return json.dumps(reply)
            with self.subTest(field=field):
                self.exercise(missing)

    def test_control_characters_and_secret_echo_names_are_not_terminal_output(self):
        for name in (" ", "fixture\tname", "fixture\nname", "fixture\rname", "fixture\x1b[31mname",
                     "fixture\x00name", "fixture\x7fname", "echo-" + TOKEN, "echo-" + SETUP_TOKEN):
            with self.subTest(kind="untrusted-name"):
                self.exercise(lambda request: confirmation(request, nodeId=name, nodeName=name))

    def test_non_object_replies_are_rejected_without_interpreter_diagnostics(self):
        for response in ("null", "[]", "true", "42", json.dumps(PRIVATE_TEXT + TOKEN + SETUP_TOKEN)):
            with self.subTest(kind=response[:4]):
                self.exercise(lambda _: response)

    def test_malformed_replies_never_become_raw_mismatch_diagnostics(self):
        for response in ("", PRIVATE_TEXT + TOKEN + SETUP_TOKEN, '{"tokenValue": "' + TOKEN,
                         confirmation(dict(host=HOST, tokenId="fixture", tokenValue=TOKEN)) + "\n{}",
                         "[" * 1500 + "]" * 1500):
            with self.subTest(kind="malformed"):
                self.exercise(lambda _: response)

    def test_unexpected_nested_and_multiline_message_is_not_printed_on_success(self):
        def reply(request):
            data = json.loads(confirmation(request))
            data["message"] = {"private": [TOKEN, SETUP_TOKEN, PRIVATE_TEXT + "\n\x1b[31m"]}
            return json.dumps(data)
        self.exercise(reply, accepted=True)


if __name__ == "__main__":
    unittest.main(verbosity=2)
