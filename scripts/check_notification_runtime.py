#!/usr/bin/env python3
"""Owned offline generic/Telegram receivers; not installed/provider acceptance.

All data, credentials and network endpoints are generated in this guest.
Never prints credentials, response bodies or application logs. Mutations are
single submissions; uncertain POST/PUT outcomes stop without replay.
"""
import argparse
import datetime
import hashlib
import http.cookiejar
import http.server
import json
import os
from pathlib import Path
import re
import secrets
import signal
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request

PUBLIC_URL = 'https://notification-runtime.example.test'
LIMIT = 2 * 1024 * 1024
# prepareWebhookData's ordinary event; resolution is a different path.
ORDINARY_EVENT = 'alert'
TELEGRAM_PATH = '/owned-telegram/sendMessage?chat_id=-1001234'
TELEGRAM_CHAT = '-1001234'
FIXTURE_HEADER = 'owned-synthetic'


class Failure(RuntimeError):
    pass


def require(condition, message):
    if not condition:
        raise Failure(message)


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


class Sink:
    def __init__(self):
        self.rows = []
        self.lock = threading.Lock()
        self.error = None
        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                try:
                    size = int(self.headers.get('Content-Length', '-1'))
                    require(0 < size <= 65536, 'sink request exceeded declared bound')
                    raw = self.rfile.read(size)
                    require(len(raw) == size, 'sink short read')
                    obj = json.loads(raw)
                    require(isinstance(obj, dict), 'sink non-object')
                    row = {'body': obj, 'sha256': hashlib.sha256(raw).hexdigest(),
                           'bytes': size, 'path': self.path,
                           'content_types': self.headers.get_all('Content-Type', []),
                           'fixture_headers': self.headers.get_all('X-Pulse-Fixture', [])}
                    require(self.path in ('/owned-alert', TELEGRAM_PATH), 'unexpected sink path')
                    # Like Telegram, refuse an untyped/empty message instead of
                    # accepting arbitrary JSON as proof of an ordinary alert.
                    accepted = self.path != TELEGRAM_PATH or all(telegram_wire_checks(row).values())
                    with owner.lock:
                        owner.rows.append(row)
                    response = b'{"ok":true}' if accepted else b'{"ok":false}'
                    self.send_response(200 if accepted else 400)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(response)))
                    self.end_headers()
                    self.wfile.write(response)
                except Exception as exc:
                    owner.error = type(exc).__name__
                    self.send_response(400)
                    self.send_header('Content-Length', '0')
                    self.end_headers()

        self.server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        self.server.daemon_threads = False
        self.port = self.server.server_port
        self.thread = threading.Thread(target=self.server.serve_forever, name='owned-webhook')
        self.thread.start()

    def snapshot(self):
        with self.lock:
            return list(self.rows)

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)
        require(not self.thread.is_alive(), 'sink thread did not join')
        with socket.socket() as sock:
            sock.settimeout(1)
            require(sock.connect_ex(('127.0.0.1', self.port)) != 0, 'sink listener stayed open')


def delivery_checks(row, expected_name):
    """Keep each asserted closed condition, not private/untrusted payload text."""
    body = row['body']
    return {'owned_path': row['path'] == '/owned-alert',
            'resource_identity': body.get('resourceName') == expected_name,
            'saved_public_url': body.get('instance') == PUBLIC_URL,
            'ordinary_event': body.get('event') == ORDINARY_EVENT,
            'cpu_metric': body.get('type') == 'cpu'}


def telegram_wire_checks(row):
    body = row['body']
    text = body.get('text')
    return {'owned_path': row['path'] == TELEGRAM_PATH,
            'json_content_type': row.get('content_types') == ['application/json'],
            'nonempty_text': isinstance(text, str) and bool(text.strip()),
            'chat_identity': body.get('chat_id') == TELEGRAM_CHAT}


def telegram_delivery_checks(row, expected_name):
    text = row['body'].get('text')
    text = text if isinstance(text, str) else ''
    return dict(telegram_wire_checks(row),
                resource_identity=('• Resource: ' + expected_name) in text.splitlines(),
                ordinary_event=text.startswith('*Pulse Alert:'),
                cpu_metric='Type: Cpu' in text,
                saved_public_url=PUBLIC_URL + '/alerts/history?alert=' in text,
                markdown=row['body'].get('parse_mode') == 'Markdown',
                unrelated_header=row.get('fixture_headers') == [FIXTURE_HEADER])


def source_binding(binary, expected):
    require(bool(re.fullmatch(r'[0-9a-f]{40}', expected)), 'invalid expected source identity')
    actual = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
    require(actual == expected, 'checkout differs from expected source')
    require(subprocess.run(['git', 'diff', '--quiet', 'HEAD'], check=False).returncode == 0,
            'checkout has tracked source changes')
    version = subprocess.check_output(['go', 'version', '-m', str(binary)], text=True)
    fields = {}
    for line in version.splitlines():
        match = re.match(r'\s*build\s+(vcs(?:\.[a-z]+)?)=(.*)$', line)
        if match:
            fields[match[1]] = match[2]
    require(fields.get('vcs') == 'git' and fields.get('vcs.revision') == expected and
            fields.get('vcs.modified') == 'false', 'binary VCS identity is not clean exact source')
    return {'checkout_sha': actual, 'binary_vcs_revision': fields['vcs.revision'],
            'binary_vcs_modified': False}


class Journey:
    def __init__(self, binary, data, log, sink):
        self.binary, self.data, self.log, self.sink = binary, data, log, sink
        self.port = free_port()
        self.origin = f'http://127.0.0.1:{self.port}'
        self.cookies = http.cookiejar.CookieJar()
        self.client = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                                  urllib.request.HTTPCookieProcessor(self.cookies))
        self.password = secrets.token_hex(24)
        self.token = None
        self.token_id = None
        self.process = None
        self.phases, self.exits, self.requests, self.deliveries = [], [], [], []
        self.delivery_observations = []
        self.destination_readbacks = []
        self.active_request = None

    def phase(self, name):
        self.phases.append(name)
        print('PHASE=' + name, flush=True)

    def request(self, path, payload=None, method=None, expected=200, auth='session'):
        headers = {'Content-Type': 'application/json', 'User-Agent': 'pulse-notification-policy-journey'}
        client = self.client
        if auth != 'session':
            client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            if auth == 'agent':
                headers['Authorization'] = 'Bearer ' + self.token
        if payload is not None and auth == 'session':
            for cookie in self.cookies:
                if cookie.name == 'pulse_csrf':
                    headers['X-CSRF-Token'] = cookie.value
        req = urllib.request.Request(self.origin + path, headers=headers, method=method,
                                     data=None if payload is None else json.dumps(payload).encode())
        self.active_request = {'path': path, 'method': req.get_method(), 'authentication': auth}
        print('REQUEST_BEGIN=' + json.dumps(self.active_request, sort_keys=True), flush=True)
        start = time.monotonic()
        try:
            response = client.open(req, timeout=30)
        except urllib.error.HTTPError as exc:
            response = exc
        with response:
            raw = response.read(LIMIT + 1)
            obs = dict(self.active_request, status=response.status,
                       elapsed_seconds=round(time.monotonic() - start, 3))
            self.requests.append(obs)
            self.active_request = None
            print('REQUEST_END=' + json.dumps(obs, sort_keys=True), flush=True)
            require(len(raw) <= LIMIT, 'HTTP response exceeded byte bound')
            require(response.status == expected, f'{path} expected HTTP {expected}, observed {response.status}')
            if expected >= 400:
                return None
            try:
                return json.loads(raw) if raw else None
            except json.JSONDecodeError:
                raise Failure(path + ' successful response was not JSON') from None

    def start(self):
        env = {'PATH': os.environ['PATH'], 'HOME': str(self.data), 'LANG': 'C.UTF-8',
               'PULSE_DATA_DIR': str(self.data), 'BIND_ADDRESS': '127.0.0.1',
               'FRONTEND_PORT': str(self.port), 'PULSE_METRICS_PORT': str(free_port()),
               'PULSE_AUTH_USER': 'runtime-check', 'PULSE_AUTH_PASS': self.password,
               'PULSE_MOCK_MODE': 'false', 'DISCOVERY_ENABLED': 'false', 'PULSE_TELEMETRY': 'false'}
        self.process = subprocess.Popen([str(self.binary)], cwd=self.data, env=env,
                                        stdout=self.log, stderr=subprocess.STDOUT, start_new_session=True)
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            require(self.process.poll() is None, 'owned server exited before readiness')
            try:
                self.request('/api/resources', expected=401, auth='none')
                return
            except (urllib.error.URLError, TimeoutError, ConnectionError):
                time.sleep(.2)
        raise Failure('owned server readiness deadline expired')

    def stop(self):
        if self.process is None:
            return
        child, self.process = self.process, None
        started = time.monotonic()
        if child.poll() is None:
            os.killpg(child.pid, signal.SIGTERM)
        forced = False
        try:
            code = child.wait(timeout=40)
        except subprocess.TimeoutExpired:
            forced = True
            os.killpg(child.pid, signal.SIGKILL)
            code = child.wait(timeout=10)
        self.exits.append({'exit': code, 'forced': forced, 'waited': True,
                           'wait_seconds': round(time.monotonic() - started, 3)})
        require(code == 0 and not forced, 'owned server shutdown not clean')
        with socket.socket() as sock:
            sock.settimeout(1)
            require(sock.connect_ex(('127.0.0.1', self.port)) != 0, 'owned listener stayed open')

    def denied_destination(self):
        self.request('/api/notifications/webhooks',
                     {'name': 'unlisted-loopback', 'url': f'http://127.0.0.2:{self.sink.port}/blocked',
                      'enabled': True, 'method': 'POST', 'service': 'generic'}, expected=400)

    def save_system_policy(self):
        # This process's deployment owns telemetry=false. Do not weaken that
        # lock or include its field in an otherwise unrelated settings patch.
        self.request('/api/system/settings/update', {'telemetryEnabled': False}, expected=409)
        self.phase('deployment-telemetry-lock-preserved')
        self.request('/api/system/settings/update', {'publicURL': PUBLIC_URL,
                     'webhookAllowedPrivateCIDRs': '127.0.0.1/32'})

    def assert_settings(self):
        settings = self.request('/api/system/settings')
        require(settings.get('publicURL') == PUBLIC_URL and
                settings.get('webhookAllowedPrivateCIDRs') == '127.0.0.1/32', 'saved system policy changed')
        require(settings.get('telemetryEnabled') is False and
                settings.get('envOverrides', {}).get('PULSE_TELEMETRY') is True,
                'deployment telemetry restriction changed')
        cfg = self.request('/api/alerts/config')
        require(cfg.get('enabled') and cfg.get('activationState') == 'active' and
                cfg.get('agentDefaults', {}).get('cpu', {}).get('trigger') == 80,
                'saved threshold/activation policy changed')

    def assert_telegram_destination(self):
        destinations = self.request('/api/notifications/webhooks')
        require(isinstance(destinations, list), 'saved destination inventory is not a list')
        matches = [row for row in destinations if row.get('name') == 'owned-builtin-telegram']
        require(len(matches) == 1, 'built-in Telegram destination identity changed')
        row = matches[0]
        checks = {'builtin_service': row.get('service') == 'telegram',
                  'no_custom_template': not row.get('template'),
                  'owned_url': row.get('url') == f'http://127.0.0.1:{self.sink.port}' + TELEGRAM_PATH,
                  'enabled_post': row.get('enabled') is True and row.get('method') == 'POST',
                  'masked_headers': row.get('headers') ==
                      {'content-type': '***REDACTED***', 'X-Pulse-Fixture': '***REDACTED***'}}
        self.destination_readbacks.append(checks)
        require(all(checks.values()), 'saved built-in Telegram destination changed')

    def report(self, identity):
        payload = {'agent': {'id': 'agent-' + identity, 'version': '0.0.0-dev', 'type': 'unified',
                             'commandsEnabled': False, 'intervalSeconds': 60},
                   'host': {'id': 'machine-' + identity, 'machineId': 'machine-' + identity,
                            'hostname': 'notification-' + identity + '.example.test',
                            'platform': 'linux', 'cpuCount': 1},
                   'timestamp': datetime.datetime.now(datetime.timezone.utc).isoformat(),
                   'sequenceId': 'v1:notification-' + identity + ':1',
                   'metrics': {'cpuUsagePercent': 95, 'memory': {'totalBytes': 1073741824,
                                    'usedBytes': 429496729, 'freeBytes': 644245095, 'usage': 40}}}
        start = len(self.sink.snapshot())
        self.request('/api/agents/agent/report', payload, auth='agent')
        deadline = time.monotonic() + 30
        expected_name = payload['host']['hostname']
        while time.monotonic() < deadline:
            require(self.process.poll() is None, 'owned server exited during ordinary delivery')
            rows = self.sink.snapshot()[start:]
            observed = []
            for service, path, verify in [('generic', '/owned-alert', delivery_checks),
                                          ('telegram', TELEGRAM_PATH, telegram_delivery_checks)]:
                matches = [row for row in rows if row['path'] == path]
                require(len(matches) <= 1, 'same first alert delivered more than once')
                if not matches:
                    continue
                row = matches[0]
                checks = verify(row, expected_name)
                require(all(checks.values()), 'ordinary delivery checks failed: ' +
                        ','.join(sorted(key for key, passed in checks.items() if not passed)))
                observed.append((service, row, checks))
            if len(observed) == 2:
                for service, row, checks in observed:
                    self.delivery_observations.append({'resource': identity, 'service': service, 'checks': checks,
                                                       'bytes': row['bytes'], 'sha256': row['sha256']})
                    self.deliveries.append({'resource': identity, 'service': service,
                                            'event': ORDINARY_EVENT, 'metric': 'cpu',
                                            'saved_public_url': True, 'count': 1,
                                            'bytes': row['bytes'], 'sha256': row['sha256']})
                self.phase('ordinary-first-alert-' + identity)
                return
            require(self.sink.error is None, 'owned destination rejected payload')
            time.sleep(.2)
        active = self.request('/api/alerts/active')
        self.phase('ordinary-delivery-deadline-active-' + ('present' if bool(active) else 'empty'))
        raise Failure('ordinary first-alert destination deadline expired')

    def run(self):
        try:
            self.start()
            self.request('/api/login', {'username': 'runtime-check', 'password': self.password})
            require(any(c.name == 'pulse_csrf' for c in self.cookies), 'login missing CSRF cookie')
            require(self.request('/api/resources?limit=500').get('data') == [], 'initial live estate not empty')
            self.phase('authenticated-empty-live-startup')
            self.save_system_policy()
            self.denied_destination()
            self.request('/api/notifications/webhooks', {'name': 'owned-first-alert',
                         'url': f'http://127.0.0.1:{self.sink.port}/owned-alert', 'enabled': True,
                         'method': 'POST', 'service': 'generic',
                         'template': '{"instance":"{{.Instance | jsonString}}","resourceName":"{{.ResourceName | jsonString}}","event":"{{.Event | jsonString}}","type":"{{.Type | jsonString}}"}'})
            # #2540: a custom generic payload cannot establish built-in
            # Telegram delivery. Submit the wrong saved Content-Type, not a
            # Test call, custom body or real provider token. The ordinary
            # sender must apply its built-in JSON headers on both processes.
            self.request('/api/notifications/webhooks', {'name': 'owned-builtin-telegram',
                         'url': f'http://127.0.0.1:{self.sink.port}' + TELEGRAM_PATH,
                         'enabled': True, 'method': 'POST', 'service': 'telegram',
                         'headers': {'content-type': 'text/plain', 'X-Pulse-Fixture': FIXTURE_HEADER}})
            cfg = self.request('/api/alerts/config')
            cfg.update(enabled=True, activationState='active', flappingEnabled=False,
                       timeThresholds={'agent': 0}, metricTimeThresholds={'agent': {'cpu': 0}},
                       metricEvaluationWindows={'agent': {'cpu': 0}},
                       agentDefaults={'cpu': {'trigger': 80, 'clear': 70}},
                       disableAllAgents=False, disableAllAgentsOffline=True)
            cfg['schedule'].update(cooldown=0, initialNotify='webhook', notifyOnResolve=False,
                                   grouping={'enabled': False, 'window': 0, 'byNode': False, 'byGuest': False})
            self.request('/api/alerts/config', cfg, method='PUT')
            response = self.request('/api/security/tokens', {'name': 'owned-monitoring-only',
                        'scopes': ['agent:report'], 'expiresIn': '1h'})
            self.token, self.token_id = response['token'], response['record']['id']
            require(response['record']['scopes'] == ['agent:report'], 'monitoring-only scope widened')
            self.request('/api/system/settings', expected=403, auth='agent')
            self.assert_settings()
            self.assert_telegram_destination()
            require(self.sink.snapshot() == [], 'destination called before ordinary synthetic alert')
            self.phase('saved-policy-and-unlisted-destination-refusal')
            self.report('a')
            self.stop()
            self.phase('first-child-exit-and-listener-closure')
            self.start()
            # Original authenticated cookies and issued token: no relogin or token replacement.
            self.assert_settings()
            self.assert_telegram_destination()
            tokens = self.request('/api/security/tokens')
            records = tokens.get('tokens', []) if isinstance(tokens, dict) else tokens
            matches = [r for r in records if r.get('id') == self.token_id]
            require(len(matches) == 1 and matches[0].get('scopes') == ['agent:report'],
                    'issued monitoring-only token not retained at restart')
            self.denied_destination()
            self.phase('restart-session-token-and-policy-readback')
            self.report('b')
            self.stop()
            self.phase('second-child-exit-and-listener-closure')
            require(self.sink.error is None and len(self.sink.snapshot()) == 4,
                    'ordinary generic/Telegram delivery inventory differs after shutdown')
        finally:
            self.stop()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--expected-source-sha', required=True)
    parser.add_argument('--receipt', type=Path)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    binding = source_binding(binary, args.expected_source_sha)
    report = {'source_binding': binding, 'kind': 'offline-development-source-notification-runtime',
              'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), 'outcome': 'failed'}
    with tempfile.TemporaryDirectory(prefix='pulse-notification-runtime-') as temporary:
        data = Path(temporary)
        sink = Sink()
        journey = None
        try:
            with (data / 'owned-server.log').open('wb') as log:
                journey = Journey(binary, data, log, sink)
                journey.run()
                report['outcome'] = 'passed'
        except Exception as exc:
            report['error'] = str(exc) if isinstance(exc, Failure) else type(exc).__name__
        finally:
            try:
                sink.close()
                report['sink_thread_joined_and_listener_closed'] = True
            except Exception as exc:
                report['sink_thread_joined_and_listener_closed'] = False
                report['cleanup_error'] = type(exc).__name__
                report['outcome'] = 'failed'
            if journey:
                report.update(phases=journey.phases, child_exits=journey.exits,
                              requests=journey.requests, incomplete_request=journey.active_request,
                              deliveries=journey.deliveries, delivery_observations=journey.delivery_observations,
                              destination_readbacks=journey.destination_readbacks)
    report['owned_temporary_data_removed'] = not data.exists()
    report['source_clean_after'] = subprocess.run(['git', 'diff', '--quiet', 'HEAD'], check=False).returncode == 0
    if not report['source_clean_after']:
        report['outcome'] = 'failed'
    if args.receipt:
        # Never replace a previous attempt's receipt. No logs or credentials.
        with args.receipt.open('x', encoding='utf-8') as handle:
            json.dump(report, handle, sort_keys=True, indent=2)
            handle.write('\n')
    print('NOTIFICATION_RUNTIME=' + json.dumps(report, sort_keys=True), flush=True)
    return 0 if report['outcome'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
