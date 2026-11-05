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

# The shared limit path: Traefik logs a refused order, setup recreates it and asks
# again. The fake answers from a queue, one answer per logs call after a recreate.
TRIAL_HARNESS = r'''
set -euo pipefail
source "$SETUP_FUNCTIONS"
log() { printf 'LOG %s\n' "$*"; }
sleep() { :; }
docker() {
    [[ "$1" == compose ]] || exit 98
    case "$2" in
        logs) printf '%s\n' "$ACME_LOG" ;;
        up) printf 'UP DOMAIN=%s\n' "$(env_value DOMAIN)" >> "$CALLS"
            if [[ -n "${ISSUE_ON_DOMAIN:-}" && "$(env_value DOMAIN)" == "$ISSUE_ON_DOMAIN" ]]; then
                cp "$ISSUED_CERTIFICATE" "$CERTIFICATE"
            fi ;;
        *) exit 98 ;;
    esac
}
TRIAL_CERTIFICATE_ATTEMPTS=2
FIRST_RUN_ENV_CREATED="${FIRST_RUN:-1}"
check_portal_readiness
printf 'READY=%s\n' "$PORTAL_CERTIFICATE_READY"
printf 'FINAL_DOMAIN=%s\n' "$(env_value DOMAIN)"
'''

# First-run values on a host with no terminal: what reaches .env.
FIRST_RUN_HARNESS = r'''
set -euo pipefail
source "$SETUP_FUNCTIONS"
log() { :; }
curl() { printf '%s\n' "$FAKE_PUBLIC_IP"; }
collect_first_run_values
ensure_env_file
cat "$PULSE_PROVIDER_MSP_INSTALL_DIR/.env"
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

    def per_client_fixture(self, domain=DOMAIN, trial=False):
        env = self.fixture / ".env"
        original = env.read_text()
        self.addCleanup(env.write_text, original)
        lines = [f"DOMAIN={domain}", "CP_PER_CLIENT_CERTIFICATES=true"]
        if trial:
            lines.append("PROVIDER_MSP_TRIAL_ADDRESS=true")
        env.write_text("\n".join(lines) + "\n")

    def test_per_client_certificates_need_only_the_portal_name(self):
        self.per_client_fixture()
        output, _ = self.report("apex-only")
        self.assertIn("READY=1", output)
        self.assertIn(f"Certificate: verified on this host for {DOMAIN}.", output)
        self.assertIn("its own certificate", output)
        self.assertNotIn("NOT READY", output)

    def test_per_client_certificates_still_verify_the_certificate(self):
        self.per_client_fixture()
        for certificate in ("untrusted", "expired", "wrong-host", "client-only"):
            with self.subTest(certificate=certificate):
                output, _ = self.report(certificate)
                self.assert_not_ready(output)

    def test_wildcard_mode_still_refuses_a_name_only_certificate(self):
        output, _ = self.report("apex-only")
        self.assert_not_ready(output)

    def test_http_challenge_failures_are_explained(self):
        self.per_client_fixture()
        cases = {
            "acme: error: 400 :: urn:ietf:params:acme:error:connection :: 203.0.113.7: Fetching http://x: Timeout during connect (likely firewall problem)":
                "could not reach port 80",
            "urn:ietf:params:acme:error:connection :: 203.0.113.7: Connection refused": "could not reach port 80",
            "urn:ietf:params:acme:error:unauthorized :: Invalid response from http://x": "something other than this server answered",
            "urn:ietf:params:acme:error:dns :: DNS problem: NXDOMAIN looking up A for x": "could not look up this name",
            "urn:ietf:params:acme:error:rateLimited :: too many failed authorizations (5) for x": "it allows 5 an hour",
            'urn:ietf:params:acme:error:rateLimited :: too many certificates (250000) already issued for "example.org"':
                "Let's Encrypt is rate limiting",
        }
        for error, explanation in cases.items():
            with self.subTest(error=error):
                output, _ = self.report(fault="no-certificate", acme_log=f'Unable to obtain ACME certificate error="{error}"')
                self.assert_not_ready(output)
                self.assertIn(explanation, output)

    def trial_report(self, domain, *, issue_on="", first_run="1", acme_log=None):
        self.per_client_fixture(domain, trial=True)
        calls = self.fixture / "calls"
        calls.write_text("")
        # A certificate for each trial name, trusted by the fixture CA.
        working = self.fixture / "trial-current.pem"
        issued = self.fixture / "trial-issued.pem"
        if issue_on:
            csr = self.fixture / "trial.csr"
            self.command("req", "-new", "-key", str(self.leaf_key), "-subj", f"/CN={issue_on}", "-out", str(csr))
            extensions = self.fixture / "trial.ext"
            extensions.write_text(f"basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:{issue_on}\nextendedKeyUsage=serverAuth\n")
            self.command("x509", "-req", "-in", str(csr), "-CA", str(self.ca), "-CAkey", str(self.ca_key),
                         "-set_serial", "99", "-days", "1", "-extfile", str(extensions), "-out", str(issued))
        shutil.copy(self.fixture / "wrong-host.pem", working)
        env = {
            **os.environ,
            "PATH": str(self.bin) + os.pathsep + os.environ["PATH"],
            "REAL_OPENSSL": self.openssl,
            "CERTIFICATE": str(working),
            "ISSUED_CERTIFICATE": str(issued),
            "ISSUE_ON_DOMAIN": issue_on,
            "FIXTURE_CA": str(self.ca),
            "CALLS": str(calls),
            "SETUP_FUNCTIONS": str(self.functions),
            "PULSE_PROVIDER_MSP_INSTALL_DIR": str(self.fixture),
            "PULSE_PROVIDER_MSP_CERT_WAIT_SECONDS": "0",
            "FIRST_RUN": first_run,
            "FAULT": "",
            "MISSING_DNS": "",
            "ACME_LOG": acme_log if acme_log is not None else (
                'Unable to obtain ACME certificate error="urn:ietf:params:acme:error:rateLimited :: '
                f'too many certificates (250000) already issued for \\"{domain.split(".", 2)[2]}\\""'),
        }
        result = subprocess.run(["bash", "-c", TRIAL_HARNESS], env=env, capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result.stdout, calls.read_text()

    def test_shared_limit_retries_and_succeeds_on_the_same_service(self):
        domain = "pulse.203-0-113-7.sslip.io"
        output, calls = self.trial_report(domain, issue_on=domain)
        self.assertIn("READY=1", output)
        self.assertIn(f"FINAL_DOMAIN={domain}", output)
        self.assertEqual(calls.count("UP "), 1)

    def test_shared_limit_falls_back_to_the_other_service_on_a_first_run(self):
        output, calls = self.trial_report("pulse.203-0-113-7.sslip.io", issue_on="pulse.203-0-113-7.nip.io")
        self.assertIn("READY=1", output)
        self.assertIn("FINAL_DOMAIN=pulse.203-0-113-7.nip.io", output)
        self.assertIn("UP DOMAIN=pulse.203-0-113-7.nip.io", calls)

    def test_shared_limit_never_renames_an_install_that_may_have_clients(self):
        output, calls = self.trial_report("pulse.203-0-113-7.sslip.io", first_run="0")
        self.assertIn("READY=0", output)
        self.assertIn("FINAL_DOMAIN=pulse.203-0-113-7.sslip.io", output)
        self.assertNotIn("nip.io", calls)
        self.assertIn("Everyone using sslip.io or nip.io shares it", output)

    def test_shared_limit_rerun_keeps_asking_on_its_current_service(self):
        output, calls = self.trial_report("pulse.203-0-113-7.nip.io", issue_on="pulse.203-0-113-7.nip.io", first_run="0")
        self.assertIn("READY=1", output)
        self.assertIn("FINAL_DOMAIN=pulse.203-0-113-7.nip.io", output)
        self.assertEqual(calls.count("UP "), 1)

    def test_shared_limit_exhausted_on_both_services_is_reported(self):
        output, calls = self.trial_report("pulse.203-0-113-7.sslip.io")
        self.assertIn("READY=0", output)
        self.assertIn("NOT READY YET:", output)
        self.assertEqual(calls.count("UP "), 4)

    def test_port_80_failure_during_retry_stops_asking(self):
        # A blocked port is not a limit: repeating it only burns the 5 tries an hour.
        output, calls = self.trial_report(
            "pulse.203-0-113-7.sslip.io",
            acme_log='Unable to obtain ACME certificate error="acme:error:connection :: Timeout during connect"',
        )
        self.assertIn("could not reach port 80", output)
        self.assertNotIn("UP ", calls)

    def first_run(self, **values):
        with tempfile.TemporaryDirectory(prefix="pulse-msp-first-run-") as directory:
            install = Path(directory)
            (install / ".env.example").write_text(
                "DOMAIN=msp.example.com\nACME_EMAIL=admin@example.com\nCF_DNS_API_TOKEN=\n"
                "ACME_CHALLENGE_SETTING_1=\nACME_CHALLENGE_SETTING_2=\nPORTAL_CERTIFICATE_SANS_LABEL=\n"
                "CP_PER_CLIENT_CERTIFICATES=false\nPROVIDER_MSP_TRIAL_ADDRESS=false\n")
            env = {
                **os.environ,
                "SETUP_FUNCTIONS": str(self.functions),
                "PULSE_PROVIDER_MSP_INSTALL_DIR": str(install),
                "FAKE_PUBLIC_IP": "203.0.113.7",
                **values,
            }
            return subprocess.run(["bash", "-c", FIRST_RUN_HARNESS], env=env, stdin=subprocess.DEVNULL,
                                  capture_output=True, text=True, timeout=15)

    def test_trial_address_needs_no_domain_or_token(self):
        result = self.first_run(PULSE_PROVIDER_MSP_TRIAL_ADDRESS="1", PULSE_PROVIDER_MSP_ACME_EMAIL="you@pulse-test.dev")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        for line in ("DOMAIN=pulse.203-0-113-7.sslip.io", "ACME_EMAIL=you@pulse-test.dev",
                     "ACME_CHALLENGE_SETTING_1=httpchallenge=true", "ACME_CHALLENGE_SETTING_2=httpchallenge.entrypoint=web",
                     "PORTAL_CERTIFICATE_SANS_LABEL=pulse.provider-msp.client-certificates=per-name",
                     "CP_PER_CLIENT_CERTIFICATES=true", "PROVIDER_MSP_TRIAL_ADDRESS=true"):
            self.assertIn(line + "\n", result.stdout)

    def test_own_domain_without_a_token_is_not_labelled_a_trial(self):
        result = self.first_run(PULSE_PROVIDER_MSP_DOMAIN="pulse.pulse-test.dev", PULSE_PROVIDER_MSP_ACME_CHALLENGE="http",
                                PULSE_PROVIDER_MSP_ACME_EMAIL="you@pulse-test.dev")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("DOMAIN=pulse.pulse-test.dev\n", result.stdout)
        self.assertIn("CP_PER_CLIENT_CERTIFICATES=true\n", result.stdout)
        self.assertIn("PROVIDER_MSP_TRIAL_ADDRESS=false\n", result.stdout)
        self.assertIn("*.pulse.pulse-test.dev", result.stdout)

    def test_dns_token_keeps_dns_01_and_the_wildcard(self):
        result = self.first_run(PULSE_PROVIDER_MSP_DOMAIN="pulse.pulse-test.dev", PULSE_PROVIDER_MSP_ACME_EMAIL="you@pulse-test.dev",
                                PULSE_PROVIDER_MSP_CF_DNS_API_TOKEN="a" * 40)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("ACME_CHALLENGE_SETTING_1=\n", result.stdout)
        self.assertIn("PORTAL_CERTIFICATE_SANS_LABEL=\n", result.stdout)
        self.assertIn("CP_PER_CLIENT_CERTIFICATES=false\n", result.stdout)

    def test_trial_address_refuses_a_host_with_no_public_ip(self):
        for address in ("192.168.0.97", "10.1.2.3", "172.16.0.1", "100.64.0.1", "127.0.0.1", "169.254.1.1", "not-an-ip", "300.1.1.1"):
            with self.subTest(address=address):
                result = self.first_run(PULSE_PROVIDER_MSP_TRIAL_ADDRESS="1", FAKE_PUBLIC_IP=address)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("could not find a public IPv4 address", result.stderr)

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
