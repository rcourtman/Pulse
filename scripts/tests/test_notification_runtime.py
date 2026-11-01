#!/usr/bin/env python3
"""Controls for the source-bound notification journey and its owned loopback sink."""
import contextlib
import copy
import hashlib
import http.client
import importlib.util
import io
import json
from pathlib import Path
import socket
import sys
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location('notification_runtime', ROOT / 'scripts/check_notification_runtime.py')
RUNTIME = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RUNTIME)
SHA = 'a' * 40


class FakeResponse:
    def __init__(self, raw=b'{}', status=200):
        self.raw, self.status = raw, status
        self.limit = None

    def __enter__(self):
        return self

    def __exit__(self, *_):
        return None

    def read(self, limit):
        self.limit = limit
        return self.raw


class NotificationRuntimeControls(unittest.TestCase):
    def valid_row(self):
        return {'path': '/owned-alert', 'body': {'resourceName': 'owned.example.test',
                    'instance': RUNTIME.PUBLIC_URL, 'event': 'alert', 'type': 'cpu'}}

    def journey(self):
        return RUNTIME.Journey(Path('/unused-binary'), Path('/unused-data'), io.BytesIO(), None)

    def test_current_ordinary_event_not_resolution_or_invented_firing(self):
        source = (ROOT / 'internal/notifications/notifications.go').read_text()
        prepare = source.split('func (n *NotificationManager) prepareWebhookData(', 1)[1]
        self.assertRegex(prepare, r'Event:\s+"alert",')
        self.assertEqual(RUNTIME.ORDINARY_EVENT, 'alert')
        row = self.valid_row()
        self.assertTrue(all(RUNTIME.delivery_checks(row, 'owned.example.test').values()))
        for event in ('firing', 'resolved', None):
            with self.subTest(event=event):
                row['body']['event'] = event
                self.assertFalse(RUNTIME.delivery_checks(row, 'owned.example.test')['ordinary_event'])

    def test_all_five_delivery_conditions_are_independent(self):
        cases = [('path', '/foreign', 'owned_path'),
                 ('resourceName', 'other.example.test', 'resource_identity'),
                 ('instance', 'https://other.example.test', 'saved_public_url'),
                 ('event', 'resolved', 'ordinary_event'),
                 ('type', 'memory', 'cpu_metric')]
        for field, value, failing in cases:
            with self.subTest(field=field):
                row = copy.deepcopy(self.valid_row())
                target = row if field == 'path' else row['body']
                target[field] = value
                checks = RUNTIME.delivery_checks(row, 'owned.example.test')
                self.assertEqual([key for key, passed in checks.items() if not passed], [failing])

    def test_uncertain_mutation_is_never_replayed_and_keeps_request_identity(self):
        journey = self.journey()
        client = mock.Mock()
        client.open.side_effect = TimeoutError('private provider text must not be printed')
        journey.client = client
        with contextlib.redirect_stdout(io.StringIO()) as output:
            with self.assertRaises(TimeoutError):
                journey.request('/api/alerts/config', {'privateValue': 'do-not-log'}, method='PUT')
        client.open.assert_called_once()
        self.assertEqual(journey.active_request, {'path': '/api/alerts/config', 'method': 'PUT',
                                                  'authentication': 'session'})
        self.assertEqual(journey.requests, [])
        self.assertNotIn('do-not-log', output.getvalue())
        self.assertNotIn('private provider text', output.getvalue())

    def test_expected_refusal_does_not_decode_or_print_response_body(self):
        journey = self.journey()
        response = FakeResponse(b'private rejection detail', 403)
        journey.client = mock.Mock()
        journey.client.open.return_value = response
        with contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertIsNone(journey.request('/api/system/settings', expected=403))
        self.assertEqual(response.limit, RUNTIME.LIMIT + 1)
        self.assertNotIn('private rejection detail', output.getvalue())
        self.assertEqual(journey.requests[-1]['status'], 403)
        self.assertIsNone(journey.active_request)

    def test_oversized_success_stops_after_one_read(self):
        journey = self.journey()
        response = FakeResponse(b'x' * (RUNTIME.LIMIT + 1))
        journey.client = mock.Mock()
        journey.client.open.return_value = response
        with contextlib.redirect_stdout(io.StringIO()):
            with self.assertRaisesRegex(RUNTIME.Failure, 'byte bound'):
                journey.request('/api/resources')
        journey.client.open.assert_called_once()

    def test_settings_patch_preserves_and_tests_deployment_telemetry_lock(self):
        journey = self.journey()
        journey.request = mock.Mock()
        with contextlib.redirect_stdout(io.StringIO()):
            journey.save_system_policy()
        self.assertEqual(journey.request.call_args_list, [
            mock.call('/api/system/settings/update', {'telemetryEnabled': False}, expected=409),
            mock.call('/api/system/settings/update', {'publicURL': RUNTIME.PUBLIC_URL,
                      'webhookAllowedPrivateCIDRs': '127.0.0.1/32'})])
        self.assertEqual(journey.phases, ['deployment-telemetry-lock-preserved'])
        handler = (ROOT / 'internal/api/system_settings.go').read_text()
        self.assertIn('rawRequest["telemetryEnabled"]; ok && h.config.EnvOverrides["PULSE_TELEMETRY"]', handler)

    def test_policy_readback_requires_disabled_locked_telemetry(self):
        system = {'publicURL': RUNTIME.PUBLIC_URL, 'webhookAllowedPrivateCIDRs': '127.0.0.1/32',
                  'telemetryEnabled': False, 'envOverrides': {'PULSE_TELEMETRY': True}}
        alerts = {'enabled': True, 'activationState': 'active',
                  'agentDefaults': {'cpu': {'trigger': 80}}}
        for telemetry, lock, allowed in [(False, True, True), (True, True, False),
                                         (None, True, False), (False, False, False)]:
            with self.subTest(telemetry=telemetry, lock=lock):
                journey = self.journey()
                settings = copy.deepcopy(system)
                settings['telemetryEnabled'], settings['envOverrides']['PULSE_TELEMETRY'] = telemetry, lock
                journey.request = mock.Mock(side_effect=[settings, alerts])
                if allowed:
                    journey.assert_settings()
                else:
                    with self.assertRaisesRegex(RUNTIME.Failure, 'telemetry restriction'):
                        journey.assert_settings()

    def test_only_clean_matching_checkout_and_binary_admitted(self):
        valid = '\tbuild\tvcs=git\n\tbuild\tvcs.revision=' + SHA + '\n\tbuild\tvcs.modified=false\n'
        cases = [(SHA, 0, valid, True),
                 ('b' * 40, 0, valid, False),
                 (SHA, 1, valid, False),
                 (SHA, 0, valid.replace('modified=false', 'modified=true'), False),
                 (SHA, 0, valid.replace(SHA, 'b' * 40), False),
                 (SHA, 0, valid.replace('vcs=git', 'vcs=hg'), False),
                 (SHA, 0, '', False)]
        for actual, dirty, binary, allowed in cases:
            with self.subTest(actual=actual, dirty=dirty, binary=binary):
                with mock.patch.object(RUNTIME.subprocess, 'check_output', side_effect=[actual, binary]), \
                     mock.patch.object(RUNTIME.subprocess, 'run', return_value=SimpleNamespace(returncode=dirty)):
                    if allowed:
                        self.assertEqual(RUNTIME.source_binding(Path('/owned-binary'), SHA)['checkout_sha'], SHA)
                    else:
                        with self.assertRaises(RUNTIME.Failure):
                            RUNTIME.source_binding(Path('/owned-binary'), SHA)

    def test_malformed_source_identity_stops_before_commands(self):
        with mock.patch.object(RUNTIME.subprocess, 'check_output') as command:
            with self.assertRaises(RUNTIME.Failure):
                RUNTIME.source_binding(Path('/owned-binary'), 'main')
            command.assert_not_called()

    def test_owned_child_has_no_ambient_credentials_or_telemetry(self):
        journey = self.journey()
        child = mock.Mock()
        child.poll.return_value = None
        journey.request = mock.Mock(return_value=None)
        with mock.patch.dict(RUNTIME.os.environ, {'PATH': '/usr/bin', 'AMBIENT_SECRET': 'not-forwarded'}, clear=True), \
             mock.patch.object(RUNTIME, 'free_port', return_value=7655), \
             mock.patch.object(RUNTIME.subprocess, 'Popen', return_value=child) as launch:
            journey.start()
        env = launch.call_args.kwargs['env']
        self.assertNotIn('AMBIENT_SECRET', env)
        self.assertEqual(env['PULSE_TELEMETRY'], 'false')
        self.assertEqual(env['PULSE_MOCK_MODE'], 'false')
        self.assertEqual(env['DISCOVERY_ENABLED'], 'false')
        self.assertEqual(env['BIND_ADDRESS'], '127.0.0.1')
        self.assertTrue(launch.call_args.kwargs['start_new_session'])

    def test_ci_reuses_the_real_embed_with_exact_source_and_attempt_receipt(self):
        workflow = (ROOT / '.github/workflows/build-and-test.yml').read_text()
        section = workflow.split('- name: Check ordinary notification policy and restart', 1)[1]
        section = section.split('\n  frontend-unit:', 1)[0]
        self.assertIn('timeout-minutes: 3', section)
        self.assertIn('--binary "$RUNNER_TEMP/pulse-real-embed"', section)
        self.assertIn('--expected-source-sha "${{ github.sha }}"', section)
        self.assertIn('--receipt "$RUNNER_TEMP/pulse-notification-runtime.json"', section)
        self.assertIn('if: always()', section)
        self.assertIn('notification-runtime-${{ github.run_id }}-${{ github.run_attempt }}', section)
        self.assertNotIn('sudo', section)
        self.assertNotIn('go build', section)


class NotificationSinkControls(unittest.TestCase):
    def test_real_request_preserves_payload_and_closes_every_owned_thread(self):
        sink = RUNTIME.Sink()
        payload = b'{"event":"alert","resourceName":"owned.example.test"}'
        client = http.client.HTTPConnection('127.0.0.1', sink.port, timeout=3)
        try:
            client.request('POST', '/owned-alert', body=payload)
            response = client.getresponse()
            self.assertEqual(response.status, 200)
            self.assertEqual(response.read(), b'OK')
            self.assertEqual(sink.snapshot(), [{'body': json.loads(payload),
                'bytes': len(payload), 'sha256': hashlib.sha256(payload).hexdigest(),
                'path': '/owned-alert'}])
        finally:
            client.close()
            sink.close()
        self.assertIsNone(sink.error)
        self.assertFalse(sink.thread.is_alive())
        self.assertFalse(any(thread.is_alive() for thread in sink.server._threads))

    def stalled_request(self, prefix, trickle=False):
        # Exercise the real parser, socket and non-daemon request thread. Use
        # a shorter instance of the fixed timer, not a fake socket/clock.
        with mock.patch.object(RUNTIME, 'SINK_REQUEST_TIMEOUT_SECONDS', .25):
            sink = RUNTIME.Sink()
            accepted = threading.Event()
            closed = threading.Event()
            stop_writes = threading.Event()
            errors = []
            original = sink.server.finish_request

            def observe(*args):
                accepted.set()
                original(*args)

            def close():
                try:
                    sink.close()
                except Exception as exc:
                    errors.append(type(exc).__name__)
                finally:
                    closed.set()

            sink.server.finish_request = observe
            peer = socket.create_connection(('127.0.0.1', sink.port), timeout=2)
            closer = None
            writer = None
            bounded = False
            try:
                peer.sendall(prefix)
                self.assertTrue(accepted.wait(2), 'loopback request was not accepted')
                if trickle:
                    def write():
                        while not stop_writes.wait(.02):
                            try:
                                peer.sendall(b' ')
                            except OSError:
                                break
                    writer = threading.Thread(target=write, name='owned-fixture-trickle')
                    writer.start()
                closer = threading.Thread(target=close, name='owned-fixture-close')
                closer.start()
                bounded = closed.wait(2)
            finally:
                # A broken regression must still finish the fixture, not hang
                # its caller. Release the peer even when bounded closure failed.
                stop_writes.set()
                try:
                    peer.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
                peer.close()
                if writer:
                    writer.join(timeout=2)
                if closer:
                    closer.join(timeout=2)
                else:
                    sink.close()
            self.assertTrue(bounded, 'owned sink cleanup waited for the stalled peer')
            self.assertEqual(errors, [])
            self.assertFalse(closer.is_alive())
            self.assertFalse(writer and writer.is_alive())
            self.assertFalse(any(thread.is_alive() for thread in sink.server._threads))
            self.assertEqual(sink.error, 'TimeoutError')
            self.assertEqual(sink.snapshot(), [])

    def test_partial_headers_cannot_block_cleanup(self):
        self.stalled_request(b'POST /owned-alert HTTP/1.0\r\nContent-Length:')

    def test_partial_body_cannot_block_cleanup(self):
        self.stalled_request(b'POST /owned-alert HTTP/1.0\r\nContent-Length: 100\r\n\r\n{')

    def test_body_trickle_cannot_reset_the_absolute_deadline(self):
        self.stalled_request(b'POST /owned-alert HTTP/1.0\r\nContent-Length: 10000\r\n\r\n{',
                             trickle=True)

    def test_invalid_http_records_closed_failure_without_payload_output(self):
        sink = RUNTIME.Sink()
        client = http.client.HTTPConnection('127.0.0.1', sink.port, timeout=3)
        try:
            with contextlib.redirect_stderr(io.StringIO()) as errors:
                client.request('PRIVATE_PAYLOAD', '/owned-alert')
                response = client.getresponse()
                self.assertEqual(response.status, 501)
                response.read()
                client.close()
                sink.close()
            self.assertEqual(errors.getvalue(), '')
        finally:
            client.close()
            sink.close()
        self.assertEqual(sink.error, 'HTTPError')
        self.assertEqual(sink.snapshot(), [])

    def test_invalid_json_rejects_without_printing_its_private_body(self):
        sink = RUNTIME.Sink()
        client = http.client.HTTPConnection('127.0.0.1', sink.port, timeout=3)
        try:
            with contextlib.redirect_stderr(io.StringIO()) as errors:
                client.request('POST', '/owned-alert', body=b'PRIVATE_SYNTHETIC_BODY')
                response = client.getresponse()
                self.assertEqual(response.status, 400)
                self.assertEqual(response.read(), b'')
                client.close()
                sink.close()
            self.assertEqual(errors.getvalue(), '')
        finally:
            client.close()
            sink.close()
        self.assertEqual(sink.error, 'JSONDecodeError')
        self.assertEqual(sink.snapshot(), [])

    def test_first_destination_failure_is_not_replaced(self):
        sink = RUNTIME.Sink()
        try:
            sink.reject('TimeoutError')
            sink.reject('Failure')
            self.assertEqual(sink.error, 'TimeoutError')
        finally:
            sink.close()

    def test_terminal_receipt_distinguishes_sink_failure_from_completed_cleanup(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / 'owned-binary'
            binary.write_bytes(b'synthetic binary identity')
            receipt = root / 'receipt.json'
            sink = SimpleNamespace(error='TimeoutError', close=mock.Mock())
            journey = SimpleNamespace(run=mock.Mock(), phases=[], exits=[], requests=[],
                                      active_request=None, deliveries=[], delivery_observations=[])
            args = ['check_notification_runtime.py', '--binary', str(binary),
                    '--expected-source-sha', SHA, '--receipt', str(receipt)]
            with mock.patch.object(sys, 'argv', args), \
                 mock.patch.object(RUNTIME, 'source_binding', return_value={'checkout_sha': SHA}), \
                 mock.patch.object(RUNTIME, 'Sink', return_value=sink), \
                 mock.patch.object(RUNTIME, 'Journey', return_value=journey), \
                 mock.patch.object(RUNTIME.subprocess, 'run', return_value=SimpleNamespace(returncode=0)), \
                 contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(RUNTIME.main(), 1)
            report = json.loads(receipt.read_text())
            self.assertEqual(report['outcome'], 'failed')
            self.assertEqual(report['sink_error'], 'TimeoutError')
            self.assertEqual(report['sink_request_timeout_seconds'], 2)
            self.assertTrue(report['sink_thread_joined_and_listener_closed'])
            self.assertTrue(report['owned_temporary_data_removed'])
            self.assertTrue(report['source_clean_after'])
            self.assertNotIn('cleanup_error', report)
            sink.close.assert_called_once()
            journey.run.assert_called_once()


if __name__ == '__main__':
    unittest.main(verbosity=2)
