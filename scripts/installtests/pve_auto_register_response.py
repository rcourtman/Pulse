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
import socket
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
    def __init__(self, response, *, legacy=False, fault_path="", status=200,
                 redirect="", stall=False, disconnect=False, partial=False, capture_only=False):
        self.requests = []
        self.errors = []
        self.ambient_headers = []
        self.done = threading.Event()
        peer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                try:
                    body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                    peer.requests.append((self.path, body))
                    peer.ambient_headers.extend(self.headers.get_all("X-Fixture-Ambient", []))
                    if capture_only:
                        payload = "{}"
                    elif self.path == "/api/setup-script-url":
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
                    failed = self.path == fault_path
                    if failed and stall:
                        peer.done.wait(25)
                        return
                    if failed and disconnect:
                        self.connection.shutdown(socket.SHUT_RDWR)
                        self.connection.close()
                        return
                    encoded = payload.encode()
                    self.send_response(status if failed else 200)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(encoded) + (20 if failed and partial else 0)))
                    self.send_header("Connection", "close")
                    if failed and redirect:
                        self.send_header("Location", redirect)
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
        self.done.set()
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
    def exercise(self, response=confirmation, *, accepted=False, name=CANONICAL_NAME, legacy=False, rotate=False,
                 fault_path="", status=200, redirect="", stall=False, disconnect=False, partial=False,
                 ambient=False, proxy=""):
        peer = RegistrationPeer(response, legacy=legacy, fault_path=fault_path, status=status,
                                redirect=redirect, stall=stall, disconnect=disconnect, partial=partial)
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
                    'if [ "$1 $2 $3" = "user token add" ]; then '
                    'if [ "$TOKEN_ROTATE" = true ] && [ ! -e "$FIXTURE/first-create" ]; then '
                    ': > "$FIXTURE/first-create"; printf "token already exists\\n"; exit 1; fi; '
                    'cat "$FIXTURE/token.json"; '
                    'elif [ "$1 $2" = "role list" ]; then printf "Sys.Audit VM.GuestAgent.Audit VM.GuestAgent.FileRead\\n"; fi\n')
                (tools / "pveum").chmod(0o700)
                (tools / "hostname").write_text('#!/bin/sh\nprintf "fixture-pve\\n"\n')
                (tools / "hostname").chmod(0o700)
                # Parent-source adverse controls execute the original caller,
                # without transplanting the new transport into it.
                transport = (function("pve_auto_register_pulse_post")
                             if "pve_auto_register_pulse_post() {" in installer.SOURCE else "")
                source = "\n".join(function(name) for name in (
                    "extract_pve_token_value", "pve_token_already_exists_error",
                    "create_pve_auto_register_token", "auto_register_pve_node")) + "\n" + transport
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
                           HOME=str(root), CURL_HOME=str(root), FIXTURE=str(root), TOKEN_ROTATE=str(rotate).lower(),
                           REAL_PYTHON=sys.executable, REAL_CURL=shutil.which("curl"),
                           ARGV_LOG=str(root / "argv"), PULSE_PVE_HOST_URL=HOST,
                           PULSE_FIXTURE_PORT=str(peer.server.server_port), PULSE_AUTO_BACKUP_PERMS="true",
                           PULSE_SKIP_AUTO_NODE="false", NO_PROXY="", no_proxy="",
                           HTTP_PROXY=proxy, http_proxy=proxy, HTTPS_PROXY=proxy, https_proxy=proxy,
                           ALL_PROXY=proxy, all_proxy=proxy)
                if ambient:
                    (root / ".curlrc").write_text(
                        'location\nretry = 3\nheader = "X-Fixture-Ambient: fixture-marker"\n'
                        'trace-ascii = "' + str(root / "trace") + '"\n')
                started = time.monotonic()
                result = subprocess.run(["bash", "-c", script], env=env,
                                        capture_output=True, text=True, timeout=22 if stall else 12)
                elapsed = time.monotonic() - started
                output = result.stdout + result.stderr
                args = [json.loads(line) for line in (root / "argv").read_text().splitlines()]
                observed = json.dumps(args)
                for secret in (TOKEN, SETUP_TOKEN, PRIVATE_TEXT):
                    self.assertFalse(secret in output, "fixture secret/private response entered installer output")
                    self.assertFalse(secret in observed, "fixture secret/private response entered child argv")
                self.assertNotIn("Traceback", output)
                self.assertEqual(result.returncode, 0, "optional registration must not abort installation")
                self.assertEqual(peer.errors, [])
                expected_paths = ["/api/setup-script-url"]
                if fault_path != "/api/setup-script-url":
                    expected_paths.append("/api/auto-register")
                self.assertEqual([path for path, _ in peer.requests], expected_paths,
                                 "bootstrap POST was repeated or did not reach its intended peer")
                self.assertEqual(peer.ambient_headers, [], "ambient curl configuration affected the request")
                self.assertFalse((root / "trace").exists(), "ambient tracing retained credential-bearing bytes")
                curl_args = [argv[1:] for argv in args if argv[0] == "curl"]
                self.assertEqual(len(curl_args), len(expected_paths))
                for argv, path in zip(curl_args, expected_paths):
                    self.assertEqual(argv[0], "--disable")
                    for flag, value in (("--proto", "=http"), ("--noproxy", "*"),
                                        ("--connect-timeout", "5"), ("--max-time", "15"), ("--retry", "0"),
                                        ("--request", "POST"), ("--data-binary", "@-"), ("--url", peer.url + path)):
                        self.assertEqual(argv[argv.index(flag) + 1], value)
                    for flag in ("--location", "-L", "--insecure", "-k", "--verbose", "--trace", "--config"):
                        self.assertNotIn(flag, argv)
                python_args = [argv[1:] for argv in args if argv[0] == "python3"]
                self.assertTrue(all(argv[0] == "-" for argv in python_args))
                if not fault_path:
                    self.assertGreaterEqual(len(python_args), 7)
                    self.assertEqual(python_args[-1], ["-"], "confirmation parser accepted a process argument")
                calls = (root / "pve-calls").read_text().splitlines() if (root / "pve-calls").exists() else []
                created = 0 if fault_path == "/api/setup-script-url" else (2 if rotate else 1)
                self.assertEqual(sum(call.startswith("user token add ") for call in calls), created)
                self.assertEqual(sum(call.startswith("user token remove ") for call in calls), 1 if rotate else 0)
                if fault_path == "/api/setup-script-url":
                    self.assertEqual(calls, [], "failed bootstrap changed PVE state")
                self.assertIn("RESULT=" + str(accepted).lower() + "\n", output)
                if accepted:
                    self.assertIn("NAME=" + name + "\nERROR=\n", output)
                    self.assertIn("SUCCESS=Registered " + name + " with Pulse automatically", output)
                    self.assertNotIn("WARN=", output)
                else:
                    error = ("setup token request failed" if fault_path == "/api/setup-script-url"
                             else "auto-register request failed" if fault_path else "auto-register unsuccessful")
                    self.assertIn("NAME=\nERROR=" + error + "\n", output)
                    if fault_path != "/api/setup-script-url":
                        self.assertIn("WARN=Pulse auto-registration could not be confirmed; check the node in Pulse before trying again", output)
                    self.assertNotIn("SUCCESS=", output)
                return elapsed
        finally:
            peer.close()

    def test_valid_modern_and_legacy_confirmation_retains_registration_without_private_output(self):
        for legacy in (False, True):
            with self.subTest(legacy=legacy):
                self.exercise(accepted=True, legacy=legacy)

    def test_existing_deterministic_token_rotation_reaches_confirmed_registration(self):
        self.exercise(accepted=True, rotate=True)

    def test_token_helper_returns_data_and_exit_status_to_actual_caller_names(self):
        for mode, expected_status in (("json", 0), ("legacy", 0), ("failed", 7)):
            script = """set -euo pipefail
pveum() {
    if [[ "$MODE" == failed ]]; then printf 'synthetic failure'; return 7; fi
    if [[ "$MODE" == legacy && "$*" == *--output-format* ]]; then
        printf 'unknown option --output-format'; return 2
    fi
    printf 'synthetic return value'
}
""" + function("create_pve_auto_register_token") + """
caller() {
    local token_output="" token_status=99
    create_pve_auto_register_token fixture token_output token_status
    [[ "$token_status" -eq "$EXPECTED_STATUS" ]]
    if [[ "$MODE" == failed ]]; then [[ "$token_output" == 'synthetic failure' ]]
    else [[ "$token_output" == 'synthetic return value' ]]; fi
    printf 'HELPER_RETURN_VERIFIED\\n'
}
caller
"""
            with self.subTest(mode=mode):
                result = subprocess.run(["bash", "-c", script],
                                        env=dict(os.environ, MODE=mode, EXPECTED_STATUS=str(expected_status)),
                                        capture_output=True, text=True, timeout=3)
                self.assertEqual(result.returncode, 0, "token helper lost its caller result/status")
                self.assertEqual(result.stdout, "HELPER_RETURN_VERIFIED\n")
                self.assertEqual(result.stderr, "")

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

    def test_ambient_config_and_proxy_cannot_observe_either_bootstrap_request(self):
        proxy = RegistrationPeer(confirmation, capture_only=True)
        try:
            self.exercise(accepted=True, ambient=True, proxy=proxy.url)
            self.assertEqual(proxy.requests, [], "ambient proxy received bootstrap data")
        finally:
            proxy.close()

    def test_redirects_never_receive_setup_or_registration_data(self):
        destination = RegistrationPeer(confirmation, capture_only=True)
        try:
            for path in ("/api/setup-script-url", "/api/auto-register"):
                for status in (301, 302, 303, 307, 308):
                    with self.subTest(path=path, status=status):
                        # Even a valid success body with a redirect status must
                        # not create/rotate a PVE token or confirm registration.
                        self.exercise(fault_path=path, status=status,
                                      redirect=destination.url + "/redirected", ambient=True)
                        self.assertEqual(destination.requests, [], "redirect received bootstrap data")
        finally:
            destination.close()

    def test_only_exact_200_can_confirm_either_bootstrap_request(self):
        for path in ("/api/setup-script-url", "/api/auto-register"):
            for status in (201, 202, 204, 400, 401, 403, 404, 429, 500, 502, 504):
                with self.subTest(path=path, status=status):
                    self.exercise(fault_path=path, status=status)

    def test_server_error_does_not_replay_registration(self):
        self.exercise(fault_path="/api/auto-register", status=503)

    def test_setup_failure_does_not_change_pve_or_replay(self):
        self.exercise(fault_path="/api/setup-script-url", status=503)

    def test_lost_response_is_not_replayed_or_reported_as_absent(self):
        for path in ("/api/setup-script-url", "/api/auto-register"):
            with self.subTest(path=path):
                self.exercise(fault_path=path, disconnect=True)

    def test_partial_response_cannot_confirm_registration_or_trigger_replay(self):
        for path in ("/api/setup-script-url", "/api/auto-register"):
            with self.subTest(path=path):
                self.exercise(fault_path=path, partial=True)

    def test_both_bootstrap_responses_have_real_total_deadlines(self):
        for path in ("/api/setup-script-url", "/api/auto-register"):
            with self.subTest(path=path):
                elapsed = self.exercise(fault_path=path, stall=True)
                self.assertGreater(elapsed, 13)
                self.assertLess(elapsed, 20)


if __name__ == "__main__":
    unittest.main(verbosity=2)
