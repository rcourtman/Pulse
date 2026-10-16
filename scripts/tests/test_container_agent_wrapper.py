#!/usr/bin/env python3
"""Execute the complete legacy wrapper with an inert installer and closed curl.

No network listener, provider, host installer, daemon or service is used. Curl
fixtures prove argument policy and complete-download-before-execution ordering,
not a real TLS handshake or installed agent recovery.
"""

import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(os.environ.get("PULSE_CONTAINER_WRAPPER_SOURCE",
                             ROOT / "scripts/install-container-agent.sh"))
BASH = shutil.which("bash")
PRIVATE = "synthetic-private-provider-detail"
TOKEN = "synthetic-wrapper-token"
INSTALLER = r'''#!/usr/bin/env bash
python3 -I -c 'import json, os, pathlib, sys
p = pathlib.Path(sys.argv[1])
mask = os.umask(0)
os.umask(mask)
pathlib.Path(os.environ["INSTALL_LOG"]).write_text(json.dumps({
    "args": sys.argv[2:], "file_mode": p.stat().st_mode & 0o777 if p.is_file() else None,
    "parent_mode": p.parent.stat().st_mode & 0o777 if p.is_file() else None, "umask": mask}))
' "$0" "$@"
exit "${INSTALL_EXIT:-0}"
'''
CURL = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
args = sys.argv[1:]
root = pathlib.Path(os.environ['WRAPPER_FIXTURE'])
(root / 'curl.json').write_text(json.dumps(args))
settings = json.loads((root / 'settings.json').read_text())
body = settings['body']
if '--output' in args:
    pathlib.Path(args[args.index('--output') + 1]).write_text(body)
    sys.stdout.write(settings['status'])
else:
    # Exact-parent pipe control: executable bytes arrive even when the
    # transfer subsequently fails. The fixture never calls the real curl.
    sys.stdout.write(body)
sys.stdout.flush()
sys.stderr.write(settings['stderr'])
sys.exit(settings['curl_exit'])
'''


class ContainerAgentWrapperTest(unittest.TestCase):
    def exercise(self, args=(), *, local=False, status="200", curl_exit=0,
                 body=INSTALLER, install_exit=0, provider_error=False):
        with tempfile.TemporaryDirectory(prefix="pulse-wrapper-control-") as temporary:
            root = Path(temporary)
            scripts, tools, scratch = root / "scripts", root / "tools", root / "tmp"
            for directory in (scripts, tools, scratch):
                directory.mkdir()
            wrapper = scripts / "install-container-agent.sh"
            wrapper.write_bytes(SOURCE.read_bytes())
            if local:
                installer = scripts / "install.sh"
                installer.write_text(INSTALLER)
                installer.chmod(0o700)
            curl = tools / "curl"
            curl.write_text(CURL)
            curl.chmod(0o700)
            (root / "settings.json").write_text(json.dumps({
                "status": status, "curl_exit": curl_exit, "body": body,
                "stderr": PRIVATE + "\n" if provider_error else "",
            }))
            # These would weaken or trace a real curl invocation if the
            # wrapper admitted ambient configuration/proxies. Here they are
            # inert; the observed flags must explicitly exclude both.
            (root / ".curlrc").write_text("insecure\nlocation\nverbose\n")
            ca = root / "verified private CA.pem"
            ca.write_text("synthetic CA fixture; never passed to real curl\n")
            args = [str(ca) if arg == "{ca}" else arg for arg in args]
            env = dict(os.environ, PATH=str(tools) + os.pathsep + os.environ["PATH"],
                       TMPDIR=str(scratch), HOME=str(root), CURL_HOME=str(root),
                       HTTPS_PROXY="http://ambient.invalid:99", ALL_PROXY="http://ambient.invalid:99",
                       WRAPPER_FIXTURE=str(root), INSTALL_LOG=str(root / "install.json"),
                       INSTALL_EXIT=str(install_exit), BASH_ENV="", ENV="")
            result = subprocess.run([BASH, str(wrapper), *args], env=env,
                                    capture_output=True, text=True, timeout=5)
            output = result.stdout + result.stderr
            self.assertNotIn(PRIVATE, output)
            self.assertNotIn(TOKEN, output)
            calls = json.loads((root / "curl.json").read_text()) if (root / "curl.json").exists() else None
            installed = json.loads((root / "install.json").read_text()) if (root / "install.json").exists() else None
            self.assertEqual(list(scratch.iterdir()), [], "bootstrap left owned temporary files")
            return result.returncode, output, calls, installed, args

    def assert_refused(self, args, **settings):
        code, output, curl, installed, _ = self.exercise(args, **settings)
        self.assertNotEqual(code, 0)
        self.assertIsNone(installed, "refused/partial installer executed")
        return output, curl

    def test_complete_download_forwards_the_existing_host_and_docker_profile(self):
        code, _, curl, installed, _ = self.exercise([
            "--url", "https://pulse.invalid/base/", "--token", TOKEN, "--interval", "45s"])
        self.assertEqual(code, 0)
        self.assertEqual(curl[-2:], ["--url", "https://pulse.invalid/base/install.sh"])
        self.assertEqual(installed["args"], ["--url", "https://pulse.invalid/base/",
                                           "--token", TOKEN, "--interval", "45s",
                                           "--enable-docker", "--enable-host"])

    def test_remote_curl_keeps_trust_deadlines_and_ambient_isolation(self):
        code, _, curl, _, _ = self.exercise(["--url", "https://pulse.invalid"])
        self.assertEqual(code, 0)
        self.assertEqual(curl[0], "--disable")
        for option, value in (("--proto", "=https"), ("--noproxy", "*"),
                              ("--connect-timeout", "5"), ("--max-time", "60"),
                              ("--write-out", "%{http_code}")):
            self.assertEqual(curl[curl.index(option) + 1], value)
        for option in ("--globoff", "--fail", "--silent", "--show-error", "--output"):
            self.assertIn(option, curl)
        for option in ("-k", "--insecure", "-L", "--location", "--location-trusted", "--retry"):
            self.assertNotIn(option, curl)

    def test_ca_bundle_is_used_for_bootstrap_and_preserved_for_the_installer(self):
        code, _, curl, installed, args = self.exercise([
            "--url", "https://pulse.invalid", "--cacert", "{ca}"])
        self.assertEqual(code, 0)
        self.assertEqual(curl[curl.index("--cacert") + 1], args[-1])
        self.assertEqual(installed["args"], [*args, "--enable-docker", "--enable-host"])

    def test_inline_legacy_token_never_enters_the_download_arguments(self):
        code, _, curl, installed, _ = self.exercise([
            "--url", "https://pulse.invalid", "--token", TOKEN])
        self.assertEqual(code, 0)
        self.assertNotIn(TOKEN, " ".join(curl))
        self.assertIn(TOKEN, installed["args"])  # Legacy downstream interface is unchanged.

    def test_download_and_execution_paths_are_private(self):
        code, _, _, installed, _ = self.exercise(["--url", "https://pulse.invalid"])
        self.assertEqual(code, 0)
        self.assertEqual(installed["file_mode"], stat.S_IRUSR | stat.S_IWUSR)
        self.assertEqual(installed["parent_mode"], stat.S_IRWXU)
        _, _, _, local, _ = self.exercise(local=True)
        self.assertEqual(installed["umask"], local["umask"], "bootstrap changed downstream file modes")

    def test_partial_executable_bodies_do_not_run_on_any_curl_failure(self):
        for code in (7, 18, 22, 28, 35, 60):
            with self.subTest(curl_exit=code):
                self.assert_refused(["--url", "https://pulse.invalid"], curl_exit=code)

    def test_redirect_bodies_are_not_executed(self):
        for status in ("301", "302", "303", "307", "308"):
            with self.subTest(status=status):
                self.assert_refused(["--url", "https://pulse.invalid"], status=status,
                                    provider_error=True)

    def test_only_exact_http_200_is_accepted_without_echoing_provider_output(self):
        for status in ("000", "204", "206", "401", "403", "404", "500", "200\n" + PRIVATE):
            with self.subTest(status=status):
                self.assert_refused(["--url", "https://pulse.invalid"], status=status,
                                    provider_error=True)

    def test_empty_success_is_not_an_installer(self):
        self.assert_refused(["--url", "https://pulse.invalid"], body="")

    def test_remote_insecure_is_refused_before_downloading(self):
        output, curl = self.assert_refused(["--url", "https://pulse.invalid", "--insecure"])
        self.assertIsNone(curl)
        self.assertIn("--cacert", output)

    def test_remote_plain_http_is_refused_before_downloading(self):
        _, curl = self.assert_refused(["--url", "http://pulse.invalid"])
        self.assertIsNone(curl)

    def test_remote_untrusted_url_shapes_never_reach_curl(self):
        for url in ("https:///missing", "https://", "https://user:" + TOKEN + "@pulse.invalid",
                    "https://pulse.invalid/path?" + TOKEN, "https://pulse.invalid/#" + TOKEN,
                    "https://pulse.invalid/line\nbreak", "https://pulse.invalid/a b",
                    "https://pulse.invalid/\\escape", "file:///tmp/install.sh"):
            with self.subTest(url=url):
                _, curl = self.assert_refused(["--url", url])
                self.assertIsNone(curl)

    def test_https_ipv6_and_base_path_remain_usable(self):
        for url in ("https://[2001:db8::1]:8000/pulse", "https://pulse.invalid:8443/base",
                    "HTTPS://pulse.invalid/base"):
            with self.subTest(url=url):
                code, _, curl, installed, _ = self.exercise(["--url", url])
                self.assertEqual(code, 0)
                self.assertEqual(curl[-1], url + "/install.sh")
                self.assertIsNotNone(installed)

    def test_downstream_exit_is_preserved_and_files_are_cleaned(self):
        for code in (0, 17):
            with self.subTest(installer_exit=code):
                result, _, _, installed, _ = self.exercise(
                    ["--url", "https://pulse.invalid"], install_exit=code)
                self.assertEqual(result, code)
                self.assertIsNotNone(installed)

    def test_local_installer_preserves_explicit_transport_and_arguments(self):
        args = ["--url", "http://pulse.invalid", "--token", TOKEN, "--interval", "45s",
                "--cacert", "{ca}", "--insecure"]
        code, _, curl, installed, expanded = self.exercise(args, local=True)
        self.assertEqual(code, 0)
        self.assertIsNone(curl)
        self.assertEqual(installed["args"], [*expanded, "--enable-docker", "--enable-host"])

    def test_local_installer_needs_no_url_and_keeps_its_exit(self):
        code, _, curl, installed, _ = self.exercise(local=True, install_exit=19)
        self.assertEqual(code, 19)
        self.assertIsNone(curl)
        self.assertEqual(installed["args"], ["--enable-docker", "--enable-host"])

    def test_uninstall_never_silently_adds_monitoring_modules(self):
        for local in (False, True):
            with self.subTest(local=local):
                code, _, _, installed, _ = self.exercise(
                    ["--url", "https://pulse.invalid", "--uninstall"], local=local)
                self.assertEqual(code, 0)
                self.assertEqual(installed["args"], ["--url", "https://pulse.invalid", "--uninstall"])

    def test_missing_values_fail_before_local_execution_or_download(self):
        for option in ("--url", "--token", "--interval", "--cacert"):
            for value in ([], [""], ["--uninstall"]):
                with self.subTest(option=option, value=value):
                    output, curl = self.assert_refused([option, *value], local=True)
                    self.assertIsNone(curl)
                    self.assertIn("requires a non-empty value", output)

    def test_help_is_an_early_read_only_exit(self):
        code, output, curl, installed, _ = self.exercise(["--help"])
        self.assertEqual(code, 0)
        self.assertIn("--enable-host=false for an intentional", output)
        self.assertIn("complete HTTP 200", output)
        self.assertIsNone(curl)
        self.assertIsNone(installed)

    def test_unsupported_input_is_not_echoed(self):
        _, curl = self.assert_refused([TOKEN])
        self.assertIsNone(curl)

    def test_missing_remote_url_never_downloads(self):
        output, curl = self.assert_refused([])
        self.assertIn("--url is required", output)
        self.assertIsNone(curl)


if __name__ == "__main__":
    unittest.main()
