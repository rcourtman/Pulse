#!/usr/bin/env python3
"""Exercise the documented helper-update commands without root or services."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


class AgentHelperUpdateDocsTest(unittest.TestCase):
    def setUp(self):
        self.document = (ROOT / "docs/UNIFIED_AGENT.md").read_text()
        self.section = self.document.split("### Updating the typed helper\n", 1)[1].split(
            "\n### ", 1
        )[0]

    def test_copyable_commands_keep_saved_profile_and_preflight_separate(self):
        commands = re.findall(r"```bash\n(.*?)\n\s*```", self.section, re.S)
        self.assertEqual(len(commands), 2)
        # A recorder stands in for sudo; neither root nor the real installer is
        # executed. Spaces in HOME must survive both copied commands unchanged.
        with tempfile.TemporaryDirectory(prefix="helper update docs ") as tmp:
            root = Path(tmp)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            sudo = bin_dir / "sudo"
            sudo.write_text(
                "#!/usr/bin/env python3\n"
                "import json, os, sys\n"
                "with open(os.environ['CALLS'], 'a') as out:\n"
                "    out.write(json.dumps(sys.argv[1:]) + '\\n')\n"
            )
            sudo.chmod(0o700)
            calls = root / "calls.jsonl"
            env = {**os.environ, "HOME": str(root / "home with spaces"),
                   "PATH": str(bin_dir) + os.pathsep + os.environ["PATH"],
                   "CALLS": str(calls)}
            for command in commands:
                subprocess.run(["bash", "--noprofile", "--norc", "-eu", "-c", command],
                               env=env, check=True, capture_output=True, text=True)
            observed = [json.loads(line) for line in calls.read_text().splitlines()]
            installer = str(root / "home with spaces/.config/pulse/agent-install.sh")
            self.assertEqual(observed, [
                ["bash", installer, "--preflight-only", "--update"],
                ["bash", installer, "--update"],
            ])

    def test_guidance_separates_versions_and_preserves_actual_boundaries(self):
        text = " ".join(self.section.split())
        for required in (
            "Automatic agent updates replace the collector only.",
            "do not replace or restart `pulse-agent-helper`",
            "does not establish the helper's release version",
            "health check confirms the requested protocol",
            "no `--version` option",
            "No new token is needed",
            "Preflight is not installation or proof of recovery",
            "least-privilege/typed-helper profile",
            "effective isolated unit and socket",
            "Do not uninstall, re-enrol",
            "remove `PrivateNetwork`",
            "not a diagnostic retest request for a healthy host",
        ):
            with self.subTest(required=required):
                self.assertIn(required, text)
        self.assertEqual((ROOT / "frontend-modern/public/docs/UNIFIED_AGENT.md").read_bytes(),
                         (ROOT / "docs/UNIFIED_AGENT.md").read_bytes())
        helper_flags = (ROOT / "cmd/pulse-agent-helper/main.go").read_text().split(
            "func parseFlags(", 1)[1].split("\nfunc ", 1)[0]
        self.assertNotRegex(helper_flags, r'\.(?:Bool|String)\("version"')
        # Keep the procedure attached to the real installer option, not the
        # similarly named public server installer or an invented helper flag.
        installer = (ROOT / "scripts/install.sh").read_text()
        self.assertIn('--update) UPDATE_ONLY="true"; shift ;;', installer)
        self.assertIn('recover_connection_state "$lifecycle_conn_env"', installer)
        self.assertIn('verify_download_signature "$TMP_HELPER_BIN" "$helper_signature"', installer)
        self.assertIn('provision_typed_privileged_helper', installer)

    def test_runtime_guide_links_to_the_immutable_documented_procedure(self):
        source = (ROOT / "internal/agentupdate/update.go").read_text()
        link = re.search(r'Str\("helper_update_guide", "([^"]+)"\)', source)
        self.assertIsNotNone(link)
        # This ancestor contains the guide itself. Runtime help must not drift
        # to a different main-branch procedure after the agent was installed.
        self.assertEqual(link.group(1),
                         "https://github.com/rcourtman/Pulse/blob/"
                         "b87c182407ef54a0503c19c06a1157a49e20b65e/"
                         "docs/UNIFIED_AGENT.md#updating-the-typed-helper")


if __name__ == "__main__":
    unittest.main()
