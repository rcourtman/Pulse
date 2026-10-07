#!/usr/bin/env python3
"""Shard artifacts expose bounded source identities, never assertion payloads."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
RECORDER = ROOT / '.github/scripts/record-internal-api-test-seconds.py'


def event(action, test=None, output=None, elapsed=None):
    value = {'Action': action, 'Package': 'example/internal/api'}
    if test is not None:
        value['Test'] = test
    if output is not None:
        value['Output'] = output
    if elapsed is not None:
        value['Elapsed'] = elapsed
    return json.dumps(value) + '\n'


class APIFailureIndexTest(unittest.TestCase):
    def record(self, stream):
        with tempfile.TemporaryDirectory() as directory:
            seconds = Path(directory) / 'api-2.txt'
            result = subprocess.run([sys.executable, str(RECORDER), str(seconds)],
                                    input=stream, text=True, capture_output=True,
                                    timeout=10)
            raw = Path(str(seconds) + '.failures.json').read_text()
            return result, seconds.read_text(), json.loads(raw), raw

    def test_pass_preserves_timing_and_quiet_output(self):
        result, seconds, index, _ = self.record(
            event('run', 'TestGood') + event('output', 'TestGood', 'private output\n') +
            event('pass', 'TestGood', elapsed=1.25) + event('pass', elapsed=2))
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout, '')
        self.assertEqual(seconds, '# package-seconds 2.00\nTestGood 1.25\n')
        self.assertEqual(index['failed_top_level_tests'],
                         {'names': [], 'observed_count': 0, 'omitted_count': 0})
        self.assertEqual(index['package_terminal_action'], 'pass')
        self.assertEqual(index['package_terminal_count'], 1)

    def test_failure_names_root_without_subtest_or_assertion_data(self):
        secret = 'customer@example.invalid'
        result, seconds, index, raw = self.record(
            event('run', 'TestBroken') +
            event('output', 'TestBroken/' + secret, secret + '\n') +
            event('fail', 'TestBroken/' + secret, elapsed=0.2) +
            event('fail', 'TestBroken', elapsed=0.3) + event('fail', elapsed=0.4))
        self.assertEqual(result.returncode, 1)
        self.assertIn(secret, result.stdout)  # Existing diagnostic log stays intact.
        self.assertEqual(seconds, '# package-seconds 0.40\nTestBroken 0.30\n')
        self.assertEqual(index['failed_top_level_tests']['names'], ['TestBroken'])
        self.assertEqual(index['unfinished_top_level_tests']['names'], [])
        self.assertEqual(index['package_terminal_action'], 'fail')
        self.assertNotIn(secret, raw)
        self.assertNotIn('Package', raw)

    def test_unfinished_root_is_not_a_completed_failure_or_pass(self):
        result, seconds, index, raw = self.record(
            event('run', 'TestInterrupted') +
            event('output', 'TestInterrupted/private-label', 'private log\n'))
        self.assertEqual(result.returncode, 1)
        self.assertIn('TestInterrupted did not finish', result.stdout)
        self.assertEqual(seconds, '')
        self.assertEqual(index['failed_top_level_tests']['names'], [])
        self.assertEqual(index['unfinished_top_level_tests']['names'], ['TestInterrupted'])
        self.assertIsNone(index['package_terminal_action'])
        self.assertEqual(index['package_terminal_count'], 0)
        self.assertNotIn('private', raw)

    def test_package_only_failure_does_not_invent_a_test_identity(self):
        result, _, index, raw = self.record(
            event('output', output='private compiler diagnostic\n') + event('fail'))
        self.assertEqual(result.returncode, 1)
        self.assertIn('compiler diagnostic', result.stdout)
        self.assertEqual(index['failed_top_level_tests']['names'], [])
        self.assertEqual(index['package_terminal_action'], 'fail')
        self.assertNotIn('diagnostic', raw)

    def test_non_json_and_empty_streams_remain_unknown(self):
        for stream, count in [('', 0), ('private non-json line\n[]\n', 2)]:
            with self.subTest(stream=bool(stream)):
                result, _, index, raw = self.record(stream)
                # The upstream Go exit still belongs to pipefail, not this index.
                self.assertEqual(result.returncode, 0)
                self.assertEqual(index['non_json_line_count'], count)
                self.assertIsNone(index['package_terminal_action'])
                self.assertNotIn('private', raw)

    def test_unsafe_and_long_source_names_are_counted_but_not_exported(self):
        for name in ['TestUnsafe\n::error::private', 'TestUnsafe\x1b[31m',
                     'TestÜnicode', 'not-a-source-test', 'Test' + 'A' * 257]:
            with self.subTest(name=name[:12]):
                result, _, index, raw = self.record(event('fail', name))
                self.assertEqual(result.returncode, 1)
                self.assertEqual(index['failed_top_level_tests'],
                                 {'names': [], 'observed_count': 1, 'omitted_count': 1})
                self.assertNotIn(name, raw)

    def test_lists_are_deduplicated_sorted_and_bounded_separately(self):
        stream = ''.join(event('fail', f'TestFailed{i:04d}') for i in range(300, -1, -1))
        stream += event('fail', 'TestFailed0000')
        stream += ''.join(event('run', f'TestPending{i:04d}') for i in range(300, -1, -1))
        result, _, index, raw = self.record(stream)
        self.assertEqual(result.returncode, 1)
        for field in ['failed_top_level_tests', 'unfinished_top_level_tests']:
            value = index[field]
            self.assertEqual(len(value['names']), 256)
            self.assertEqual(value['observed_count'], 301)
            self.assertEqual(value['omitted_count'], 45)
            self.assertEqual(value['names'], sorted(set(value['names'])))
        self.assertLess(len(raw.encode()), 150_000)

    def test_package_actions_preserve_contrary_observations_without_green_claim(self):
        result, _, index, _ = self.record(event('fail') + event('pass'))
        self.assertEqual(result.returncode, 1)
        self.assertEqual(index['package_terminal_action'], 'pass')
        self.assertEqual(index['package_terminal_count'], 2)
        self.assertNotIn('success', index)
        self.assertEqual(index['recorder_exit_code'], 1)

    def test_skip_and_supported_source_name_forms(self):
        stream = ''.join(event('fail', name) for name in
                         ['Test', 'TestUnder_score123', 'Example', 'ExampleThing', 'FuzzThing'])
        result, _, index, _ = self.record(stream + event('skip'))
        self.assertEqual(result.returncode, 1)
        self.assertEqual(index['failed_top_level_tests']['observed_count'], 5)
        self.assertEqual(index['failed_top_level_tests']['omitted_count'], 0)
        self.assertEqual(index['package_terminal_action'], 'skip')

    def test_sidecar_is_ignored_by_the_existing_timing_refresh(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            seconds = root / 'api-2.txt'
            result = subprocess.run([sys.executable, str(RECORDER), str(seconds)],
                                    input=event('pass', 'TestTimed', elapsed=1.25) +
                                          event('pass', elapsed=2),
                                    text=True, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            weights = root / 'weights.out'
            result = subprocess.run(
                [sys.executable, str(ROOT / '.github/scripts/refresh-internal-api-test-seconds.py'),
                 '--dir', str(root), '--output', str(weights)],
                text=True, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('TestTimed 1.25', weights.read_text())
            self.assertNotIn('failures.json', result.stdout)

    def test_pipefail_preserves_an_upstream_failure_with_no_json(self):
        with tempfile.TemporaryDirectory() as directory:
            seconds = Path(directory) / 'api-2.txt'
            result = subprocess.run(
                ['bash', '-c', 'set -o pipefail; (printf "compile diagnostic\\n"; exit 17) | '
                 'python3 "$1" "$2"', 'recorder-proof', str(RECORDER), str(seconds)],
                text=True, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 17)
            index = json.loads(Path(str(seconds) + '.failures.json').read_text())
            self.assertIsNone(index['package_terminal_action'])
            self.assertEqual(index['recorder_exit_code'], 0)
            self.assertEqual(index['non_json_line_count'], 1)

    def test_existing_upload_collects_sidecar_on_a_failed_shard(self):
        workflow = (ROOT / '.github/workflows/build-and-test.yml').read_text()
        job = workflow.split('\n  backend-api:\n', 1)[1].split('\n  backend-api-verdict:', 1)[0]
        self.assertIn('| python3 .github/scripts/record-internal-api-test-seconds.py', job)
        self.assertIn('if: always() && needs.changes.outputs.code ==', job)
        self.assertIn('path: ${{ runner.temp }}/internal-api-test-seconds/', job)
        self.assertIn('name: internal-api-test-seconds-${{ matrix.index }}', job)
        self.assertIn('set -euo pipefail', job)
        self.assertNotIn('continue-on-error:', job)


if __name__ == '__main__':
    unittest.main()
