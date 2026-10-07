#!/usr/bin/env python3
"""Shard artifacts expose bounded source identities, never assertion payloads."""

import json
import os
from pathlib import Path
import shutil
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
    def record(self, stream, selected=None, package='example/internal/api'):
        with tempfile.TemporaryDirectory() as directory:
            seconds = Path(directory) / 'api-2.txt'
            command = [sys.executable, str(RECORDER), str(seconds)]
            if selected is not None:
                selection = Path(directory) / 'api-2.selected'
                selection.write_text('\n'.join(selected) + '\n')
                command.extend([str(selection), package])
            result = subprocess.run(command,
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
                # Unknown completion cannot become a passing workflow verdict,
                # even if a producer happens to return zero.
                self.assertEqual(result.returncode, 1)
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
            (root / 'api-2.selected').write_text('TestTimed\nTestSelectionIsNotATiming\n')
            result = subprocess.run([sys.executable, str(RECORDER), str(seconds)],
                                    input=event('run', 'TestTimed') +
                                          event('pass', 'TestTimed', elapsed=1.25) +
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
            # Both producer and recorder fail; pipefail returns the last
            # failing stage rather than manufacturing success.
            self.assertEqual(result.returncode, 1)
            index = json.loads(Path(str(seconds) + '.failures.json').read_text())
            self.assertIsNone(index['package_terminal_action'])
            self.assertEqual(index['recorder_exit_code'], 1)
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
        self.assertIn('printf \'%s\\n\' "$selected" > "$timings/api-${API_SHARD_INDEX}.selected"', job)
        self.assertIn('package=$(go list ./internal/api)', job)
        self.assertIn('"$timings/api-${API_SHARD_INDEX}.selected" "$package"', job)

    def test_selected_pass_and_explicit_skip_complete_coverage(self):
        stream = (event('run', 'TestGood') + event('pass', 'TestGood', elapsed=0.2) +
                  event('run', 'TestNative') + event('skip', 'TestNative', elapsed=0) +
                  event('pass', elapsed=0.3))
        result, _, index, _ = self.record(stream, ['TestGood', 'TestNative'])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(index['selected_test_count'], 2)
        self.assertEqual(index['completed_top_level_test_count'], 2)
        self.assertEqual(index['missing_selected_tests']['observed_count'], 0)
        self.assertEqual(index['unexpected_top_level_tests']['observed_count'], 0)

    def test_successful_producer_cannot_substitute_incomplete_execution(self):
        good = event('run', 'TestGood') + event('pass', 'TestGood', elapsed=0.2)
        for stream in ['', event('pass'), good, 'malformed\n' + good + event('pass'),
                       good + event('pass') + event('pass'),
                       event('pass', 'TestGood') + event('pass'),
                       good + good + event('pass'), good + event('skip')]:
            with self.subTest(stream=stream):
                result, _, index, _ = self.record(stream, ['TestGood'])
                self.assertEqual(result.returncode, 1)
                self.assertEqual(index['recorder_exit_code'], 1)
                self.assertIn('no passing verdict', result.stderr)

    def test_selected_tests_must_not_be_dropped_or_replaced(self):
        good = event('run', 'TestGood') + event('pass', 'TestGood', elapsed=0.2)
        result, _, index, _ = self.record(good + event('pass'), ['TestGood', 'TestMissing'])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(index['missing_selected_tests']['names'], ['TestMissing'])
        other = event('run', 'TestOther') + event('pass', 'TestOther', elapsed=0.2)
        result, _, index, _ = self.record(other + event('pass'), ['TestGood'])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(index['missing_selected_tests']['names'], ['TestGood'])
        self.assertEqual(index['unexpected_top_level_tests']['names'], ['TestOther'])

    def test_unfinished_selected_test_retains_both_coverage_and_original_log(self):
        stream = event('run', 'TestHung') + event('output', 'TestHung/private', 'private log\n')
        result, _, index, raw = self.record(stream, ['TestHung'])
        self.assertEqual(result.returncode, 1)
        self.assertIn('private log', result.stdout)
        self.assertEqual(index['unfinished_top_level_tests']['names'], ['TestHung'])
        self.assertEqual(index['missing_selected_tests']['names'], ['TestHung'])
        self.assertNotIn('private', raw)

    def test_wrong_or_multiple_packages_cannot_satisfy_selection(self):
        good = event('run', 'TestGood') + event('pass', 'TestGood') + event('pass')
        for stream, package in [(good, 'other/private-package'),
                                (good.replace('example/internal/api', 'other/private-package', 1),
                                 'example/internal/api')]:
            result, _, index, raw = self.record(stream, ['TestGood'], package)
            self.assertEqual(result.returncode, 1)
            self.assertGreater(index['unexpected_package_event_count'], 0)
            self.assertNotIn('private-package', raw)

    def test_invalid_event_shapes_are_non_passing_not_recorder_crashes(self):
        for changed in [{'Action': 'invented'}, {'Package': None}, {'Test': []}, {'Output': 1}]:
            value = {'Action': 'output', 'Package': 'example/internal/api'}
            value.update(changed)
            stream = (json.dumps(value) + '\n' + event('run', 'TestGood') +
                      event('pass', 'TestGood') + event('pass'))
            result, _, index, _ = self.record(stream, ['TestGood'])
            self.assertEqual(result.returncode, 1)
            self.assertGreater(index['invalid_event_count'], 0)
            self.assertNotIn('Traceback', result.stderr)

    def test_go_build_diagnostics_remain_visible_without_test_package_identity(self):
        good = event('run', 'TestGood') + event('pass', 'TestGood') + event('pass')
        for action, code in [('build-output', 0), ('build-fail', 1)]:
            build = json.dumps({'Action': action, 'ImportPath': 'private/dependency',
                                'Output': 'retained build diagnostic\n'}) + '\n'
            result, _, index, raw = self.record(build + good, ['TestGood'])
            self.assertEqual(result.returncode, code)
            self.assertIn('retained build diagnostic', result.stdout)
            self.assertEqual(index['package_identity_count'], 1)
            self.assertNotIn('private/dependency', raw)
            self.assertNotIn('diagnostic', raw)

    def test_coverage_names_stay_bounded_without_rejecting_unicode_source(self):
        missing = [f'TestMissing{i:04d}' for i in range(301)]
        missing.extend(['TestÜnicode', 'Test' + 'A' * 257])
        stream = event('run', 'TestGood') + event('pass', 'TestGood') + event('pass')
        result, _, index, raw = self.record(stream, ['TestGood'] + missing)
        self.assertEqual(result.returncode, 1)
        observation = index['missing_selected_tests']
        self.assertEqual(observation['observed_count'], 303)
        self.assertEqual(len(observation['names']), 256)
        self.assertEqual(observation['omitted_count'], 47)
        self.assertNotIn('Ünicode', raw)
        result, _, _, _ = self.record(
            event('run', 'TestÜnicode') + event('pass', 'TestÜnicode') + event('pass'),
            ['TestÜnicode'])
        self.assertEqual(result.returncode, 0)

    def test_invalid_selection_is_rejected_before_observation(self):
        with tempfile.TemporaryDirectory() as directory:
            seconds = Path(directory) / 'api.txt'
            selected = Path(directory) / 'api.selected'
            for names in ['', 'TestA\nTestA\n', 'TestA/subtest\n', 'TestBad-name\n']:
                selected.write_text(names)
                result = subprocess.run([sys.executable, str(RECORDER), str(seconds),
                                         str(selected), 'example/internal/api'],
                                        input='', text=True, capture_output=True, timeout=10)
                self.assertEqual(result.returncode, 2)
                self.assertFalse(seconds.exists())
            selected.unlink()
            result = subprocess.run([sys.executable, str(RECORDER), str(seconds),
                                     str(selected), 'example/internal/api'],
                                    input='', text=True, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 2)
            self.assertIn('unavailable', result.stderr)

    @unittest.skipUnless(shutil.which('go'), 'real Go JSON requires the declared compiler')
    def test_real_race_json_pass_skip_fuzz_example_failure_and_missing_coverage(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            package = 'example.invalid/shardfixture'
            (root / 'go.mod').write_text(f'module {package}\n\ngo 1.24\n')
            (root / 'fixture_test.go').write_text('''package shardfixture
import ("fmt"; "testing")
type Fixture struct{}
func TestGood(t *testing.T) { t.Log("passing fixture output") }
func TestSkip(t *testing.T) { t.Skip("explicit native fixture exclusion") }
func TestFailure(t *testing.T) { t.Fatal("retained fixture failure") }
func FuzzSeed(f *testing.F) { f.Add(1); f.Fuzz(func(t *testing.T, n int) {}) }
func ExampleFixture() { fmt.Println("fixture")
// Output: fixture
}
''')
            env = {**os.environ, 'GOWORK': 'off', 'GOPROXY': 'off', 'GOTOOLCHAIN': 'local'}
            listing = subprocess.run(['go', 'test', '-race', '-list', '.', '.'],
                                     cwd=root, env=env, text=True, capture_output=True, timeout=120)
            self.assertEqual(listing.returncode, 0, listing.stderr)
            names = [name for name in listing.stdout.splitlines()
                     if name.startswith(('Test', 'Fuzz', 'Example'))]
            selected = [name for name in names if name != 'TestFailure']
            self.assertEqual(set(selected), {'TestGood', 'TestSkip', 'FuzzSeed', 'ExampleFixture'})
            command = ['go', 'test', '-race', '-count=1', '-timeout', '30s', '-json']
            # Exercise the same shorter-side -skip path used by large CI shards.
            passing = subprocess.run(command + ['-skip', '^TestFailure$', '.'],
                                     cwd=root, env=env, text=True, capture_output=True, timeout=120)
            self.assertEqual(passing.returncode, 0, passing.stderr)
            result, _, index, _ = self.record(passing.stdout, selected, package)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(index['completed_top_level_test_count'], 4)
            self.assertNotIn('passing fixture output', result.stdout)

            # Real Go exits zero while selecting fewer tests than the source
            # list expects. Neither pipefail nor a package pass detects that.
            omitted = subprocess.run(command + ['-run', '^TestGood$', '.'],
                                     cwd=root, env=env, text=True, capture_output=True, timeout=120)
            self.assertEqual(omitted.returncode, 0, omitted.stderr)
            result, _, index, _ = self.record(omitted.stdout, selected, package)
            self.assertEqual(result.returncode, 1)
            self.assertEqual(index['missing_selected_tests']['observed_count'], 3)

            # Completed tests without the final package event are not a pass.
            events = [json.loads(line) for line in passing.stdout.splitlines()]
            truncated = ''.join(json.dumps(value) + '\n' for value in events
                                if not (value.get('Action') == 'pass' and 'Test' not in value))
            result, _, index, _ = self.record(truncated, selected, package)
            self.assertEqual(result.returncode, 1)
            self.assertIsNone(index['package_terminal_action'])

            failing = subprocess.run(command + ['-run', '^TestFailure$', '.'],
                                     cwd=root, env=env, text=True, capture_output=True, timeout=120)
            self.assertEqual(failing.returncode, 1, failing.stderr)
            result, _, index, _ = self.record(failing.stdout, ['TestFailure'], package)
            self.assertEqual(result.returncode, 1)
            self.assertIn('retained fixture failure', result.stdout)
            self.assertEqual(index['failed_top_level_tests']['names'], ['TestFailure'])
            self.assertEqual(index['missing_selected_tests']['observed_count'], 0)
            print('Real Go race JSON: complete pass/skip/fuzz/example accepted; '
                  'producer-zero omission, truncated completion and actual failure rejected.')


if __name__ == '__main__':
    unittest.main()
