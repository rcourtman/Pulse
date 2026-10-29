#!/usr/bin/env python3
"""Execute the canonical security guide with synthetic secrets and loopback.

Run via pulse-worker-source-proof. The fixture checks copied requests and private
files, not a deployed Pulse instance; existing Go tests check transfer authority,
encryption and lockout handlers.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import unittest

from test_api_auth_docs import (ROOT, TEST_TOKEN, exercise_curl,
                                private_recording_server, recording_server,
                                request_helper)


DOC = ROOT / "SECURITY.md"
TEST_PASSPHRASE = "synthetic-security-export-passphrase"
EXPECTED = (
    ("GET", "/api/state/summary", None),
    ("POST", "/api/config/export", {"passphrase": TEST_PASSPHRASE}),
    ("GET", "/api/monitoring/scheduler/health", None),
    ("POST", "/api/security/reset-lockout", {"identifier": "username"}),
    ("POST", "/api/security/reset-lockout", {"identifier": "198.51.100.100"}),
)


def blocks() -> list[str]:
    return re.findall(r"```bash\n(.*?)```", DOC.read_text(encoding="utf-8"), re.DOTALL)


def recipes() -> list[str]:
    return [step for block in blocks() for step in re.split(r"\n(?=# )", block)
            if "pulse_api " in step]


def private_export_request(home: Path):
    request = home / ".config/pulse/export-request.json"
    request.parent.mkdir(parents=True, exist_ok=True)
    request.write_text(json.dumps({"passphrase": TEST_PASSPHRASE}))
    request.chmod(0o600)


class SecurityDocsTest(unittest.TestCase):
    def test_shipped_copy_matches_canonical_guide(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/SECURITY.md").read_bytes())

    def test_copied_requests_use_the_existing_private_response_helper(self):
        requests = recipes()
        self.assertEqual(len(requests), len(EXPECTED))
        for request, (method, path, _body) in zip(requests, EXPECTED):
            self.assertEqual(request.count("pulse_api "), 1,
                             "each copied block must send only its selected operation")
            self.assertIn(f"pulse_api {method} {path}", request)
            self.assertNotRegex(request, r"(?:\b\w*TOKEN=|X-API-Token:|Authorization:|Bearer\s|--cookie\b)")
            self.assertNotRegex(request, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
            if path == "/api/config/export":
                self.assertIn('pulse_api POST /api/config/export < "$request_file"', request)
                self.assertNotIn('"passphrase":', request)
            elif method == "POST":
                self.assertIn("<<'JSON'", request)
        shell = "\n".join(blocks())
        self.assertNotRegex(shell, r"(?:--token\s|PULSE_AUTH_PASS=|API_TOKEN[S]?=|curl\s)")
        self.assertNotIn("rm -rf", shell)
        self.assertNotIn("authorized_keys", shell)
        for block in blocks():
            subprocess.run(["bash", "-n", "-c", block], check=True, capture_output=True)

    def export_preparation(self):
        return next(block for block in blocks() if 'vi "$request_file"' in block)

    def test_export_preparation_protects_new_and_existing_files_without_erasing_them(self):
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            tools = home / "tools"
            tools.mkdir()
            editor = tools / "vi"
            editor.write_text('#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = "$HOME/.config/pulse/export-request.json" ]\n')
            editor.chmod(0o700)
            env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
            request = home / ".config/pulse/export-request.json"
            for existing in (False, True):
                with self.subTest(existing=existing):
                    if existing:
                        private_export_request(home)
                        request.chmod(0o644)
                        request.parent.chmod(0o755)
                    # No caller-provided -e: the copied block must fail closed.
                    subprocess.run(["bash", "-c", self.export_preparation()], env=env, check=True, timeout=10)
                    self.assertEqual(stat.S_IMODE(request.stat().st_mode), 0o600)
                    self.assertEqual(stat.S_IMODE(request.parent.stat().st_mode), 0o700)
                    self.assertEqual(request.read_text(), json.dumps({"passphrase": TEST_PASSPHRASE}) if existing else "")

    def test_exact_methods_paths_and_bodies_with_both_supported_headers(self):
        with private_recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                for step, (method, path, body) in zip(recipes(), EXPECTED):
                    with self.subTest(header=key, method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary)
                        private_export_request(home)
                        before = len(requests)
                        result = exercise_curl(self, home, f"{key}: {value}", port,
                                               request_helper() + "\n" + step,
                                               private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1)
                        self.assertEqual(requests[-1][0], path)
                        self.assertEqual(requests[-1][1][key], value)
                        self.assertNotIn("X-Curlrc-Injected", requests[-1][1])
                        self.assertEqual(requests[-1][2], method)
                        self.assertEqual(json.loads(requests[-1][3]) if body is not None else None, body)
                        self.assertNotIn(TEST_PASSPHRASE.encode(), result.stdout + result.stderr)
                        self.assertNotIn(TEST_PASSPHRASE, (home / "argv.json").read_text())

    def test_loopback_security_requests_bypass_environment_proxy(self):
        with private_recording_server() as (port, requests), recording_server() as (proxy_port, proxy_requests):
            for step, (method, path, _body) in zip(recipes(), EXPECTED):
                with self.subTest(method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                    home = Path(temporary)
                    private_export_request(home)
                    before = len(requests)
                    result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}", port,
                                           request_helper() + "\n" + step,
                                           private_response=True, proxy_port=proxy_port)
                    self.assertEqual(result.returncode, 0, result.stderr.decode())
                    self.assertEqual(len(requests), before + 1)
                    self.assertEqual(requests[-1][0], path)
                    self.assertEqual(proxy_requests, [], "proxy received a private loopback request")

    def test_http_errors_redirects_and_partial_bodies_fail_privately_without_retry(self):
        for status, partial, expected_exit in ((401, False, 22), (403, False, 22),
                                               (500, False, 22), (302, False, 1),
                                               (200, True, 18)):
            with self.subTest(status=status, partial=partial), private_recording_server(status, partial) as (port, requests):
                for step, (method, path, _body) in zip(recipes(), EXPECTED):
                    with self.subTest(method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary)
                        private_export_request(home)
                        before = len(requests)
                        result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}", port,
                                               request_helper() + "\n" + step,
                                               private_response=True)
                        self.assertEqual(result.returncode, expected_exit, result.stderr.decode())
                        self.assertEqual(len(requests), before + 1,
                                         "a failed or uncertain operation must not be retried or redirected")
                        self.assertEqual(requests[-1][0], path)
                        self.assertNotIn(TEST_PASSPHRASE.encode(), result.stdout + result.stderr)

    def test_export_paths_are_checked_before_editor_or_transport(self):
        export = next(step for step in recipes() if "/api/config/export" in step)
        for operation, script in (("prepare", self.export_preparation()),
                                  ("send", export)):
            for component in (".config", ".config/pulse", ".config/pulse/export-request.json"):
                for kind in ("symlink", "dangling", "wrong-type"):
                    with self.subTest(operation=operation, component=component, kind=kind), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary) / "home"
                        home.mkdir()
                        node = home / component
                        node.parent.mkdir(parents=True, exist_ok=True)
                        target = Path(temporary) / "target"
                        is_file = component.endswith(".json")
                        if kind != "dangling":
                            if is_file:
                                target.write_text("preserve existing sensitive fixture\n")
                                target.chmod(0o640)
                            else:
                                target.mkdir()
                                target.chmod(0o750)
                        if kind == "wrong-type":
                            if is_file:
                                os.mkfifo(node)
                            else:
                                node.write_text("not a directory\n")
                        else:
                            node.symlink_to(target)
                        tools = home / "tools"
                        tools.mkdir()
                        for name in ("vi", "curl"):
                            tool = tools / name
                            tool.write_text(f'#!/bin/sh\n: > "$HOME/{name}-called"\nexit 79\n')
                            tool.chmod(0o700)
                        before_mode = stat.S_IMODE(target.stat().st_mode) if target.exists() else None
                        env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
                        result = subprocess.run(["bash", "-c", request_helper() + "\n" + script],
                                                env=env, capture_output=True, timeout=10)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertFalse((home / "vi-called").exists())
                        self.assertFalse((home / "curl-called").exists())
                        self.assertNotIn(TEST_PASSPHRASE.encode(), result.stdout + result.stderr)
                        if before_mode is not None:
                            self.assertEqual(stat.S_IMODE(target.stat().st_mode), before_mode)
                            if is_file:
                                self.assertEqual(target.read_text(), "preserve existing sensitive fixture\n")
                            else:
                                self.assertEqual(list(target.iterdir()), [])
                        else:
                            self.assertFalse(target.exists())

    def test_preparation_failures_stop_before_editor_and_send_failures_before_transport(self):
        export = next(step for step in recipes() if "/api/config/export" in step)
        for operation, script, commands in (("prepare", self.export_preparation(), ("mkdir", "chmod", "touch")),
                                             ("send", export, ("chmod",))):
            for command in commands:
                with self.subTest(operation=operation, command=command), tempfile.TemporaryDirectory() as temporary:
                    home = Path(temporary)
                    private_export_request(home)
                    tools = home / "tools"
                    tools.mkdir()
                    for name in ("vi", "curl"):
                        tool = tools / name
                        tool.write_text(f'#!/bin/sh\n: > "$HOME/{name}-called"\n')
                        tool.chmod(0o700)
                    failure = tools / command
                    failure.write_text("#!/bin/sh\nexit 73\n")
                    failure.chmod(0o700)
                    env = dict(os.environ, HOME=str(home), PATH=f"{tools}:{os.environ['PATH']}")
                    result = subprocess.run(["bash", "-c", request_helper() + "\n" + script],
                                            env=env, capture_output=True, timeout=10)
                    self.assertEqual(result.returncode, 73)
                    self.assertFalse((home / "vi-called").exists())
                    self.assertFalse((home / "curl-called").exists())
                    self.assertEqual((home / ".config/pulse/export-request.json").read_text(),
                                     json.dumps({"passphrase": TEST_PASSPHRASE}))

    def test_guidance_retains_authority_and_credential_lifecycle_boundaries(self):
        guide = " ".join(DOC.read_text().split())
        for boundary in ("monitoring:read", "settings:read", "settings:write", "CSRF",
                         "organization-bound", "direct loopback", "export only",
                         "12 characters", "public", "separate from the export",
                         "entire configuration bundle", "31 March 2027", "no longer sold",
                         "does not reset a password", "deleting `.env` is not a universal reset",
                         "fingerprint", "Revoking the public key", "privileged inspection"):
            self.assertTrue(boundary in guide, f"missing safety boundary: {boundary}")
        self.assertIn("docs/UNIFIED_AGENT.md#private-file-installation-linux-macos-and-nas", guide)
        self.assertIn("docs/TROUBLESHOOTING.md#i-forgot-my-password", guide)
        for boundary in ("same Bash session", "not an installed Pulse command",
                         "prints only its HTTP status", "preserves earlier responses",
                         "bypasses environment proxies", "does not follow redirects",
                         "private response-file path", "not a complete result",
                         "not proof of an import or tested recovery", "concurrent path replacement",
                         "Choose **one** request", "separate counters",
                         "do not run both examples as a batch", "repeat an uncertain POST"):
            self.assertIn(boundary, guide)
        self.assertNotIn("### What's **Not** Encrypted", guide)

    def test_encrypted_file_examples_match_the_actual_store_and_data_path(self):
        guide = " ".join(DOC.read_text().split())
        persistence = (ROOT / "internal/config/persistence.go").read_text()
        crypto = (ROOT / "internal/crypto/crypto.go").read_text()
        for filename in ("nodes.enc", "email.enc", "webhooks.enc", "truenas.enc",
                         "vmware.enc", "ai.enc", "sso.enc"):
            with self.subTest(filename=filename):
                self.assertIn(f'resolveLeaf("{filename}")', persistence)
                self.assertIn(f"`{filename}`", guide)
        self.assertRegex(crypto, r'encryptionKeyFileName\s*=\s*"\.encryption\.key"')
        for boundary in ("active data directory", "`PULSE_DATA_DIR`", "`/etc/pulse`",
                         "`/data`", "existing deployment's actual data path"):
            self.assertIn(boundary, guide)
        self.assertNotIn("/etc/pulse/nodes.enc", guide)

    def test_key_scope_and_recovery_never_imply_safe_public_or_unmatched_copies(self):
        guide = " ".join(DOC.read_text().split())
        for boundary in ("stored alongside", "not an encrypted credential file",
                         "matching key", "can decrypt", "entire data directory",
                         "private recovery backup", "not in a public attachment",
                         "owner-only", "0600", "0700", "service ownership",
                         "Do not delete or regenerate the key", "mix files from different instances",
                         "Preserve the existing files", "not verified recovery",
                         "docs/MIGRATION.md"):
            self.assertIn(boundary, guide)

    def test_deployment_credentials_and_unencrypted_evidence_remain_private(self):
        guide = " ".join(DOC.read_text().split())
        for boundary in ("not whole-directory or whole-disk encryption",
                         "environment files", "service definitions", "agent token files",
                         "logs and history", "bcrypt", "plaintext `PULSE_AUTH_PASS`",
                         "does not rewrite", "original deployment source",
                         "hashes and raw client credentials private",
                         "does not scrub earlier backups", "external deployment configuration"):
            self.assertIn(boundary, guide)
        for overclaim in ("Passwords never stored in plain text", "Tokens never stored in plain text",
                          "Credentials encrypted at rest (always enabled)"):
            self.assertNotIn(overclaim, guide)

    def test_security_score_and_log_masking_are_not_disclosure_guarantees(self):
        guide = " ".join(DOC.read_text().split())
        for boundary in ("not a complete security audit", "every file is encrypted",
                         "not a guarantee", "Free-text errors and historical records",
                         "Review excerpts locally", "#collect-diagnostics-safely",
                         "#inspect-notification-logs"):
            self.assertIn(boundary, guide)
        self.assertNotIn("token values masked with `***` in all outputs", guide)

    def test_security_checks_use_existing_bounded_evidence_not_broad_log_recipes(self):
        source = DOC.read_text()
        audit = source.split("#### Auditing Your Deployment\n", 1)[1].split("\n---\n", 1)[0]
        checklist = source.split("### Verification Checklist\n", 1)[1].split("\n## ", 1)[0]
        self.assertNotRegex(audit + checklist,
                            r"(?:systemctl status|journalctl|docker logs|ls /home/|```bash)")
        for link in ("docs/TEMPERATURE_MONITORING.md#check-existing-linux-readings-safely",
                     "docs/VM_DISK_MONITORING.md#backup-safety"):
            self.assertIn(link, audit)
        for link in ("docs/TROUBLESHOOTING.md#inspect-notification-logs",
                     "docs/TROUBLESHOOTING.md#if-you-already-posted-private-information"):
            self.assertIn(link, checklist)
        guide = " ".join((audit + checklist).split())
        for boundary in ("does not prove", "does not revoke", "full service definitions",
                         "container inspections", "Copy as cURL", "partial hint match"):
            self.assertIn(boundary, guide)

    def test_logging_defaults_and_file_only_rotation_match_server_source(self):
        source = DOC.read_text()
        logging = source.split("### Runtime Logging Configuration\n", 1)[1].split("\n## ", 1)[0]
        rows = dict(re.findall(r"^\| `(LOG_[A-Z_]+)` \| [^|]+ \| (.*?) \|$",
                               logging, re.MULTILINE))
        config = (ROOT / "internal/config/config.go").read_text()
        defaults = dict(re.findall(r'envconfig:"(LOG_[A-Z_]+)" default:"([^"]*)"', config))
        self.assertEqual(set(rows), set(defaults))
        self.assertEqual({key: ("Unset" if not value else f"`{value}`")
                          for key, value in defaults.items()}, rows)
        writer = (ROOT / "internal/logging/logging.go").read_text()
        self.assertRegex(writer, r'func newRollingFileWriter\(cfg Config\).*?\{\s*'
                                r'path := strings.TrimSpace\(cfg.FilePath\)\s*'
                                r'if path == "" \{\s*return nil, nil')
        guide = " ".join(logging.split())
        for boundary in ("only when `LOG_FILE` is configured", "systemd journal",
                         "container logs", "browser log buffer", "separate **Audit Log**",
                         "Zero or negative", "do not disable cleanup",
                         "do not redact", "do not delete logs or shorten retention"):
            self.assertIn(boundary, guide)
        self.assertNotIn("manage audit log retention", guide)

    def test_logging_changes_preserve_the_existing_deployment_and_runtime_distinction(self):
        logging = DOC.read_text().split("### Runtime Logging Configuration\n", 1)[1].split("\n## ", 1)[0]
        # Environment-file settings are not a new-install command or a shell
        # script containing a systemd [Service] stanza.
        self.assertNotIn("```bash", logging)
        guide = " ".join(logging.split())
        for boundary in ("server-wide threshold immediately", "at startup unless `LOG_LEVEL`",
                         "docs/CONFIGURATION.md#log-levels", "environment file or drop-in",
                         "inside the Pulse container", "recreate/redeploy",
                         "`docker restart` does not apply it", "same image",
                         "persistent data mounts", "credentials and other settings"):
            self.assertIn(boundary, guide)

    def test_debug_display_and_support_bundle_guidance_does_not_invite_replay_or_disclosure(self):
        source = DOC.read_text()
        logging = source.split("### Runtime Logging Configuration\n", 1)[1].split("\n## ", 1)[0]
        observability = source.split("### Scheduler Health API\n", 1)[1].split("\n### ", 1)[0]
        self.assertNotIn("Enable debug logging temporarily", source)
        guide = " ".join(logging.split())
        for boundary in ("Debug is not a prerequisite", "repeat the failed action",
                         "previous working level", "not proof of underlying recovery",
                         "**Pause Stream**", "**Clear Log Output**", "only the browser display",
                         "complete log file", "configuration and environment information",
                         "Keep it private", "locally reviewed, redacted excerpts",
                         "backups, freeze/thaw", "unresponsive-host incident"):
            self.assertIn(boundary, guide)
        for section in (logging, observability):
            self.assertIn("docs/TROUBLESHOOTING.md#collect-diagnostics-safely", section)
        self.assertIn("not proof that every workload or collector is healthy", observability)


if __name__ == "__main__":
    unittest.main()
