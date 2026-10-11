#!/usr/bin/env python3
"""Controls for the source-bound ordinary notification journey, without a server."""
import contextlib
import copy
import importlib.util
import io
from pathlib import Path
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
        section = section.split('# An audit service failure', 1)[0]
        self.assertIn('timeout-minutes: 3', section)
        self.assertIn('--binary "$RUNNER_TEMP/pulse-real-embed"', section)
        self.assertIn('--expected-source-sha "${{ github.sha }}"', section)
        self.assertIn('--receipt "$RUNNER_TEMP/pulse-notification-runtime.json"', section)
        self.assertIn('if: always()', section)
        self.assertIn('notification-runtime-${{ github.run_id }}-${{ github.run_attempt }}', section)
        self.assertNotIn('sudo', section)
        self.assertNotIn('go build', section)


if __name__ == '__main__':
    unittest.main(verbosity=2)
