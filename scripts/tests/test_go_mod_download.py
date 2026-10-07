#!/usr/bin/env python3
"""Execute the Docker module-download command with controlled Go outcomes."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/go-mod-download.sh'
STREAM_ERROR = ('go: go.opentelemetry.io/otel@v1.44.0: read '
                '"https://proxy.golang.org/go.opentelemetry.io/otel/@v/v1.44.0.zip": '
                'stream error: stream ID 693; INTERNAL_ERROR; received from peer\n')


def docker_download_command():
    # Execute the actual RUN command, omitting only Docker cache-mount syntax.
    dockerfile = Path(os.environ.get('PULSE_TEST_DOCKERFILE', ROOT / 'Dockerfile')).read_text()
    stage = dockerfile.split('COPY go.mod go.sum ./\n', 1)[1].split('\n\n', 1)[0]
    command = stage.split('RUN ', 1)[1]
    command = re.sub(r'--mount=[^\s]+\s*\\\n\s*', '', command).strip()
    return command.replace('/usr/local/bin/pulse-go-mod-download', str(HELPER))


class GoModDownloadTests(unittest.TestCase):
    def run_download(self, outcomes, *, command=None, timeout_status=None, args=()):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tools = root / 'tools'
            tools.mkdir()
            temporary = root / 'temporary'
            temporary.mkdir()
            (root / 'outcomes.json').write_text(json.dumps(outcomes))
            go = tools / 'go'
            go.write_text('''#!/usr/bin/env python3
import json, os, pathlib, sys
root = pathlib.Path(os.environ['DOWNLOAD_FIXTURE'])
with (root / 'calls.jsonl').open('a') as f:
    f.write(json.dumps(sys.argv[1:]) + '\\n')
count = len((root / 'calls.jsonl').read_text().splitlines())
outcomes = json.loads((root / 'outcomes.json').read_text())
status, text = outcomes[min(count - 1, len(outcomes) - 1)]
print(text, end='', file=sys.stderr)
sys.exit(status)
''')
            go.chmod(0o755)
            timeout = tools / 'timeout'
            timeout.write_text('''#!/bin/sh
printf '%s\\n' "$*" >> "$DOWNLOAD_FIXTURE/timeout.calls"
[ "$1" = -k ] && [ "$2" = 5 ] && [ "$3" = 180 ] || exit 99
if [ -n "${FIXTURE_TIMEOUT_STATUS:-}" ]; then exit "$FIXTURE_TIMEOUT_STATUS"; fi
shift 3
exec "$@"
''')
            timeout.chmod(0o755)
            sleep = tools / 'sleep'
            sleep.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOWNLOAD_FIXTURE/sleep.calls"\n')
            sleep.chmod(0o755)
            env = {**os.environ, 'PATH': str(tools) + ':' + os.environ['PATH'],
                   'DOWNLOAD_FIXTURE': str(root), 'TMPDIR': str(temporary)}
            if timeout_status is not None:
                env['FIXTURE_TIMEOUT_STATUS'] = str(timeout_status)
            else:
                env.pop('FIXTURE_TIMEOUT_STATUS', None)
            argv = ['sh', '-c', command] if command else ['sh', str(HELPER), *args]
            result = subprocess.run(argv, cwd=ROOT, env=env, capture_output=True,
                                    text=True, timeout=10)
            calls = [json.loads(line) for line in (root / 'calls.jsonl').read_text().splitlines()] if (root / 'calls.jsonl').exists() else []
            delays = (root / 'sleep.calls').read_text().splitlines() if (root / 'sleep.calls').exists() else []
            timeouts = (root / 'timeout.calls').read_text().splitlines() if (root / 'timeout.calls').exists() else []
            self.assertEqual(list(temporary.iterdir()), [], 'temporary diagnostic retained')
            self.assertTrue(all(call == ['mod', 'download'] for call in calls), calls)
            return result, calls, delays, timeouts

    def test_success_has_one_unchanged_download(self):
        result, calls, delays, timeouts = self.run_download([(0, 'download complete\n')])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'download complete\n')
        self.assertEqual(len(calls), 1)
        self.assertEqual(delays, [])
        self.assertEqual(timeouts, ['-k 5 180 go mod download'])

    def test_docker_stage_recovers_observed_failure(self):
        result, calls, delays, timeouts = self.run_download(
            [(1, STREAM_ERROR), (0, '')], command=docker_download_command())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 2)
        self.assertEqual(delays, ['2'])
        self.assertEqual(timeouts, ['-k 5 180 go mod download'] * 2)
        self.assertIn(STREAM_ERROR, result.stderr)

    def test_two_failures_can_recover_on_last_attempt(self):
        result, calls, delays, _ = self.run_download([(1, STREAM_ERROR), (1, STREAM_ERROR), (0, '')])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 3)
        self.assertEqual(delays, ['2', '4'])
        self.assertEqual(result.stderr.count(STREAM_ERROR), 2)

    def test_repeated_failure_exhausts_fixed_budget_and_stays_failed(self):
        result, calls, delays, _ = self.run_download([(1, STREAM_ERROR)])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 3)
        self.assertEqual(delays, ['2', '4'])
        self.assertEqual(result.stderr.count(STREAM_ERROR), 3)

    def test_multiple_matching_module_failures_can_recover(self):
        result, calls, _, _ = self.run_download([(1, STREAM_ERROR * 2), (0, '')])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 2)

    def test_access_integrity_and_unknown_errors_never_retry(self):
        for error in ['401 Unauthorized', '403 Forbidden', 'SECURITY ERROR',
                      'checksum mismatch', 'unknown revision v9.0.0',
                      'x509: certificate signed by unknown authority',
                      'proxyconnect tcp: connection refused', '']:
            for transcript in [error, STREAM_ERROR + error]:
                with self.subTest(error=error, mixed=transcript.startswith(STREAM_ERROR)):
                    # An empty extra line is unknown too; it cannot disguise
                    # an access or integrity failure as a transport-only one.
                    result, calls, delays, _ = self.run_download([(1, transcript + '\n'), (0, '')])
                    self.assertEqual(result.returncode, 1)
                    self.assertEqual(len(calls), 1)
                    self.assertEqual(delays, [])

    def test_empty_transcript_never_retries(self):
        result, calls, delays, _ = self.run_download([(1, ''), (0, '')])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 1)
        self.assertEqual(delays, [])

    def test_only_exact_https_stream_signature_retries(self):
        for text in [STREAM_ERROR.replace('https:', 'http:'),
                     STREAM_ERROR.replace('INTERNAL_ERROR', 'REFUSED_STREAM'),
                     STREAM_ERROR.replace('received from peer', 'other failure'),
                     'an unrelated log mentioning ' + STREAM_ERROR]:
            with self.subTest(text=text):
                result, calls, delays, _ = self.run_download([(1, text), (0, '')])
                self.assertEqual(result.returncode, 1)
                self.assertEqual(len(calls), 1)
                self.assertEqual(delays, [])

    def test_nonordinary_command_exit_is_preserved_without_retry(self):
        for status in [2, 124, 126, 127, 130, 137, 143]:
            with self.subTest(status=status):
                result, calls, delays, _ = self.run_download([(status, STREAM_ERROR), (0, '')])
                self.assertEqual(result.returncode, status)
                self.assertEqual(len(calls), 1)
                self.assertEqual(delays, [])

    def test_timeout_or_missing_timeout_never_runs_a_second_download(self):
        for status in [124, 127, 137, 143]:
            with self.subTest(status=status):
                result, calls, delays, timeouts = self.run_download([(0, '')], timeout_status=status)
                self.assertEqual(result.returncode, status)
                self.assertEqual(calls, [])
                self.assertEqual(delays, [])
                self.assertEqual(timeouts, ['-k 5 180 go mod download'])

    def test_arbitrary_arguments_are_rejected_before_download(self):
        result, calls, delays, timeouts = self.run_download([(0, '')], args=('-x',))
        self.assertEqual(result.returncode, 2)
        self.assertEqual(calls, [])
        self.assertEqual(delays, [])
        self.assertEqual(timeouts, [])

    def test_docker_copies_helper_before_download_and_has_no_proxy_override(self):
        dockerfile = (ROOT / 'Dockerfile').read_text()
        copy = 'COPY scripts/go-mod-download.sh /usr/local/bin/pulse-go-mod-download'
        self.assertIn(copy, dockerfile)
        self.assertLess(dockerfile.index(copy), dockerfile.index('sh /usr/local/bin/pulse-go-mod-download'))
        source = HELPER.read_text()
        self.assertNotIn('GOPROXY=', source)
        self.assertNotIn('GOSUMDB=', source)
        self.assertNotIn('go mod tidy', source)
        self.assertNotIn('go get', source)
        self.assertNotIn('chmod', source)


if __name__ == '__main__':
    unittest.main(verbosity=2)
