#!/usr/bin/env python3
"""Execute module downloads and their Docker/hosted workflow consumers offline."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/go-mod-download.sh'
STREAM_ERROR = ('go: go.opentelemetry.io/otel@v1.44.0: read '
                '"https://proxy.golang.org/go.opentelemetry.io/otel/@v/v1.44.0.zip": '
                'stream error: stream ID 693; INTERNAL_ERROR; received from peer\n')
TLS_TIMEOUT = ('go: golang.org/x/crypto@v0.58.0: Get '
               '"https://proxy.golang.org/golang.org/x/crypto/@v/v0.58.0.zip": '
               'net/http: TLS handshake timeout\n')
SOURCE_SHA = 'a' * 40
WORKFLOW = ROOT / '.github/workflows/compile-release-payload.yml'


def qualification_job():
    return WORKFLOW.read_text().split('  compile-secure-runtime-qualification:\n', 1)[1]


def qualification_build_command():
    step = qualification_job().split('      - name: Build hosted qualification subjects\n', 1)[1]
    step = step.split('\n      - name:', 1)[0]
    return textwrap.dedent(step.split('        run: |\n', 1)[1])


def docker_download_command():
    # Execute the actual RUN command, omitting only Docker cache-mount syntax.
    dockerfile = Path(os.environ.get('PULSE_TEST_DOCKERFILE', ROOT / 'Dockerfile')).read_text()
    stage = dockerfile.split('COPY go.mod go.sum ./\n', 1)[1].split('\n\n', 1)[0]
    command = stage.split('RUN ', 1)[1]
    command = re.sub(r'--mount=[^\s]+\s*\\\n\s*', '', command).strip()
    return command.replace('/usr/local/bin/pulse-go-mod-download', str(HELPER))


class GoModDownloadFixture(unittest.TestCase):
    def run_download(self, outcomes, *, command=None, timeout_status=None, args=(),
                     qualification=False, build_status=0, diff_status=0,
                     actual_sha=SOURCE_SHA, version_file='6.8.0\n'):
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
            if qualification:
                # Execute the actual workflow run block. Only Git observations
                # and the expensive build are doubles; acquisition is the real
                # helper and must complete first, without a build retry.
                scripts = root / 'scripts'
                scripts.mkdir()
                shutil.copyfile(HELPER, scripts / 'go-mod-download.sh')
                (root / 'VERSION').write_text(version_file)
                git = tools / 'git'
                git.write_text('''#!/bin/sh
case "$*" in
  'rev-parse HEAD') printf '%s\\n' "$FIXTURE_SOURCE_SHA" ;;
  'diff --exit-code -- go.mod go.sum') exit "$FIXTURE_DIFF_STATUS" ;;
  *) exit 99 ;;
esac
''')
                git.chmod(0o755)
                build = scripts / 'build-secure-runtime-qualification.sh'
                build.write_text('''#!/bin/sh
[ "$#" = 3 ] && [ "$1" = "$VERSION" ] &&
  [ "$2" = "$RUNNER_TEMP/secure-runtime-qualification" ] &&
  [ "$3" = amd64 ] || exit 99
echo 'qualification subject build invoked'
exit "$FIXTURE_BUILD_STATUS"
''')
                build.chmod(0o755)
                env.update(VERSION='6.8.0', EXPECTED_SOURCE_SHA=SOURCE_SHA,
                           RUNNER_TEMP=str(root / 'runner'),
                           FIXTURE_SOURCE_SHA=actual_sha,
                           FIXTURE_DIFF_STATUS=str(diff_status),
                           FIXTURE_BUILD_STATUS=str(build_status))
            if timeout_status is not None:
                env['FIXTURE_TIMEOUT_STATUS'] = str(timeout_status)
            else:
                env.pop('FIXTURE_TIMEOUT_STATUS', None)
            if qualification:
                argv = ['bash', '-c', qualification_build_command()]
            else:
                argv = ['sh', '-c', command] if command else ['sh', str(HELPER), *args]
            result = subprocess.run(argv, cwd=root if qualification else ROOT, env=env, capture_output=True,
                                    text=True, timeout=10)
            calls = [json.loads(line) for line in (root / 'calls.jsonl').read_text().splitlines()] if (root / 'calls.jsonl').exists() else []
            delays = (root / 'sleep.calls').read_text().splitlines() if (root / 'sleep.calls').exists() else []
            timeouts = (root / 'timeout.calls').read_text().splitlines() if (root / 'timeout.calls').exists() else []
            self.assertEqual(list(temporary.iterdir()), [], 'temporary diagnostic retained')
            self.assertTrue(all(call == ['mod', 'download'] for call in calls), calls)
            return result, calls, delays, timeouts


class GoModDownloadTests(GoModDownloadFixture):
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

    def test_observed_official_proxy_tls_timeout_recovers_with_same_download(self):
        result, calls, delays, timeouts = self.run_download([(1, TLS_TIMEOUT), (0, '')])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 2)
        self.assertEqual(delays, ['2'])
        self.assertEqual(timeouts, ['-k 5 180 go mod download'] * 2)
        self.assertIn(TLS_TIMEOUT, result.stderr)

    def test_mixed_known_transport_failures_can_recover(self):
        result, calls, delays, _ = self.run_download(
            [(1, STREAM_ERROR + TLS_TIMEOUT), (0, '')])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 2)
        self.assertEqual(delays, ['2'])

    def test_tls_timeout_exhausts_the_same_fixed_budget(self):
        result, calls, delays, _ = self.run_download([(1, TLS_TIMEOUT)])
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 3)
        self.assertEqual(delays, ['2', '4'])
        self.assertEqual(result.stderr.count(TLS_TIMEOUT), 3)

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

    def test_tls_timeout_does_not_hide_refusal_integrity_or_unknown_diagnostics(self):
        for error in ['401 Unauthorized', '403 Forbidden', 'SECURITY ERROR',
                      'checksum mismatch', 'unknown revision v9.0.0',
                      'x509: certificate signed by unknown authority',
                      'proxyconnect tcp: connection refused', '']:
            for transcript in [TLS_TIMEOUT + error + '\n', error + '\n' + TLS_TIMEOUT]:
                with self.subTest(error=error, transcript=transcript):
                    result, calls, delays, _ = self.run_download([(1, transcript), (0, '')])
                    self.assertEqual(result.returncode, 1)
                    self.assertEqual(len(calls), 1)
                    self.assertEqual(delays, [])

    def test_only_the_exact_official_https_tls_timeout_signature_retries(self):
        for text in [TLS_TIMEOUT.replace('https:', 'http:'),
                     TLS_TIMEOUT.replace('proxy.golang.org/', 'proxy.example.invalid/'),
                     TLS_TIMEOUT.replace('proxy.golang.org/', 'proxy.golang.org.example.invalid/'),
                     TLS_TIMEOUT.replace('proxy.golang.org/', 'user@proxy.golang.org/'),
                     TLS_TIMEOUT.replace('TLS handshake timeout', 'TLS handshake failure'),
                     TLS_TIMEOUT.replace('Get ', 'read '),
                     'an unrelated log mentioning ' + TLS_TIMEOUT]:
            with self.subTest(text=text):
                result, calls, delays, _ = self.run_download([(1, text), (0, '')])
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
            for diagnostic in [STREAM_ERROR, TLS_TIMEOUT]:
                with self.subTest(status=status, diagnostic=diagnostic):
                    result, calls, delays, _ = self.run_download([(status, diagnostic), (0, '')])
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


class HostedQualificationDownloadTests(GoModDownloadFixture):
    def test_real_workflow_block_recovers_before_one_unchanged_subject_build(self):
        result, calls, delays, timeouts = self.run_download(
            [(1, TLS_TIMEOUT), (0, '')], qualification=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 2)
        self.assertEqual(delays, ['2'])
        self.assertEqual(timeouts, ['-k 5 180 go mod download'] * 2)
        self.assertEqual(result.stdout.count('qualification subject build invoked'), 1)
        self.assertIn(TLS_TIMEOUT, result.stderr)

    def test_successful_download_proceeds_once_without_retry(self):
        result, calls, delays, _ = self.run_download([(0, '')], qualification=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 1)
        self.assertEqual(delays, [])
        self.assertEqual(result.stdout.count('qualification subject build invoked'), 1)

    def test_download_refusal_integrity_unknown_and_exhaustion_never_build(self):
        for diagnostic, attempts in [(TLS_TIMEOUT, 3), (STREAM_ERROR, 3),
                                     ('401 Unauthorized\n', 1), ('403 Forbidden\n', 1),
                                     ('checksum mismatch\n', 1),
                                     (TLS_TIMEOUT + 'unknown diagnostic\n', 1)]:
            with self.subTest(diagnostic=diagnostic):
                result, calls, _, _ = self.run_download([(1, diagnostic)], qualification=True)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(len(calls), attempts)
                self.assertNotIn('qualification subject build invoked', result.stdout)

    def test_wrong_source_or_version_stops_before_download_or_build(self):
        for identity in [dict(actual_sha='b' * 40), dict(version_file='6.8.1\n')]:
            with self.subTest(identity=identity):
                result, calls, delays, timeouts = self.run_download(
                    [(0, '')], qualification=True, **identity)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual((calls, delays, timeouts), ([], [], []))
                self.assertNotIn('qualification subject build invoked', result.stdout)

    def test_manifest_change_stops_after_download_without_build(self):
        result, calls, delays, _ = self.run_download(
            [(0, '')], qualification=True, diff_status=1)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(len(calls), 1)
        self.assertEqual(delays, [])
        self.assertNotIn('qualification subject build invoked', result.stdout)

    def test_build_failure_is_preserved_without_retry(self):
        result, calls, delays, _ = self.run_download(
            [(0, '')], qualification=True, build_status=42)
        self.assertEqual(result.returncode, 42)
        self.assertEqual(len(calls), 1)
        self.assertEqual(delays, [])
        self.assertEqual(result.stdout.count('qualification subject build invoked'), 1)

    def test_download_timeout_is_terminal_before_build(self):
        result, calls, delays, _ = self.run_download(
            [(0, '')], qualification=True, timeout_status=124)
        self.assertEqual(result.returncode, 124)
        self.assertEqual((calls, delays), ([], []))
        self.assertNotIn('qualification subject build invoked', result.stdout)

    def test_hosted_trust_identity_and_manifest_binding_are_unchanged(self):
        job = qualification_job()
        self.assertIn('runs-on: ubuntu-24.04', job)
        self.assertIn('timeout-minutes: 20', job)
        self.assertIn('cache: false', job)
        self.assertIn('persist-credentials: false', job)
        self.assertIn('ref: ${{ inputs.source_sha }}', job)
        self.assertIn('test "${GITHUB_WORKFLOW_SHA}" = "${EXPECTED_SOURCE_SHA}"', job)
        self.assertNotIn('continue-on-error:', job)
        self.assertNotIn('GOPROXY', job)
        self.assertNotIn('GOSUMDB', job)
        self.assertLess(job.index('git diff --exit-code -- go.mod go.sum'),
                        job.index('./scripts/build-secure-runtime-qualification.sh'))
        self.assertLess(job.index('./scripts/build-secure-runtime-qualification.sh'),
                        job.index('- name: Attest hosted qualification subjects'))
        for version in (6, 7):
            manifest = ROOT / f'scripts/release_control/secure_runtime_source_manifest_v{version}.json'
            self.assertIn('scripts/go-mod-download.sh', json.loads(manifest.read_text())['exact_paths'])


if __name__ == '__main__':
    unittest.main(verbosity=2)
