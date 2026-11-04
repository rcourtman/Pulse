#!/usr/bin/env python3
"""Exercise best-effort evaluation setup with inert licence transport.

The real setup functions, JSON parser and private-file writes run; installer
main, network, Docker, live licensing and client runtimes do not. The normal
smoke inventory discovers this suite.
"""

import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SETUP = Path(os.environ.get("PULSE_MSP_SETUP_UNDER_TEST", ROOT / "deploy/provider-msp/setup.sh"))
ENTRY = '\nmain "$@"\n'
LICENSE = "synthetic.signed.license"
PRIVATE = "synthetic-private-key-never-send"
ENV = f"CP_PROVIDER_MSP_LICENSE_FILE=\nCP_ENTITLEMENT_SIGNING_PRIVATE_KEY={PRIVATE}\nDOMAIN=provider.invalid\n"

HARNESS = r'''
derive_lease_signing_public_key() {
  printf 'derive\n' >> "$FIXTURE/derivations"
  if [[ "$DERIVE" == fail ]]; then exit 1; fi
  printf 'synthetic-public-key'
}
curl() {
  printf '%s\0' "$@" >> "$FIXTURE/curl-args"
  printf 'request\n' >> "$FIXTURE/requests"
  local fail=0 write_out=0 arg
  for arg in "$@"; do
    case "$arg" in -fsS|--fail) fail=1 ;; --write-out|-w) write_out=1 ;; esac
  done
  printf 'synthetic transport stderr must stay private\n' >&2
  # Model --fail faithfully so the parent control observes the original
  # HTTP-rejection/transport conflation instead of a double's artefact.
  if [[ "$TRANSPORT_EXIT" == 0 && "$fail" == 1 && "$HTTP_STATUS" == [45]* ]]; then
    return 22
  fi
  printf '%s' "$RESPONSE"
  if [[ "$write_out" == 1 ]]; then printf '\n%s' "$HTTP_STATUS"; fi
  return "$TRANSPORT_EXIT"
}
ensure_eval_license
if [[ "$SHOW_SUMMARY" == 1 ]]; then
  PORTAL_READINESS_REPORT="synthetic portal reachability, not client acceptance"
  print_summary
fi
printf 'FUNCTION_COMPLETED\n'
'''


class ProviderMSPEvalLicenseTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        source = SETUP.read_text()
        if not source.endswith(ENTRY) or source.count(ENTRY) != 1:
            raise AssertionError("cannot safely remove the exact installer entrypoint")
        cls.functions = source[:-len(ENTRY)]

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="pulse-msp-eval-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.env_file = self.root / ".env"
        self.env_file.write_text(ENV)
        self.env_file.chmod(0o600)
        # Saved client/identity data is deliberately outside the licence path.
        (self.root / "saved-client").write_text("preserve client identity and state\n")
        self.eval_file = self.root / "provider-msp-eval-license.jwt"

    def run_setup(self, *, status="200", transport=0, response=None, skip="0", derive="ok", email="", summary=False):
        if response is None:
            response = json.dumps({"license": LICENSE, "expires_at": "2030-01-02T03:04:05Z"})
        env = {
            "PATH": os.environ["PATH"], "FIXTURE": str(self.root),
            "PULSE_PROVIDER_MSP_INSTALL_DIR": str(self.root),
            "PULSE_PROVIDER_MSP_LICENSE_URL": "https://license.invalid/",
            "PULSE_PROVIDER_MSP_SKIP_EVAL_LICENSE": skip,
            "PULSE_PROVIDER_MSP_EVAL_EMAIL": email,
            "PULSE_PROVIDER_MSP_SIGNUP_SOURCE": "synthetic-setup",
            "HTTP_STATUS": status, "TRANSPORT_EXIT": str(transport),
            "RESPONSE": response, "DERIVE": derive, "SHOW_SUMMARY": "1" if summary else "0",
        }
        result = subprocess.run(["bash", "-c", self.functions + HARNESS], env=env,
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("FUNCTION_COMPLETED", result.stdout)
        self.assertEqual(result.stderr, "")
        self.assertNotIn(PRIVATE, result.stdout)
        self.assertNotIn(LICENSE, result.stdout)
        self.assertNotIn("synthetic transport stderr", result.stdout)
        self.assertEqual((self.root / "saved-client").read_text(), "preserve client identity and state\n")
        return result.stdout

    def requests(self):
        path = self.root / "requests"
        return path.read_text().splitlines() if path.exists() else []

    def assert_unlicensed(self, output):
        self.assertFalse(self.eval_file.exists())
        self.assertEqual(self.env_file.read_text(), ENV)
        self.assertEqual(stat.S_IMODE(self.env_file.stat().st_mode), 0o600)
        self.assertNotIn("evaluation license installed", output)
        self.assertIn("continuing unlicensed", output)
        self.assertIn("CP_PROVIDER_MSP_LICENSE_FILE", output)
        self.assertNotIn("portal and client isolation work", output)
        self.assertEqual(len(self.requests()), 1, "no automatic retries")

    def test_http_rejection_is_not_a_connection_failure(self):
        for status in ("400", "401", "403", "409", "422"):
            with self.subTest(status=status):
                self.reset_fixture()
                output = self.run_setup(status=status, response='{"error":"private response must not be logged"}')
                self.assertIn(f"rejected the evaluation request (HTTP {status})", output)
                self.assertIn("Check the request details before retrying", output)
                self.assertNotIn("could not reach", output)
                self.assertNotIn("private response", output)
                self.assert_unlicensed(output)

    def reset_fixture(self):
        # Each subcase has its own complete request identity and saved state.
        for name in ("requests", "curl-args", "derivations", "provider-msp-eval-license.jwt"):
            (self.root / name).unlink(missing_ok=True)
        self.env_file.write_text(ENV)

    def test_rate_limit_and_server_failure_have_distinct_recovery(self):
        for status, message in (("429", "rate limited"), ("500", "temporarily unavailable"), ("503", "temporarily unavailable")):
            with self.subTest(status=status):
                self.reset_fixture()
                output = self.run_setup(status=status)
                self.assertIn(message, output)
                self.assertIn(f"HTTP {status}", output)
                self.assertNotIn("could not reach", output)
                self.assert_unlicensed(output)

    def test_transport_failure_retains_exit_even_with_partial_response(self):
        for code in (6, 7, 23, 28, 35, 51, 60):
            with self.subTest(code=code):
                self.reset_fixture()
                output = self.run_setup(status="200", transport=code)
                self.assertIn(f"request failed (curl exit {code})", output)
                if code in (35, 51, 60):
                    self.assertIn("certificate trust", output)
                    self.assertNotIn("--insecure", output)
                self.assert_unlicensed(output)

    def test_unexpected_status_never_accepts_even_a_license_body(self):
        for status in ("000", "199", "301", "307", "600", "private-status"):
            with self.subTest(status=status):
                self.reset_fixture()
                output = self.run_setup(status=status)
                self.assertIn("unexpected HTTP status", output)
                self.assertNotIn("private-status", output)
                self.assert_unlicensed(output)

    def test_success_requires_one_object_with_nonempty_string_license(self):
        for response in ("", "not JSON", "{}", '"string"', "[]", '{"license":null}',
                         '{"license":42}', '{"license":true}', '{"license":{}}',
                         '{"license":[]}', '{"license":""}', '{"license":"bad\\nvalue"}',
                         '{"license":"bad value"}', '{"license":"bad\\u0001value"}',
                         '{"license":"one"}\n{"license":"two"}'):
            with self.subTest(response=response):
                self.reset_fixture()
                output = self.run_setup(response=response)
                self.assertIn("no valid license", output)
                self.assert_unlicensed(output)

    def test_success_writes_private_license_and_preserves_other_configuration(self):
        for status in ("200", "201", "299"):
            with self.subTest(status=status):
                self.reset_fixture()
                output = self.run_setup(status=status, email="synthetic@example.invalid")
                self.assertIn("evaluation license installed: 2 client workspaces", output)
                self.assertIn("expires 2030-01-02T03:04:05Z", output)
                self.assertEqual(self.eval_file.read_text(), LICENSE)
                self.assertEqual(stat.S_IMODE(self.eval_file.stat().st_mode), 0o600)
                self.assertEqual(self.env_file.read_text(), ENV.replace("CP_PROVIDER_MSP_LICENSE_FILE=\n",
                    "CP_PROVIDER_MSP_LICENSE_FILE=./provider-msp-eval-license.jwt\n"))
                self.assertEqual(len(self.requests()), 1)
                args = (self.root / "curl-args").read_text().split("\0")[:-1]
                self.assertEqual(args[-1], "https://license.invalid/v1/provider-msp/eval-license")
                self.assertEqual(args[args.index("--max-time") + 1], "20")
                self.assertEqual(args[args.index("--write-out") + 1], "\n%{http_code}")
                for forbidden in ("--insecure", "-k", "--location", "-L", "--retry", "--fail", "-fsS"):
                    self.assertNotIn(forbidden, args)
                body = json.loads(args[args.index("-d") + 1])
                self.assertEqual(body, {"entitlement_signing_public_key": "synthetic-public-key",
                    "setup_stage": "images_ready", "email": "synthetic@example.invalid", "signup_source": "synthetic-setup"})
                self.assertNotIn(PRIVATE, str(args))

    def test_response_metadata_is_not_arbitrary_log_output(self):
        output = self.run_setup(response=json.dumps({"license": LICENSE,
            "expires_at": "private response metadata\nforged success"}))
        self.assertNotIn("private response metadata", output)
        self.assertNotIn("forged success", output)
        self.assertEqual(self.eval_file.read_text(), LICENSE)

    def test_omits_email_without_changing_setup_marker(self):
        self.run_setup()
        args = (self.root / "curl-args").read_text().split("\0")
        body = json.loads(args[args.index("-d") + 1])
        self.assertNotIn("email", body)
        self.assertEqual(body["setup_stage"], "images_ready")

    def test_configured_license_and_existing_eval_need_no_request(self):
        for kind in ("configured", "existing"):
            with self.subTest(kind=kind):
                self.reset_fixture()
                if kind == "configured":
                    self.env_file.write_text(ENV.replace("CP_PROVIDER_MSP_LICENSE_FILE=\n", "CP_PROVIDER_MSP_LICENSE_FILE=./operator.jwt\n"))
                else:
                    self.eval_file.write_text(LICENSE)
                    self.eval_file.chmod(0o600)
                self.run_setup()
                self.assertEqual(self.requests(), [])
                self.assertFalse((self.root / "derivations").exists())
                self.assertIn("CP_ENTITLEMENT_SIGNING_PRIVATE_KEY=" + PRIVATE, self.env_file.read_text())

    def test_skip_and_derivation_failure_still_do_not_issue(self):
        for skip, derive in (("1", "ok"), ("0", "fail")):
            with self.subTest(skip=skip, derive=derive):
                self.reset_fixture()
                output = self.run_setup(skip=skip, derive=derive)
                self.assertNotIn("evaluation license installed", output)
                self.assertEqual(self.requests(), [])
                self.assertFalse(self.eval_file.exists())
                self.assertEqual(self.env_file.read_text(), ENV)

    def test_explicit_retry_can_install_without_resetting_saved_state(self):
        output = self.run_setup(status="403")
        self.assert_unlicensed(output)
        self.run_setup()
        self.assertEqual(len(self.requests()), 2, "one request per deliberate invocation")
        self.assertEqual(self.eval_file.read_text(), LICENSE)
        self.run_setup()
        self.assertEqual(len(self.requests()), 2, "installed licence must not be reissued")

    def test_final_summary_does_not_turn_rejection_into_capability_success(self):
        output = self.run_setup(status="403", summary=True)
        self.assert_unlicensed(output)
        self.assertIn("running portal alone does not confirm active client MSP capabilities", output)
        self.assertIn("check Plan in the portal for this deployment's licence status", output)
        self.assertNotIn("Plan: the evaluation covers two clients", output)


if __name__ == "__main__":
    unittest.main()
