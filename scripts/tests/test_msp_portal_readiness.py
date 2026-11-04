#!/usr/bin/env python3
"""Exercise setup's real readiness report with offline certificate controls.

Only s_client's transport is replaced. Its verification behaviour is modelled
with real OpenSSL verification against a disposable fixture CA. No listener,
DNS request, Docker operation, installation or external ACME request is made.
"""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SETUP = Path(os.environ.get("PULSE_MSP_SETUP_UNDER_TEST", ROOT / "deploy/provider-msp/setup.sh"))
DOMAIN = "msp.example.test"

OPENSSL_TRANSPORT = r'''#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" != s_client ]]; then
    exec "$REAL_OPENSSL" "$@"
fi
printf '%s\n' "$*" >> "$CALLS"
hostname=""
fail_on_verify_error=0
while (( $# )); do
    case "$1" in
        -verify_hostname) hostname="$2"; shift ;;
        -verify_return_error) fail_on_verify_error=1 ;;
    esac
    shift
done
[[ "$FAULT" != no-certificate ]] || exit 1
# s_client can emit the peer certificate even when verification fails.
cat "$CERTIFICATE"
[[ "$FAULT" != transport-timeout ]] || exit 124
args=(verify -CAfile "$FIXTURE_CA" -purpose sslserver)
[[ -z "$hostname" ]] || args+=(-verify_hostname "$hostname")
if ! "$REAL_OPENSSL" "${args[@]}" "$CERTIFICATE" >/dev/null 2>&1; then
    (( fail_on_verify_error == 0 )) || exit 1
fi
'''

HARNESS = r'''
set -euo pipefail
source "$SETUP_FUNCTIONS"
log() { :; }
sleep() { :; }
docker() {
    [[ "$#" == 4 && "$1" == compose && "$2" == logs && "$3" == --no-color && "$4" == traefik ]] || exit 98
    printf '%s\n' "$ACME_LOG"
}
check_portal_readiness
printf 'READY=%s\n' "$PORTAL_CERTIFICATE_READY"
'''


class PortalReadinessTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.directory = tempfile.TemporaryDirectory(prefix="pulse-msp-tls-")
        cls.addClassCleanup(cls.directory.cleanup)
        cls.fixture = Path(cls.directory.name)
        cls.openssl = shutil.which("openssl")
        if not cls.openssl:
            raise RuntimeError("OpenSSL is required for the certificate controls")
        source = SETUP.read_text()
        invocation = '\nmain "$@"\n'
        if source.count(invocation) != 1 or not source.endswith(invocation):
            raise AssertionError("setup must have exactly one final main invocation")
        cls.functions = cls.fixture / "setup-functions.sh"
        cls.functions.write_text(source[:-len(invocation)] + "\n")
        cls.bin = cls.fixture / "bin"
        cls.bin.mkdir()
        wrapper = cls.bin / "openssl"
        wrapper.write_text(OPENSSL_TRANSPORT)
        wrapper.chmod(0o755)
        dns = cls.bin / "getent"
        dns.write_text('''#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == ahostsv4 ]] || exit 98
case "$MISSING_DNS:$2" in
    all:*|apex:msp.example.test|wildcard:pulse-dns-check.msp.example.test) exit 2 ;;
esac
printf '192.0.2.10 STREAM %s\\n' "$2"
''')
        dns.chmod(0o755)
        (cls.fixture / ".env").write_text(f"DOMAIN={DOMAIN}\n")
        cls.ca = cls.fixture / "ca.pem"
        cls.ca_key = cls.fixture / "ca.key"
        cls.leaf_key = cls.fixture / "leaf.key"
        cls.command("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2",
                    "-subj", "/CN=Pulse readiness test CA", "-keyout", str(cls.ca_key), "-out", str(cls.ca),
                    "-addext", "basicConstraints=critical,CA:TRUE")
        cls.command("genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", str(cls.leaf_key))
        csr = cls.fixture / "leaf.csr"
        cls.command("req", "-new", "-key", str(cls.leaf_key), "-subj", f"/CN={DOMAIN}", "-out", str(csr))
        variants = {
            "valid": (f"DNS:{DOMAIN},DNS:*.{DOMAIN}", "serverAuth", "1"),
            "suffix": (f"DNS:{DOMAIN},DNS:*.{DOMAIN}.invalid", "serverAuth", "1"),
            "apex-only": (f"DNS:{DOMAIN}", "serverAuth", "1"),
            "wildcard-only": (f"DNS:*.{DOMAIN}", "serverAuth", "1"),
            "wrong-host": ("DNS:other.example.test,DNS:*.other.example.test", "serverAuth", "1"),
            "expired": (f"DNS:{DOMAIN},DNS:*.{DOMAIN}", "serverAuth", "-1"),
            "client-only": (f"DNS:{DOMAIN},DNS:*.{DOMAIN}", "clientAuth", "1"),
        }
        for serial, (name, (sans, purpose, days)) in enumerate(variants.items(), 1):
            extensions = cls.fixture / f"{name}.ext"
            extensions.write_text(f"basicConstraints=critical,CA:FALSE\nsubjectAltName={sans}\nextendedKeyUsage={purpose}\n")
            cls.command("x509", "-req", "-in", str(csr), "-CA", str(cls.ca), "-CAkey", str(cls.ca_key),
                        "-set_serial", str(serial), "-days", days, "-extfile", str(extensions),
                        "-out", str(cls.fixture / f"{name}.pem"))
        cls.command("req", "-x509", "-key", str(cls.leaf_key), "-days", "1", "-subj", f"/CN={DOMAIN}",
                    "-addext", f"subjectAltName=DNS:{DOMAIN},DNS:*.{DOMAIN}",
                    "-out", str(cls.fixture / "untrusted.pem"))

    @classmethod
    def command(cls, *args):
        subprocess.run([cls.openssl, *args], check=True, capture_output=True, timeout=15)

    def report(self, certificate="valid", *, fault="", missing_dns="", acme_log=""):
        calls = self.fixture / "calls"
        calls.write_text("")
        env = {
            **os.environ,
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "REAL_OPENSSL": self.openssl,
            "CERTIFICATE": str(self.fixture / f"{certificate}.pem"),
            "FIXTURE_CA": str(self.ca),
            "CALLS": str(calls),
            "SETUP_FUNCTIONS": str(self.functions),
            "PULSE_PROVIDER_MSP_INSTALL_DIR": str(self.fixture),
            "PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS": "0",
            "FAULT": fault,
            "MISSING_DNS": missing_dns,
            "ACME_LOG": acme_log,
        }
        result = subprocess.run(["bash", "-c", HARNESS], env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(result.stderr, "")
        return result.stdout, calls.read_text()

    def assert_not_ready(self, output):
        self.assertIn("READY=0", output)
        self.assertIn("NOT READY YET:", output)
        self.assertNotIn("Certificate:", output)
        self.assertNotIn("BEGIN CERTIFICATE", output)
        self.assertIn("sudo nano", output)
        self.assertIn("--force-recreate traefik control-plane", output)

    def test_trusted_current_apex_and_exact_wildcard_are_ready(self):
        output, calls = self.report()
        self.assertIn("READY=1", output)
        self.assertIn(f"Certificate: verified on this host for {DOMAIN} and *.{DOMAIN}.", output)
        self.assertNotIn("NOT READY", output)
        self.assertIn("-connect 127.0.0.1:443", calls)
        self.assertIn(f"-servername {DOMAIN}", calls)
        self.assertIn("-verify_return_error", calls)
        self.assertIn(f"-verify_hostname {DOMAIN}", calls)

    def test_untrusted_self_signed_wildcard_is_not_ready(self):
        output, _ = self.report("untrusted")
        self.assert_not_ready(output)
        self.assertIn("verified TLS certificate", output)

    def test_expired_server_certificate_is_not_ready(self):
        output, _ = self.report("expired")
        self.assert_not_ready(output)

    def test_client_only_certificate_is_not_ready(self):
        output, _ = self.report("client-only")
        self.assert_not_ready(output)

    def test_wrong_apex_or_missing_exact_wildcard_is_not_ready(self):
        for certificate in ("wrong-host", "wildcard-only", "apex-only", "suffix"):
            with self.subTest(certificate=certificate):
                output, _ = self.report(certificate)
                self.assert_not_ready(output)

    def test_no_certificate_and_transport_timeout_are_not_ready(self):
        for fault in ("no-certificate", "transport-timeout"):
            with self.subTest(fault=fault):
                output, _ = self.report(fault=fault)
                self.assert_not_ready(output)

    def test_resolving_both_names_remains_required(self):
        for missing_dns in ("apex", "wildcard", "all"):
            with self.subTest(missing_dns=missing_dns):
                output, _ = self.report(missing_dns=missing_dns)
                self.assertIn("READY=1", output)  # Certificate is ready, DNS is not.
                self.assertIn("NOT READY YET:", output)
                self.assertIn("cannot resolve", output)
                self.assertNotIn("Certificate:", output)
                self.assertNotIn("No verified TLS certificate", output)

    def test_acme_failure_guidance_is_preserved(self):
        cases = {
            "invalidContact": "Let's Encrypt refused ACME_EMAIL",
            "Cloudflare denied token": "Zone / Zone / Read and Zone / DNS / Edit",
            "rateLimited": "Let's Encrypt is rate limiting",
            "NXDOMAIN": "DNS challenge record was not visible",
        }
        for error, explanation in cases.items():
            with self.subTest(error=error):
                output, _ = self.report(fault="no-certificate", acme_log=f'Unable to obtain ACME certificate error="{error}"')
                self.assert_not_ready(output)
                self.assertIn(explanation, output)


if __name__ == "__main__":
    unittest.main()
