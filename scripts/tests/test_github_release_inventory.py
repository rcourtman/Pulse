#!/usr/bin/env python3
"""Exercise the continuity workflow, not a parallel inventory implementation."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest


ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/read-github-release-inventory.sh'
STREAM_ERROR = 'stream error: stream ID 1; CANCEL; received from peer\n'
LATEST = {'tag_name': 'v6.5.0', 'draft': False, 'published_at': '2026-10-06T00:54:36Z'}
REFS = [[{'ref': 'refs/tags/v6.5.0'}]]
RELEASES = [[LATEST]]


def success(value):
    return [0, json.dumps(value), '']


def workflow_command():
    path = Path(os.environ.get('PULSE_TEST_SECURITY_WORKFLOW', ROOT / '.github/workflows/security-scan.yml'))
    step = path.read_text().split('      - name: Reject orphaned stable publication tags\n', 1)[1]
    step = step.split('\n      - name: Bind the release activation marker', 1)[0]
    return textwrap.dedent(step.split('        run: |\n', 1)[1])


class GithubReleaseInventoryTests(unittest.TestCase):
    def run_inventory(self, outcomes=None, *, workflow=False, existing=None,
                      timeout_status=None, registry_tags=None, repository='rcourtman/Pulse'):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            evidence = root / 'release-continuity-evidence'
            evidence.mkdir()
            (evidence / 'release.json').write_text(json.dumps(LATEST))
            for name, value in (existing or {}).items():
                (evidence / name).write_text(value)
            (root / 'outcomes.json').write_text(json.dumps(outcomes or {}))
            (tools / 'gh').write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['INVENTORY_FIXTURE'])
args = sys.argv[1:]
with (root / 'gh.jsonl').open('a') as f:
    f.write(json.dumps(args) + '\\n')
name = 'stable-refs' if '/git/matching-refs/' in args[-1] else 'releases'
calls = [json.loads(line) for line in (root / 'gh.jsonl').read_text().splitlines()]
count = sum(call[-1] == args[-1] for call in calls)
defaults = {'stable-refs': [0, '[[{"ref":"refs/tags/v6.5.0"}]]', ''],
            'releases': [0, '[[{"tag_name":"v6.5.0","draft":false,"published_at":"2026-10-06T00:54:36Z"}]]', '']}
results = json.loads((root / 'outcomes.json').read_text()).get(name, [defaults[name]])
status, output, diagnostic = results[min(count - 1, len(results) - 1)]
print(output, end='')
print(diagnostic, end='', file=sys.stderr)
sys.exit(status)
''')
            (tools / 'sleep').write_text('''#!/bin/sh
printf '%s\\n' "$*" >> "$INVENTORY_FIXTURE/sleep.calls"
''')
            (tools / 'timeout').write_text('''#!/bin/sh
printf '%s\\n' "$*" >> "$INVENTORY_FIXTURE/timeout.calls"
[ "$1" = -k ] && [ "$2" = 5 ] && [ "$3" = 180 ] || exit 99
if [ -n "${FIXTURE_TIMEOUT_STATUS:-}" ]; then exit "$FIXTURE_TIMEOUT_STATUS"; fi
shift 3
exec "$@"
''')
            (tools / 'curl').write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['INVENTORY_FIXTURE'])
with (root / 'curl.jsonl').open('a') as f:
    f.write(json.dumps(sys.argv[1:]) + '\\n')
if '/tags/list?' in sys.argv[-1]:
    print(json.dumps({'tags': json.loads(os.environ['FIXTURE_REGISTRY_TAGS'])}))
else:
    print('{"token":"synthetic-registry-token"}')
''')
            for tool in tools.iterdir():
                tool.chmod(0o755)
            env = {**os.environ, 'PATH': str(tools) + ':' + os.environ['PATH'],
                   'INVENTORY_FIXTURE': str(root), 'REPOSITORY': repository,
                   'GITHUB_REPOSITORY_OWNER': 'rcourtman',
                   'FIXTURE_REGISTRY_TAGS': json.dumps(registry_tags or ['v6.5.0'])}
            env.pop('FIXTURE_TIMEOUT_STATUS', None)
            if timeout_status is not None:
                env['FIXTURE_TIMEOUT_STATUS'] = str(timeout_status)
            if workflow:
                # Keep the checked-out validator/helper exact. Only local
                # evidence and external commands are supplied by the fixture.
                (root / 'scripts').symlink_to(ROOT / 'scripts', target_is_directory=True)
                command = ['bash', '-c', workflow_command()]
            else:
                command = ['bash', str(HELPER), repository, str(evidence)]
            result = subprocess.run(command, cwd=root, env=env, capture_output=True,
                                    text=True, timeout=15)
            calls = [json.loads(line) for line in (root / 'gh.jsonl').read_text().splitlines()] if (root / 'gh.jsonl').exists() else []
            delays = (root / 'sleep.calls').read_text().splitlines() if (root / 'sleep.calls').exists() else []
            timeouts = (root / 'timeout.calls').read_text().splitlines() if (root / 'timeout.calls').exists() else []
            files = {p.name: p.read_text() for p in evidence.iterdir() if p.is_file()}
            registry_calls = (root / 'curl.jsonl').read_text().splitlines() if (root / 'curl.jsonl').exists() else []
            return result, calls, delays, timeouts, files, registry_calls

    def assert_reads(self, calls, names):
        endpoints = {'stable-refs': 'git/matching-refs/tags/v?per_page=100',
                     'releases': 'releases?per_page=100'}
        self.assertEqual(calls, [['api', '--paginate', '--slurp',
                                 '-H', 'Accept: application/vnd.github+json',
                                 '-H', 'X-GitHub-Api-Version: 2026-03-10',
                                 'repos/rcourtman/Pulse/' + endpoints[name]] for name in names])

    def test_success_preserves_complete_pages_without_another_read(self):
        pages = [RELEASES[0], [{'tag_name': 'v6.4.1'}], []]
        result, calls, delays, timeouts, files, _ = self.run_inventory({'releases': [success(pages)]})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_reads(calls, ['stable-refs', 'releases'])
        self.assertEqual(delays, [])
        self.assertEqual(len(timeouts), 2)
        self.assertTrue(all(line.startswith('-k 5 180 gh api --paginate --slurp ') for line in timeouts))
        self.assertEqual(files['releases.json'], json.dumps(pages))

    def test_workflow_recovers_truncated_cancel_without_combining_attempts(self):
        partial = '[[{"tag_name":"broken'
        result, calls, delays, _, files, registry = self.run_inventory(
            {'releases': [[1, partial, STREAM_ERROR], success(RELEASES)]}, workflow=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_reads(calls, ['stable-refs', 'releases', 'releases'])
        self.assertEqual(delays, ['2'])
        self.assertEqual(files['releases-attempt-1.json'], partial)
        self.assertEqual(files['releases-attempt-1.stderr'], STREAM_ERROR)
        self.assertEqual(json.loads(files['releases.json']), RELEASES)
        self.assertEqual(json.loads(files['frontier-diagnostic.json'])['status'], 'success')
        self.assertEqual(len(registry), 6)

    def test_refs_can_recover_on_the_last_identical_read(self):
        result, calls, delays, _, files, _ = self.run_inventory(
            {'stable-refs': [[1, '[', STREAM_ERROR], [1, '[[]', STREAM_ERROR], success(REFS)]})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_reads(calls, ['stable-refs', 'stable-refs', 'stable-refs', 'releases'])
        self.assertEqual(delays, ['2', '4'])
        self.assertEqual(files['stable-refs-attempt-1.json'], '[')
        self.assertEqual(files['stable-refs-attempt-2.json'], '[[]')

    def test_exhaustion_stops_before_registry_or_frontier(self):
        result, calls, delays, _, files, registry = self.run_inventory(
            {'releases': [[1, 'partial', STREAM_ERROR]]}, workflow=True)
        self.assertEqual(result.returncode, 1)
        self.assert_reads(calls, ['stable-refs', 'releases', 'releases', 'releases'])
        self.assertEqual(delays, ['2', '4'])
        self.assertNotIn('releases.json', files)
        self.assertNotIn('frontier-diagnostic.json', files)
        self.assertEqual(registry, [])
        self.assertEqual(files['releases-attempt-3.stderr'], STREAM_ERROR)

    def test_access_unknown_and_mixed_diagnostics_never_retry(self):
        for diagnostic in ['gh: Forbidden (HTTP 403)\n', 'gh: Not Found (HTTP 404)\n',
                           'gh: Bad Gateway (HTTP 502)\n', 'x509: unknown authority\n',
                           '', 'unknown failure\n']:
            for text in [diagnostic, STREAM_ERROR + diagnostic + '\n']:
                with self.subTest(diagnostic=text):
                    result, calls, delays, _, files, registry = self.run_inventory(
                        {'stable-refs': [[1, 'partial', text], success(REFS)]}, workflow=True)
                    self.assertEqual(result.returncode, 1, result.stderr)
                    self.assert_reads(calls, ['stable-refs'])
                    self.assertEqual(delays, [])
                    self.assertNotIn('stable-refs.json', files)
                    self.assertEqual(registry, [])

    def test_only_the_observed_transport_signature_can_retry(self):
        for text in [STREAM_ERROR.replace('CANCEL', 'REFUSED_STREAM'),
                     STREAM_ERROR.replace('CANCEL', 'INTERNAL_ERROR'),
                     STREAM_ERROR.replace('received from peer', 'local cancellation'),
                     'unexpected prefix ' + STREAM_ERROR,
                     STREAM_ERROR + '\n']:
            with self.subTest(text=text):
                result, calls, delays, _, _, _ = self.run_inventory({'stable-refs': [[1, '', text]]})
                self.assertEqual(result.returncode, 1)
                self.assert_reads(calls, ['stable-refs'])
                self.assertEqual(delays, [])

    def test_timeout_and_nonordinary_exits_preserve_failure_without_retry(self):
        for status in [2, 124, 130, 137, 143]:
            with self.subTest(status=status):
                result, calls, delays, _, files, _ = self.run_inventory(
                    {'stable-refs': [[status, '[', STREAM_ERROR]]})
                self.assertEqual(result.returncode, status)
                self.assert_reads(calls, ['stable-refs'])
                self.assertEqual(delays, [])
                self.assertNotIn('stable-refs.json', files)
        result, calls, delays, timeouts, _, _ = self.run_inventory(timeout_status=124)
        self.assertEqual(result.returncode, 124)
        self.assertEqual(calls, [])
        self.assertEqual(delays, [])
        self.assertEqual(len(timeouts), 1)

    def test_successful_but_incomplete_or_wrong_shaped_output_never_retries(self):
        for output in ['', '[[{"ref":', '[]', '{}', '[{}]', '[[null]]', '[[],{}]', '[[]]tail']:
            with self.subTest(output=output):
                result, calls, delays, _, files, _ = self.run_inventory({'stable-refs': [[0, output, '']]})
                self.assertEqual(result.returncode, 1)
                self.assert_reads(calls, ['stable-refs'])
                self.assertEqual(delays, [])
                self.assertEqual(files['stable-refs-attempt-1.json'], output)
                self.assertNotIn('stable-refs.json', files)

    def test_success_with_unexplained_stderr_stops(self):
        result, calls, delays, _, files, _ = self.run_inventory(
            {'stable-refs': [[0, json.dumps(REFS), 'unexplained diagnostic\n']]})
        self.assertEqual(result.returncode, 1)
        self.assert_reads(calls, ['stable-refs'])
        self.assertEqual(delays, [])
        self.assertNotIn('stable-refs.json', files)

    def test_genuinely_empty_page_is_not_a_failed_read(self):
        result, calls, delays, _, files, _ = self.run_inventory(
            {'stable-refs': [success([[]])], 'releases': [success([[]])]})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_reads(calls, ['stable-refs', 'releases'])
        self.assertEqual(delays, [])
        self.assertEqual(json.loads(files['releases.json']), [[]])

    def test_existing_accepted_inventory_is_never_reused_or_overwritten(self):
        for name in ['stable-refs.json', 'releases.json']:
            with self.subTest(name=name):
                result, calls, delays, _, files, _ = self.run_inventory(existing={name: 'previous evidence'})
                self.assertEqual(result.returncode, 1)
                self.assertEqual(calls, [])
                self.assertEqual(delays, [])
                self.assertEqual(files[name], 'previous evidence')

    def test_invalid_repository_never_contacts_github(self):
        result, calls, _, _, _, _ = self.run_inventory(repository='--method POST')
        self.assertEqual(result.returncode, 2)
        self.assertEqual(calls, [])

    def test_recovered_inventory_still_rejects_orphaned_publication(self):
        result, calls, delays, _, files, _ = self.run_inventory(
            {'stable-refs': [success([[*REFS[0], {'ref': 'refs/tags/v6.6.0'}]])],
             'releases': [[1, 'partial', STREAM_ERROR], success(RELEASES)]}, workflow=True)
        self.assertEqual(result.returncode, 1)
        self.assert_reads(calls, ['stable-refs', 'releases', 'releases'])
        self.assertEqual(delays, ['2'])
        diagnostic = json.loads(files['frontier-diagnostic.json'])
        self.assertEqual(diagnostic['status'], 'failure')
        self.assertIn('stable_tag_without_release', [v['code'] for v in diagnostic['violations']])

    def test_recovered_inventory_still_rejects_newer_public_release_and_registry(self):
        newer = {'tag_name': 'v6.6.0', 'draft': False, 'published_at': '2026-10-18T00:00:00Z'}
        result, calls, _, _, files, _ = self.run_inventory(
            {'releases': [[1, 'partial', STREAM_ERROR], success([[LATEST, newer]])]},
            workflow=True, registry_tags=['v6.5.0', '6.6.0'])
        self.assertEqual(result.returncode, 1)
        self.assert_reads(calls, ['stable-refs', 'releases', 'releases'])
        diagnostic = json.loads(files['frontier-diagnostic.json'])
        self.assertEqual({v['code'] for v in diagnostic['violations']},
                         {'newer_stable_release_not_advertised', 'registry_stable_tag_beyond_latest'})


if __name__ == '__main__':
    unittest.main(verbosity=2)
