#!/usr/bin/env python3
"""Run the public paid Docker readback recipe against synthetic Docker observations."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
GUIDE = ROOT / 'docs/DOCKER.md'
SHIPPED = ROOT / 'frontend-modern/public/docs/DOCKER.md'
PLACEHOLDER = 'paste-the-selected-image-reference-here'
ID = 'a' * 64
IMAGE = 'sha256:' + 'b' * 64
REFERENCE = 'registry.example.invalid/pulse-pro@sha256:' + 'c' * 64
STAMP = '2026-11-20T20:01:00.123456789Z'
RUNNING = f'{ID} {IMAGE} running {STAMP}\n'
VERSION = 'Pulse Pro version 6.9.0'

SNAPSHOT = ['container', 'inspect', '--format',
            '{{.Id}} {{.Image}} {{.State.Status}} {{.State.StartedAt}}', '--', 'pulse']
DIGESTS = ['image', 'inspect', '--format', '{{range .RepoDigests}}{{println .}}{{end}}', '--', IMAGE]
READ_VERSION = ['container', 'exec', '--', ID, '/app/pulse', '--version']
BEFORE = (SNAPSHOT, 0, RUNNING)
MATCH = (DIGESTS, 0, REFERENCE + '\n')
VERSION_RESULT = (READ_VERSION, 0, VERSION + '\n')


class PaidDockerHelpTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        section = SHIPPED.read_text().split('## Check the running Pro image\n', 1)[1]
        cls.recipe = section.split('```bash\n', 1)[1].split('\n```', 1)[0]

    def run_recipe(self, operations, reference=REFERENCE):
        with tempfile.TemporaryDirectory() as tmp:
            directory = Path(tmp)
            (directory / 'plan.json').write_text(json.dumps(operations))
            docker = directory / 'docker'
            docker.write_text(f'#!{sys.executable}\n' + '''\
import json, sys
from pathlib import Path

plan = json.loads(Path('plan.json').read_text())
trace = Path('trace.jsonl')
step = len(trace.read_text().splitlines()) if trace.exists() else 0
args = sys.argv[1:]
with trace.open('a') as out:
    out.write(json.dumps(args) + '\\n')
if step >= len(plan) or args != plan[step][0]:
    sys.exit('unexpected Docker command')
_, code, output = plan[step]
sys.stdout.write(output)
sys.exit(code)
''')
            docker.chmod(0o700)
            self.assertEqual(self.recipe.count(PLACEHOLDER), 1)
            result = subprocess.run(
                ['bash', '-c', self.recipe.replace(PLACEHOLDER, reference)],
                cwd=directory,
                env={'PATH': str(directory) + os.pathsep + os.defpath, 'HOME': tmp, 'LC_ALL': 'C'},
                text=True, capture_output=True, timeout=8, check=False,
            )
            trace = directory / 'trace.jsonl'
            self.assertEqual([json.loads(line) for line in trace.read_text().splitlines()]
                             if trace.exists() else [], [args for args, _, _ in operations])
            return result

    def assert_unconfirmed(self, operations, reason, reference=REFERENCE):
        result = self.run_recipe(operations, reference)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, '')
        self.assertEqual(result.stderr, f'Running image unconfirmed: {reason}\n')

    def test_download_guidance_ships_with_the_update_link_and_no_mutation(self):
        self.assertEqual(GUIDE.read_bytes(), SHIPPED.read_bytes())
        text = SHIPPED.read_text()
        updates = text.split('## 🔄 Updates\n', 1)[1].split('## 🔄 Docker / Podman Updates', 1)[0]
        self.assertIn('](#check-the-running-pro-image)', updates)
        self.assertIn('A match does not\nprove the licence activated', text)

    def test_selected_reference_and_version_after_same_start(self):
        result = self.run_recipe([BEFORE, MATCH, VERSION_RESULT, BEFORE])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'Selected private image matches the running container.\n'
                                       + VERSION + '\n')

    def test_restarted_replaced_or_stopped_container_makes_version_unconfirmed(self):
        for changed in (RUNNING.replace(STAMP, '2026-11-20T20:01:01Z'),
                        RUNNING.replace(ID, 'd' * 64), RUNNING.replace('running', 'exited')):
            with self.subTest(changed=changed):
                self.assert_unconfirmed([BEFORE, MATCH, VERSION_RESULT, (SNAPSHOT, 0, changed)],
                                        'the container changed or restarted during the check')
        self.assert_unconfirmed([BEFORE, MATCH, VERSION_RESULT, (SNAPSHOT, 1, RUNNING)],
                                'cannot recheck the container')

    def test_no_start_no_version_or_wrong_repository_cannot_be_confirmed(self):
        for stamp in ('', '<no value>', '0001-01-01T00:00:00Z'):
            with self.subTest(stamp=stamp):
                self.assert_unconfirmed([(SNAPSHOT, 0, RUNNING.replace(STAMP, stamp))],
                                        'Docker did not report a start time')
        self.assert_unconfirmed([BEFORE, (DIGESTS, 0, REFERENCE.replace('/pulse-pro@', '/other@'))],
                                'the selected reference is not recorded')
        self.assert_unconfirmed([BEFORE, (DIGESTS, 1, REFERENCE + '\n')],
                                'cannot read registry references for the running image')
        self.assert_unconfirmed([BEFORE, MATCH, (READ_VERSION, 0, '')],
                                'the container did not report a version')
        self.assert_unconfirmed([], 'use the complete repository@sha256 reference',
                                'registry.example.invalid/pulse-pro:stable')


if __name__ == '__main__':
    unittest.main()
