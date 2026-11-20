#!/usr/bin/env python3
"""Check the copyable HTTPS drop-in without installing or starting any unit."""

import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


def https_drop_in():
    guide = (ROOT / "docs/CONFIGURATION.md").read_text(encoding="utf-8")
    _, _, section = guide.partition("### HTTPS with systemd\n")
    section = section.split("\n### ", 1)[0]
    matches = re.findall(r"^\x60\x60\x60ini\n(.*?)^\x60\x60\x60$", section, re.M | re.S)
    if len(matches) != 1:
        raise AssertionError("expected one copyable systemd HTTPS drop-in")
    return matches[0]


class ServerHttpsDocsTest(unittest.TestCase):
    def test_drop_in_contains_all_three_service_environment_assignments(self):
        header, *assignments = https_drop_in().splitlines()
        self.assertEqual(header, "[Service]")
        variables = {}
        for assignment in assignments:
            match = re.fullmatch(r'Environment="([A-Z_]+)=([^"]+)"', assignment)
            self.assertIsNotNone(match, assignment)
            name, value = match.groups()
            self.assertNotIn(name, variables)
            variables[name] = value
        self.assertEqual(set(variables), {"HTTPS_ENABLED", "TLS_CERT_FILE", "TLS_KEY_FILE"})
        self.assertEqual(variables["HTTPS_ENABLED"], "true")
        for key in ("TLS_CERT_FILE", "TLS_KEY_FILE"):
            self.assertTrue(Path(variables[key]).is_absolute(), variables[key])

    @unittest.skipUnless(shutil.which("systemd-analyze"), "systemd parser unavailable")
    def test_systemd_accepts_the_example_as_a_drop_in(self):
        with tempfile.TemporaryDirectory(prefix="pulse-https-docs-") as directory:
            root = Path(directory)
            unit = root / "pulse-https-example.service"
            unit.write_text(
                "[Unit]\nDefaultDependencies=no\n[Service]\nExecStart=/bin/true\n",
                encoding="utf-8",
            )
            drop_ins = root / (unit.name + ".d")
            drop_ins.mkdir()
            (drop_ins / "override.conf").write_text(https_drop_in(), encoding="utf-8")
            result = subprocess.run(
                ["systemd-analyze", "verify", "--man=no", "--generators=no", str(unit)],
                env={**os.environ, "SYSTEMD_UNIT_PATH": directory},
                capture_output=True, text=True, timeout=10,
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, "", result.stderr)


if __name__ == "__main__":
    unittest.main()
