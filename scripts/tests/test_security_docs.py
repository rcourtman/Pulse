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
                                private_recording_server, request_helper)


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
            if "curl " in step or "pulse_api " in step]


def executable_recipe(step: str) -> str:
    # Execute the same shared helper that the reader is told to define, not a
    # test-only client. Preserve legacy recipes for the baseline control.
    return request_helper() + "\n" + step if "pulse_api " in step else step


def private_export_request(home: Path):
    request = home / ".config/pulse/export-request.json"
    request.parent.mkdir(parents=True, exist_ok=True)
    request.write_text(json.dumps({"passphrase": TEST_PASSPHRASE}))
    request.chmod(0o600)


class SecurityDocsTest(unittest.TestCase):
    def test_shipped_copy_matches_canonical_guide(self):
        self.assertEqual(DOC.read_bytes(),
                         (ROOT / "frontend-modern/public/docs/SECURITY.md").read_bytes())

    def test_copied_requests_never_expose_credentials_or_weaken_tls(self):
        requests = recipes()
        self.assertEqual(len(requests), len(EXPECTED))
        for request in requests:
            self.assertNotRegex(request, r"(?:\b\w*TOKEN=|X-API-Token:|Authorization:|Bearer\s|--cookie\b)")
            self.assertNotRegex(request, r"(?:--insecure|--verbose|--trace\S*|--location|\s-k\b)")
            self.assertIn("pulse_api ", request)
            self.assertNotIn("curl ", request)
            if "/api/config/export" in request:
                self.assertIn('< "$HOME/.config/pulse/export-request.json"', request)
                self.assertNotIn('"passphrase":', request)
            elif "pulse_api POST" in request:
                self.assertIn("--data-binary @-", request_helper())
                self.assertIn("<<'JSON'", request)
        shell = "\n".join(blocks())
        self.assertNotRegex(shell, r"(?:--token\s|PULSE_AUTH_PASS=|API_TOKEN[S]?=|curl[^\n]*\|)")
        self.assertNotIn("rm -rf", shell)
        self.assertNotIn("authorized_keys", shell)

    def test_export_preparation_protects_new_and_existing_files_without_erasing_them(self):
        preparation = next(block for block in blocks() if 'vi "$request_file"' in block)
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
                    subprocess.run(["bash", "-eu", "-c", preparation], env=env, check=True, timeout=10)
                    self.assertEqual(stat.S_IMODE(request.stat().st_mode), 0o600)
                    self.assertEqual(stat.S_IMODE(request.parent.stat().st_mode), 0o700)
                    self.assertEqual(request.read_text(), json.dumps({"passphrase": TEST_PASSPHRASE}) if existing else "")

    def test_exact_methods_paths_and_bodies_with_both_supported_headers(self):
        steps = recipes()
        with private_recording_server() as (port, requests):
            for key, value in (("X-API-Token", TEST_TOKEN), ("Authorization", "Bearer " + TEST_TOKEN)):
                for step, (method, path, body) in zip(steps, EXPECTED):
                    with self.subTest(header=key, method=method, path=path), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary)
                        private_export_request(home)
                        result = exercise_curl(self, home, f"{key}: {value}", port, executable_recipe(step), private_response=True)
                        self.assertEqual(result.returncode, 0, result.stderr.decode())
                        self.assertEqual(requests[-1][0], path)
                        self.assertEqual(requests[-1][1][key], value)
                        self.assertNotIn("X-Curlrc-Injected", requests[-1][1])
                        self.assertEqual(requests[-1][2], method)
                        self.assertEqual(json.loads(requests[-1][3]) if body is not None else None, body)
                        self.assertNotIn(TEST_PASSPHRASE.encode(), result.stdout + result.stderr)
                        self.assertNotIn(TEST_PASSPHRASE, (home / "argv.json").read_text())

    def test_unauthorised_and_wrong_scope_responses_never_report_success(self):
        for status in (302, 401, 403, 500):
            with self.subTest(status=status), private_recording_server(status) as (port, requests):
                for step, (_, path, _) in zip(recipes(), EXPECTED):
                    with self.subTest(path=path), tempfile.TemporaryDirectory() as temporary:
                        home = Path(temporary)
                        private_export_request(home)
                        before = len(requests)
                        result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}", port, executable_recipe(step), private_response=True)
                        self.assertEqual(len(requests), before + 1)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertEqual(requests[-1][0], path)

    def test_loopback_credentials_bypass_hostile_proxy_environment(self):
        with private_recording_server() as (port, requests), \
                private_recording_server() as (proxy_port, proxy_requests):
            for step in recipes():
                with tempfile.TemporaryDirectory() as temporary:
                    home = Path(temporary)
                    private_export_request(home)
                    # exercise_curl clears inherited proxies; explicitly set
                    # hostile ones inside the copied recipe's shell instead.
                    environment = (f"export http_proxy=http://127.0.0.1:{proxy_port} "
                                   f"HTTP_PROXY=http://127.0.0.1:{proxy_port} "
                                   f"https_proxy=http://127.0.0.1:{proxy_port} "
                                   f"ALL_PROXY=http://127.0.0.1:{proxy_port} "
                                   "NO_PROXY='' no_proxy=''\n")
                    before = len(requests)
                    result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}",
                                           port, environment + executable_recipe(step),
                                           private_response=True)
                    self.assertEqual(result.returncode, 0, result.stderr.decode())
                    self.assertEqual(len(requests), before + 1)
                    self.assertEqual(proxy_requests, [], "a proxy received the credential")

    def test_partial_responses_are_private_failures_without_replay(self):
        with private_recording_server(partial=True) as (port, requests):
            for step in recipes():
                with tempfile.TemporaryDirectory() as temporary:
                    home = Path(temporary)
                    private_export_request(home)
                    before = len(requests)
                    result = exercise_curl(self, home, f"X-API-Token: {TEST_TOKEN}",
                                           port, executable_recipe(step), private_response=True)
                    self.assertEqual(result.returncode, 18)
                    self.assertEqual(len(requests), before + 1)
                    self.assertNotIn(TEST_PASSPHRASE.encode(), result.stdout + result.stderr)

    def test_export_preparation_refuses_symlinks_and_nonregular_files(self):
        preparation = next(block for block in blocks() if 'vi "$request_file"' in block)
        for component in (".config", ".config/pulse", ".config/pulse/export-request.json"):
            with self.subTest(component=component), tempfile.TemporaryDirectory() as temporary:
                home = Path(temporary) / "home"
                home.mkdir()
                target = Path(temporary) / "target"
                if component.endswith(".json"):
                    target.write_text("preserve private request\n")
                    target.chmod(0o644)
                else:
                    target.mkdir(mode=0o755)
                link = home / component
                link.parent.mkdir(parents=True, exist_ok=True)
                link.symlink_to(target)
                before = stat.S_IMODE(target.stat().st_mode)
                tools = home / "tools"
                tools.mkdir()
                editor = tools / "vi"
                editor.write_text('#!/bin/sh\ntouch "$HOME/editor-called"\n')
                editor.chmod(0o700)
                result = subprocess.run(["bash", "-c", preparation], capture_output=True,
                                        env=dict(os.environ, HOME=str(home),
                                                 PATH=f"{tools}:{os.environ['PATH']}"), timeout=10)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(stat.S_IMODE(target.stat().st_mode), before)
                self.assertFalse((home / "editor-called").exists())
                if target.is_file():
                    self.assertEqual(target.read_text(), "preserve private request\n")
        with tempfile.TemporaryDirectory() as temporary:
            home = Path(temporary)
            (home / ".config/pulse/export-request.json").mkdir(parents=True)
            result = subprocess.run(["bash", "-c", preparation], capture_output=True,
                                    env=dict(os.environ, HOME=str(home)), timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b"regular file", result.stderr)

    def test_all_shell_recipes_parse_and_explain_shared_transport_and_uncertainty(self):
        for block in blocks():
            subprocess.run(["bash", "-n", "-c", block], check=True, capture_output=True)
        guide = " ".join(DOC.read_text().split())
        for boundary in ("docs/API.md#api-token-recommended", "same Bash session",
                         "not an installed Pulse command", "one request", "owner-only",
                         "prints only", "including on failure", "redacted excerpt",
                         "proxy environment settings", "HTTPS origin and protocol changes",
                         "certificate verification", "--globoff", "five-second",
                         "twenty-second", "no redirects", "automatic retries",
                         "not a batch", "after the reset was applied",
                         "do not repeat a reset", "does not prove that login has recovered"):
            self.assertIn(boundary, guide)

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
