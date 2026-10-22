#!/usr/bin/env python3
"""Run the complete read-only agent preflight with closed command doubles.

This checks the curl deadline contract and caller behaviour, not libcurl's
timeout implementation. The Go HTTP fixture covers that separately in the
offline source-proof VM. No listener, provider or real service manager is used.
"""

import json
from pathlib import Path
import subprocess
import tempfile
import unittest


INSTALLER = Path(__file__).resolve().parents[1] / "install.sh"
CURL = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
root = Path(os.environ["FIXTURE"])
with (root / "curl-calls").open("a") as out:
    out.write(json.dumps(args) + "\n")
if args[-1].endswith("/api/health"):
    sys.exit(int(os.environ["HEALTH_EXIT"]))
if "/download/pulse-agent?arch=" in args[-1]:
    Path(args[args.index("-D") + 1]).write_text(
        "HTTP/1.1 200 OK\r\nX-Checksum-Sha256: " + "a" * 64 + "\r\n\r\n")
    sys.exit(0)
sys.exit(97)
'''


class PreflightDeadlineTests(unittest.TestCase):
    def exercise(self, health_exit, trust):
        with tempfile.TemporaryDirectory() as raw:
            root = Path(raw)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            # Darwin avoids Linux/NAS auto-discovery. Neither service control
            # nor platform mutations are admissible in this fixture.
            stubs = {
                "curl": CURL,
                "uname": '#!/bin/sh\ncase "$1" in -s) echo Darwin;; -m) echo x86_64;; *) exit 97;; esac\n',
                "systemctl": '#!/bin/sh\n[ "$*" = "is-active --quiet pulse-agent" ] && exit 3\nprintf mutation >> "$FIXTURE/mutations"\nexit 97\n',
            }
            for name in ("sudo", "launchctl", "service", "rc-service", "install", "chown"):
                stubs[name] = '#!/bin/sh\nprintf mutation >> "$FIXTURE/mutations"\nexit 97\n'
            for name, content in stubs.items():
                path = bin_dir / name
                path.write_text(content)
                path.chmod(0o700)
            state = root / "state"
            ca = root / "explicit-ca.pem"
            ca.write_text("fixture-only CA selection\n")
            trust_args = {"verified": [], "insecure": ["--insecure"],
                          "ca": ["--cacert", str(ca)]}[trust]
            env = {"PATH": str(bin_dir) + ":/usr/bin:/bin", "HOME": raw,
                   "TMPDIR": raw, "LC_ALL": "C", "TERM": "dumb",
                   "FIXTURE": raw, "HEALTH_EXIT": str(health_exit)}
            result = subprocess.run(
                ["bash", str(INSTALLER), "--url", "https://pulse.invalid",
                 "--preflight-only", "--output", "json", "--non-interactive",
                 "--disable-docker", "--disable-kubernetes", "--disable-proxmox",
                 "--state-dir", str(state), *trust_args],
                env=env, capture_output=True, text=True, timeout=5,
            )
            self.assertFalse((root / "mutations").exists(), result.stdout + result.stderr)
            self.assertFalse(state.exists(), "preflight created installed state")
            self.assertEqual(ca.read_text(), "fixture-only CA selection\n")
            calls = [json.loads(row) for row in (root / "curl-calls").read_text().splitlines()]
            self.assertEqual(len(calls), 2, calls)
            health, artifact = calls
            self.assertEqual(health[0], "--disable")
            self.assertEqual(health[health.index("--connect-timeout") + 1], "5")
            self.assertEqual(health[health.index("--max-time") + 1], "10")
            self.assertNotIn("-I", health)
            self.assertTrue(health[-1].endswith("/api/health"))
            self.assertEqual(artifact[0], "--disable")
            self.assertEqual(artifact[artifact.index("--max-time") + 1], "30")
            self.assertIn("-fsSIL", artifact)
            self.assertTrue(artifact[-1].endswith("/download/pulse-agent?arch=darwin-amd64"))
            for call in calls:
                self.assertEqual("-k" in call, trust == "insecure")
                self.assertEqual("--cacert" in call, trust == "ca")
                if trust == "ca":
                    self.assertEqual(call[call.index("--cacert") + 1], str(ca))
            events = [json.loads(row) for row in result.stdout.splitlines() if row.startswith("{")]
            return result, events

    def test_timeout_fails_without_installing_and_still_checks_artifact(self):
        for trust in ("verified", "insecure", "ca"):
            with self.subTest(trust=trust):
                result, events = self.exercise(28, trust)
                self.assertEqual(result.returncode, 14, result.stdout + result.stderr)
                self.assertIn("pulse_unreachable", [event["code"] for event in events])
                self.assertIn("agent_download_available", [event["code"] for event in events])
                self.assertEqual(events[-1]["code"], "failed")
                self.assertEqual(events[-1]["exitCode"], 14)
                self.assertNotIn("ok", [event["code"] for event in events])

    def test_success_preserves_read_only_preflight_and_selected_tls(self):
        for trust in ("verified", "insecure", "ca"):
            with self.subTest(trust=trust):
                result, events = self.exercise(0, trust)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertIn("pulse_reachable", [event["code"] for event in events])
                self.assertEqual(events[-1]["code"], "ok")
                self.assertEqual(events[-1]["exitCode"], 0)


if __name__ == "__main__":
    unittest.main()
