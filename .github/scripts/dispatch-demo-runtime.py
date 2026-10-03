#!/usr/bin/env python3
"""The two existing demo CI paths share one retained, independently owned child."""
from __future__ import annotations

import base64
import hashlib
import importlib.util
import json
import datetime
import re
import os
from pathlib import Path
import shlex
import subprocess
import sys

BOOTSTRAP = r'''
import base64, fcntl, hashlib, json, os, pathlib, subprocess, sys, time
root = pathlib.Path('/var/lib/pulse-deploy/demo')
payload = json.loads(sys.stdin.read(192 * 1024 * 1024 + 1))
assert os.geteuid() == 0 and set(payload) == {'request', 'source', 'binary', 'version_file'}
source = base64.b64decode(payload['source'], validate=True)
binary = base64.b64decode(payload['binary'], validate=True)
version_file = base64.b64decode(payload['version_file'], validate=True)
assert len(source) < 128 * 1024 and len(binary) <= 128 * 1024 * 1024 and len(version_file) <= 128
for name, data in [('binary', binary), ('version', version_file)]:
    assert (hashlib.sha256(data).hexdigest() == payload['request'][name + '_sha256']) if data else (payload['request'][name + '_sha256'] == '')
identity = hashlib.sha256(json.dumps(payload, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
root.mkdir(mode=0o700, parents=True, exist_ok=True)
assert not root.is_symlink() and root.stat().st_uid == 0
os.chmod(root, 0o700)
(root / 'attempts').mkdir(mode=0o700, exist_ok=True)
assert not (root / 'attempts').is_symlink()
fd = os.open(root / 'submit.lock', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
fcntl.flock(fd, fcntl.LOCK_EX)
attempt = root / 'attempts' / identity
if not attempt.exists():
    attempt.mkdir(mode=0o700)
    for name, data in [('transaction.py', source), ('binary', binary), ('version', version_file),
                       ('request.json', json.dumps(payload['request']).encode()),
                       ('intent.json', json.dumps({'schema_version': 1, 'identity': identity, 'state': 'intent'}).encode())]:
        with open(attempt / name, 'xb') as stream:
            os.chmod(attempt / name, 0o600)
            stream.write(data); stream.flush(); os.fsync(stream.fileno())
    # Intent is durable before starting the unit. An uncertain launch is never replayed.
    for directory in (attempt, root / 'attempts', root, root.parent):
        dfd = os.open(directory, os.O_DIRECTORY); os.fsync(dfd); os.close(dfd)
    try:
        child = subprocess.run(['systemd-run', '--quiet',
                                '--unit=pulse-demo-' + identity[:32],
                                '--property=RemainAfterExit=yes',
                                '--property=RuntimeMaxSec=45min',
                                '--property=TimeoutStopSec=20min', '--property=KillMode=mixed',
                                '/usr/bin/python3', str(attempt / 'transaction.py'), str(attempt)],
                               capture_output=True, timeout=30)
        launch = {'schema_version': 1, 'state': 'submitted' if child.returncode == 0 else 'uncertain',
                  'unit': 'pulse-demo-' + identity[:32]}
    except (OSError, subprocess.TimeoutExpired):
        launch = {'schema_version': 1, 'state': 'uncertain'}
    with open(attempt / 'launch.json', 'x') as stream:
        os.chmod(attempt / 'launch.json', 0o600)
        json.dump(launch, stream); stream.flush(); os.fsync(stream.fileno())
    dfd = os.open(attempt, os.O_DIRECTORY); os.fsync(dfd); os.close(dfd)
os.close(fd)
# Closing or losing this observer never stops the independently owned unit.
deadline = time.monotonic() + 4000
terminal = {'committed', 'healthy_noop', 'rolled_back', 'rollback_failed', 'recovery_required', 'refused', 'observation_failed'}
while time.monotonic() < deadline:
    try:
        receipt = json.loads((attempt / 'receipt.json').read_text())
    except (OSError, ValueError):
        receipt = None
    if receipt and receipt.get('status') in terminal:
        # A replace can become visible before directory fsync fails. Never
        # accept that provisional favourable receipt while its writer is still
        # running, or after the writer exits failed. Retain the unit's result;
        # reading it neither launches nor stops a child.
        try:
            child = subprocess.run(['systemctl', 'show', 'pulse-demo-' + identity[:32],
                                    '--property=ActiveState,SubState,MainPID,ExecMainCode,ExecMainStatus',
                                    '--no-pager'], capture_output=True, timeout=20)
            if child.returncode:
                break
            fields = dict(line.split('=', 1) for line in child.stdout.decode().splitlines() if '=' in line)
            pid = int(fields['MainPID'])
            code = int(fields['ExecMainCode'])
            status = int(fields['ExecMainStatus'])
            pending = (pid > 0 and fields['ActiveState'] in {'activating', 'active', 'deactivating'})
            complete = (pid == 0 and code in {1, 2, 3}
                        and (fields['ActiveState'], fields['SubState']) in {('active', 'exited'), ('failed', 'failed')})
            if complete:
                succeeded = (fields['ActiveState'], fields['SubState'], code, status) == ('active', 'exited', 1, 0)
                receipt['request_identity'] = identity
                receipt['child_result'] = {'exit_code': status if code == 1 else None,
                                           'signal': status if code in {2, 3} else None}
                favourable = receipt['status'] in {'committed', 'healthy_noop'}
                expected = 0 if favourable else (2 if receipt['status'] == 'observation_failed' else 1)
                if code != 1 or status != expected or favourable != succeeded:
                    receipt.update(status='uncertain', failure='terminal-child-mismatch', recovery_required=True)
                print(json.dumps(receipt))
                sys.exit(0 if favourable and succeeded else 1)
            if not pending:
                break
        except (OSError, ValueError, KeyError, subprocess.TimeoutExpired):
            break
    launch = attempt / 'launch.json'
    if launch.exists() and json.loads(launch.read_text()).get('state') == 'uncertain':
        break
    time.sleep(5)
print(json.dumps({'schema_version': 2, 'status': 'uncertain', 'request_identity': identity,
                  'failure': 'terminal-receipt-unavailable', 'recovery_required': True}))
sys.exit(1)
'''


def payload(mode, environ=os.environ):
    profile = {key: environ['MOCK_' + key.upper()] for key in
               ('nodes', 'vms_per_node', 'lxcs_per_node', 'docker_hosts', 'docker_containers',
                'generic_hosts', 'k8s_clusters', 'k8s_nodes', 'k8s_pods', 'k8s_deployments',
                'seed_duration', 'sample_interval', 'update_interval')}
    binary = Path('/tmp/pulse-demo-binary').read_bytes() if mode == 'update' else b''
    version_file = Path('/tmp/pulse-demo-VERSION').read_bytes() if mode == 'update' else b''
    request = {'mode': mode, 'hostname': environ['DEMO_EXPECTED_HOSTNAME'],
               'local_url': environ['DEMO_LOCAL_BASE_URL'], 'public_url': environ['DEMO_PUBLIC_HEALTH_URL'],
               'version': environ['EXPECTED_VERSION'], 'profile': profile,
               'control_sha': environ['GITHUB_SHA'], 'run_id': environ['GITHUB_RUN_ID'],
               'run_attempt': environ['GITHUB_RUN_ATTEMPT'],
               'binary_sha256': hashlib.sha256(binary).hexdigest() if binary else '',
               'version_sha256': hashlib.sha256(version_file).hexdigest() if version_file else ''}
    return {'request': request, 'source': base64.b64encode(Path(__file__).with_name('demo-runtime-transaction.py').read_bytes()).decode(),
            'binary': base64.b64encode(binary).decode(), 'version_file': base64.b64encode(version_file).decode()}


NATIVE_PATHS = ('.github/scripts/demo-runtime-transaction.py',
                '.github/scripts/dispatch-demo-runtime.py',
                '.github/scripts/tests/demo_runtime_native.py',
                '.github/workflows/demo-runtime-native.yml')


def require_native(environ=os.environ):
    """Fail closed before SSH; a metadata/component pass is not native proof."""
    sha, repository = environ['GITHUB_SHA'], environ['GITHUB_REPOSITORY']
    if not re.fullmatch(r'[0-9a-f]{40}', sha) or repository != 'rcourtman/Pulse':
        raise ValueError('native-source-identity')
    since = (datetime.datetime.now(datetime.timezone.utc).date() - datetime.timedelta(days=6)).isoformat()
    base = f'repos/{repository}/actions/workflows/demo-runtime-native.yml/runs?branch=main&event=push&per_page=5'

    def read(query):
        result = subprocess.run(['gh', 'api', query], capture_output=True, timeout=30, check=False)
        if result.returncode:
            raise ValueError('native-evidence-unavailable')
        rows = json.loads(result.stdout)['workflow_runs']
        if not isinstance(rows, list) or any(not isinstance(row, dict) for row in rows):
            raise ValueError('native-evidence-malformed')
        for row in rows:
            if (type(row.get('id')) is not int or not re.fullmatch(r'[0-9a-f]{40}', str(row.get('head_sha')))
                    or type(row.get('run_attempt')) is not int or not isinstance(row.get('updated_at'), str)
                    or not isinstance(row.get('created_at'), str)):
                raise ValueError('native-evidence-malformed')
        return rows

    rows = read(base + '&created=' + since + '..*')
    if len(rows) < 5:
        rows += read(base)
    merged = {}
    for row in rows:
        old = merged.get(row['id'])
        if old is None or (row['run_attempt'], row['updated_at']) > (old['run_attempt'], old['updated_at']):
            merged[row['id']] = row
    for row in sorted(merged.values(), key=lambda r: (r['created_at'], r['id']), reverse=True):
        head = row['head_sha']
        if (row.get('event') != 'push' or row.get('head_branch') != 'main'
                or row.get('path') != '.github/workflows/demo-runtime-native.yml'):
            raise ValueError('native-run-identity')
        ancestor = subprocess.run(['git', 'merge-base', '--is-ancestor', head, sha], capture_output=True)
        if ancestor.returncode != 0:
            continue
        matching = subprocess.run(['git', 'diff', '--quiet', head, sha, '--', *NATIVE_PATHS], capture_output=True)
        if matching.returncode == 1:
            continue
        if matching.returncode != 0:
            raise ValueError('native-source-unavailable')
        # Never select an older green sample over a newer matching failure or
        # pending run. The native driver exits success only with full cleanup.
        if row.get('status') != 'completed' or row.get('conclusion') != 'success':
            raise ValueError('native-acceptance-not-passed')
        return {'run_id': row['id'], 'run_attempt': row['run_attempt'], 'source_sha': head}
    raise ValueError('native-acceptance-missing')


def prepare_runtime():
    version = os.environ['EXPECTED_VERSION']
    if not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+', version):
        raise ValueError('release-version')
    spec = importlib.util.spec_from_file_location('demo_runtime', Path(__file__).with_name('demo-runtime-transaction.py'))
    engine = importlib.util.module_from_spec(spec); spec.loader.exec_module(engine)
    # Called only after mandatory pinned-key SSH signature verification in CI.
    files = engine.runtime_members('/tmp/pulse-demo-release.tgz', version)
    for label, name in (('binary', '/tmp/pulse-demo-binary'), ('version', '/tmp/pulse-demo-VERSION')):
        with open(name, 'xb') as stream:
            os.chmod(name, 0o600)
            stream.write(files[label])


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in {'prepare', 'update', 'recover'}:
        return 2
    if sys.argv[1] == 'prepare':
        prepare_runtime()
        return 0
    # Check exact reviewed native source before opening the production SSH path.
    try:
        require_native()
    except (ValueError, KeyError, OSError, subprocess.TimeoutExpired):
        print(json.dumps({'schema_version': 2, 'status': 'refused', 'mutated': False,
                          'failure': 'native-acceptance-unavailable'}))
        return 1
    # Same project CI SSH identity and pinned host verification as before. This
    # is not an installed worker dispatcher or a new project credential.
    command = ['ssh', '-i', str(Path.home() / '.ssh/id_ed25519'), '-o', 'IdentitiesOnly=yes',
               '-o', 'StrictHostKeyChecking=yes', '-o', 'UserKnownHostsFile=' + str(Path.home() / '.ssh/known_hosts'),
               os.environ['DEMO_SERVER_USER'] + '@' + os.environ['DEMO_SERVER_HOST'],
               'sudo python3 -c ' + shlex.quote(BOOTSTRAP)]
    try:
        result = subprocess.run(command, input=json.dumps(payload(sys.argv[1])).encode(), timeout=4100, check=False)
    except (OSError, subprocess.TimeoutExpired):
        print(json.dumps({'schema_version': 2, 'status': 'uncertain', 'failure': 'observer-unavailable', 'recovery_required': True}))
        return 1
    return result.returncode


if __name__ == '__main__':
    sys.exit(main())
