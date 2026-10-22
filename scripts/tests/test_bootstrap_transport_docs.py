#!/usr/bin/env python3
"""Keep first-run entrypoints on protected browser transport.

Copied Docker recipes run only against an argv recorder. OpenSSH's -G mode
validates the actual tunnel recipe without opening a connection or reading a
personal SSH configuration. Native tunnel exchange is covered by installtests.
"""
from pathlib import Path
import json
import os
import re
import shlex
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
ENTRYPOINTS = ('README.md', 'docs/INSTALL.md', 'docs/DOCKER.md',
               'docs/i18n/de/README.md', 'docs/i18n/es/README.md')
TARGET = 'INSTALL.md#open-a-protected-setup-connection'


def read(name):
    return (ROOT / name).read_text()


def fences(text, language):
    return re.findall(r'```' + language + r'\n(.*?)```', text, re.S)


class BootstrapTransportDocsTest(unittest.TestCase):
    def test_all_new_install_entrypoints_bind_loopback_and_link_to_transport(self):
        for name in ENTRYPOINTS:
            with self.subTest(name=name):
                text = read(name)
                self.assertIn("#open-a-protected-setup-connection" if name == "docs/INSTALL.md" else TARGET, text)
                commands = [b for b in fences(text, 'bash') if b.startswith('docker run -d')]
                self.assertEqual(len(commands), 1)
                with tempfile.TemporaryDirectory() as temp:
                    root = Path(temp)
                    tool = root / 'docker'
                    tool.write_text('#!/usr/bin/env python3\nimport json, os, sys\n'
                                    'from pathlib import Path\n'
                                    'Path(os.environ["RECORD"]).write_text(json.dumps(sys.argv[1:]))\n')
                    tool.chmod(0o700)
                    subprocess.run(['bash', '-c', commands[0]], check=True, timeout=5,
                                   env={**os.environ, 'PATH': f'{root}:{os.environ["PATH"]}',
                                        'RECORD': str(root / 'record')}, capture_output=True)
                    args = json.loads((root / 'record').read_text())
                    self.assertEqual(args[args.index('-p') + 1], '127.0.0.1:7655:7655')
                    self.assertEqual(args[args.index('-v') + 1], 'pulse_data:/data')
                    self.assertEqual(args[-1], 'rcourtman/pulse:vX.Y.Z')
                    self.assertFalse(any('AUTH_PASS=' in x or 'TOKEN=' in x for x in args))
                for compose in (b for b in fences(text, 'yaml') if 'image: ${PULSE_IMAGE:' in b and 'ports:' in b):
                    self.assertIn('"127.0.0.1:7655:7655"', compose)
                    self.assertNotIn('"7655:7655"', compose)
                    self.assertIn('pulse_data:/data', compose)
                if name != 'docs/INSTALL.md':
                    self.assertNotIn('http://<your-ip>:7655', text)

    def test_real_openssh_configuration_is_noninteractive_key_checked_local_forward_only(self):
        command, = [b for b in fences(read('docs/INSTALL.md'), 'bash') if b.startswith('ssh -N')]
        # Supply no user/system SSH config and no destination connection. The
        # documented command itself still supplies every safety-critical flag.
        args = shlex.split(command.replace('\\\n', ''))
        result = subprocess.run([args[0], '-G', '-F', '/dev/null', *args[1:]],
                                capture_output=True, text=True, check=True, timeout=5)
        settings = dict(line.split(' ', 1) for line in result.stdout.splitlines() if ' ' in line)
        for name, value in {'user': 'admin', 'hostname': 'pulse-host', 'exitonforwardfailure': 'yes',
                            'requesttty': 'false', 'sessiontype': 'none', 'stricthostkeychecking': 'true',
                            'localforward': '[127.0.0.1]:17655 [127.0.0.1]:7655'}.items():
            self.assertEqual(settings[name], value)
        self.assertNotRegex(command, r'(?i)password|token|StrictHostKeyChecking=no|0\.0\.0\.0')

    def test_setup_order_and_failure_limits_are_explicit(self):
        text = ' '.join(read('docs/INSTALL.md').split())
        self.assertLess(text.index('### Open a protected setup connection'), text.index('### Step 1: Get the Token'))
        for phrase in ('does not encrypt the connection', 'Do not bypass a certificate warning',
                       'not its Proxmox host', 'unknown or changed key must stop',
                       'Stop on a tunnel error', 'never a wildcard bind', 'does not stop Pulse',
                       'do not recreate it', "not the workstation's setup-tunnel address",
                       'A private LAN or VPN address alone does not encrypt HTTP',
                       'screenshots containing credentials out of reports'):
            self.assertIn(phrase, text)
        self.assertIn('[protected setup connection](#open-a-protected-setup-connection) chosen above', text)
        for name in ENTRYPOINTS[1:]:
            self.assertEqual((ROOT / name).read_bytes(),
                             (ROOT / 'frontend-modern/public' / name).read_bytes())


if __name__ == '__main__':
    unittest.main()
